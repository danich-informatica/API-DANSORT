// (Eliminado: función duplicada antes de package)
package sorter

import (
	"API-GREENEX/internal/communication/plc"
	"API-GREENEX/internal/db"
	"API-GREENEX/internal/listeners"
	"API-GREENEX/internal/models"
	"API-GREENEX/internal/shared"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"
)

type Sorter struct {
	ID            int                       `json:"id"`
	Ubicacion     string                    `json:"ubicacion"`
	PLCInputNode  string                    `json:"plc_input_node"`  // Nodo OPC UA para enviar al PLC
	PLCOutputNode string                    `json:"plc_output_node"` // Nodo OPC UA para recibir del PLC
	Salidas       []shared.Salida           `json:"salidas"`
	Cognex        *listeners.CognexListener `json:"cognex"`
	ctx           context.Context
	cancel        context.CancelFunc

	// PLC Manager para comunicación OPC UA
	plcManager *plc.Manager

	// Funciones para cancelar suscripciones OPC UA
	cancelSubscriptions []func()
	subscriptionMutex   sync.Mutex

	// Canal de SKUs assignables para este sorter
	skuChannel chan []models.SKUAssignable

	// Canal de estadísticas de flujo
	flowStatsChannel chan models.FlowStatistics

	// SKUs actualmente asignados
	assignedSKUs []models.SKUAssignable

	// Estadísticas de flujo (ventana deslizante)
	lecturaRecords []models.LecturaRecord
	lastFlowStats  map[string]float64 // SKU → Porcentaje del último cálculo
	flowMutex      sync.RWMutex

	// Sistema de distribución por lotes (por SKU)
	batchCounters map[string]*BatchDistributor // SKU → Distribuidor de lotes
	batchMutex    sync.RWMutex

	// WebSocket Hub para enviar eventos
	wsHub *listeners.WebSocketHub

	// Database manager para registrar salidas de cajas
	dbManager interface{}

	// Estadísticas generales
	LecturasExitosas int
	LecturasFallidas int
}

// BatchDistributor maneja la distribución por lotes entre salidas
type BatchDistributor struct {
	Salidas      []int       // IDs de salidas con esta SKU
	CurrentIndex int         // Índice de salida actual
	CurrentCount int         // Contador actual del lote
	BatchSizes   map[int]int // Salida ID → Batch Size
}

// NextSalida retorna el ID de la próxima salida según el patrón de lotes
func (bd *BatchDistributor) NextSalida() int {
	if len(bd.Salidas) == 0 {
		return -1 // No hay salidas configuradas
	}

	// Si solo hay una salida, siempre retornar la misma
	if len(bd.Salidas) == 1 {
		return bd.Salidas[0]
	}

	// Obtener ID de salida actual
	salidaID := bd.Salidas[bd.CurrentIndex]
	batchSize := bd.BatchSizes[salidaID]

	// Incrementar contador
	bd.CurrentCount++

	// Si alcanzamos el tamaño del lote, avanzar a siguiente salida
	if bd.CurrentCount >= batchSize {
		bd.CurrentIndex = (bd.CurrentIndex + 1) % len(bd.Salidas)
		bd.CurrentCount = 0
	}

	return salidaID
}

// GetSalidas retorna todas las salidas del sorter
func (s *Sorter) GetSalidas() []shared.Salida {
	return s.Salidas
}

// updateBatchDistributor actualiza o crea el distribuidor de lotes para una SKU
// DEBE SER LLAMADO CON batchMutex LOCKED
func (s *Sorter) updateBatchDistributor(skuName string) {
	// Buscar todas las salidas que contienen esta SKU
	var salidaIDs []int
	batchSizes := make(map[int]int)

	for _, salida := range s.Salidas {
		for _, sku := range salida.SKUs_Actuales {
			if sku.SKU == skuName {
				salidaIDs = append(salidaIDs, salida.ID)
				batchSizes[salida.ID] = salida.BatchSize
				break
			}
		}
	}

	// Si solo hay una salida o ninguna, no necesitamos distribuidor
	if len(salidaIDs) <= 1 {
		delete(s.batchCounters, skuName)
		return
	}

	// Crear o actualizar el distribuidor
	if bd, exists := s.batchCounters[skuName]; exists {
		// Actualizar existente
		bd.Salidas = salidaIDs
		bd.BatchSizes = batchSizes
		// Resetear si cambió la configuración
		if bd.CurrentIndex >= len(salidaIDs) {
			bd.CurrentIndex = 0
			bd.CurrentCount = 0
		}
	} else {
		// Crear nuevo
		s.batchCounters[skuName] = &BatchDistributor{
			Salidas:      salidaIDs,
			CurrentIndex: 0,
			CurrentCount: 0,
			BatchSizes:   batchSizes,
		}
		log.Printf("🔄 Sorter #%d: BatchDistributor creado para SKU '%s' con %d salidas", s.ID, skuName, len(salidaIDs))
	}
}

func GetNewSorter(ID int, ubicacion string, plcInputNode string, plcOutputNode string, salidas []shared.Salida, cognex *listeners.CognexListener, wsHub *listeners.WebSocketHub, dbManager interface{}, plcManager *plc.Manager) *Sorter {
	ctx, cancel := context.WithCancel(context.Background())

	// Registrar canales para este sorter
	channelMgr := shared.GetChannelManager()
	sorterID := fmt.Sprintf("%d", ID)
	skuChannel := channelMgr.RegisterSorterSKUChannel(sorterID, 10)            // Buffer de 10
	flowStatsChannel := channelMgr.RegisterSorterFlowStatsChannel(sorterID, 5) // Buffer de 5

	return &Sorter{
		ID:                  ID,
		Ubicacion:           ubicacion,
		PLCInputNode:        plcInputNode,
		PLCOutputNode:       plcOutputNode,
		Salidas:             salidas,
		Cognex:              cognex,
		ctx:                 ctx,
		cancel:              cancel,
		plcManager:          plcManager,
		cancelSubscriptions: make([]func(), 0),
		skuChannel:          skuChannel,
		flowStatsChannel:    flowStatsChannel,
		assignedSKUs:        make([]models.SKUAssignable, 0),
		lecturaRecords:      make([]models.LecturaRecord, 0, 1000), // Capacidad inicial 1000
		lastFlowStats:       make(map[string]float64),              // Inicializar mapa de porcentajes
		batchCounters:       make(map[string]*BatchDistributor),    // Inicializar distribuidores de lote
		wsHub:               wsHub,                                 // Asignar WebSocket Hub
		dbManager:           dbManager,                             // Asignar DB Manager
	}
}

func (s *Sorter) Start() error {
	log.Printf("🚀 Sorter #%d: Iniciando en %s", s.ID, s.Ubicacion)
	log.Printf("📍 Sorter #%d: Salidas configuradas: %d", s.ID, len(s.Salidas))

	for _, salida := range s.Salidas {
		log.Printf("  ↳ Sorter #%d: Salida %d: %s", s.ID, salida.ID, salida.Salida_Sorter)
	}

	// Iniciar el listener de Cognex
	if s.Cognex != nil {
		err := s.Cognex.Start()
		if err != nil {
			return err
		}
	}

	// 🆕 Iniciar goroutine para procesar eventos de lectura
	go s.procesarEventosCognex()

	// 🆕 Iniciar suscripciones OPC UA para ESTADO y BLOQUEO de cada salida
	if s.plcManager != nil {
		s.startPLCSubscriptions()
	}

	log.Printf("✅ Sorter #%d: Iniciado y escuchando eventos de Cognex", s.ID)

	return nil
}

// startPLCSubscriptions inicia UNA suscripción OPC UA para monitorear TODOS los nodos (ESTADO y BLOQUEO)
func (s *Sorter) startPLCSubscriptions() {
	log.Printf("🔔 Sorter #%d: Iniciando suscripción OPC UA compartida para Estado y Bloqueo...", s.ID)

	// Recolectar todos los nodeIDs a monitorear
	nodeIDs := make([]string, 0, len(s.Salidas)*2)
	nodeToSalidaMap := make(map[string]*shared.Salida) // nodeID -> salida
	nodeToTypeMap := make(map[string]string)           // nodeID -> "estado" o "bloqueo"

	for i := range s.Salidas {
		salida := &s.Salidas[i]

		if salida.EstadoNode != "" {
			nodeIDs = append(nodeIDs, salida.EstadoNode)
			nodeToSalidaMap[salida.EstadoNode] = salida
			nodeToTypeMap[salida.EstadoNode] = "estado"
		}

		if salida.BloqueoNode != "" {
			nodeIDs = append(nodeIDs, salida.BloqueoNode)
			nodeToSalidaMap[salida.BloqueoNode] = salida
			nodeToTypeMap[salida.BloqueoNode] = "bloqueo"
		}
	}

	if len(nodeIDs) == 0 {
		log.Printf("⚠️  Sorter #%d: No hay nodos OPC UA configurados para monitorear", s.ID)
		return
	}

	log.Printf("📊 Sorter #%d: Monitoreando %d nodos en una sola suscripción", s.ID, len(nodeIDs))

	// Crear UNA suscripción para TODOS los nodos
	ctx := s.ctx
	interval := 100 * time.Millisecond

	dataChan, cancelFunc, err := s.plcManager.MonitorMultipleNodes(ctx, s.ID, nodeIDs, interval)
	if err != nil {
		log.Printf("❌ Sorter #%d: Error creando suscripción compartida: %v", s.ID, err)
		return
	}

	// Guardar función de cancelación
	s.subscriptionMutex.Lock()
	s.cancelSubscriptions = append(s.cancelSubscriptions, cancelFunc)
	s.subscriptionMutex.Unlock()

	// Goroutine para procesar TODAS las notificaciones
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case nodeInfo, ok := <-dataChan:
				if !ok {
					log.Printf("⚠️  Sorter #%d: Canal de suscripción OPC UA cerrado", s.ID)
					return
				}

				// Obtener la salida y el tipo asociado a este nodeID
				salida, exists := nodeToSalidaMap[nodeInfo.NodeID]
				if !exists {
					continue
				}

				nodeType := nodeToTypeMap[nodeInfo.NodeID]

				// Procesar según el tipo de nodo
				if nodeType == "estado" {
					// Convertir valor a int16
					var estadoValor int16
					switch v := nodeInfo.Value.(type) {
					case int16:
						estadoValor = v
					case int32:
						estadoValor = int16(v)
					case int64:
						estadoValor = int16(v)
					case int:
						estadoValor = int16(v)
					default:
						log.Printf("⚠️ Sorter #%d: Tipo de valor inesperado para ESTADO de salida %d: %T",
							s.ID, salida.ID, nodeInfo.Value)
						continue
					}

					// Obtener valor anterior
					estadoAnterior := salida.GetEstado()

					// Actualizar estado
					salida.SetEstado(estadoValor)

					// Log solo si cambió
					if estadoAnterior != estadoValor {
						estadoNombre := "DESCONOCIDO"
						switch estadoValor {
						case 0:
							estadoNombre = "APAGADO"
						case 1:
							estadoNombre = "ANDANDO"
						case 2:
							estadoNombre = "FALLA"
						}
						log.Printf("🔄 Sorter #%d: Salida %d (%s) - ESTADO cambió: %d→%d (%s)",
							s.ID, salida.ID, salida.Salida_Sorter, estadoAnterior, estadoValor, estadoNombre)
					}

				} else if nodeType == "bloqueo" {
					// Convertir valor a bool
					var bloqueoValor bool
					switch v := nodeInfo.Value.(type) {
					case bool:
						bloqueoValor = v
					case int16:
						bloqueoValor = v != 0
					case int32:
						bloqueoValor = v != 0
					case int64:
						bloqueoValor = v != 0
					case int:
						bloqueoValor = v != 0
					default:
						log.Printf("⚠️ Sorter #%d: Tipo de valor inesperado para BLOQUEO de salida %d: %T",
							s.ID, salida.ID, nodeInfo.Value)
						continue
					}

					// Obtener valor anterior
					bloqueoAnterior := salida.GetBloqueo()

					// Actualizar bloqueo
					salida.SetBloqueo(bloqueoValor)

					// Log solo si cambió
					if bloqueoAnterior != bloqueoValor {
						estadoTexto := "DESBLOQUEADA"
						if bloqueoValor {
							estadoTexto = "BLOQUEADA"
						}
						log.Printf("🔒 Sorter #%d: Salida %d (%s) - BLOQUEO cambió: %t→%t (%s)",
							s.ID, salida.ID, salida.Salida_Sorter, bloqueoAnterior, bloqueoValor, estadoTexto)
					}
				}
			}
		}
	}()

	log.Printf("✅ Sorter #%d: Suscripción compartida iniciada para %d nodos", s.ID, len(nodeIDs))
}

// stopPLCSubscriptions detiene todas las suscripciones OPC UA
func (s *Sorter) stopPLCSubscriptions() {
	s.subscriptionMutex.Lock()
	defer s.subscriptionMutex.Unlock()

	log.Printf("🔕 Sorter #%d: Deteniendo %d suscripciones OPC UA...", s.ID, len(s.cancelSubscriptions))

	for _, cancelFunc := range s.cancelSubscriptions {
		cancelFunc()
	}

	s.cancelSubscriptions = nil
}

// StartFlowStatistics inicia el cálculo periódico de estadísticas de flujo
func (s *Sorter) StartFlowStatistics(calculationInterval, windowDuration time.Duration) {
	log.Printf("📊 Sorter #%d: Iniciando sistema de estadísticas de flujo", s.ID)
	log.Printf("   ⏱️  Sorter #%d: Intervalo de cálculo: %v", s.ID, calculationInterval)
	log.Printf("   🪟  Sorter #%d: Ventana de tiempo: %v", s.ID, windowDuration)

	ticker := time.NewTicker(calculationInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			log.Printf("🛑 Sorter #%d: Deteniendo sistema de estadísticas de flujo", s.ID)
			return

		case <-ticker.C:
			// Limpiar registros fuera de la ventana
			s.limpiarVentana(windowDuration)

			// Calcular estadísticas
			stats := s.calcularFlowStatistics(windowDuration)

			// Actualizar snapshot de porcentajes
			s.actualizarLastFlowStats(stats)

			// Publicar al canal de flow stats
			s.publicarFlowStatistics(stats)

			// 🔔 IMPORTANTE: Trigger UpdateSKUs para que WebSocket envíe sku_assigned actualizado
			s.UpdateSKUs(s.assignedSKUs)
		}
	}
}

// 🆕 Procesar eventos de lectura de Cognex
func (s *Sorter) procesarEventosCognex() {
	log.Println("👂 Sorter: Escuchando eventos de Cognex...")

	for {
		select {
		case <-s.ctx.Done():
			log.Println("🛑 Sorter: Deteniendo procesamiento de eventos")
			return

		case evento, ok := <-s.Cognex.EventChan:
			if !ok {
				log.Println("⚠️  Sorter: Canal de eventos cerrado")
				return
			}

			// Procesar el evento según su tipo
			tipoLectura := evento.GetTipo()

			if evento.Exitoso {
				s.LecturasExitosas++

				// 📊 Registrar lectura exitosa para estadísticas de flujo
				s.registrarLectura(evento.SKU)

				salida := s.determinarSalida(evento.SKU, evento.Calibre)
				log.Printf("✅ Sorter #%d: Lectura #%d | SKU: %s | Salida: %s (ID: %d) | Razón: sort por SKU",
					s.ID, s.LecturasExitosas, evento.SKU, salida.Salida_Sorter, salida.ID) // � Enviar señal al PLC para activar la salida
				if s.plcManager != nil && salida.SealerPhysicalID > 0 {

					ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
					if err := s.plcManager.AssignLaneToBox(ctx, s.ID, int16(salida.SealerPhysicalID)); err != nil {
						log.Printf("⚠️  [Sorter #%d] Error al enviar señal PLC para salida %d (PhysicalID=%d): %v",
							s.ID, salida.ID, salida.SealerPhysicalID, err)
					} else {
						log.Printf("📤 [Sorter #%d] Señal PLC enviada → Salida %d (PhysicalID=%d)",
							s.ID, salida.ID, salida.SealerPhysicalID)
					}
					cancel()
				}

				// �📤 Enviar evento WebSocket de lectura procesada
				s.PublishLecturaEvent(evento, &salida, true)

				// 💾 Registrar en base de datos la salida de la caja
				if err := s.RegistrarSalidaCaja(evento.Correlativo, &salida); err != nil {
					log.Printf("⚠️  Sorter #%d: Error al registrar salida de caja %s: %v", s.ID, evento.Correlativo, err)
				}
			} else {
				s.LecturasFallidas++
				var salida shared.Salida
				razon := ""
				switch tipoLectura {
				case models.LecturaNoRead:
					salidaPtr := s.GetDiscardSalida()
					if salidaPtr != nil {
						salida = *salidaPtr
					}
					razon = "sort por descarte (NO_READ)"
				case models.LecturaFormato, models.LecturaSKU:
					salidaPtr := s.GetDiscardSalida()
					if salidaPtr != nil {
						salida = *salidaPtr
					}
					razon = "sort por descarte (formato/SKU inválido)"
				case models.LecturaDB:
					razon = "error de base de datos"
				}
				log.Printf("❌ Sorter #%d: Fallo #%d | SKU: %s | Salida: %s (ID: %d) | Razón: %s | %s",
					s.ID, s.LecturasFallidas, evento.SKU, salida.Salida_Sorter, salida.ID, razon, evento.String()) // � Enviar señal al PLC para activar la salida de descarte
				if s.plcManager != nil && salida.SealerPhysicalID > 0 {
					ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
					if err := s.plcManager.AssignLaneToBox(ctx, s.ID, int16(salida.SealerPhysicalID)); err != nil {
						log.Printf("⚠️  [Sorter #%d] Error al enviar señal PLC para descarte (PhysicalID=%d): %v",
							s.ID, salida.SealerPhysicalID, err)
					} else {
						log.Printf("📤 [Sorter #%d] Señal PLC enviada a DESCARTE → Salida %d (PhysicalID=%d)",
							s.ID, salida.ID, salida.SealerPhysicalID)
					}
					cancel()
				}

				// �📤 Enviar evento WebSocket de lectura fallida
				s.PublishLecturaEvent(evento, &salida, false)

				// 💾 Registrar en base de datos la salida de la caja (también para fallidas)
				if err := s.RegistrarSalidaCaja(evento.Correlativo, &salida); err != nil {
					log.Printf("⚠️  Sorter #%d: Error al registrar salida de caja fallida %s: %v", s.ID, evento.Correlativo, err)
				}
			}

			// Mostrar estadísticas cada 10 lecturas
			total := s.LecturasExitosas + s.LecturasFallidas
			if total%10 == 0 && total > 0 {
				tasaExito := float64(s.LecturasExitosas) / float64(total) * 100
				log.Printf("📊 Sorter #%d: Stats: Total=%d | Exitosas=%d | Fallidas=%d | Tasa=%.1f%%",
					total, s.LecturasExitosas, s.LecturasFallidas, tasaExito)
			}
		}
	}
}

// Determinar a qué salida debe ir la caja según el SKU/Calibre
func (s *Sorter) determinarSalida(sku, calibre string) shared.Salida {
	// Si el SKU es REJECT (id=0 o texto 'REJECT'), buscar salida manual o de descarte
	if sku == "REJECT" || sku == "0" {
		for _, salida := range s.Salidas {
			if salida.Tipo == "manual" && salida.IsAvailable() {
				return salida
			}
		}
		// Si no hay salida manual disponible, buscar cualquier salida disponible
		for _, salida := range s.Salidas {
			if salida.IsAvailable() {
				log.Printf("⚠️ Sorter #%d: No hay salida manual disponible, usando salida %d", s.ID, salida.ID)
				return salida
			}
		}
		// Último recurso: usar la última salida (ignorar disponibilidad)
		if len(s.Salidas) > 0 {
			log.Printf("🚨 Sorter #%d: ADVERTENCIA - Todas las salidas bloqueadas/en falla, usando última salida", s.ID)
			return s.Salidas[len(s.Salidas)-1]
		}
	}

	// Verificar si hay distribución por lotes para esta SKU
	s.batchMutex.Lock()
	bd, hasBatchDistribution := s.batchCounters[sku]
	if hasBatchDistribution && len(bd.Salidas) > 1 {
		// Obtener próxima salida según patrón de lotes
		salidaID := bd.NextSalida()
		s.batchMutex.Unlock()

		// Buscar objeto Salida por ID y verificar disponibilidad
		for _, salida := range s.Salidas {
			if salida.ID == salidaID {
				if salida.IsAvailable() {
					log.Printf("🔄 Sorter #%d: SKU '%s' → Salida %d '%s' (lote %d/%d)",
						s.ID, sku, salida.ID, salida.Salida_Sorter,
						bd.CurrentCount, bd.BatchSizes[salidaID])
					return salida
				} else {
					estado := salida.GetEstado()
					bloqueo := salida.GetBloqueo()
					log.Printf("⚠️ Sorter #%d: Salida %d no disponible (Estado=%d, Bloqueo=%t), buscando alternativa...",
						s.ID, salida.ID, estado, bloqueo)
					// Continuar buscando alternativas
					break
				}
			}
		}
	} else {
		s.batchMutex.Unlock()
	}

	// Buscar la salida que tenga configurado ese SKU (fallback sin distribución)
	// 🆕 FILTRAR salidas que NO estén disponibles (ESTADO==2 o BLOQUEO==true)
	for _, salida := range s.Salidas {
		for _, skuConfig := range salida.SKUs_Actuales {
			if skuConfig.SKU == sku {
				if salida.IsAvailable() {
					return salida
				} else {
					estado := salida.GetEstado()
					bloqueo := salida.GetBloqueo()
					log.Printf("⚠️ Sorter #%d: Salida %d configurada para SKU '%s' pero NO disponible (Estado=%d, Bloqueo=%t)",
						s.ID, salida.ID, sku, estado, bloqueo)
				}
			}
		}
	}

	// Si no hay salida configurada y disponible, enviar a descarte
	for _, salida := range s.Salidas {
		if salida.Tipo == "manual" && salida.IsAvailable() {
			log.Printf("⚠️ Sorter #%d: SKU '%s' sin salida disponible configurada, enviando a descarte (salida %d)",
				s.ID, sku, salida.ID)
			return salida
		}
	}

	// Último recurso: cualquier salida disponible
	for _, salida := range s.Salidas {
		if salida.IsAvailable() {
			log.Printf("🚨 Sorter #%d: SKU '%s' sin salida manual disponible, usando salida %d",
				s.ID, sku, salida.ID)
			return salida
		}
	}

	// Si TODAS las salidas están bloqueadas/en falla, usar la última
	if len(s.Salidas) > 0 {
		log.Printf("🚨 Sorter #%d: ADVERTENCIA CRÍTICA - Todas las salidas bloqueadas/en falla, usando última salida", s.ID)
		return s.Salidas[len(s.Salidas)-1]
	}

	// Si no hay salidas configuradas, devolver un valor vacío
	return shared.Salida{}
}

func (s *Sorter) Stop() error {
	log.Printf("🛑 Deteniendo Sorter #%d", s.ID)

	// Detener suscripciones OPC UA primero
	s.stopPLCSubscriptions()

	// Cancelar contexto
	s.cancel()

	if s.Cognex != nil {
		return s.Cognex.Stop()
	}

	return nil
}

// UpdateSKUs actualiza la lista de SKUs asignados a este sorter y publica al canal
func (s *Sorter) UpdateSKUs(skus []models.SKUAssignable) {
	s.assignedSKUs = skus

	// Publicar al canal (no bloqueante)
	select {
	case s.skuChannel <- skus:
		log.Printf("📦 Sorter #%d: SKUs actualizados (%d SKUs)", s.ID, len(skus))
	default:
		log.Printf("⚠️  Sorter #%d: Canal lleno, SKUs no publicados", s.ID)
	}
}

// GetID retorna el ID del sorter (implementa SorterInterface)
func (s *Sorter) GetID() int {
	return s.ID
}

// GetFlowStatsChannel retorna el canal read-only de estadísticas de flujo (implementa SorterInterface)
func (s *Sorter) GetFlowStatsChannel() <-chan models.FlowStatistics {
	return s.flowStatsChannel
}

// registrarLectura registra una lectura exitosa en la ventana deslizante
func (s *Sorter) registrarLectura(sku string) {
	s.flowMutex.Lock()
	defer s.flowMutex.Unlock()

	s.lecturaRecords = append(s.lecturaRecords, models.LecturaRecord{
		SKU:       sku,
		Timestamp: time.Now(),
	})
}

// limpiarVentana elimina lecturas fuera de la ventana de tiempo
func (s *Sorter) limpiarVentana(windowDuration time.Duration) {
	s.flowMutex.Lock()
	defer s.flowMutex.Unlock()

	now := time.Now()
	cutoff := now.Add(-windowDuration)

	// Encontrar índice del primer registro dentro de la ventana
	validIdx := 0
	for i, record := range s.lecturaRecords {
		if record.Timestamp.After(cutoff) {
			validIdx = i
			break
		}
	}

	// Eliminar registros antiguos
	if validIdx > 0 {
		s.lecturaRecords = s.lecturaRecords[validIdx:]
	}
}

// calcularFlowStatistics calcula las estadísticas de flujo para la ventana actual
func (s *Sorter) calcularFlowStatistics(windowDuration time.Duration) models.FlowStatistics {
	s.flowMutex.RLock()
	defer s.flowMutex.RUnlock()

	// Contar lecturas por SKU
	skuCounts := make(map[string]int)
	totalLecturas := 0

	now := time.Now()
	cutoff := now.Add(-windowDuration)

	for _, record := range s.lecturaRecords {
		if record.Timestamp.After(cutoff) {
			skuCounts[record.SKU]++
			totalLecturas++
		}
	}

	// Convertir a slice de estadísticas y actualizar lastFlowStats
	stats := make([]models.SKUFlowStat, 0, len(skuCounts))
	newFlowStats := make(map[string]float64, len(skuCounts))

	for sku, count := range skuCounts {
		porcentaje := 0.0
		if totalLecturas > 0 {
			porcentajeRaw := float64(count) / float64(totalLecturas) * 100.0
			porcentaje = float64(int(porcentajeRaw + 0.5)) // Redondear a entero
		}

		stats = append(stats, models.SKUFlowStat{
			SKU:        sku,
			Lecturas:   count,
			Porcentaje: porcentaje,
		})

		newFlowStats[sku] = porcentaje
	}

	return models.FlowStatistics{
		SorterID:      s.ID,
		Timestamp:     now,
		Stats:         stats,
		TotalLecturas: totalLecturas,
		WindowSeconds: int(windowDuration.Seconds()),
	}
}

// actualizarLastFlowStats actualiza el snapshot de porcentajes (thread-safe)
func (s *Sorter) actualizarLastFlowStats(stats models.FlowStatistics) {
	s.flowMutex.Lock()
	defer s.flowMutex.Unlock()

	// Limpiar mapa anterior
	s.lastFlowStats = make(map[string]float64, len(stats.Stats))

	// Llenar con nuevos porcentajes
	for _, stat := range stats.Stats {
		s.lastFlowStats[stat.SKU] = stat.Porcentaje
	}
}

// publicarFlowStatistics publica las estadísticas al canal
func (s *Sorter) publicarFlowStatistics(stats models.FlowStatistics) {
	select {
	case s.flowStatsChannel <- stats:
		log.Printf("📊 Sorter #%d: Flow stats publicadas (%d SKUs diferentes, %d lecturas totales)",
			s.ID, len(stats.Stats), stats.TotalLecturas)
	default:
		log.Printf("⚠️  Sorter #%d: Canal de flow stats lleno", s.ID)
	}
}

// GetSKUChannel retorna el canal de SKUs de este sorter (read-only)
func (s *Sorter) GetSKUChannel() <-chan []models.SKUAssignable {
	return s.skuChannel
}

// GetCurrentSKUs retorna la lista actual de SKUs asignados con porcentajes actualizados (implementa SorterInterface)
func (s *Sorter) GetCurrentSKUs() []models.SKUAssignable {
	s.flowMutex.RLock()
	defer s.flowMutex.RUnlock()

	// Crear copia de SKUs con porcentajes actualizados
	skusWithPercentage := make([]models.SKUAssignable, len(s.assignedSKUs))
	copy(skusWithPercentage, s.assignedSKUs)

	// Actualizar percentage de cada SKU (redondeado a entero)
	for i := range skusWithPercentage {
		if percentage, exists := s.lastFlowStats[skusWithPercentage[i].SKU]; exists {
			skusWithPercentage[i].Percentage = float64(int(percentage + 0.5)) // Redondear a entero
		} else {
			skusWithPercentage[i].Percentage = 0.0
		}
	}

	return skusWithPercentage
}

// GetSKUFlowPercentage retorna el porcentaje de flujo de un SKU (thread-safe)
func (s *Sorter) GetSKUFlowPercentage(skuName string) float64 {
	s.flowMutex.RLock()
	defer s.flowMutex.RUnlock()

	if percentage, exists := s.lastFlowStats[skuName]; exists {
		return percentage
	}
	return 0.0
}

// AssignSKUToSalida asigna una SKU a una salida específica
// Retorna los datos del SKU asignado (calibre, variedad, embalaje) para insertar en BD
// Retorna error si:
// - La salida no existe
// - La SKU es REJECT (ID=0) y la salida es automática
// - La SKU no existe en assignedSKUs
func (s *Sorter) AssignSKUToSalida(skuID uint32, salidaID int) (calibre, variedad, embalaje string, err error) {
	// Buscar la salida
	var targetSalida *shared.Salida
	for i := range s.Salidas {
		if s.Salidas[i].ID == salidaID {
			targetSalida = &s.Salidas[i]
			break
		}
	}

	if targetSalida == nil {
		return "", "", "", fmt.Errorf("salida con ID %d no encontrada en sorter #%d", salidaID, s.ID)
	}

	// VALIDACIÓN CRÍTICA: REJECT no puede ir a salida automática
	if skuID == 0 && targetSalida.Tipo == "automatico" {
		return "", "", "", fmt.Errorf("no se puede asignar SKU REJECT (ID=0) a salida automática '%s' (ID=%d)",
			targetSalida.Salida_Sorter, salidaID)
	}

	// Buscar la SKU en assignedSKUs
	var targetSKU *models.SKUAssignable
	for i := range s.assignedSKUs {
		if uint32(s.assignedSKUs[i].ID) == skuID {
			targetSKU = &s.assignedSKUs[i]
			break
		}
	}

	if targetSKU == nil {
		return "", "", "", fmt.Errorf("SKU con ID %d no encontrada en las SKUs disponibles del sorter #%d", skuID, s.ID)
	}

	// Convertir SKUAssignable a SKU para agregar a la salida
	sku := models.SKU{
		SKU:      targetSKU.SKU,
		Variedad: targetSKU.Variedad,
		Calibre:  targetSKU.Calibre,
		Embalaje: targetSKU.Embalaje,
		Estado:   true,
	}

	// Agregar SKU a la salida
	targetSalida.SKUs_Actuales = append(targetSalida.SKUs_Actuales, sku)

	// Marcar SKU como asignada
	targetSKU.IsAssigned = true

	// Actualizar BatchDistributor para esta SKU
	s.batchMutex.Lock()
	s.updateBatchDistributor(targetSKU.SKU)
	s.batchMutex.Unlock()

	log.Printf("✅ Sorter #%d: SKU '%s' (ID=%d) asignada a salida '%s' (ID=%d, tipo=%s)",
		s.ID, targetSKU.SKU, skuID, targetSalida.Salida_Sorter, salidaID, targetSalida.Tipo)

	// Publicar estado actualizado al canal para WebSocket
	s.UpdateSKUs(s.assignedSKUs)

	return targetSKU.Calibre, targetSKU.Variedad, targetSKU.Embalaje, nil
}

// FindSalidaForSKU busca en qué salida está asignada una SKU específica
// Retorna el ID de la salida, o -1 si no está asignada (debe ir a descarte)
func (s *Sorter) FindSalidaForSKU(skuText string) int {
	for _, salida := range s.Salidas {
		for _, sku := range salida.SKUs_Actuales {
			if sku.SKU == skuText {
				return salida.ID
			}
		}
	}
	return -1 // No encontrada, debe ir a descarte
}

// GetDiscardSalida retorna una salida de descarte (tipo manual)
// Si no hay ninguna configurada, asigna una aleatoriamente
func (s *Sorter) GetDiscardSalida() *shared.Salida {
	// Buscar salida que tenga SKU REJECT
	for i := range s.Salidas {
		for _, sku := range s.Salidas[i].SKUs_Actuales {
			if sku.SKU == "REJECT" {
				return &s.Salidas[i]
			}
		}
	}

	// Si no hay salida con REJECT, buscar primera salida manual
	for i := range s.Salidas {
		if s.Salidas[i].Tipo == "manual" {
			log.Printf("⚠️  Sorter #%d: Asignando salida manual '%s' (ID=%d) como descarte automático",
				s.ID, s.Salidas[i].Salida_Sorter, s.Salidas[i].ID)
			return &s.Salidas[i]
		}
	}

	// Último recurso: usar la última salida (no debería pasar)
	if len(s.Salidas) > 0 {
		log.Printf("🚨 Sorter #%d: ADVERTENCIA - No hay salida manual, usando última salida como descarte", s.ID)
		return &s.Salidas[len(s.Salidas)-1]
	}

	return nil
}

// StartSKUPublisher inicia un publicador periódico de SKUs (cada N segundos)
func (s *Sorter) StartSKUPublisher(intervalSeconds int, skuSource func() []models.SKUAssignable) {
	go func() {
		ticker := time.NewTicker(time.Duration(intervalSeconds) * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-s.ctx.Done():
				log.Printf("⏸️  Sorter #%d: Publicador de SKUs detenido", s.ID)
				return
			case <-ticker.C:
				skus := skuSource()
				s.UpdateSKUs(skus)
			}
		}
	}()
	log.Printf("⏰ Sorter #%d: Publicador de SKUs iniciado (cada %ds)", s.ID, intervalSeconds)
}

// RemoveSKUFromSalida elimina una SKU específica de una salida
// Retorna los datos del SKU eliminado (calibre, variedad, embalaje) para eliminar de BD
// Retorna error si:
// - La salida no existe
// - La SKU no está asignada a esa salida
func (s *Sorter) RemoveSKUFromSalida(skuID uint32, salidaID int) (calibre, variedad, embalaje string, err error) {
	// Buscar la salida
	var targetSalida *shared.Salida
	var salidaIndex int
	for i := range s.Salidas {
		if s.Salidas[i].ID == salidaID {
			targetSalida = &s.Salidas[i]
			salidaIndex = i
			break
		}
	}

	if targetSalida == nil {
		return "", "", "", fmt.Errorf("salida con ID %d no encontrada en sorter #%d", salidaID, s.ID)
	}

	// Buscar la SKU en la salida
	skuFound := false
	var removedSKU models.SKU
	newSKUs := make([]models.SKU, 0, len(targetSalida.SKUs_Actuales))

	for _, sku := range targetSalida.SKUs_Actuales {
		// Generar ID del SKU actual para comparar
		currentSKUID := uint32(sku.GetNumericID())
		if currentSKUID == skuID {
			skuFound = true
			removedSKU = sku
			continue // No agregar este SKU a la nueva lista
		}
		newSKUs = append(newSKUs, sku)
	}

	if !skuFound {
		return "", "", "", fmt.Errorf("SKU con ID %d no encontrada en salida %d del sorter #%d", skuID, salidaID, s.ID)
	}

	// Actualizar la lista de SKUs en la salida
	s.Salidas[salidaIndex].SKUs_Actuales = newSKUs

	// Marcar la SKU como no asignada en assignedSKUs
	for i := range s.assignedSKUs {
		if uint32(s.assignedSKUs[i].ID) == skuID {
			s.assignedSKUs[i].IsAssigned = false
			break
		}
	}

	log.Printf("🗑️  Sorter #%d: SKU '%s' (ID=%d) eliminada de salida '%s' (ID=%d)",
		s.ID, removedSKU.SKU, skuID, targetSalida.Salida_Sorter, salidaID)

	// Publicar estado actualizado al canal para WebSocket
	s.UpdateSKUs(s.assignedSKUs)

	return removedSKU.Calibre, removedSKU.Variedad, removedSKU.Embalaje, nil
}

// RemoveAllSKUsFromSalida elimina TODAS las SKUs de una salida específica
// Retorna un slice con todas las SKUs eliminadas
// Retorna error si la salida no existe
func (s *Sorter) RemoveAllSKUsFromSalida(salidaID int) ([]models.SKU, error) {
	// Buscar la salida
	var targetSalida *shared.Salida
	var salidaIndex int
	for i := range s.Salidas {
		if s.Salidas[i].ID == salidaID {
			targetSalida = &s.Salidas[i]
			salidaIndex = i
			break
		}
	}

	if targetSalida == nil {
		return nil, fmt.Errorf("salida con ID %d no encontrada en sorter #%d", salidaID, s.ID)
	}

	// Guardar copia de las SKUs eliminadas para retornar
	removedSKUs := make([]models.SKU, len(targetSalida.SKUs_Actuales))
	copy(removedSKUs, targetSalida.SKUs_Actuales)

	// Marcar todas las SKUs como no asignadas en assignedSKUs
	for _, sku := range removedSKUs {
		skuID := uint32(sku.GetNumericID())
		for i := range s.assignedSKUs {
			if uint32(s.assignedSKUs[i].ID) == skuID {
				s.assignedSKUs[i].IsAssigned = false
			}
		}
	}

	// Limpiar la lista de SKUs de la salida
	s.Salidas[salidaIndex].SKUs_Actuales = []models.SKU{}

	log.Printf("🧹 Sorter #%d: Eliminadas %d SKUs de salida '%s' (ID=%d)",
		s.ID, len(removedSKUs), targetSalida.Salida_Sorter, salidaID)

	// Publicar estado actualizado al canal para WebSocket
	s.UpdateSKUs(s.assignedSKUs)

	return removedSKUs, nil
}

// PublishLecturaEvent publica un evento de lectura individual procesada al WebSocket
func (s *Sorter) PublishLecturaEvent(evento models.LecturaEvent, salida *shared.Salida, exitoso bool) {
	if s.wsHub == nil {
		return
	}

	// Construir el mensaje JSON
	message := map[string]interface{}{
		"type":        "lectura_procesada",
		"qr_code":     evento.Mensaje,
		"sku":         evento.SKU,
		"salida_id":   salida.ID,
		"salida_name": salida.Salida_Sorter,
		"exitoso":     exitoso,
		"timestamp":   evento.Timestamp.Format(time.RFC3339),
		"sorter_id":   s.ID,
		"correlativo": evento.Correlativo,
		"calibre":     evento.Calibre,
		"variedad":    evento.Variedad,
		"embalaje":    evento.Embalaje,
	}

	// Serializar a JSON
	jsonBytes, err := json.Marshal(message)
	if err != nil {
		log.Printf("❌ Error al serializar lectura_procesada: %v", err)
		return
	}

	// Publicar al room correspondiente
	roomName := fmt.Sprintf("assignment_%d", s.ID)
	s.wsHub.Broadcast <- &listeners.BroadcastMessage{
		RoomName: roomName,
		Message:  jsonBytes,
	}
}

// RegistrarSalidaCaja registra en la base de datos que una caja fue enviada a una salida física
func (s *Sorter) RegistrarSalidaCaja(correlativo string, salida *shared.Salida) error {
	if s.dbManager == nil {
		return fmt.Errorf("dbManager no inicializado")
	}

	// Type assertion para obtener el PostgresManager
	pgManager, ok := s.dbManager.(*db.PostgresManager)
	if !ok {
		return fmt.Errorf("dbManager no es un PostgresManager válido")
	}

	// Registrar en la tabla salida_caja
	ctx := context.Background()
	err := pgManager.InsertSalidaCaja(ctx, correlativo, salida.ID, salida.SealerPhysicalID)
	if err != nil {
		return fmt.Errorf("error al registrar salida de caja %s: %w", correlativo, err)
	}

	return nil
}
