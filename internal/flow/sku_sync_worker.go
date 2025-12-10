package flow

import (
	"context"
	"log"
	"strings"
	"time"

	"API-DANSORT/internal/shared"
	"API-GREENEX/internal/db"
	"API-GREENEX/internal/models"
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

	// 1. Consultar SQL Server
	rows, err := db.FetchSegregazioneProgramma(ctx, w.sqlServerMgr)
	if err != nil {
		log.Printf("❌ Sync SKU: ERROR consultando SQL Server UNITEC: %v", err)
		log.Printf("   → Verifica: conexión, vista VW_INT_DANICH_ENVIVO, permisos")
		return
	}

	if len(rows) == 0 {
		log.Println("⚠️  Sync SKU: vista retornó 0 filas")
		log.Println("   → La vista está vacía o no tiene datos que cumplan WHERE (NOT NULL)")
		log.Println("   → No se modificará la tabla sku (se mantiene estado actual)")
		return
	}

	log.Printf("✅ Sync SKU: %d SKUs obtenidas de UNITEC", len(rows))

	// 1.5 CRÍTICO: Sincronizar variedades PRIMERO (sku.variedad es FK a variedad.codigo_variedad)
	// Extraer variedades únicas
	variedadesMap := make(map[string]string) // codigo -> nombre
	for _, row := range rows {
		if row.Variedad.Valid && row.NombreVariedad.Valid {
			codigo := strings.TrimSpace(row.Variedad.String)
			nombre := strings.TrimSpace(row.NombreVariedad.String)
			if codigo != "" && nombre != "" {
				variedadesMap[codigo] = nombre
			}
		}
	}
	log.Printf("🔄 Extrayendo %d variedades únicas de %d filas...", len(variedadesMap), len(rows))

	// Insertar/actualizar variedades en PostgreSQL ANTES de tocar SKUs
	variedadCount := 0
	variedadErrors := 0
	for codigo, nombre := range variedadesMap {
		if err := w.postgresMgr.InsertVariedad(ctx, codigo, nombre); err != nil {
			log.Printf("⚠️  Error al sincronizar variedad %s: %v", codigo, err)
			variedadErrors++
		} else {
			variedadCount++
		}
	}
	log.Printf("✅ Variedades sincronizadas: %d exitosas, %d errores", variedadCount, variedadErrors)

	// Si hay errores críticos en variedades, abortar para evitar FK violations
	if variedadErrors > 0 && variedadCount == 0 {
		log.Printf("❌ Sync SKU: no se pudo sincronizar ninguna variedad, abortando para evitar FK violations")
		return
	}

	log.Println("🔄 [Sync] Iniciando transacción PostgreSQL...")
	// 2. Usar transacción PostgreSQL para atomicidad
	tx, err := w.postgresMgr.BeginTx(ctx)
	if err != nil {
		log.Printf("❌ Sync SKU: error iniciando transacción: %v", err)
		return
	}
	defer tx.Rollback(ctx) // Rollback automático si no se hace commit
	log.Println("✅ [Sync] Transacción iniciada correctamente")

	log.Println("🔄 [Sync] Marcando todas las SKUs como false...")
	// 3. PASO CRÍTICO: Marcar todas las SKUs como false
	if _, err := tx.Exec(ctx, db.UPDATE_TO_FALSE_SKU_STATE_INTERNAL_DB); err != nil {
		log.Printf("❌ Sync SKU: error marcando SKUs como false: %v", err)
		return
	}
	log.Println("✅ [Sync] Todas las SKUs marcadas como false")

	// 4. Insertar/actualizar desde vista con estado = true
	syncedCount := 0
	skippedCount := 0

	// Mapa para agrupar por tipo de SKU
	skuStats := make(map[string]int)

	log.Printf("🔄 [Sync] Procesando %d SKUs para upsert...", len(rows))
	for idx, row := range rows {
		// Progreso cada 100 SKUs
		if idx > 0 && idx%100 == 0 {
			log.Printf("   📊 Progreso: %d/%d SKUs procesadas...", idx, len(rows))
		}

		if !row.Variedad.Valid || !row.Calibre.Valid || !row.Embalaje.Valid {
			skippedCount++
			continue
		}

		calibre := row.Calibre.String
		variedad := row.Variedad.String
		embalaje := row.Embalaje.String
		dark := int(0)
		if row.Dark.Valid {
			dark = int(row.Dark.Int64)
		}
		linea := ""
		if row.Linea.Valid {
			linea = strings.TrimSpace(row.Linea.String)
		}

		// INSERT con ON CONFLICT → UPDATE estado = true
		result, err := tx.Exec(ctx, db.INSERT_SKU_INTERNAL_DB, calibre, variedad, embalaje, dark, linea, true)
		if err != nil {
			log.Printf("⚠️  Sync SKU: error upsert %s-%s-%s (dark=%d, linea=%s): %v", calibre, variedad, embalaje, dark, linea, err)
			continue
		}

		// Contar inserciones vs actualizaciones
		if result.RowsAffected() > 0 {
			// Agrupar stats por variedad
			skuStats[variedad]++
			syncedCount++
		}
	}

	log.Printf("🔄 [Sync] Haciendo commit de transacción (%d SKUs sincronizados)...", syncedCount)
	// 5. Commit de la transacción
	if err := tx.Commit(ctx); err != nil {
		log.Printf("❌ Sync SKU: error haciendo commit: %v", err)
		return
	}
	log.Println("✅ [Sync] Commit exitoso")
	/*
		// 5.5. LIMPIEZA: Eliminar asignaciones de SKUs inactivas en salida_sku
		// IMPORTANTE: NO eliminar REJECT (calibre='REJECT') porque es una SKU especial permanente
		cleanupQuery := `
			DELETE FROM salida_sku ss
			WHERE ss.calibre != 'REJECT'
			  AND NOT EXISTS (
				SELECT 1 FROM sku s
				WHERE s.calibre = ss.calibre
				  AND s.variedad = ss.variedad
				  AND s.embalaje = ss.embalaje
				  AND s.dark = ss.dark
				  AND s.estado = true
			)
		`
		result, err := w.postgresMgr.Pool().Exec(ctx, cleanupQuery)
		if err != nil {
			log.Printf("⚠️  Sync SKU: error limpiando asignaciones inactivas: %v", err)
		} else if result.RowsAffected() > 0 {
			log.Printf("🧹 Sync SKU: %d asignaciones de SKUs inactivas eliminadas", result.RowsAffected())
		}
	*/
	log.Println("🔄 [Sync] Recargando SKUManager desde PostgreSQL...")
	// 6. Recargar SKUManager desde PostgreSQL (solo SKUs con estado = true)
	if err := w.skuManager.ReloadFromDB(ctx); err != nil {
		log.Printf("❌ Sync SKU: error recargando SKUManager: %v", err)
		return
	}
	log.Println("✅ [Sync] SKUManager recargado exitosamente")

	// 7. Propagar a todos los sorters
	activeSKUs := w.skuManager.GetActiveSKUs()
	assignableSKUs := []models.SKUAssignable{models.GetRejectSKU()} // Agregar REJECT al inicio

	for _, sku := range activeSKUs {
		assignableSKUs = append(assignableSKUs, sku.ToAssignableWithHash())
	}

	log.Printf("📤 Sync SKU: Propagando %d SKUs (REJECT + %d activos) a sorters...", len(assignableSKUs), len(activeSKUs))

	for _, s := range w.sorters {
		// Actualizar lista de SKUs disponibles
		s.UpdateSKUs(assignableSKUs)
		log.Printf("   → Sorter #%d: %d SKUs actualizados", s.GetID(), len(assignableSKUs))

		// NOTA: NO recargamos salidas aquí porque sobrescribiría las asignaciones manuales del usuario
		// Las asignaciones se recargan solo:
		// 1. Al iniciar el sistema
		// 2. Cuando el usuario hace cambios via API (POST/DELETE /assignment)
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
