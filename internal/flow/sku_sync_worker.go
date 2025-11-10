package flow

import (
	"context"
	"log"
	"time"

	"api-dansort/internal/db"
	"api-dansort/internal/models"
	"api-dansort/internal/shared"
)

// SKUSyncWorker sincroniza periódicamente SKUs desde SQL Server a PostgreSQL
// y propaga cambios a todos los sorters
type SKUSyncWorker struct {
	ctx          context.Context
	cancel       context.CancelFunc
	sqlServerMgr *db.Manager         // SQL Server (vista)
	postgresMgr  *db.PostgresManager // PostgreSQL (tabla sku)
	skuManager   *SKUManager
	sorters      []shared.SorterInterface // Lista de sorters a notificar
	interval     time.Duration
	skuFormat    string // Formato de SKU global
}

// NewSKUSyncWorker crea una nueva instancia del worker de sincronización
func NewSKUSyncWorker(
	ctx context.Context,
	sqlServerMgr *db.Manager,
	postgresMgr *db.PostgresManager,
	skuManager *SKUManager,
	sorters []shared.SorterInterface,
	interval time.Duration,
	skuFormat string,
) *SKUSyncWorker {
	workerCtx, cancel := context.WithCancel(ctx)
	return &SKUSyncWorker{
		ctx:          workerCtx,
		cancel:       cancel,
		sqlServerMgr: sqlServerMgr,
		postgresMgr:  postgresMgr,
		skuManager:   skuManager,
		sorters:      sorters,
		interval:     interval,
		skuFormat:    skuFormat,
	}
}

// Start inicia el worker de sincronización
func (w *SKUSyncWorker) Start() {
	go w.run()
	log.Printf("🔄 SKU Sync Worker iniciado (intervalo: %v)", w.interval)
}

// Stop detiene el worker de sincronización
func (w *SKUSyncWorker) Stop() {
	w.cancel()
	log.Println("🛑 SKU Sync Worker detenido")
}

func (w *SKUSyncWorker) run() {
	// CRÍTICO: Capturar panics para debug
	defer func() {
		if r := recover(); r != nil {
			log.Printf("❌ PANIC en SKU Sync Worker: %v", r)
		}
	}()

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	log.Println("🔄 [Worker] Goroutine iniciada")

	// NOTA: NO recargamos desde PostgreSQL aquí porque:
	// 1. Ya se hizo en main.go antes de crear el worker
	// 2. El primer syncSKUs() inmediatamente recargará desde UNITEC (fuente de verdad)
	// 3. Evitamos race condition / deadlock por doble lectura concurrente
	log.Println("⏭️  [Worker] Omitiendo carga inicial (ya hecha en main.go)")

	// Ejecutar sync desde UNITEC inmediatamente después (actualiza desde fuente de verdad)
	log.Println("🔄 [Worker] Ejecutando primer syncSKUs()...")
	w.syncSKUs()
	log.Println("✅ [Worker] Primer syncSKUs() completado")

	log.Println("🔄 [Worker] Entrando al loop principal...")
	for {
		select {
		case <-w.ctx.Done():
			log.Println("🛑 [Worker] Contexto cancelado, saliendo...")
			return
		case <-ticker.C:
			log.Println("⏰ [Worker] Ticker disparado, ejecutando syncSKUs()...")
			w.syncSKUs()
			log.Println("✅ [Worker] syncSKUs() periódico completado")
		}
	}
}

func (w *SKUSyncWorker) syncSKUs() {
	startTime := time.Now()
	ctx, cancel := context.WithTimeout(w.ctx, 30*time.Second)
	defer cancel()

	log.Println("🔄 Iniciando sincronización de SKUs desde UNITEC...")

	// 1. Obtener SKUs desde SQL Server (fuente de verdad)
	skusFromSQLServer, err := w.sqlServerMgr.GetSKUsFromView(ctx, w.skuFormat)
	if err != nil {
		log.Printf("❌ Error al obtener SKUs desde SQL Server: %v", err)
		return
	}
	log.Printf("   → %d SKUs obtenidos desde SQL Server", len(skusFromSQLServer))

	// 2. Sincronizar con PostgreSQL (insertar/actualizar)
	inserted, updated, err := w.postgresMgr.SyncSKUs(ctx, skusFromSQLServer)
	if err != nil {
		log.Printf("❌ Error al sincronizar SKUs con PostgreSQL: %v", err)
		return
	}
	log.Printf("   → %d SKUs insertados, %d actualizados en PostgreSQL", inserted, updated)

	// 3. Recargar todos los SKUs desde PostgreSQL (ahora que está actualizado)
	if err := w.skuManager.ReloadFromDB(ctx); err != nil {
		log.Printf("❌ Error al recargar SKUs desde PostgreSQL: %v", err)
		return
	}
	log.Println("✅ SKUs recargados desde PostgreSQL")

	// 4. Propagar a todos los sorters
	activeSKUs := w.skuManager.GetActiveSKUs()
	assignableSKUs := []models.SKUAssignable{models.GetRejectSKU()} // Agregar REJECT al inicio

	for _, sku := range activeSKUs {
		assignableSKUs = append(assignableSKUs, sku.ToAssignableWithHash())
	}

	log.Printf("📤 Sync SKU: Propagando %d SKUs (REJECT + %d activos) a sorters...", len(assignableSKUs), len(activeSKUs))

	for _, sorter := range w.sorters {
		sorter.UpdateSKUs(assignableSKUs)
	}

	log.Printf("✅ Sincronización de SKUs completada en %v", time.Since(startTime))
}
