package sorter

import (
	"API-GREENEX/internal/shared"
	"log"
)

// determinarSalida determina a qué salida debe ir la caja según el SKU/Calibre
func (s *Sorter) determinarSalida(sku, calibre string) shared.Salida {
	// Buscar salida con balance
	if salida := s.getSalidaConBatchDistribution(sku); salida != nil {
		return *salida
	}

	// Si no encuentra el SKU específico, buscar REJECT
	if salida := s.getSalidaConBatchDistribution("REJECT"); salida != nil {
		return *salida
	}

	return s.getSalidaDescarte(sku)
}

// getSalidaConBatchDistribution obtiene salida usando round-robin con validación de disponibilidad
func (s *Sorter) getSalidaConBatchDistribution(sku string) *shared.Salida {
	// Buscar TODAS las salidas que tienen este SKU en SKUs_Actuales
	var todasLasSalidas []*shared.Salida

	for i := range s.Salidas {
		for _, skuConfig := range s.Salidas[i].SKUs_Actuales {
			if skuConfig.SKU == sku {
				todasLasSalidas = append(todasLasSalidas, &s.Salidas[i])
				break
			}
		}
	}

	// DEBUG: Ver qué encontró
	if len(todasLasSalidas) == 0 {
		log.Printf("[Sorter %d] ⚠️ SKU '%s' NOT FOUND in any salida's SKUs_Actuales", s.ID, sku)
		return nil
	}

	// DEBUG: Mostrar IDs de las salidas encontradas
	salidaIDs := make([]int, len(todasLasSalidas))
	for i, sal := range todasLasSalidas {
		salidaIDs[i] = sal.ID
	}
	log.Printf("[Sorter %d] ✓ SKU '%s' found in %d salida(s): %v", s.ID, sku, len(todasLasSalidas), salidaIDs)

	// Si solo hay una salida, retornarla si está disponible
	if len(todasLasSalidas) == 1 {
		if todasLasSalidas[0].IsAvailable() {
			return todasLasSalidas[0]
		}
		return nil
	}

	// Múltiples salidas: usar round-robin simple por índice
	s.batchMutex.Lock()
	defer s.batchMutex.Unlock()

	// Obtener índice actual para este SKU
	bd, exists := s.batchCounters[sku]
	var startIdx int

	if !exists {
		// Primera vez: empezar en 0
		startIdx = 0
		s.batchCounters[sku] = &BatchDistributor{
			CurrentIndex: 0,
		}
		bd = s.batchCounters[sku]
		log.Printf("[Sorter %d] Balance activated for SKU '%s' with %d lanes", s.ID, sku, len(todasLasSalidas))
	} else {
		// Avanzar al siguiente
		startIdx = (bd.CurrentIndex + 1) % len(todasLasSalidas)
	}

	// Buscar la primera salida disponible empezando desde startIdx
	for attempts := 0; attempts < len(todasLasSalidas); attempts++ {
		idx := (startIdx + attempts) % len(todasLasSalidas)
		if todasLasSalidas[idx].IsAvailable() {
			// Actualizar índice para la próxima vez
			bd.CurrentIndex = idx
			return todasLasSalidas[idx]
		}
	}

	// Ninguna salida disponible
	return nil
} // getSalidaDescarte obtiene salida de descarte como último recurso
// SOLO usa la salida con SKU "REJECT" asignado, no cualquier salida manual
func (s *Sorter) getSalidaDescarte(sku string) shared.Salida {
	// Buscar SOLO la salida que tiene "REJECT" asignado
	for i := range s.Salidas {
		for _, skuConfig := range s.Salidas[i].SKUs_Actuales {
			if skuConfig.SKU == "REJECT" {
				if s.Salidas[i].IsAvailable() {
					log.Printf("⚠️ Sorter #%d: SKU '%s' sin salida configurada, enviando a REJECT (salida %d)",
						s.ID, sku, s.Salidas[i].ID)
					return s.Salidas[i]
				}

				estado := s.Salidas[i].GetEstado()
				bloqueo := s.Salidas[i].GetBloqueo()
				log.Printf("🚨 Sorter #%d: Salida REJECT (ID=%d) NO disponible (Estado=%d, Bloqueo=%t) para SKU '%s'",
					s.ID, s.Salidas[i].ID, estado, bloqueo, sku)
				break
			}
		}
	}

	// Si no hay salida REJECT disponible, NO enviar a ninguna salida
	log.Printf("🚨 Sorter #%d: ERROR CRÍTICO - No hay salida REJECT disponible para SKU '%s', caja perdida",
		s.ID, sku)

	// Retornar salida vacía (no válida)
	return shared.Salida{}
}

// GetDiscardSalida retorna una salida de descarte
func (s *Sorter) GetDiscardSalida() *shared.Salida {
	for i := range s.Salidas {
		for _, sku := range s.Salidas[i].SKUs_Actuales {
			if sku.SKU == "REJECT" {
				return &s.Salidas[i]
			}
		}
	}

	for i := range s.Salidas {
		if s.Salidas[i].Tipo == "manual" {
			log.Printf("⚠️  Sorter #%d: Asignando salida manual '%s' (ID=%d) como descarte automático",
				s.ID, s.Salidas[i].Salida_Sorter, s.Salidas[i].ID)
			return &s.Salidas[i]
		}
	}

	if len(s.Salidas) > 0 {
		log.Printf("🚨 Sorter #%d: ADVERTENCIA - No hay salida manual, usando última salida como descarte", s.ID)
		return &s.Salidas[len(s.Salidas)-1]
	}

	return nil
}

// FindSalidaForSKU busca en qué salida está asignada una SKU específica
func (s *Sorter) FindSalidaForSKU(skuText string) int {
	for _, salida := range s.Salidas {
		for _, sku := range salida.SKUs_Actuales {
			if sku.SKU == skuText {
				return salida.ID
			}
		}
	}
	return -1
}

// updateBatchDistributor actualiza o crea el distribuidor de lotes para una SKU
func (s *Sorter) updateBatchDistributor(skuName string) {
	var salidaIDs []int
	batchSizes := make(map[int]int)

	for _, salida := range s.Salidas {
		for _, sku := range salida.SKUs_Actuales {
			if sku.SKU == skuName {
				salidaIDs = append(salidaIDs, salida.ID)
				batchSizes[salida.ID] = salida.BatchSize
				break
			}
		}
	}

	if len(salidaIDs) <= 1 {
		delete(s.batchCounters, skuName)
		return
	}

	if bd, exists := s.batchCounters[skuName]; exists {
		bd.Salidas = salidaIDs
		bd.BatchSizes = batchSizes
		if bd.CurrentIndex >= len(salidaIDs) {
			bd.CurrentIndex = 0
			bd.CurrentCount = 0
		}
	} else {
		s.batchCounters[skuName] = &BatchDistributor{
			Salidas:      salidaIDs,
			CurrentIndex: 0,
			CurrentCount: 0,
			BatchSizes:   batchSizes,
		}
		log.Printf("🔄 Sorter #%d: BatchDistributor creado para SKU '%s' con %d salidas", s.ID, skuName, len(salidaIDs))
	}
}
