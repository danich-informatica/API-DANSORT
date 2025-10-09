package shared

import "API-GREENEX/internal/models"

type SorterInterface interface {
	GetCurrentSKUs() []models.SKUAssignable
	GetID() int
	GetSKUChannel() <-chan []models.SKUAssignable
	GetFlowStatsChannel() <-chan models.FlowStatistics
	GetSKUFlowPercentage(skuName string) float64
	AssignSKUToSalida(skuID uint32, salidaID int) (calibre, variedad, embalaje string, err error)
	RemoveSKUFromSalida(skuID uint32, salidaID int) (calibre, variedad, embalaje string, err error)
	RemoveAllSKUsFromSalida(salidaID int) ([]models.SKU, error)
	GetSalidas() []Salida
}
