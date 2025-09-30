package models

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
	DEFAULT_HEARTBEAT_NODE   = "ns=4;i=3"
	DEFAULT_SEGREGATION_NODE = "ns=4;i=2"

	// Intervalos de tiempo
	DEFAULT_SUBSCRIPTION_INTERVAL = "1000ms"
	DEFAULT_CONNECTION_TIMEOUT    = "30s"

	// Nombres de suscripciones por defecto
	HEARTBEAT_SUBSCRIPTION   = "heartbeat_subscription"
	SEGREGATION_SUBSCRIPTION = "segregation_subscription"
	DEFAULT_SUBSCRIPTION     = "default_subscription"
)

// WagoTestNodes contiene los NodeIDs para las variables de prueba de WAGO
const (
	WAGO_BoleanoTest = "ns=4;s=|var|WAGO TEST.Application.DB_OPC.BoleanoTest"
	WAGO_ByteTest    = "ns=4;s=|var|WAGO TEST.Application.DB_OPC.ByteTest"
	WAGO_EnteroTest  = "ns=4;s=|var|WAGO TEST.Application.DB_OPC.EnteroTest"
	WAGO_RealTest    = "ns=4;s=|var|WAGO TEST.Application.DB_OPC.RealTest"
	WAGO_StringTest  = "ns=4;s=|var|WAGO TEST.Application.DB_OPC.StringTest"
	WAGO_VectorBool  = "ns=4;s=|var|WAGO TEST.Application.DB_OPC.VectorBool"
	WAGO_VectorInt   = "ns=4;s=|var|WAGO TEST.Application.DB_OPC.VectorInt"
	WAGO_VectorWord  = "ns=4;s=|var|WAGO TEST.Application.DB_OPC.VectorWord"
	WAGO_WordTest    = "ns=4;s=|var|WAGO TEST.Application.DB_OPC.WordTest"
)

const (
	MESA_ESTADO_JSON_RESPONSE = `{
        "status": "exito",
        "mensaje": "200 - Consulta exitosa.",
        "dataList": [
            {
                "idMesa": 1,
                "estado": 2,
                "descripcionEstado": "Bloqueado",
                "estadoPLC": 4,
                "descripcionEstadoPLC": "Avisos",
                "numerosCajasPale": 16,
                "numerosPaleActual": 1,
                "codigoTipoCaja": "COD01",
                "codigoTipoPale": "COD01",
                "cajasPorCapa": 4,
                "numerosPales": 2
            }
        ],
        "errorList": null
    }`

	MESA_FABRICAION_JSON_RESPONSE = `{
        "status": "exito",
        "mensaje": "201 - Orden creada exitosamente en la mesa."
    }`

	MESA_VACIADO_JSON_RESPONSE = `{
        "status": "exito",
        "mensaje": "ÉXITO 200 - Solicitud de vaciado registrada correctamente. Tipo de vaciado: Al finalizar la capa."
    }`
)

// Función helper para construir NodeIDs con el namespace correcto
func BuildNodeID(identifier int32) string {
	return fmt.Sprintf("ns=%d;i=%d", OPCUA_NAMESPACE, identifier)
}

// Función helper para construir NodeIDs de string con el namespace correcto
func BuildNodeIDString(identifier string) string {
	return fmt.Sprintf("ns=%d;s=%s", OPCUA_NAMESPACE, identifier)
}
