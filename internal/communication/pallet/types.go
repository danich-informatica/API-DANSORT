package pallet

import "time"

// EstadoMesa representa el estado completo de una mesa de paletizado
type EstadoMesa struct {
	IDMesa               int             `json:"idMesa"`
	Estado               int             `json:"estado"` // 1 = libre, 2 = bloqueado (hay orden activa)
	DescripcionEstado    string          `json:"descripcionEstado"`
	EstadoPLC            int             `json:"estadoPLC"` // 1=Parado 2=Manual 3=Automático 4=Avisos 5=Alarmas
	DescripcionEstadoPLC string          `json:"descripcionEstadoPLC"`
	DatosProduccion      DatosProduccion `json:"datosProduccion"`
	DatosPaletizado      DatosPaletizado `json:"datosPaletizado"`
}

// DatosProduccion contiene información sobre el progreso de la producción actual
type DatosProduccion struct {
	NumeroPaleActual      int `json:"numeroPaleActual"`      // Número del palé actual
	NumeroCajasEnPale     int `json:"numeroCajasEnPale"`     // Cajas en el palé actual
	TotalPalesFinalizados int `json:"totalPalesFinalizados"` // Total de palés completados
	TotalCajasPaletizadas int `json:"totalCajasPaletizadas"` // Total de cajas paletizadas
}

// DatosPaletizado contiene la configuración del paletizado
type DatosPaletizado struct {
	CajasPorPale     int    `json:"cajasPorPale"`     // Número de cajas por palé
	CodigoTipoEnvase string `json:"codigoTipoEnvase"` // Código del tipo de envase
	CodigoTipoPale   string `json:"codigoTipoPale"`   // Código del tipo de palé
	CajasPorCapa     int    `json:"cajasPorCapa"`     // Cajas por capa
}

// NuevaCajaRequest representa la solicitud para registrar una nueva caja
type NuevaCajaRequest struct {
	IDCaja int `json:"idCaja"` // Código de la caja leída
}

// OrdenFabricacionRequest representa la solicitud para crear una orden de fabricación
type OrdenFabricacionRequest struct {
	NumeroPales       int    `json:"numeroPales"`       // Cantidad de palés a hacer
	CajasPerPale      int    `json:"cajasPorPale"`      // Número de cajas por palé
	CajasPerCapa      int    `json:"cajasPorCapa"`      // Cuántas cajas habrá en cada capa
	CodigoTipoEnvase  string `json:"codigoTipoEnvase"`  // Código del tipo de envase
	CodigoTipoPale    string `json:"codigoTipoPale"`    // Código del tipo de palé
	IDProgramaFlejado int    `json:"idProgramaFlejado"` // ID del programa de flejado
}

// VaciarMesaMode representa el modo de vaciado de mesa
type VaciarMesaMode int

const (
	// VaciarModoContinuar (Modo 1): Vaciar y continuar con la OF
	// Se vaciará la mesa con el palé actual, pero al finalizar se continuará con la misma orden
	VaciarModoContinuar VaciarMesaMode = 1

	// VaciarModoFinalizar (Modo 2): Vaciar y finalizar OF
	// Se vaciará la mesa con el palé actual y al finalizar se borrará la orden del PLC
	// La mesa quedará libre para insertar una nueva orden
	VaciarModoFinalizar VaciarMesaMode = 2
)

// Estructuras de respuesta estándar de la API

// APIResponse representa la estructura de respuesta estándar con un solo objeto de datos
type APIResponse struct {
	Mensaje string      `json:"mensaje"`        // Descripción de la respuesta
	Status  string      `json:"status"`         // Estado descriptivo (ej: exito, error_formato_incorrecto)
	Data    interface{} `json:"data,omitempty"` // Datos de respuesta (opcional)
}

// APIResponseList representa la estructura de respuesta estándar con múltiples objetos de datos
type APIResponseList struct {
	Mensaje  string      `json:"mensaje"`            // Descripción de la respuesta
	Status   string      `json:"status"`             // Estado descriptivo (ej: exito, error_formato_incorrecto)
	DataList interface{} `json:"dataList,omitempty"` // Lista de datos de respuesta (opcional)
}

// Status codes comunes usados en el campo "status"
const (
	StatusExito                  = "exito"
	StatusErrorValidacion        = "error_validacion"
	StatusErrorFormatoIncorrecto = "error_formato_incorrecto"
	StatusErrorNoEncontrado      = "error_no_encontrado"
	StatusErrorInterno           = "error_interno"
	StatusMesaSinOrden           = "mesa_sin_orden"
	StatusMesaNoDisponible       = "mesa_no_disponible"
	StatusMesaYaVacia            = "mesa_ya_vacia"
	StatusCajaDuplicada          = "caja_duplicada"
)

// APIError representa un error devuelto por la API de paletizado
type APIError struct {
	StatusCode int
	Message    string
	Endpoint   string
	Timestamp  time.Time
}

func (e *APIError) Error() string {
	return e.Message
}

// ErrorResponse representa errores específicos de la API
var (
	// Errores GET /Mesa/Estado
	ErrMesaNoActiva     = &APIError{StatusCode: 202, Message: "Mesa NO tiene OF activa"}
	ErrMesaNoEncontrada = &APIError{StatusCode: 404, Message: "Mesa no encontrada"}
	ErrEstadoInterno    = &APIError{StatusCode: 500, Message: "Ha ocurrido un error intentando obtener el estado de la mesa"}

	// Errores POST /Mesa/NuevaCaja
	ErrCajaRegistrada       = &APIError{StatusCode: 200, Message: "La caja ha sido registrada correctamente en la mesa"}
	ErrCajaDuplicada        = &APIError{StatusCode: 202, Message: "Lectura duplicada. La caja ya existe"}
	ErrFormatoIncorrecto    = &APIError{StatusCode: 400, Message: "El formato de la petición enviada es incorrecto. Caja NO registrada"}
	ErrMesaNoEncontradaCaja = &APIError{StatusCode: 404, Message: "Mesa no encontrada"}
	ErrRegistroCajaError    = &APIError{StatusCode: 500, Message: "No se ha podido registrar la caja en la mesa"}

	// Errores POST /Mesa (Orden de fabricación)
	ErrOrdenCreada           = &APIError{StatusCode: 200, Message: "Orden creada exitosamente"}
	ErrMesaNoDisponible      = &APIError{StatusCode: 209, Message: "La mesa no está disponible para iniciar una orden"}
	ErrFormatoOrden          = &APIError{StatusCode: 400, Message: "El formato de la orden enviada es incorrecto"}
	ErrMesaNoEncontradaOrden = &APIError{StatusCode: 404, Message: "Mesa, envase o palé no encontrados"}
	ErrCrearOrdenError       = &APIError{StatusCode: 500, Message: "Ha ocurrido un error en el proceso de crear la orden"}

	// Errores POST /Mesa/Vaciar
	ErrVaciadoRegistrado      = &APIError{StatusCode: 200, Message: "Solicitud de vaciado registrada correctamente en la mesa"}
	ErrMesaYaVacia            = &APIError{StatusCode: 202, Message: "La mesa ya está vacía"}
	ErrFormatoVaciado         = &APIError{StatusCode: 400, Message: "El formato de la petición enviada es incorrecto o el modo de vaciado indicado no existe"}
	ErrMesaNoEncontradaVaciar = &APIError{StatusCode: 404, Message: "Mesa no encontrada"}
	ErrVaciarMesaError        = &APIError{StatusCode: 500, Message: "Ha ocurrido un error intentando procesar la petición de vaciar la mesa"}
)
