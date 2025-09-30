package main

import (
	"log"
	"os"

	"API-GREENEX/internal/flow"
	"API-GREENEX/internal/listeners"

	"github.com/gin-gonic/gin"
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

	// Agregar la ruta /status al router de Gin
	router := httpService.GetRouter()
	router.GET("/status", func(c *gin.Context) {
		// Aquí puedes adaptar tu StatusPageHandler para Gin
		c.HTML(200, "status.html", gin.H{
			"title": "Estado del Sistema",
		})
	})

	// Iniciar servidor HTTP usando Gin (NO http.ListenAndServe)
	log.Println("Servidor HTTP iniciando en puerto " + getEnv("HTTP_PORT", "8080"))

	// Usar el método Start() de tu HTTPFrontend que ya usa Gin
	if err := httpService.Start(); err != nil {
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
