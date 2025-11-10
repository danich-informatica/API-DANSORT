package main

import (
	"context"
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

	"API-DANSORT/internal/config"
	"API-DANSORT/internal/db"

	"github.com/gin-gonic/gin"
)

// Código NO_READ constante
const NO_READ_CODE = "NO_READ"

// SKU representa una SKU de la base de datos
type SKU struct {
	Calibre  string
	Variedad string
	Embalaje string
	Dark     int
}

var (
	especies        = []string{"E003", "E001", "E005", "E007"}
	marcas          = []string{"CHLION", "GREENEX", "PREMIUM", "SELECT"}
	skusFromDB      []SKU
	skusLastUpdated time.Time
	skusMutex       sync.RWMutex
	dbManager       *db.PostgresManager
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

	// Modo burst/disparador
	isBurstMode   bool
	burstStopChan chan bool
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
	IsBurstMode   bool   `json:"is_burst_mode"`
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

// loadSKUsFromDB carga las SKUs activas desde la base de datos
func loadSKUsFromDB() error {
	ctx := context.Background()

	activeSKUs, err := dbManager.GetActiveSKUs(ctx)
	if err != nil {
		return fmt.Errorf("error al cargar SKUs: %w", err)
	}

	skusMutex.Lock()
	defer skusMutex.Unlock()

	skusFromDB = make([]SKU, 0, len(activeSKUs))
	for _, sku := range activeSKUs {
		skusFromDB = append(skusFromDB, SKU{
			Calibre:  sku.Calibre,
			Variedad: sku.Variedad,
			Embalaje: sku.Embalaje,
			Dark:     sku.Dark,
		})
	}
	skusLastUpdated = time.Now()

	log.Printf("✅ Cargadas %d SKUs activas desde la base de datos", len(skusFromDB))
	return nil
}

// startSKURefreshWorker actualiza las SKUs periódicamente
func startSKURefreshWorker() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		if err := loadSKUsFromDB(); err != nil {
			log.Printf("⚠️  Error al refrescar SKUs: %v", err)
		}
	}
}

// generarQR genera un código QR simulado con formato: ESPECIE;CALIBRE;DARK;EMBALAJE;MARCA;VARIEDAD
func generarQR() string {
	skusMutex.RLock()
	defer skusMutex.RUnlock()

	// Si no hay SKUs en la BD, usar valores por defecto
	if len(skusFromDB) == 0 {
		log.Printf("⚠️  No hay SKUs en la BD, usando valores por defecto")
		return "E003;XL;0;CECDCAM5;GREENEX;V018"
	}

	// Seleccionar SKU aleatoria de la BD
	sku := skusFromDB[rand.Intn(len(skusFromDB))]
	especie := especies[rand.Intn(len(especies))]
	marca := marcas[rand.Intn(len(marcas))]

	// Formato correcto: Especie;Calibre;Dark;Embalaje;Marca;Variedad
	return fmt.Sprintf("%s;%s;%d;%s;%s;%s",
		especie,
		sku.Calibre,
		sku.Dark,
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

	// Conectar al listener (siempre localhost porque el simulador corre en el mismo servidor que api-greenex)
	address := fmt.Sprintf("localhost:%d", cs.Port)
	log.Printf("📡 [Cognex #%d] Conectando a %s (cámara virtual: %s)...", cs.ID, address, cs.Host)

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

// SendBurst envía una ráfaga de N cajas con intervalo específico y luego pausa
func (cs *CognexSimulator) SendBurst(cantidad int, intervaloMs int, pausaSegundos int) error {
	cs.mu.Lock()
	if cs.isBurstMode {
		cs.mu.Unlock()
		return fmt.Errorf("ya hay una ráfaga en curso")
	}

	// Verificar que esté conectado Y corriendo
	if cs.conn == nil || !cs.IsRunning {
		cs.mu.Unlock()
		return fmt.Errorf("simulador no está activo - debe estar conectado y corriendo")
	}

	cs.isBurstMode = true
	cs.burstStopChan = make(chan bool)
	cs.mu.Unlock()

	log.Printf("🔫 [Cognex #%d] Iniciando RÁFAGA: %d cajas, intervalo=%dms, pausa=%ds",
		cs.ID, cantidad, intervaloMs, pausaSegundos)

	go func() {
		defer func() {
			cs.mu.Lock()
			cs.isBurstMode = false
			close(cs.burstStopChan)
			cs.mu.Unlock()
			log.Printf("✅ [Cognex #%d] Ráfaga completada", cs.ID)
		}()

		// Enviar N cajas
		for i := 0; i < cantidad; i++ {
			select {
			case <-cs.burstStopChan:
				log.Printf("⚠️ [Cognex #%d] Ráfaga cancelada en caja %d/%d", cs.ID, i+1, cantidad)
				return
			default:
				// Verificar que siga corriendo
				cs.mu.RLock()
				isRunning := cs.IsRunning
				noReadChance := cs.NoReadPercent
				cs.mu.RUnlock()

				if !isRunning {
					log.Printf("⚠️ [Cognex #%d] Ráfaga interrumpida: dispositivo detenido en caja %d/%d", cs.ID, i+1, cantidad)
					return
				}

				var codigo string
				if rand.Intn(100) < noReadChance {
					codigo = NO_READ_CODE
				} else {
					codigo = cs.generarCodigo()
				}

				mensaje := codigo + "\r\n"

				cs.mu.RLock()
				conn := cs.conn
				cs.mu.RUnlock()

				if conn == nil {
					log.Printf("⚠️ [Cognex #%d] Ráfaga interrumpida: conexión cerrada en caja %d/%d", cs.ID, i+1, cantidad)
					return
				}

				_, err := conn.Write([]byte(mensaje))
				if err != nil {
					log.Printf("❌ [Cognex #%d] Error al enviar en ráfaga: %v", cs.ID, err)
					return
				}

				log.Printf("📤 [Cognex #%d] Ráfaga %d/%d: %s", cs.ID, i+1, cantidad, codigo)

				// Esperar intervalo (excepto en la última caja)
				if i < cantidad-1 {
					time.Sleep(time.Duration(intervaloMs) * time.Millisecond)
				}
			}
		}

		// Pausa después de la ráfaga
		if pausaSegundos > 0 {
			log.Printf("⏸️  [Cognex #%d] Pausa de %d segundos después de ráfaga...", cs.ID, pausaSegundos)
			select {
			case <-cs.burstStopChan:
				return
			case <-time.After(time.Duration(pausaSegundos) * time.Second):
				log.Printf("▶️  [Cognex #%d] Pausa completada", cs.ID)
			}
		}
	}()

	return nil
}

// StopBurst detiene una ráfaga en curso
func (cs *CognexSimulator) StopBurst() error {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	if !cs.isBurstMode {
		return fmt.Errorf("no hay ráfaga en curso")
	}

	close(cs.burstStopChan)
	cs.isBurstMode = false

	log.Printf("🛑 [Cognex #%d] Ráfaga detenida manualmente", cs.ID)
	return nil
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
		IsBurstMode:   cs.isBurstMode,
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

func sendBurst(c *gin.Context) {
	var req struct {
		ID            int `json:"id" binding:"required"`
		Cantidad      int `json:"cantidad" binding:"required"`
		IntervaloMs   int `json:"intervalo_ms" binding:"required"`
		PausaSegundos int `json:"pausa_segundos"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validaciones
	if req.Cantidad <= 0 || req.Cantidad > 1000 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "cantidad debe estar entre 1 y 1000"})
		return
	}
	if req.IntervaloMs < 10 || req.IntervaloMs > 60000 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "intervalo_ms debe estar entre 10 y 60000"})
		return
	}
	if req.PausaSegundos < 0 || req.PausaSegundos > 300 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "pausa_segundos debe estar entre 0 y 300"})
		return
	}

	manager.mu.RLock()
	sim, exists := manager.simulators[req.ID]
	manager.mu.RUnlock()

	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Cognex no encontrado"})
		return
	}

	if err := sim.SendBurst(req.Cantidad, req.IntervaloMs, req.PausaSegundos); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": fmt.Sprintf("Ráfaga iniciada: %d cajas con intervalo de %dms", req.Cantidad, req.IntervaloMs),
		"status":  sim.GetStatus(),
	})
}

func stopBurst(c *gin.Context) {
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

	if err := sim.StopBurst(); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Ráfaga detenida",
		"status":  sim.GetStatus(),
	})
}

func main() {
	// No necesitamos rand.Seed() en Go 1.20+

	// Cargar configuración
	cfg, err := config.LoadConfig("config/config.yaml")
	if err != nil {
		log.Fatalf("❌ Error cargando configuración: %v", err)
	}

	// Inicializar gestor de base de datos PostgreSQL
	log.Println("🔌 Conectando a base de datos PostgreSQL...")
	dbManager, err = db.GetPostgresManagerWithURL(
		context.Background(),
		cfg.Database.Postgres.URL,
		1,  // minConns
		10, // maxConns
		10*time.Second,
		30*time.Second,
	)
	if err != nil {
		log.Fatalf("❌ Error al conectar con PostgreSQL: %v", err)
	}
	defer dbManager.Close()
	log.Println("✅ Conectado a PostgreSQL")

	// Cargar SKUs activas desde la base de datos
	log.Println("📦 Cargando SKUs activas desde la base de datos...")
	if err := loadSKUsFromDB(); err != nil {
		log.Printf("⚠️  No se pudieron cargar SKUs: %v (usando valores por defecto)", err)
	}

	// Iniciar worker para refrescar SKUs periódicamente
	go startSKURefreshWorker()

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
		api.POST("/burst", sendBurst)
		api.POST("/burst/stop", stopBurst)
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
