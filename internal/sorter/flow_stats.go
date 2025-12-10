package sorter

import (
	"API-GREENEX/internal/models"
	"hash/fnv"
	"log"
	"time"
)

// StartFlowStatistics inicia el cálculo periódico de estadísticas de flujo
func (s *Sorter) StartFlowStatistics(calculationInterval, windowDuration time.Duration) {
	log.Printf("📊 Sorter #%d: Iniciando sistema de estadísticas de flujo", s.ID)
	log.Printf("   ⏱️  Sorter #%d: Intervalo de cálculo: %v", s.ID, calculationInterval)
	log.Printf("   🪟  Sorter #%d: Ventana de tiempo: %v", s.ID, windowDuration)

	ticker := time.NewTicker(calculationInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			log.Printf("🛑 Sorter #%d: Deteniendo sistema de estadísticas de flujo", s.ID)
			return

		case <-ticker.C:
			s.limpiarVentana(windowDuration)
			stats := s.calcularFlowStatistics(windowDuration)
			s.actualizarLastFlowStats(stats)
			s.publicarFlowStatistics(stats)
			s.UpdateSKUs(s.assignedSKUs)
		}
	}
}

// registrarLectura registra una lectura exitosa en la ventana deslizante
func (s *Sorter) registrarLectura(sku string) {
	s.flowMutex.Lock()
	defer s.flowMutex.Unlock()

	s.lecturaRecords = append(s.lecturaRecords, models.LecturaRecord{
		SKU:       sku,
		Timestamp: time.Now(),
	})
}

// limpiarVentana elimina lecturas fuera de la ventana de tiempo
func (s *Sorter) limpiarVentana(windowDuration time.Duration) {
	s.flowMutex.Lock()
	defer s.flowMutex.Unlock()

	now := time.Now()
	cutoff := now.Add(-windowDuration)

	validIdx := 0
	for i, record := range s.lecturaRecords {
		if record.Timestamp.After(cutoff) {
			validIdx = i
			break
		}
	}

	if validIdx > 0 {
		s.lecturaRecords = s.lecturaRecords[validIdx:]
	}
}

// generateSKUID genera un ID numérico determinístico para un SKU
// usando el mismo algoritmo que models.SKU.GetNumericID()
func generateSKUID(sku string) int {
	h := fnv.New32a()
	_, _ = h.Write([]byte(sku))
	return int(h.Sum32() & 0x7FFFFFFF) // Máscara para asegurar valor positivo
}

// calcularFlowStatistics calcula las estadísticas de flujo para la ventana actual
func (s *Sorter) calcularFlowStatistics(windowDuration time.Duration) models.FlowStatistics {
	s.flowMutex.Lock()
	defer s.flowMutex.Unlock()

	skuCounts := make(map[string]int)
	totalLecturas := 0

	now := time.Now()
	cutoff := now.Add(-windowDuration)

	// Contar lecturas en la ventana
	for _, record := range s.lecturaRecords {
		if record.Timestamp.After(cutoff) {
			skuCounts[record.SKU]++
			totalLecturas++
		}
	}

	// Crear mapa con todas las SKUs asignadas (inicializadas en 0)
	// También crear mapa de SKU -> ID para incluir el ID en las stats
	allSKUs := make(map[string]int)
	skuIDMap := make(map[string]int)
	skuLineaMap := make(map[string]string)
	skusNeedingID := make([]string, 0) // SKUs que necesitan ID generado

	for _, skuAssignable := range s.assignedSKUs {
		allSKUs[skuAssignable.SKU] = 0
		skuIDMap[skuAssignable.SKU] = skuAssignable.ID
		skuLineaMap[skuAssignable.SKU] = skuAssignable.Linea
	}

	// Actualizar con los conteos reales
	for sku, count := range skuCounts {
		allSKUs[sku] = count

		// Si el SKU no está en el mapa de IDs, necesita uno
		if _, exists := skuIDMap[sku]; !exists {
			skusNeedingID = append(skusNeedingID, sku)
		}
	}

	// Generar IDs para SKUs nuevos y actualizar assignedSKUs
	if len(skusNeedingID) > 0 {
		skusUpdated := false
		for _, sku := range skusNeedingID {
			if sku == "REJECT" {
				skuIDMap[sku] = 0
				continue
			}

			skuID := generateSKUID(sku)
			skuIDMap[sku] = skuID
			log.Printf("⚠️  Sorter #%d: SKU '%s' no tiene ID asignado, generando ID determinístico: %d", s.ID, sku, skuID)

			// Buscar si ya existe en assignedSKUs
			found := false
			for i := range s.assignedSKUs {
				if s.assignedSKUs[i].SKU == sku {
					s.assignedSKUs[i].ID = skuID
					found = true
					skusUpdated = true
					break
				}
			}

			// Si no existe, agregar como nuevo SKU temporal (será persistido después)
			if !found {
				// Extraer componentes del SKU para crear entrada completa
				newSKU := models.SKUAssignable{
					ID:           skuID,
					SKU:          sku,
					Linea:        "", // Se actualizará en siguiente sync
					Percentage:   0.0,
					IsAssigned:   false,
					IsMasterCase: false,
				}
				s.assignedSKUs = append(s.assignedSKUs, newSKU)
				skusUpdated = true
				log.Printf("➕ Sorter #%d: SKU '%s' agregado temporalmente con ID=%d", s.ID, sku, skuID)
			}
		}

		// Actualizar solo si hubo cambios
		if skusUpdated {
			// Crear copia para evitar race conditions
			skusCopy := make([]models.SKUAssignable, len(s.assignedSKUs))
			copy(skusCopy, s.assignedSKUs)

			// Liberar mutex antes de llamar UpdateSKUs (que puede bloquearse)
			s.flowMutex.Unlock()
			s.UpdateSKUs(skusCopy)
			s.flowMutex.Lock()
		}
	}

	// Generar estadísticas para TODAS las SKUs asignadas
	stats := make([]models.SKUFlowStat, 0, len(allSKUs))

	for sku, count := range allSKUs {
		porcentaje := 0.0
		if totalLecturas > 0 {
			porcentajeRaw := float64(count) / float64(totalLecturas) * 100.0
			porcentaje = float64(int(porcentajeRaw + 0.5))
		}

		skuID := skuIDMap[sku]

		stats = append(stats, models.SKUFlowStat{
			ID:         skuID,
			SKU:        sku,
			Linea:      skuLineaMap[sku],
			Lecturas:   count,
			Porcentaje: porcentaje,
		})
	}

	return models.FlowStatistics{
		SorterID:      s.ID,
		Timestamp:     now,
		Stats:         stats,
		TotalLecturas: totalLecturas,
		WindowSeconds: int(windowDuration.Seconds()),
	}
}

// actualizarLastFlowStats actualiza el snapshot de porcentajes
func (s *Sorter) actualizarLastFlowStats(stats models.FlowStatistics) {
	s.flowMutex.Lock()
	defer s.flowMutex.Unlock()

	// Inicializar todas las SKUs asignadas en 0
	s.lastFlowStats = make(map[string]float64)
	for _, skuAssignable := range s.assignedSKUs {
		s.lastFlowStats[skuAssignable.SKU] = 0.0
	}

	// Actualizar con los porcentajes reales
	for _, stat := range stats.Stats {
		s.lastFlowStats[stat.SKU] = stat.Porcentaje
	}
}

// publicarFlowStatistics publica las estadísticas al canal
func (s *Sorter) publicarFlowStatistics(stats models.FlowStatistics) {
	select {
	case s.flowStatsChannel <- stats:
		log.Printf("📊 Sorter #%d: Flow stats publicadas (%d SKUs diferentes, %d lecturas totales)",
			s.ID, len(stats.Stats), stats.TotalLecturas)
	default:
		log.Printf("⚠️  Sorter #%d: Canal de flow stats lleno", s.ID)
	}
}

// GetSKUFlowPercentage retorna el porcentaje de flujo de un SKU
func (s *Sorter) GetSKUFlowPercentage(skuName string) float64 {
	s.flowMutex.RLock()
	defer s.flowMutex.RUnlock()

	if percentage, exists := s.lastFlowStats[skuName]; exists {
		return percentage
	}
	return 0.0
}
