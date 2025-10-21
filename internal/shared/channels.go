package shared

import (
	"sync"

	"API-GREENEX/internal/models"
)

// ChannelManager gestiona todos los canales compartidos de la aplicación usando patrón Singleton
type ChannelManager struct {
	// Canal para eventos de lectura de Cognex
	cognexQRChannel chan models.LecturaEvent

	// Canal para streaming de SKUs
	skuStreamChannel models.SKUChannel

	// Mapa de canales de SKUs assignables por sorter_id
	sorterSKUChannels map[string]chan []models.SKUAssignable

	// Mapa de canales de estadísticas de flujo por sorter_id
	sorterFlowStatsChannels map[string]chan models.FlowStatistics

	// Mutex para acceso concurrente seguro
	mu sync.RWMutex

	// Flags para saber si los canales ya fueron inicializados
	cognexInitialized         bool
	skuInitialized            bool
	sorterChannelsInitialized bool
	flowStatsInitialized      bool
}

var (
	// Instancia única del ChannelManager (Singleton)
	instance *ChannelManager
	once     sync.Once
)

// GetChannelManager retorna la instancia única del gestor de canales
// Si no existe, la crea automáticamente (thread-safe)
func GetChannelManager() *ChannelManager {
	once.Do(func() {
		instance = &ChannelManager{
			cognexInitialized:         false,
			skuInitialized:            false,
			sorterChannelsInitialized: false,
			flowStatsInitialized:      false,
			sorterSKUChannels:         make(map[string]chan []models.SKUAssignable),
			sorterFlowStatsChannels:   make(map[string]chan models.FlowStatistics),
		}
	})
	return instance
}

// GetCognexQRChannel retorna el canal de eventos de lectura de Cognex
// Si no existe, lo crea automáticamente con buffer de 100
func (cm *ChannelManager) GetCognexQRChannel() chan models.LecturaEvent {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if !cm.cognexInitialized {
		cm.cognexQRChannel = make(chan models.LecturaEvent, 100)
		cm.cognexInitialized = true
	}

	return cm.cognexQRChannel
}

// GetSKUStreamChannel retorna el canal de streaming de SKUs
// Si no existe, lo crea automáticamente con buffer de 1000
func (cm *ChannelManager) GetSKUStreamChannel() models.SKUChannel {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if !cm.skuInitialized {
		cm.skuStreamChannel = models.NewSKUChannel(1000)
		cm.skuInitialized = true
	}

	return cm.skuStreamChannel
}

// IsCognexChannelInitialized verifica si el canal de Cognex ya está inicializado
func (cm *ChannelManager) IsCognexChannelInitialized() bool {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.cognexInitialized
}

// IsSKUChannelInitialized verifica si el canal de SKU ya está inicializado
func (cm *ChannelManager) IsSKUChannelInitialized() bool {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.skuInitialized
}

// RegisterSorterSKUChannel registra un nuevo canal de SKUs para un sorter específico
// Si el canal ya existe, lo retorna sin crear uno nuevo
func (cm *ChannelManager) RegisterSorterSKUChannel(sorterID string, bufferSize int) chan []models.SKUAssignable {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	// Si ya existe, retornar el canal existente
	if ch, exists := cm.sorterSKUChannels[sorterID]; exists {
		return ch
	}

	// Crear nuevo canal con buffer especificado
	ch := make(chan []models.SKUAssignable, bufferSize)
	cm.sorterSKUChannels[sorterID] = ch
	cm.sorterChannelsInitialized = true

	return ch
}

// GetSorterSKUChannel obtiene el canal de SKUs de un sorter específico
// Retorna nil si el sorter no tiene canal registrado
func (cm *ChannelManager) GetSorterSKUChannel(sorterID string) chan []models.SKUAssignable {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	return cm.sorterSKUChannels[sorterID]
}

// IsSorterRegistered verifica si un sorter tiene canal registrado
func (cm *ChannelManager) IsSorterRegistered(sorterID string) bool {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	_, exists := cm.sorterSKUChannels[sorterID]
	return exists
}

// GetAllSorterIDs retorna la lista de IDs de sorters registrados
func (cm *ChannelManager) GetAllSorterIDs() []string {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	ids := make([]string, 0, len(cm.sorterSKUChannels))
	for id := range cm.sorterSKUChannels {
		ids = append(ids, id)
	}
	return ids
}

// RegisterSorterFlowStatsChannel registra un nuevo canal de estadísticas de flujo para un sorter
func (cm *ChannelManager) RegisterSorterFlowStatsChannel(sorterID string, bufferSize int) chan models.FlowStatistics {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	// Si ya existe, retornar el canal existente
	if ch, exists := cm.sorterFlowStatsChannels[sorterID]; exists {
		return ch
	}

	// Crear nuevo canal con buffer especificado
	ch := make(chan models.FlowStatistics, bufferSize)
	cm.sorterFlowStatsChannels[sorterID] = ch
	cm.flowStatsInitialized = true

	return ch
}

// GetSorterFlowStatsChannel obtiene el canal de estadísticas de flujo de un sorter
func (cm *ChannelManager) GetSorterFlowStatsChannel(sorterID string) chan models.FlowStatistics {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	return cm.sorterFlowStatsChannels[sorterID]
}

// CloseAll cierra todos los canales (llamar al finalizar la aplicación)
func (cm *ChannelManager) CloseAll() {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if cm.cognexInitialized && cm.cognexQRChannel != nil {
		close(cm.cognexQRChannel)
		cm.cognexInitialized = false
	}

	if cm.skuInitialized && cm.skuStreamChannel != nil {
		close(cm.skuStreamChannel)
		cm.skuInitialized = false
	}

	// Cerrar todos los canales de sorters
	if cm.sorterChannelsInitialized {
		for id, ch := range cm.sorterSKUChannels {
			if ch != nil {
				close(ch)
				delete(cm.sorterSKUChannels, id)
			}
		}
		cm.sorterChannelsInitialized = false
	}

	// Cerrar todos los canales de estadísticas de flujo
	if cm.flowStatsInitialized {
		for id, ch := range cm.sorterFlowStatsChannels {
			if ch != nil {
				close(ch)
				delete(cm.sorterFlowStatsChannels, id)
			}
		}
		cm.flowStatsInitialized = false
	}
}
