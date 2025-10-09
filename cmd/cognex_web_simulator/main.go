package main

import (
	"fmt"
	"log"
	"math/rand"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"API-GREENEX/internal/config"

	"github.com/gin-gonic/gin"
)

// Código NO_READ constante
const NO_READ_CODE = "NO_READ"

// Combinaciones válidas de SKUs
var skusCombinaciones = []struct {
	Calibre  string
	Variedad string
	Embalaje string
}{
	{"XL", "V018", "CECDCAM5"},
	{"4J", "V018", "CEMDSAM4"},
	{"2J", "V018", "CEMGKAM5"},
	{"2J", "V018", "CEMDSAM4"},
	{"XLD", "V018", "CECGKAM5"},
	{"2J", "V022", "CEMGKAM5"},
	{"3J", "V022", "CEMGKAM5"},
	{"J", "V022", "CEMGKAM5"},
	{"JD", "V022", "CEMGKAM5"},
}

var (
	especies = []string{"E003", "E001", "E005", "E007"}
	marcas   = []string{"CHLION", "GREENEX", "PREMIUM", "SELECT"}
)

// CognexSimulator simula un lector Cognex
type CognexSimulator struct {
	ID            int
	Name          string
	Host          string
	Port          int
	ScanMethod    string
	Ubicacion     string
	IsRunning     bool
	IntervalMs    int
	NoReadPercent int
	conn          net.Conn
	stopChan      chan bool
	mu            sync.RWMutex
}

// SimulatorManager gestiona todos los simuladores
type SimulatorManager struct {
	simulators map[int]*CognexSimulator
	mu         sync.RWMutex
}

var manager *SimulatorManager

// CognexStatus representa el estado de un Cognex para la API
type CognexStatus struct {
	ID            int    `json:"id"`
	Name          string `json:"name"`
	Host          string `json:"host"`
	Port          int    `json:"port"`
	Ubicacion     string `json:"ubicacion"`
	IsRunning     bool   `json:"is_running"`
	IntervalMs    int    `json:"interval_ms"`
	NoReadPercent int    `json:"no_read_percent"`
}

// UpdateConfig actualiza configuración de un simulador
type UpdateConfig struct {
	IntervalMs    int `json:"interval_ms"`
	NoReadPercent int `json:"no_read_percent"`
}

func init() {
	manager = &SimulatorManager{
		simulators: make(map[int]*CognexSimulator),
	}
}

// generarQR genera un código QR simulado con formato: ESPECIE;CALIBRE;INVERTIR;EMBALAJE;MARCA;VARIEDAD
func generarQR() string {
	// Seleccionar combinación aleatoria
	sku := skusCombinaciones[rand.Intn(len(skusCombinaciones))]
	especie := especies[rand.Intn(len(especies))]
	marca := marcas[rand.Intn(len(marcas))]
	invertirColores := "0" // Puede ser "0" o "1"

	// Formato correcto: E003;XL;0;CECDCAM5;PREMIUM;V018
	return fmt.Sprintf("%s;%s;%s;%s;%s;%s",
		especie,
		sku.Calibre,
		invertirColores,
		sku.Embalaje,
		marca,
		sku.Variedad,
	)
}

// generarDataMatrix genera un ID de 10 dígitos aleatorio para DATAMATRIX
func generarDataMatrix() string {
	// Generar ID aleatorio de 10 dígitos
	return fmt.Sprintf("%010d", rand.Intn(10000000000))
}

// generarCodigo genera el código según el método de escaneo
func (cs *CognexSimulator) generarCodigo() string {
	if cs.ScanMethod == "DATAMATRIX" {
		return generarDataMatrix()
	}
	return generarQR()
}

// Start inicia el simulador
func (cs *CognexSimulator) Start() error {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	if cs.IsRunning {
		return fmt.Errorf("simulador ya está corriendo")
	}

	// Conectar al listener
	address := fmt.Sprintf("%s:%d", cs.Host, cs.Port)
	log.Printf("📡 [Cognex #%d] Conectando a %s...", cs.ID, address)

	conn, err := net.Dial("tcp", address)
	if err != nil {
		return fmt.Errorf("error al conectar: %v", err)
	}

	cs.conn = conn
	cs.IsRunning = true
	cs.stopChan = make(chan bool)

	// Iniciar goroutine de envío
	go cs.sendLoop()

	log.Printf("✅ [Cognex #%d] %s iniciado (intervalo: %dms, NO_READ: %d%%)",
		cs.ID, cs.Name, cs.IntervalMs, cs.NoReadPercent)

	return nil
}

// Stop detiene el simulador
func (cs *CognexSimulator) Stop() error {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	if !cs.IsRunning {
		return fmt.Errorf("simulador no está corriendo")
	}

	close(cs.stopChan)
	if cs.conn != nil {
		cs.conn.Close()
	}
	cs.IsRunning = false

	log.Printf("🛑 [Cognex #%d] %s detenido", cs.ID, cs.Name)
	return nil
}

// sendLoop envía códigos QR en loop
func (cs *CognexSimulator) sendLoop() {
	ticker := time.NewTicker(time.Duration(cs.IntervalMs) * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-cs.stopChan:
			return
		case <-ticker.C:
			cs.mu.RLock()
			noReadChance := cs.NoReadPercent
			cs.mu.RUnlock()

			var codigo string
			if rand.Intn(100) < noReadChance {
				codigo = NO_READ_CODE
			} else {
				codigo = cs.generarCodigo()
			}

			mensaje := codigo + "\r\n"
			_, err := cs.conn.Write([]byte(mensaje))
			if err != nil {
				log.Printf("❌ [Cognex #%d] Error al enviar: %v", cs.ID, err)
				cs.mu.Lock()
				cs.IsRunning = false
				cs.mu.Unlock()
				return
			}

			log.Printf("📤 [Cognex #%d] Enviado: %s", cs.ID, codigo)
		}
	}
}

// UpdateConfig actualiza la configuración sin detener
func (cs *CognexSimulator) UpdateConfig(intervalMs, noReadPercent int) {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	cs.IntervalMs = intervalMs
	cs.NoReadPercent = noReadPercent

	log.Printf("⚙️ [Cognex #%d] Configuración actualizada: intervalo=%dms, NO_READ=%d%%",
		cs.ID, intervalMs, noReadPercent)
}

// GetStatus retorna el estado actual
func (cs *CognexSimulator) GetStatus() CognexStatus {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	return CognexStatus{
		ID:            cs.ID,
		Name:          cs.Name,
		Host:          cs.Host,
		Port:          cs.Port,
		Ubicacion:     cs.Ubicacion,
		IsRunning:     cs.IsRunning,
		IntervalMs:    cs.IntervalMs,
		NoReadPercent: cs.NoReadPercent,
	}
}

// API Handlers

func getAllStatus(c *gin.Context) {
	manager.mu.RLock()
	defer manager.mu.RUnlock()

	statuses := make([]CognexStatus, 0, len(manager.simulators))
	for _, sim := range manager.simulators {
		statuses = append(statuses, sim.GetStatus())
	}

	c.JSON(http.StatusOK, gin.H{
		"cognex_devices": statuses,
	})
}

func startCognex(c *gin.Context) {
	var req struct {
		ID int `json:"id" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	manager.mu.RLock()
	sim, exists := manager.simulators[req.ID]
	manager.mu.RUnlock()

	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Cognex no encontrado"})
		return
	}

	if err := sim.Start(); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Cognex iniciado",
		"status":  sim.GetStatus(),
	})
}

func stopCognex(c *gin.Context) {
	var req struct {
		ID int `json:"id" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	manager.mu.RLock()
	sim, exists := manager.simulators[req.ID]
	manager.mu.RUnlock()

	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Cognex no encontrado"})
		return
	}

	if err := sim.Stop(); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Cognex detenido",
		"status":  sim.GetStatus(),
	})
}

func updateConfig(c *gin.Context) {
	var req struct {
		ID            int `json:"id" binding:"required"`
		IntervalMs    int `json:"interval_ms" binding:"required"`
		NoReadPercent int `json:"no_read_percent" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	manager.mu.RLock()
	sim, exists := manager.simulators[req.ID]
	manager.mu.RUnlock()

	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Cognex no encontrado"})
		return
	}

	sim.UpdateConfig(req.IntervalMs, req.NoReadPercent)

	c.JSON(http.StatusOK, gin.H{
		"message": "Configuración actualizada",
		"status":  sim.GetStatus(),
	})
}

func main() {
	rand.Seed(time.Now().UnixNano())

	// Cargar configuración
	cfg, err := config.LoadConfig("config/config.yaml")
	if err != nil {
		log.Fatalf("❌ Error cargando configuración: %v", err)
	}

	// Crear simuladores desde configuración
	log.Println("🔧 Inicializando simuladores Cognex...")
	for _, cognexCfg := range cfg.CognexDevices {
		// Valores por defecto si no están en config
		intervalMs := cognexCfg.IntervalMs
		if intervalMs <= 0 {
			intervalMs = 1000 // Default: 1 segundo
		}
		noReadPercent := cognexCfg.NoReadPercent
		if noReadPercent < 0 || noReadPercent > 100 {
			noReadPercent = 10 // Default: 10% de NO_READ
		}

		sim := &CognexSimulator{
			ID:            cognexCfg.ID,
			Name:          cognexCfg.Name,
			Host:          cognexCfg.Host,
			Port:          cognexCfg.Port,
			ScanMethod:    cognexCfg.ScanMethod,
			Ubicacion:     cognexCfg.Ubicacion,
			IsRunning:     false,
			IntervalMs:    intervalMs,
			NoReadPercent: noReadPercent,
		}
		manager.simulators[sim.ID] = sim
		log.Printf("  ↳ Cognex #%d: %s (%s:%d) - Intervalo: %dms, NO_READ: %d%%",
			sim.ID, sim.Name, sim.Host, sim.Port, intervalMs, noReadPercent)
	}

	// Configurar servidor web
	gin.SetMode(gin.ReleaseMode)
	router := gin.Default()

	// CORS simple
	router.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	})

	// API endpoints
	api := router.Group("/api")
	{
		api.GET("/status", getAllStatus)
		api.POST("/start", startCognex)
		api.POST("/stop", stopCognex)
		api.POST("/config", updateConfig)
	}

	// Servir interfaz web
	router.StaticFile("/", "./web/simulator.html")
	router.Static("/static", "./web/static")

	// Iniciar servidor
	port := 9091
	log.Printf("🌐 Servidor web iniciado en http://0.0.0.0:%d", port)
	log.Println("📋 Simuladores disponibles:")
	for _, sim := range manager.simulators {
		log.Printf("   • %s (ID: %d) - %s:%d", sim.Name, sim.ID, sim.Host, sim.Port)
	}

	// Manejar señales de interrupción
	go func() {
		if err := router.Run(fmt.Sprintf("0.0.0.0:%d", port)); err != nil {
			log.Fatalf("❌ Error iniciando servidor: %v", err)
		}
	}()

	// Esperar señal de interrupción
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("\n🛑 Deteniendo todos los simuladores...")
	manager.mu.RLock()
	for _, sim := range manager.simulators {
		if sim.IsRunning {
			sim.Stop()
		}
	}
	manager.mu.RUnlock()

	log.Println("👋 ¡Adiós!")
}
