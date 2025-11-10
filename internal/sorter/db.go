package sorter

import (
	"api-dansort/internal/db"
	"api-dansort/internal/listeners"
	"api-dansort/internal/models"
	"api-dansort/internal/shared"
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

	// Determinar si la salida final estaba realmente disponible. Si la salida destino
	// (salida) no está disponible porque las otras asignadas estaban llenas, debemos
	// marcar la salida "intended" como llena y registrar la caja apuntando a la salida
	// donde terminó (p.ej. salida de rechazo).

	// Por defecto asumimos que no estuvo llena la salida original.
	llena := false

	// Si la salida actual no está disponible, intentamos encontrar la salida configurada
	// para el SKU (la que originalmente debería haberla recibido). Si existe y es distinta
	// de la salida final, marcaremos esa intendedSalida como llena.
	intendedSalidaID := -1
	if !salida.IsAvailable() {
		// Buscar la salida configurada para el SKU
		for _, so := range s.Salidas {
			for _, skuCfg := range so.SKUs_Actuales {
				if skuCfg.SKU == sku {
					intendedSalidaID = so.ID
					break
				}
			}
			if intendedSalidaID != -1 {
				break
			}
		}

		if intendedSalidaID != -1 && intendedSalidaID != salida.ID {
			// La caja terminó en 'salida' (p.ej. REJECT) pero originalmente correspondía a intendedSalidaID
			llena = true
			// Buscar el SealerPhysicalID de la intendedSalida
			sealerPhysical := 0
			for _, so := range s.Salidas {
				if so.ID == intendedSalidaID {
					sealerPhysical = so.SealerPhysicalID
					break
				}
			}

			// Registrar un record indicando que la intended salida estuvo llena y la caja fue redirigida
			// InsertSalidaCaja guarda el campo 'llena' y el id_salida que representa dónde debería haber ido
			if sealerPhysical == 0 {
				log.Printf("⚠️  No se encontró SealerPhysicalID para intendedSalida %d (caja %s)", intendedSalidaID, correlativo)
			}
			if err := pgManager.InsertSalidaCaja(ctx, correlativo, intendedSalidaID, sealerPhysical, true); err != nil {
				log.Printf("⚠️  Error al marcar intended salida como llena para caja %s: %v", correlativo, err)
			}
		}
	}

	// Finalmente insertar el registro real donde la caja fue enviada
	err := pgManager.InsertSalidaCaja(ctx, correlativo, salida.ID, salida.SealerPhysicalID, llena)
	if err != nil {
		return fmt.Errorf("error al registrar salida de caja %s: %w", correlativo, err)
	}

	// 📡 Broadcast a WebSocket de historial (pasamos el flag 'llena')
	s.PublishHistorialEvent(correlativo, sku, calibre, salida, llena)

	return nil
}

// PublishHistorialEvent publica un evento de historial al WebSocket
func (s *Sorter) PublishHistorialEvent(correlativo, sku, calibre string, salida *shared.Salida, isFull bool) {
	if s.wsHub == nil {
		return
	}

	message := map[string]interface{}{
		"type":                "history_update",
		"box_id":              correlativo,
		"sku":                 sku,
		"caliber":             calibre,
		"sealer":              salida.SealerPhysicalID,
		"is_sealer_full_type": isFull,
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

// ReloadSalidasFromDB recarga las asignaciones de SKUs desde PostgreSQL
// para mantener sincronizadas las salidas después de un sync de SKUs
func (s *Sorter) ReloadSalidasFromDB(ctx context.Context) error {
	if s.dbManager == nil {
		return fmt.Errorf("dbManager no inicializado")
	}

	pgManager, ok := s.dbManager.(*db.PostgresManager)
	if !ok {
		return fmt.Errorf("dbManager no es un PostgresManager válido")
	}

	// Cargar SKUs asignadas desde la base de datos
	skusBySalida, err := pgManager.LoadAssignedSKUsForSorter(ctx, s.ID)
	if err != nil {
		return fmt.Errorf("error al cargar SKUs desde BD: %w", err)
	}

	// Actualizar solo las SKUs en cada salida (sin recrear las salidas)
	updatedCount := 0
	for i := range s.Salidas {
		salidaID := s.Salidas[i].ID

		if skus, existe := skusBySalida[salidaID]; existe {
			s.Salidas[i].SKUs_Actuales = skus
			updatedCount += len(skus)
		} else {
			// Si no hay SKUs asignadas para esta salida, vaciar la lista
			s.Salidas[i].SKUs_Actuales = []models.SKU{}
		}
	}

	log.Printf("🔄 Sorter #%d: Salidas recargadas desde BD (%d SKUs totales)", s.ID, updatedCount)

	// CRÍTICO: Notificar al WebSocket que las asignaciones cambiaron
	// Esto asegura que el frontend vea los cambios en tiempo real
	select {
	case s.skuChannel <- s.assignedSKUs:
		log.Printf("📡 Sorter #%d: Notificación de asignaciones enviada al WebSocket", s.ID)
	default:
		log.Printf("⚠️  Sorter #%d: Canal WebSocket lleno, notificación omitida", s.ID)
	}

	return nil
}
