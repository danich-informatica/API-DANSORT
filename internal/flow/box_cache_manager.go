package flow

import (
	"context"
	"fmt"
	"log"
	"sort"
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
// del proceso actual, con búsqueda binaria O(log n) para máxima velocidad
type BoxCacheManager struct {
	ctx         context.Context
	cancel      context.CancelFunc
	ssmsManager *db.Manager

	// Caché ordenado por codCaja DESC para búsqueda binaria
	boxes []BoxData
	mu    sync.RWMutex

	// Configuración
	cacheSize int           // Número de cajas a cachear (ej: 500)
	interval  time.Duration // Intervalo de polling (ej: 5s)

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
		boxes:       make([]BoxData, 0, cacheSize),
		cacheSize:   cacheSize,
		interval:    interval,
	}
}

// Start inicia el worker de sincronización periódica
func (m *BoxCacheManager) Start() {
	log.Printf("🚀 [BoxCache] Iniciando con configuración:")
	log.Printf("   📦 Tamaño de caché: %d cajas", m.cacheSize)
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
	startTotal := time.Now()
	log.Printf("🔄 [BoxCache] Iniciando refresh del caché...")
	log.Printf("   📊 Tamaño configurado: %d cajas", m.cacheSize)

	// 1. Query a SQL Server
	log.Printf("   ⏱️  Query SQL Server iniciada...")
	startQuery := time.Now()

	ctx, cancel := context.WithTimeout(m.ctx, 30*time.Second)
	defer cancel()

	rows, err := m.ssmsManager.Query(ctx, db.SELECT_TOP_N_BOXES_FROM_CURRENT_PROCESO, m.cacheSize)
	if err != nil {
		return fmt.Errorf("error en query: %w", err)
	}
	defer rows.Close()

	// 2. Leer resultados
	var newBoxes []BoxData
	for rows.Next() {
		var box BoxData
		if err := rows.Scan(&box.Especie, &box.Calibre, &box.Variedad, &box.Embalaje, &box.Dark, &box.CodCaja); err != nil {
			log.Printf("⚠️  [BoxCache] Error al escanear fila: %v", err)
			continue
		}
		newBoxes = append(newBoxes, box)
	}

	queryDuration := time.Since(startQuery)
	log.Printf("   ✅ Query completada en %.2fms (%d filas)", queryDuration.Seconds()*1000, len(newBoxes))

	if len(newBoxes) == 0 {
		log.Printf("   ⚠️  Query retornó 0 filas (proceso actual vacío?)")
		return nil
	}

	// 3. Ordenar por codCaja DESC (para búsqueda binaria)
	log.Printf("   🔧 Ordenando por codCaja DESC...")
	startSort := time.Now()
	sort.Slice(newBoxes, func(i, j int) bool {
		return newBoxes[i].CodCaja > newBoxes[j].CodCaja
	})
	sortDuration := time.Since(startSort)
	log.Printf("   ✅ Ordenamiento completado en %.3fms", sortDuration.Seconds()*1000)

	// 4. Swap atómico del caché
	log.Printf("   🔄 Actualizando caché atómicamente...")
	startSwap := time.Now()
	m.mu.Lock()
	m.boxes = newBoxes
	m.mu.Unlock()
	swapDuration := time.Since(startSwap)
	log.Printf("   ✅ Caché actualizado en %.3fms", swapDuration.Seconds()*1000)

	// 5. Registrar métricas de tiempo
	totalDuration := time.Since(startTotal)
	m.mu2.Lock()
	m.lastRefreshDuration = totalDuration
	m.lastRefreshTime = time.Now()
	m.mu2.Unlock()

	log.Printf("✅ [BoxCache] Refresh completado en %.2fms total", totalDuration.Seconds()*1000)
	log.Printf("   📊 Caché actualizado con %d cajas", len(newBoxes))

	// Mostrar estadísticas actuales
	hits := m.hits.Load()
	misses := m.misses.Load()
	total := hits + misses
	if total > 0 {
		ratio := float64(hits) / float64(total) * 100
		log.Printf("   📈 Stats acumuladas: Hits=%d, Misses=%d, Ratio=%.1f%%", hits, misses, ratio)
	}

	return nil
}

// GetBoxData busca una caja en el caché usando búsqueda binaria
// Retorna los datos y un bool indicando si se encontró
func (m *BoxCacheManager) GetBoxData(codCaja string) (BoxData, bool) {
	startSearch := time.Now()

	log.Printf("🔍 [BoxCache] Búsqueda de caja: %s", codCaja)
	log.Printf("   ⚡ Búsqueda binaria iniciada...")

	m.mu.RLock()
	boxes := m.boxes
	m.mu.RUnlock()

	if len(boxes) == 0 {
		log.Printf("   ⚠️  Caché vacío, retornando MISS")
		m.misses.Add(1)
		return BoxData{}, false
	}

	// Búsqueda binaria (boxes está ordenado DESC)
	comparisons := 0
	left, right := 0, len(boxes)-1

	for left <= right {
		comparisons++
		mid := (left + right) / 2

		if boxes[mid].CodCaja == codCaja {
			searchDuration := time.Since(startSearch)
			m.hits.Add(1)

			hits := m.hits.Load()
			misses := m.misses.Load()
			total := hits + misses
			ratio := float64(hits) / float64(total) * 100

			log.Printf("   ✅ CACHE HIT en %.3fms (%d comparaciones)", searchDuration.Seconds()*1000, comparisons)
			log.Printf("   📊 Stats: Hits=%d, Misses=%d, Ratio=%.1f%%", hits, misses, ratio)

			return boxes[mid], true
		} else if boxes[mid].CodCaja > codCaja {
			left = mid + 1
		} else {
			right = mid - 1
		}
	}

	// No encontrado
	searchDuration := time.Since(startSearch)
	m.misses.Add(1)

	hits := m.hits.Load()
	misses := m.misses.Load()
	total := hits + misses
	ratio := float64(hits) / float64(total) * 100

	log.Printf("   ⚠️  CACHE MISS en %.3fms (%d comparaciones)", searchDuration.Seconds()*1000, comparisons)
	log.Printf("   📊 Stats: Hits=%d, Misses=%d, Ratio=%.1f%%", hits, misses, ratio)

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
