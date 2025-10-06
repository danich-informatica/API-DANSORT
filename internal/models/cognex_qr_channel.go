package models

import (
	"fmt"
	"time"
)

// LecturaEvent representa un evento de lectura desde cualquier dispositivo de escaneo
type LecturaEvent struct {
	Timestamp   time.Time `json:"timestamp"`
	Exitoso     bool      `json:"exitoso"`
	SKU         string    `json:"sku"` // Vacío si fue NO_READ o error
	Especie     string    `json:"especie"`
	Calibre     string    `json:"calibre"`
	Variedad    string    `json:"variedad"`
	Embalaje    string    `json:"embalaje"`
	Correlativo string    `json:"correlativo"` // ID de la caja insertada en DB
	Mensaje     string    `json:"mensaje"`     // Mensaje original recibido
	Error       error     `json:"error"`       // Error si hubo fallo
	Dispositivo string    `json:"dispositivo"` // Identificador del dispositivo (ej: "Cognex-01")
}

// TipoLectura representa el tipo de lectura
type TipoLectura string

const (
	LecturaExitosa     TipoLectura = "EXITOSA"
	LecturaNoRead      TipoLectura = "NO_READ"
	LecturaFormato     TipoLectura = "ERROR_FORMATO"
	LecturaSKU         TipoLectura = "ERROR_SKU"
	LecturaDB          TipoLectura = "ERROR_DB"
	LecturaDesconocido TipoLectura = "ERROR_DESCONOCIDO"
)

// GetTipo devuelve el tipo de lectura basado en el estado del evento
func (e *LecturaEvent) GetTipo() TipoLectura {
	if e.Exitoso {
		return LecturaExitosa
	}

	if e.Error == nil {
		return LecturaDesconocido
	}

	errorMsg := e.Error.Error()
	switch {
	case errorMsg == "NO_READ":
		return LecturaNoRead
	case errorMsg == "formato inválido" || errorMsg == "componentes vacíos":
		return LecturaFormato
	case contains(errorMsg, "SKU"):
		return LecturaSKU
	case contains(errorMsg, "insertar") || contains(errorMsg, "database"):
		return LecturaDB
	default:
		return LecturaDesconocido
	}
}

// String implementa fmt.Stringer
func (e *LecturaEvent) String() string {
	if e.Exitoso {
		return fmt.Sprintf("[%s] ✅ SKU: %s | Correlativo: %s",
			e.Timestamp.Format("15:04:05"), e.SKU, e.Correlativo)
	}
	return fmt.Sprintf("[%s] ❌ Error: %v | Mensaje: %s",
		e.Timestamp.Format("15:04:05"), e.Error, e.Mensaje)
}

// NewLecturaExitosa crea un evento de lectura exitosa
func NewLecturaExitosa(sku, especie, calibre, variedad, embalaje, correlativo, mensaje, dispositivo string) LecturaEvent {
	return LecturaEvent{
		Timestamp:   time.Now(),
		Exitoso:     true,
		SKU:         sku,
		Especie:     especie,
		Calibre:     calibre,
		Variedad:    variedad,
		Embalaje:    embalaje,
		Correlativo: correlativo,
		Mensaje:     mensaje,
		Dispositivo: dispositivo,
		Error:       nil,
	}
}

// NewLecturaFallida crea un evento de lectura fallida
func NewLecturaFallida(err error, mensaje, dispositivo string) LecturaEvent {
	return LecturaEvent{
		Timestamp:   time.Now(),
		Exitoso:     false,
		SKU:         "",
		Mensaje:     mensaje,
		Dispositivo: dispositivo,
		Error:       err,
	}
}

// NewLecturaFallidaConDatos crea un evento de lectura fallida con datos parciales
func NewLecturaFallidaConDatos(err error, especie, calibre, variedad, embalaje, mensaje, dispositivo string) LecturaEvent {
	return LecturaEvent{
		Timestamp:   time.Now(),
		Exitoso:     false,
		SKU:         "",
		Especie:     especie,
		Calibre:     calibre,
		Variedad:    variedad,
		Embalaje:    embalaje,
		Mensaje:     mensaje,
		Dispositivo: dispositivo,
		Error:       err,
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 &&
		(s == substr || (len(s) >= len(substr) && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
