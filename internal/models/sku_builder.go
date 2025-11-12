package models

import (
	"fmt"
	"strings"
)

func RequestSKU(variedad, calibre, embalaje string, dark int) (SKU, error) {
	var varity string = strings.Split(variedad, "-")[0]
	var cal string = strings.Split(calibre, "-")[0]
	var amb string = strings.Split(embalaje, "-")[0]
	var sku string = fmt.Sprintf("%s-%s-%s-%d", cal, varity, amb, dark)
	// Validaciones básicas
	if varity == "" || cal == "" || amb == "" {
		return SKU{}, fmt.Errorf("sku: uno de los componentes está vacío")
	}

	var skuObj = SKU{
		Variedad: varity,
		Calibre:  cal,
		Embalaje: amb,
		Dark:     dark,
		SKU:      sku,
		Estado:   true, // Por defecto, se asume activo
	}
	return skuObj, nil
}

// modificamos el to string para hacerle print
func (s *SKU) String() string {
	return fmt.Sprintf("SKU{Variedad: %s, Calibre: %s, Embalaje: %s, Dark: %d, Linea: %s, SKU: %s, Estado: %t, NombreVariedad: %s}",
		s.Variedad, s.Calibre, s.Embalaje, s.Dark, s.Linea, s.SKU, s.Estado, s.NombreVariedad)
}
