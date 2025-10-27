package pallet

import "time"

// EstadoMesa representa el estado completo de una mesa de paletizado
type EstadoMesa struct {
	IDMesa               int    `json:"idMesa"`
	Estado               int    `json:"estado"` // 1 = libre, 2 = bloqueado (hay orden activa)
	DescripcionEstado    string `json:"descripcionEstado"`
	EstadoPLC            int    `json:"estadoPLC"` // 1=Parado 2=Manual 3=Automático 4=Avisos 5=Alarmas
	DescripcionEstadoPLC string `json:"descripcionEstadoPLC"`
	NumeroCajaPale       int    `json:"numeroCajaPale"`
	NumeroPaleActual     int    `json:"numeroPaleActual"` // 0 si ninguno
	CodigoTipoCaja       string `json:"codigoTipoCaja"`
	CodigoTipoPale       string `json:"codigoTipoPale"`
	CajasPerCapa         int    `json:"cajasPerCapa"`
	NumeroPales          int    `json:"numeroPales"`
}

// NuevaCajaRequest representa la solicitud para registrar una nueva caja
type NuevaCajaRequest struct {
	IDCaja string `json:"idCaja"` // Código de la caja leída
}

// OrdenFabricacionRequest representa la solicitud para crear una orden de fabricación
type OrdenFabricacionRequest struct {
	NumeroPales       int    `json:"numeroPales"`       // Cantidad de palés a hacer
	CajasPerPale      int    `json:"cajasPerPale"`      // Número de cajas por palé
	CajasPerCapa      int    `json:"cajasPerCapa"`      // Cuántas cajas habrá en cada capa
	CodigoEnvase      string `json:"codigoEnvase"`      // Código del envase
	CodigoPale        string `json:"codigoPale"`        // Código del tipo de palé
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
