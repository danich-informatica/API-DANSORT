package flow

import (
	"context"
	"fmt"
	"log"
	"sync"

	"API-GREENEX/internal/db"
	"API-GREENEX/internal/models"
)

// SKUManager gestiona las operaciones relacionadas con los SKUs.
type SKUManager struct {
	ctx       context.Context
	dbManager *db.PostgresManager
	skus      map[string]models.SKU // Mapa para acceso rápido por SKU
	mu        sync.RWMutex          // Mutex para acceso concurrente seguro
}

// NewSKUManager crea una nueva instancia de SKUManager usando un dbManager existente.
func NewSKUManager(ctx context.Context, dbManager *db.PostgresManager) (*SKUManager, error) {
	if dbManager == nil {
		log.Println("⚠️  NewSKUManager: dbManager es nil")
		return nil, nil
	}

	// Cargar todos los SKUs desde la base de datos al iniciar el manager
	skus, err := dbManager.GetAllSKUs(ctx)
	if err != nil {
		return nil, err
	}

	manager := &SKUManager{
		ctx:       ctx,
		dbManager: dbManager,
		skus:      make(map[string]models.SKU),
	}

	for _, sku := range skus {
		// Usar una clave compuesta: calibre-variedad-embalaje
		key := sku.Calibre + "-" + sku.Variedad + "-" + sku.Embalaje
		manager.skus[key] = sku
	}

	log.Printf("✅ SKUManager inicializado con %d SKUs", len(skus))
	return manager, nil
}

func (m *SKUManager) GetAllSKUs() []models.SKU {
	m.mu.RLock()
	defer m.mu.RUnlock()

	skus := make([]models.SKU, 0, len(m.skus))
	for _, sku := range m.skus {
		skus = append(skus, sku)
	}
	return skus
}

func (m *SKUManager) GetActiveSKUs() []models.SKU {
	m.mu.RLock()
	defer m.mu.RUnlock()

	activeSKUs := make([]models.SKU, 0)
	for _, sku := range m.skus {
		if sku.Estado {
			activeSKUs = append(activeSKUs, sku)
		}
	}
	return activeSKUs
}

// StreamActiveSKUs envía SKUs activos a través de un canal para procesamiento no bloqueante.
// Ideal para conjuntos de datos grandes (millones de registros).
// El canal se cierra automáticamente al finalizar.
func (m *SKUManager) StreamActiveSKUs(ctx context.Context, skuChan models.SKUChannel) {
	defer close(skuChan)

	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, sku := range m.skus {
		// Verificar si el contexto fue cancelado
		select {
		case <-ctx.Done():
			log.Println("StreamActiveSKUs: contexto cancelado")
			return
		default:
		}

		// Enviar solo SKUs activos
		if sku.Estado {
			select {
			case skuChan <- sku:
				// SKU enviado correctamente
			case <-ctx.Done():
				log.Println("StreamActiveSKUs: contexto cancelado durante envío")
				return
			}
		}
	}
}

// StreamActiveSKUsWithLimit envía hasta 'limit' SKUs activos a través del canal.
// Si limit <= 0, no hay límite (equivalente a StreamActiveSKUs).
func (m *SKUManager) StreamActiveSKUsWithLimit(ctx context.Context, skuChan models.SKUChannel, limit int) {
	defer close(skuChan)

	m.mu.RLock()
	defer m.mu.RUnlock()

	sent := 0
	for _, sku := range m.skus {
		// Verificar límite
		if limit > 0 && sent >= limit {
			return
		}

		// Verificar si el contexto fue cancelado
		select {
		case <-ctx.Done():
			log.Println("StreamActiveSKUsWithLimit: contexto cancelado")
			return
		default:
		}

		// Enviar solo SKUs activos
		if sku.Estado {
			select {
			case skuChan <- sku:
				sent++
			case <-ctx.Done():
				log.Println("StreamActiveSKUsWithLimit: contexto cancelado durante envío")
				return
			}
		}
	}
}

// ReloadFromDB recarga todos los SKUs desde PostgreSQL (thread-safe)
// Se usa después de sincronizar con SQL Server para actualizar el cache
func (m *SKUManager) ReloadFromDB(ctx context.Context) error {
	if m.dbManager == nil {
		return fmt.Errorf("dbManager no inicializado")
	}

	// Obtener SKUs desde DB
	skus, err := m.dbManager.GetAllSKUs(ctx)
	if err != nil {
		return fmt.Errorf("error al obtener SKUs: %w", err)
	}

	// Actualizar mapa de forma thread-safe
	m.mu.Lock()

	// Limpiar mapa actual
	m.skus = make(map[string]models.SKU)

	// Recargar con los nuevos datos
	activeCount := 0
	for _, sku := range skus {
		key := sku.Calibre + "-" + sku.Variedad + "-" + sku.Embalaje
		m.skus[key] = sku
		if sku.Estado {
			activeCount++
		}
	}

	m.mu.Unlock() // Liberar lock ANTES de log

	log.Printf("♻️  SKUManager recargado: %d SKUs totales (%d activos)",
		len(skus), activeCount)
	return nil
}
