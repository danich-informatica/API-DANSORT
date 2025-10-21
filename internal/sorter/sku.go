package sorter

import (
	"API-GREENEX/internal/models"
	"log"
	"time"
)

// UpdateSKUs actualiza la lista de SKUs asignados a este sorter y publica al canal
func (s *Sorter) UpdateSKUs(skus []models.SKUAssignable) {
	s.assignedSKUs = skus

	select {
	case s.skuChannel <- skus:
		log.Printf("📦 Sorter #%d: SKUs actualizados (%d SKUs)", s.ID, len(skus))
	default:
		log.Printf("⚠️  Sorter #%d: Canal lleno, SKUs no publicados", s.ID)
	}
}

// GetSKUChannel retorna el canal de SKUs de este sorter
func (s *Sorter) GetSKUChannel() <-chan []models.SKUAssignable {
	return s.skuChannel
}

// GetCurrentSKUs retorna la lista actual de SKUs asignados con porcentajes actualizados
func (s *Sorter) GetCurrentSKUs() []models.SKUAssignable {
	s.flowMutex.RLock()
	defer s.flowMutex.RUnlock()

	skusWithPercentage := make([]models.SKUAssignable, len(s.assignedSKUs))
	copy(skusWithPercentage, s.assignedSKUs)

	for i := range skusWithPercentage {
		if percentage, exists := s.lastFlowStats[skusWithPercentage[i].SKU]; exists {
			skusWithPercentage[i].Percentage = float64(int(percentage + 0.5))
		} else {
			skusWithPercentage[i].Percentage = 0.0
		}
	}

	return skusWithPercentage
}

// StartSKUPublisher inicia un publicador periódico de SKUs
func (s *Sorter) StartSKUPublisher(intervalSeconds int, skuSource func() []models.SKUAssignable) {
	go func() {
		ticker := time.NewTicker(time.Duration(intervalSeconds) * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-s.ctx.Done():
				log.Printf("⏸️  Sorter #%d: Publicador de SKUs detenido", s.ID)
				return
			case <-ticker.C:
				skus := skuSource()
				s.UpdateSKUs(skus)
			}
		}
	}()
	log.Printf("⏰ Sorter #%d: Publicador de SKUs iniciado (cada %ds)", s.ID, intervalSeconds)
}
