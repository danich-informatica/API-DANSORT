package plc

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/gopcua/opcua"
	"github.com/gopcua/opcua/ua"
)

// Client encapsula la conexión a un servidor OPC UA
type Client struct {
	endpoint        string
	client          *opcua.Client
	config          PLCConfig
	cache           *LRUCache          // Cache LRU para lecturas frecuentes
	heartbeatCancel context.CancelFunc // Para detener el heartbeat
}

// NewClient crea un nuevo cliente OPC UA sin conectar
func NewClient(config PLCConfig) *Client {
	return &Client{
		endpoint: config.Endpoint,
		config:   config,
		cache:    NewLRUCache(1000, 100*time.Millisecond), // Cache de 1000 entradas, TTL 100ms
	}
}

// Connect establece la conexión con el servidor OPC UA y activa la sesión
func (c *Client) Connect(ctx context.Context) error {
	t0 := time.Now()

	opts := []opcua.Option{
		opcua.SecurityMode(ua.MessageSecurityModeNone),
		opcua.SecurityPolicy(ua.SecurityPolicyURINone),
		opcua.AutoReconnect(true),
	}

	t1 := time.Now()
	client, err := opcua.NewClient(c.endpoint, opts...)
	if err != nil {
		return fmt.Errorf("error creando cliente para %s: %w", c.endpoint, err)
	}
	log.Printf("⏱️  [Connect] NewClient: %dms", t1.Sub(t0).Milliseconds())

	t2 := time.Now()
	if err := client.Connect(ctx); err != nil {
		return fmt.Errorf("error al conectar a %s: %w", c.endpoint, err)
	}
	log.Printf("⏱️  [Connect] ⚡ client.Connect(): %dms", time.Since(t2).Milliseconds())

	c.client = client

	// Activar la sesión haciendo una lectura dummy del nodo Server (garantiza sesión activa)
	// Esto evita el error "StatusBadSessionNotActivated" en operaciones posteriores
	t3 := time.Now()
	dummyNodeID, _ := ua.ParseNodeID("i=2253") // Server.ServerStatus node
	req := &ua.ReadRequest{
		MaxAge:             2000,
		NodesToRead:        []*ua.ReadValueID{{NodeID: dummyNodeID}},
		TimestampsToReturn: ua.TimestampsToReturnBoth,
	}
	if _, err := client.Read(ctx, req); err != nil {
		log.Printf("⚠️ Advertencia: no se pudo activar sesión con lectura dummy: %v", err)
		// No retornamos error porque la conexión básica funciona
	} else {
		log.Printf("⏱️  [Connect] Session activation (dummy read): %dms", time.Since(t3).Milliseconds())
	}

	// ⚡ NUEVO: Iniciar heartbeat para mantener sesión activa
	heartbeatCtx, cancel := context.WithCancel(context.Background())
	c.heartbeatCancel = cancel
	go c.startHeartbeat(heartbeatCtx, 15*time.Second) // Ping cada 15s

	log.Printf("✅ Conexión establecida a %s (total: %dms)", c.endpoint, time.Since(t0).Milliseconds())
	return nil
}

// Close cierra la conexión con el servidor OPC UA
func (c *Client) Close(ctx context.Context) error {
	// Detener heartbeat si está activo
	if c.heartbeatCancel != nil {
		c.heartbeatCancel()
	}

	if c.client != nil {
		return c.client.Close(ctx)
	}
	return nil
}

// startHeartbeat mantiene la sesión activa enviando lecturas periódicas
func (c *Client) startHeartbeat(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	log.Printf("💓 Heartbeat OPC UA iniciado para %s (intervalo: %v)", c.endpoint, interval)

	for {
		select {
		case <-ctx.Done():
			log.Printf("🛑 Heartbeat detenido para %s", c.endpoint)
			return

		case <-ticker.C:
			// Leer Server.ServerStatus para mantener sesión activa
			dummyNodeID, _ := ua.ParseNodeID("i=2253") // Server.ServerStatus
			req := &ua.ReadRequest{
				MaxAge:             2000,
				NodesToRead:        []*ua.ReadValueID{{NodeID: dummyNodeID}},
				TimestampsToReturn: ua.TimestampsToReturnBoth,
			}

			_, err := c.client.Read(ctx, req)
			if err != nil {
				log.Printf("⚠️  Heartbeat falló para %s: %v (intentando reconectar)", c.endpoint, err)
				// La próxima operación detectará el problema y reconectará
			} else {
				log.Printf("💓 Heartbeat OK para %s", c.endpoint)
			}
		}
	}
}

// ReadNode lee el valor de un nodo específico
func (c *Client) ReadNode(ctx context.Context, nodeID string) (*NodeInfo, error) {
	if c.client == nil {
		return nil, fmt.Errorf("cliente no conectado")
	}

	// Intentar obtener de cache primero
	if cached, found := c.cache.Get(nodeID); found {
		log.Printf("💾 Cache HIT para %s", nodeID)
		return cached.Value.(*NodeInfo), nil
	}

	// Parsear el NodeID
	id, err := ua.ParseNodeID(nodeID)
	if err != nil {
		return nil, fmt.Errorf("nodeID inválido '%s': %w", nodeID, err)
	}

	// Crear request de lectura
	req := &ua.ReadRequest{
		NodesToRead: []*ua.ReadValueID{
			{
				NodeID:      id,
				AttributeID: ua.AttributeIDValue,
			},
		},
	}

	// Ejecutar lectura
	resp, err := c.client.Read(ctx, req)
	if err != nil {
		// Detectar error de sesión inválida y reconectar
		if isSessionError(err) {
			log.Printf("⚠️ Sesión inválida detectada, reconectando...")
			if reconnectErr := c.reconnect(ctx); reconnectErr != nil {
				return nil, fmt.Errorf("error al reconectar: %w", reconnectErr)
			}
			// Reintentar después de reconectar
			resp, err = c.client.Read(ctx, req)
			if err != nil {
				return nil, fmt.Errorf("error al leer nodo %s después de reconexión: %w", nodeID, err)
			}
		} else {
			return nil, fmt.Errorf("error al leer nodo %s: %w", nodeID, err)
		}
	}

	// Validar respuesta
	if len(resp.Results) == 0 {
		return nil, fmt.Errorf("lectura de %s sin resultados", nodeID)
	}

	result := resp.Results[0]
	if result.Status != ua.StatusOK {
		return nil, fmt.Errorf("lectura de %s con status: %s", nodeID, result.Status)
	}

	// Extraer valor
	value := result.Value.Value()
	valueType := fmt.Sprintf("%T", value)

	nodeInfo := &NodeInfo{
		NodeID:    nodeID,
		Value:     value,
		ValueType: valueType,
	}

	// Guardar en cache
	c.cache.Set(nodeID, nodeInfo)
	log.Printf("💾 Cache MISS - guardado %s", nodeID)

	return nodeInfo, nil
}

// isSessionError detecta si un error es por sesión inválida
func isSessionError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return contains(errStr, "StatusBadSessionIDInvalid") ||
		contains(errStr, "StatusBadSessionClosed") ||
		contains(errStr, "StatusBadSessionNotActivated")
}

// contains verifica si una cadena contiene un substring usando strings.Contains
func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}

// reconnect cierra la conexión actual y establece una nueva
func (c *Client) reconnect(ctx context.Context) error {
	t0 := time.Now()
	log.Printf("🔄 Reconectando a %s...", c.endpoint)

	// Cerrar conexión anterior si existe
	t1 := time.Now()
	if c.client != nil {
		_ = c.client.Close(ctx)
	}
	log.Printf("⏱️  [Reconnect] Close old connection: %dms", time.Since(t1).Milliseconds())

	// Crear nueva conexión
	t2 := time.Now()
	opts := []opcua.Option{
		opcua.SecurityMode(ua.MessageSecurityModeNone),
		opcua.SecurityPolicy(ua.SecurityPolicyURINone),
		opcua.AutoReconnect(true),
	}

	client, err := opcua.NewClient(c.endpoint, opts...)
	if err != nil {
		return fmt.Errorf("error creando cliente: %w", err)
	}
	log.Printf("⏱️  [Reconnect] NewClient: %dms", time.Since(t2).Milliseconds())

	t3 := time.Now()
	if err := client.Connect(ctx); err != nil {
		return fmt.Errorf("error al conectar: %w", err)
	}
	log.Printf("⏱️  [Reconnect] ⚡ client.Connect(): %dms", time.Since(t3).Milliseconds())

	c.client = client

	// Activar la sesión con lectura dummy (igual que en Connect())
	t4 := time.Now()
	dummyNodeID, _ := ua.ParseNodeID("i=2253")
	req := &ua.ReadRequest{
		MaxAge:             2000,
		NodesToRead:        []*ua.ReadValueID{{NodeID: dummyNodeID}},
		TimestampsToReturn: ua.TimestampsToReturnBoth,
	}
	if _, err := client.Read(ctx, req); err != nil {
		log.Printf("⚠️ Advertencia en reconexión: no se pudo activar sesión: %v", err)
	} else {
		log.Printf("⏱️  [Reconnect] Session activation: %dms", time.Since(t4).Milliseconds())
	}

	c.cache.Clear() // Invalidar cache después de reconexión

	// ⚡ Reiniciar heartbeat después de reconectar
	if c.heartbeatCancel != nil {
		c.heartbeatCancel() // Detener el anterior
	}
	heartbeatCtx, cancel := context.WithCancel(context.Background())
	c.heartbeatCancel = cancel
	go c.startHeartbeat(heartbeatCtx, 15*time.Second)

	log.Printf("✅ Reconexión exitosa a %s (total: %dms)", c.endpoint, time.Since(t0).Milliseconds())
	return nil
}

// ReadNodes lee el valor de múltiples nodos en una sola petición
func (c *Client) ReadNodes(ctx context.Context, nodeIDs []string) ([]*NodeInfo, error) {
	if c.client == nil {
		return nil, fmt.Errorf("cliente no conectado")
	}

	results := make([]*NodeInfo, len(nodeIDs))
	validNodesToRead := make([]*ua.ReadValueID, 0, len(nodeIDs))
	// indexMap mapea el índice en validNodesToRead al índice original en nodeIDs
	indexMap := make(map[int]int)

	for i, nodeID := range nodeIDs {
		id, err := ua.ParseNodeID(nodeID)
		if err != nil {
			results[i] = &NodeInfo{NodeID: nodeID, Error: fmt.Errorf("nodeID inválido")}
		} else {
			indexMap[len(validNodesToRead)] = i
			validNodesToRead = append(validNodesToRead, &ua.ReadValueID{NodeID: id, AttributeID: ua.AttributeIDValue})
		}
	}

	// Si no hay nodos válidos para leer, devolvemos los resultados con los errores de parseo
	if len(validNodesToRead) == 0 {
		return results, nil
	}

	req := &ua.ReadRequest{NodesToRead: validNodesToRead}
	resp, err := c.client.Read(ctx, req)
	if err != nil {
		// Si toda la petición falla, propagamos el error a todos los nodos que intentamos leer
		for i := range validNodesToRead {
			originalIndex := indexMap[i]
			if results[originalIndex] == nil {
				results[originalIndex] = &NodeInfo{NodeID: nodeIDs[originalIndex], Error: err}
			}
		}
		return results, nil
	}

	if len(resp.Results) != len(validNodesToRead) {
		return nil, fmt.Errorf("la respuesta de lectura múltiple no coincide con la petición (%d vs %d)", len(resp.Results), len(validNodesToRead))
	}

	// Procesar resultados individuales
	for i, result := range resp.Results {
		originalIndex := indexMap[i]
		nodeInfo := &NodeInfo{NodeID: nodeIDs[originalIndex]}
		if result.Status != ua.StatusOK {
			nodeInfo.Error = fmt.Errorf("status: %s", result.Status)
		} else {
			nodeInfo.Value = result.Value.Value()
			nodeInfo.ValueType = fmt.Sprintf("%T", nodeInfo.Value)
		}
		results[originalIndex] = nodeInfo
	}

	return results, nil
}

// WriteNode escribe un valor en un nodo específico
func (c *Client) WriteNode(ctx context.Context, nodeID string, value interface{}) error {
	if c.client == nil {
		return fmt.Errorf("cliente no conectado")
	}

	id, err := ua.ParseNodeID(nodeID)
	if err != nil {
		return fmt.Errorf("nodeID inválido '%s': %w", nodeID, err)
	}

	v, err := ua.NewVariant(value)
	if err != nil {
		return fmt.Errorf("valor de escritura inválido '%v': %w", value, err)
	}

	req := &ua.WriteRequest{
		NodesToWrite: []*ua.WriteValue{
			{
				NodeID:      id,
				AttributeID: ua.AttributeIDValue,
				Value: &ua.DataValue{
					EncodingMask: ua.DataValueValue, // Crucial para que el PLC acepte la escritura
					Value:        v,
				},
			},
		},
	}

	resp, err := c.client.Write(ctx, req)
	if err != nil {
		// Detectar error de sesión y reconectar
		if isSessionError(err) {
			log.Printf("⚠️ Sesión inválida detectada en escritura, reconectando...")
			if reconnectErr := c.reconnect(ctx); reconnectErr != nil {
				return fmt.Errorf("error al reconectar: %w", reconnectErr)
			}
			// Reintentar después de reconectar
			resp, err = c.client.Write(ctx, req)
			if err != nil {
				return fmt.Errorf("error al escribir en el nodo %s después de reconexión: %w", nodeID, err)
			}
		} else {
			return fmt.Errorf("error al escribir en el nodo %s: %w", nodeID, err)
		}
	}

	if len(resp.Results) == 0 {
		return fmt.Errorf("escritura en %s sin resultados", nodeID)
	}

	if resp.Results[0] != ua.StatusOK {
		return fmt.Errorf("escritura en %s con status: %s", nodeID, resp.Results[0])
	}

	// Invalidar cache después de escritura exitosa
	c.cache.Clear()

	return nil
}

// WriteNodeTyped escribe un valor con conversión de tipo explícita (como dantrack)
func (c *Client) WriteNodeTyped(ctx context.Context, nodeID string, value interface{}, dataType string) error {
	if c.client == nil {
		return fmt.Errorf("cliente no conectado")
	}

	id, err := ua.ParseNodeID(nodeID)
	if err != nil {
		return fmt.Errorf("nodeID inválido '%s': %w", nodeID, err)
	}

	// Convertir el valor al tipo correcto antes de escribir (crítico para WAGO/Codesys)
	convertedValue, err := ConvertValueForWrite(value, dataType)
	if err != nil {
		return fmt.Errorf("error convirtiendo valor para escritura: %w", err)
	}

	v, err := ua.NewVariant(convertedValue)
	if err != nil {
		return fmt.Errorf("valor de escritura inválido '%v': %w", convertedValue, err)
	}

	req := &ua.WriteRequest{
		NodesToWrite: []*ua.WriteValue{
			{
				NodeID:      id,
				AttributeID: ua.AttributeIDValue,
				Value: &ua.DataValue{
					EncodingMask: ua.DataValueValue, // Importante para WAGO/Codesys
					Value:        v,
				},
			},
		},
	}

	resp, err := c.client.Write(ctx, req)
	if err != nil {
		return fmt.Errorf("error al escribir en el nodo %s: %w", nodeID, err)
	}

	if len(resp.Results) == 0 {
		return fmt.Errorf("escritura en %s sin resultados", nodeID)
	}

	if resp.Results[0] != ua.StatusOK {
		return fmt.Errorf("escritura en %s con status: %s", nodeID, resp.Results[0])
	}

	// Invalidar cache después de escritura exitosa
	c.cache.Clear()

	return nil
}

// AppendToArrayNode lee un array de ExtensionObject, agrega un nuevo elemento y lo escribe de vuelta
func (c *Client) AppendToArrayNode(ctx context.Context, nodeID string, newElement *ua.ExtensionObject) error {
	if c.client == nil {
		return fmt.Errorf("cliente no conectado")
	}

	// 1. Leer el array actual
	nodeInfo, err := c.ReadNode(ctx, nodeID)
	if err != nil {
		return fmt.Errorf("error al leer el array actual: %w", err)
	}

	// 2. Verificar que es un array de ExtensionObject
	currentArray, ok := nodeInfo.Value.([]*ua.ExtensionObject)
	if !ok {
		return fmt.Errorf("el nodo no contiene un array de ExtensionObject, tipo actual: %T", nodeInfo.Value)
	}

	log.Printf("📋 Array actual tiene %d elemento(s)", len(currentArray))

	// 3. Crear nuevo array con el elemento adicional
	newArray := make([]*ua.ExtensionObject, len(currentArray)+1)
	copy(newArray, currentArray)
	newArray[len(currentArray)] = newElement

	log.Printf("➕ Agregando elemento. Nuevo tamaño del array: %d", len(newArray))

	// 4. Escribir el array completo de vuelta
	return c.WriteNode(ctx, nodeID, newArray)
}

// AppendToUInt32Array lee un array de uint32, agrega un nuevo elemento y lo escribe de vuelta
func (c *Client) AppendToUInt32Array(ctx context.Context, nodeID string, newValue uint32) error {
	if c.client == nil {
		return fmt.Errorf("cliente no conectado")
	}

	// 1. Leer el array actual
	nodeInfo, err := c.ReadNode(ctx, nodeID)
	if err != nil {
		return fmt.Errorf("error al leer el array actual: %w", err)
	}

	// 2. Verificar que es un array de uint32
	currentArray, ok := nodeInfo.Value.([]uint32)
	if !ok {
		return fmt.Errorf("el nodo no contiene un array de uint32, tipo actual: %T", nodeInfo.Value)
	}

	log.Printf("📋 Array actual tiene %d elemento(s)", len(currentArray))

	// 3. Crear nuevo array con el elemento adicional
	newArray := make([]uint32, len(currentArray)+1)
	copy(newArray, currentArray)
	newArray[len(currentArray)] = newValue

	log.Printf("➕ Agregando elemento uint32: %d. Nuevo tamaño del array: %d", newValue, len(newArray))

	// 4. Escribir el array completo de vuelta (WriteNode ya tiene EncodingMask)
	return c.WriteNode(ctx, nodeID, newArray)
}

// BrowseNode explora los nodos hijos de un nodo específico
func (c *Client) BrowseNode(ctx context.Context, nodeID string) ([]BrowseResult, error) {
	if c.client == nil {
		return nil, fmt.Errorf("cliente no conectado")
	}

	id, err := ua.ParseNodeID(nodeID)
	if err != nil {
		return nil, fmt.Errorf("nodeID inválido '%s': %w", nodeID, err)
	}

	// Crear request de browse
	req := &ua.BrowseRequest{
		NodesToBrowse: []*ua.BrowseDescription{
			{
				NodeID:          id,
				BrowseDirection: ua.BrowseDirectionForward,
				ReferenceTypeID: ua.NewNumericNodeID(0, 0), // Todas las referencias
				IncludeSubtypes: true,
				NodeClassMask:   uint32(ua.NodeClassAll),
				ResultMask:      uint32(ua.BrowseResultMaskAll),
			},
		},
	}

	resp, err := c.client.Browse(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("error al explorar nodo %s: %w", nodeID, err)
	}

	if len(resp.Results) == 0 {
		return nil, fmt.Errorf("exploración de %s sin resultados", nodeID)
	}

	result := resp.Results[0]
	if result.StatusCode != ua.StatusOK {
		return nil, fmt.Errorf("exploración de %s con status: %s", nodeID, result.StatusCode)
	}

	// Convertir referencias a resultados
	var browseResults []BrowseResult
	for _, ref := range result.References {
		browseName := ref.BrowseName.Name
		if ref.BrowseName.NamespaceIndex > 0 {
			browseName = fmt.Sprintf("%d:%s", ref.BrowseName.NamespaceIndex, ref.BrowseName.Name)
		}

		browseResults = append(browseResults, BrowseResult{
			NodeID:        ref.NodeID.NodeID.String(),
			BrowseName:    browseName,
			DisplayName:   ref.DisplayName.Text,
			NodeClass:     ref.NodeClass,
			IsForward:     ref.IsForward,
			ReferenceType: ref.ReferenceTypeID.String(),
		})
	}

	return browseResults, nil
}

// CallMethod invoca un método OPC UA en el servidor
func (c *Client) CallMethod(ctx context.Context, objectID string, methodID string, inputArgs []*ua.Variant) ([]interface{}, error) {
	// ⏱️ TIMING: Inicio
	t0 := time.Now()

	if c.client == nil {
		return nil, fmt.Errorf("cliente no conectado")
	}

	// ⏱️ TIMING: Parsear ObjectID
	t1 := time.Now()
	objID, err := ua.ParseNodeID(objectID)
	if err != nil {
		return nil, fmt.Errorf("objectID inválido '%s': %w", objectID, err)
	}
	t2 := time.Now()
	log.Printf("⏱️  [CallMethod] Parse ObjectID: %dus", t2.Sub(t1).Microseconds())

	// ⏱️ TIMING: Parsear MethodID
	methID, err := ua.ParseNodeID(methodID)
	if err != nil {
		return nil, fmt.Errorf("methodID inválido '%s': %w", methodID, err)
	}
	t3 := time.Now()
	log.Printf("⏱️  [CallMethod] Parse MethodID: %dus", t3.Sub(t2).Microseconds())

	// ⏱️ TIMING: Crear request
	req := &ua.CallMethodRequest{
		ObjectID:       objID,
		MethodID:       methID,
		InputArguments: inputArgs,
	}
	t4 := time.Now()
	log.Printf("⏱️  [CallMethod] Create Request: %dus", t4.Sub(t3).Microseconds())
	// ⏱️ TIMING: Llamada al PLC (CRÍTICO - aquí está el delay probable)
	log.Printf("⏱️  [CallMethod] Iniciando Call() al PLC...")
	resp, err := c.client.Call(ctx, req)
	t5 := time.Now()
	log.Printf("⏱️  [CallMethod] ⚡ PLC Call() completado: %dms (%dus)",
		t5.Sub(t4).Milliseconds(), t5.Sub(t4).Microseconds())

	if err != nil {
		// Si es un error de sesión, la librería debería estar intentando reconectar en background.
		// Le damos una pequeña pausa y reintentamos una vez para darle tiempo a actuar.
		if isSessionError(err) {
			log.Printf("⚠️ Sesión inválida en CallMethod. Pausando 100ms y reintentando...")
			time.Sleep(10 * time.Millisecond)

			// Reintentar la llamada
			tRetry := time.Now()
			resp, err = c.client.Call(ctx, req)
			log.Printf("⏱️  [CallMethod] Retry Call() después de pausa: %dms", time.Since(tRetry).Milliseconds())

			// Si el reintento también falla, ahora sí devolvemos el error.
			if err != nil {
				return nil, fmt.Errorf("error al llamar método %s después de reintento: %w", methodID, err)
			}
		} else {
			return nil, fmt.Errorf("error al llamar método %s: %w", methodID, err)
		}
	}

	// ⏱️ TIMING: Procesar respuesta
	t6 := time.Now()
	log.Printf("🔍 PLC Response: método=%s | statusCode=%s | numOutputs=%d",
		methodID, resp.StatusCode, len(resp.OutputArguments))

	if resp.StatusCode != ua.StatusOK {
		return nil, fmt.Errorf("llamada a método %s con status: %s", methodID, resp.StatusCode)
	}

	// Extraer valores de salida
	var outputValues []interface{}
	for i, outArg := range resp.OutputArguments {
		value := outArg.Value()
		outputValues = append(outputValues, value)
		log.Printf("🔍 PLC Output[%d]: tipo=%T | valor=%v", i, value, value)
	}
	t7 := time.Now()
	log.Printf("⏱️  [CallMethod] Process Response: %dus", t7.Sub(t6).Microseconds())

	// ⏱️ TIMING: Total
	log.Printf("⏱️  [CallMethod] 🎯 TOTAL: %dms (%dus) | PLC Call fue %d%% del total",
		t7.Sub(t0).Milliseconds(),
		t7.Sub(t0).Microseconds(),
		(t5.Sub(t4).Microseconds()*100)/t7.Sub(t0).Microseconds())

	return outputValues, nil
}

// MonitorNode crea una suscripción para monitorear cambios en un nodo
// Devuelve un canal que emite los nuevos valores y una función para cancelar la suscripción
func (c *Client) MonitorNode(ctx context.Context, nodeID string, interval time.Duration) (<-chan *NodeInfo, func(), error) {
	if c.client == nil {
		return nil, nil, fmt.Errorf("cliente no conectado")
	}

	id, err := ua.ParseNodeID(nodeID)
	if err != nil {
		return nil, nil, fmt.Errorf("nodeID inválido '%s': %w", nodeID, err)
	}

	// Canal para recibir notificaciones del servidor
	notifsChan := make(chan *opcua.PublishNotificationData, 10)

	// Crear suscripción
	sub, err := c.client.Subscribe(ctx, &opcua.SubscriptionParameters{
		Interval: interval,
	}, notifsChan)
	if err != nil {
		return nil, nil, fmt.Errorf("error al crear suscripción: %w", err)
	}

	// Canal para enviar notificaciones al usuario
	userChan := make(chan *NodeInfo, 10)

	// Configurar monitoreo del nodo
	miRequest := opcua.NewMonitoredItemCreateRequestWithDefaults(id, ua.AttributeIDValue, 0)
	res, err := sub.Monitor(ctx, ua.TimestampsToReturnBoth, miRequest)
	if err != nil {
		sub.Cancel(ctx)
		close(notifsChan)
		return nil, nil, fmt.Errorf("error al monitorear nodo: %w", err)
	}

	if res.Results[0].StatusCode != ua.StatusOK {
		sub.Cancel(ctx)
		close(notifsChan)
		return nil, nil, fmt.Errorf("monitoreo con status: %s", res.Results[0].StatusCode)
	}

	log.Printf("✅ Suscripción creada para %s (intervalo: %v)", nodeID, interval)

	// Goroutine para procesar notificaciones
	go func() {
		defer close(userChan)
		for {
			select {
			case <-ctx.Done():
				return
			case msg, ok := <-notifsChan:
				if !ok {
					return
				}
				if msg.Error != nil {
					log.Printf("⚠️ Error en notificación: %v", msg.Error)
					continue
				}

				switch notification := msg.Value.(type) {
				case *ua.DataChangeNotification:
					for _, item := range notification.MonitoredItems {
						if item.Value.Status == ua.StatusOK {
							nodeInfo := &NodeInfo{
								NodeID:    nodeID,
								Value:     item.Value.Value.Value(),
								ValueType: fmt.Sprintf("%T", item.Value.Value.Value()),
							}
							select {
							case userChan <- nodeInfo:
							default:
								// Canal lleno, descartar valor antiguo
							}
						}
					}
				}
			}
		}
	}()

	// Función para cancelar suscripción
	cancelFunc := func() {
		sub.Cancel(context.Background())
		close(notifsChan)
	}

	return userChan, cancelFunc, nil
}

// MonitorMultipleNodes crea UNA suscripción para monitorear MÚLTIPLES nodos
// Esto evita el error "StatusBadTooManySubscriptions"
func (c *Client) MonitorMultipleNodes(ctx context.Context, nodeIDs []string, interval time.Duration) (<-chan *NodeInfo, func(), error) {
	if c.client == nil {
		return nil, nil, fmt.Errorf("cliente no conectado")
	}

	if len(nodeIDs) == 0 {
		return nil, nil, fmt.Errorf("no se proporcionaron nodeIDs")
	}

	// Parsear todos los nodeIDs
	parsedNodes := make([]*ua.NodeID, 0, len(nodeIDs))
	nodeIDMap := make(map[uint32]string) // clientHandle -> nodeID original

	for _, nodeID := range nodeIDs {
		id, err := ua.ParseNodeID(nodeID)
		if err != nil {
			log.Printf("⚠️ NodeID inválido '%s', se omitirá: %v", nodeID, err)
			continue
		}
		parsedNodes = append(parsedNodes, id)
	}

	if len(parsedNodes) == 0 {
		return nil, nil, fmt.Errorf("ningún nodeID válido")
	}

	// Canal para recibir notificaciones del servidor
	notifsChan := make(chan *opcua.PublishNotificationData, 10)

	// Crear UNA suscripción para todos los nodos
	sub, err := c.client.Subscribe(ctx, &opcua.SubscriptionParameters{
		Interval: interval,
	}, notifsChan)
	if err != nil {
		return nil, nil, fmt.Errorf("error al crear suscripción: %w", err)
	}

	// Crear monitored items para todos los nodos
	monitorRequests := make([]*ua.MonitoredItemCreateRequest, 0, len(parsedNodes))
	for i, id := range parsedNodes {
		req := opcua.NewMonitoredItemCreateRequestWithDefaults(id, ua.AttributeIDValue, uint32(i))
		monitorRequests = append(monitorRequests, req)
		nodeIDMap[uint32(i)] = nodeIDs[i]
	}

	res, err := sub.Monitor(ctx, ua.TimestampsToReturnBoth, monitorRequests...)
	if err != nil {
		sub.Cancel(ctx)
		close(notifsChan)
		return nil, nil, fmt.Errorf("error al monitorear nodos: %w", err)
	}

	// Verificar resultados
	successCount := 0
	for i, result := range res.Results {
		if result.StatusCode == ua.StatusOK {
			successCount++
		} else {
			log.Printf("⚠️ Error al monitorear nodo %s: %s", nodeIDs[i], result.StatusCode)
		}
	}

	if successCount == 0 {
		sub.Cancel(ctx)
		close(notifsChan)
		return nil, nil, fmt.Errorf("no se pudo monitorear ningún nodo")
	}

	log.Printf("✅ Suscripción creada para %d nodos (intervalo: %v)", successCount, interval)

	// Canal para enviar notificaciones al usuario
	userChan := make(chan *NodeInfo, 50) // Buffer más grande para múltiples nodos

	// Goroutine para procesar notificaciones
	go func() {
		defer close(userChan)
		for {
			select {
			case <-ctx.Done():
				return
			case msg, ok := <-notifsChan:
				if !ok {
					return
				}
				if msg.Error != nil {
					log.Printf("⚠️ Error en notificación: %v", msg.Error)
					continue
				}

				switch notification := msg.Value.(type) {
				case *ua.DataChangeNotification:
					for _, item := range notification.MonitoredItems {
						if item.Value.Status == ua.StatusOK {
							// Obtener el nodeID original usando el clientHandle
							originalNodeID := nodeIDMap[item.ClientHandle]
							nodeInfo := &NodeInfo{
								NodeID:    originalNodeID,
								Value:     item.Value.Value.Value(),
								ValueType: fmt.Sprintf("%T", item.Value.Value.Value()),
							}
							select {
							case userChan <- nodeInfo:
							default:
								// Canal lleno, descartar valor antiguo
							}
						}
					}
				}
			}
		}
	}()

	// Función para cancelar suscripción
	cancelFunc := func() {
		sub.Cancel(context.Background())
		close(notifsChan)
	}

	return userChan, cancelFunc, nil
}
