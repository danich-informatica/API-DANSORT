package db

import (
	"API-GREENEX/internal/models"
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/jackc/pgx/v5"
)

// CajaInfo representa información de una caja para el dashboard
type CajaInfo struct {
	Correlativo string
	Especie     string
	Variedad    string
	Calibre     string
	Embalaje    string
	Fecha       string
}

// GetRecentBoxes obtiene las últimas N cajas procesadas
func (m *PostgresManager) GetRecentBoxes(ctx context.Context, limit int) ([]CajaInfo, error) {
	if m == nil || m.pool == nil {
		return nil, fmt.Errorf("manager no inicializado")
	}

	rows, err := m.pool.Query(ctx, SELECT_RECENT_BOXES_INTERNAL_DB, limit)
	if err != nil {
		return nil, fmt.Errorf("error al consultar cajas recientes: %w", err)
	}
	defer rows.Close()

	var cajas []CajaInfo
	for rows.Next() {
		var caja CajaInfo
		err := rows.Scan(
			&caja.Correlativo,
			&caja.Especie,
			&caja.Variedad,
			&caja.Calibre,
			&caja.Embalaje,
			&caja.Fecha,
		)
		if err != nil {
			return nil, fmt.Errorf("error al escanear fila: %w", err)
		}
		cajas = append(cajas, caja)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error al iterar filas: %w", err)
	}

	return cajas, nil
}

// GetTotalBoxesCount obtiene el total de cajas procesadas
func (m *PostgresManager) GetTotalBoxesCount(ctx context.Context) (int, error) {
	if m == nil || m.pool == nil {
		return 0, fmt.Errorf("manager no inicializado")
	}

	var count int
	err := m.pool.QueryRow(ctx, COUNT_BOXES_INTERNAL_DB).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("error al contar cajas: %w", err)
	}

	return count, nil
}

// PostgresDBAdapter adapta PostgresManager a la interfaz requerida por HTTPFrontend
type PostgresDBAdapter struct {
	manager *PostgresManager
}

// NewPostgresDBAdapter crea un nuevo adaptador
func NewPostgresDBAdapter(manager *PostgresManager) *PostgresDBAdapter {
	return &PostgresDBAdapter{manager: manager}
}

// GetRecentBoxes implementa la interfaz con tipos interface{}
func (a *PostgresDBAdapter) GetRecentBoxes(ctx interface{}, limit int) (interface{}, error) {
	ctxTyped, ok := ctx.(context.Context)
	if !ok {
		return nil, fmt.Errorf("contexto inválido")
	}
	return a.manager.GetRecentBoxes(ctxTyped, limit)
}

// GetTotalBoxesCount implementa la interfaz con tipos interface{}
func (a *PostgresDBAdapter) GetTotalBoxesCount(ctx interface{}) (int, error) {
	ctxTyped, ok := ctx.(context.Context)
	if !ok {
		return 0, fmt.Errorf("contexto inválido")
	}
	return a.manager.GetTotalBoxesCount(ctxTyped)
}

// InsertSKU inserta una única SKU en la base de datos
// Excluye SKUs nulas, inválidas y duplicadas
func (m *PostgresManager) InsertSKU(ctx context.Context, calibre, variedad, embalaje string) error {
	if m == nil || m.pool == nil {
		return fmt.Errorf("manager no inicializado")
	}

	// Validar que no sea nulo o vacío
	if isNullOrEmpty(calibre, variedad, embalaje) {
		return fmt.Errorf("SKU inválida: componentes nulos o vacíos")
	}

	result, err := m.pool.Exec(ctx, INSERT_SKU_IF_NOT_EXISTS_INTERNAL_DB,
		strings.TrimSpace(calibre),
		strings.TrimSpace(variedad),
		strings.TrimSpace(embalaje),
		true)

	if err != nil {
		return fmt.Errorf("error al insertar SKU: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("SKU ya existe en la base de datos")
	}

	return nil
}

// GetAllSKUs obtiene todas las SKUs de la base de datos
func (m *PostgresManager) GetAllSKUs(ctx context.Context) ([]models.SKU, error) {
	if m == nil || m.pool == nil {
		return nil, fmt.Errorf("manager no inicializado")
	}

	rows, err := m.pool.Query(ctx, SELECT_ALL_SKUS_INTERNAL_DB)
	if err != nil {
		return nil, fmt.Errorf("error al consultar SKUs: %w", err)
	}
	defer rows.Close()

	var skus []models.SKU

	for rows.Next() {
		var sku models.SKU

		if err := rows.Scan(&sku.Calibre, &sku.Variedad, &sku.Embalaje, &sku.SKU, &sku.Estado); err != nil {
			return nil, fmt.Errorf("error al escanear fila: %w", err)
		}

		skus = append(skus, sku)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error al iterar filas: %w", err)
	}

	return skus, nil
}

// CheckSKUExists verifica si una SKU existe en la base de datos
func (m *PostgresManager) CheckSKUExists(ctx context.Context, calibre, variedad, embalaje string) (bool, error) {
	if m == nil || m.pool == nil {
		return false, fmt.Errorf("manager no inicializado")
	}

	var exists bool
	err := m.pool.QueryRow(ctx, SELECT_IF_EXISTS_SKU_INTERNAL_DB,
		strings.TrimSpace(calibre),
		strings.TrimSpace(variedad),
		strings.TrimSpace(embalaje)).Scan(&exists)

	if err != nil && err != pgx.ErrNoRows {
		return false, fmt.Errorf("error al verificar existencia de SKU: %w", err)
	}

	return exists, nil
}

// isNullOrEmpty verifica si alguno de los componentes es nulo, vacío o "(NULL)"
func isNullOrEmpty(calibre, variedad, embalaje string) bool {
	calibre = strings.TrimSpace(calibre)
	variedad = strings.TrimSpace(variedad)
	embalaje = strings.TrimSpace(embalaje)

	// Verificar vacíos
	if calibre == "" || variedad == "" || embalaje == "" {
		return true
	}

	// Verificar "(NULL)"
	if strings.ToUpper(calibre) == "(NULL)" ||
		strings.ToUpper(variedad) == "(NULL)" ||
		strings.ToUpper(embalaje) == "(NULL)" {
		return true
	}

	return false
}

func (m *PostgresManager) InsertNewBox(ctx context.Context, especie, variedad, calibre, embalaje string) (string, error) {
	// NO crear un nuevo manager cada vez, usar el singleton existente
	if m == nil || m.pool == nil {
		return "", fmt.Errorf("gestor de base de datos no inicializado")
	}

	// Paso 1: Verificar si la SKU existe, si no existe, crearla
	exists, err := m.CheckSKUExists(ctx, calibre, variedad, embalaje)
	if err != nil {
		return "", fmt.Errorf("error al verificar SKU: %w", err)
	}

	if !exists {
		log.Printf("⚠️  SKU no existe (%s-%s-%s), creándola automáticamente...", calibre, variedad, embalaje)

		// Insertar la SKU con estado true
		err = m.InsertSKU(ctx, calibre, variedad, embalaje)
		if err != nil {
			return "", fmt.Errorf("error al crear SKU: %w", err)
		}

		log.Printf("✅ SKU creada: %s-%s-%s", calibre, variedad, embalaje)
	}

	// Paso 2: Insertar la caja
	var correlativo string // ⚠️ CAMBIO: ahora es string, no int

	err = m.pool.QueryRow(ctx, INSERT_CAJA_SIN_CORRELATIVO_INTERNAL_DB, especie, variedad, calibre, embalaje).Scan(&correlativo)
	if err != nil {
		return "", fmt.Errorf("error al insertar caja: %w", err)
	}

	log.Printf("📦 Correlativo de caja insertado: %s", correlativo)
	return correlativo, nil
}

func (m *PostgresManager) GetActiveSKUs(ctx context.Context) ([]models.SKU, error) {
	if m == nil || m.pool == nil {
		return nil, fmt.Errorf("manager no inicializado")
	}

	rows, err := m.pool.Query(ctx, SELECT_ACTIVE_SKUS_INTERNAL_DB)
	if err != nil {
		return nil, fmt.Errorf("error al consultar SKUs activas: %w", err)
	}
	defer rows.Close()

	var skus []models.SKU

	for rows.Next() {
		var sku models.SKU
		if err := rows.Scan(&sku.Calibre, &sku.Variedad, &sku.Embalaje, &sku.SKU, &sku.Estado); err != nil {
			return nil, fmt.Errorf("error al escanear fila: %w", err)
		}
		skus = append(skus, sku)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error al iterar filas: %w", err)
	}

	return skus, nil
}

// Inserta un sorter en la base de datos si no existe usando la query de queries.go
func (m *PostgresManager) InsertSorterIfNotExists(ctx context.Context, id int, ubicacion string) error {
	if m == nil || m.pool == nil {
		return fmt.Errorf("manager no inicializado")
	}
	_, err := m.pool.Exec(ctx, INSERT_NEW_SORTER_IF_NOT_EXISTS_INTERNAL_DB, id, ubicacion)
	if err != nil {
		return fmt.Errorf("error al insertar sorter: %w", err)
	}
	return nil
}

// Inserta una salida en la base de datos si no existe usando la query de queries.go
func (m *PostgresManager) InsertSalidaIfNotExists(ctx context.Context, id int, sorterID int, salidaSorter int, estado bool) error {
	if m == nil || m.pool == nil {
		return fmt.Errorf("manager no inicializado")
	}
	_, err := m.pool.Exec(ctx, INSERT_NEW_SALIDA_IF_NOT_EXISTS_INTERNAL_DB, id, sorterID, salidaSorter, estado)
	if err != nil {
		return fmt.Errorf("error al insertar salida: %w", err)
	}
	return nil
}

// InsertSalidaSKU inserta una asignación de SKU a salida en la tabla salida_sku
func (m *PostgresManager) InsertSalidaSKU(ctx context.Context, salidaID int, calibre, variedad, embalaje string) error {
	if m == nil || m.pool == nil {
		return fmt.Errorf("manager no inicializado")
	}
	_, err := m.pool.Exec(ctx, INSERT_SALIDA_SKU_INTERNAL_DB, salidaID, calibre, variedad, embalaje)
	if err != nil {
		return fmt.Errorf("error al insertar asignación salida-sku: %w", err)
	}
	return nil
}

// LoadAssignedSKUsForSorter carga todas las SKUs asignadas a las salidas de un sorter específico
// Retorna un mapa donde la clave es el salida_id y el valor es un slice de SKUs
func (m *PostgresManager) LoadAssignedSKUsForSorter(ctx context.Context, sorterID int) (map[int][]models.SKU, error) {
	if m == nil || m.pool == nil {
		return nil, fmt.Errorf("manager no inicializado")
	}

	rows, err := m.pool.Query(ctx, SELECT_ASSIGNED_SKUS_FOR_SORTER_INTERNAL_DB, sorterID)
	if err != nil {
		return nil, fmt.Errorf("error al consultar SKUs asignadas: %w", err)
	}
	defer rows.Close()

	// Mapa para agrupar SKUs por salida_id
	skusBySalida := make(map[int][]models.SKU)

	for rows.Next() {
		var salidaID int
		var salidaSorter string
		var salidaEstado bool
		var calibre, variedad, embalaje string
		var skuEstado bool

		err := rows.Scan(
			&salidaID,
			&salidaSorter,
			&salidaEstado,
			&calibre,
			&variedad,
			&embalaje,
			&skuEstado,
		)
		if err != nil {
			return nil, fmt.Errorf("error al escanear fila: %w", err)
		}

		// Construir el SKU completo: calibre-variedad-embalaje
		skuName := fmt.Sprintf("%s-%s-%s", calibre, variedad, embalaje)

		sku := models.SKU{
			Calibre:  calibre,
			Variedad: variedad,
			Embalaje: embalaje,
			SKU:      skuName,
			Estado:   skuEstado,
		}

		// Agregar SKU al mapa agrupado por salida_id
		skusBySalida[salidaID] = append(skusBySalida[salidaID], sku)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error al iterar filas: %w", err)
	}

	return skusBySalida, nil
}

// DeleteSalidaSKU elimina una SKU específica de una salida en la base de datos
func (m *PostgresManager) DeleteSalidaSKU(ctx context.Context, salidaID int, calibre, variedad, embalaje string) error {
	if m == nil || m.pool == nil {
		return fmt.Errorf("manager no inicializado")
	}

	// Primero verificar si existe
	var exists bool
	err := m.pool.QueryRow(ctx, CHECK_SALIDA_SKU_EXISTS_INTERNAL_DB, salidaID, calibre, variedad, embalaje).Scan(&exists)
	if err != nil {
		return fmt.Errorf("error al verificar existencia de SKU: %w", err)
	}

	if !exists {
		return fmt.Errorf("SKU %s-%s-%s no encontrada en salida %d", calibre, variedad, embalaje, salidaID)
	}

	// Ejecutar DELETE
	commandTag, err := m.pool.Exec(ctx, DELETE_SALIDA_SKU_INTERNAL_DB, salidaID, calibre, variedad, embalaje)
	if err != nil {
		return fmt.Errorf("error al eliminar SKU de salida: %w", err)
	}

	if commandTag.RowsAffected() == 0 {
		return fmt.Errorf("no se pudo eliminar la SKU de la salida %d", salidaID)
	}

	log.Printf("✅ [DB] SKU %s-%s-%s eliminada de salida %d", calibre, variedad, embalaje, salidaID)
	return nil
}

// DeleteAllSalidaSKUs elimina TODAS las SKUs asignadas a una salida
func (m *PostgresManager) DeleteAllSalidaSKUs(ctx context.Context, salidaID int) (int64, error) {
	if m == nil || m.pool == nil {
		return 0, fmt.Errorf("manager no inicializado")
	}

	commandTag, err := m.pool.Exec(ctx, DELETE_ALL_SALIDA_SKUS_INTERNAL_DB, salidaID)
	if err != nil {
		return 0, fmt.Errorf("error al eliminar SKUs de salida: %w", err)
	}

	rowsAffected := commandTag.RowsAffected()
	log.Printf("✅ [DB] Eliminadas %d SKUs de salida %d", rowsAffected, salidaID)
	return rowsAffected, nil
}

// InsertSalidaCaja registra que una caja fue enviada a una salida específica
// Parámetros:
//   - correlativo: Correlativo de la caja (ej: "10888")
//   - salidaID: ID de la salida física en la tabla salida (ej: 8)
//   - salidaRelativa: Número relativo de salida del sorter (1, 2, 3, etc.)
//
// InsertSalidaCaja registra que una caja fue enviada a una salida específica
// Parámetros:
//   - correlativo: Correlativo de la caja (ej: "10888")
//   - salidaID: ID de la salida física en la tabla salida (ej: 8)
//   - salidaRelativa: Número relativo de salida del sorter (1, 2, 3, etc.)
//   - llena: si la salida original estaba llena (true) o no (false)
func (m *PostgresManager) InsertSalidaCaja(ctx context.Context, correlativo string, salidaID int, salidaRelativa int, llena bool) error {
	if m == nil || m.pool == nil {
		return fmt.Errorf("manager no inicializado")
	}

	// Validaciones básicas
	if correlativo == "" {
		return fmt.Errorf("correlativo vacío")
	}
	if salidaID <= 0 {
		return fmt.Errorf("salidaID inválido: %d", salidaID)
	}
	if salidaRelativa <= 0 {
		return fmt.Errorf("salidaRelativa inválido: %d", salidaRelativa)
	}

	commandTag, err := m.pool.Exec(ctx, INSERT_SALIDA_CAJA_INTERNAL_DB, correlativo, salidaID, salidaRelativa, llena)
	if err != nil {
		return fmt.Errorf("error al insertar salida_caja: %w", err)
	}

	if commandTag.RowsAffected() == 0 {
		return fmt.Errorf("no se pudo insertar registro en salida_caja")
	}

	log.Printf("✅ [DB] Caja %s registrada en salida %d (relativa: %d)", correlativo, salidaID, salidaRelativa)
	return nil
}

// GetHistorialDesvios obtiene las últimas 100 lecturas/desvíos de un sorter
func (m *PostgresManager) GetHistorialDesvios(ctx context.Context, sorterID int) ([]map[string]interface{}, error) {
	if m == nil || m.pool == nil {
		return nil, fmt.Errorf("manager no inicializado")
	}

	rows, err := m.pool.Query(ctx, SELECT_HISTORIAL_DESVIOS_INTERNAL_DB, sorterID)
	if err != nil {
		return nil, fmt.Errorf("error al consultar historial: %w", err)
	}
	defer rows.Close()

	var historial []map[string]interface{}
	for rows.Next() {
		var boxID, sku, caliber string
		var sealer, sorterIDResult int
		var isFull bool
		var createdAt interface{}

		err := rows.Scan(&boxID, &sku, &caliber, &sealer, &isFull, &createdAt, &sorterIDResult)
		if err != nil {
			log.Printf("⚠️  Error al escanear fila: %v", err)
			continue
		}

		historial = append(historial, map[string]interface{}{
			"box_id":              boxID,
			"sku":                 sku,
			"caliber":             caliber,
			"sealer":              sealer,
			"is_sealer_full_type": isFull,
			"created_at":          createdAt,
			"sorter_id":           sorterIDResult,
		})
	}

	return historial, nil
}

// SetAllSKUsToFalse marca todas las SKUs como inactivas (estado = false)
// Se ejecuta ANTES de sincronizar para que solo las de la vista queden activas
func (m *PostgresManager) SetAllSKUsToFalse(ctx context.Context) error {
	if m == nil || m.pool == nil {
		return fmt.Errorf("manager no inicializado")
	}

	commandTag, err := m.pool.Exec(ctx, UPDATE_TO_FALSE_SKU_STATE_INTERNAL_DB)
	if err != nil {
		return fmt.Errorf("error al marcar SKUs como false: %w", err)
	}

	rowsAffected := commandTag.RowsAffected()
	log.Printf("🔄 [DB] %d SKUs marcadas como inactivas (estado=false)", rowsAffected)
	return nil
}

// UpsertSKU inserta o actualiza una SKU con estado = true
// Si existe (conflicto), actualiza estado a true
func (m *PostgresManager) UpsertSKU(ctx context.Context, calibre, variedad, embalaje string, estado bool) error {
	if m == nil || m.pool == nil {
		return fmt.Errorf("manager no inicializado")
	}

	// Validar que no sea nulo o vacío
	if isNullOrEmpty(calibre, variedad, embalaje) {
		return fmt.Errorf("SKU inválida: componentes nulos o vacíos")
	}

	commandTag, err := m.pool.Exec(ctx, INSERT_SKU_INTERNAL_DB,
		strings.TrimSpace(calibre),
		strings.TrimSpace(variedad),
		strings.TrimSpace(embalaje),
		estado)

	if err != nil {
		return fmt.Errorf("error al upsert SKU: %w", err)
	}

	if commandTag.RowsAffected() == 0 {
		return fmt.Errorf("no se insertó/actualizó la SKU")
	}

	return nil
}

// BeginTx inicia una transacción PostgreSQL
func (m *PostgresManager) BeginTx(ctx context.Context) (pgx.Tx, error) {
	if m == nil || m.pool == nil {
		return nil, fmt.Errorf("manager no inicializado")
	}
	return m.pool.Begin(ctx)
}
