package manager

import (
    "log"
    "time"
    "API-GREENEX/communication"
    "API-GREENEX/shared"
)

type MethodManager struct {
    isRunning bool
    dataChan  chan MethodData
}

// Estructura para datos de métodos
type MethodData struct {
    Operation   string // "read", "write", "call"
    NodeID      string
    Value       interface{}
    Result      interface{}
    Error       error
    ReceivedAt  time.Time
    Duration    time.Duration
}

// NewMethodManager crea una nueva instancia del manager de métodos
func NewMethodManager() *MethodManager {
    return &MethodManager{
        isRunning: false,
        dataChan:  make(chan MethodData, 100),
    }
}

// Start inicia el manager de métodos
func (mm *MethodManager) Start() {
    log.Println("Iniciando Method Manager...")
    mm.isRunning = true
    
    // Procesar datos de métodos
    go mm.processMethodData()
    
    // Mantener el manager corriendo
    for mm.isRunning {
        time.Sleep(1 * time.Second)
    }
}

// processMethodData procesa los datos de métodos
func (mm *MethodManager) processMethodData() {
    for mm.isRunning {
        select {
        case data := <-mm.dataChan:
            mm.handleMethodData(data)
        case <-time.After(100 * time.Millisecond):
            // Timeout para permitir verificar isRunning
        }
    }
}

// handleMethodData maneja los datos de métodos
func (mm *MethodManager) handleMethodData(data MethodData) {
    // Log detallado de la comunicación de métodos
    log.Printf("╔═══ MÉTODO OPC UA ═══╗")
    log.Printf("║ Operación: %-10s ║", data.Operation)
    log.Printf("║ NodeID: %-13s ║", data.NodeID)
    
    if data.Value != nil {
        log.Printf("║ Valor: %-14v ║", data.Value)
    }
    
    if data.Result != nil {
        log.Printf("║ Resultado: %-11v ║", data.Result)
    }
    
    if data.Error != nil {
        log.Printf("║ Error: %-14s ║", data.Error.Error())
    } else {
        log.Printf("║ Estado: %-13s ║", "OK")
    }
    
    if data.Duration > 0 {
        log.Printf("║ Duración: %-11s ║", data.Duration.String())
    }
    
    log.Printf("║ Ejecutado: %-10s ║", data.ReceivedAt.Format("15:04:05"))
    log.Printf("╚═══════════════════════╝")
    
    // Procesar lógica específica del método
    mm.processMethodLogic(data)
}

// processMethodLogic procesa la lógica específica de métodos usando constantes
func (mm *MethodManager) processMethodLogic(data MethodData) {
    // Identificar si es un método conocido usando constantes
    mm.identifyMethodByNodeID(data)
    
    switch data.Operation {
    case "read":
        mm.handleReadOperation(data)
    case "write":
        mm.handleWriteOperation(data)
    case "call":
        mm.handleCallOperation(data)
    default:
        mm.handleGenericOperation(data)
    }
}

// identifyMethodByNodeID identifica el método basado en las constantes
func (mm *MethodManager) identifyMethodByNodeID(data MethodData) {
    // Verificar si es un nodo conocido usando constantes
    switch data.NodeID {
    case shared.DEFAULT_SEGREGATION_NODE:
        log.Printf("🎯 Método identificado: Segregación (nodo por defecto)")
    case shared.DEFAULT_HEARTBEAT_NODE:
        log.Printf("🎯 Método identificado: Heartbeat (nodo por defecto)")
    case shared.BuildNodeID(shared.OPCUA_SEGREGATION_METHOD):
        log.Printf("🎯 Método identificado: Segregación (ns=%d;i=%d)", 
                   shared.OPCUA_NAMESPACE, shared.OPCUA_SEGREGATION_METHOD)
    default:
        // Verificar si es un nodo construido con el namespace correcto
        if mm.isFromCorrectNamespace(data.NodeID) {
            log.Printf("🎯 Nodo del namespace correcto (%d): %s", shared.OPCUA_NAMESPACE, data.NodeID)
        } else {
            log.Printf("❓ Nodo desconocido o namespace incorrecto: %s", data.NodeID)
        }
    }
}

// isFromCorrectNamespace verifica si el nodeID pertenece al namespace correcto
func (mm *MethodManager) isFromCorrectNamespace(nodeID string) bool {
    // Verificar si el nodeID comienza con el namespace correcto
    expectedPrefix := shared.BuildNodeID(0)[:6] // "ns=4;i" parte
    return len(nodeID) > 6 && nodeID[:6] == expectedPrefix
}

// handleReadOperation maneja operaciones de lectura usando constantes
func (mm *MethodManager) handleReadOperation(data MethodData) {
    if data.Error != nil {
        log.Printf("❌ Error en lectura de %s: %v", data.NodeID, data.Error)
        mm.handleReadError(data)
    } else {
        log.Printf("✅ Lectura exitosa de %s (Namespace: %d)", data.NodeID, shared.OPCUA_NAMESPACE)
        mm.processReadData(data)
    }
    
    // Verificar si la lectura tardó más del timeout configurado
    timeoutDuration := time.Duration(shared.OPCUA_TIMEOUT) * time.Second
    if data.Duration > timeoutDuration {
        log.Printf("⚠️  ADVERTENCIA: Lectura lenta (%v > %v timeout)", data.Duration, timeoutDuration)
    }
}

// handleWriteOperation maneja operaciones de escritura usando constantes
func (mm *MethodManager) handleWriteOperation(data MethodData) {
    if data.Error != nil {
        log.Printf("❌ Error en escritura de %s: %v", data.NodeID, data.Error)
        mm.handleWriteError(data)
    } else {
        log.Printf("✅ Escritura exitosa en %s: %v (Namespace: %d)", 
                   data.NodeID, data.Value, shared.OPCUA_NAMESPACE)
        mm.processWriteConfirmation(data)
    }
    
    // Verificar timeout para escrituras
    timeoutDuration := time.Duration(shared.OPCUA_TIMEOUT) * time.Second
    if data.Duration > timeoutDuration {
        log.Printf("⚠️  ADVERTENCIA: Escritura lenta (%v > %v timeout)", data.Duration, timeoutDuration)
    }
    
    // Lógica específica para escrituras en métodos conocidos
    mm.handleSpecificMethodWrite(data)
}

// handleCallOperation maneja llamadas a métodos usando constantes
func (mm *MethodManager) handleCallOperation(data MethodData) {
    if data.Error != nil {
        log.Printf("❌ Error en llamada a método %s: %v", data.NodeID, data.Error)
        mm.handleCallError(data)
    } else {
        log.Printf("✅ Llamada exitosa a método %s (Namespace: %d)", 
                   data.NodeID, shared.OPCUA_NAMESPACE)
        mm.processCallResult(data)
    }
    
    // Verificar timeout para llamadas
    timeoutDuration := time.Duration(shared.OPCUA_TIMEOUT) * time.Second
    if data.Duration > timeoutDuration {
        log.Printf("⚠️  ADVERTENCIA: Llamada lenta (%v > %v timeout)", data.Duration, timeoutDuration)
    }
}

// handleSpecificMethodWrite maneja escrituras en métodos específicos
func (mm *MethodManager) handleSpecificMethodWrite(data MethodData) {
    switch data.NodeID {
    case shared.DEFAULT_SEGREGATION_NODE, shared.BuildNodeID(shared.OPCUA_SEGREGATION_METHOD):
        log.Printf("⚙️  Escribiendo en método de segregación: %v", data.Value)
        if value, ok := data.Value.(int32); ok {
            if value == shared.OPCUA_SEGREGATION_METHOD {
                log.Printf("✅ Activando método de segregación con valor: %d", value)
            }
        }
    case shared.DEFAULT_HEARTBEAT_NODE:
        log.Printf("💓 Escribiendo en heartbeat: %v", data.Value)
    }
}

// processReadData procesa datos leídos exitosamente usando constantes
func (mm *MethodManager) processReadData(data MethodData) {
    if result, ok := data.Result.(*communication.NodeData); ok {
        log.Printf("📖 Datos leídos: Valor=%v, Calidad=%v, Timestamp=%v", 
                   result.Value, result.Quality, result.Timestamp.Format("15:04:05.000"))
        
        // Verificar si los datos están dentro del timeout
        dataAge := time.Since(result.Timestamp)
        timeoutDuration := time.Duration(shared.OPCUA_TIMEOUT) * time.Second
        
        if dataAge > timeoutDuration {
            log.Printf("⚠️  ADVERTENCIA: Datos antiguos (edad: %v, timeout: %v)", dataAge, timeoutDuration)
        }
        
        // Procesar según el tipo de nodo
        mm.processDataByNodeType(data.NodeID, result)
    }
}

// processDataByNodeType procesa datos según el tipo de nodo usando constantes
func (mm *MethodManager) processDataByNodeType(nodeID string, data *communication.NodeData) {
    switch nodeID {
    case shared.DEFAULT_SEGREGATION_NODE, shared.BuildNodeID(shared.OPCUA_SEGREGATION_METHOD):
        log.Printf("⚙️  Procesando datos de segregación: %v", data.Value)
    case shared.DEFAULT_HEARTBEAT_NODE:
        log.Printf("💓 Procesando datos de heartbeat: %v", data.Value)
    default:
        if mm.isFromCorrectNamespace(nodeID) {
            log.Printf("📊 Procesando datos del namespace %d: %v", shared.OPCUA_NAMESPACE, data.Value)
        }
    }
}



// handleGenericOperation maneja operaciones genéricas
func (mm *MethodManager) handleGenericOperation(data MethodData) {
    log.Printf("🔧 Operación genérica: %s en %s", data.Operation, data.NodeID)
}

// Métodos específicos para manejo de errores y resultados

func (mm *MethodManager) handleReadError(data MethodData) {
    // Lógica específica para errores de lectura
    log.Printf("🔍 Analizando error de lectura en %s", data.NodeID)
}

func (mm *MethodManager) handleWriteError(data MethodData) {
    // Lógica específica para errores de escritura
    log.Printf("✏️  Analizando error de escritura en %s", data.NodeID)
}

func (mm *MethodManager) processWriteConfirmation(data MethodData) {
    // Lógica específica para confirmación de escritura
    log.Printf("✏️  Escritura confirmada en %s", data.NodeID)
}

func (mm *MethodManager) handleCallError(data MethodData) {
    // Lógica específica para errores de llamada
    log.Printf("📞 Analizando error de llamada en %s", data.NodeID)
}

func (mm *MethodManager) processCallResult(data MethodData) {
    // Lógica específica para resultados de llamada
    log.Printf("📞 Resultado de llamada procesado para %s", data.NodeID)
}

// Métodos públicos para enviar datos al manager

// OnMethodRead envía datos de lectura al manager
func (mm *MethodManager) OnMethodRead(nodeID string, result *communication.NodeData, err error, duration time.Duration) {
    if !mm.isRunning {
        return
    }
    
    methodData := MethodData{
        Operation:  "read",
        NodeID:     nodeID,
        Result:     result,
        Error:      err,
        ReceivedAt: time.Now(),
        Duration:   duration,
    }
    
    select {
    case mm.dataChan <- methodData:
        // Enviado exitosamente
    default:
        log.Printf("⚠️  Warning: Method Manager channel full, dropping read data for %s", nodeID)
    }
}

// OnMethodWrite envía datos de escritura al manager
func (mm *MethodManager) OnMethodWrite(nodeID string, value interface{}, err error, duration time.Duration) {
    if !mm.isRunning {
        return
    }
    
    methodData := MethodData{
        Operation:  "write",
        NodeID:     nodeID,
        Value:      value,
        Error:      err,
        ReceivedAt: time.Now(),
        Duration:   duration,
    }
    
    select {
    case mm.dataChan <- methodData:
        // Enviado exitosamente
    default:
        log.Printf("⚠️  Warning: Method Manager channel full, dropping write data for %s", nodeID)
    }
}

// OnMethodCall envía datos de llamada a método al manager
func (mm *MethodManager) OnMethodCall(nodeID string, inputArgs interface{}, result interface{}, err error, duration time.Duration) {
    if !mm.isRunning {
        return
    }
    
    methodData := MethodData{
        Operation:  "call",
        NodeID:     nodeID,
        Value:      inputArgs,
        Result:     result,
        Error:      err,
        ReceivedAt: time.Now(),
        Duration:   duration,
    }
    
    select {
    case mm.dataChan <- methodData:
        // Enviado exitosamente
    default:
        log.Printf("⚠️  Warning: Method Manager channel full, dropping call data for %s", nodeID)
    }
}

// Stop detiene el manager de métodos
func (mm *MethodManager) Stop() {
    log.Println("Deteniendo Method Manager...")
    mm.isRunning = false
    
    // Cerrar channel
    close(mm.dataChan)
    
    log.Println("Method Manager detenido")
}

// GetStats retorna estadísticas del manager de métodos
func (mm *MethodManager) GetStats() map[string]interface{} {
    return map[string]interface{}{
        "running":    mm.isRunning,
        "queue_size": len(mm.dataChan),
        "queue_cap":  cap(mm.dataChan),
    }
}

// GetOperationStats retorna estadísticas por operación
func (mm *MethodManager) GetOperationStats() map[string]int {
    // Implementar contadores de operaciones si es necesario
    return map[string]int{
        "reads":  0,
        "writes": 0,
        "calls":  0,
        "errors": 0,
    }
}