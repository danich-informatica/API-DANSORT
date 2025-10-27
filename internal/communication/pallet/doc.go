// Package pallet proporciona un cliente HTTP para interactuar con la API REST
// del sistema de paletizado automático Danich.
//
// La API permite:
//   - Consultar el estado de las mesas de paletizado
//   - Registrar lecturas de cajas
//   - Crear órdenes de fabricación (OF)
//   - Vaciar mesas con diferentes modos
//
// Ejemplo de uso básico:
//
//	client := pallet.NewClient("127.0.0.1", 9093, 10*time.Second)
//	defer client.Close()
//
//	// Consultar estado
//	estados, err := client.GetEstadoMesa(context.Background(), 1)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Registrar caja
//	err = client.RegistrarNuevaCaja(context.Background(), 1, "CAJA001")
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Crear orden de fabricación
//	orden := pallet.OrdenFabricacionRequest{
//	    NumeroPales:  5,
//	    CajasPerPale: 10,
//	    CajasPerCapa: 2,
//	    CodigoEnvase: "ENV001",
//	    CodigoPale:   "PALE01",
//	    IDProgramaFlejado: 1,
//	}
//	err = client.CrearOrdenFabricacion(context.Background(), 1, orden)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Vaciar mesa (finalizar OF)
//	err = client.VaciarMesa(context.Background(), 1, pallet.VaciarModoFinalizar)
//	if err != nil {
//	    log.Fatal(err)
//	}
package pallet
