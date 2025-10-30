package db

import (
	"context"
	"database/sql"
	"fmt"

	"API-GREENEX/internal/config"
)

// ============================================================================
// FXSyncManager - Gestiona conexión específica a FX_Sync
// ============================================================================

// FXSyncManager gestiona la conexión específica a FX_Sync
// Envuelve el Manager genérico con métodos específicos del dominio FX_Sync
type FXSyncManager struct {
	*Manager // Composición: hereda todos los métodos del Manager genérico
}

// NewFXSyncManager crea una nueva instancia del gestor para FX_Sync
func NewFXSyncManager(ctx context.Context, cfg config.SQLServerConfig) (*FXSyncManager, error) {
	// Crear una copia del config y forzar la base de datos correcta
	fxSyncConfig := cfg
	fxSyncConfig.Database = "FX_Sync"

	// Usar el Manager genérico con label personalizada
	manager, err := GetManagerWithConfigAndLabel(ctx, fxSyncConfig, "FX_Sync")
	if err != nil {
		return nil, fmt.Errorf("error al crear FXSyncManager: %w", err)
	}

	return &FXSyncManager{Manager: manager}, nil
}

// GetFXSyncManager crea FXSyncManager desde el config completo
func GetFXSyncManager(ctx context.Context, cfg *config.Config) (*FXSyncManager, error) {
	return NewFXSyncManager(ctx, cfg.Database.SQLServer)
}

// OrdenFabricacionData contiene los datos necesarios para crear una orden de fabricación
type OrdenFabricacionData struct {
	CajasPerPale     int    // CANTIDAD_CAJAS
	CajasPerCapa     int    // CAJAS POR CAPA
	CodigoTipoEnvase string // CODIGO ENVASE
	CodigoTipoPale   string // CODIGO PALLET
}

// GetOFData obtiene los datos de una orden de fabricación desde la vista V_Danish
// usando el CODIGO_EMBALAJE como key
// Retorna interface{} para evitar import cycles
func (m *FXSyncManager) GetOFData(ctx context.Context, codigoEmbalaje string) (interface{}, error) {
	if m == nil || m.Manager == nil || m.Manager.db == nil {
		return nil, fmt.Errorf("FXSyncManager no inicializado")
	}

	var data OrdenFabricacionData
	var nombreEnvase sql.NullString
	var anchoCaja, largoCaja, altoCaja, anchoPallet, largoPallet, altoPallet sql.NullFloat64

	err := m.Manager.QueryRow(ctx, SELECT_V_DANISH_BY_CODIGO_EMBALAJE,
		sql.Named("p1", codigoEmbalaje),
	).Scan(
		&data.CajasPerPale,
		&data.CajasPerCapa,
		&data.CodigoTipoEnvase,
		&anchoCaja,
		&largoCaja,
		&altoCaja,
		&nombreEnvase,
		&data.CodigoTipoPale,
		&anchoPallet,
		&largoPallet,
		&altoPallet,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("no se encontró configuración para código de embalaje '%s' en vista V_Danish", codigoEmbalaje)
	}

	if err != nil {
		return nil, fmt.Errorf("error al consultar vista V_Danish para código '%s': %w", codigoEmbalaje, err)
	}

	// Validar que los datos esenciales no estén vacíos
	if data.CajasPerPale == 0 {
		return nil, fmt.Errorf("CajasPerPale es 0 para código de embalaje '%s'", codigoEmbalaje)
	}

	if data.CajasPerCapa == 0 {
		return nil, fmt.Errorf("CajasPerCapa es 0 para código de embalaje '%s'", codigoEmbalaje)
	}

	if data.CodigoTipoEnvase == "" {
		return nil, fmt.Errorf("CodigoTipoEnvase vacío para código de embalaje '%s'", codigoEmbalaje)
	}

	if data.CodigoTipoPale == "" {
		return nil, fmt.Errorf("CodigoTipoPale vacío para código de embalaje '%s'", codigoEmbalaje)
	}

	return &data, nil
}
