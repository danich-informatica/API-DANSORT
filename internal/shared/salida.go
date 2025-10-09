package shared

import (
	"API-GREENEX/internal/models"
)

type Salida struct {
	ID               int          `json:"id"`
	SealerPhysicalID int          `json:"sealer_physical_id"`
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
		SealerPhysicalID: ID, // Por ahora igual, pero puede ser diferente en el futuro
		Salida_Sorter:    salida_sorter,
		Tipo:             tipo,
		BatchSize:        batchSize,
		SKUs_Actuales:    []models.SKU{},
		Estado:           0,
		Ingreso:          false,
		IsEnabled:        true,
	}
}
