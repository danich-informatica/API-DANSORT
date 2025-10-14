package sorter

import (
	"API-GREENEX/internal/db"
	"API-GREENEX/internal/listeners"
	"API-GREENEX/internal/models"
	"API-GREENEX/internal/shared"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"
)

// PublishLecturaEvent publica un evento de lectura individual procesada al WebSocket
func (s *Sorter) PublishLecturaEvent(evento models.LecturaEvent, salida *shared.Salida, exitoso bool) {
	if s.wsHub == nil {
		return
	}

	message := map[string]interface{}{
		"type":        "lectura_procesada",
		"qr_code":     evento.Mensaje,
		"sku":         evento.SKU,
		"salida_id":   salida.ID,
		"salida_name": salida.Salida_Sorter,
		"exitoso":     exitoso,
		"timestamp":   evento.Timestamp.Format(time.RFC3339),
		"sorter_id":   s.ID,
		"correlativo": evento.Correlativo,
		"calibre":     evento.Calibre,
		"variedad":    evento.Variedad,
		"embalaje":    evento.Embalaje,
	}

	jsonBytes, err := json.Marshal(message)
	if err != nil {
		log.Printf("❌ Error al serializar lectura_procesada: %v", err)
		return
	}

	roomName := fmt.Sprintf("assignment_%d", s.ID)
	s.wsHub.Broadcast <- &listeners.BroadcastMessage{
		RoomName: roomName,
		Message:  jsonBytes,
	}
}

// RegistrarSalidaCaja registra en la base de datos que una caja fue enviada a una salida física
func (s *Sorter) RegistrarSalidaCaja(correlativo string, salida *shared.Salida, sku, calibre string) error {
	if s.dbManager == nil {
		return fmt.Errorf("dbManager no inicializado")
	}

	pgManager, ok := s.dbManager.(*db.PostgresManager)
	if !ok {
		return fmt.Errorf("dbManager no es un PostgresManager válido")
	}

	ctx := context.Background()
	err := pgManager.InsertSalidaCaja(ctx, correlativo, salida.ID, salida.SealerPhysicalID)
	if err != nil {
		return fmt.Errorf("error al registrar salida de caja %s: %w", correlativo, err)
	}

	// 📡 Broadcast a WebSocket de historial
	s.PublishHistorialEvent(correlativo, sku, calibre, salida)

	return nil
}

// PublishHistorialEvent publica un evento de historial al WebSocket
func (s *Sorter) PublishHistorialEvent(correlativo, sku, calibre string, salida *shared.Salida) {
	if s.wsHub == nil {
		return
	}

	message := map[string]interface{}{
		"type":                "history_update",
		"box_id":              correlativo,
		"sku":                 sku,
		"caliber":             calibre,
		"sealer":              salida.SealerPhysicalID,
		"is_sealer_full_type": nil,
		"created_at":          time.Now().UTC().Format(time.RFC3339),
		"sorter_id":           s.ID,
	}

	jsonBytes, err := json.Marshal(message)
	if err != nil {
		log.Printf("❌ Error al serializar history_update: %v", err)
		return
	}

	roomName := fmt.Sprintf("history_sorter_%d", s.ID)
	s.wsHub.Broadcast <- &listeners.BroadcastMessage{
		RoomName: roomName,
		Message:  jsonBytes,
	}
}
