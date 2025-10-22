package sorter

import (
	"API-GREENEX/internal/models"
	"API-GREENEX/internal/shared"
	"fmt"
	"log"
)

// AssignSKUToSalida asigna una SKU a una salida específica
func (s *Sorter) AssignSKUToSalida(skuID uint32, salidaID int) (calibre, variedad, embalaje string, dark int, err error) {
	targetSalida := s.findSalidaByID(salidaID)
	if targetSalida == nil {
		return "", "", "", 0, fmt.Errorf("salida con ID %d no encontrada en sorter #%d", salidaID, s.ID)
	}

	if skuID == 0 && targetSalida.Tipo == "automatico" {
		return "", "", "", 0, fmt.Errorf("no se puede asignar SKU REJECT (ID=0) a salida automática '%s' (ID=%d)",
			targetSalida.Salida_Sorter, salidaID)
	}

	targetSKU := s.findSKUByID(skuID)
	if targetSKU == nil {
		// Solo mostrar detalle cuando falla
		log.Printf("❌ Sorter #%d: SKU ID=%d no disponible. Total SKUs en memoria: %d", s.ID, skuID, len(s.assignedSKUs))
		return "", "", "", 0, fmt.Errorf("SKU con ID %d no encontrada en las SKUs disponibles del sorter #%d", skuID, s.ID)
	}

	sku := models.SKU{
		SKU:      targetSKU.SKU,
		Variedad: targetSKU.Variedad,
		Calibre:  targetSKU.Calibre,
		Embalaje: targetSKU.Embalaje,
		Dark:     targetSKU.Dark,
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

	return targetSKU.Calibre, targetSKU.Variedad, targetSKU.Embalaje, targetSKU.Dark, nil
}

// RemoveSKUFromSalida elimina una SKU específica de una salida
func (s *Sorter) RemoveSKUFromSalida(skuID uint32, salidaID int) (calibre, variedad, embalaje string, dark int, err error) {
	// ✅ PROTECCIÓN: NO permitir eliminar SKU REJECT (ID=0)
	if skuID == 0 {
		return "", "", "", 0, fmt.Errorf("no se puede eliminar SKU REJECT (ID=0), está protegida")
	}

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
		return "", "", "", 0, fmt.Errorf("salida con ID %d no encontrada en sorter #%d", salidaID, s.ID)
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
		return "", "", "", 0, fmt.Errorf("SKU con ID %d no encontrada en salida %d del sorter #%d", skuID, salidaID, s.ID)
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

	return removedSKU.Calibre, removedSKU.Variedad, removedSKU.Embalaje, removedSKU.Dark, nil
}

// RemoveAllSKUsFromSalida elimina TODAS las SKUs de una salida específica
// y re-inserta automáticamente la SKU REJECT
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

	// Guardar todas las SKUs actuales antes de borrar
	removedSKUs := make([]models.SKU, len(targetSalida.SKUs_Actuales))
	copy(removedSKUs, targetSalida.SKUs_Actuales)

	// Marcar todas como no asignadas
	for _, sku := range removedSKUs {
		skuID := uint32(sku.GetNumericID())
		for i := range s.assignedSKUs {
			if uint32(s.assignedSKUs[i].ID) == skuID {
				s.assignedSKUs[i].IsAssigned = false
			}
		}
	}

	// 🧹 BORRAR TODO
	s.Salidas[salidaIndex].SKUs_Actuales = []models.SKU{}

	// ♻️ RE-INSERTAR SKU REJECT automáticamente
	rejectSKU := models.SKU{
		SKU:      "REJECT",
		Calibre:  "REJECT",
		Variedad: "REJECT",
		Embalaje: "REJECT",
	}
	s.Salidas[salidaIndex].SKUs_Actuales = append(s.Salidas[salidaIndex].SKUs_Actuales, rejectSKU)

	log.Printf("🧹 Sorter #%d: Eliminadas %d SKUs de salida '%s' (ID=%d)",
		s.ID, len(removedSKUs), targetSalida.Salida_Sorter, salidaID)
	log.Printf("♻️  Sorter #%d: SKU REJECT re-insertada automáticamente en salida %d", s.ID, salidaID)

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
