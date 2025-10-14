package models

import "time"

// SKUFlowStat representa las estadísticas de flujo de un SKU específico
type SKUFlowStat struct {
	ID         int     `json:"id"`
	SKU        string  `json:"sku"`
	Lecturas   int     `json:"lecturas"`
	Porcentaje float64 `json:"porcentaje"`
}

// FlowStatistics representa las estadísticas de flujo completas de un sorter
type FlowStatistics struct {
	SorterID      int           `json:"sorter_id"`
	Timestamp     time.Time     `json:"timestamp"`
	Stats         []SKUFlowStat `json:"stats"`
	TotalLecturas int           `json:"total_lecturas"`
	WindowSeconds int           `json:"window_seconds"` // Ventana de tiempo en segundos
}

// LecturaRecord representa una lectura individual con timestamp
// Se usa para la ventana deslizante
type LecturaRecord struct {
	SKU       string
	Timestamp time.Time
}
