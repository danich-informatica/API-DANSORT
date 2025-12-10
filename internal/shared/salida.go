package shared

import (
	"API-GREENEX/internal/db"
	"API-GREENEX/internal/models"
	"context"
	"database/sql"
	"fmt"
	"log"
	"strconv"
	"sync"
	"time"
)

// TipoEstadoCaja representa el estado del procesamiento de una caja
type TipoEstadoCaja string

const (
	EstadoCorrelativoVacio TipoEstadoCaja = "correlativo_vacio"
	EstadoCajaCorrecta     TipoEstadoCaja = "caja_correcta"
	EstadoCajaIncorrecta   TipoEstadoCaja = "caja_incorrecta"
	EstadoNoEncontrada     TipoEstadoCaja = "caja_no_encontrada"
	EstadoErrorConsulta    TipoEstadoCaja = "error_consulta"
)

// EstadoCaja representa el estado de procesamiento de una caja individual
type EstadoCaja struct {
	Correlativo string         `json:"correlativo"`
	Estado      TipoEstadoCaja `json:"estado"`
	Calibre     string         `json:"calibre,omitempty"`
	Variedad    string         `json:"variedad,omitempty"`
	Embalaje    string         `json:"embalaje,omitempty"`
	SKU         string         `json:"sku,omitempty"`
	Timestamp   time.Time      `json:"timestamp"`
	Mensaje     string         `json:"mensaje,omitempty"`
}

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

	// Manager de SSMS (SQL Server) - agregado para permitir inyección
	SSMSManager *db.Manager // Manager de SSMS para operaciones con la base de datos

	// WebSocket Hub para publicar eventos en tiempo real
	WebSocketHub interface{} // Interface para evitar import cycle
	SorterID     int         // ID del sorter al que pertenece esta salida

	// Buffer circular para estados de cajas (últimas 100)
	estadosCajas     [100]EstadoCaja // Buffer circular de estados
	estadosIndex     int             // Índice actual en el buffer circular
	estadosMutex     sync.RWMutex    // Mutex para operaciones thread-safe
	EstadosCajasChan chan EstadoCaja // Channel para publicar eventos de estado
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

func GetNewSalida(ID int, salida_sorter string, tipo string, mesaID int, batchSize int, SSMSmanager *db.Manager) Salida {
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
		SSMSManager:      SSMSmanager,
		Ingreso:          false,
		IsEnabled:        true,
		EventChannel:     make(chan interface{}, 100),
		EstadosCajasChan: make(chan EstadoCaja, 100), // Buffer de 100 para evitar bloqueos
		estadosIndex:     0,
	}
}

// GetNewSalidaWithPhysicalID crea una nueva salida con ID físico específico
func GetNewSalidaWithPhysicalID(ID int, physicalID int, salida_sorter string, tipo string, mesaID int, batchSize int, SSMSmanager *db.Manager) Salida {
	salida := GetNewSalida(ID, salida_sorter, tipo, mesaID, batchSize, SSMSmanager)
	salida.SealerPhysicalID = physicalID
	return salida
}

// GetNewSalidaComplete crea una nueva salida con todos los parámetros
func GetNewSalidaComplete(ID int, physicalID int, cognexID int, salida_sorter string, tipo string, mesaID int, batchSize int, SSMSmanager *db.Manager) Salida {
	salida := GetNewSalida(ID, salida_sorter, tipo, mesaID, batchSize, SSMSmanager)
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

// SetSSMSManager permite inyectar un manager de SSMS (SQL Server) ya configurado.
// Esto evita que `ProcessDataMatrix` intente crear/usar el singleton con valores
// por defecto (que pueden apuntar a localhost).
func (s *Salida) SetSSMSManager(mgr *db.Manager) {
	s.SSMSManager = mgr
}

// SetWebSocketHub vincula el WebSocket Hub a esta salida para publicar eventos
func (s *Salida) SetWebSocketHub(hub interface{}, sorterID int) {
	s.WebSocketHub = hub
	s.SorterID = sorterID
}

// StartBoxStatusWorker inicia un worker que escucha el channel de estados de cajas
// y publica las estadísticas al WebSocket Hub
func (s *Salida) StartBoxStatusWorker() {
	go func() {
		log.Printf("📊 [Salida %d] Worker de estados de cajas iniciado", s.SealerPhysicalID)

		for estado := range s.EstadosCajasChan {
			// Calcular estadísticas actualizadas
			estadoActual, ultimos5, contadores, porcentajes, totalCajas := s.CalcularEstadisticasParaWebSocket()

			// Crear estructura de datos para WebSocket
			// Necesitamos hacer type assertion para usar el método NotifyBoxStatus
			type BoxStatusNotifier interface {
				NotifyBoxStatus(sorterID int, data interface{})
			}

			if hub, ok := s.WebSocketHub.(BoxStatusNotifier); ok && s.SorterID > 0 {
				// Convertir contadores y porcentajes a map[string]interface{} para JSON
				contadoresJSON := make(map[string]int)
				for k, v := range contadores {
					contadoresJSON[string(k)] = v
				}
				
				porcentajesJSON := make(map[string]float64)
				for k, v := range porcentajes {
					porcentajesJSON[string(k)] = v
				}

				// Crear estructura de datos compatible con BoxStatusData en websocket.go
				data := map[string]interface{}{
					"salida_id":          s.ID,
					"salida_physical_id": s.SealerPhysicalID,
					"estado_actual":      estadoActual,
					"ultimos_5":          ultimos5,
					"contadores":         contadoresJSON,
					"porcentajes":        porcentajesJSON,
					"total_cajas":        totalCajas,
				}

				hub.NotifyBoxStatus(s.SorterID, data)

				log.Printf("📤 [Salida %d] Estadísticas de cajas publicadas: Total=%d, Estado=%s",
					s.SealerPhysicalID, totalCajas, estado.Estado)
			}
		}

		log.Printf("⚠️  [Salida %d] Channel de estados cerrado, worker terminado", s.SealerPhysicalID)
	}()
}

// InitializeBoxNumbers inicializa la lista de números de caja disponibles
func (s *Salida) InitializeBoxNumbers(boxNumbers []int) {
	s.boxIndexMu.Lock()
	defer s.boxIndexMu.Unlock()

	s.availableBoxNums = boxNumbers
	s.currentBoxIndex = 0
	log.Printf("📦 [Salida %d] Inicializada con %d números de caja disponibles", s.ID, len(boxNumbers))
}

// RegistrarEstadoCaja agrega un estado al buffer circular y lo publica en el channel
func (s *Salida) RegistrarEstadoCaja(estado EstadoCaja) {
	s.estadosMutex.Lock()

	// Agregar timestamp si no existe
	if estado.Timestamp.IsZero() {
		estado.Timestamp = time.Now()
	}

	// Guardar en buffer circular
	s.estadosCajas[s.estadosIndex] = estado
	s.estadosIndex = (s.estadosIndex + 1) % 100

	s.estadosMutex.Unlock()

	// Publicar en channel de forma no bloqueante
	select {
	case s.EstadosCajasChan <- estado:
		// Evento publicado exitosamente
	default:
		// Channel lleno, no bloquear
		log.Printf("⚠️  [Salida %d] Channel de estados lleno, evento descartado", s.SealerPhysicalID)
	}
}

// ObtenerHistorialEstados retorna los últimos estados registrados (hasta 100)
func (s *Salida) ObtenerHistorialEstados() []EstadoCaja {
	s.estadosMutex.RLock()
	defer s.estadosMutex.RUnlock()

	historial := make([]EstadoCaja, 0, 100)

	// Leer desde el índice actual hacia atrás (más reciente primero)
	for i := 0; i < 100; i++ {
		idx := (s.estadosIndex - 1 - i + 100) % 100
		estado := s.estadosCajas[idx]

		// Si encontramos un estado con timestamp cero, hemos llegado al inicio
		if estado.Timestamp.IsZero() {
			break
		}

		historial = append(historial, estado)
	}

	return historial
}

// ObtenerEstadisticasEstados retorna un resumen de los estados en el historial
func (s *Salida) ObtenerEstadisticasEstados() map[TipoEstadoCaja]int {
	s.estadosMutex.RLock()
	defer s.estadosMutex.RUnlock()

	stats := make(map[TipoEstadoCaja]int)

	for i := 0; i < 100; i++ {
		estado := s.estadosCajas[i]
		if !estado.Timestamp.IsZero() {
			stats[estado.Estado]++
		}
	}

	return stats
}

// CalcularEstadisticasParaWebSocket calcula todas las métricas necesarias para el frontend
func (s *Salida) CalcularEstadisticasParaWebSocket() (EstadoCaja, []EstadoCaja, map[TipoEstadoCaja]int, map[TipoEstadoCaja]float64, int) {
	s.estadosMutex.RLock()
	defer s.estadosMutex.RUnlock()

	// 1. Estado actual (el más reciente)
	var estadoActual EstadoCaja
	if s.estadosIndex > 0 {
		estadoActual = s.estadosCajas[(s.estadosIndex-1+100)%100]
	} else {
		// Si no hay estados aún, tomar el último slot
		estadoActual = s.estadosCajas[99]
	}

	// 2. Últimos 5 estados
	ultimos5 := make([]EstadoCaja, 0, 5)
	for i := 0; i < 5 && i < 100; i++ {
		idx := (s.estadosIndex - 1 - i + 100) % 100
		estado := s.estadosCajas[idx]
		if !estado.Timestamp.IsZero() {
			ultimos5 = append(ultimos5, estado)
		} else {
			break // No hay más estados
		}
	}

	// 3. Contadores por tipo de estado
	contadores := make(map[TipoEstadoCaja]int)
	totalCajas := 0

	for i := 0; i < 100; i++ {
		estado := s.estadosCajas[i]
		if !estado.Timestamp.IsZero() {
			contadores[estado.Estado]++
			totalCajas++
		}
	}

	// 4. Porcentajes por tipo de estado
	porcentajes := make(map[TipoEstadoCaja]float64)
	if totalCajas > 0 {
		for tipo, count := range contadores {
			porcentajes[tipo] = float64(count) * 100.0 / float64(totalCajas)
		}
	}

	return estadoActual, ultimos5, contadores, porcentajes, totalCajas
}

// ProcessDataMatrix procesa una lectura de DataMatrix
// Correlativo: Código de orden de fabricación (del ID de orden activa)
// Número de Caja: Código correlativo leído por el DataMatrix
// Retorna el número de caja asignado y error si hubo
func (s *Salida) ProcessDataMatrix(ctx context.Context, correlativoStr string) (int, error) {
	// Validar correlativo vacío
	if correlativoStr == "" {
		estado := EstadoCaja{
			Correlativo: correlativoStr,
			Estado:      EstadoCorrelativoVacio,
			Mensaje:     "Correlativo vacío recibido",
		}
		s.RegistrarEstadoCaja(estado)
		log.Printf("⚠️  [Salida %d] Correlativo vacío recibido", s.SealerPhysicalID)
		return 0, fmt.Errorf("correlativo vacío")
	}

	// Convertir correlativo string a int64
	correlativo, err := strconv.ParseInt(correlativoStr, 10, 64)
	if err != nil {
		estado := EstadoCaja{
			Correlativo: correlativoStr,
			Estado:      EstadoCorrelativoVacio,
			Mensaje:     fmt.Sprintf("Error al convertir correlativo a número: %v", err),
		}
		s.RegistrarEstadoCaja(estado)
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
	numeroCaja := correlativo % int64(len(s.availableBoxNums))

	cajaCorrecta := false // Asumir que la caja no es correcta hasta verificar
	var calibreStr, variedadStr, embalajeStr string

	if s.SSMSManager != nil {
		// Ejecutar la consulta en la DB de Unitec
		rows, err := s.SSMSManager.Query(ctx, db.SELECT_BOX_DATA_FROM_UNITEC_DB, sql.Named("p1", correlativoStr))
		if err != nil {
			// Registrar error de consulta
			estado := EstadoCaja{
				Correlativo: correlativoStr,
				Estado:      EstadoErrorConsulta,
				Mensaje:     fmt.Sprintf("Error ejecutando consulta: %v", err),
			}
			s.RegistrarEstadoCaja(estado)
			log.Printf("❌ [Salida %d] Error ejecutando consulta SELECT_BOX_DATA_FROM_UNITEC_DB: %v", s.SealerPhysicalID, err)
		} else {
			defer func() {
				if cerr := rows.Close(); cerr != nil {
					log.Printf("⚠️  [Salida %d] Error cerrando resultados de consulta Unitec: %v", s.SealerPhysicalID, cerr)
				}
			}()
			var calibre, variedad, embalaje sql.NullString
			if rows.Next() {
				if err := rows.Scan(&calibre, &variedad, &embalaje); err != nil {
					// Registrar error al leer resultado
					estado := EstadoCaja{
						Correlativo: correlativoStr,
						Estado:      EstadoErrorConsulta,
						Mensaje:     fmt.Sprintf("Error leyendo resultado: %v", err),
					}
					s.RegistrarEstadoCaja(estado)
					log.Printf("❌ [Salida %d] Error leyendo resultado de consulta Unitec: %v", s.SealerPhysicalID, err)
				} else {
					calibreStr = calibre.String
					variedadStr = variedad.String
					embalajeStr = embalaje.String

					log.Printf("✅ [Salida %d] Datos caja desde Unitec -> calibre=%s variedad=%s embalaje=%s",
						s.SealerPhysicalID, calibreStr, variedadStr, embalajeStr)

					// Buscar coincidencia con SKUs actuales
					for _, sku := range s.SKUs_Actuales {
						if sku.Calibre == calibreStr && sku.Variedad == variedadStr && sku.Embalaje == embalajeStr {
							log.Printf("📦 [Salida %d] La caja con codCaja=%s corresponde a la SKU %s",
								s.SealerPhysicalID, correlativoStr, sku.SKU)
							cajaCorrecta = true

							// Registrar caja correcta
							estado := EstadoCaja{
								Correlativo: correlativoStr,
								Estado:      EstadoCajaCorrecta,
								Calibre:     calibreStr,
								Variedad:    variedadStr,
								Embalaje:    embalajeStr,
								SKU:         sku.SKU,
								Mensaje:     fmt.Sprintf("Caja válida para SKU %s", sku.SKU),
							}
							s.RegistrarEstadoCaja(estado)
							break
						}
					}

					if !cajaCorrecta {
						// Registrar caja incorrecta
						estado := EstadoCaja{
							Correlativo: correlativoStr,
							Estado:      EstadoCajaIncorrecta,
							Calibre:     calibreStr,
							Variedad:    variedadStr,
							Embalaje:    embalajeStr,
							Mensaje:     "Caja no corresponde a ninguna SKU válida para esta salida",
						}
						s.RegistrarEstadoCaja(estado)
						log.Printf("❌ [Salida %d] La caja con codCaja=%s NO corresponde a ninguna SKU válida para esta salida",
							s.SealerPhysicalID, correlativoStr)
					}
				}
			} else {
				// Registrar caja no encontrada
				estado := EstadoCaja{
					Correlativo: correlativoStr,
					Estado:      EstadoNoEncontrada,
					Mensaje:     "No se encontraron datos en Unitec para este correlativo",
				}
				s.RegistrarEstadoCaja(estado)
				log.Printf("⚠️  [Salida %d] No se encontraron datos en Unitec para codCaja=%s", s.SealerPhysicalID, correlativoStr)
			}
		}
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

	return int(numeroCaja), nil
}
