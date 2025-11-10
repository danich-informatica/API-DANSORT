package sorter

import (
	"API-DANSORT/internal/shared"
	"log"
	"time"
)

// determinarSalida determina a qué salida debe ir la caja según el SKU/Calibre
func (s *Sorter) determinarSalida(sku, calibre string) shared.Salida {
	startRouting := time.Now() // ⏱️ Medición de routing completo

	// Buscar salida con balance
	if salida := s.getSalidaConBatchDistribution(sku); salida != nil {
		routingDuration := time.Since(startRouting)
		log.Printf("⏱️  [Sorter %d] ROUTING completado en %.3fms (SKU encontrado)", s.ID, routingDuration.Seconds()*1000)
		return *salida
	}

	// Si no encuentra el SKU específico, buscar REJECT
	if salida := s.getSalidaConBatchDistribution("REJECT"); salida != nil {
		routingDuration := time.Since(startRouting)
		log.Printf("⏱️  [Sorter %d] ROUTING completado en %.3fms (usando REJECT)", s.ID, routingDuration.Seconds()*1000)
		return *salida
	}

	routingDuration := time.Since(startRouting)
	log.Printf("⏱️  [Sorter %d] ROUTING completado en %.3fms (usando DESCARTE)", s.ID, routingDuration.Seconds()*1000)
	return s.getSalidaDescarte(sku)
}

// getSalidaConBatchDistribution obtiene salida usando round-robin con validación de disponibilidad
func (s *Sorter) getSalidaConBatchDistribution(sku string) *shared.Salida {
	startSearch := time.Now() // ⏱️ Medición de búsqueda de SKU

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

	searchDuration := time.Since(startSearch)

	// DEBUG: Ver qué encontró
	if len(todasLasSalidas) == 0 {
		log.Printf("[Sorter %d] ⚠️ SKU '%s' NOT FOUND in any salida's SKUs_Actuales (búsqueda: %.3fms)",
			s.ID, sku, searchDuration.Seconds()*1000)
		return nil
	}

	// DEBUG: Mostrar IDs de las salidas encontradas
	salidaIDs := make([]int, len(todasLasSalidas))
	for i, sal := range todasLasSalidas {
		salidaIDs[i] = sal.ID
	}
	log.Printf("[Sorter %d] ✓ SKU '%s' found in %d salida(s): %v (búsqueda: %.3fms)",
		s.ID, sku, len(todasLasSalidas), salidaIDs, searchDuration.Seconds()*1000)

	// Si solo hay una salida, retornarla si está disponible
	if len(todasLasSalidas) == 1 {
		if todasLasSalidas[0].IsAvailable() {
			return todasLasSalidas[0]
		}
		// 🚨 LOG CRÍTICO: Por qué la salida NO está disponible
		log.Printf("🚨 [Sorter %d] Salida ID=%d NO disponible para SKU '%s' (Estado=%d, Bloqueo=%t)",
			s.ID, todasLasSalidas[0].ID, sku,
			todasLasSalidas[0].GetEstado(), todasLasSalidas[0].GetBloqueo())
		return nil
	}

	// Múltiples salidas: usar round-robin con batch_size
	s.batchMutex.Lock()
	defer s.batchMutex.Unlock()

	// Obtener o crear distribuidor para este SKU
	bd, exists := s.batchCounters[sku]
	if !exists {
		// Primera vez: crear distribuidor en índice 0
		s.batchCounters[sku] = &BatchDistributor{
			CurrentIndex: 0,
			CurrentCount: 0,
		}
		bd = s.batchCounters[sku]
		log.Printf("[Sorter %d] ⚖️ Balance activated for SKU '%s' with %d lanes", s.ID, sku, len(todasLasSalidas))
	}

	// Buscar salida disponible empezando desde índice actual
	startIdx := bd.CurrentIndex
	for attempts := 0; attempts < len(todasLasSalidas); attempts++ {
		idx := (startIdx + attempts) % len(todasLasSalidas)
		salida := todasLasSalidas[idx]

		if salida.IsAvailable() {
			// ✅ Incrementar contador de cajas enviadas a esta salida
			bd.CurrentCount++

			// ✅ Si alcanzamos el batch_size, rotar a la siguiente salida
			if bd.CurrentCount >= salida.BatchSize {
				bd.CurrentIndex = (idx + 1) % len(todasLasSalidas)
				bd.CurrentCount = 0
				log.Printf("[Sorter %d] 🔄 SKU '%s': Batch completado en salida %d (%d cajas), rotando a siguiente",
					s.ID, sku, salida.ID, salida.BatchSize)
			} else {
				// Mantener en la misma salida (batch no completado)
				bd.CurrentIndex = idx
			}

			return salida
		}
		// 🚨 LOG: Por qué esta salida no está disponible
		log.Printf("⚠️  [Sorter %d] Salida ID=%d NO disponible (intento %d/%d) para SKU '%s' (Estado=%d, Bloqueo=%t)",
			s.ID, salida.ID, attempts+1, len(todasLasSalidas), sku,
			salida.GetEstado(), salida.GetBloqueo())
	}

	// Ninguna salida disponible
	log.Printf("🚨 [Sorter %d] SKU '%s': Ninguna de las %d salidas está disponible (todas bloqueadas/llenas)", s.ID, sku, len(todasLasSalidas))
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
