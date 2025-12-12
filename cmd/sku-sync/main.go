package main

import (
	"context"
	"log"
	"os"
	"time"

	"API-GREENEX/internal/config"
	"API-GREENEX/internal/db"
	"API-GREENEX/internal/flow"

	"github.com/joho/godotenv"
)

func main() {
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds)
	log.Println("🚀 ========================================")
	log.Println("🚀 SKU Sync - Herramienta de Sincronización")
	log.Println("🚀 ========================================")
	log.Println("")

	// Cargar .env
	if err := godotenv.Load(); err != nil {
		log.Println("⚠️  Archivo .env no encontrado, usando valores por defecto")
	}

	// 1. Cargar configuración
	configPath := os.Getenv("CONFIG_FILE")
	if configPath == "" {
		configPath = "config/config.yaml"
	}

	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		log.Fatalf("❌ Error cargando configuración: %v", err)
	}
	log.Printf("✅ Configuración cargada desde: %s", configPath)

	ctx := context.Background()

	// 2. Conectar a SQL Server (UNITEC)
	log.Println("🔌 Conectando a SQL Server (UNITEC)...")
	sqlServerMgr, err := db.GetManagerWithConfig(ctx, cfg.Database.SQLServer)
	if err != nil {
		log.Fatalf("❌ Error conectando a SQL Server: %v", err)
	}
	defer sqlServerMgr.Close()
	log.Println("✅ Conectado a SQL Server")

	// 3. Conectar a PostgreSQL
	log.Println("🔌 Conectando a PostgreSQL...")

	connectTimeout, err := cfg.Database.Postgres.GetConnectTimeoutDuration()
	if err != nil {
		log.Printf("⚠️  Error parseando connect_timeout: %v, usando 30s", err)
		connectTimeout = 30 * time.Second
	}

	healthCheckInterval, err := cfg.Database.Postgres.GetHealthcheckIntervalDuration()
	if err != nil {
		log.Printf("⚠️  Error parseando healthcheck_interval: %v, usando 1m", err)
		healthCheckInterval = 1 * time.Minute
	}

	postgresMgr, err := db.GetPostgresManagerWithURL(
		ctx,
		cfg.Database.Postgres.URL,
		int32(cfg.Database.Postgres.MinConns),
		int32(cfg.Database.Postgres.MaxConns),
		connectTimeout,
		healthCheckInterval,
	)
	if err != nil {
		log.Fatalf("❌ Error conectando a PostgreSQL: %v", err)
	}
	defer postgresMgr.Close()
	log.Println("✅ Conectado a PostgreSQL")
	log.Println("")

	// 4. Crear SKUManager (necesario para la lógica del worker)
	log.Println("📦 Inicializando SKUManager...")
	skuManager, err := flow.NewSKUManager(ctx, postgresMgr)
	if err != nil {
		log.Fatalf("❌ Error creando SKUManager: %v", err)
	}

	activeSKUs := skuManager.GetActiveSKUs()
	log.Printf("✅ SKUManager inicializado (%d SKUs activas cargadas)", len(activeSKUs))
	log.Println("")

	// 5. Configurar intervalo de sincronización
	syncInterval := 1 * time.Second // Por defecto 10 segundos
	if intervalStr := os.Getenv("SKU_SYNC_INTERVAL"); intervalStr != "" {
		if parsed, err := time.ParseDuration(intervalStr); err == nil {
			syncInterval = parsed
			log.Printf("⏱️  Intervalo de sincronización configurado: %v", syncInterval)
		} else {
			log.Printf("⚠️  Error parseando SKU_SYNC_INTERVAL '%s': %v, usando %v", intervalStr, err, syncInterval)
		}
	} else {
		log.Printf("⏱️  Intervalo de sincronización por defecto: %v", syncInterval)
	}
	log.Println("")

	// 6. Crear worker temporal (sin sorters, solo para ejecutar la lógica de sync)
	log.Println("🔄 Creando worker de sincronización...")
	syncWorker := flow.NewSKUSyncWorker(
		ctx,
		sqlServerMgr,
		postgresMgr,
		skuManager,
		nil,          // No hay sorters en este comando standalone
		syncInterval, // Usamos el intervalo configurado
	)
	log.Println("✅ Worker creado")
	log.Println("")

	// 7. Ejecutar sincronización CONTINUA
	log.Println("🔁 Iniciando sincronización continua...")
	log.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	log.Printf("⏱️  Sincronización se ejecutará cada %v", syncInterval)
	log.Println("🛑 Presiona Ctrl+C para detener")
	log.Println("")

	// Ejecutar primera sincronización inmediatamente
	log.Println("🚀 Ejecutando primera sincronización...")
	syncWorker.SyncOnce()
	log.Println("")

	// Iniciar el ticker para sincronizaciones periódicas
	ticker := time.NewTicker(syncInterval)
	defer ticker.Stop()

	syncCount := 1
	for range ticker.C {
		syncCount++
		log.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		log.Printf("🔁 Sincronización #%d", syncCount)
		log.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		log.Println("")

		startTime := time.Now()
		syncWorker.SyncOnce()
		elapsed := time.Since(startTime)

		log.Println("")
		log.Printf("✅ Sincronización #%d completada en %v", syncCount, elapsed)
		log.Printf("⏰ Próxima sincronización en %v", syncInterval)
		log.Println("")
	}
}
