package sku

import (
	"fmt"
	"strings"

	"API-GREENEX/internal/models"
)

func RequestSKU(variedad, calibre, embalaje string) (models.SKU, error) {
	var varity string = strings.Split(variedad, "-")[0]
	var cal string = strings.Split(calibre, "-")[0]
	var amb string = strings.Split(embalaje, "-")[0]
	var sku string = fmt.Sprintf("%s-%s-%s", cal, varity, amb)
	// Validaciones básicas
	if varity == "" || cal == "" || amb == "" {
		return models.SKU{}, fmt.Errorf("sku: uno de los componentes está vacío")
	}

	var skuObj = models.SKU{
		Variedad: varity,
		Calibre:  cal,
		Embalaje: amb,
		SKU:      sku,
	}
	return skuObj, nil
}
