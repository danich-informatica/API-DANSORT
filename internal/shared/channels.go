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

	// Mutex para acceso concurrente seguro
	mu sync.RWMutex

	// Flags para saber si los canales ya fueron inicializados
	cognexInitialized bool
	skuInitialized    bool
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
			cognexInitialized: false,
			skuInitialized:    false,
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
		cm.skuInitialized = true
	}
}
