package main

import (
	"log"
	"os"
	"net/http"

	"API-GREENEX/internal/flow"
	"API-GREENEX/internal/listeners"
	"API-GREENEX/internal/web"
)

func main() {
	log.Println(`
	░█████╗░██████╗░██╗░░░░░░░██████╗░██████╗░███████╗███████╗███╗░░██╗███████╗██╗░░██╗
	██╔══██╗██╔══██╗██║░░░░░░██╔════╝░██╔══██╗██╔════╝██╔════╝████╗░██║██╔════╝╚██╗██╔╝
	███████║██████╔╝██║█████╗██║░░██╗░██████╔╝█████╗░░█████╗░░██╔██╗██║█████╗░░░╚███╔╝░
	██╔══██║██╔═══╝░██║╚════╝██║░░╚██╗██╔══██╗██╔══╝░░██╔══╝░░██║╚████║██╔══╝░░░██╔██╗░
	██║░░██║██║░░░░░██║░░░░░░╚██████╔╝██║░░██║███████╗███████╗██║░╚███║███████╗██╔╝╚██╗
	╚═╝░░╚═╝╚═╝░░░░░╚═╝░░░░░░░╚═════╝░╚═╝░░╚═╝╚══════╝╚══════╝╚═╝░░╚══╝╚══════╝╚═╝░░╚═╝
	`)

	log.Println("Iniciando API-Greenex...")

	// Inicializar managers separados
	subscriptionManager := flow.NewSubscriptionManager()
	methodManager := flow.NewMethodManager()

	// Inicializar servicios
	opcuaService := listeners.NewOPCUAService()
	httpService := listeners.NewHTTPFrontend(getEnv("HTTP_PORT", "8080"))

	// Conectar los managers con el servicio OPC UA
	opcuaService.SetSubscriptionManager(subscriptionManager)
	opcuaService.SetMethodManager(methodManager)

	// Inyectar el servicio OPC UA (como escritor) en el manager de suscripciones
	subscriptionManager.SetOPCUAWriter(opcuaService)
	
	// Vincular el servicio OPC UA al HTTP para métodos de lectura/escritura
	httpService.SetOPCUAService(opcuaService)

	// Iniciar managers en background
	go subscriptionManager.Start()
	go methodManager.Start()

	// Iniciar OPC UA en background
	go opcuaService.Start()

	// Iniciar loop de escritura/lectura WAGO en background
	go flow.WagoLoop(opcuaService)


	// Integrar página de estado web
	log.Println("Servidor HTTP iniciando en puerto " + getEnv("HTTP_PORT", "8080"))
	httpPort := getEnv("HTTP_PORT", "8080")
	httpAddr := ":" + httpPort

	// Registrar handler /status usando el paquete web
	// (asegúrate de importar "API-GREENEX/internal/web" al inicio)
	http.HandleFunc("/status", web.StatusPageHandler(opcuaService))

	// Iniciar servidor HTTP
	if err := http.ListenAndServe(httpAddr, nil); err != nil {
		log.Fatal("Error al iniciar el servidor HTTP:", err)
	}
}

// Helper function para obtener variables de entorno
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
