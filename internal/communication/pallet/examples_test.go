package pallet_test

import (
	"context"
	"fmt"
	"time"

	"api-dansort/internal/communication/pallet"
)

// ExampleClient_GetEstadoMesa demuestra cómo consultar el estado de una mesa
func ExampleClient_GetEstadoMesa() {
	// Crear cliente
	client := pallet.NewClient("127.0.0.1", 9093, 10*time.Second)
	defer client.Close()

	ctx := context.Background()

	// Consultar estado de la mesa 1
	estados, err := client.GetEstadoMesa(ctx, 1)
	if err != nil {
		fmt.Printf("Error: %s\n", pallet.FormatError(err))
		return
	}

	for _, estado := range estados {
		fmt.Printf("Mesa %d - Estado: %s (PLC: %s)\n",
			estado.IDMesa,
			estado.DescripcionEstado,
			estado.DescripcionEstadoPLC,
		)
	}
}

// ExampleClient_RegistrarNuevaCaja demuestra cómo registrar una caja
func ExampleClient_RegistrarNuevaCaja() {
	client := pallet.NewClient("127.0.0.1", 9093, 10*time.Second)
	defer client.Close()

	ctx := context.Background()

	// Registrar caja en la mesa 1
	err := client.RegistrarNuevaCaja(ctx, 1, "CAJA123456")
	if err != nil {
		if err == pallet.ErrCajaDuplicada {
			fmt.Println("La caja ya fue registrada anteriormente")
			return
		}
		fmt.Printf("Error: %s\n", pallet.FormatError(err))
		return
	}

	fmt.Println("Caja registrada correctamente")
}

// ExampleClient_CrearOrdenFabricacion demuestra cómo crear una orden de fabricación
func ExampleClient_CrearOrdenFabricacion() {
	client := pallet.NewClient("127.0.0.1", 9093, 10*time.Second)
	defer client.Close()

	ctx := context.Background()

	orden := pallet.OrdenFabricacionRequest{
		NumeroPales:       5,
		CajasPerPale:      10,
		CajasPerCapa:      2,
		CodigoTipoEnvase:  "ENV001",
		CodigoTipoPale:    "PALE01",
		IDProgramaFlejado: 1,
	}

	err := client.CrearOrdenFabricacion(ctx, 1, orden)
	if err != nil {
		if err == pallet.ErrMesaNoDisponible {
			fmt.Println("La mesa está ocupada, no se puede crear una nueva orden")
			return
		}
		fmt.Printf("Error: %s\n", pallet.FormatError(err))
		return
	}

	fmt.Println("Orden de fabricación creada exitosamente")
}

// ExampleClient_VaciarMesa demuestra cómo vaciar una mesa
func ExampleClient_VaciarMesa() {
	client := pallet.NewClient("127.0.0.1", 9093, 10*time.Second)
	defer client.Close()

	ctx := context.Background()

	// Vaciar y finalizar la orden
	err := client.VaciarMesa(ctx, 1, pallet.VaciarModoFinalizar)
	if err != nil {
		if err == pallet.ErrMesaYaVacia {
			fmt.Println("La mesa ya está vacía")
			return
		}
		fmt.Printf("Error: %s\n", pallet.FormatError(err))
		return
	}

	fmt.Println("Mesa vaciada correctamente")
}

// ExampleRetryableError demuestra cómo manejar errores reintentables
func ExampleRetryableError() {
	client := pallet.NewClient("127.0.0.1", 9093, 5*time.Second)
	defer client.Close()

	ctx := context.Background()
	maxRetries := 3

	for i := 0; i < maxRetries; i++ {
		estados, err := client.GetEstadoMesa(ctx, 1)
		if err == nil {
			fmt.Printf("Estado obtenido: %v\n", estados)
			return
		}

		if pallet.IsRetryable(err) {
			fmt.Printf("Intento %d/%d falló, reintentando...\n", i+1, maxRetries)
			time.Sleep(time.Second * time.Duration(i+1))
			continue
		}

		// Error no reintentable
		fmt.Printf("Error no reintentable: %s\n", pallet.FormatError(err))
		return
	}

	fmt.Println("Máximo de reintentos alcanzado")
}
