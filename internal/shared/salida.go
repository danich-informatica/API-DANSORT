package shared

import (
	"API-GREENEX/internal/models"
)

type Salida struct {
	ID               int          `json:"id"`
	SealerPhysicalID int          `json:"sealer_physical_id"`
	CognexID         int          `json:"cognex_id"` // ID de Cognex asignado a esta salida (0 = sin cognex)
	Salida_Sorter    string       `json:"salida_sorter"`
	Tipo             string       `json:"tipo"`       // "automatico" o "manual"
	BatchSize        int          `json:"batch_size"` // Tamaño de lote para distribución
	SKUs_Actuales    []models.SKU `json:"skus_actuales"`
	Estado           int          `json:"estado"`
	Ingreso          bool         `json:"ingreso"`
	IsEnabled        bool         `json:"is_enabled"`
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
