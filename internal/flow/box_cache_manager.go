package flow

import (
	"context"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"API-DANSORT/internal/db"
)

// BoxData representa los datos de una caja cacheados en memoria
type BoxData struct {
	CodCaja  string
	Especie  string
	Calibre  string
	Variedad string
	Embalaje string
	Dark     int
}

// BoxCacheManager gestiona un caché en memoria de las últimas N cajas
// del proceso actual, con acceso directo O(1) por código de caja
type BoxCacheManager struct {
	ctx         context.Context
	cancel      context.CancelFunc
	ssmsManager *db.Manager

	// Caché indexado por codCaja para acceso O(1) directo
	boxes map[string]BoxData
	mu    sync.RWMutex

	// Configuración
	cacheSize  int           // Número de cajas a cachear (ej: 1000)
	interval   time.Duration // Intervalo de polling (ej: 2s)
	started    atomic.Bool   // Flag para evitar múltiples Start()
	refreshing atomic.Bool   // Flag para evitar refreshes simultáneos

	// Métricas
	hits   atomic.Int64 // Búsquedas exitosas en caché
	misses atomic.Int64 // Búsquedas fallidas (fallback a DB)

	// Observabilidad de tiempos
	lastRefreshDuration time.Duration // Tiempo del último refresh completo
	lastRefreshTime     time.Time     // Timestamp del último refresh
	mu2                 sync.RWMutex  // Mutex para métricas de tiempo
}

// NewBoxCacheManager crea una nueva instancia del gestor de caché de cajas
func NewBoxCacheManager(ctx context.Context, ssmsManager *db.Manager, cacheSize int, interval time.Duration) *BoxCacheManager {
	workerCtx, cancel := context.WithCancel(ctx)

	if cacheSize <= 0 {
		cacheSize = 500 // Default
		log.Printf("⚠️  [BoxCache] Tamaño de caché inválido, usando default: %d", cacheSize)
	}

	if interval <= 0 {
		interval = 5 * time.Second // Default
		log.Printf("⚠️  [BoxCache] Intervalo inválido, usando default: %v", interval)
	}

	return &BoxCacheManager{
		ctx:         workerCtx,
		cancel:      cancel,
		ssmsManager: ssmsManager,
		boxes:       make(map[string]BoxData, cacheSize),
		cacheSize:   cacheSize,
		interval:    interval,
	}
}

// Start inicia el worker de sincronización periódica
func (m *BoxCacheManager) Start() {
	// Protección contra múltiples Start()
	if !m.started.CompareAndSwap(false, true) {
		log.Printf("⚠️  [BoxCache] Start() ya fue llamado, ignorando llamada duplicada")
		return
	}

	log.Printf("🚀 [BoxCache] Iniciando con configuración:")
	log.Printf("   📦 Tamaño de caché: %d cajas (acceso O(1) directo por ID)", m.cacheSize)
	log.Printf("   ⏱️  Intervalo de polling: %v", m.interval)
	log.Printf("   🔄 Primera carga inmediata...")

	// Primera carga inmediata (bloqueante)
	if err := m.refresh(); err != nil {
		log.Printf("❌ [BoxCache] Error en carga inicial: %v", err)
		log.Printf("   ⚠️  El caché estará vacío hasta el próximo refresh exitoso")
	}

	// Iniciar goroutine de polling
	go m.run()
	log.Printf("✅ [BoxCache] Worker iniciado correctamente")
}

// Stop detiene el worker de forma segura
func (m *BoxCacheManager) Stop() {
	log.Printf("🛑 [BoxCache] Deteniendo worker...")
	m.cancel()
	log.Printf("✅ [BoxCache] Worker detenido")
}

// run ejecuta el loop de polling periódico
func (m *BoxCacheManager) run() {
	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			if err := m.refresh(); err != nil {
				log.Printf("❌ [BoxCache] Error en refresh periódico: %v", err)
			}
		}
	}
}

// refresh ejecuta la query y actualiza el caché atómicamente
func (m *BoxCacheManager) refresh() error {
	// Protección contra refreshes simultáneos
	if !m.refreshing.CompareAndSwap(false, true) {
		return nil // Ya hay un refresh en curso, skip
	}
	defer m.refreshing.Store(false)

	startTotal := time.Now()

	// VERIFICACIÓN: Asegurarse de que el manager de SSMS no sea nil
	if m.ssmsManager == nil {
		log.Printf("❌ [BoxCache] Error: ssmsManager es nil. El refresh no puede continuar.")
		return fmt.Errorf("ssmsManager no está inicializado")
	}

	// 1. Query a SQL Server
	ctx, cancel := context.WithTimeout(m.ctx, 10*time.Second)
	defer cancel()

	rows, err := m.ssmsManager.Query(ctx, db.SELECT_TOP_N_BOXES_FROM_CURRENT_PROCESO, m.cacheSize)
	if err != nil {
		log.Printf("❌ [BoxCache] Error en query: %v", err)
		return fmt.Errorf("error en query: %w", err)
	}
	defer rows.Close()

	// 2. Leer resultados y construir map indexado por codCaja
	newBoxes := make(map[string]BoxData, m.cacheSize)
	rowCount := 0
	for rows.Next() {
		var box BoxData
		var nombreVariedad string // No se usa en BoxData, pero viene en la query
		if err := rows.Scan(&box.Especie, &box.Variedad, &box.Calibre, &box.Embalaje, &box.Dark, &nombreVariedad, &box.CodCaja); err != nil {
			log.Printf("⚠️  [BoxCache] Error al escanear fila: %v", err)
			continue
		}
		newBoxes[box.CodCaja] = box
		rowCount++
	}

	if len(newBoxes) == 0 {
		log.Printf("⚠️  [BoxCache] Query retornó 0 filas")
		return nil
	}

	// 3. Swap atómico del caché
	m.mu.Lock()
	m.boxes = newBoxes
	m.mu.Unlock()

	// 4. Registrar métricas de tiempo
	totalDuration := time.Since(startTotal)
	m.mu2.Lock()
	m.lastRefreshDuration = totalDuration
	m.lastRefreshTime = time.Now()
	m.mu2.Unlock()

	log.Printf("✅ [BoxCache] Refresh OK: %d cajas en %.1fms", rowCount, totalDuration.Seconds()*1000)

	return nil
}

// GetBoxData busca una caja en el caché usando acceso directo O(1) al map
// Retorna los datos y un bool indicando si se encontró
func (m *BoxCacheManager) GetBoxData(codCaja string) (BoxData, bool) {
	m.mu.RLock()
	boxes := m.boxes
	m.mu.RUnlock()

	if len(boxes) == 0 {
		m.misses.Add(1)
		return BoxData{}, false
	}

	// Acceso directo O(1) al map por código de caja
	box, found := boxes[codCaja]

	if found {
		m.hits.Add(1)
		return box, true
	}

	// No encontrado
	m.misses.Add(1)

	return BoxData{}, false
}

// GetStats retorna las estadísticas del caché
func (m *BoxCacheManager) GetStats() map[string]interface{} {
	hits := m.hits.Load()
	misses := m.misses.Load()
	total := hits + misses

	var ratio float64
	if total > 0 {
		ratio = float64(hits) / float64(total) * 100
	}

	m.mu.RLock()
	cacheSize := len(m.boxes)
	m.mu.RUnlock()

	m.mu2.RLock()
	lastRefreshDuration := m.lastRefreshDuration
	lastRefreshTime := m.lastRefreshTime
	m.mu2.RUnlock()

	return map[string]interface{}{
		"hits":                 hits,
		"misses":               misses,
		"total_searches":       total,
		"hit_ratio_percent":    ratio,
		"cache_size_current":   cacheSize,
		"cache_size_max":       m.cacheSize,
		"last_refresh_ms":      lastRefreshDuration.Milliseconds(),
		"last_refresh_time":    lastRefreshTime.Format("2006-01-02 15:04:05"),
		"refresh_interval_sec": m.interval.Seconds(),
	}
}
