package flow

import (
	"context"
	"log"
	"time"

	"API-GREENEX/internal/db"
	"API-GREENEX/internal/models"
	"API-GREENEX/internal/shared"
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
}

// NewSKUSyncWorker crea una nueva instancia del worker de sincronización
func NewSKUSyncWorker(
	ctx context.Context,
	sqlServerMgr *db.Manager,
	postgresMgr *db.PostgresManager,
	skuManager *SKUManager,
	sorters []shared.SorterInterface,
	interval time.Duration,
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
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	// Ejecutar inmediatamente al inicio
	w.syncSKUs()

	for {
		select {
		case <-w.ctx.Done():
			return
		case <-ticker.C:
			w.syncSKUs()
		}
	}
}

func (w *SKUSyncWorker) syncSKUs() {
	startTime := time.Now()
	ctx, cancel := context.WithTimeout(w.ctx, 30*time.Second)
	defer cancel()

	log.Println("🔄 Iniciando sincronización de SKUs...")

	// 1. Consultar SQL Server (vista VW_INT_DANICH_ENVIVO)
	rows, err := db.FetchSegregazioneProgramma(ctx, w.sqlServerMgr)
	if err != nil {
		log.Printf("❌ Sync SKU: error consultando SQL Server: %v", err)
		return
	}

	if len(rows) == 0 {
		log.Println("⚠️  Sync SKU: vista SQL Server retornó 0 filas, no se hace cambio")
		return
	}

	log.Printf("📊 Sync SKU: %d filas obtenidas de SQL Server", len(rows))

	// 2. Usar transacción PostgreSQL para atomicidad
	tx, err := w.postgresMgr.BeginTx(ctx)
	if err != nil {
		log.Printf("❌ Sync SKU: error iniciando transacción: %v", err)
		return
	}
	defer tx.Rollback(ctx) // Rollback automático si no se hace commit

	// 3. PASO CRÍTICO: Marcar todas las SKUs como false
	if _, err := tx.Exec(ctx, db.UPDATE_TO_FALSE_SKU_STATE_INTERNAL_DB); err != nil {
		log.Printf("❌ Sync SKU: error marcando SKUs como false: %v", err)
		return
	}

	// 4. Insertar/actualizar desde vista con estado = true
	syncedCount := 0
	skippedCount := 0

	// Mapa para agrupar por tipo de SKU
	skuStats := make(map[string]int)

	for _, row := range rows {
		if !row.Variedad.Valid || !row.Calibre.Valid || !row.Embalaje.Valid {
			skippedCount++
			continue
		}

		calibre := row.Calibre.String
		variedad := row.Variedad.String
		embalaje := row.Embalaje.String

		// INSERT con ON CONFLICT → UPDATE estado = true
		result, err := tx.Exec(ctx, db.INSERT_SKU_INTERNAL_DB, calibre, variedad, embalaje, true)
		if err != nil {
			log.Printf("⚠️  Sync SKU: error upsert %s-%s-%s: %v", calibre, variedad, embalaje, err)
			continue
		}

		// Contar inserciones vs actualizaciones
		if result.RowsAffected() > 0 {
			// Agrupar stats por variedad
			skuStats[variedad]++

			// Podemos inferir si fue insert o update comparando con antes,
			// pero para simplificar contamos todo como "sincronizado"
			syncedCount++
		}
	}

	// 5. Commit de la transacción
	if err := tx.Commit(ctx); err != nil {
		log.Printf("❌ Sync SKU: error haciendo commit: %v", err)
		return
	}

	// 6. Recargar SKUManager desde PostgreSQL (solo SKUs con estado = true)
	if err := w.skuManager.ReloadFromDB(ctx); err != nil {
		log.Printf("❌ Sync SKU: error recargando SKUManager: %v", err)
		return
	}

	// 7. Propagar a todos los sorters
	activeSKUs := w.skuManager.GetActiveSKUs()
	assignableSKUs := []models.SKUAssignable{models.GetRejectSKU()} // Agregar REJECT al inicio

	for _, sku := range activeSKUs {
		assignableSKUs = append(assignableSKUs, sku.ToAssignableWithHash())
	}

	for _, s := range w.sorters {
		// Actualizar lista de SKUs disponibles
		s.UpdateSKUs(assignableSKUs)

		// Recargar asignaciones desde PostgreSQL para actualizar estado de salidas
		if err := s.ReloadSalidasFromDB(ctx); err != nil {
			log.Printf("⚠️  Sync SKU: error recargando salidas de Sorter #%d: %v", s.GetID(), err)
		}
	}

	elapsed := time.Since(startTime)

	// 8. Log de resumen agrupado
	log.Println("✅ ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	log.Printf("✅ Sync SKU completado en %v", elapsed)
	log.Printf("   📊 SKUs sincronizados: %d", syncedCount)
	log.Printf("   ⏭️  SKUs omitidos (nulos): %d", skippedCount)
	log.Printf("   🎯 SKUs activos finales: %d", len(activeSKUs))
	log.Printf("   📡 Sorters notificados: %d", len(w.sorters))

	if len(skuStats) > 0 {
		log.Println("   📈 Distribución por variedad:")
		for variedad, count := range skuStats {
			log.Printf("      • %s: %d SKUs", variedad, count)
		}
	}

	log.Println("✅ ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
}
