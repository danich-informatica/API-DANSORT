package db

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/url"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/joho/godotenv"
	_ "github.com/microsoft/go-mssqldb"
)

// Manager encapsula un pool de conexiones a SQL Server y expone helpers seguros
// para ejecutar consultas dentro del proyecto.
type Manager struct {
	db        *sql.DB
	closeOnce sync.Once
}

var (
	managerOnce     sync.Once
	managerInstance *Manager
	managerErr      error
)

// GetManager devuelve una instancia singleton del gestor de base de datos.
// La primera invocación crea el pool utilizando la configuración proveniente
// de variables de entorno.
func GetManager(ctx context.Context) (*Manager, error) {
	managerOnce.Do(func() {
		var mgr *Manager
		mgr, managerErr = newManager(ctx)
		if managerErr == nil {
			managerInstance = mgr
		}
	})

	if managerErr != nil {
		return nil, managerErr
	}
	return managerInstance, nil
}

// GetDB devuelve el *sql.DB subyacente para quien necesite acceso directo.
func GetDB(ctx context.Context) (*sql.DB, error) {
	mgr, err := GetManager(ctx)
	if err != nil {
		return nil, err
	}
	return mgr.db, nil
}

func newManager(ctx context.Context) (*Manager, error) {
	// Cargamos variables de entorno (no es grave si falla; puede que ya se hayan cargado).
	if err := godotenv.Load(); err != nil {
		log.Println("db: no se ha encontrado archivo .env, usando únicamente variables de entorno del sistema")
	}

	cfg, err := loadConfig()
	if err != nil {
		return nil, err
	}

	db, err := sql.Open("sqlserver", cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("db: no fue posible crear el pool de conexiones: %w", err)
	}

	if cfg.MaxOpenConns > 0 {
		db.SetMaxOpenConns(cfg.MaxOpenConns)
	}
	if cfg.MaxIdleConns >= 0 {
		db.SetMaxIdleConns(cfg.MaxIdleConns)
	}
	if cfg.ConnMaxLifetime > 0 {
		db.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	}
	if cfg.ConnMaxIdleTime > 0 {
		db.SetConnMaxIdleTime(cfg.ConnMaxIdleTime)
	}

	// Validar la conexión inmediatamente para fallar temprano si algo está mal.
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := db.PingContext(pingCtx); err != nil {
		db.Close()
		return nil, fmt.Errorf("db: no fue posible conectarse a SQL Server: %w", err)
	}

	log.Printf(
		"db: pool SQL Server inicializado (host=%s port=%s user=%s encrypt=%s database=%s)",
		cfg.Host, cfg.Port, cfg.User, cfg.Encrypt, visibleDatabase(cfg.Database),
	)

	return &Manager{db: db}, nil
}

// Close cierra el pool de conexiones de manera segura; es idempotente.
func (m *Manager) Close() {
	if m == nil {
		return
	}

	m.closeOnce.Do(func() {
		m.db.Close()
	})
}

// DB devuelve el manejador subyacente *sql.DB.
func (m *Manager) DB() *sql.DB {
	return m.db
}

// Exec ejecuta una sentencia SQL (INSERT/UPDATE/DELETE) y devuelve el resultado.
func (m *Manager) Exec(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return m.db.ExecContext(ctx, query, args...)
}

// Query ejecuta una consulta que devuelve filas múltiples.
func (m *Manager) Query(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return m.db.QueryContext(ctx, query, args...)
}

// QueryRow ejecuta una consulta que espera exactamente una fila de resultado.
func (m *Manager) QueryRow(ctx context.Context, query string, args ...any) *sql.Row {
	return m.db.QueryRowContext(ctx, query, args...)
}

// Ping permite verificar el estado del pool contra la base de datos.
func (m *Manager) Ping(ctx context.Context) error {
	return m.db.PingContext(ctx)
}

type sqlServerConfig struct {
	DSN             string
	Host            string
	Port            string
	User            string
	Database        string
	Encrypt         string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
}

func loadConfig() (*sqlServerConfig, error) {
	host := getEnv("GREENEX_SSMS_HOST", "localhost")
	port := getEnv("GREENEX_SSMS_PORT", "1433")
	user := getEnv("GREENEX_SSMS_DB_USER", "sa")
	password := os.Getenv("GREENEX_SSMS_DB_PASSWORD")
	database := os.Getenv("GREENEX_SSMS_DB_NAME")
	encrypt := getEnv("GREENEX_SSMS_DB_ENCRYPT", "disable")
	trustCert := getEnv("GREENEX_SSMS_DB_TRUST_CERT", "true")
	appName := getEnv("GREENEX_SSMS_APP_NAME", "API-Greenex")
	connectTimeout := getEnv("GREENEX_SSMS_CONNECT_TIMEOUT", "15")

	query := url.Values{}
	query.Set("encrypt", encrypt)
	query.Set("TrustServerCertificate", trustCert)
	query.Set("app name", appName)
	if database != "" {
		query.Set("database", database)
	}
	if connectTimeout != "" {
		query.Set("connection timeout", connectTimeout)
	}

	u := &url.URL{
		Scheme: "sqlserver",
		Host:   fmt.Sprintf("%s:%s", host, port),
	}
	if password != "" {
		u.User = url.UserPassword(user, password)
	} else {
		u.User = url.User(user)
	}
	u.RawQuery = query.Encode()

	cfg := &sqlServerConfig{
		DSN:             u.String(),
		Host:            host,
		Port:            port,
		User:            user,
		Database:        database,
		Encrypt:         encrypt,
		MaxOpenConns:    getIntEnv("DB_MAX_CONNS", 10),
		MaxIdleConns:    getIntEnv("DB_MIN_CONNS", 5),
		ConnMaxLifetime: getDurationEnv("DB_MAX_CONN_LIFETIME", "30m"),
		ConnMaxIdleTime: getDurationEnv("DB_MAX_CONN_IDLE_TIME", "5m"),
	}

	log.Printf(
		"db: configuración -> host=%s port=%s user=%s encrypt=%s database=%s",
		host, port, user, encrypt, visibleDatabase(database),
	)

	return cfg, nil
}

func visibleDatabase(name string) string {
	if name == "" {
		return "<predeterminada>"
	}
	return name
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func getIntEnv(key string, fallback int) int {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}

	value, err := strconv.Atoi(raw)
	if err != nil {
		log.Printf("db: valor inválido para %s='%s', usando %d", key, raw, fallback)
		return fallback
	}
	return value
}

func getDurationEnv(key, fallback string) time.Duration {
	raw := os.Getenv(key)
	if raw == "" {
		raw = fallback
	}

	value, err := time.ParseDuration(raw)
	if err != nil {
		log.Printf("db: duración inválida para %s='%s', usando %s", key, raw, fallback)
		value, _ = time.ParseDuration(fallback)
	}
	return value
}
