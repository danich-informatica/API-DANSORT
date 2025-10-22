package sorter

import (
	"API-GREENEX/internal/listeners"
	"API-GREENEX/internal/models"
	"API-GREENEX/internal/shared"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"
)

// procesarEventosCognex procesa eventos de lectura QR/SKU de Cognex
func (s *Sorter) procesarEventosCognex() {
	log.Println("👂 Sorter: Escuchando eventos QR/SKU de Cognex...")

	for {
		select {
		case <-s.ctx.Done():
			log.Println("🛑 Sorter: Deteniendo procesamiento de eventos QR/SKU")
			return

		case evento, ok := <-s.Cognex.EventChan:
			if !ok {
				log.Println("⚠️  Sorter: Canal de eventos QR/SKU cerrado")
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

// procesarEventosDataMatrix procesa eventos de lectura DataMatrix de Cognex (flujo separado)
func (s *Sorter) procesarEventosDataMatrix() {
	log.Printf("📊 [Sorter #%d] Escuchando eventos DataMatrix de Cognex...", s.ID)

	for {
		select {
		case <-s.ctx.Done():
			log.Printf("🛑 [Sorter #%d] Deteniendo procesamiento de eventos DataMatrix", s.ID)
			return

		case dmEvent, ok := <-s.Cognex.DataMatrixChan:
			if !ok {
				log.Printf("⚠️  [Sorter #%d] Canal de eventos DataMatrix cerrado", s.ID)
				return
			}

			log.Printf("📥 [Sorter #%d] DataMatrix recibido: %s", s.ID, dmEvent.String())
			s.processDataMatrixEvent(dmEvent)
		}
	}
}

// processDataMatrixEvent procesa un evento DataMatrix y lo distribuye a las salidas
func (s *Sorter) processDataMatrixEvent(dmEvent models.DataMatrixEvent) {
	// Buscar todas las salidas asociadas a este Cognex
	salidas := s.findSalidasByCognexID(dmEvent.CognexID)

	if len(salidas) == 0 {
		log.Printf("⚠️  [Sorter #%d] No hay salidas configuradas para Cognex #%d", s.ID, dmEvent.CognexID)
		return
	}

	log.Printf("🎯 [Sorter #%d] Distribuyendo DataMatrix a %d salida(s)", s.ID, len(salidas))

	// Enviar el evento a todas las salidas asociadas
	for _, salida := range salidas {
		go func(sal *shared.Salida) {
			numeroCaja, err := sal.ProcessDataMatrix(s.ctx, dmEvent.Codigo)
			if err != nil {
				log.Printf("❌ [Sorter #%d] Error procesando DataMatrix en Salida %d: %v", s.ID, sal.ID, err)
			} else {
				log.Printf("✅ [Sorter #%d] DataMatrix procesado en Salida %d (Caja #%d)", s.ID, sal.ID, numeroCaja)
				// Notificar vía WebSocket
				s.notifyDataMatrixRead(sal, dmEvent.Codigo, numeroCaja)
			}
		}(salida)
	}
}

// processLecturaExitosa procesa una lectura exitosa QR/SKU
func (s *Sorter) processLecturaExitosa(evento models.LecturaEvent) {
	s.LecturasExitosas++
	s.registrarLectura(evento.SKU)

	salida := s.determinarSalida(evento.SKU, evento.Calibre)
	log.Printf("✅ Sorter #%d: Lectura #%d | SKU: %s | Salida: %s (ID: %d) | Razón: sort por SKU",
		s.ID, s.LecturasExitosas, evento.SKU, salida.Salida_Sorter, salida.ID)

	s.sendPLCSignal(&salida)
	//plcManager.CallMethod(ctx, sorterID, objectID, methodID, inputArgs)
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

	// Protección contra salida nil
	if salida == nil {
		log.Printf("❌ Sorter #%d: Fallo #%d | SKU: %s | Sin salida disponible | Razón: %s | %s",
			s.ID, s.LecturasFallidas, evento.SKU, razon, evento.String())
		return
	}

	log.Printf("❌ Sorter #%d: Fallo #%d | SKU: %s | Salida: %s (ID: %d) | Razón: %s | %s",
		s.ID, s.LecturasFallidas, evento.SKU, salida.Salida_Sorter, salida.ID, razon, evento.String())

	s.sendPLCSignal(salida)
	s.PublishLecturaEvent(evento, salida, false)

	if err := s.RegistrarSalidaCaja(evento.Correlativo, salida, evento.SKU, evento.Calibre); err != nil {
		log.Printf("⚠️  Sorter #%d: Error al registrar salida de caja fallida %s: %v", s.ID, evento.Correlativo, err)
	}
}

// getSalidaForFallo obtiene la salida y razón para un fallo
func (s *Sorter) getSalidaForFallo(tipoLectura models.TipoLectura) (salida *shared.Salida, razon string) {
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
		salida = salidaPtr
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

// findSalidasByCognexID busca todas las salidas asociadas a un Cognex
func (s *Sorter) findSalidasByCognexID(cognexID int) []*shared.Salida {
	var result []*shared.Salida

	for i := range s.Salidas {
		if s.Salidas[i].CognexID == cognexID {
			result = append(result, &s.Salidas[i])
		}
	}

	return result
}

// notifyDataMatrixRead notifica al frontend via WebSocket
func (s *Sorter) notifyDataMatrixRead(salida *shared.Salida, correlativo string, numeroCaja int) {
	if s.wsHub == nil {
		return
	}

	// Crear mensaje para WebSocket
	message := map[string]interface{}{
		"type":      "datamatrix_read",
		"timestamp": time.Now().Format(time.RFC3339),
		"sorter_id": s.ID,
		"data": map[string]interface{}{
			"salida_id":     salida.ID,
			"salida_fisica": salida.SealerPhysicalID,
			"correlativo":   correlativo,
			"numero_caja":   numeroCaja,
			"fecha_lectura": time.Now().Format(time.RFC3339),
		},
	}

	// Encodear como JSON
	jsonData, err := json.Marshal(message)
	if err != nil {
		log.Printf("❌ Error al serializar mensaje WebSocket: %v", err)
		return
	}

	roomName := fmt.Sprintf("assignment_%d", s.ID)
	s.wsHub.Broadcast <- &listeners.BroadcastMessage{
		RoomName: roomName,
		Message:  jsonData,
	}
	log.Printf("📤 [Sorter #%d] WebSocket: datamatrix_read → room %s", s.ID, roomName)
}
