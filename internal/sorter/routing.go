package sorter

import (
	"API-GREENEX/internal/shared"
	"log"
)

// determinarSalida determina a qué salida debe ir la caja según el SKU/Calibre
func (s *Sorter) determinarSalida(sku, calibre string) shared.Salida {
	if sku == "REJECT" || sku == "0" {
		return s.getSalidaForReject()
	}

	if salida := s.getSalidaConBatchDistribution(sku); salida != nil {
		return *salida
	}

	if salida := s.getSalidaConfiguradaParaSKU(sku); salida != nil {
		return *salida
	}

	return s.getSalidaDescarte(sku)
}

// getSalidaForReject busca salida para SKUs REJECT
func (s *Sorter) getSalidaForReject() shared.Salida {
	for _, salida := range s.Salidas {
		if salida.Tipo == "manual" && salida.IsAvailable() {
			return salida
		}
	}

	for _, salida := range s.Salidas {
		if salida.IsAvailable() {
			log.Printf("⚠️ Sorter #%d: No hay salida manual disponible, usando salida %d", s.ID, salida.ID)
			return salida
		}
	}

	if len(s.Salidas) > 0 {
		log.Printf("🚨 Sorter #%d: ADVERTENCIA - Todas las salidas bloqueadas/en falla, usando última salida", s.ID)
		return s.Salidas[len(s.Salidas)-1]
	}

	return shared.Salida{}
}

// getSalidaConBatchDistribution obtiene salida usando distribución por lotes
func (s *Sorter) getSalidaConBatchDistribution(sku string) *shared.Salida {
	s.batchMutex.Lock()
	bd, hasBatchDistribution := s.batchCounters[sku]
	if !hasBatchDistribution || len(bd.Salidas) <= 1 {
		s.batchMutex.Unlock()
		return nil
	}

	salidaID := bd.NextSalida()
	s.batchMutex.Unlock()

	for i := range s.Salidas {
		if s.Salidas[i].ID == salidaID {
			if s.Salidas[i].IsAvailable() {
				log.Printf("🔄 Sorter #%d: SKU '%s' → Salida %d '%s' (lote %d/%d)",
					s.ID, sku, s.Salidas[i].ID, s.Salidas[i].Salida_Sorter,
					bd.CurrentCount, bd.BatchSizes[salidaID])
				return &s.Salidas[i]
			}

			estado := s.Salidas[i].GetEstado()
			bloqueo := s.Salidas[i].GetBloqueo()
			log.Printf("⚠️ Sorter #%d: Salida %d no disponible (Estado=%d, Bloqueo=%t), buscando alternativa...",
				s.ID, s.Salidas[i].ID, estado, bloqueo)
			break
		}
	}

	return nil
}

// getSalidaConfiguradaParaSKU busca salida configurada para un SKU específico
func (s *Sorter) getSalidaConfiguradaParaSKU(sku string) *shared.Salida {
	for i := range s.Salidas {
		for _, skuConfig := range s.Salidas[i].SKUs_Actuales {
			if skuConfig.SKU == sku {
				if s.Salidas[i].IsAvailable() {
					return &s.Salidas[i]
				}

				estado := s.Salidas[i].GetEstado()
				bloqueo := s.Salidas[i].GetBloqueo()
				log.Printf("⚠️ Sorter #%d: Salida %d configurada para SKU '%s' pero NO disponible (Estado=%d, Bloqueo=%t)",
					s.ID, s.Salidas[i].ID, sku, estado, bloqueo)
			}
		}
	}
	return nil
}

// getSalidaDescarte obtiene salida de descarte como último recurso
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
