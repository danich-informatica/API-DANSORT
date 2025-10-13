package plc

import (
	"context"
	"fmt"
	"log"

	"API-GREENEX/internal/config"
)

// Manager coordina las conexiones a múltiples PLCs y la lectura de nodos
type Manager struct {
	clients map[string]*Client // endpoint -> client
	config  *config.Config
}

// NewManager crea un nuevo manager de PLCs
func NewManager(cfg *config.Config) *Manager {
	return &Manager{
		clients: make(map[string]*Client),
		config:  cfg,
	}
}

// ConnectAll establece conexiones con todos los PLCs configurados
func (m *Manager) ConnectAll(ctx context.Context) error {
	// Identificar todos los endpoints únicos
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

	// Conectar a cada endpoint único
	for endpoint := range endpoints {
		plcConfig := PLCConfig{
			Endpoint: endpoint,
		}

		client := NewClient(plcConfig)
		if err := client.Connect(ctx); err != nil {
			return fmt.Errorf("error conectando a %s: %w", endpoint, err)
		}

		m.clients[endpoint] = client
	}

	log.Printf("✅ Todas las conexiones OPC UA establecidas")
	return nil
}

// CloseAll cierra todas las conexiones
func (m *Manager) CloseAll(ctx context.Context) {
	for endpoint, client := range m.clients {
		if err := client.Close(ctx); err != nil {
			log.Printf("⚠️  Error cerrando conexión a %s: %v", endpoint, err)
		} else {
			log.Printf("🔌 Conexión cerrada: %s", endpoint)
		}
	}
	m.clients = make(map[string]*Client)
}

// getClientForEndpoint obtiene el cliente correspondiente a un endpoint
func (m *Manager) getClientForEndpoint(endpoint string) (*Client, error) {
	client, exists := m.clients[endpoint]
	if !exists {
		return nil, fmt.Errorf("no hay cliente conectado para %s", endpoint)
	}
	return client, nil
}

// ReadNode lee un nodo específico de un sorter
func (m *Manager) ReadNode(ctx context.Context, sorterID int, nodeID string) (*NodeInfo, error) {
	// Buscar sorter por ID
	var sorterCfg *config.Sorter
	for _, s := range m.config.Sorters {
		if s.ID == sorterID {
			sorterCfg = &s
			break
		}
	}
	if sorterCfg == nil {
		return nil, fmt.Errorf("sorter %d no encontrado", sorterID)
	}

	// Obtener cliente
	client, err := m.getClientForEndpoint(sorterCfg.PLCEndpoint)
	if err != nil {
		return nil, err
	}

	// Leer nodo
	return client.ReadNode(ctx, nodeID)
}

// ReadAllSorterNodes lee todos los nodos de todos los sorters configurados
func (m *Manager) ReadAllSorterNodes(ctx context.Context) ([]SorterNodes, error) {
	results := make([]SorterNodes, 0, len(m.config.Sorters))

	for _, sorterCfg := range m.config.Sorters {
		sorterNodes, err := m.ReadSorterNodes(ctx, sorterCfg.ID)
		if err != nil {
			log.Printf("⚠️  Error leyendo sorter %d: %v", sorterCfg.ID, err)
			continue
		}
		results = append(results, *sorterNodes)
	}

	return results, nil
}

// ReadSorterNodes lee todos los nodos de un sorter específico
func (m *Manager) ReadSorterNodes(ctx context.Context, sorterID int) (*SorterNodes, error) {
	// Buscar configuración del sorter
	var sorterCfg *config.Sorter
	for i := range m.config.Sorters {
		if m.config.Sorters[i].ID == sorterID {
			sorterCfg = &m.config.Sorters[i]
			break
		}
	}

	if sorterCfg == nil {
		return nil, fmt.Errorf("sorter %d no encontrado en configuración", sorterID)
	}

	if sorterCfg.PLCEndpoint == "" {
		return nil, fmt.Errorf("sorter %d no tiene endpoint OPC UA configurado", sorterID)
	}

	// Obtener cliente para este endpoint
	client, err := m.getClientForEndpoint(sorterCfg.PLCEndpoint)
	if err != nil {
		return nil, err
	}

	// Inicializar estructura de resultado
	sorterNodes := &SorterNodes{
		SorterID:   sorterCfg.ID,
		SorterName: sorterCfg.Name,
		Endpoint:   sorterCfg.PLCEndpoint,
	}

	// Leer nodo de entrada del PLC
	if sorterCfg.PLCInputNode != "" {
		inputNode, err := client.ReadNode(ctx, sorterCfg.PLCInputNode)
		if err != nil {
			log.Printf("⚠️  Error leyendo input node: %v", err)
			inputNode = &NodeInfo{
				NodeID:      sorterCfg.PLCInputNode,
				Description: "PLC Input",
				Error:       err,
			}
		} else {
			inputNode.Description = "PLC Input"
		}
		sorterNodes.InputNode = inputNode
	}

	// Leer nodo de salida del PLC
	if sorterCfg.PLCOutputNode != "" {
		outputNode, err := client.ReadNode(ctx, sorterCfg.PLCOutputNode)
		if err != nil {
			log.Printf("⚠️  Error leyendo output node: %v", err)
			outputNode = &NodeInfo{
				NodeID:      sorterCfg.PLCOutputNode,
				Description: "PLC Output",
				Error:       err,
			}
		} else {
			outputNode.Description = "PLC Output"
		}
		sorterNodes.OutputNode = outputNode
	}

	// Leer nodos de cada salida
	sorterNodes.SalidaNodes = make([]SalidaNodes, 0, len(sorterCfg.Salidas))
	for _, salidaCfg := range sorterCfg.Salidas {
		salidaNodes := m.readSalidaNodes(ctx, client, &salidaCfg)
		sorterNodes.SalidaNodes = append(sorterNodes.SalidaNodes, *salidaNodes)
	}

	return sorterNodes, nil
}

// readSalidaNodes lee los nodos de una salida específica
func (m *Manager) readSalidaNodes(ctx context.Context, client *Client, salidaCfg *config.Salida) *SalidaNodes {
	salidaNodes := &SalidaNodes{
		SalidaID:   salidaCfg.ID,
		SalidaName: salidaCfg.Name,
		PhysicalID: salidaCfg.PhysicalID,
	}

	// Leer nodo de estado
	if salidaCfg.EstadoNode != "" {
		estadoNode, err := client.ReadNode(ctx, salidaCfg.EstadoNode)
		if err != nil {
			log.Printf("⚠️  Error leyendo estado node de salida %d: %v", salidaCfg.ID, err)
			estadoNode = &NodeInfo{
				NodeID:      salidaCfg.EstadoNode,
				Description: fmt.Sprintf("Salida %d - Estado", salidaCfg.ID),
				Error:       err,
			}
		} else {
			estadoNode.Description = fmt.Sprintf("Salida %d - Estado", salidaCfg.ID)
		}
		salidaNodes.EstadoNode = estadoNode
	}

	// Leer nodo de bloqueo (si existe)
	if salidaCfg.BloqueoNode != "" {
		bloqueoNode, err := client.ReadNode(ctx, salidaCfg.BloqueoNode)
		if err != nil {
			log.Printf("⚠️  Error leyendo bloqueo node de salida %d: %v", salidaCfg.ID, err)
			bloqueoNode = &NodeInfo{
				NodeID:      salidaCfg.BloqueoNode,
				Description: fmt.Sprintf("Salida %d - Bloqueo", salidaCfg.ID),
				Error:       err,
			}
		} else {
			bloqueoNode.Description = fmt.Sprintf("Salida %d - Bloqueo", salidaCfg.ID)
		}
		salidaNodes.BloqueoNode = bloqueoNode
	}

	return salidaNodes
}

// CallSorterMethod llama al método OPC UA configurado para un sorter
// Útil para enviar comandos al PLC (ej: enviar physical_id de salida seleccionada)
func (m *Manager) CallSorterMethod(ctx context.Context, sorterID int, inputArgs ...interface{}) (*MethodCallResult, error) {
	// Buscar configuración del sorter
	var sorterCfg *config.Sorter
	for i := range m.config.Sorters {
		if m.config.Sorters[i].ID == sorterID {
			sorterCfg = &m.config.Sorters[i]
			break
		}
	}

	if sorterCfg == nil {
		return nil, fmt.Errorf("sorter %d no encontrado", sorterID)
	}

	if sorterCfg.PLCObjectID == "" || sorterCfg.PLCMethodID == "" {
		return nil, fmt.Errorf("sorter %d no tiene método OPC UA configurado (plc_object_id o plc_method_id vacíos)", sorterID)
	}

	// Obtener cliente
	client, err := m.getClientForEndpoint(sorterCfg.PLCEndpoint)
	if err != nil {
		return nil, err
	}

	// Llamar al método
	return client.CallMethod(ctx, sorterCfg.PLCObjectID, sorterCfg.PLCMethodID, inputArgs...)
}
