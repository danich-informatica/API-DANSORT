package main

import (
	"context"
	"log"
	"os"
	"strconv"

	"API-GREENEX/internal/db"
	"API-GREENEX/internal/listeners"
	"API-GREENEX/internal/sorter"
)

func main() {
	// Configurar logger
	log.SetOutput(os.Stdout)
	log.SetFlags(log.Ldate | log.Ltime)

	log.Println("🚀 Iniciando sistema Sorter - API Greenex")

	// 1. Inicializar PostgreSQL
	ctx := context.Background()
	dbManager, err := db.GetPostgresManager(ctx)
	if err != nil {
		log.Fatalf("❌ Error al obtener PostgresManager: %v", err)
	}
	log.Println("✅ Base de datos PostgreSQL inicializada")

	// 2. Obtener configuración desde variables de entorno
	sorterID := getEnvInt("SORTER_ID", 1)
	sorterUbicacion := getEnv("SORTER_UBICACION", "Ubicación 1")
	cognexHost := getEnv("COGNEX_HOST", "127.0.0.1")
	cognexPort := getEnvInt("COGNEX_PORT", 8085)
	scanMethod := getEnv("SCAN_METHOD", "QR")

	// 3. Crear salidas desde variables de entorno
	salidas := []sorter.Salida{
		sorter.Salida{}.GetNewSalida(1, getEnv("SALIDA_1", "Salida 1")),
		sorter.Salida{}.GetNewSalida(2, getEnv("SALIDA_2", "Salida 2")),
		sorter.Salida{}.GetNewSalida(3, getEnv("SALIDA_3", "Salida 3")),
	}

	// 4. Crear el CognexListener
	cognexListener := listeners.NewCognexListener(cognexHost, cognexPort, scanMethod, dbManager)

	// 5. Crear el sorter
	s := sorter.GetNewSorter(sorterID, sorterUbicacion, salidas, cognexListener)

	// 6. Iniciar el sorter
	err = s.Start()
	if err != nil {
		log.Fatalf("❌ Error al iniciar sorter: %v", err)
	}

	log.Println("✅ Sorter iniciado correctamente")
	log.Println("Presiona Ctrl+C para detener...")

	// Mantener el programa en ejecución
	select {}
}

// getEnv obtiene una variable de entorno o devuelve un valor por defecto
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvInt obtiene una variable de entorno como int o devuelve un valor por defecto
func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}
