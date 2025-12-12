package flow

import (
	"context"
	"fmt"
	"log"
	"strings"
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
	log.Printf("🔄 [Sync SKU] Worker iniciado (intervalo: %v)", w.interval)
}

// Stop detiene el worker de sincronización
func (w *SKUSyncWorker) Stop() {
	w.cancel()
	log.Println("🛑 [Sync SKU] Worker detenido")
}

func (w *SKUSyncWorker) run() {
	// CRÍTICO: Capturar panics para debug
	defer func() {
		if r := recover(); r != nil {
			log.Printf("❌ [Sync SKU] PANIC en worker: %v", r)
		}
	}()

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	log.Println("🔄 [Sync SKU] Goroutine iniciada")

	// NOTA: NO recargamos desde PostgreSQL aquí porque:
	// 1. Ya se hizo en main.go antes de crear el worker
	// 2. El primer syncSKUs() inmediatamente recargará desde UNITEC (fuente de verdad)
	// 3. Evitamos race condition / deadlock por doble lectura concurrente
	log.Println("⏭️  [Sync SKU] Omitiendo carga inicial (ya hecha en main.go)")

	// Ejecutar sync desde UNITEC inmediatamente después (actualiza desde fuente de verdad)
	log.Println("🔄 [Sync SKU] Ejecutando primer sync...")
	w.syncSKUs()
	log.Println("✅ [Sync SKU] Primer sync completado")

	log.Println("🔄 [Sync SKU] Entrando al loop principal...")
	for {
		select {
		case <-w.ctx.Done():
			log.Println("🛑 [Sync SKU] Contexto cancelado, saliendo...")
			return
		case <-ticker.C:
			log.Println("⏰ [Sync SKU] Ticker disparado, ejecutando sync...")
			w.syncSKUs()
			log.Println("✅ [Sync SKU] Sync periódico completado")
		}
	}
}

func (w *SKUSyncWorker) syncSKUs() {
	startTime := time.Now()
	ctx, cancel := context.WithTimeout(w.ctx, 30*time.Second)
	defer cancel()

	log.Println("🔄 [Sync SKU] Iniciando sincronización desde UNITEC...")

	// 1. Consultar SQL Server (vista VW_INT_DANICH_ENVIVO)
	rows, err := db.FetchSegregazioneProgramma(ctx, w.sqlServerMgr)
	if err != nil {
		log.Printf("❌ [Sync SKU] ERROR consultando SQL Server UNITEC: %v", err)
		log.Printf("   → Verifica: conexión, vista VW_INT_DANICH_ENVIVO, permisos")
		return
	}

	if len(rows) == 0 {
		log.Println("⚠️  [Sync SKU] Vista VW_INT_DANICH_ENVIVO retornó 0 filas")
		log.Println("   → La vista está vacía o no tiene datos que cumplan WHERE (NOT NULL)")
		log.Println("   → No se modificará la tabla sku (se mantiene estado actual)")
		return
	}

	log.Printf("✅ [Sync SKU] %d SKUs obtenidas de UNITEC", len(rows))

	// 1.1 VERIFICAR DUPLICADOS Y LISTAR TODAS LAS SKUs DE UNITEC
	unitecSKUs := make(map[string]int)
	unitecSKUList := make([]string, 0, len(rows))
	duplicates := 0

	log.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	log.Println("📋 [Sync SKU] ANÁLISIS DE DATOS DE UNITEC")
	log.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	for i, row := range rows {
		if !row.Variedad.Valid || !row.Calibre.Valid || !row.Embalaje.Valid {
			log.Printf("   ⏭️  Fila %d: OMITIDA (campos NULL)", i+1)
			continue
		}

		calibre := strings.TrimSpace(row.Calibre.String)
		variedad := strings.TrimSpace(row.Variedad.String)
		embalaje := strings.TrimSpace(row.Embalaje.String)
		dark := int(0)
		if row.Dark.Valid {
			dark = int(row.Dark.Int64)
		}
		linea := "0"
		if row.Linea.Valid {
			linea = strings.TrimSpace(row.Linea.String)
		}

		skuKey := fmt.Sprintf("%s-%s-%s-%d-%s", calibre, variedad, embalaje, dark, linea)
		unitecSKUs[skuKey]++

		if unitecSKUs[skuKey] == 1 {
			unitecSKUList = append(unitecSKUList, skuKey)
			log.Printf("   ✅ %2d. %s", len(unitecSKUList), skuKey)
		} else {
			duplicates++
			log.Printf("   🚨 %2d. %s [DUPLICADA x%d]", i+1, skuKey, unitecSKUs[skuKey])
		}
	}

	log.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	log.Printf("📊 [Sync SKU] Total filas UNITEC: %d", len(rows))
	log.Printf("📊 [Sync SKU] SKUs ÚNICAS: %d", len(unitecSKUs))
	log.Printf("📊 [Sync SKU] Duplicados detectados: %d", duplicates)

	if duplicates > 0 {
		log.Printf("🚨 [Sync SKU] ¡ADVERTENCIA! Hay %d SKUs duplicadas en UNITEC", duplicates)
		log.Println("   → Esto indica un problema en la vista VW_INT_DANICH_ENVIVO")
		log.Println("   → Solo se insertará UNA vez cada SKU única")
	}

	log.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

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

	// Insertar/actualizar variedades en PostgreSQL ANTES de tocar SKUs
	variedadCount := 0
	variedadErrors := 0
	for codigo, nombre := range variedadesMap {
		if err := w.postgresMgr.InsertVariedad(ctx, codigo, nombre); err != nil {
			log.Printf("⚠️  [Sync SKU] Error al sincronizar variedad %s: %v", codigo, err)
			variedadErrors++
		} else {
			variedadCount++
		}
	}
	log.Printf("✅ [Sync SKU] Variedades sincronizadas: %d exitosas, %d errores", variedadCount, variedadErrors)

	// Si hay errores críticos en variedades, abortar para evitar FK violations
	if variedadErrors > 0 && variedadCount == 0 {
		log.Printf("❌ [Sync SKU] No se pudo sincronizar ninguna variedad, abortando para evitar FK violations")
		return
	}

	log.Println("🔄 [Sync SKU] Iniciando transacción PostgreSQL...")
	// 2. Usar transacción PostgreSQL para atomicidad
	tx, err := w.postgresMgr.BeginTx(ctx)
	if err != nil {
		log.Printf("❌ [Sync SKU] Error iniciando transacción: %v", err)
		return
	}
	defer tx.Rollback(ctx) // Rollback automático si no se hace commit
	log.Println("✅ [Sync SKU] Transacción iniciada correctamente")

	log.Println("🔄 [Sync SKU] Marcando todas las SKUs como false...")
	// 3. PASO CRÍTICO: Marcar todas las SKUs como false
	if _, err := tx.Exec(ctx, db.UPDATE_TO_FALSE_SKU_STATE_INTERNAL_DB); err != nil {
		log.Printf("❌ [Sync SKU] Error marcando SKUs como false: %v", err)
		return
	}
	log.Println("✅ [Sync SKU] Todas las SKUs marcadas como false")

	// 4. Insertar/actualizar desde vista con estado = true
	syncedCount := 0
	skippedCount := 0
	errorCount := 0

	// Mapa para agrupar por tipo de SKU
	skuStats := make(map[string]int)
	errorDetails := make([]string, 0)

	log.Printf("🔄 [Sync SKU] Procesando %d filas de UNITEC...", len(rows))

	for i, row := range rows {
		// Log de valores antes de validar
		if !row.Variedad.Valid || !row.Calibre.Valid || !row.Embalaje.Valid {
			skippedCount++
			log.Printf("⏭️  [Sync SKU] Fila %d/%d OMITIDA: campos NULL (Variedad=%v, Calibre=%v, Embalaje=%v)",
				i+1, len(rows), row.Variedad.Valid, row.Calibre.Valid, row.Embalaje.Valid)
			continue
		}

		calibre := strings.TrimSpace(row.Calibre.String)
		variedad := strings.TrimSpace(row.Variedad.String)
		embalaje := strings.TrimSpace(row.Embalaje.String)
		dark := int(0)
		if row.Dark.Valid {
			dark = int(row.Dark.Int64)
		}
		linea := "0"
		if row.Linea.Valid {
			linea = strings.TrimSpace(row.Linea.String)
		}

		skuKey := calibre + "-" + variedad + "-" + embalaje + "-" + linea

		// INSERT con ON CONFLICT → UPDATE estado = true
		result, err := tx.Exec(ctx, db.INSERT_SKU_INTERNAL_DB, calibre, variedad, embalaje, dark, linea, true)
		if err != nil {
			errorCount++
			errMsg := err.Error()
			log.Printf("❌ [Sync SKU] Fila %d/%d ERROR: SKU=%s", i+1, len(rows), skuKey)
			log.Printf("   → Calibre='%s', Variedad='%s', Embalaje='%s', Dark=%d, Linea='%s'",
				calibre, variedad, embalaje, dark, linea)
			log.Printf("   → Error: %v", err)

			// Detectar tipo de error
			if strings.Contains(errMsg, "violates foreign key constraint") && strings.Contains(errMsg, "variedad") {
				log.Printf("   → CAUSA: Variedad '%s' no existe en tabla 'variedad'", variedad)
				log.Printf("   → SOLUCIÓN: Verificar que la variedad se haya sincronizado correctamente antes")
			} else if strings.Contains(errMsg, "duplicate key") {
				log.Printf("   → CAUSA: Clave duplicada (posible conflicto de constraint único)")
			}

			errorDetails = append(errorDetails, skuKey)
			continue
		}

		// Contar inserciones vs actualizaciones
		rowsAffected := result.RowsAffected()

		if rowsAffected > 0 {
			// Agrupar stats por variedad
			skuStats[variedad]++
			syncedCount++
		} else {
			log.Printf("⚠️  [Sync SKU] Fila %d/%d: NO hubo cambios para SKU=%s (rowsAffected=0)",
				i+1, len(rows), skuKey)
		}
	}

	log.Printf("📊 [Sync SKU] Procesamiento completado: %d sincronizados, %d omitidos, %d errores",
		syncedCount, skippedCount, errorCount)

	// CRÍTICO: Si hay errores, listarlos antes del commit
	if errorCount > 0 {
		log.Printf("⚠️  [Sync SKU] Se detectaron %d errores durante el procesamiento:", errorCount)
		for i, skuKey := range errorDetails {
			log.Printf("   %d. SKU fallida: %s", i+1, skuKey)
		}
		log.Printf("⚠️  [Sync SKU] ADVERTENCIA: Solo %d de %d SKUs se sincronizarán correctamente",
			syncedCount, len(rows))
	}

	// Validación crítica: verificar que syncedCount coincida con rows esperadas
	expectedCount := len(rows) - skippedCount
	if syncedCount != expectedCount {
		log.Printf("🚨 [Sync SKU] ERROR CRÍTICO: Discrepancia en conteo!")
		log.Printf("   → SKUs recibidas de UNITEC: %d", len(rows))
		log.Printf("   → SKUs omitidas (NULL): %d", skippedCount)
		log.Printf("   → SKUs esperadas: %d", expectedCount)
		log.Printf("   → SKUs sincronizadas: %d", syncedCount)
		log.Printf("   → SKUs perdidas: %d", expectedCount-syncedCount)
		log.Printf("   → SKUs con error: %d", errorCount)
	}

	log.Printf("🔄 [Sync SKU] Haciendo commit de transacción (%d SKUs sincronizados)...", syncedCount)
	// 5. Commit de la transacción
	if err := tx.Commit(ctx); err != nil {
		log.Printf("❌ [Sync SKU] Error haciendo commit: %v", err)
		return
	}
	log.Println("✅ [Sync SKU] Commit exitoso")

	// 5.1 VERIFICACIÓN POST-COMMIT: Consultar directamente PostgreSQL
	log.Println("")
	log.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	log.Println("🔍 [Sync SKU] VERIFICACIÓN POST-COMMIT")
	log.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	// Consultar SKUs con estado=true directamente
	queryCtx, queryCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer queryCancel()

	pgRows, err := w.postgresMgr.Query(queryCtx, `
		SELECT calibre, variedad, embalaje, dark, linea 
		FROM sku 
		WHERE estado = true 
		ORDER BY variedad, calibre, embalaje, linea
	`)
	if err != nil {
		log.Printf("❌ [Sync SKU] Error consultando PostgreSQL post-commit: %v", err)
	} else {
		defer pgRows.Close()

		pgSKUs := make(map[string]bool)
		pgSKUList := make([]string, 0)

		for pgRows.Next() {
			var calibre, variedad, embalaje, linea string
			var dark int
			if err := pgRows.Scan(&calibre, &variedad, &embalaje, &dark, &linea); err != nil {
				log.Printf("⚠️  Error escaneando fila PostgreSQL: %v", err)
				continue
			}
			skuKey := fmt.Sprintf("%s-%s-%s-%d-%s", calibre, variedad, embalaje, dark, linea)
			pgSKUs[skuKey] = true
			pgSKUList = append(pgSKUList, skuKey)
		}

		log.Printf("📊 [Sync SKU] SKUs en PostgreSQL (estado=true): %d", len(pgSKUs))
		log.Println("📋 [Sync SKU] Listado completo de PostgreSQL:")
		for i, sku := range pgSKUList {
			log.Printf("   ✅ %2d. %s", i+1, sku)
		}

		// COMPARAR: UNITEC vs PostgreSQL
		log.Println("")
		log.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		log.Println("🔎 [Sync SKU] COMPARACIÓN UNITEC vs PostgreSQL")
		log.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

		// SKUs en UNITEC pero NO en PostgreSQL (PERDIDAS)
		missing := make([]string, 0)
		for _, skuKey := range unitecSKUList {
			if !pgSKUs[skuKey] {
				missing = append(missing, skuKey)
			}
		}

		// SKUs en PostgreSQL pero NO en UNITEC (NO DEBERÍAN EXISTIR)
		extra := make([]string, 0)
		for _, skuKey := range pgSKUList {
			if unitecSKUs[skuKey] == 0 {
				extra = append(extra, skuKey)
			}
		}

		log.Printf("📊 SKUs esperadas de UNITEC: %d", len(unitecSKUs))
		log.Printf("📊 SKUs encontradas en PostgreSQL: %d", len(pgSKUs))
		log.Printf("📊 SKUs PERDIDAS: %d", len(missing))
		log.Printf("📊 SKUs EXTRA (no deberían estar): %d", len(extra))

		if len(missing) > 0 {
			log.Println("")
			log.Println("🚨 SKUs PERDIDAS (en UNITEC pero NO en PostgreSQL):")
			for i, sku := range missing {
				log.Printf("   ❌ %d. %s", i+1, sku)
			}
		}

		if len(extra) > 0 {
			log.Println("")
			log.Println("⚠️  SKUs EXTRA (en PostgreSQL pero NO en UNITEC):")
			for i, sku := range extra {
				log.Printf("   ⚠️  %d. %s", i+1, sku)
			}
		}

		if len(missing) == 0 && len(extra) == 0 {
			log.Println("✅ PERFECTO: UNITEC y PostgreSQL están 100% sincronizados")
		} else {
			log.Println("🚨 ERROR CRÍTICO: Hay discrepancias entre UNITEC y PostgreSQL")
		}

		log.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	}

	log.Println("")
	log.Println("🔄 [Sync SKU] Recargando SKUManager desde PostgreSQL...")
	// 6. Recargar SKUManager desde PostgreSQL (todas las SKUs, sin filtro de estado)
	log.Println("   → Iniciando ReloadFromDB()...")
	if err := w.skuManager.ReloadFromDB(ctx); err != nil {
		log.Printf("❌ [Sync SKU] Error recargando SKUManager: %v", err)
		return
	}
	log.Println("✅ [Sync SKU] SKUManager recargado exitosamente")

	// 7. Propagar a todos los sorters (solo SKUs activas = las de UNITEC)
	// Las SKUs de UNITEC siempre están en true después del sync
	log.Println("🔄 [Sync SKU] Obteniendo SKUs activas del manager...")
	activeSKUs := w.skuManager.GetActiveSKUs()
	log.Printf("   → SKUs activas obtenidas: %d", len(activeSKUs))

	log.Println("🔄 [Sync SKU] Construyendo lista de SKUs asignables...")
	assignableSKUs := []models.SKUAssignable{models.GetRejectSKU()} // Agregar REJECT al inicio
	log.Printf("   → REJECT agregada (ID=0)")

	for i, sku := range activeSKUs {
		assignable := sku.ToAssignableWithHash()
		assignableSKUs = append(assignableSKUs, assignable)
		if i < 5 { // Mostrar solo las primeras 5 para no saturar logs
			// Mostrar clave completa para evitar confusión (incluye línea)
			skuKey := fmt.Sprintf("%s-%s-%s-%d-%s", sku.Calibre, sku.Variedad, sku.Embalaje, sku.Dark, sku.Linea)
			log.Printf("   → SKU #%d: %s (ID=%d)", i+1, skuKey, assignable.ID)
		}
	}
	if len(activeSKUs) > 5 {
		log.Printf("   → ... y %d SKUs más", len(activeSKUs)-5)
	}

	log.Printf("📤 [Sync SKU] Propagando %d SKUs (REJECT + %d activas de UNITEC) a sorters...", len(assignableSKUs), len(activeSKUs))

	for _, s := range w.sorters {
		sorterID := s.GetID()
		log.Printf("   🔄 Propagando a Sorter #%d...", sorterID)

		// Actualizar lista de SKUs disponibles
		s.UpdateSKUs(assignableSKUs)
		log.Printf("   ✅ Sorter #%d: %d SKUs actualizados correctamente", sorterID, len(assignableSKUs))

		// NOTA: NO recargamos salidas aquí porque sobrescribiría las asignaciones manuales del usuario
		// Las asignaciones se recargan solo:
		// 1. Al iniciar el sistema
		// 2. Cuando el usuario hace cambios via API (POST/DELETE /assignment)
	}

	log.Printf("✅ [Sync SKU] Propagación completada a %d sorters", len(w.sorters))

	elapsed := time.Since(startTime)

	// 8. Log de resumen agrupado
	log.Println("✅ ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	log.Printf("✅ [Sync SKU] Completado en %v", elapsed)
	log.Printf("   📊 SKUs recibidas de UNITEC: %d", len(rows))
	log.Printf("   ✅ SKUs sincronizados exitosamente: %d", syncedCount)
	log.Printf("   ⏭️  SKUs omitidos (campos NULL): %d", skippedCount)
	log.Printf("   ❌ SKUs con errores: %d", errorCount)
	log.Printf("   🎯 SKUs activas propagadas: %d", len(activeSKUs))
	log.Printf("   📡 Sorters notificados: %d", len(w.sorters))

	// Validación final: alertar si hay discrepancia
	expectedSync := len(rows) - skippedCount - errorCount
	if syncedCount != expectedSync {
		log.Printf("   🚨 ALERTA: Discrepancia detectada (esperadas=%d, sincronizadas=%d)",
			expectedSync, syncedCount)
	}

	if len(skuStats) > 0 {
		log.Println("   📈 Distribución por variedad:")
		for variedad, count := range skuStats {
			log.Printf("      • %s: %d SKUs", variedad, count)
		}
	}

	if errorCount > 0 {
		log.Printf("   ⚠️  SKUs fallidas: %v", errorDetails)
	}

	log.Println("✅ ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
}

// SyncOnce ejecuta una sincronización única (útil para comandos standalone)
// Este método es público y puede ser llamado desde fuera del paquete
func (w *SKUSyncWorker) SyncOnce() {
	log.Println("🔄 [Sync SKU] Ejecutando sincronización única...")
	w.syncSKUs()
	log.Println("✅ [Sync SKU] Sincronización única completada")
}
