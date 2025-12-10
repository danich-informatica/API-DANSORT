package listeners

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"

	"API-GREENEX/internal/models"
	. "API-GREENEX/internal/shared"
)

// WebSocketMessage representa un mensaje enviado a través del WebSocket
type WebSocketMessage struct {
	Type      string      `json:"type"`      // "assignment_added", "assignment_removed", "assignments_cleared", "box_status"
	Timestamp string      `json:"timestamp"` // ISO 8601 timestamp
	SorterID  int         `json:"sorter_id"`
	SealerID  int         `json:"sealer_id"`
	Data      interface{} `json:"data"` // Datos específicos del evento
}

// BoxStatusData representa las estadísticas de estados de cajas para enviar al frontend
type BoxStatusData struct {
	SalidaID         int                `json:"salida_id"`
	SalidaPhysicalID int                `json:"salida_physical_id"`
	EstadoActual     interface{}        `json:"estado_actual"` // EstadoCaja
	Ultimos5         []interface{}      `json:"ultimos_5"`     // []EstadoCaja
	Contadores       map[string]int     `json:"contadores"`    // map[TipoEstadoCaja]int
	Porcentajes      map[string]float64 `json:"porcentajes"`   // map[TipoEstadoCaja]float64
	TotalCajas       int                `json:"total_cajas"`   // Total de cajas procesadas
}

// Client representa un cliente WebSocket conectado
type Client struct {
	ID       string
	Conn     *websocket.Conn
	RoomName string // "assignment_1", "assignment_2", etc.
	Send     chan []byte
	Hub      *WebSocketHub
}

// WebSocketHub maneja todas las conexiones WebSocket y las rooms
type WebSocketHub struct {
	// Rooms organiza clientes por nombre de room (ej: "assignment_1")
	Rooms map[string]map[*Client]bool

	// Canales de comunicación
	Register   chan *Client
	Unregister chan *Client
	Broadcast  chan *BroadcastMessage

	mu sync.RWMutex
}

// BroadcastMessage contiene el mensaje y el nombre de la room objetivo
type BroadcastMessage struct {
	RoomName string
	Message  []byte
}

// Upgrader de HTTP a WebSocket
var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		// En producción, verificar origen
		return true // Permitir todos los orígenes (desarrollo)
	},
}

// NewWebSocketHub crea un nuevo hub de WebSocket
func NewWebSocketHub() *WebSocketHub {
	return &WebSocketHub{
		Rooms:      make(map[string]map[*Client]bool),
		Register:   make(chan *Client, 10),
		Unregister: make(chan *Client, 10),
		Broadcast:  make(chan *BroadcastMessage, 100),
	}
}

// Run inicia el hub de WebSocket (debe ejecutarse en goroutine)
func (h *WebSocketHub) Run() {
	log.Println("🔌 WebSocket Hub iniciado")

	for {
		select {
		case client := <-h.Register:
			h.mu.Lock()
			// Crear room si no existe
			if h.Rooms[client.RoomName] == nil {
				h.Rooms[client.RoomName] = make(map[*Client]bool)
				log.Printf("📦 Room creada: %s", client.RoomName)
			}
			h.Rooms[client.RoomName][client] = true
			h.mu.Unlock()
			log.Printf("✅ Cliente %s conectado a room %s (Total: %d)",
				client.ID, client.RoomName, len(h.Rooms[client.RoomName]))

		case client := <-h.Unregister:
			h.mu.Lock()
			if clients, ok := h.Rooms[client.RoomName]; ok {
				if _, exists := clients[client]; exists {
					delete(clients, client)
					close(client.Send)
					log.Printf("❌ Cliente %s desconectado de room %s (Restantes: %d)",
						client.ID, client.RoomName, len(clients))

					// Eliminar room si está vacía
					if len(clients) == 0 {
						delete(h.Rooms, client.RoomName)
						log.Printf("🗑️  Room %s eliminada (vacía)", client.RoomName)
					}
				}
			}
			h.mu.Unlock()

		case message := <-h.Broadcast:
			h.mu.RLock()
			clients := h.Rooms[message.RoomName]
			h.mu.RUnlock()

			sentCount := 0
			for client := range clients {
				select {
				case client.Send <- message.Message:
					sentCount++
				default:
					// Canal lleno, desconectar cliente
					log.Printf("⚠️  Canal lleno para cliente %s, desconectando", client.ID)
					h.Unregister <- client
				}
			}

			if sentCount > 0 {
				log.Printf("✅ Mensaje enviado a %d cliente(s) en room %s", sentCount, message.RoomName)
			} else {
				log.Printf("⚠️  Mensaje NO enviado - room %s sin clientes", message.RoomName)
			}
		}
	}
}

// CreateRoomsForSorters crea rooms para cada sorter
func (h *WebSocketHub) CreateRoomsForSorters(sorterIDs []int) {
	h.mu.Lock()
	defer h.mu.Unlock()

	for _, sorterID := range sorterIDs {
		roomName := fmt.Sprintf("assignment_%d", sorterID)
		if h.Rooms[roomName] == nil {
			h.Rooms[roomName] = make(map[*Client]bool)
			log.Printf("📦 Room pre-creada: %s", roomName)
		}
	}
}

// NotifySKUAssigned notifica el estado actual de asignaciones siguiendo la estructura de /sku/assigned
func (h *WebSocketHub) NotifySKUAssigned(sorter SorterInterface) {
	sorterID := sorter.GetID()
	roomName := fmt.Sprintf("assignment_%d", sorterID)

	var result []map[string]interface{}
	for _, salida := range sorter.GetSalidas() {
		assignments := []map[string]interface{}{}
		for _, sku := range salida.SKUs_Actuales {
			assignments = append(assignments, map[string]interface{}{
				"sku_id":         sku.GetNumericID(),
				"sku_name":       sku.SKU,
				"is_master_case": false,
			})
		}
		result = append(result, map[string]interface{}{
			"sealer_db_id":       salida.ID,
			"sealer_physical_id": salida.SealerPhysicalID,
			"sorter_id":          sorter.GetID(),
			"sorter_name":        "",
			"mode":               salida.Tipo,
			"assignments":        assignments,
			"is_enabled":         salida.IsEnabled,
		})
	}

	message := WebSocketMessage{
		Type:      "sku_assigned",
		Timestamp: time.Now().Format(time.RFC3339),
		SorterID:  sorterID,
		SealerID:  0, // No aplica
		Data:      result,
	}

	h.sendMessageToRoom(roomName, message)
	log.Printf("📤 [WS] sku_assigned → room %s (%d salidas)", roomName, len(result))
}

// SubscribeToSorter suscribe el hub al canal de SKUs de un sorter
// Escucha cambios y hace broadcast automático a la room correspondiente
func (h *WebSocketHub) SubscribeToSorter(sorter SorterInterface) {
	// Suscripción al canal de SKUs
	go func() {
		sorterID := sorter.GetID()
		channel := sorter.GetSKUChannel()

		log.Printf("🔔 WebSocket suscrito al canal de SKUs del Sorter #%d", sorterID)

		// Escuchar canal indefinidamente
		for range channel {
			// Cuando el sorter publica SKUs actualizadas, notificar
			h.NotifySKUAssigned(sorter)
		}

		log.Printf("⚠️  Canal de SKUs del Sorter #%d cerrado, suscripción terminada", sorterID)
	}()

	// Suscripción al canal de flow statistics
	go func() {
		sorterID := sorter.GetID()
		channel := sorter.GetFlowStatsChannel()

		log.Printf("📊 WebSocket suscrito al canal de Flow Stats del Sorter #%d", sorterID)

		// Escuchar canal indefinidamente
		for stats := range channel {
			// Cuando el sorter publica estadísticas, notificar
			h.NotifyFlowStatistics(stats)
		}

		log.Printf("⚠️  Canal de Flow Stats del Sorter #%d cerrado, suscripción terminada", sorterID)
	}()
}

// NotifyFlowStatistics notifica las estadísticas de flujo a los clientes
func (h *WebSocketHub) NotifyFlowStatistics(stats models.FlowStatistics) {
	roomName := fmt.Sprintf("assignment_%d", stats.SorterID)

	message := WebSocketMessage{
		Type:      "sku_flow_stats",
		Timestamp: stats.Timestamp.Format(time.RFC3339),
		SorterID:  stats.SorterID,
		SealerID:  0, // No aplica
		Data:      stats,
	}

	h.sendMessageToRoom(roomName, message)
	log.Printf("📤 [WS] sku_flow_stats → room %s (%d lecturas totales, %d SKUs diferentes, ventana: %ds)",
		roomName, stats.TotalLecturas, len(stats.Stats), stats.WindowSeconds)
}

// NotifyBoxStatus notifica las estadísticas de estados de cajas a los clientes
// Ahora espera un array de salidas en lugar de una sola salida
func (h *WebSocketHub) NotifyBoxStatus(sorterID int, salidas []map[string]interface{}) {
	roomName := fmt.Sprintf("assignment_%d", sorterID)

	message := WebSocketMessage{
		Type:      "box_status",
		Timestamp: time.Now().Format(time.RFC3339),
		SorterID:  sorterID,
		Data:      salidas, // Array de salidas con sus estadísticas
	}

	h.sendMessageToRoom(roomName, message)

	// Log con información consolidada
	log.Printf("📤 [WS] box_status → room %s (%d salidas automáticas)",
		roomName, len(salidas))
}

// sendMessageToRoom envía un mensaje a todos los clientes de una room
func (h *WebSocketHub) sendMessageToRoom(roomName string, message WebSocketMessage) {
	jsonData, err := json.Marshal(message)
	if err != nil {
		log.Printf("❌ Error al serializar mensaje WebSocket: %v", err)
		return
	}

	// Log: verificar cuántos clientes hay en la room
	h.mu.RLock()
	clientCount := len(h.Rooms[roomName])
	h.mu.RUnlock()

	log.Printf("📡 Enviando mensaje a room %s (%d clientes conectados)", roomName, clientCount)

	h.Broadcast <- &BroadcastMessage{
		RoomName: roomName,
		Message:  jsonData,
	}

	log.Printf("📤 Mensaje enviado a room %s: type=%s", roomName, message.Type)
}

// GetRoomStats retorna estadísticas de las rooms
func (h *WebSocketHub) GetRoomStats() map[string]int {
	h.mu.RLock()
	defer h.mu.RUnlock()

	stats := make(map[string]int)
	for roomName, clients := range h.Rooms {
		stats[roomName] = len(clients)
	}
	return stats
}

// readPump lee mensajes del cliente WebSocket
func (c *Client) readPump() {
	defer func() {
		c.Hub.Unregister <- c
		c.Conn.Close()
	}()

	c.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.Conn.SetPongHandler(func(string) error {
		c.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, message, err := c.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("⚠️  Error de lectura WebSocket: %v", err)
			}
			break
		}

		// Procesar mensajes del cliente (ej: ping/pong, comandos)
		log.Printf("📨 Mensaje recibido de cliente %s: %s", c.ID, string(message))
	}
}

// writePump escribe mensajes al cliente WebSocket
func (c *Client) writePump() {
	ticker := time.NewTicker(54 * time.Second)
	defer func() {
		ticker.Stop()
		c.Conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.Send:
			c.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				// Hub cerró el canal
				c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.Conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			// Agregar mensajes en cola al frame actual
			n := len(c.Send)
			for i := 0; i < n; i++ {
				w.Write([]byte{'\n'})
				w.Write(<-c.Send)
			}

			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			c.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// HandleWebSocketConnection maneja una nueva conexión WebSocket
func HandleWebSocketConnection(hub *WebSocketHub) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Obtener room del parámetro de ruta
		roomName := c.Param("room")
		if roomName == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "room parameter is required",
			})
			return
		}

		// Validar formato de room. Aceptamos dos formatos:
		//  - assignment_{id}
		//  - history_sorter_{id}
		var sorterID int
		if _, err := fmt.Sscanf(roomName, "assignment_%d", &sorterID); err != nil {
			// intentar historial
			if _, err2 := fmt.Sscanf(roomName, "history_sorter_%d", &sorterID); err2 != nil {
				c.JSON(http.StatusBadRequest, gin.H{
					"error": "invalid room format, expected: assignment_N or history_sorter_N",
				})
				return
			}
		}

		// Upgrade HTTP a WebSocket
		conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			log.Printf("❌ Error al hacer upgrade WebSocket: %v", err)
			return
		}

		// Crear cliente
		clientID := fmt.Sprintf("%s_%d", c.ClientIP(), time.Now().UnixNano())
		client := &Client{
			ID:       clientID,
			Conn:     conn,
			RoomName: roomName,
			Send:     make(chan []byte, 256),
			Hub:      hub,
		}

		// Registrar cliente
		client.Hub.Register <- client

		// Iniciar pumps en goroutines
		go client.writePump()
		go client.readPump()

		log.Printf("🔌 Cliente WebSocket conectado: %s → %s", clientID, roomName)
	}
}

// SetupWebSocketRoutes configura las rutas de WebSocket en el router
func SetupWebSocketRoutes(router *gin.Engine, hub *WebSocketHub) {
	// WebSocket endpoint: ws://host/ws/assignment_1
	router.GET("/ws/:room", HandleWebSocketConnection(hub))

	// Endpoint REST para estadísticas de WebSocket
	router.GET("/ws/stats", func(c *gin.Context) {
		stats := hub.GetRoomStats()
		c.JSON(http.StatusOK, gin.H{
			"rooms":       stats,
			"total_rooms": len(stats),
			"total_clients": func() int {
				total := 0
				for _, count := range stats {
					total += count
				}
				return total
			}(),
		})
	})
}
