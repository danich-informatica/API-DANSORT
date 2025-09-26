package flow

import (
	"fmt"
	"log"
	"time"

	"API-GREENEX/internal/listeners"
	"API-GREENEX/internal/models"
)

type SubscriptionManager struct {
	isRunning bool
	dataChan  chan SubscriptionData
}

// Estructura para datos de suscripción
type SubscriptionData struct {
	SubscriptionName string
	NodeID           string
	Data             *listeners.NodeData
	ReceivedAt       time.Time
}

// NewSubscriptionManager crea una nueva instancia del manager de suscripciones
func NewSubscriptionManager() *SubscriptionManager {
    return &SubscriptionManager{
        isRunning: false,
        dataChan:  make(chan SubscriptionData, 100),
    }
}

// Start inicia el manager de suscripciones
func (sm *SubscriptionManager) Start() {
    log.Println("Iniciando Subscription Manager...")
    sm.isRunning = true
    
    // Procesar datos de suscripciones
    go sm.processSubscriptionData()
    
    // Mantener el manager corriendo
    for sm.isRunning {
        time.Sleep(1 * time.Second)
    }
}

// processSubscriptionData procesa los datos de suscripciones
func (sm *SubscriptionManager) processSubscriptionData() {
    for sm.isRunning {
        select {
        case data := <-sm.dataChan:
            sm.handleSubscriptionData(data)
        case <-time.After(100 * time.Millisecond):
            // Timeout para permitir verificar isRunning
        }
    }
}

// handleSubscriptionData maneja los datos de suscripciones
func (sm *SubscriptionManager) handleSubscriptionData(data SubscriptionData) {
    // Convertir StatusCode a string legible
    qualityStr := sm.statusCodeToString(data.Data.Quality)
    
    // Log detallado de la comunicación de suscripción
    log.Printf("╔═══ SUSCRIPCIÓN OPC UA ═══╗")
    log.Printf("║ Suscripción: %-12s ║", data.SubscriptionName)
    log.Printf("║ NodeID: %-17s ║", data.NodeID)
    log.Printf("║ Valor: %-18v ║", data.Data.Value)
    log.Printf("║ Timestamp: %-14s ║", data.Data.Timestamp.Format("15:04:05.000"))
    log.Printf("║ Calidad: %-16s ║", qualityStr)
    log.Printf("║ Recibido: %-15s ║", data.ReceivedAt.Format("15:04:05.000"))
    log.Printf("╚═══════════════════════════╝")
    
    // Aquí puedes agregar más lógica específica para suscripciones:
    sm.processSubscriptionLogic(data)
}

// processSubscriptionLogic procesa la lógica específica de suscripciones
func (sm *SubscriptionManager) processSubscriptionLogic(data SubscriptionData) {
	// Lógica específica para suscripciones usando constantes
	switch data.SubscriptionName {
	case models.DEFAULT_SUBSCRIPTION:
		sm.handleDefaultSubscription(data)
	case models.HEARTBEAT_SUBSCRIPTION:
		sm.handleHeartbeatSubscription(data)
	case models.SEGREGATION_SUBSCRIPTION:
		sm.handleSegregationSubscription(data)
	default:
		sm.handleGenericSubscription(data)
	}
}

// handleDefaultSubscription maneja suscripciones por defecto
func (sm *SubscriptionManager) handleDefaultSubscription(data SubscriptionData) {
	log.Printf("🔄 Procesando suscripción por defecto: %s = %v", data.NodeID, data.Data.Value)

	// Identificar el tipo de nodo basado en las constantes
	switch data.NodeID {
	case models.DEFAULT_SEGREGATION_NODE:
		sm.handleSegregationValue(data)
	case models.DEFAULT_HEARTBEAT_NODE:
		sm.handleHeartbeatValue(data)
	default:
		log.Printf("📝 Nodo desconocido en suscripción por defecto: %s", data.NodeID)
	}
}

// handleHeartbeatSubscription maneja suscripciones de heartbeat
func (sm *SubscriptionManager) handleHeartbeatSubscription(data SubscriptionData) {
    log.Printf("💓 Procesando heartbeat: %s = %v", data.NodeID, data.Data.Value)
    
    // Lógica específica para heartbeat
    if value, ok := data.Data.Value.(bool); ok {
        if value {
            log.Printf("✅ Sistema activo - Heartbeat OK")
        } else {
            log.Printf("⚠️  ADVERTENCIA: Heartbeat inactivo")
        }
    }
}

// handleSegregationSubscription maneja suscripciones de segregation
func (sm *SubscriptionManager) handleSegregationSubscription(data SubscriptionData) {
    log.Printf("🔄 Procesando segregación: %s = %v", data.NodeID, data.Data.Value)
    
    // Lógica específica para segregation
    if value, ok := data.Data.Value.(int32); ok {
        switch value {
        case 0:
            log.Printf("🔴 Segregación detenida")
        case 1:
            log.Printf("🟢 Segregación en proceso")
        case 2:
            log.Printf("🟡 Segregación pausada")
        default:
            log.Printf("❓ Estado de segregación desconocido: %d", value)
        }
    }
}

// handleSegregationValue maneja valores específicos de segregation
func (sm *SubscriptionManager) handleSegregationValue(data SubscriptionData) {
	log.Printf("⚙️  Valor de segregación recibido: %v", data.Data.Value)

	// Validar que el nodo corresponde al método de segregación
	expectedNodeID := models.BuildNodeID(models.OPCUA_SEGREGATION_METHOD)
	if data.NodeID != expectedNodeID {
		log.Printf("⚠️  Nodo ID no coincide. Esperado: %s, Recibido: %s", expectedNodeID, data.NodeID)
	}
}

// handleHeartbeatValue maneja valores específicos de heartbeat
func (sm *SubscriptionManager) handleHeartbeatValue(data SubscriptionData) {
	log.Printf("💓 Valor de heartbeat recibido: %v", data.Data.Value)

	// Validar timestamp del heartbeat
	timeSinceLastHeartbeat := time.Since(data.Data.Timestamp)
	timeoutDuration := time.Duration(models.OPCUA_TIMEOUT) * time.Second

	if timeSinceLastHeartbeat > timeoutDuration {
		log.Printf("🔴 TIMEOUT: Heartbeat expirado hace %v (timeout: %v)",
			timeSinceLastHeartbeat, timeoutDuration)
	} else {
		log.Printf("✅ Heartbeat dentro del tiempo permitido")
	}
}

// handleAlarmSubscription maneja suscripciones de alarmas
func (sm *SubscriptionManager) handleAlarmSubscription(data SubscriptionData) {
    log.Printf("🚨 Procesando alarma: %s = %v", data.NodeID, data.Data.Value)
    
    // Lógica específica para alarmas
    if value, ok := data.Data.Value.(bool); ok && value {
        log.Printf("🔴 ALARMA ACTIVA en nodo: %s", data.NodeID)
        // Aquí puedes enviar notificaciones, emails, etc.
    }
}

// handleDataSubscription maneja suscripciones de datos
func (sm *SubscriptionManager) handleDataSubscription(data SubscriptionData) {
    log.Printf("📊 Procesando datos: %s = %v", data.NodeID, data.Data.Value)
    
    // Lógica específica para datos
    // Ejemplo: Almacenar en base de datos, calcular estadísticas, etc.
}

// handleGenericSubscription maneja suscripciones genéricas
func (sm *SubscriptionManager) handleGenericSubscription(data SubscriptionData) {
    log.Printf("📝 Procesando suscripción genérica: %s", data.SubscriptionName)
}

// OnSubscriptionData envía datos de suscripción al manager
func (sm *SubscriptionManager) OnSubscriptionData(subscriptionName, nodeID string, data *listeners.NodeData) {
	if !sm.isRunning {
		return
	}

	subscriptionData := SubscriptionData{
		SubscriptionName: subscriptionName,
		NodeID:           nodeID,
		Data:             data,
		ReceivedAt:       time.Now(),
	}

	// Envío no bloqueante
	select {
	case sm.dataChan <- subscriptionData:
	// Enviado exitosamente
	default:
		log.Printf("⚠️  Warning: Subscription Manager channel full, dropping data for %s", nodeID)
	}
}

// Stop detiene el manager de suscripciones
func (sm *SubscriptionManager) Stop() {
    log.Println("Deteniendo Subscription Manager...")
    sm.isRunning = false
    
    // Cerrar channel
    close(sm.dataChan)
    
    log.Println("Subscription Manager detenido")
}

// GetStats retorna estadísticas del manager de suscripciones
func (sm *SubscriptionManager) GetStats() map[string]interface{} {
    return map[string]interface{}{
        "running":    sm.isRunning,
        "queue_size": len(sm.dataChan),
        "queue_cap":  cap(sm.dataChan),
    }
}

// statusCodeToString convierte StatusCode a string legible
func (sm *SubscriptionManager) statusCodeToString(code interface{}) string {
    // Convertir el StatusCode a uint32 para comparación
    var statusCode uint32
    switch v := code.(type) {
    case uint32:
        statusCode = v
    case int:
        statusCode = uint32(v)
    default:
        return fmt.Sprintf("%v", code)
    }
    
    switch statusCode {
    case 0x00000000: // Good
        return "Good"
    case 0x40000000: // Uncertain
        return "Uncertain"
    case 0x80000000: // Bad
        return "Bad"
    case 0x000A0000: // BadNodeIdUnknown
        return "BadNodeIdUnknown"
    case 0x80340000: // BadNotConnected
        return "BadNotConnected"
    case 0x800F0000: // BadOutOfService
        return "BadOutOfService"
    case 0x40920000: // UncertainLastUsableValue
        return "UncertainLastUsableValue"
    default:
        return fmt.Sprintf("0x%08X", statusCode)
    }
}

// CreateCustomSubscriptionHandler permite crear handlers personalizados
func (sm *SubscriptionManager) CreateCustomSubscriptionHandler(subscriptionName string, handler func(SubscriptionData)) {
    // Implementar sistema de handlers personalizados si es necesario
    log.Printf("Handler personalizado registrado para: %s", subscriptionName)
}