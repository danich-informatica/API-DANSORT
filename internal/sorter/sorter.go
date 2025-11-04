package sorter

import (
	"API-GREENEX/internal/communication/plc"
	"API-GREENEX/internal/listeners"
	"API-GREENEX/internal/models"
	"API-GREENEX/internal/shared"
	"context"
	"fmt"
	"log"
	"sync"
)

// Sorter representa un sistema sorter con sus salidas y configuración
type Sorter struct {
	ID            int                               `json:"id"`
	Ubicacion     string                            `json:"ubicacion"`
	PLCInputNode  string                            `json:"plc_input_node"`
	PLCOutputNode string                            `json:"plc_output_node"`
	Salidas       []shared.Salida                   `json:"salidas"`
	Cognex        *listeners.CognexListener         `json:"cognex"` // Cognex principal para QR/SKU
	CognexDevices map[int]*listeners.CognexListener `json:"-"`      // Múltiples cámaras DataMatrix (key=cognexID)
	ctx           context.Context
	cancel        context.CancelFunc

	// Configuración de paletizado automático
	PaletHost string `json:"palet_host"` // Host del servidor de paletizado
	PaletPort int    `json:"palet_port"` // Puerto del servidor de paletizado

	plcManager          *plc.Manager // Cliente PLC para método AssignLaneToBox
	plcMonitorManager   *plc.Manager // Cliente PLC para leer/escribir nodos de salidas (opcional)
	fxSyncManager       interface{}  // *db.FXSyncManager (interface para evitar import cycle)
	cancelSubscriptions []func()
	subscriptionMutex   sync.Mutex

	skuChannel       chan []models.SKUAssignable
	flowStatsChannel chan models.FlowStatistics
	assignedSKUs     []models.SKUAssignable

	lecturaRecords []models.LecturaRecord
	lastFlowStats  map[string]float64
	flowMutex      sync.RWMutex

	batchCounters map[string]*BatchDistributor
	batchMutex    sync.RWMutex

	wsHub     *listeners.WebSocketHub
	dbManager interface{}

	LecturasExitosas int
	LecturasFallidas int
}

// BatchDistributor maneja la distribución por lotes entre salidas
type BatchDistributor struct {
	Salidas      []int
	CurrentIndex int
	CurrentCount int
	BatchSizes   map[int]int
}

// NextSalida retorna el ID de la próxima salida según el patrón de lotes
func (bd *BatchDistributor) NextSalida() int {
	if len(bd.Salidas) == 0 {
		return -1
	}

	if len(bd.Salidas) == 1 {
		return bd.Salidas[0]
	}

	// Obtener salida actual
	salidaID := bd.Salidas[bd.CurrentIndex]
	batchSize := bd.BatchSizes[salidaID]

	// Incrementar contador
	bd.CurrentCount++

	// Si alcanzamos el batch size, avanzar al siguiente
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

// GetNewSorter crea una nueva instancia de Sorter
func GetNewSorter(ID int, ubicacion string, plcInputNode string, plcOutputNode string, paletHost string, paletPort int, salidas []shared.Salida, cognex *listeners.CognexListener, cognexDevices map[int]*listeners.CognexListener, wsHub *listeners.WebSocketHub, dbManager interface{}, plcManager *plc.Manager, plcMonitorManager *plc.Manager, fxSyncManager interface{}) *Sorter {
	ctx, cancel := context.WithCancel(context.Background())

	channelMgr := shared.GetChannelManager()
	sorterID := fmt.Sprintf("%d", ID)
	skuChannel := channelMgr.RegisterSorterSKUChannel(sorterID, 10)
	flowStatsChannel := channelMgr.RegisterSorterFlowStatsChannel(sorterID, 5)

	return &Sorter{
		ID:                  ID,
		Ubicacion:           ubicacion,
		PLCInputNode:        plcInputNode,
		PLCOutputNode:       plcOutputNode,
		PaletHost:           paletHost,
		PaletPort:           paletPort,
		Salidas:             salidas,
		Cognex:              cognex,
		CognexDevices:       cognexDevices, // Mapa de cámaras DataMatrix
		ctx:                 ctx,
		cancel:              cancel,
		plcManager:          plcManager,        // Cliente PLC para método
		plcMonitorManager:   plcMonitorManager, // Cliente PLC para monitoreo (puede ser nil si usa mismo endpoint)
		fxSyncManager:       fxSyncManager,
		cancelSubscriptions: make([]func(), 0),
		skuChannel:          skuChannel,
		flowStatsChannel:    flowStatsChannel,
		assignedSKUs:        make([]models.SKUAssignable, 0),
		lecturaRecords:      make([]models.LecturaRecord, 0, 1000),
		lastFlowStats:       make(map[string]float64),
		batchCounters:       make(map[string]*BatchDistributor),
		wsHub:               wsHub,
		dbManager:           dbManager,
	}
}

// Start inicia el sorter
func (s *Sorter) Start() error {
	log.Printf("🚀 Sorter #%d: Iniciando en %s", s.ID, s.Ubicacion)
	log.Printf("📍 Sorter #%d: Salidas configuradas: %d", s.ID, len(s.Salidas))

	for i := range s.Salidas {
		log.Printf("  ↳ Sorter #%d: Salida %d: %s", s.ID, s.Salidas[i].ID, s.Salidas[i].Salida_Sorter)
	}

	// Iniciar gorutinas para salidas automáticas
	for i := range s.Salidas {
		if s.Salidas[i].Tipo == "automatico" {
			log.Printf("  ↳ Sorter #%d: Iniciando gorutina para salida automática %d", s.ID, s.Salidas[i].ID)
			s.Salidas[i].Start()
		}
	}

	if s.Cognex != nil {
		err := s.Cognex.Start()
		if err != nil {
			return err
		}
	}

	// Iniciar procesamiento de eventos QR/SKU (canal original)
	go s.procesarEventosCognex()

	// Iniciar listeners de cámaras DataMatrix
	for cognexID, cognexListener := range s.CognexDevices {
		log.Printf("🎯 [Sorter #%d] Iniciando cámara DataMatrix Cognex #%d", s.ID, cognexID)
		if err := cognexListener.Start(); err != nil {
			log.Printf("❌ [Sorter #%d] Error iniciando Cognex #%d: %v", s.ID, cognexID, err)
			return err
		}
		// Iniciar procesamiento de eventos DataMatrix para cada cámara
		go s.procesarEventosDataMatrixCognex(cognexListener)
	}

	if s.plcManager != nil {
		s.startPLCSubscriptions()
	}

	log.Printf("✅ Sorter #%d: Iniciado y escuchando eventos (QR/SKU + %d cámaras DataMatrix)", s.ID, len(s.CognexDevices))

	return nil
}

// Stop detiene el sorter
func (s *Sorter) Stop() error {
	log.Printf("🛑 Deteniendo Sorter #%d", s.ID)

	s.stopPLCSubscriptions()
	s.cancel()

	if s.Cognex != nil {
		return s.Cognex.Stop()
	}

	return nil
}

// GetID retorna el ID del sorter
func (s *Sorter) GetID() int {
	return s.ID
}

// GetFlowStatsChannel retorna el canal de estadísticas de flujo
func (s *Sorter) GetFlowStatsChannel() <-chan models.FlowStatistics {
	return s.flowStatsChannel
}
