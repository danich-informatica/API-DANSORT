package shared

import "fmt"

// Constantes OPC UA correspondientes a las definidas en Rust
const (
    // Namespace OPC UA
    OPCUA_NAMESPACE uint16 = 4
    
    // Métodos OPC UA
    // OPCUA_HEARTBEAT_METHOD int32 = 3  // Comentado como en Rust
    OPCUA_SEGREGATION_METHOD int32 = 2
    
    // Timeout OPC UA (en segundos)
    OPCUA_TIMEOUT uint64 = 3
)

// Constantes adicionales para la aplicación Go
const (
    // NodeIDs por defecto usando el namespace
    DEFAULT_HEARTBEAT_NODE    = "ns=4;i=3"
    DEFAULT_SEGREGATION_NODE  = "ns=4;i=2"
    
    // Intervalos de tiempo
    DEFAULT_SUBSCRIPTION_INTERVAL = "1000ms"
    DEFAULT_CONNECTION_TIMEOUT    = "30s"
    
    // Nombres de suscripciones por defecto
    HEARTBEAT_SUBSCRIPTION    = "heartbeat_subscription"
    SEGREGATION_SUBSCRIPTION  = "segregation_subscription"
    DEFAULT_SUBSCRIPTION      = "default_subscription"
)

// Función helper para construir NodeIDs con el namespace correcto
func BuildNodeID(identifier int32) string {
    return fmt.Sprintf("ns=%d;i=%d", OPCUA_NAMESPACE, identifier)
}

// Función helper para construir NodeIDs de string con el namespace correcto  
func BuildNodeIDString(identifier string) string {
    return fmt.Sprintf("ns=%d;s=%s", OPCUA_NAMESPACE, identifier)
}