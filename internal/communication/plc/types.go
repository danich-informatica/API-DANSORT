package plc

import (
	"github.com/gopcua/opcua/ua"
)

// NodeInfo representa la información de un nodo OPC UA y su valor leído.
// Es la unidad fundamental de datos que se pasa a través del sistema.
type NodeInfo struct {
	NodeID    string      // ID del nodo (ej: "ns=4;i=22")
	Name      string      // Nombre descriptivo del nodo (ej: "Input", "Estado")
	Value     interface{} // Valor leído del nodo
	ValueType string      // Tipo del valor como string (bool, int16, etc.)
	DataType  ua.TypeID   // Tipo de dato OPC UA subyacente
	IsPLCNode bool        // Indica si es un nodo principal del PLC (Input/Output)
	Error     error       // Error si hubo problema al leer
}

// SalidaNodes agrupa los nodos de una salida específica.
type SalidaNodes struct {
	ID         int        // ID de la salida desde la config
	Name       string     // Nombre de la salida
	PhysicalID int        // ID físico que se envía al PLC
	Nodes      []NodeInfo // Nodos asociados (Estado, Bloqueo)
}

// SorterNodes agrupa todos los nodos de un sorter.
type SorterNodes struct {
	SorterID   int           // ID del sorter
	SorterName string        // Nombre del sorter
	Endpoint   string        // Endpoint OPC UA
	Nodes      []NodeInfo    // Nodos principales del PLC (Input/Output)
	Salidas    []SalidaNodes // Lista de nodos de cada salida
}

// PLCConfig contiene la configuración necesaria para conectarse a un PLC.
type PLCConfig struct {
	Endpoint string
}

// BrowseResult representa un nodo descubierto durante la exploración.
type BrowseResult struct {
	NodeID        string
	BrowseName    string
	DisplayName   string
	NodeClass     ua.NodeClass
	IsForward     bool
	ReferenceType string
}
