package models

import (
	"fmt"
	"hash/fnv"
	"strings"
)

// SKU representa un SKU en la base de datos
type SKU struct {
	Variedad       string // Código de variedad (ej: "V018") - uso interno
	Calibre        string
	Embalaje       string
	Dark           int
	SKU            string
	Estado         bool
	NombreVariedad string // Nombre de variedad (ej: "LAPINS") - para frontend
}

// BuildSKU construye el string SKU en formato: calibre-NOMBRE_VARIEDAD-embalaje-dark
// Usa NombreVariedad si existe, sino usa Variedad (código)
// Siempre en MAYÚSCULAS
func (s *SKU) BuildSKU() string {
	variedad := s.NombreVariedad
	if variedad == "" {
		variedad = s.Variedad // Fallback al código si no hay nombre
	}
	return fmt.Sprintf("%s-%s-%s-%d", s.Calibre, strings.ToUpper(variedad), s.Embalaje, s.Dark)
}

// CleanSKU limpia el formato de SKU de la base de datos:
// - Remueve paréntesis: (XXX,XXX,XXX) -> XXX,XXX,XXX
// - Remueve ",t" al final
// - Convierte comas a guiones: XXX,XXX,XXX -> XXX-XXX-XXX
func (s *SKU) CleanSKU() string {
	cleaned := strings.TrimSpace(s.SKU)

	// Remover paréntesis
	cleaned = strings.Trim(cleaned, "()")

	// Remover ",t" al final
	cleaned = strings.TrimSuffix(cleaned, ",t")

	// Reemplazar comas por guiones
	cleaned = strings.ReplaceAll(cleaned, ",", "-")

	return cleaned
}

// SKUAssignable representa la estructura JSON para el endpoint HTTP de SKUs asignables
type SKUAssignable struct {
	ID             int     `json:"id"`
	SKU            string  `json:"sku"`
	Calibre        string  `json:"calibre,omitempty"`
	Variedad       string  `json:"variedad,omitempty"`        // Código de variedad (ej: "V018")
	NombreVariedad string  `json:"nombre_variedad,omitempty"` // Nombre de variedad (ej: "LAPINS")
	Embalaje       string  `json:"embalaje,omitempty"`
	Dark           int     `json:"dark"`
	Percentage     float64 `json:"percentage"`
	IsAssigned     bool    `json:"is_assigned"`
	IsMasterCase   bool    `json:"is_master_case"`
}

// SKUChannel es un canal tipado para streaming de SKUs
type SKUChannel chan SKU

// NewSKUChannel crea un nuevo canal de SKUs con buffer especificado
func NewSKUChannel(buffer int) SKUChannel {
	return make(SKUChannel, buffer)
}

// GetNumericID genera un ID numérico único basado en el hash del SKU
// El mismo SKU siempre generará el mismo ID (determinístico)
func (s *SKU) GetNumericID() int {
	cleaned := s.CleanSKU()
	h := fnv.New32a()
	h.Write([]byte(cleaned))
	// Convertir a int positivo y limitar a rango razonable
	return int(h.Sum32() & 0x7FFFFFFF) // Máscara para asegurar valor positivo
}

// ToAssignable convierte un SKU a su representación HTTP asignable
func (s *SKU) ToAssignable(id int) SKUAssignable {
	return SKUAssignable{
		ID:             id,
		SKU:            s.CleanSKU(), // Limpia paréntesis del formato de BD
		Calibre:        s.Calibre,
		Variedad:       s.Variedad,       // Código (ej: "V018")
		NombreVariedad: s.NombreVariedad, // Nombre (ej: "LAPINS")
		Embalaje:       s.Embalaje,
		Dark:           s.Dark,
		Percentage:     0.0,
		IsAssigned:     false,
		IsMasterCase:   false,
	}
}

// ToAssignableWithHash convierte un SKU usando su hash como ID numérico
func (s *SKU) ToAssignableWithHash() SKUAssignable {
	return SKUAssignable{
		ID:             s.GetNumericID(),
		SKU:            s.CleanSKU(),
		Calibre:        s.Calibre,
		Variedad:       s.Variedad,       // Código (ej: "V018")
		NombreVariedad: s.NombreVariedad, // Nombre (ej: "LAPINS")
		Embalaje:       s.Embalaje,
		Dark:           s.Dark,
		Percentage:     0.0,
		IsAssigned:     false,
		IsMasterCase:   false,
	}
}

// GetRejectSKU retorna el SKU especial de REJECT con ID 0
func GetRejectSKU() SKUAssignable {
	return SKUAssignable{
		ID:           0,
		SKU:          "REJECT",
		Dark:         0,
		Percentage:   0.0,
		IsAssigned:   false,
		IsMasterCase: false,
	}
}
