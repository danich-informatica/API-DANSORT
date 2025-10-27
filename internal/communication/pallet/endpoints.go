package pallet

const (
	// EndpointGetEstado obtiene el estado de una o todas las mesas
	// Query params: id (integer) - 0 para todas las mesas, ID específico para una mesa
	EndpointGetEstado = "/Mesa/Estado"

	// EndpointPostNuevaCaja registra una nueva caja leída en una mesa
	// Query params: idMesa (integer) - ID de la mesa
	// Body: {"idCaja": "código"}
	EndpointPostNuevaCaja = "/Mesa/NuevaCaja"

	// EndpointPostOrden inserta una nueva orden de fabricación en una mesa
	// Query params: id (integer) - ID de la mesa
	// Body: DatosOrdenFabricacion
	EndpointPostOrden = "/Mesa"

	// EndpointPostVaciar vacía una mesa de paletizado
	// Query params: id (integer) - ID de la mesa, modo (integer) - 1 o 2
	// Modo 1: Vaciar y continuar con la OF
	// Modo 2: Vaciar y finalizar OF
	EndpointPostVaciar = "/Mesa/Vaciar"
)
