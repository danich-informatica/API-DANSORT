package db

import (
	"api-dansort/internal/models"
	"context"
	"fmt"
	"log"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
)

type PostgresManager struct {
	pool      *pgxpool.Pool
	closeOnce sync.Once
}

func (m *PostgresManager) InsertSKUsBatch(ctx context.Context, skuList []struct {
	Calibre  string
	Variedad string
	Embalaje string
}) (any, any, any) {
	panic("unimplemented")
}

var (
	postgresOnce sync.Once
	postgresMgr  *PostgresManager
	postgresErr  error
)

// GetPostgresManagerWithURL crea un manager con una URL específica
func GetPostgresManagerWithURL(ctx context.Context, connURL string, minConns, maxConns int32, connectTimeout, healthCheckPeriod time.Duration) (*PostgresManager, error) {
	poolConfig, err := pgxpool.ParseConfig(connURL)
	if err != nil {
		return nil, fmt.Errorf("db: configuración PostgreSQL inválida: %w", err)
	}

	poolConfig.MinConns = minConns
	poolConfig.MaxConns = maxConns
	poolConfig.HealthCheckPeriod = healthCheckPeriod
	poolConfig.ConnConfig.ConnectTimeout = connectTimeout

	ctxTimeout := ctx
	if connectTimeout > 0 {
		var cancel context.CancelFunc
		ctxTimeout, cancel = context.WithTimeout(ctx, connectTimeout)
		defer cancel()
	}

	pool, err := pgxpool.NewWithConfig(ctxTimeout, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("db: no fue posible crear el pool de PostgreSQL: %w", err)
	}

	if err := pool.Ping(ctxTimeout); err != nil {
		pool.Close()
		return nil, fmt.Errorf("db: ping fallido: %w", err)
	}

	log.Printf("db: Postgres pool inicializado -> host=%s port=%d user=%s db=%s sslmode=%s",
		poolConfig.ConnConfig.Host, poolConfig.ConnConfig.Port, poolConfig.ConnConfig.User,
		visibleDatabase(poolConfig.ConnConfig.Database), poolConfig.ConnConfig.RuntimeParams["sslmode"])

	return &PostgresManager{pool: pool}, nil
}

func GetPostgresManager(ctx context.Context) (*PostgresManager, error) {
	postgresOnce.Do(func() {
		if err := godotenv.Load(); err != nil {
			log.Println("db: no se ha encontrado archivo .env, usando únicamente variables de entorno del sistema")
		}

		cfg, err := buildPostgresConfig()
		if err != nil {
			postgresErr = err
			return
		}

		poolConfig, err := pgxpool.ParseConfig(cfg.connString)
		if err != nil {
			postgresErr = fmt.Errorf("db: configuración PostgreSQL inválida: %w", err)
			return
		}

		poolConfig.MinConns = cfg.minConns
		poolConfig.MaxConns = cfg.maxConns
		poolConfig.HealthCheckPeriod = cfg.healthCheckPeriod
		poolConfig.ConnConfig.ConnectTimeout = cfg.connectTimeout

		ctxTimeout := ctx
		if cfg.connectTimeout > 0 {
			var cancel context.CancelFunc
			ctxTimeout, cancel = context.WithTimeout(ctx, cfg.connectTimeout)
			defer cancel()
		}

		pool, err := pgxpool.NewWithConfig(ctxTimeout, poolConfig)
		if err != nil {
			postgresErr = fmt.Errorf("db: no fue posible crear el pool de PostgreSQL: %w", err)
			return
		}

		if err := pool.Ping(ctxTimeout); err != nil {
			pool.Close()
			postgresErr = fmt.Errorf("db: ping fallido: %w", err)
			return
		}

		log.Printf("db: Postgres pool inicializado -> host=%s port=%d user=%s db=%s sslmode=%s", poolConfig.ConnConfig.Host, poolConfig.ConnConfig.Port, poolConfig.ConnConfig.User, visibleDatabase(poolConfig.ConnConfig.Database), poolConfig.ConnConfig.RuntimeParams["sslmode"])

		postgresMgr = &PostgresManager{pool: pool}
	})
	return postgresMgr, postgresErr
}

func (m *PostgresManager) Close() {
	if m == nil {
		return
	}

	m.closeOnce.Do(func() {
		if m.pool != nil {
			m.pool.Close()
		}
	})
}

func (m *PostgresManager) Pool() *pgxpool.Pool {
	return m.pool
}

func (m *PostgresManager) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	return m.pool.Exec(ctx, sql, args...)
}

func (m *PostgresManager) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return m.pool.Query(ctx, sql, args...)
}

func (m *PostgresManager) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return m.pool.QueryRow(ctx, sql, args...)
}

func (m *PostgresManager) RunQuery(ctx context.Context, query string, args ...any) ([]map[string]any, error) {
	rows, err := m.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("db: error ejecutando query: %w", err)
	}
	defer rows.Close()

	fieldDescs := rows.FieldDescriptions()
	columns := make([]string, len(fieldDescs))
	for i, fd := range fieldDescs {
		columns[i] = string(fd.Name)
	}

	var result []map[string]any
	for rows.Next() {
		values, err := rows.Values()
		if err != nil {
			return nil, fmt.Errorf("db: error obteniendo valores: %w", err)
		}

		row := make(map[string]any, len(values))
		for i, col := range columns {
			row[col] = values[i]
		}
		result = append(result, row)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("db: error iterando filas: %w", err)
	}

	return result, nil
}

func (m *PostgresManager) Ping(ctx context.Context) error {
	return m.pool.Ping(ctx)
}

// SyncSKUs sincroniza los SKUs con la base de datos PostgreSQL
func (m *PostgresManager) SyncSKUs(ctx context.Context, skus []models.SKU) (int, int, error) {
	tx, err := m.pool.Begin(ctx)
	if err != nil {
		return 0, 0, fmt.Errorf("error al iniciar transacción: %w", err)
	}
	defer tx.Rollback(ctx)

	insertedCount := 0
	updatedCount := 0

	for _, sku := range skus {
		// Intentar insertar el SKU
		_, err := tx.Exec(ctx,
			`INSERT INTO sku (calibre, variedad, embalaje, dark, estado, linea)
			 VALUES ($1, $2, $3, $4, $5, $6)
			 ON CONFLICT (calibre, variedad, embalaje, dark) DO UPDATE SET
			 estado = $5, linea = $6`,
			sku.Calibre, sku.Variedad, sku.Embalaje, sku.Dark, sku.Estado, sku.Linea,
		)

		if err != nil {
			// Comprobar si el error es por violación de unicidad para contar como actualización
			if pgErr, ok := err.(*pgconn.PgError); ok && pgErr.Code == "23505" {
				updatedCount++
			} else {
				log.Printf("Error al sincronizar SKU '%s': %v", sku.SKU, err)
				// Decidir si continuar o abortar
			}
		} else {
			insertedCount++
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, 0, fmt.Errorf("error al confirmar transacción: %w", err)
	}

	return insertedCount, updatedCount, nil
}

type pgConfig struct {
	connString        string
	connectTimeout    time.Duration
	healthCheckPeriod time.Duration
	minConns          int32
	maxConns          int32
}

func buildPostgresConfig() (*pgConfig, error) {
	dsn := os.Getenv("DANICH_PSQL_DB_URL")
	dsn = strings.Trim(dsn, "'\"")

	if dsn == "" {
		host := getEnv("GREENEX_PG_HOST", "localhost")
		port := getEnv("GREENEX_PG_PORT", "5432")
		user := getEnv("GREENEX_PG_USER", "postgres")
		password := os.Getenv("GREENEX_PG_PASSWORD")
		database := getEnv("GREENEX_PG_DATABASE", "postgres")
		sslMode := getEnv("GREENEX_PG_SSLMODE", "disable")
		appName := getEnv("GREENEX_PG_APP_NAME", "api-greenex")

		u := &url.URL{
			Scheme: "postgres",
			Host:   fmt.Sprintf("%s:%s", host, port),
			Path:   "/" + database,
		}

		if password != "" {
			u.User = url.UserPassword(user, password)
		} else {
			u.User = url.User(user)
		}

		q := u.Query()
		q.Set("sslmode", sslMode)
		if appName != "" {
			q.Set("application_name", appName)
		}
		u.RawQuery = q.Encode()

		dsn = u.String()
	}

	if dsn == "" {
		return nil, fmt.Errorf("db: DANICH_PSQL_DB_URL vacío")
	}

	cfg := &pgConfig{
		connString:        dsn,
		connectTimeout:    getDurationEnv("GREENEX_PG_CONNECT_TIMEOUT", "10s"),
		healthCheckPeriod: getDurationEnv("GREENEX_PG_HEALTHCHECK_INTERVAL", "30s"),
		minConns:          int32(getIntEnv("GREENEX_PG_MIN_CONNS", 1)),
		maxConns:          int32(getIntEnv("GREENEX_PG_MAX_CONNS", 10)),
	}

	return cfg, nil
}
