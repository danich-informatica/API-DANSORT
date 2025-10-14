package plc

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"API-GREENEX/internal/config"

	"github.com/gopcua/opcua/ua"
)

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

// AssignLaneToBox replica el comportamiento del código Rust:
// 1. Intenta llamar al método del PLC para asignar una caja a una salida
// 2. Si falla (método bloqueado), escribe directamente en el nodo ESTADO como Plan B
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

	// Plan A: Intentar llamar al método con el NÚMERO de salida como int16
	if sorterConfig.PLC.ObjectID != "" && sorterConfig.PLC.MethodID != "" {
		log.Printf("🔄 [Sorter %d] Intentando método PLC con número de salida %d...", sorterID, laneNumber)

		// CRÍTICO: El método espera int16 con el número de salida, NO un NodeID
		variant := ua.MustVariant(laneNumber) // laneNumber ya es int16
		inputArgs := []*ua.Variant{variant}

		outputValues, err := m.CallMethod(ctx, sorterID, sorterConfig.PLC.ObjectID, sorterConfig.PLC.MethodID, inputArgs)

		if err == nil {
			log.Printf("✅ [Sorter %d] Método ejecutado exitosamente - Lane %d asignado. Output: %v", sorterID, laneNumber, outputValues)
			return nil
		}

		// Si falla, loguear warning pero continuar con Plan B (como Rust)
		log.Printf("⚠️ [Sorter %d] Método falló: %v - usando Plan B (escritura directa)", sorterID, err)
	}

	// Plan B: Escribir directamente en el nodo BLOQUEO de la salida
	// (ESTADO es read-only, pero BLOQUEO sí es escribible)
	var bloqueoNodeID string
	for _, salida := range sorterConfig.Salidas {
		if salida.PhysicalID == int(laneNumber) {
			bloqueoNodeID = salida.PLC.BloqueoNodeID
			break
		}
	}

	if bloqueoNodeID == "" {
		return fmt.Errorf("no se encontró nodo BLOQUEO para lane %d en sorter %d", laneNumber, sorterID)
	}

	log.Printf("📝 [Sorter %d] Escribiendo directamente en nodo BLOQUEO %s", sorterID, bloqueoNodeID)

	// Escribir false en BLOQUEO para desbloquear/habilitar la salida
	err := m.WriteNodeTyped(ctx, sorterID, bloqueoNodeID, false, "bool")
	if err != nil {
		return fmt.Errorf("plan B falló - no se pudo escribir en BLOQUEO: %w", err)
	}

	log.Printf("✅ [Sorter %d] Plan B exitoso - Lane %d desbloqueado", sorterID, laneNumber)
	return nil
}
