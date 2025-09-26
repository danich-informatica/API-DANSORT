package main

import (
	"log"
	"os"

	"API-GREENEX/internal/flow"
	"API-GREENEX/internal/listeners"
)

func main() {
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

	// Iniciar managers en background
	go subscriptionManager.Start()
	go methodManager.Start()

	// Iniciar OPC UA en background
	go opcuaService.Start()

	// Iniciar HTTP (bloqueante)
	log.Println("Servidor HTTP iniciando en puerto "+getEnv("HTTP_PORT", "8080"))
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
