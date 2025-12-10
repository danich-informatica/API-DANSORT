package sorter

import (
	"API-DANSORT/internal/shared"
	"log"
	"sync"
	"time"
)

// StartBoxStatusAggregator inicia un worker que recopila los estados de todas
// las salidas automáticas del sorter y publica un único evento consolidado
func (s *Sorter) StartBoxStatusAggregator(publishInterval time.Duration) {
	log.Printf("📊 [Sorter %d] Iniciando agregador de estados de cajas (intervalo: %v)", s.ID, publishInterval)

	// Crear channels para cada salida automática y escuchar sus eventos
	var automaticSalidas []*shared.Salida
	for i := range s.Salidas {

		if s.Salidas[i].Tipo == "automatico" {
			automaticSalidas = append(automaticSalidas, &s.Salidas[i])
		}
	}

	if len(automaticSalidas) == 0 {
		log.Printf("⚠️  [Sorter %d] No hay salidas automáticas, agregador no iniciado", s.ID)
		return
	}

	log.Printf("📋 [Sorter %d] Monitoreando %d salida(s) automática(s)", s.ID, len(automaticSalidas))

	// Iniciar un ticker para publicar estadísticas periódicamente
	ticker := time.NewTicker(publishInterval)
	defer ticker.Stop()

	// Mutex para sincronizar actualizaciones
	var mu sync.RWMutex
	needsUpdate := false

	// Escuchar eventos de cada salida en goroutines separadas
	for _, salida := range automaticSalidas {
		go func(sal *shared.Salida) {
			for range sal.EstadosCajasChan {
				// Marcar que hay una actualización disponible
				mu.Lock()
				needsUpdate = true
				mu.Unlock()
			}
		}(salida)
	}

	// Worker principal que publica periódicamente
	go func() {
		for {
			select {
			case <-ticker.C:
				mu.RLock()
				hasUpdate := needsUpdate
				mu.RUnlock()

				if !hasUpdate {
					continue // No hay cambios, saltar publicación
				}

				// Recopilar estadísticas de todas las salidas
				salidas := make([]map[string]interface{}, 0, len(automaticSalidas))

				for _, salida := range automaticSalidas {
					estadoActual, ultimos5, contadores, porcentajes, totalCajas := salida.CalcularEstadisticasParaWebSocket()

					// Convertir contadores y porcentajes a map[string]interface{}
					contadoresJSON := make(map[string]int)
					for k, v := range contadores {
						contadoresJSON[string(k)] = v
					}

					porcentajesJSON := make(map[string]float64)
					for k, v := range porcentajes {
						porcentajesJSON[string(k)] = v
					}

					// Crear estructura de datos para esta salida
					salidaData := map[string]interface{}{
						"salida_id":          salida.ID,
						"salida_physical_id": salida.SealerPhysicalID,
						"estado_actual":      estadoActual,
						"ultimos_5":          ultimos5,
						"contadores":         contadoresJSON,
						"porcentajes":        porcentajesJSON,
						"total_cajas":        totalCajas,
					}

					salidas = append(salidas, salidaData)
				}

				// Publicar evento consolidado a través del WebSocket Hub
				if s.wsHub != nil {
					s.wsHub.NotifyBoxStatus(s.ID, salidas)
					log.Printf("📤 [Sorter %d] Estadísticas consolidadas publicadas (%d salidas)", s.ID, len(salidas))
				}

				// Resetear flag de actualización
				mu.Lock()
				needsUpdate = false
				mu.Unlock()

			case <-s.ctx.Done():
				log.Printf("⚠️  [Sorter %d] Agregador de estados detenido", s.ID)
				return
			}
		}
	}()
}
