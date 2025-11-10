package plc

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"api-dansort/internal/config"

	"github.com/gopcua/opcua/ua"
)

// logTs imprime log con timestamp de microsegundos para debugging preciso
func logTs(format string, args ...interface{}) {
	ts := time.Now().Format("2006-01-02T15:04:05.000000")
	log.Printf("[%s] "+format, append([]interface{}{ts}, args...)...)
}

// Manager gestiona múltiples clientes OPC UA para diferentes sorters
type Manager struct {
	config       *config.Config
	clients      map[string]*Client
	clientsMutex sync.RWMutex
}

// NewManager crea un nuevo gestor de clientes OPC UA
func NewManager(cfg *config.Config) *Manager {
	return &Manager{
		clients: make(map[string]*Client),
		config:  cfg,
	}
}

// ConnectAll establece conexiones con todos los PLCs configurados
func (m *Manager) ConnectAll(ctx context.Context) error {
	endpoints := make(map[string]bool)
	for _, sorter := range m.config.Sorters {
		if sorter.PLCEndpoint != "" {
			endpoints[sorter.PLCEndpoint] = true
		}
	}

	if len(endpoints) == 0 {
		return fmt.Errorf("no hay endpoints OPC UA configurados")
	}

	log.Printf("📡 Conectando a %d endpoint(s) OPC UA...", len(endpoints))

	var wg sync.WaitGroup
	errChan := make(chan error, len(endpoints))

	for endpoint := range endpoints {
		wg.Add(1)
		go func(ep string) {
			defer wg.Done()
			client := NewClient(PLCConfig{Endpoint: ep})
			if err := client.Connect(ctx); err != nil {
				errChan <- fmt.Errorf("error conectando a %s: %w", ep, err)
				return
			}
			m.clientsMutex.Lock()
			m.clients[ep] = client
			m.clientsMutex.Unlock()
			log.Printf("✅ Conexión establecida con %s", ep)
		}(endpoint)
	}

	wg.Wait()
	close(errChan)

	// Recoger el primer error que haya ocurrido, si lo hay.
	for err := range errChan {
		if err != nil {
			// Devolvemos el primer error y cerramos las conexiones existentes
			m.CloseAll(ctx)
			return err
		}
	}

	log.Printf("✅ Todas las conexiones OPC UA establecidas correctamente")
	return nil
}

// CloseAll cierra todas las conexiones
func (m *Manager) CloseAll(ctx context.Context) {
	m.clientsMutex.Lock()
	defer m.clientsMutex.Unlock()
	for endpoint, client := range m.clients {
		if err := client.Close(ctx); err != nil {
			log.Printf("⚠️  Error cerrando conexión a %s: %v", endpoint, err)
		} else {
			log.Printf("🔌 Conexión cerrada: %s", endpoint)
		}
	}
	m.clients = make(map[string]*Client)
}

// getClientForSorter busca el cliente OPC UA para un sorter específico.
func (m *Manager) getClientForSorter(sorterID int) (*Client, *config.Sorter, error) {
	var sorterCfg *config.Sorter
	for i := range m.config.Sorters {
		if m.config.Sorters[i].ID == sorterID {
			sorterCfg = &m.config.Sorters[i]
			break
		}
	}
	if sorterCfg == nil {
		return nil, nil, fmt.Errorf("sorter %d no encontrado", sorterID)
	}

	m.clientsMutex.RLock()
	defer m.clientsMutex.RUnlock()
	client, exists := m.clients[sorterCfg.PLCEndpoint]
	if !exists {
		return nil, nil, fmt.Errorf("no hay cliente conectado para el endpoint %s del sorter %d", sorterCfg.PLCEndpoint, sorterID)
	}
	return client, sorterCfg, nil
}

// ReadNode lee un nodo específico de un sorter
func (m *Manager) ReadNode(ctx context.Context, sorterID int, nodeID string) (*NodeInfo, error) {
	client, _, err := m.getClientForSorter(sorterID)
	if err != nil {
		return nil, err
	}
	return client.ReadNode(ctx, nodeID)
}

// WriteNode escribe en un nodo específico de un sorter
func (m *Manager) WriteNode(ctx context.Context, sorterID int, nodeID string, value interface{}) error {
	client, _, err := m.getClientForSorter(sorterID)
	if err != nil {
		return err
	}
	return client.WriteNode(ctx, nodeID, value)
}

// WriteNodeTyped escribe en un nodo con conversión de tipo explícita (como dantrack)
func (m *Manager) WriteNodeTyped(ctx context.Context, sorterID int, nodeID string, value interface{}, dataType string) error {
	client, _, err := m.getClientForSorter(sorterID)
	if err != nil {
		return err
	}
	return client.WriteNodeTyped(ctx, nodeID, value, dataType)
}

// AppendToArrayNode agrega un elemento a un array de ExtensionObject en un nodo de un sorter
func (m *Manager) AppendToArrayNode(ctx context.Context, sorterID int, nodeID string, newElement interface{}) error {
	client, _, err := m.getClientForSorter(sorterID)
	if err != nil {
		return err
	}

	// Convertir el elemento a ExtensionObject si es necesario
	extObj, ok := newElement.(*ua.ExtensionObject)
	if !ok {
		return fmt.Errorf("el elemento debe ser un *ua.ExtensionObject")
	}

	return client.AppendToArrayNode(ctx, nodeID, extObj)
}

// AppendToUInt32Array agrega un elemento a un array de uint32 en un nodo de un sorter
func (m *Manager) AppendToUInt32Array(ctx context.Context, sorterID int, nodeID string, newValue uint32) error {
	client, _, err := m.getClientForSorter(sorterID)
	if err != nil {
		return err
	}

	return client.AppendToUInt32Array(ctx, nodeID, newValue)
}

// BrowseNode explora los nodos hijos de un nodo específico de un sorter
func (m *Manager) BrowseNode(ctx context.Context, sorterID int, nodeID string) ([]BrowseResult, error) {
	client, _, err := m.getClientForSorter(sorterID)
	if err != nil {
		return nil, err
	}

	return client.BrowseNode(ctx, nodeID)
}

// CallMethod invoca un método OPC UA en un sorter específico
func (m *Manager) CallMethod(ctx context.Context, sorterID int, objectID string, methodID string, inputArgs []*ua.Variant) ([]interface{}, error) {
	client, _, err := m.getClientForSorter(sorterID)
	if err != nil {
		return nil, err
	}

	return client.CallMethod(ctx, objectID, methodID, inputArgs)
}

// ReadAllSorterNodes lee todos los nodos de todos los sorters configurados de forma masiva.
func (m *Manager) ReadAllSorterNodes(ctx context.Context) ([]SorterNodes, error) {
	var allSortersData []SorterNodes
	m.clientsMutex.RLock()
	defer m.clientsMutex.RUnlock()

	type nodeMeta struct {
		id        string
		name      string
		isPLCNode bool
		salidaID  int // -1 si no es de una salida
	}

	for _, sorterCfg := range m.config.Sorters {
		client, ok := m.clients[sorterCfg.PLCEndpoint]
		if !ok {
			log.Printf("Advertencia: No se encontró cliente para el endpoint %s del sorter %d", sorterCfg.PLCEndpoint, sorterCfg.ID)
			continue
		}

		// 1. Recopilar todos los NodeIDs y sus metadatos para este sorter
		var metas []nodeMeta
		if sorterCfg.PLC.InputNodeID != "" {
			metas = append(metas, nodeMeta{sorterCfg.PLC.InputNodeID, "Input", true, -1})
		}
		if sorterCfg.PLC.OutputNodeID != "" {
			metas = append(metas, nodeMeta{sorterCfg.PLC.OutputNodeID, "Output", true, -1})
		}
		for _, salidaCfg := range sorterCfg.Salidas {
			if salidaCfg.PLC.EstadoNodeID != "" {
				metas = append(metas, nodeMeta{salidaCfg.PLC.EstadoNodeID, "Estado", false, salidaCfg.ID})
			}
			if salidaCfg.PLC.BloqueoNodeID != "" {
				metas = append(metas, nodeMeta{salidaCfg.PLC.BloqueoNodeID, "Bloqueo", false, salidaCfg.ID})
			}
		}

		if len(metas) == 0 {
			continue // No hay nodos que leer para este sorter
		}

		nodeIDs := make([]string, len(metas))
		for i, meta := range metas {
			nodeIDs[i] = meta.id
		}

		// 2. Leer todos los nodos en una sola llamada
		nodeInfos, _ := client.ReadNodes(ctx, nodeIDs) // Ignoramos el error general para procesar resultados parciales

		// 3. Construir la estructura de datos con los resultados
		sorterData := SorterNodes{
			SorterID:   sorterCfg.ID,
			SorterName: sorterCfg.Name,
			Endpoint:   sorterCfg.PLCEndpoint,
			Nodes:      []NodeInfo{},
			Salidas:    make([]SalidaNodes, len(sorterCfg.Salidas)),
		}
		salidaMap := make(map[int]*SalidaNodes)
		for i, s := range sorterCfg.Salidas {
			sorterData.Salidas[i] = SalidaNodes{
				ID:         s.ID,
				Name:       s.Nombre,
				PhysicalID: s.PhysicalID,
				Nodes:      []NodeInfo{},
			}
			salidaMap[s.ID] = &sorterData.Salidas[i]
		}

		for i, meta := range metas {
			info := nodeInfos[i] // info es un puntero a NodeInfo
			info.Name = meta.name
			info.IsPLCNode = meta.isPLCNode

			if meta.isPLCNode {
				sorterData.Nodes = append(sorterData.Nodes, *info)
			} else {
				if salida, ok := salidaMap[meta.salidaID]; ok {
					salida.Nodes = append(salida.Nodes, *info)
				}
			}
		}
		allSortersData = append(allSortersData, sorterData)
	}

	return allSortersData, nil
}

// GetCacheStats retorna estadísticas de la cache de cada cliente
func (m *Manager) GetCacheStats() map[string]int {
	m.clientsMutex.RLock()
	defer m.clientsMutex.RUnlock()

	stats := make(map[string]int)
	for endpoint, client := range m.clients {
		stats[endpoint] = client.cache.Len()
	}

	return stats
}

// MonitorNode crea una suscripción para monitorear cambios en un nodo de un sorter específico
func (m *Manager) MonitorNode(ctx context.Context, sorterID int, nodeID string, interval time.Duration) (<-chan *NodeInfo, func(), error) {
	// Buscar endpoint del sorter
	var endpoint string
	for _, sorter := range m.config.Sorters {
		if sorter.ID == sorterID {
			endpoint = sorter.PLCEndpoint
			break
		}
	}

	if endpoint == "" {
		return nil, nil, fmt.Errorf("sorter ID %d no encontrado en configuración", sorterID)
	}

	m.clientsMutex.RLock()
	client, exists := m.clients[endpoint]
	m.clientsMutex.RUnlock()

	if !exists {
		return nil, nil, fmt.Errorf("cliente no conectado para endpoint %s", endpoint)
	}

	return client.MonitorNode(ctx, nodeID, interval)
}

// MonitorMultipleNodes crea UNA suscripción para monitorear MÚLTIPLES nodos de un sorter
// Esto evita el error "StatusBadTooManySubscriptions"
func (m *Manager) MonitorMultipleNodes(ctx context.Context, sorterID int, nodeIDs []string, interval time.Duration) (<-chan *NodeInfo, func(), error) {
	// Buscar endpoint del sorter
	var endpoint string
	for _, sorter := range m.config.Sorters {
		if sorter.ID == sorterID {
			endpoint = sorter.PLCEndpoint
			break
		}
	}

	if endpoint == "" {
		return nil, nil, fmt.Errorf("sorter ID %d no encontrado en configuración", sorterID)
	}

	m.clientsMutex.RLock()
	client, exists := m.clients[endpoint]
	m.clientsMutex.RUnlock()

	if !exists {
		return nil, nil, fmt.Errorf("cliente no conectado para endpoint %s", endpoint)
	}

	return client.MonitorMultipleNodes(ctx, nodeIDs, interval)
}

// AssignLaneToBox replica el comportamiento del código Rust:
// Intenta llamar al método del PLC para asignar una caja a una salida
func (m *Manager) AssignLaneToBox(ctx context.Context, sorterID int, laneNumber int16) error {
	// Buscar configuración del sorter
	var sorterConfig *config.Sorter
	for _, sorter := range m.config.Sorters {
		if sorter.ID == sorterID {
			sorterConfig = &sorter
			break
		}
	}

	if sorterConfig == nil {
		return fmt.Errorf("sorter ID %d no encontrado", sorterID)
	}

	// Buscar el NodeID del ESTADO de la salida (el método espera un NodeID, NO un número)
	var estadoNodeID string
	for _, salida := range sorterConfig.Salidas {
		if salida.PhysicalID == int(laneNumber) {
			estadoNodeID = salida.PLC.EstadoNodeID
			break
		}
	}

	if estadoNodeID == "" {
		return fmt.Errorf("no se encontró nodo ESTADO para lane %d en sorter %d", laneNumber, sorterID)
	}

	// Intentar llamar al método con el NÚMERO de salida como int16
	if sorterConfig.PLC.ObjectID != "" && sorterConfig.PLC.MethodID != "" {
		logTs("🔄 [Sorter %d] Intentando método PLC con número de salida %d...", sorterID, laneNumber)

		// ESPERA INTELIGENTE: Verificar trigger antes de llamar al método
		if sorterConfig.PLC.TriggerNodeID != "" {
			logTs("⏳ [Sorter %d] Esperando trigger disponible...", sorterID)
			// warm up al método llamando con un 0 antes de la llamada real

			warmUpVariant := ua.MustVariant(int16(0))
			warmUpInputArgs := []*ua.Variant{warmUpVariant}
			_, err := m.CallMethod(ctx, sorterID, sorterConfig.PLC.ObjectID, sorterConfig.PLC.MethodID, warmUpInputArgs)
			if err != nil {
				logTs("⚠️  [Sorter %d] Error en warm up del método PLC: %v", sorterID, err)
			} else {
				logTs("✅ [Sorter %d] Warm up del método PLC exitoso", sorterID)
			}

			maxWaitTime := 2 * time.Second
			ctxWithTimeout, cancel := context.WithTimeout(ctx, maxWaitTime)
			defer cancel()

			ticker := time.NewTicker(20 * time.Millisecond) // Polling cada 20ms
			defer ticker.Stop()

			startTime := time.Now()
			triggerReady := false

			for !triggerReady {
				select {
				case <-ctxWithTimeout.Done():
					elapsed := time.Since(startTime)
					log.Printf("⚠️  [Sorter %d] Timeout esperando trigger después de %v, continuando de todas formas...", sorterID, elapsed)
					triggerReady = true

				case <-ticker.C:
					// Leer el estado actual del trigger
					nodeInfo, err := m.ReadNode(ctx, sorterID, sorterConfig.PLC.TriggerNodeID)
					if err != nil {
						log.Printf("⚠️  [Sorter %d] Error leyendo trigger: %v", sorterID, err)
						continue // Seguir intentando
					}

					// Verificar si el trigger está en false (disponible)
					if boolValue, ok := nodeInfo.Value.(bool); ok {
						if !boolValue {
							elapsed := time.Since(startTime)
							logTs("✅ [Sorter %d] Trigger disponible después de %v", sorterID, elapsed)
							triggerReady = true
						}
					} else {
						logTs("⚠️  [Sorter %d] Trigger no es booleano (tipo=%T, valor=%v), continuando", sorterID, nodeInfo.Value, nodeInfo.Value)
						triggerReady = true
					}
				}
			}
		} else {
			// Fallback: Si no hay trigger configurado, usar sleep fijo
			log.Printf("⚠️  [Sorter %d] No hay trigger_node_id, usando sleep fijo de 0ms", sorterID)
			//time.Sleep(100 * time.Millisecond)
		}

		// CRÍTICO: El método espera int16 con el número de salida, NO un NodeID
		variant := ua.MustVariant(laneNumber) // laneNumber ya es int16
		inputArgs := []*ua.Variant{variant}

		// 🔁 REINTENTOS: Hasta 3 intentos con política del PLC (25ms entre intentos)
		maxRetries := 3
		var outputValues []interface{}
		var lastErr error

		for attempt := 1; attempt <= maxRetries; attempt++ {
			if attempt > 1 {
				logTs("🔄 [Sorter %d] Reintento %d/%d para Lane %d...", sorterID, attempt, maxRetries, laneNumber)
				time.Sleep(25 * time.Millisecond) // Política del PLC: 25ms entre reintentos
			}

			outputValues, lastErr = m.CallMethod(ctx, sorterID, sorterConfig.PLC.ObjectID, sorterConfig.PLC.MethodID, inputArgs)

			// ✅ ÉXITO: Sin error
			if lastErr == nil {
				// Validar que hay output (algunos PLCs retornan valores de confirmación)
				if len(outputValues) > 0 {
					logTs("✅ [Sorter %d] Método ejecutado - Lane %d asignado. Output: %v", sorterID, laneNumber, outputValues)
				} else {
					logTs("✅ [Sorter %d] Método ejecutado - Lane %d asignado (sin output)", sorterID, laneNumber)
				}
				return nil
			}

			// ⚠️ ERROR DE SESIÓN: Reintentar en cualquier intento
			if isSessionError(lastErr) {
				logTs("⚠️ [Sorter %d] Error de sesión OPC UA en intento %d/%d, reintentando...", sorterID, attempt, maxRetries)
				continue // Continuar al siguiente intento
			}

			// ❌ ERROR NO RECUPERABLE: Salir inmediatamente
			logTs("❌ [Sorter %d] Error no recuperable en intento %d: %v", sorterID, attempt, lastErr)
			break
		}

		// Si llegamos aquí, todos los intentos fallaron - ignorar envío según política del PLC
		logTs("❌ [Sorter %d] Lane %d NO asignado después de %d intentos - IGNORADO", sorterID, laneNumber, maxRetries)
		return fmt.Errorf("error llamando método PLC para lane %d en sorter %d (intentos: %d): %w", laneNumber, sorterID, maxRetries, lastErr)
	}

	// Si no hay ObjectID o MethodID configurado, retornar error
	return fmt.Errorf("no hay método PLC configurado para sorter %d", sorterID)
}

// WaitForSorterReady espera hasta que el trigger del sorter esté en false (disponible)
// Retorna error si excede el timeout o si el nodo trigger no está configurado
func (m *Manager) WaitForSorterReady(ctx context.Context, sorterID int, timeout time.Duration) error {
	// Buscar configuración del sorter
	var sorterConfig *config.Sorter
	for _, sorter := range m.config.Sorters {
		if sorter.ID == sorterID {
			sorterConfig = &sorter
			break
		}
	}

	if sorterConfig == nil {
		return fmt.Errorf("sorter ID %d no encontrado", sorterID)
	}

	// Si no hay trigger configurado, retornar inmediatamente (sin espera)
	if sorterConfig.PLC.TriggerNodeID == "" {
		log.Printf("⚠️  [Sorter %d] No hay trigger_node_id configurado, sin espera inteligente", sorterID)
		return nil
	}

	log.Printf("⏳ [Sorter %d] Esperando a que trigger esté disponible (false)...", sorterID)

	// Crear contexto con timeout
	ctxWithTimeout, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(50 * time.Millisecond) // Verificar cada 50ms
	defer ticker.Stop()

	startTime := time.Now()

	for {
		select {
		case <-ctxWithTimeout.Done():
			elapsed := time.Since(startTime)
			return fmt.Errorf("timeout esperando trigger del sorter %d después de %v", sorterID, elapsed)

		case <-ticker.C:
			// Leer el valor del trigger
			nodeInfo, err := m.ReadNode(ctx, sorterID, sorterConfig.PLC.TriggerNodeID)
			if err != nil {
				log.Printf("⚠️  [Sorter %d] Error leyendo trigger: %v", sorterID, err)
				continue // Reintentar en el siguiente tick
			}

			// Verificar si el valor es booleano y está en false
			if boolValue, ok := nodeInfo.Value.(bool); ok {
				if !boolValue { // Trigger en false = disponible
					elapsed := time.Since(startTime)
					log.Printf("✅ [Sorter %d] Trigger disponible después de %v", sorterID, elapsed)
					return nil
				}
				// Trigger en true = ocupado, seguir esperando
				continue
			}

			// Si no es booleano, loguear advertencia y continuar
			log.Printf("⚠️  [Sorter %d] Trigger no es booleano (tipo=%T, valor=%v)", sorterID, nodeInfo.Value, nodeInfo.Value)
		}
	}
}

func (m *Manager) LockSalida(sorterID int, bloqueoNode string) error {
	if bloqueoNode == "" {
		return fmt.Errorf("nodo de bloqueo vacío")
	}

	ctx := context.Background()
	return m.WriteNode(ctx, sorterID, bloqueoNode, true)
}

func (m *Manager) UnlockSalida(sorterID int, bloqueoNode string) error {
	if bloqueoNode == "" {
		return fmt.Errorf("nodo de bloqueo vacío")
	}

	ctx := context.Background()
	return m.WriteNode(ctx, sorterID, bloqueoNode, false)
}
