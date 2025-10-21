package models

import (
	"hash/fnv"
	"strings"
)

// SKU representa un SKU en la base de datos
type SKU struct {
	Variedad string
	Calibre  string
	Embalaje string
	SKU      string
	Estado   bool
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
	ID           int     `json:"id"`
	SKU          string  `json:"sku"`
	Calibre      string  `json:"calibre,omitempty"`
	Variedad     string  `json:"variedad,omitempty"`
	Embalaje     string  `json:"embalaje,omitempty"`
	Percentage   float64 `json:"percentage"`
	IsAssigned   bool    `json:"is_assigned"`
	IsMasterCase bool    `json:"is_master_case"`
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
		ID:           id,
		SKU:          s.CleanSKU(), // Limpia paréntesis del formato de BD
		Calibre:      s.Calibre,
		Variedad:     s.Variedad,
		Embalaje:     s.Embalaje,
		Percentage:   0.0,
		IsAssigned:   false,
		IsMasterCase: false,
	}
}

// ToAssignableWithHash convierte un SKU usando su hash como ID numérico
func (s *SKU) ToAssignableWithHash() SKUAssignable {
	return SKUAssignable{
		ID:           s.GetNumericID(),
		SKU:          s.CleanSKU(),
		Calibre:      s.Calibre,
		Variedad:     s.Variedad,
		Embalaje:     s.Embalaje,
		Percentage:   0.0,
		IsAssigned:   false,
		IsMasterCase: false,
	}
}

// GetRejectSKU retorna el SKU especial de REJECT con ID 0
func GetRejectSKU() SKUAssignable {
	return SKUAssignable{
		ID:           0,
		SKU:          "REJECT",
		Percentage:   0.0,
		IsAssigned:   false,
		IsMasterCase: false,
	}
}
