package models

import (
	"fmt"
	"time"
)

// DataMatrixEvent representa una lectura de código DataMatrix desde Cognex
type DataMatrixEvent struct {
	Codigo      string    `json:"codigo"`      // Código DataMatrix leído (correlativo)
	Dispositivo string    `json:"dispositivo"` // Nombre del dispositivo Cognex
	CognexID    int       `json:"cognex_id"`   // ID del Cognex que leyó el código
	Timestamp   time.Time `json:"timestamp"`   // Momento de la lectura
	RawMessage  string    `json:"raw_message"` // Mensaje crudo recibido
}

// NewDataMatrixEvent crea un nuevo evento de DataMatrix
func NewDataMatrixEvent(codigo string, dispositivo string, cognexID int, rawMessage string) DataMatrixEvent {
	return DataMatrixEvent{
		Codigo:      codigo,
		Dispositivo: dispositivo,
		CognexID:    cognexID,
		Timestamp:   time.Now(),
		RawMessage:  rawMessage,
	}
}

// String retorna representación en texto del evento
func (e DataMatrixEvent) String() string {
	return fmt.Sprintf("DataMatrix[Cognex#%d | Código:%s | %s]",
		e.CognexID, e.Codigo, e.Timestamp.Format("15:04:05"))
}

// GetCorrelativo retorna el correlativo (código DataMatrix)
func (e DataMatrixEvent) GetCorrelativo() string {
	return e.Codigo
}
