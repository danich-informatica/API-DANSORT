package db

import (
	"context"
	"database/sql"
	"fmt"
)

// QueryExecutor expone el subconjunto de métodos necesario para ejecutar consultas.
// Está pensado para ser satisfecho por *Manager o por un mock en pruebas.
type QueryExecutor interface {
	Query(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

// SegregazioneProgrammaRow representa una fila de la consulta SELECT_UNITEC_DB_DBO_SEGREGAZIONE_PROGRAMMA.
type SegregazioneProgrammaRow struct {
	Variedad sql.NullString
	Calibre  sql.NullString
	Embalaje sql.NullString
}

// FetchSegregazioneProgramma ejecuta la consulta definida en queries.go y devuelve todas las filas.
func FetchSegregazioneProgramma(ctx context.Context, executor QueryExecutor) ([]SegregazioneProgrammaRow, error) {
	rows, err := executor.Query(ctx, SELECT_UNITEC_DB_DBO_SEGREGAZIONE_PROGRAMMA)
	if err != nil {
		return nil, fmt.Errorf("db: no fue posible ejecutar la consulta de segregazione: %w", err)
	}
	defer rows.Close()

	var result []SegregazioneProgrammaRow
	for rows.Next() {
		var row SegregazioneProgrammaRow
		if err := rows.Scan(&row.Variedad, &row.Calibre, &row.Embalaje); err != nil {
			return nil, fmt.Errorf("db: error leyendo resultados de segregazione: %w", err)
		}
		result = append(result, row)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("db: error iterando resultados de segregazione: %w", err)
	}

	return result, nil
}
