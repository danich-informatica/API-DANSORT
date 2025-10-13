package plc

import "time"

// NodeInfo representa la información de un nodo OPC UA y su valor leído
type NodeInfo struct {
	NodeID      string      // ID del nodo (ej: "ns=4;i=22")
	Description string      // Descripción legible (ej: "PLC Input", "Salida 1 - Estado")
	Value       interface{} // Valor leído del nodo
	ValueType   string      // Tipo del valor (bool, int16, int32, string, etc.)
	ReadTime    time.Time   // Momento de la lectura
	Error       error       // Error si hubo problema al leer
}

// SorterNodes agrupa todos los nodos de un sorter
type SorterNodes struct {
	SorterID    int           // ID del sorter
	SorterName  string        // Nombre del sorter
	Endpoint    string        // Endpoint OPC UA (ej: "opc.tcp://192.168.120.100:4840")
	InputNode   *NodeInfo     // Nodo de entrada del PLC
	OutputNode  *NodeInfo     // Nodo de salida del PLC
	SalidaNodes []SalidaNodes // Lista de nodos de cada salida
}

// SalidaNodes agrupa los nodos de una salida específica
type SalidaNodes struct {
	SalidaID    int       // ID de la salida
	SalidaName  string    // Nombre de la salida
	PhysicalID  int       // ID físico que se envía al PLC
	EstadoNode  *NodeInfo // Nodo de estado
	BloqueoNode *NodeInfo // Nodo de bloqueo (puede ser nil si no existe)
}

// PLCConfig contiene la configuración necesaria para conectarse a un PLC
type PLCConfig struct {
	Endpoint       string
	ConnectTimeout time.Duration
	ReadTimeout    time.Duration
}

// MethodCallRequest representa una solicitud de llamada a un método OPC UA
type MethodCallRequest struct {
	ObjectID       string        // NodeID del objeto que contiene el método
	MethodID       string        // NodeID del método a llamar
	InputArguments []interface{} // Argumentos de entrada para el método
}

// MethodCallResult representa el resultado de una llamada a método
type MethodCallResult struct {
	OutputArguments []interface{} // Valores retornados por el método
	StatusCode      string        // Código de estado de la operación
	Error           error         // Error si la llamada falló
}
