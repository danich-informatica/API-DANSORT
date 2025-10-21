package monitoring

import (
	"context"
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"API-GREENEX/internal/models"
)

// DeviceMonitor gestiona el monitoreo de dispositivos con heartbeat
type DeviceMonitor struct {
	ctx               context.Context
	cancel            context.CancelFunc
	devices           map[int]*models.DeviceStatus // key: device ID
	devicesMu         sync.RWMutex
	heartbeatInterval time.Duration
	timeoutDuration   time.Duration
}

// NewDeviceMonitor crea una nueva instancia del monitor
func NewDeviceMonitor(heartbeatInterval, timeout time.Duration) *DeviceMonitor {
	ctx, cancel := context.WithCancel(context.Background())

	return &DeviceMonitor{
		ctx:               ctx,
		cancel:            cancel,
		devices:           make(map[int]*models.DeviceStatus),
		heartbeatInterval: heartbeatInterval,
		timeoutDuration:   timeout,
	}
}

// RegisterDevice registra un nuevo dispositivo para monitoreo
func (m *DeviceMonitor) RegisterDevice(device *models.DeviceStatus) {
	m.devicesMu.Lock()
	defer m.devicesMu.Unlock()

	device.LastCheck = time.Now()
	device.IsDisconnected = false
	m.devices[device.ID] = device

	log.Printf("📡 Dispositivo registrado para monitoreo: %s (%s:%d) en sección %d",
		device.DeviceName, device.IP, device.Port, device.SectionID)
}

// Start inicia el monitoreo continuo con heartbeat
func (m *DeviceMonitor) Start() {
	log.Printf("🔄 Iniciando monitoreo de dispositivos (intervalo: %v, timeout: %v)",
		m.heartbeatInterval, m.timeoutDuration)

	ticker := time.NewTicker(m.heartbeatInterval)
	defer ticker.Stop()

	// Primer chequeo inmediato
	m.checkAllDevices()

	for {
		select {
		case <-m.ctx.Done():
			log.Println("🛑 Monitoreo de dispositivos detenido")
			return
		case <-ticker.C:
			m.checkAllDevices()
		}
	}
}

// Stop detiene el monitoreo
func (m *DeviceMonitor) Stop() {
	m.cancel()
}

// checkAllDevices verifica el estado de todos los dispositivos
func (m *DeviceMonitor) checkAllDevices() {
	m.devicesMu.RLock()
	devicesCopy := make([]*models.DeviceStatus, 0, len(m.devices))
	for _, device := range m.devices {
		devicesCopy = append(devicesCopy, device)
	}
	m.devicesMu.RUnlock()

	// Chequear dispositivos en paralelo
	var wg sync.WaitGroup
	for _, device := range devicesCopy {
		wg.Add(1)
		go func(dev *models.DeviceStatus) {
			defer wg.Done()
			m.checkDevice(dev)
		}(device)
	}
	wg.Wait()
}

// checkDevice verifica el estado de un dispositivo usando TCP dial
func (m *DeviceMonitor) checkDevice(device *models.DeviceStatus) {
	address := net.JoinHostPort(device.IP, fmt.Sprintf("%d", device.Port))
	start := time.Now()

	conn, err := net.DialTimeout("tcp", address, m.timeoutDuration)
	elapsed := time.Since(start).Milliseconds()

	m.devicesMu.Lock()
	defer m.devicesMu.Unlock()

	device.LastCheck = time.Now()
	device.ResponseTimeMs = elapsed

	if err != nil {
		// Dispositivo desconectado
		if !device.IsDisconnected {
			now := time.Now()
			device.LastDisconnection = &now
			device.IsDisconnected = true
			log.Printf("❌ Dispositivo desconectado: %s (%s:%d) - Error: %v",
				device.DeviceName, device.IP, device.Port, err)
		}
	} else {
		conn.Close()
		// Dispositivo conectado
		if device.IsDisconnected {
			log.Printf("✅ Dispositivo reconectado: %s (%s:%d) - Tiempo: %dms",
				device.DeviceName, device.IP, device.Port, elapsed)
			device.IsDisconnected = false
		}
	}
}

// GetSectionStatuses retorna el estado de todas las secciones
func (m *DeviceMonitor) GetSectionStatuses() []models.SectionStatus {
	m.devicesMu.RLock()
	defer m.devicesMu.RUnlock()

	// Agrupar dispositivos por sección
	sectionMap := make(map[int]*models.SectionStatus)

	for _, device := range m.devices {
		section, exists := sectionMap[device.SectionID]
		if !exists {
			section = &models.SectionStatus{
				SectionID:     device.SectionID,
				SectionName:   fmt.Sprintf("Sorter #%d", device.SectionID),
				HasConnection: true,
			}
			sectionMap[device.SectionID] = section
		}

		section.DeviceCount++

		if device.IsDisconnected {
			section.HasConnection = false
			section.DisconnectedCount++
		}
	}

	// Convertir a slice
	result := make([]models.SectionStatus, 0, len(sectionMap))
	for _, section := range sectionMap {
		result = append(result, *section)
	}

	return result
}

// GetDevicesBySection retorna todos los dispositivos de una sección
func (m *DeviceMonitor) GetDevicesBySection(sectionID int) []models.DeviceStatus {
	m.devicesMu.RLock()
	defer m.devicesMu.RUnlock()

	result := make([]models.DeviceStatus, 0)

	for _, device := range m.devices {
		if device.SectionID == sectionID {
			result = append(result, *device)
		}
	}

	return result
}

// GetAllDevices retorna todos los dispositivos monitoreados
func (m *DeviceMonitor) GetAllDevices() []models.DeviceStatus {
	m.devicesMu.RLock()
	defer m.devicesMu.RUnlock()

	result := make([]models.DeviceStatus, 0, len(m.devices))
	for _, device := range m.devices {
		result = append(result, *device)
	}

	return result
}
