package sorter

import (
	"API-GREENEX/internal/models"
)

type Salida struct {
	ID            int          `json:"id"`
	Salida_Sorter string       `json:"salida_sorter"`
	SKUs_Actuales []models.SKU `json:"skus_actuales"`
	Estado        int          `json:"estado"`
	Ingreso       bool         `json:"ingreso"`
}

func (s Salida) GetNewSalida(ID int, salida_sorter string) Salida {
	return Salida{
		ID:            ID,
		Salida_Sorter: salida_sorter,
		SKUs_Actuales: []models.SKU{},
		Estado:        0,
		Ingreso:       false,
	}
}
