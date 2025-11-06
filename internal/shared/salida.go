package shared

import (
	"API-GREENEX/internal/models"
	"context"
	"log"
	"strconv"
	"sync"
	//"time"
)

type Salida struct {
	ID               int              `json:"id"`
	SealerPhysicalID int              `json:"sealer_physical_id"`
	CognexID         int              `json:"cognex_id"` // ID de Cognex asignado a esta salida (0 = sin cognex)
	Salida_Sorter    string           `json:"salida_sorter"`
	Tipo             string           `json:"tipo"`       // "automatico" o "manual"
	MesaID           int              `json:"mesa_id"`    // ID de la mesa de paletizado (solo para salidas automáticas)
	BatchSize        int              `json:"batch_size"` // Tamaño de lote para distribución
	SKUs_Actuales    []models.SKU     `json:"skus_actuales"`
	EventChannel     chan interface{} `json:"-"`

	// Valores en tiempo real desde PLC (protegidos por mutex)
	estadoMutex sync.RWMutex
	Estado      int16 `json:"estado"` // 0=apagado, 1=andando, 2=falla (actualizado vía OPC UA)

	bloqueoMutex sync.RWMutex
	Bloqueo      bool `json:"bloqueo"` // true=bloqueada, false=disponible (actualizado vía OPC UA)

	Ingreso     bool   `json:"ingreso"`
	IsEnabled   bool   `json:"is_enabled"`
	EstadoNode  string `json:"estado_node"`  // Nodo OPC UA para estado
	BloqueoNode string `json:"bloqueo_node"` // Nodo OPC UA para bloqueo

	// Orden de fabricación activa
	IDOrdenActiva int `json:"id_orden_activa"` // ID de la última orden de fabricación activa en esta salida/mesa

	// Campos para DataMatrix (FX6)
	fx6Manager       interface{} // *db.FX6Manager (interface para evitar import cycle)
	palletClient     interface{} // Cliente para comunicación con Serfruit (interface para evitar import cycle)
	availableBoxNums []int       // Lista de números de caja disponibles
	currentBoxIndex  int         // Índice actual en la lista de cajas
	boxIndexMu       sync.Mutex  // Mutex para el índice
}

// Start inicia la gorutina para escuchar eventos de la salida
func (s *Salida) Start() {
	go func() {
		log.Printf("[Salida %d] Gorutina iniciada, escuchando eventos...", s.ID)
		for event := range s.EventChannel {
			log.Printf("[Salida %d] Evento recibido: %+v", s.ID, event)
		}
		log.Printf("[Salida %d] Canal de eventos cerrado. Gorutina terminada.", s.ID)
	}()
}

// GetEstado retorna el estado actual de forma thread-safe
func (s *Salida) GetEstado() int16 {
	s.estadoMutex.RLock()
	defer s.estadoMutex.RUnlock()
	return s.Estado
}

// SetEstado actualiza el estado de forma thread-safe
func (s *Salida) SetEstado(estado int16) {
	s.estadoMutex.Lock()
	defer s.estadoMutex.Unlock()
	s.Estado = estado
}

// GetBloqueo retorna el estado de bloqueo de forma thread-safe
func (s *Salida) GetBloqueo() bool {
	s.bloqueoMutex.RLock()
	defer s.bloqueoMutex.RUnlock()
	return s.Bloqueo
}

// SetBloqueo actualiza el estado de bloqueo de forma thread-safe
func (s *Salida) SetBloqueo(bloqueo bool) {
	s.bloqueoMutex.Lock()
	defer s.bloqueoMutex.Unlock()
	s.Bloqueo = bloqueo
}

// IsAvailable retorna true si la salida está disponible para routing
// (no está en falla y no está bloqueada)
func (s *Salida) IsAvailable() bool {
	estado := s.GetEstado()
	bloqueo := s.GetBloqueo()
	// Disponible si: ESTADO != 2 (no falla) AND BLOQUEO == false (no bloqueada)
	return estado != 2 && !bloqueo
}

func GetNewSalida(ID int, salida_sorter string, tipo string, mesaID int, batchSize int) Salida {
	// Si no se especifica batch_size, usar 1 por defecto
	if batchSize <= 0 {
		batchSize = 1
	}

	return Salida{
		ID:               ID,
		SealerPhysicalID: ID, // Por defecto igual, se puede sobrescribir después
		CognexID:         0,  // 0 = sin cognex asignado
		Salida_Sorter:    salida_sorter,
		Tipo:             tipo,
		MesaID:           mesaID,
		BatchSize:        batchSize,
		SKUs_Actuales:    []models.SKU{},
		Estado:           0,
		Ingreso:          false,
		IsEnabled:        true,
		EventChannel:     make(chan interface{}, 100),
	}
}

// GetNewSalidaWithPhysicalID crea una nueva salida con ID físico específico
func GetNewSalidaWithPhysicalID(ID int, physicalID int, salida_sorter string, tipo string, mesaID int, batchSize int) Salida {
	salida := GetNewSalida(ID, salida_sorter, tipo, mesaID, batchSize)
	salida.SealerPhysicalID = physicalID
	return salida
}

// GetNewSalidaComplete crea una nueva salida con todos los parámetros
func GetNewSalidaComplete(ID int, physicalID int, cognexID int, salida_sorter string, tipo string, mesaID int, batchSize int) Salida {
	salida := GetNewSalida(ID, salida_sorter, tipo, mesaID, batchSize)
	salida.SealerPhysicalID = physicalID
	salida.CognexID = cognexID
	return salida
}

// SetFX6Manager vincula el manager FX6 a esta salida
func (s *Salida) SetFX6Manager(fx6Manager interface{}) {
	s.fx6Manager = fx6Manager
}

// SetPalletClient vincula el cliente de Serfruit a esta salida
func (s *Salida) SetPalletClient(palletClient interface{}) {
	s.palletClient = palletClient
}

// InitializeBoxNumbers inicializa la lista de números de caja disponibles
func (s *Salida) InitializeBoxNumbers(boxNumbers []int) {
	s.boxIndexMu.Lock()
	defer s.boxIndexMu.Unlock()

	s.availableBoxNums = boxNumbers
	s.currentBoxIndex = 0
	log.Printf("📦 [Salida %d] Inicializada con %d números de caja disponibles", s.ID, len(boxNumbers))
}

// ProcessDataMatrix procesa una lectura de DataMatrix
// Correlativo: Código de orden de fabricación (del ID de orden activa)
// Número de Caja: Código correlativo leído por el DataMatrix
// Retorna el número de caja asignado y error si hubo
func (s *Salida) ProcessDataMatrix(ctx context.Context, correlativoStr string) (int, error) {
	// Convertir correlativo string a int64
	correlativo, err := strconv.ParseInt(correlativoStr, 10, 64)
	if err != nil {
		log.Printf("❌ [Salida %d] Error al convertir correlativo '%s' a número: %v", s.SealerPhysicalID, correlativoStr, err)
		return 0, err
	}

	// Obtener siguiente número de caja del pool
	numeroCaja := correlativo % int64(len(s.availableBoxNums))

	//fechaLectura := time.Now()
	/*LOGICA VIEJA
	// Insertar en SQL Server FX6 solo si hay manager disponible
	if s.fx6Manager != nil {
		// Type assertion para usar el método
		type FX6Inserter interface {
			InsertLecturaDataMatrix(ctx context.Context, salida int, correlativo int64, numeroCaja int64, fechaLectura time.Time) error
		}

		if fx6, ok := s.fx6Manager.(FX6Inserter); ok {
			err := fx6.InsertLecturaDataMatrix(
				ctx,
				s.SealerPhysicalID,
				int64(s.IDOrdenActiva),
				correlativo,
				fechaLectura,
			)

			if err != nil {
				// Solo logear, no detener el proceso
				log.Printf("❌ [Salida %d] Error al insertar DataMatrix en FX6 (Correlativo=%d, Caja=%d): %v",
					s.SealerPhysicalID, correlativo, numeroCaja, err)
			} else {
				log.Printf("✅ [Salida %d] DataMatrix registrado en FX6: Correlativo=%d → Caja=%d",
					s.SealerPhysicalID, correlativo, numeroCaja)
			}
		}
	}
	*/
	// Enviar nueva caja a Serfruit (solo para salidas automáticas con mesa)
	if s.Tipo == "automatico" && s.MesaID > 0 && s.palletClient != nil {
		// Type assertion para usar el método RegistrarNuevaCaja
		type PalletCajaRegistrar interface {
			RegistrarNuevaCaja(ctx context.Context, idMesa int, idCaja string) error
		}

		if pallet, ok := s.palletClient.(PalletCajaRegistrar); ok {
			err := pallet.RegistrarNuevaCaja(ctx, s.MesaID, correlativoStr)
			if err != nil {
				// Solo logear, no detener el proceso
				log.Printf("⚠️  [Salida %d] Error al enviar caja a Serfruit (Mesa=%d, IDCaja=%s): %v",
					s.SealerPhysicalID, s.MesaID, correlativoStr, err)
			} else {
				log.Printf("✅ [Salida %d] Caja enviada a Serfruit: Mesa=%d, IDCaja=%s",
					s.SealerPhysicalID, s.MesaID, correlativoStr)
			}
		}
	}

	return int(numeroCaja), nil
}
