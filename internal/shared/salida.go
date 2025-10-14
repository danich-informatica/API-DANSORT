package shared

import (
	"API-GREENEX/internal/models"
	"sync"
)

type Salida struct {
	ID               int          `json:"id"`
	SealerPhysicalID int          `json:"sealer_physical_id"`
	CognexID         int          `json:"cognex_id"` // ID de Cognex asignado a esta salida (0 = sin cognex)
	Salida_Sorter    string       `json:"salida_sorter"`
	Tipo             string       `json:"tipo"`       // "automatico" o "manual"
	BatchSize        int          `json:"batch_size"` // Tamaño de lote para distribución
	SKUs_Actuales    []models.SKU `json:"skus_actuales"`

	// Valores en tiempo real desde PLC (protegidos por mutex)
	estadoMutex sync.RWMutex
	Estado      int16 `json:"estado"` // 0=apagado, 1=andando, 2=falla (actualizado vía OPC UA)

	bloqueoMutex sync.RWMutex
	Bloqueo      bool `json:"bloqueo"` // true=bloqueada, false=disponible (actualizado vía OPC UA)

	Ingreso     bool   `json:"ingreso"`
	IsEnabled   bool   `json:"is_enabled"`
	EstadoNode  string `json:"estado_node"`  // Nodo OPC UA para estado
	BloqueoNode string `json:"bloqueo_node"` // Nodo OPC UA para bloqueo
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

func GetNewSalida(ID int, salida_sorter string, tipo string, batchSize int) Salida {
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
		BatchSize:        batchSize,
		SKUs_Actuales:    []models.SKU{},
		Estado:           0,
		Ingreso:          false,
		IsEnabled:        true,
	}
}

// GetNewSalidaWithPhysicalID crea una nueva salida con ID físico específico
func GetNewSalidaWithPhysicalID(ID int, physicalID int, salida_sorter string, tipo string, batchSize int) Salida {
	salida := GetNewSalida(ID, salida_sorter, tipo, batchSize)
	salida.SealerPhysicalID = physicalID
	return salida
}

// GetNewSalidaComplete crea una nueva salida con todos los parámetros
func GetNewSalidaComplete(ID int, physicalID int, cognexID int, salida_sorter string, tipo string, batchSize int) Salida {
	salida := GetNewSalida(ID, salida_sorter, tipo, batchSize)
	salida.SealerPhysicalID = physicalID
	salida.CognexID = cognexID
	return salida
}
