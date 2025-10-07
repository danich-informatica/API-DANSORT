package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"API-GREENEX/internal/config"
	"API-GREENEX/internal/db"
	"API-GREENEX/internal/flow"
	"API-GREENEX/internal/listeners"
	"API-GREENEX/internal/shared"

	"github.com/joho/godotenv"
)

func main() {
	// Banner
	fmt.Println("")
	fmt.Println("    ██╗  ██╗████████╗████████╗██████╗     ███████╗███████╗██████╗ ██╗   ██╗███████╗██████╗ ")
	fmt.Println("    ██║  ██║╚══██╔══╝╚══██╔══╝██╔══██╗    ██╔════╝██╔════╝██╔══██╗██║   ██║██╔════╝██╔══██╗")
	fmt.Println("    ███████║   ██║      ██║   ██████╔╝    ███████╗█████╗  ██████╔╝██║   ██║█████╗  ██████╔╝")
	fmt.Println("    ██╔══██║   ██║      ██║   ██╔═══╝     ╚════██║██╔══╝  ██╔══██╗╚██╗ ██╔╝██╔══╝  ██╔══██╗")
	fmt.Println("    ██║  ██║   ██║      ██║   ██║         ███████║███████╗██║  ██║ ╚████╔╝ ███████╗██║  ██║")
	fmt.Println("    ╚═╝  ╚═╝   ╚═╝      ╚═╝   ╚═╝         ╚══════╝╚══════╝╚═╝  ╚═╝  ╚═══╝  ╚══════╝╚═╝  ╚═╝")
	fmt.Println("")
	fmt.Println("🌐 Iniciando Servidor HTTP...")
	fmt.Println("")

	// Inicializar gestor de canales compartidos (Singleton)
	channelMgr := shared.GetChannelManager()
	defer channelMgr.CloseAll()
	log.Println("✅ Gestor de canales inicializado")

	// 1. Cargar .env
	if err := godotenv.Load(); err != nil {
		log.Println("⚠️  Archivo .env no encontrado")
	}

	// 2. Cargar configuración
	configPath := os.Getenv("CONFIG_FILE")
	if configPath == "" {
		configPath = "config/config.yaml"
	}

	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		log.Fatalf("❌ Error al cargar configuración: %v", err)
	}
	log.Printf("✅ Configuración cargada desde: %s", configPath)

	// 3. Conectar a PostgreSQL
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	connectTimeout, _ := cfg.Database.Postgres.GetConnectTimeoutDuration()
	healthCheckInterval, _ := cfg.Database.Postgres.GetHealthcheckIntervalDuration()

	dbManager, err := db.GetPostgresManagerWithURL(
		ctx,
		cfg.Database.Postgres.URL,
		int32(cfg.Database.Postgres.MinConns),
		int32(cfg.Database.Postgres.MaxConns),
		connectTimeout,
		healthCheckInterval,
	)
	if err != nil {
		log.Printf("⚠️  PostgreSQL no disponible: %v", err)
		dbManager = nil
	} else {
		defer dbManager.Close()
		log.Println("✅ PostgreSQL conectado")
	}

	// 4. Usar HTTPFrontend configurado en listeners/http.go
	httpPort := fmt.Sprintf("%d", cfg.HTTP.Port)
	if cfg.HTTP.Port == 0 {
		httpPort = "8080"
	}

	httpService := listeners.NewHTTPFrontend(httpPort)

	// Vincular base de datos si está disponible
	if dbManager != nil {
		httpService.SetPostgresManager(dbManager)
	}

	// 3.5. Inicializar SKUManager para endpoints de streaming
	var skuManager *flow.SKUManager
	if dbManager != nil {
		skuManager, err = flow.NewSKUManager(ctx, dbManager)
		if err != nil {
			log.Printf("⚠️  Error al inicializar SKUManager: %v", err)
			skuManager = nil
		}
	}

	if skuManager != nil {
		httpService.SetSKUManager(skuManager)
	}

	// 5. Mostrar información
	log.Printf("🌐 Servidor HTTP iniciando en puerto %s...", httpPort)
	log.Println("📊 Endpoints disponibles:")
	log.Println("   GET  /Mesa/Estado")
	log.Println("   POST /Mesa")
	log.Println("   POST /Mesa/Vaciar")
	if skuManager != nil {
		log.Println("   GET  /skus/assignables/:sorter_id (⚡ streaming eficiente)")
	} else {
		log.Println("   GET  /skus/assignables/:sorter_id (⚠️  SKUManager no disponible)")
	}
	log.Printf("🚀 Servidor listo en http://localhost:%s\n", httpPort)

	// 6. Iniciar servidor usando HTTPFrontend
	if err := httpService.Start(); err != nil {
		log.Fatalf("❌ Error al iniciar servidor: %v", err)
	}
}
