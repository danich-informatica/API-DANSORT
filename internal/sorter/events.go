package sorter

import (
	"API-GREENEX/internal/models"
	"API-GREENEX/internal/shared"
	"context"
	"log"
	"time"
)

// procesarEventosCognex procesa eventos de lectura de Cognex
func (s *Sorter) procesarEventosCognex() {
	log.Println("👂 Sorter: Escuchando eventos de Cognex...")

	for {
		select {
		case <-s.ctx.Done():
			log.Println("🛑 Sorter: Deteniendo procesamiento de eventos")
			return

		case evento, ok := <-s.Cognex.EventChan:
			if !ok {
				log.Println("⚠️  Sorter: Canal de eventos cerrado")
				return
			}

			if evento.Exitoso {
				s.processLecturaExitosa(evento)
			} else {
				s.processLecturaFallida(evento)
			}

			s.showStatsIfNeeded()
		}
	}
}

// processLecturaExitosa procesa una lectura exitosa
func (s *Sorter) processLecturaExitosa(evento models.LecturaEvent) {
	s.LecturasExitosas++
	s.registrarLectura(evento.SKU)

	salida := s.determinarSalida(evento.SKU, evento.Calibre)
	log.Printf("✅ Sorter #%d: Lectura #%d | SKU: %s | Salida: %s (ID: %d) | Razón: sort por SKU",
		s.ID, s.LecturasExitosas, evento.SKU, salida.Salida_Sorter, salida.ID)

	s.sendPLCSignal(&salida)
	s.PublishLecturaEvent(evento, &salida, true)

	if err := s.RegistrarSalidaCaja(evento.Correlativo, &salida, evento.SKU, evento.Calibre); err != nil {
		log.Printf("⚠️  Sorter #%d: Error al registrar salida de caja %s: %v", s.ID, evento.Correlativo, err)
	}
}

// processLecturaFallida procesa una lectura fallida
func (s *Sorter) processLecturaFallida(evento models.LecturaEvent) {
	s.LecturasFallidas++

	tipoLectura := evento.GetTipo()
	salida, razon := s.getSalidaForFallo(tipoLectura)

	log.Printf("❌ Sorter #%d: Fallo #%d | SKU: %s | Salida: %s (ID: %d) | Razón: %s | %s",
		s.ID, s.LecturasFallidas, evento.SKU, salida.Salida_Sorter, salida.ID, razon, evento.String())

	s.sendPLCSignal(&salida)
	s.PublishLecturaEvent(evento, &salida, false)

	if err := s.RegistrarSalidaCaja(evento.Correlativo, &salida, evento.SKU, evento.Calibre); err != nil {
		log.Printf("⚠️  Sorter #%d: Error al registrar salida de caja fallida %s: %v", s.ID, evento.Correlativo, err)
	}
}

// getSalidaForFallo obtiene la salida y razón para un fallo
func (s *Sorter) getSalidaForFallo(tipoLectura models.TipoLectura) (salida shared.Salida, razon string) {
	var salidaPtr *shared.Salida

	switch tipoLectura {
	case models.LecturaNoRead:
		salidaPtr = s.GetDiscardSalida()
		razon = "sort por descarte (NO_READ)"
	case models.LecturaFormato, models.LecturaSKU:
		salidaPtr = s.GetDiscardSalida()
		razon = "sort por descarte (formato/SKU inválido)"
	case models.LecturaDB:
		razon = "error de base de datos"
	}

	if salidaPtr != nil {
		salida = *salidaPtr
	}

	return salida, razon
}

// sendPLCSignal envía señal al PLC para activar una salida
func (s *Sorter) sendPLCSignal(salida *shared.Salida) {
	if s.plcManager == nil || salida.SealerPhysicalID <= 0 {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := s.plcManager.AssignLaneToBox(ctx, s.ID, int16(salida.SealerPhysicalID)); err != nil {
		log.Printf("⚠️  [Sorter #%d] Error al enviar señal PLC para salida %d (PhysicalID=%d): %v",
			s.ID, salida.ID, salida.SealerPhysicalID, err)
	} else {
		log.Printf("📤 [Sorter #%d] Señal PLC enviada → Salida %d (PhysicalID=%d)",
			s.ID, salida.ID, salida.SealerPhysicalID)
	}
}

// showStatsIfNeeded muestra estadísticas cada 10 lecturas
func (s *Sorter) showStatsIfNeeded() {
	total := s.LecturasExitosas + s.LecturasFallidas
	if total%10 == 0 && total > 0 {
		tasaExito := float64(s.LecturasExitosas) / float64(total) * 100
		log.Printf("📊 Sorter #%d: Stats: Total=%d | Exitosas=%d | Fallidas=%d | Tasa=%.1f%%",
			s.ID, total, s.LecturasExitosas, s.LecturasFallidas, tasaExito)
	}
}
