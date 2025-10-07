package db

import (
	"API-GREENEX/internal/models"
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/jackc/pgx/v5"
)

// InsertSKUsBatch inserta múltiples SKUs en la base de datos
// Excluye SKUs nulas, inválidas y duplicadas
// Actualiza el estado: pone todas las SKUs existentes en false y las nuevas/actualizadas en true
func (m *PostgresManager) InsertSKUsBatch(ctx context.Context, skus []struct {
	Calibre  string
	Variedad string
	Embalaje string
}) (inserted int, skipped int, errors []string) {
	if m == nil || m.pool == nil {
		return 0, 0, []string{"manager no inicializado"}
	}

	// Paso 1: Poner todas las SKUs existentes en estado false
	log.Println("Desactivando todas las SKUs existentes (estado = false)...")
	result, err := m.pool.Exec(ctx, UPDATE_TO_FALSE_SKU_STATE_INTERNAL_DB)
	if err != nil {
		errMsg := fmt.Sprintf("error al desactivar SKUs existentes: %v", err)
		errors = append(errors, errMsg)
		log.Println(errMsg)
		return 0, 0, errors
	}
	deactivated := result.RowsAffected()
	log.Printf("✓ %d SKUs desactivadas", deactivated)

	// Mapa para detectar duplicados en el batch
	seen := make(map[string]bool)

	for i, sku := range skus {
		// Validar que no sea nulo o vacío
		if isNullOrEmpty(sku.Calibre, sku.Variedad, sku.Embalaje) {
			skipped++
			log.Printf("fila %d: SKU omitida - componentes nulos o vacíos: %s-%s-%s",
				i+1, sku.Calibre, sku.Variedad, sku.Embalaje)
			continue
		}

		// Crear clave única para detectar duplicados
		key := fmt.Sprintf("%s|%s|%s",
			strings.TrimSpace(sku.Calibre),
			strings.TrimSpace(sku.Variedad),
			strings.TrimSpace(sku.Embalaje))

		// Verificar si ya se procesó en este batch
		if seen[key] {
			skipped++
			log.Printf("fila %d: SKU duplicada en batch: %s-%s-%s",
				i+1, sku.Calibre, sku.Variedad, sku.Embalaje)
			continue
		}

		seen[key] = true

		// Insertar o actualizar: si existe, actualiza estado a true; si no existe, inserta con estado true
		result, err := m.pool.Exec(ctx, INSERT_SKU_INTERNAL_DB,
			strings.TrimSpace(sku.Calibre),
			strings.TrimSpace(sku.Variedad),
			strings.TrimSpace(sku.Embalaje),
			true)

		if err != nil {
			errMsg := fmt.Sprintf("fila %d: error al insertar/actualizar SKU %s-%s-%s: %v",
				i+1, sku.Calibre, sku.Variedad, sku.Embalaje, err)
			errors = append(errors, errMsg)
			log.Println(errMsg)
			continue
		}

		// RowsAffected() == 1 para INSERT o UPDATE
		rowsAffected := result.RowsAffected()
		if rowsAffected > 0 {
			// SKU procesada exitosamente (insertada nueva o reactivada)
			inserted++
			log.Printf("fila %d: SKU procesada (insertada/reactivada): %s-%s-%s",
				i+1, sku.Calibre, sku.Variedad, sku.Embalaje)
		} else {
			skipped++
			log.Printf("fila %d: SKU no procesada: %s-%s-%s",
				i+1, sku.Calibre, sku.Variedad, sku.Embalaje)
		}
	}

	log.Printf("✓ Total SKUs procesadas: %d (insertadas nuevas + reactivadas)", inserted)
	return inserted, skipped, errors
}

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
