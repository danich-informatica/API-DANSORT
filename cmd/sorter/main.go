package main

import (
	"context"
	"log"
	"os"

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
	// Configurar logger
	log.SetOutput(os.Stdout)
	log.SetFlags(log.Ldate | log.Ltime)

	log.Println("🚀 Iniciando sistema Sorter - API Greenex")

	// 0. Inicializar gestor de canales compartidos (Singleton)
	channelMgr := shared.GetChannelManager()
	defer channelMgr.CloseAll()
	log.Println("✅ Gestor de canales inicializado")

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

	// 3. Inicializar PostgreSQL usando config YAML
	ctx := context.Background()

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
		log.Fatalf("❌ Error al obtener PostgresManager: %v", err)
	}
	log.Println("✅ Base de datos PostgreSQL inicializada")

	// 4. Obtener el primer sorter configurado en el YAML
	if len(cfg.Sorters) == 0 {
		log.Fatalf("❌ No hay sorters configurados en el archivo YAML")
	}

	sorterCfg := cfg.Sorters[0] // Primer sorter
	log.Printf("📦 Configurando Sorter #%d: %s", sorterCfg.ID, sorterCfg.Name)

	// 5. Buscar el Cognex asociado al sorter
	var cognexCfg *config.CognexDevice
	for i := range cfg.CognexDevices {
		if cfg.CognexDevices[i].ID == sorterCfg.CognexID {
			cognexCfg = &cfg.CognexDevices[i]
			break
		}
	}

	if cognexCfg == nil {
		log.Fatalf("❌ No se encontró Cognex con ID %d para el sorter", sorterCfg.CognexID)
	}

	// 6. Crear el CognexListener
	cognexListener := listeners.NewCognexListener(
		cognexCfg.Host,
		cognexCfg.Port,
		cognexCfg.ScanMethod,
		dbManager,
	)

	// 7. Crear salidas desde config YAML
	var salidas []sorter.Salida
	for _, salidaCfg := range sorterCfg.Salidas {
		tipo := salidaCfg.Tipo
		if tipo == "" {
			tipo = "manual" // Default
		}
		salidas = append(salidas, sorter.Salida{}.GetNewSalida(salidaCfg.ID, salidaCfg.Name, tipo))
	}

	// 8. Crear el sorter
	s := sorter.GetNewSorter(sorterCfg.ID, sorterCfg.Ubicacion, salidas, cognexListener)

	// 8.5. Inicializar SKUManager para cargar SKUs activos
	skuManager, err := flow.NewSKUManager(ctx, dbManager)
	if err != nil {
		log.Printf("⚠️  Error al inicializar SKUManager: %v (sorter sin SKUs iniciales)", err)
		skuManager = nil
	} else {
		log.Printf("✅ SKUManager inicializado con %d SKUs", len(skuManager.GetActiveSKUs()))

		// Cargar y publicar SKUs activos al canal del sorter
		log.Println("🔄 Cargando SKUs activos para el sorter...")
		activeSKUs := skuManager.GetActiveSKUs()

		// Convertir SKUs a formato assignable con hash
		assignableSKUs := make([]models.SKUAssignable, 0, len(activeSKUs)+1)

		// Agregar SKU REJECT (ID 0) al inicio
		assignableSKUs = append(assignableSKUs, models.GetRejectSKU())

		// Convertir SKUs activos usando hash como ID
		for _, sku := range activeSKUs {
			assignableSKUs = append(assignableSKUs, sku.ToAssignableWithHash())
		}

		// Publicar SKUs al sorter
		s.UpdateSKUs(assignableSKUs)
		log.Printf("✅ Sorter #%d: %d SKUs publicados al canal", sorterCfg.ID, len(assignableSKUs))
	}

	// 9. Iniciar el sorter
	err = s.Start()
	if err != nil {
		log.Fatalf("❌ Error al iniciar sorter: %v", err)
	}

	log.Println("✅ Sorter iniciado correctamente")
	log.Println("Presiona Ctrl+C para detener...")

	// Mantener el programa en ejecución
	select {}
}
