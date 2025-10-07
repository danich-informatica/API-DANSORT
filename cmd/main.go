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
	// Configurar logger sin timestamps para el banner
	log.SetOutput(os.Stdout)
	log.SetFlags(0)

	// Tu hermoso banner ASCII
	log.Println("")
	log.Println("    ░█████╗░██████╗░██╗░░░░░░░██████╗░██████╗░███████╗███████╗███╗░░██╗███████╗██╗░░██╗")
	log.Println("    ██╔══██╗██╔══██╗██║░░░░░░██╔════╝░██╔══██╗██╔════╝██╔════╝████╗░██║██╔════╝╚██╗██╔╝")
	log.Println("    ███████║██████╔╝██║█████╗██║░░██╗░██████╔╝█████╗░░█████╗░░██╔██╗██║█████╗░░░╚███╔╝░")
	log.Println("    ██╔══██║██╔═══╝░██║╚════╝██║░░╚██╗██╔══██╗██╔══╝░░██╔══╝░░██║╚████║██╔══╝░░░██╔██╗░")
	log.Println("    ██║░░██║██║░░░░░██║░░░░░░╚██████╔╝██║░░██║███████╗███████╗██║░╚███║███████╗██╔╝╚██╗")
	log.Println("    ╚═╝░░╚═╝╚═╝░░░░░╚═╝░░░░░░░╚═════╝░╚═╝░░╚═╝╚══════╝╚══════╝╚═╝░░╚══╝╚══════╝╚═╝░░╚═╝")
	log.Println("")
	log.Println("Iniciando API-Greenex...")
	log.Println("")

	// Ahora activar fecha/hora para los logs normales
	log.SetFlags(log.Ldate | log.Ltime)

	// 0. Inicializar gestor de canales compartidos (Singleton)
	channelMgr := shared.GetChannelManager()
	defer channelMgr.CloseAll()
	log.Println("✅ Gestor de canales inicializado")
	log.Println("")

	// 1. Cargar archivo .env para obtener ruta del config
	if err := godotenv.Load(); err != nil {
		log.Println("⚠️  Archivo .env no encontrado, usando valores por defecto")
	}

	// 2. Cargar configuración desde YAML
	configPath := os.Getenv("CONFIG_FILE")
	if configPath == "" {
		configPath = "config/config.yaml"
	}

	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		log.Fatalf("❌ Error al cargar configuración: %v", err)
	}
	log.Printf("✅ Configuración cargada desde: %s", configPath)

	// 3. Inicializar la conexión a PostgreSQL usando config YAML
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
		log.Fatalf("❌ Error al inicializar PostgreSQL: %v", err)
	}
	defer dbManager.Close()
	log.Println("✅ Base de datos PostgreSQL inicializada correctamente")

	// 3.5. Inicializar SKUManager para gestión eficiente con streaming
	skuManager, err := flow.NewSKUManager(ctx, dbManager)
	if err != nil {
		log.Printf("⚠️  Error al inicializar SKUManager: %v (continuando sin caché de SKUs)", err)
		skuManager = nil
	}

	// 4. Iniciar listeners de Cognex (todos los configurados)
	log.Println("")
	log.Printf("📷 Configurando %d dispositivo(s) Cognex...", len(cfg.CognexDevices))
	var cognexListeners []*listeners.CognexListener

	for _, cognexCfg := range cfg.CognexDevices {
		log.Println("  ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		log.Printf("  📌 Cognex #%d: %s", cognexCfg.ID, cognexCfg.Name)
		log.Printf("     Ubicación: %s", cognexCfg.Ubicacion)
		log.Printf("     Host: %s:%d", cognexCfg.Host, cognexCfg.Port)
		log.Printf("     Método: %s", cognexCfg.ScanMethod)

		cognexListener := listeners.NewCognexListener(
			cognexCfg.Host,
			cognexCfg.Port,
			cognexCfg.ScanMethod,
			dbManager,
		)

		if err := cognexListener.Start(); err != nil {
			log.Fatalf("     ❌ Error al iniciar: %v", err)
		}
		cognexListeners = append(cognexListeners, cognexListener)
		log.Printf("     ✅ Escuchando correctamente")
	}
	log.Println("  ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	log.Println("")

	// Mostrar información de Sorters configurados (si hay)
	if len(cfg.Sorters) > 0 {
		log.Printf("🔀 Sorters configurados: %d", len(cfg.Sorters))
		for _, sorterCfg := range cfg.Sorters {
			log.Println("  ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
			log.Printf("  📦 Sorter #%d: %s", sorterCfg.ID, sorterCfg.Name)
			log.Printf("     Ubicación: %s", sorterCfg.Ubicacion)
			log.Printf("     Cognex ID: %d", sorterCfg.CognexID)
			log.Printf("     Método: %s", sorterCfg.ScanMethod)
			log.Printf("     Salidas: %d", len(sorterCfg.Salidas))
			for _, salida := range sorterCfg.Salidas {
				log.Printf("       ↳ Salida %d: %s", salida.ID, salida.Name)
			}
			// despues se debe implementar una logica para que los sorters se inicien mediante go routines
		}
		log.Println("  ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		log.Println("")
	}

	// Cerrar todos los listeners al finalizar
	defer func() {
		for _, cl := range cognexListeners {
			cl.Stop()
		}
	}()

	// 5. Crear e iniciar el servidor HTTP con endpoints
	httpPort := fmt.Sprintf("%d", cfg.HTTP.Port)
	httpService := listeners.NewHTTPFrontend(httpPort)
	httpService.SetPostgresManager(dbManager)

	// Vincular SKUManager si está disponible para endpoints de streaming
	if skuManager != nil {
		httpService.SetSKUManager(skuManager)
	}

	log.Printf("🌐 Servidor HTTP iniciando en puerto %s...", httpPort)
	log.Println("📊 Endpoints disponibles:")
	log.Println("   GET  /Mesa/Estado")
	log.Println("   POST /Mesa")
	log.Println("   POST /Mesa/Vaciar")
	log.Println("   GET  /status")
	if skuManager != nil {
		log.Println("   GET  /skus/assignables/:sorter_id (⚡ streaming eficiente)")
	} else {
		log.Println("   GET  /skus/assignables/:sorter_id (⚠️  SKUManager no disponible)")
	}

	// Iniciar servidor HTTP con las rutas configuradas
	if err := httpService.Start(); err != nil {
		log.Fatalf("❌ Error al iniciar servidor HTTP: %v", err)
	}
}
