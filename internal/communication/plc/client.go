package plc

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/gopcua/opcua"
	"github.com/gopcua/opcua/ua"
)

// Client encapsula la conexión a un servidor OPC UA
type Client struct {
	endpoint string
	client   *opcua.Client
	config   PLCConfig
}

// NewClient crea un nuevo cliente OPC UA sin conectar
func NewClient(config PLCConfig) *Client {
	return &Client{
		endpoint: config.Endpoint,
		config:   config,
	}
}

// Connect establece la conexión con el servidor OPC UA
func (c *Client) Connect(ctx context.Context) error {
	opts := []opcua.Option{
		opcua.SecurityMode(ua.MessageSecurityModeNone),
		opcua.SecurityPolicy(ua.SecurityPolicyURINone),
		opcua.AutoReconnect(true),
	}

	client, err := opcua.NewClient(c.endpoint, opts...)
	if err != nil {
		return fmt.Errorf("error creando cliente para %s: %w", c.endpoint, err)
	}

	if err := client.Connect(ctx); err != nil {
		return fmt.Errorf("error al conectar a %s: %w", c.endpoint, err)
	}

	c.client = client
	log.Printf("✅ Conexión establecida a %s", c.endpoint)
	return nil
}

// Close cierra la conexión con el servidor OPC UA
func (c *Client) Close(ctx context.Context) error {
	if c.client != nil {
		return c.client.Close(ctx)
	}
	return nil
}

// ReadNode lee el valor de un nodo específico
func (c *Client) ReadNode(ctx context.Context, nodeID string) (*NodeInfo, error) {
	if c.client == nil {
		return nil, fmt.Errorf("cliente no conectado")
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
		return nil, fmt.Errorf("error al leer nodo %s: %w", nodeID, err)
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
		ReadTime:  time.Now(),
		Error:     nil,
	}

	return nodeInfo, nil
}

// ReadMultipleNodes lee múltiples nodos en una sola operación (más eficiente)
func (c *Client) ReadMultipleNodes(ctx context.Context, nodeIDs []string) ([]*NodeInfo, error) {
	if c.client == nil {
		return nil, fmt.Errorf("cliente no conectado")
	}

	if len(nodeIDs) == 0 {
		return []*NodeInfo{}, nil
	}

	// Parsear todos los NodeIDs
	nodesToRead := make([]*ua.ReadValueID, 0, len(nodeIDs))
	for _, nodeID := range nodeIDs {
		id, err := ua.ParseNodeID(nodeID)
		if err != nil {
			log.Printf("⚠️  NodeID inválido '%s': %v", nodeID, err)
			continue
		}
		nodesToRead = append(nodesToRead, &ua.ReadValueID{
			NodeID:      id,
			AttributeID: ua.AttributeIDValue,
		})
	}

	// Ejecutar lectura batch
	req := &ua.ReadRequest{NodesToRead: nodesToRead}
	resp, err := c.client.Read(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("error en lectura múltiple: %w", err)
	}

	// Procesar resultados
	results := make([]*NodeInfo, 0, len(resp.Results))
	for i, result := range resp.Results {
		nodeID := nodeIDs[i]

		nodeInfo := &NodeInfo{
			NodeID:   nodeID,
			ReadTime: time.Now(),
		}

		if result.Status != ua.StatusOK {
			nodeInfo.Error = fmt.Errorf("status: %s", result.Status)
		} else {
			nodeInfo.Value = result.Value.Value()
			nodeInfo.ValueType = fmt.Sprintf("%T", nodeInfo.Value)
		}

		results = append(results, nodeInfo)
	}

	return results, nil
}

// WriteNode escribe un valor a un nodo específico
func (c *Client) WriteNode(ctx context.Context, nodeID string, value interface{}) error {
	if c.client == nil {
		return fmt.Errorf("cliente no conectado")
	}

	// Parsear el NodeID
	id, err := ua.ParseNodeID(nodeID)
	if err != nil {
		return fmt.Errorf("nodeID inválido '%s': %w", nodeID, err)
	}

	// Crear variante con el valor
	variant, err := ua.NewVariant(value)
	if err != nil {
		return fmt.Errorf("error creando variante para %v: %w", value, err)
	}

	// Crear request de escritura
	req := &ua.WriteRequest{
		NodesToWrite: []*ua.WriteValue{
			{
				NodeID:      id,
				AttributeID: ua.AttributeIDValue,
				Value: &ua.DataValue{
					EncodingMask: ua.DataValueValue,
					Value:        variant,
				},
			},
		},
	}

	// Ejecutar escritura
	resp, err := c.client.Write(ctx, req)
	if err != nil {
		return fmt.Errorf("error al escribir nodo %s: %w", nodeID, err)
	}

	// Validar respuesta
	if len(resp.Results) == 0 {
		return fmt.Errorf("escritura de %s sin resultados", nodeID)
	}

	if resp.Results[0] != ua.StatusOK {
		return fmt.Errorf("escritura de %s con status: %s", nodeID, resp.Results[0])
	}

	log.Printf("✅ Escritura exitosa en %s: %v (%T)", nodeID, value, value)
	return nil
}

// CallMethod ejecuta un método OPC UA en el PLC
// objectID: NodeID del objeto que contiene el método
// methodID: NodeID del método a ejecutar
// inputArgs: Argumentos de entrada para el método (pueden ser vacíos)
func (c *Client) CallMethod(ctx context.Context, objectID string, methodID string, inputArgs ...interface{}) (*MethodCallResult, error) {
	if c.client == nil {
		return nil, fmt.Errorf("cliente no conectado")
	}

	// Parsear NodeIDs
	objID, err := ua.ParseNodeID(objectID)
	if err != nil {
		return nil, fmt.Errorf("objectID inválido '%s': %w", objectID, err)
	}

	methID, err := ua.ParseNodeID(methodID)
	if err != nil {
		return nil, fmt.Errorf("methodID inválido '%s': %w", methodID, err)
	}

	// Convertir argumentos de entrada a Variants
	variants := make([]*ua.Variant, len(inputArgs))
	for i, arg := range inputArgs {
		v, err := ua.NewVariant(arg)
		if err != nil {
			return nil, fmt.Errorf("argumento de entrada #%d inválido: %w", i, err)
		}
		variants[i] = v
	}

	log.Printf("📞 Llamando método %s (objeto: %s) con %d argumento(s)", methodID, objectID, len(variants))

	// Crear request de llamada a método
	req := &ua.CallMethodRequest{
		ObjectID:       objID,
		MethodID:       methID,
		InputArguments: variants,
	}

	// Ejecutar llamada
	result, err := c.client.Call(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("error en llamada a método: %w", err)
	}

	// Preparar resultado
	methodResult := &MethodCallResult{
		StatusCode: result.StatusCode.Error(),
	}

	if result.StatusCode != ua.StatusOK {
		methodResult.Error = fmt.Errorf("método retornó status: %s", result.StatusCode)
		return methodResult, methodResult.Error
	}

	// Extraer argumentos de salida
	methodResult.OutputArguments = make([]interface{}, len(result.OutputArguments))
	for i, outArg := range result.OutputArguments {
		methodResult.OutputArguments[i] = outArg.Value()
	}

	log.Printf("✅ Método ejecutado exitosamente. %d valor(es) retornado(s)", len(methodResult.OutputArguments))

	return methodResult, nil
}
