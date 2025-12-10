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

// procesarEventosDataMatrix procesa eventos de lectura DataMatrix de Cognex principal (legacy)
func (s *Sorter) procesarEventosDataMatrix() {
	if s.Cognex == nil {
		return
	}

	log.Printf("📊 [Sorter #%d] Escuchando eventos DataMatrix de Cognex principal...", s.ID)

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

// procesarEventosDataMatrixCognex procesa eventos de una cámara DataMatrix específica
func (s *Sorter) procesarEventosDataMatrixCognex(cognexListener *listeners.CognexListener) {
	log.Printf("📊 [Sorter #%d] Escuchando eventos DataMatrix de Cognex #%d...", s.ID, cognexListener.GetID())

	for {
		select {
		case <-s.ctx.Done():
			log.Printf("🛑 [Sorter #%d] Deteniendo procesamiento DataMatrix Cognex #%d", s.ID, cognexListener.GetID())
			return

		case dmEvent, ok := <-cognexListener.DataMatrixChan:
			if !ok {
				log.Printf("⚠️  [Sorter #%d] Canal DataMatrix cerrado para Cognex #%d", s.ID, cognexListener.GetID())
				return
			}

			log.Printf("📥 [Sorter #%d] DataMatrix de Cognex #%d: %s", s.ID, cognexListener.GetID(), dmEvent.String())
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

	if err := s.sendPLCSignal(&salida); err != nil {
		log.Printf("❌ [Sorter #%d] Error crítico al asignar salida para caja %s (SKU: %s): %v",
			s.ID, evento.Correlativo, evento.SKU, err)
		// Registrar el error pero continuar para no bloquear el flujo
	}

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

	if err := s.sendPLCSignal(salida); err != nil {
		log.Printf("❌ [Sorter #%d] Error crítico al asignar salida para caja fallida %s: %v",
			s.ID, evento.Correlativo, err)
		// Registrar el error pero continuar para no bloquear el flujo
	}

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

// sendPLCSignal envía señal al PLC para activar una salida con reintentos automáticos
func (s *Sorter) sendPLCSignal(salida *shared.Salida) error {
	if s.plcManager == nil {
		log.Printf("⚠️  [Sorter #%d] sendPLCSignal: plcManager es nil, no se puede enviar señal", s.ID)
		return fmt.Errorf("plcManager no inicializado")
	}

	if salida.SealerPhysicalID <= 0 {
		log.Printf("⚠️  [Sorter #%d] sendPLCSignal: SealerPhysicalID inválido (%d) para salida ID=%d, no se envía señal PLC",
			s.ID, salida.SealerPhysicalID, salida.ID)
		return fmt.Errorf("SealerPhysicalID inválido para salida ID=%d", salida.ID)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	startTime := time.Now()
	destino := salida.SealerPhysicalID
	if salida.Tipo == "descarte" {
		log.Printf("⚠️  [Sorter #%d] Enviando señal PLC de DESCARTE → Salida %d (PhysicalID=%d)",
			s.ID, salida.ID, 0)
		destino = 0
	}

	// Intento inicial
	err := s.plcManager.AssignLaneToBox(ctx, s.ID, int16(destino))
	elapsed := time.Since(startTime)

	if err == nil {
		log.Printf("✅ [Sorter #%d] Señal PLC confirmada → Salida %d (PhysicalID=%d) en %v",
			s.ID, salida.ID, salida.SealerPhysicalID, elapsed)
		return nil
	}

	// ❌ Falló el primer intento
	log.Printf("❌ [Sorter #%d] Error al enviar señal PLC para salida %d (PhysicalID=%d) después de %v: %v",
		s.ID, salida.ID, salida.SealerPhysicalID, elapsed, err)

	// 🔄 SISTEMA DE REINTENTOS CON SALIDAS ALTERNATIVAS
	// Solo reintentar si hay múltiples salidas con el mismo SKU
	return s.retryWithAlternativeSalida(salida, err)
}

// retryWithAlternativeSalida intenta asignar la caja a una salida alternativa
func (s *Sorter) retryWithAlternativeSalida(salidaOriginal *shared.Salida, originalError error) error {
	// Obtener el SKU de la salida original
	if len(salidaOriginal.SKUs_Actuales) == 0 {
		log.Printf("⚠️  [Sorter #%d] Salida %d no tiene SKUs asignados, no se puede buscar alternativa",
			s.ID, salidaOriginal.ID)
		return originalError
	}

	sku := salidaOriginal.SKUs_Actuales[0].SKU
	maxRetries := 3
	excludedSalidas := []int{salidaOriginal.ID}

	log.Printf("🔄 [Sorter #%d] Iniciando búsqueda de salidas alternativas para SKU '%s' (intentos: %d)",
		s.ID, sku, maxRetries)

	for attempt := 1; attempt <= maxRetries; attempt++ {
		// Buscar salida alternativa, excluyendo las que ya fallaron
		salidaAlternativa := s.getAlternativeSalida(sku, excludedSalidas)

		if salidaAlternativa == nil {
			log.Printf("❌ [Sorter #%d] No hay más salidas alternativas disponibles para SKU '%s' (intento %d/%d)",
				s.ID, sku, attempt, maxRetries)
			break
		}

		log.Printf("🔄 [Sorter #%d] Intento %d/%d con salida alternativa ID=%d (PhysicalID=%d) para SKU '%s'",
			s.ID, attempt, maxRetries, salidaAlternativa.ID, salidaAlternativa.SealerPhysicalID, sku)

		// Intentar enviar señal a la salida alternativa
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		startTime := time.Now()

		destino := salidaAlternativa.SealerPhysicalID
		if salidaAlternativa.Tipo == "descarte" {
			destino = 0
		}

		err := s.plcManager.AssignLaneToBox(ctx, s.ID, int16(destino))
		elapsed := time.Since(startTime)
		cancel()

		if err == nil {
			log.Printf("✅ [Sorter #%d] Señal PLC confirmada con salida alternativa %d (PhysicalID=%d) en %v (intento %d/%d)",
				s.ID, salidaAlternativa.ID, salidaAlternativa.SealerPhysicalID, elapsed, attempt, maxRetries)
			return nil
		}

		// Falló también con esta salida
		log.Printf("❌ [Sorter #%d] Error con salida alternativa %d (PhysicalID=%d) después de %v: %v",
			s.ID, salidaAlternativa.ID, salidaAlternativa.SealerPhysicalID, elapsed, err)

		// Agregar a la lista de salidas excluidas
		excludedSalidas = append(excludedSalidas, salidaAlternativa.ID)

		// Pequeña pausa antes del siguiente intento
		time.Sleep(50 * time.Millisecond)
	}

	// Si llegamos aquí, todos los intentos fallaron
	log.Printf("❌ [Sorter #%d] FALLO CRÍTICO: No se pudo asignar caja para SKU '%s' después de %d intentos (salidas fallidas: %v)",
		s.ID, sku, maxRetries, excludedSalidas)

	// Retornar el error original
	return fmt.Errorf("fallo asignando caja para SKU '%s' después de %d intentos con salidas %v: %w",
		sku, maxRetries, excludedSalidas, originalError)
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
