package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Database      DatabaseConfig   `yaml:"database"`
	HTTP          HTTPConfig       `yaml:"http"`
	Statistics    StatisticsConfig `yaml:"statistics"`
	CognexDevices []CognexDevice   `yaml:"cognex_devices"`
	Sorters       []Sorter         `yaml:"sorters"`
}

type StatisticsConfig struct {
	FlowCalculationInterval string `yaml:"flow_calculation_interval"` // ej: "10s"
	FlowWindowDuration      string `yaml:"flow_window_duration"`      // ej: "60s"
}

// GetFlowCalculationInterval retorna la duración del intervalo de cálculo
func (s *StatisticsConfig) GetFlowCalculationInterval() time.Duration {
	duration, err := time.ParseDuration(s.FlowCalculationInterval)
	if err != nil {
		return 10 * time.Second // default
	}
	return duration
}

// GetFlowWindowDuration retorna la duración de la ventana de tiempo
func (s *StatisticsConfig) GetFlowWindowDuration() time.Duration {
	duration, err := time.ParseDuration(s.FlowWindowDuration)
	if err != nil {
		return 60 * time.Second // default
	}
	return duration
}

type DatabaseConfig struct {
	Postgres  PostgresConfig  `yaml:"postgres"`
	SQLServer SQLServerConfig `yaml:"sqlserver"`
}

type PostgresConfig struct {
	URL                 string `yaml:"url"`
	MinConns            int    `yaml:"min_conns"`
	MaxConns            int    `yaml:"max_conns"`
	ConnectTimeout      string `yaml:"connect_timeout"`
	HealthcheckInterval string `yaml:"healthcheck_interval"`
}

type SQLServerConfig struct {
	Host            string `yaml:"host"`
	Port            int    `yaml:"port"`
	User            string `yaml:"user"`
	Password        string `yaml:"password"`
	Database        string `yaml:"database"`
	Encrypt         string `yaml:"encrypt"`
	TrustCert       bool   `yaml:"trust_cert"`
	AppName         string `yaml:"app_name"`
	ConnectTimeout  int    `yaml:"connect_timeout"`
	MaxConns        int    `yaml:"max_conns"`
	MinConns        int    `yaml:"min_conns"`
	MaxConnLifetime string `yaml:"max_conn_lifetime"`
	MaxConnIdleTime string `yaml:"max_conn_idle_time"`
}

type OPCUAConfig struct {
	Endpoint             string `yaml:"endpoint"`
	Username             string `yaml:"username"`
	Password             string `yaml:"password"`
	SecurityPolicy       string `yaml:"security_policy"`
	SecurityMode         string `yaml:"security_mode"`
	CertificatePath      string `yaml:"certificate_path"`
	PrivateKeyPath       string `yaml:"private_key_path"`
	ConnectionTimeout    string `yaml:"connection_timeout"`
	SessionTimeout       string `yaml:"session_timeout"`
	SubscriptionInterval string `yaml:"subscription_interval"`
	KeepaliveCount       uint32 `yaml:"keepalive_count"`
	LifetimeCount        uint32 `yaml:"lifetime_count"`
	MaxNotifications     uint32 `yaml:"max_notifications"`
}

type HTTPConfig struct {
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
}

type CognexDevice struct {
	ID            int    `yaml:"id"`
	Name          string `yaml:"name"`
	Host          string `yaml:"host"`            // Host remoto informativo (de dónde viene)
	Port          int    `yaml:"port"`            // Puerto en el que escuchará (siempre 0.0.0.0)
	ScanMethod    string `yaml:"scan_method"`     // Método de escaneo por defecto: "QR" o "DATAMATRIX"
	Ubicacion     string `yaml:"ubicacion"`       // Ubicación física del dispositivo
	IntervalMs    int    `yaml:"interval_ms"`     // Intervalo entre lecturas (milisegundos) para simulador
	NoReadPercent int    `yaml:"no_read_percent"` // Porcentaje de lecturas NO_READ para simulador
}

type Sorter struct {
	ID            int             `yaml:"id"`
	Name          string          `yaml:"name"`
	PLCEndpoint   string          `yaml:"plc_endpoint"` // Endpoint OPC UA (ej: "opc.tcp://192.168.120.100:4840")
	PLC           SorterPLCConfig `yaml:"plc"`
	Salidas       []Salida        `yaml:"salidas"`
	DefaultSalida int             `yaml:"default_salida"`
}

type SorterPLCConfig struct {
	InputNodeID  string `yaml:"input_node_id"`
	OutputNodeID string `yaml:"output_node_id"`
	ObjectID     string `yaml:"object_id"`
	MethodID     string `yaml:"method_id"`
}

type Salida struct {
	ID         int             `yaml:"id"`
	Nombre     string          `yaml:"nombre"`
	Tipo       string          `yaml:"tipo"`        // "manual", "automatico", "automatica", "descarte"
	PhysicalID int             `yaml:"physical_id"` // ID físico relativo del sorter (1, 2, 3, etc.)
	PLC        SalidaPLCConfig `yaml:"plc"`
}

type SalidaPLCConfig struct {
	EstadoNodeID  string `yaml:"estado_node_id"`  // Nodo OPC UA para leer/escribir estado numérico
	BloqueoNodeID string `yaml:"bloqueo_node_id"` // Nodo OPC UA para leer/escribir bloqueo (opcional, "" = no tiene)
}

// LoadConfig carga la configuración desde el archivo YAML
func LoadConfig(configPath string) (*Config, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("error leyendo archivo de configuración: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("error parseando YAML: %w", err)
	}

	return &config, nil
}

// Métodos helper para conversión de tipos
func (p PostgresConfig) GetConnectTimeoutDuration() (time.Duration, error) {
	return time.ParseDuration(p.ConnectTimeout)
}

func (p PostgresConfig) GetHealthcheckIntervalDuration() (time.Duration, error) {
	return time.ParseDuration(p.HealthcheckInterval)
}

func (o OPCUAConfig) GetConnectionTimeoutDuration() (time.Duration, error) {
	return time.ParseDuration(o.ConnectionTimeout)
}

func (o OPCUAConfig) GetSessionTimeoutDuration() (time.Duration, error) {
	return time.ParseDuration(o.SessionTimeout)
}

func (o OPCUAConfig) GetSubscriptionIntervalDuration() (time.Duration, error) {
	return time.ParseDuration(o.SubscriptionInterval)
}

func (s SQLServerConfig) GetMaxConnLifetimeDuration() (time.Duration, error) {
	return time.ParseDuration(s.MaxConnLifetime)
}

func (s SQLServerConfig) GetMaxConnIdleTimeDuration() (time.Duration, error) {
	return time.ParseDuration(s.MaxConnIdleTime)
}
