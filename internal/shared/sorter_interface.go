package shared

import (
	"context"

	"api-dansort/internal/models"
)

type SorterInterface interface {
	GetCurrentSKUs() []models.SKUAssignable
	GetID() int
	GetSKUChannel() <-chan []models.SKUAssignable
	GetFlowStatsChannel() <-chan models.FlowStatistics
	GetSKUFlowPercentage(skuName string) float64
	AssignSKUToSalida(skuID uint32, salidaID int) (calibre, variedad, embalaje string, dark int, err error)
	RemoveSKUFromSalida(skuID uint32, salidaID int) (calibre, variedad, embalaje string, dark int, err error)
	RemoveAllSKUsFromSalida(salidaID int) ([]models.SKU, error)
	GetSalidas() []Salida
	UpdateSKUs(skus []models.SKUAssignable)        // Para sincronización de SKUs
	ReloadSalidasFromDB(ctx context.Context) error // Para recargar asignaciones desde PostgreSQL
}
