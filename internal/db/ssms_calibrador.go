package db

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"api-dansort/internal/config"
)

// QueryExecutor expone el subconjunto mínimo necesario para ejecutar la consulta
// de segregación; lo satisface el manager de SQL Server y facilita el mockeo en pruebas.
type QueryExecutor interface {
	Query(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

// SegregazioneProgrammaRow representa una fila de la consulta SELECT_UNITEC_DB_DBO_SEGREGAZIONE_PROGRAMMA.
type SegregazioneProgrammaRow struct {
	Variedad       sql.NullString
	Calibre        sql.NullString
	Embalaje       sql.NullString
	Dark           sql.NullInt64  // 0 o 1, default 0 si no existe la columna
	NombreVariedad sql.NullString // VIE_Descrizione (ej: "LAPINS")
	Linea          sql.NullString // VIE_codLinea (código de línea)
}

// FetchSegregazioneProgramma ejecuta la consulta definida en queries.go y devuelve todas las filas.
// Intenta primero con VIE_Dark, si falla usa query sin esa columna (para compatibilidad).
func FetchSegregazioneProgramma(ctx context.Context, executor QueryExecutor) ([]SegregazioneProgrammaRow, error) {
	// Intentar primero con VIE_Dark
	rows, err := executor.Query(ctx, SELECT_UNITEC_DB_DBO_SEGREGAZIONE_PROGRAMMA)
	if err != nil {
		// Si falla (probablemente columna no existe), intentar sin VIE_Dark
		rows, err = executor.Query(ctx, SELECT_UNITEC_DB_DBO_SEGREGAZIONE_PROGRAMMA_FALLBACK)
		if err != nil {
			return nil, fmt.Errorf("db: no fue posible ejecutar la consulta de segregazione: %w", err)
		}
	}
	defer rows.Close()

	var result []SegregazioneProgrammaRow
	for rows.Next() {
		var row SegregazioneProgrammaRow
		if err := rows.Scan(&row.Variedad, &row.Calibre, &row.Embalaje, &row.Dark, &row.NombreVariedad, &row.Linea); err != nil {
			return nil, fmt.Errorf("db: error leyendo resultados de segregazione: %w", err)
		}
		result = append(result, row)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("db: error iterando resultados de segregazione: %w", err)
	}

	return result, nil
}

// ============================================================================
// FX6Manager - Gestiona conexión específica a FX6_packing_Garate_Operaciones
// ============================================================================

// FX6Manager gestiona la conexión específica a FX6_packing_Garate_Operaciones
// Envuelve el Manager genérico con métodos específicos del dominio FX6
type FX6Manager struct {
	*Manager // Composición: hereda todos los métodos del Manager genérico
}

// NewFX6Manager crea una nueva instancia del gestor para FX6
func NewFX6Manager(ctx context.Context, cfg config.SQLServerConfig) (*FX6Manager, error) {
	// Crear una copia del config y forzar la base de datos correcta
	fx6Config := cfg
	fx6Config.Database = "FX6_packing_Garate_Operaciones"

	// Usar el Manager genérico con label personalizada
	manager, err := GetManagerWithConfigAndLabel(ctx, fx6Config, "FX6")
	if err != nil {
		return nil, fmt.Errorf("error al crear FX6Manager: %w", err)
	}

	return &FX6Manager{Manager: manager}, nil
}

// GetFX6Manager crea FX6Manager desde el config completo
func GetFX6Manager(ctx context.Context, cfg *config.Config) (*FX6Manager, error) {
	return NewFX6Manager(ctx, cfg.Database.SQLServer)
}

// InsertLecturaDataMatrix registra una lectura de DataMatrix en FX6
// Método específico del dominio FX6
func (m *FX6Manager) InsertLecturaDataMatrix(ctx context.Context, salida int, correlativo int64, numeroCaja int64, fechaLectura time.Time) error {
	if m == nil || m.Manager == nil || m.Manager.db == nil {
		return fmt.Errorf("FX6Manager no inicializado")
	}

	_, err := m.Manager.Exec(ctx, INSERT_LECTURA_DATAMATRIX_SSMS,
		sql.Named("p1", salida),
		sql.Named("p2", correlativo),
		sql.Named("p3", numeroCaja),
		sql.Named("p4", fechaLectura),
	)

	if err != nil {
		return fmt.Errorf("error al insertar lectura DataMatrix en FX6: %w", err)
	}

	return nil
}

// GetLecturasRecientes obtiene las últimas N lecturas de una salida específica
func (m *FX6Manager) GetLecturasRecientes(ctx context.Context, salida int, limit int) ([]LecturaDataMatrix, error) {
	if m == nil || m.Manager == nil {
		return nil, fmt.Errorf("FX6Manager no inicializado")
	}

	query := `
		SELECT TOP (@p1)
			Salida,
			Correlativo,
			Numero_Caja,
			Fecha_Lectura,
			Terminado
		FROM PKG_Pallets_Externos
		WHERE Salida = @p2
		ORDER BY Fecha_Lectura DESC
	`

	rows, err := m.Manager.Query(ctx, query, sql.Named("p1", limit), sql.Named("p2", salida))
	if err != nil {
		return nil, fmt.Errorf("error al consultar lecturas recientes: %w", err)
	}
	defer rows.Close()

	var lecturas []LecturaDataMatrix
	for rows.Next() {
		var l LecturaDataMatrix
		if err := rows.Scan(&l.Salida, &l.Correlativo, &l.NumeroCaja, &l.FechaLectura, &l.Terminado); err != nil {
			return nil, fmt.Errorf("error al escanear lectura: %w", err)
		}
		lecturas = append(lecturas, l)
	}

	return lecturas, nil
}

// GetContadorActual obtiene el último número de caja registrado para una salida
// Útil para reinicializar el contador en memoria
func (m *FX6Manager) GetContadorActual(ctx context.Context, salida int) (int, error) {
	if m == nil || m.Manager == nil {
		return 0, fmt.Errorf("FX6Manager no inicializado")
	}

	query := `
		SELECT ISNULL(MAX(Numero_Caja), 0)
		FROM PKG_Pallets_Externos
		WHERE Salida = @p1
	`

	var maxCaja int
	err := m.Manager.QueryRow(ctx, query, sql.Named("p1", salida)).Scan(&maxCaja)
	if err != nil {
		return 0, fmt.Errorf("error al obtener contador actual: %w", err)
	}

	return maxCaja, nil
}

// LecturaDataMatrix representa una lectura de DataMatrix
type LecturaDataMatrix struct {
	Salida       int
	Correlativo  int64
	NumeroCaja   int
	FechaLectura time.Time
	Terminado    int
}
