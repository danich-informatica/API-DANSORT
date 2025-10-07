package shared

import "API-GREENEX/internal/models"

type SorterInterface interface {
	GetCurrentSKUs() []models.SKUAssignable
	GetID() int
	AssignSKUToSalida(skuID uint32, salidaID int) (calibre, variedad, embalaje string, err error)
	GetSalidas() []Salida
}
