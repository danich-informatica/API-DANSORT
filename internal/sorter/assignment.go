package sorter

import (
	"API-GREENEX/internal/models"
	"API-GREENEX/internal/shared"
	"fmt"
	"log"
)

// AssignSKUToSalida asigna una SKU a una salida específica
func (s *Sorter) AssignSKUToSalida(skuID uint32, salidaID int) (calibre, variedad, embalaje string, err error) {
	targetSalida := s.findSalidaByID(salidaID)
	if targetSalida == nil {
		return "", "", "", fmt.Errorf("salida con ID %d no encontrada en sorter #%d", salidaID, s.ID)
	}

	if skuID == 0 && targetSalida.Tipo == "automatico" {
		return "", "", "", fmt.Errorf("no se puede asignar SKU REJECT (ID=0) a salida automática '%s' (ID=%d)",
			targetSalida.Salida_Sorter, salidaID)
	}

	targetSKU := s.findSKUByID(skuID)
	if targetSKU == nil {
		return "", "", "", fmt.Errorf("SKU con ID %d no encontrada en las SKUs disponibles del sorter #%d", skuID, s.ID)
	}

	sku := models.SKU{
		SKU:      targetSKU.SKU,
		Variedad: targetSKU.Variedad,
		Calibre:  targetSKU.Calibre,
		Embalaje: targetSKU.Embalaje,
		Estado:   true,
	}

	targetSalida.SKUs_Actuales = append(targetSalida.SKUs_Actuales, sku)
	targetSKU.IsAssigned = true

	s.batchMutex.Lock()
	s.updateBatchDistributor(targetSKU.SKU)
	s.batchMutex.Unlock()

	log.Printf("✅ Sorter #%d: SKU '%s' (ID=%d) asignada a salida '%s' (ID=%d, tipo=%s)",
		s.ID, targetSKU.SKU, skuID, targetSalida.Salida_Sorter, salidaID, targetSalida.Tipo)

	s.UpdateSKUs(s.assignedSKUs)

	return targetSKU.Calibre, targetSKU.Variedad, targetSKU.Embalaje, nil
}

// RemoveSKUFromSalida elimina una SKU específica de una salida
func (s *Sorter) RemoveSKUFromSalida(skuID uint32, salidaID int) (calibre, variedad, embalaje string, err error) {
	var targetSalida *shared.Salida
	var salidaIndex int

	for i := range s.Salidas {
		if s.Salidas[i].ID == salidaID {
			targetSalida = &s.Salidas[i]
			salidaIndex = i
			break
		}
	}

	if targetSalida == nil {
		return "", "", "", fmt.Errorf("salida con ID %d no encontrada en sorter #%d", salidaID, s.ID)
	}

	skuFound := false
	var removedSKU models.SKU
	newSKUs := make([]models.SKU, 0, len(targetSalida.SKUs_Actuales))

	for _, sku := range targetSalida.SKUs_Actuales {
		currentSKUID := uint32(sku.GetNumericID())
		if currentSKUID == skuID {
			skuFound = true
			removedSKU = sku
			continue
		}
		newSKUs = append(newSKUs, sku)
	}

	if !skuFound {
		return "", "", "", fmt.Errorf("SKU con ID %d no encontrada en salida %d del sorter #%d", skuID, salidaID, s.ID)
	}

	s.Salidas[salidaIndex].SKUs_Actuales = newSKUs

	for i := range s.assignedSKUs {
		if uint32(s.assignedSKUs[i].ID) == skuID {
			s.assignedSKUs[i].IsAssigned = false
			break
		}
	}

	log.Printf("🗑️  Sorter #%d: SKU '%s' (ID=%d) eliminada de salida '%s' (ID=%d)",
		s.ID, removedSKU.SKU, skuID, targetSalida.Salida_Sorter, salidaID)

	s.UpdateSKUs(s.assignedSKUs)

	return removedSKU.Calibre, removedSKU.Variedad, removedSKU.Embalaje, nil
}

// RemoveAllSKUsFromSalida elimina TODAS las SKUs de una salida específica
func (s *Sorter) RemoveAllSKUsFromSalida(salidaID int) ([]models.SKU, error) {
	var targetSalida *shared.Salida
	var salidaIndex int

	for i := range s.Salidas {
		if s.Salidas[i].ID == salidaID {
			targetSalida = &s.Salidas[i]
			salidaIndex = i
			break
		}
	}

	if targetSalida == nil {
		return nil, fmt.Errorf("salida con ID %d no encontrada en sorter #%d", salidaID, s.ID)
	}

	// Separar SKUs removibles de REJECT (ID=0)
	var removedSKUs []models.SKU
	var keepSKUs []models.SKU

	for _, sku := range targetSalida.SKUs_Actuales {
		skuID := uint32(sku.GetNumericID())

		// ✅ PROTECCIÓN: NO eliminar SKU REJECT (ID=0)
		if skuID == 0 {
			keepSKUs = append(keepSKUs, sku)
			log.Printf("🛡️  Sorter #%d: SKU REJECT (ID=0) protegida, NO se eliminará de salida %d", s.ID, salidaID)
			continue
		}

		// Marcar SKU como no asignada
		for i := range s.assignedSKUs {
			if uint32(s.assignedSKUs[i].ID) == skuID {
				s.assignedSKUs[i].IsAssigned = false
			}
		}

		removedSKUs = append(removedSKUs, sku)
	}

	// Mantener solo las SKUs protegidas (REJECT)
	s.Salidas[salidaIndex].SKUs_Actuales = keepSKUs

	log.Printf("🧹 Sorter #%d: Eliminadas %d SKUs de salida '%s' (ID=%d), %d SKUs protegidas",
		s.ID, len(removedSKUs), targetSalida.Salida_Sorter, salidaID, len(keepSKUs))

	s.UpdateSKUs(s.assignedSKUs)

	return removedSKUs, nil
}

// findSalidaByID busca una salida por ID
func (s *Sorter) findSalidaByID(salidaID int) *shared.Salida {
	for i := range s.Salidas {
		if s.Salidas[i].ID == salidaID {
			return &s.Salidas[i]
		}
	}
	return nil
}

// findSKUByID busca una SKU por ID
func (s *Sorter) findSKUByID(skuID uint32) *models.SKUAssignable {
	for i := range s.assignedSKUs {
		if uint32(s.assignedSKUs[i].ID) == skuID {
			return &s.assignedSKUs[i]
		}
	}
	return nil
}
