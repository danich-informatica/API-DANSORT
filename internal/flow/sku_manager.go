package flow

import (
	"context"
	"log"

	"API-GREENEX/internal/db"
	"API-GREENEX/internal/models"
)

// SKUManager gestiona las operaciones relacionadas con los SKUs.
type SKUManager struct {
	ctx       context.Context
	dbManager *db.PostgresManager
	skus      map[string]models.SKU // Mapa para acceso rápido por SKU
}

// NewSKUManager crea una nueva instancia de SKUManager.
func NewSKUManager() (*SKUManager, error) {
	ctx := context.Background()

	dbManager, err := db.GetPostgresManager()
	if err != nil {
		return nil, err
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
	skus := make([]models.SKU, 0, len(m.skus))
	for _, sku := range m.skus {
		skus = append(skus, sku)
	}
	return skus
}

func (m *SKUManager) GetActiveSKUs() []models.SKU {
	activeSKUs := make([]models.SKU, 0)
	for _, sku := range m.skus {
		if sku.Estado {
			activeSKUs = append(activeSKUs, sku)
		}
	}
	return activeSKUs
}
