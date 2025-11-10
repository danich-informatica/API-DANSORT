package shared

import (
	"api-dansort/internal/db"
	"api-dansort/internal/models"
	"context"
	"database/sql"
	"fmt"
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
	fx6Manager   interface{} // *db.FX6Manager (interface para evitar import cycle)
	palletClient interface{} // Cliente para comunicación con Serfruit (interface para evitar import cycle)
	// SSMSManager opcional: permite inyectar un manager configurado para la DB de Unitec
	// Si no se inyecta, se intentará obtener el singleton con db.GetManager(ctx)
	SSMSManager      interface{}
	availableBoxNums []int      // Lista de números de caja disponibles
	currentBoxIndex  int        // Índice actual en la lista de cajas
	boxIndexMu       sync.Mutex // Mutex para el índice
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

// GetNewSalidaComplete crea una nueva salida con todos los parámetros
func (s *Salida) GetNewSalidaComplete(ID int, physicalID int, cognexID int, salida_sorter string, tipo string, mesaID int, batchSize int) *Salida {
	salida := GetNewSalida(ID, salida_sorter, tipo, mesaID, batchSize)
	salida.SealerPhysicalID = physicalID
	salida.CognexID = cognexID
	return &salida
}

func (s *Salida) GetNewSalidaWithPhysicalID(ID int, physicalID int, salida_sorter string, tipo string, mesaID int, batchSize int) *Salida {
	salida := GetNewSalida(ID, salida_sorter, tipo, mesaID, batchSize)
	salida.SealerPhysicalID = physicalID
	return &salida
}

// SetFX6Manager vincula el manager FX6 a esta salida
func (s *Salida) SetFX6Manager(fx6Manager interface{}) {
	s.fx6Manager = fx6Manager
}

// SetPalletClient vincula el cliente de Serfruit a esta salida
func (s *Salida) SetPalletClient(palletClient interface{}) {
	s.palletClient = palletClient
}

// SetSSMSManager permite inyectar un manager de SSMS (SQL Server) ya configurado.
// Esto evita que `ProcessDataMatrix` intente crear/usar el singleton con valores
// por defecto (que pueden apuntar a localhost). Pasar nil restaura el comportamiento
// de fallback (db.GetManager).
func (s *Salida) SetSSMSManager(mgr interface{}) {
	s.SSMSManager = mgr
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
	_, err := strconv.ParseInt(correlativoStr, 10, 64)
	if err != nil {
		log.Printf("❌ [Salida %d] Error al convertir correlativo '%s' a número: %v", s.SealerPhysicalID, correlativoStr, err)
		return 0, err
	}

	// Guardar y validar pool de cajas
	if len(s.availableBoxNums) == 0 {
		err := fmt.Errorf("no hay números de caja inicializados para la salida %d", s.ID)
		log.Printf("❌ [Salida %d] %v", s.SealerPhysicalID, err)
		return 0, err
	}

	// Obtener siguiente número de caja del pool
	s.boxIndexMu.Lock()
	if s.currentBoxIndex >= len(s.availableBoxNums) {
		s.currentBoxIndex = 0 // Reiniciar si llega al final
	}
	numeroCaja := s.availableBoxNums[s.currentBoxIndex]
	s.currentBoxIndex++
	s.boxIndexMu.Unlock()

	// Usar el manager inyectado (s.SSMSManager) si está disponible.
	// Si no, como fallback, intentar obtener el singleton (que podría fallar).
	var cajaCorrecta bool = false // Por defecto, la caja no es correcta hasta que se valide

	type QueryExecutor interface {
		QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error)
	}

	var querier QueryExecutor

	if s.SSMSManager != nil {
		var ok bool
		querier, ok = s.SSMSManager.(QueryExecutor)
		if !ok {
			log.Printf("⚠️  [Salida %d] El SSMSManager inyectado no es un db.QueryExecutor válido. No se puede validar la caja.", s.SealerPhysicalID)
		}
	} else {
		log.Printf("⚠️  [Salida %d] No se inyectó un SSMSManager, intentando fallback a db.GetManager()", s.SealerPhysicalID)
		// El tipo concreto que devuelve GetManager debe implementar QueryExecutor.
		// Asumimos que db.GetManager() devuelve un tipo que lo cumple.
		// NOTA: Esta es una dependencia implícita que debería manejarse mejor con interfaces.
		ssmsManager, mgrErr := db.GetManager(ctx)
		if mgrErr != nil {
			log.Printf("⚠️  [Salida %d] Fallback fallido: No fue posible obtener SSMS manager: %v", s.SealerPhysicalID, mgrErr)
		} else {
			querier = ssmsManager
		}
	}

	if querier != nil {
		// Ahora podemos usar 'querier' para ejecutar la consulta
		query := "SELECT calibre, variedad, embalaje FROM UNITEC_DB.dbo.DatosCajas WHERE cod_caja_produccion = @p1"
		rows, err := querier.QueryContext(ctx, query, correlativoStr)
		if err != nil {
			log.Printf("❌ [Salida %d] Error al consultar Unitec para codCaja=%s: %v", s.SealerPhysicalID, correlativoStr, err)
		} else {
			defer rows.Close()
			if rows.Next() {
				var calibre, variedad, embalaje sql.NullString
				if err := rows.Scan(&calibre, &variedad, &embalaje); err != nil {
					log.Printf("❌ [Salida %d] Error leyendo resultado de consulta Unitec: %v", s.SealerPhysicalID, err)
				} else {
					log.Printf("✅ [Salida %d] Datos caja desde Unitec -> calibre=%s variedad=%s embalaje=%s", s.SealerPhysicalID, calibre.String, variedad.String, embalaje.String)
					// Aquí iría la lógica para comparar con los SKUs de la salida
					// Por ahora, si encontramos datos, asumimos que es correcta.
					cajaCorrecta = true
				}
			} else {
				log.Printf("⚠️  [Salida %d] No se encontraron datos en Unitec para codCaja=%s", s.SealerPhysicalID, correlativoStr)
			}
		}
	} else {
		log.Printf("❌ [Salida %d] No hay un SSMS manager disponible para consultar la caja %s. No se puede validar.", s.SealerPhysicalID, correlativoStr)
	}

	// Enviar nueva caja a Serfruit (solo para salidas automáticas con mesa)
	if s.Tipo == "automatico" && s.MesaID > 0 && s.palletClient != nil && cajaCorrecta {
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

	return numeroCaja, nil
}
