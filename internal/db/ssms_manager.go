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

	"api-dansort/internal/config"
	"api-dansort/internal/models"

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

// GetManagerWithConfig devuelve una instancia del gestor usando configuración YAML
// No usa singleton, crea una nueva instancia cada vez
func GetManagerWithConfig(ctx context.Context, cfg config.SQLServerConfig) (*Manager, error) {
	return newManagerWithConfig(ctx, cfg, "")
}

// GetManagerWithConfigAndLabel crea un manager con label personalizada para logs
func GetManagerWithConfigAndLabel(ctx context.Context, cfg config.SQLServerConfig, label string) (*Manager, error) {
	return newManagerWithConfig(ctx, cfg, label)
}

// newManagerWithConfig es la función interna que crea el manager
func newManagerWithConfig(ctx context.Context, cfg config.SQLServerConfig, label string) (*Manager, error) {
	// Construir URL con encoding apropiado para caracteres especiales
	query := url.Values{}
	if cfg.Database != "" {
		query.Add("database", cfg.Database)
	}
	query.Add("encrypt", cfg.Encrypt)
	query.Add("TrustServerCertificate", fmt.Sprintf("%t", cfg.TrustCert))

	appName := cfg.AppName
	if label != "" {
		appName = fmt.Sprintf("%s-%s", cfg.AppName, label)
	}
	query.Add("app name", appName)
	query.Add("connection timeout", fmt.Sprintf("%d", cfg.ConnectTimeout))

	u := &url.URL{
		Scheme:   "sqlserver",
		User:     url.UserPassword(cfg.User, cfg.Password),
		Host:     fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		RawQuery: query.Encode(),
	}

	connString := u.String()

	db, err := sql.Open("sqlserver", connString)
	if err != nil {
		return nil, fmt.Errorf("db: no fue posible crear el pool de conexiones: %w", err)
	}

	// Configurar pool
	db.SetMaxOpenConns(cfg.MaxConns)
	db.SetMaxIdleConns(cfg.MinConns)

	if cfg.MaxConnLifetime != "" {
		duration, _ := time.ParseDuration(cfg.MaxConnLifetime)
		db.SetConnMaxLifetime(duration)
	}
	if cfg.MaxConnIdleTime != "" {
		duration, _ := time.ParseDuration(cfg.MaxConnIdleTime)
		db.SetConnMaxIdleTime(duration)
	}

	// Validar conexión
	pingCtx, cancel := context.WithTimeout(ctx, time.Duration(cfg.ConnectTimeout)*time.Second)
	defer cancel()

	if err := db.PingContext(pingCtx); err != nil {
		db.Close()
		return nil, fmt.Errorf("db: no fue posible conectarse a SQL Server: %w", err)
	}

	logLabel := label
	if logLabel == "" {
		logLabel = "SQL Server"
	}

	log.Printf(
		"✅ %s inicializado (host=%s:%d user=%s database=%s)",
		logLabel, cfg.Host, cfg.Port, cfg.User, visibleDatabase(cfg.Database),
	)

	return &Manager{db: db}, nil
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

// Exec ejecuta una sentencia SQL (INSERT/UPDATE/DELETE) y devuelve el resultado.
func (m *Manager) Exec(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return m.db.ExecContext(ctx, query, args...)
}

// QueryContext ejecuta una consulta que devuelve filas, típicamente un SELECT.
func (m *Manager) QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	return m.db.QueryContext(ctx, query, args...)
}

// Query ejecuta una consulta que devuelve filas múltiples.
func (m *Manager) Query(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return m.db.QueryContext(ctx, query, args...)
}

// QueryRow ejecuta una consulta que espera exactamente una fila de resultado.
func (m *Manager) QueryRow(ctx context.Context, query string, args ...any) *sql.Row {
	return m.db.QueryRowContext(ctx, query, args...)
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

// GetSKUsFromView obtiene los SKUs desde la vista de UNITEC DB.
func (m *Manager) GetSKUsFromView(ctx context.Context) ([]models.SKU, error) {
	rows, err := m.db.QueryContext(ctx, SELECT_UNITEC_DB_DBO_SEGREGAZIONE_PROGRAMMA)
	if err != nil {
		// Fallback a la query sin VIE_Dark
		log.Printf("⚠️  Error con la query principal de SKUs, intentando fallback: %v", err)
		rows, err = m.db.QueryContext(ctx, SELECT_UNITEC_DB_DBO_SEGREGAZIONE_PROGRAMMA_FALLBACK)
		if err != nil {
			return nil, fmt.Errorf("error al consultar SKUs desde UNITEC (incluso con fallback): %w", err)
		}
	}
	defer rows.Close()

	var skus []models.SKU
	for rows.Next() {
		var sku models.SKU
		var nombreVariedad sql.NullString
		if err := rows.Scan(&sku.Calibre, &sku.Variedad, &sku.Embalaje, &sku.Dark, &nombreVariedad, &sku.Linea); err != nil {
			log.Printf("⚠️  Error al escanear fila de SKU: %v", err)
			continue
		}
		sku.Estado = true // Marcar como activo por defecto
		skus = append(skus, sku)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error durante la iteración de filas de SKU: %w", err)
	}

	log.Printf("✅ %d SKUs obtenidos desde UNITEC", len(skus))
	return skus, nil
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
