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
	"API-GREENEX/internal/models"
	"API-GREENEX/internal/shared"
	"API-GREENEX/internal/sorter"

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

	// 4. Crear listeners de Cognex (NO iniciarlos aún, los sorters lo harán)
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

		cognexListeners = append(cognexListeners, cognexListener)
		log.Printf("     ✅ CognexListener creado (será iniciado por el sorter)")
	}
	log.Println("  ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	log.Println("")

	// Inicializar Sorters (si hay configurados)
	var sorters []*sorter.Sorter
	if len(cfg.Sorters) > 0 {
		log.Printf("🔀 Inicializando %d Sorter(s)...", len(cfg.Sorters))
		log.Println("")

		for _, sorterCfg := range cfg.Sorters {
			log.Println("  ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
			log.Printf("  📦 Sorter #%d: %s", sorterCfg.ID, sorterCfg.Name)
			log.Printf("     Ubicación: %s", sorterCfg.Ubicacion)
			log.Printf("     Cognex ID: %d", sorterCfg.CognexID)

			// Buscar el CognexListener correspondiente por índice (CognexID - 1)
			var cognexListener *listeners.CognexListener
			if sorterCfg.CognexID > 0 && sorterCfg.CognexID <= len(cognexListeners) {
				cognexListener = cognexListeners[sorterCfg.CognexID-1]
			}

			if cognexListener == nil {
				log.Printf("     ⚠️  No se encontró Cognex Listener para Cognex ID %d, usando el primero disponible", sorterCfg.CognexID)
				if len(cognexListeners) > 0 {
					cognexListener = cognexListeners[0]
				}
			}

			// Crear salidas desde config YAML
			var salidas []shared.Salida
			for _, salidaCfg := range sorterCfg.Salidas {
				tipo := salidaCfg.Tipo
				if tipo == "" {
					tipo = "manual" // Default si no está especificado
				}
				salidas = append(salidas, shared.GetNewSalida(salidaCfg.ID, salidaCfg.Name, tipo))
				log.Printf("       ↳ Salida %d: %s (%s)", salidaCfg.ID, salidaCfg.Name, tipo)
			}
			s := sorter.GetNewSorter(sorterCfg.ID, sorterCfg.Ubicacion, salidas, cognexListener)
			sorters = append(sorters, s)

			log.Printf("     ✅ Sorter #%d creado y registrado", sorterCfg.ID)
		}

		log.Println("  ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		log.Println("")

		// Cargar y publicar SKUs activos a TODOS los sorters
		if skuManager != nil {
			log.Println("🔄 Cargando SKUs activos para todos los sorters...")
			activeSKUs := skuManager.GetActiveSKUs()

			// Convertir SKUs a formato assignable con hash
			assignableSKUs := make([]models.SKUAssignable, 0, len(activeSKUs)+1)

			// Agregar SKU REJECT (ID 0) al inicio
			assignableSKUs = append(assignableSKUs, models.GetRejectSKU())

			// Convertir SKUs activos usando hash como ID
			for _, sku := range activeSKUs {
				assignableSKUs = append(assignableSKUs, sku.ToAssignableWithHash())
			}

			// Publicar SKUs a cada sorter usando su método UpdateSKUs
			for _, s := range sorters {
				s.UpdateSKUs(assignableSKUs)
				log.Printf("   ✅ Sorter #%d: %d SKUs publicados", s.ID, len(assignableSKUs))
			}
			log.Println("")
		} else {
			log.Println("⚠️  SKUManager no disponible, sorters sin SKUs iniciales")
			log.Println("")
		}

		// Iniciar todos los sorters
		log.Println("🚀 Iniciando sorters...")
		for _, s := range sorters {
			if err := s.Start(); err != nil {
				log.Printf("   ⚠️  Error al iniciar Sorter #%d: %v", s.ID, err)
			} else {
				log.Printf("   ✅ Sorter #%d iniciado correctamente", s.ID)
			}
		}
		log.Println("")
	}

	// Cerrar todos los listeners y sorters al finalizar
	defer func() {
		for _, cl := range cognexListeners {
			cl.Stop()
		}
		for _, s := range sorters {
			s.Stop()
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

	// Registrar todos los sorters para acceso directo desde HTTP
	for _, s := range sorters {
		httpService.RegisterSorter(s)
	}

	log.Printf("🌐 Servidor HTTP iniciando en puerto %s...", httpPort)
	log.Println("📊 Endpoints disponibles:")
	log.Println("   GET  /Mesa/Estado")
	log.Println("   POST /Mesa")
	log.Println("   POST /Mesa/Vaciar")
	log.Println("   GET  /status")
	log.Printf("   GET  /skus/assignables/:sorter_id (⚡ acceso directo sin bloqueo, %d sorters registrados)", len(sorters))

	// Iniciar servidor HTTP con las rutas configuradas
	if err := httpService.Start(); err != nil {
		log.Fatalf("❌ Error al iniciar servidor HTTP: %v", err)
	}
}
