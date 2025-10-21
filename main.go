package main

import (
	"context"
	"fmt"

	"log"

	"github.com/gopcua/opcua"
	"github.com/gopcua/opcua/ua"
)

func main() {
	// Reemplaza esta URL con la de tu servidor OPC UA
	endpoint := "opc.tcp://192.168.120.100:4840"
	ctx := context.Background()

	// Conectar al servidor
	c, err := opcua.NewClient(endpoint)
	if err != nil {
		log.Fatal(err)
	}
	if err := c.Connect(ctx); err != nil {
		log.Fatal(err)
	}
	defer c.Close(ctx)
	fmt.Println("¡Conexión exitosa al servidor OPC UA!")

	// 1. Definir los NodeIDs del objeto y el método
	objectID, err := ua.ParseNodeID("ns=4;i=1") // Server Interface_1
	if err != nil {
		log.Fatalf("ID de objeto inválido: %s", err)
	}
	methodID, err := ua.ParseNodeID("ns=4;i=21") // DB_OPC_S7000
	if err != nil {
		log.Fatalf("ID de método inválido: %s", err)
	}

	// 2. Preparar el argumento de entrada (un NodeId)
	// ¡CAMBIA ESTE VALOR por el NodeID que realmente quieras enviar!
	nodeIdParaEnviar, err := ua.ParseNodeID("ns=4;i=26")
	if err != nil {
		log.Fatalf("NodeID para enviar es inválido: %s", err)
	}

	inputArgument, err := ua.NewVariant(nodeIdParaEnviar)
	if err != nil {
		log.Fatalf("Error al crear el Variant para el argumento: %s", err)
	}

	// 3. Crear y ejecutar la petición
	req := &ua.CallMethodRequest{
		ObjectID:       objectID,
		MethodID:       methodID,
		InputArguments: []*ua.Variant{inputArgument},
	}

	fmt.Printf("Llamando al método %s en el objeto %s...\n", methodID, objectID)
	resp, err := c.Call(ctx, req)
	if err != nil {
		log.Fatalf("La llamada al método falló: %s", err)
	}
	if resp.StatusCode != ua.StatusOK {
		log.Fatalf("Estado de la llamada no exitoso: %s", resp.StatusCode)
	}

	fmt.Println("¡Método llamado con éxito!")

	// 4. Procesar los argumentos de salida
	if len(resp.OutputArguments) > 0 {
		output := resp.OutputArguments[0]
		if nodeIdResultado, ok := output.Value().(*ua.NodeID); ok {
			fmt.Printf("Resultado recibido (OUT1): %s\n", nodeIdResultado.String())
		} else {
			fmt.Printf("Se recibió una respuesta, pero no es del tipo NodeId: %T\n", output.Value())
		}
	} else {
		fmt.Println("El método se ejecutó pero no devolvió argumentos.")
	}
}
