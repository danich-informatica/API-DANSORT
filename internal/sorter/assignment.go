package sorter

import (
	"context"
	"fmt"
	"log"
	"reflect"
	"strings"
	"time"

	"API-GREENEX/internal/communication/pallet"
	"API-GREENEX/internal/models"
	"API-GREENEX/internal/shared"
)

// AssignSKUToSalida asigna una SKU a una salida específica
func (s *Sorter) AssignSKUToSalida(skuID uint32, salidaID int) (calibre, variedad, embalaje string, dark int, err error) {
	log.Printf("🔵 Sorter #%d: Iniciando asignación de SKU ID=%d a salida ID=%d", s.ID, skuID, salidaID)

	targetSalida := s.findSalidaByID(salidaID)
	if targetSalida == nil {
		return "", "", "", 0, fmt.Errorf("salida con ID %d no encontrada en sorter #%d", salidaID, s.ID)
	}

	if skuID == 0 && targetSalida.Tipo == "automatico" {
		return "", "", "", 0, fmt.Errorf("no se puede asignar SKU REJECT (ID=0) a salida automática '%s' (ID=%d)",
			targetSalida.Salida_Sorter, salidaID)
	}

	targetSKU := s.findSKUByID(skuID)
	if targetSKU == nil {
		// Solo mostrar detalle cuando falla
		log.Printf("❌ Sorter #%d: SKU ID=%d no disponible. Total SKUs en memoria: %d", s.ID, skuID, len(s.assignedSKUs))
		return "", "", "", 0, fmt.Errorf("SKU con ID %d no encontrada en las SKUs disponibles del sorter #%d", skuID, s.ID)
	}

	sku := models.SKU{
		SKU:      targetSKU.SKU,
		Variedad: targetSKU.Variedad,
		Calibre:  targetSKU.Calibre,
		Embalaje: targetSKU.Embalaje,
		Dark:     targetSKU.Dark,
		Estado:   true,
	}

	// logica para asignar SKU en paletizaje automatico en produccion
	// Normalizar: tanto "automatico" como "automatica" son válidos
	log.Printf("ℹ️ Sorter #%d: Verificando tipo de salida. Salida ID=%d es tipo '%s'", s.ID, salidaID, targetSalida.Tipo)
	if targetSalida.Tipo == "automatico" || targetSalida.Tipo == "automatica" {
		// Usar configuración del sorter para conectar al servidor de paletizado
		client := pallet.NewClient(s.PaletHost, s.PaletPort, 10*time.Second)
		defer client.Close()

		log.Printf("Sorter #%d: validando factibilidad de orden de paletizaje para salida %d (servidor: %s:%d, mesa_id: %d).", s.ID, targetSalida.ID, s.PaletHost, s.PaletPort, targetSalida.MesaID)

		// Validar disponibilidad de la mesa
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		mesaDisponible, err := s.IsTableAvailable(ctx, targetSalida.MesaID, client)
		if err != nil {
			log.Printf("❌ Sorter #%d: error al consultar estado de mesa %d: %v", s.ID, targetSalida.MesaID, err)
			return "", "", "", 0, fmt.Errorf("error al validar disponibilidad de mesa %d: %w", targetSalida.MesaID, err)
		}

		if !mesaDisponible {
			log.Printf("⚠️ Sorter #%d: mesa %d no está disponible para orden de paletizaje (salida %d)",
				s.ID, targetSalida.MesaID, targetSalida.ID)
			return "", "", "", 0, fmt.Errorf("mesa %d no está disponible para orden de paletizaje", targetSalida.MesaID)
		}

		log.Printf("✅ Sorter #%d: mesa %d disponible, orden de paletizaje factible para salida %d", s.ID, targetSalida.MesaID, targetSalida.ID)

		// creamos orden de paletizaje en una go rutine
		log.Printf("🚀 Sorter #%d: Lanzando goroutine SendFrabricationOrder para salida %d", s.ID, targetSalida.ID)
		go s.SendFrabricationOrder(targetSalida, sku, client)
	}

	targetSalida.SKUs_Actuales = append(targetSalida.SKUs_Actuales, sku)
	targetSKU.IsAssigned = true

	log.Printf("✅ Sorter #%d: SKU '%s' (ID=%d) asignada a salida '%s' (ID=%d, tipo=%s)",
		s.ID, targetSKU.SKU, skuID, targetSalida.Salida_Sorter, salidaID, targetSalida.Tipo)

	s.UpdateSKUs(s.assignedSKUs)

	return targetSKU.Calibre, targetSKU.Variedad, targetSKU.Embalaje, targetSKU.Dark, nil
}

// IsTableAvailable consulta si una mesa de paletizado está disponible
// Retorna true si la mesa está libre (estado = 1, sin orden activa)
func (s *Sorter) IsTableAvailable(ctx context.Context, mesaID int, client *pallet.Client) (bool, error) {
	// Consultar estado de la mesa específica
	estados, err := client.GetEstadoMesa(ctx, mesaID)
	if err != nil {
		// Si el error es "Mesa NO tiene OF activa" (código 202), la mesa está LIBRE
		if err == pallet.ErrMesaNoActiva {
			log.Printf("✅ Sorter #%d: Mesa %d está libre (sin orden activa)", s.ID, mesaID)
			return true, nil
		}
		return false, fmt.Errorf("error al consultar estado de mesa %d: %w", mesaID, err)
	}

	if len(estados) == 0 {
		return false, fmt.Errorf("no se recibió información de la mesa %d", mesaID)
	}

	estado := estados[0]

	// Estado = 1: Libre (disponible para nueva orden)
	// Estado = 2: Bloqueado (tiene orden activa)
	if estado.Estado == 1 {
		log.Printf("✅ Sorter #%d: Mesa %d está libre (estado=%d)", s.ID, mesaID, estado.Estado)
		return true, nil
	}
	log.Printf("⚠️ Sorter #%d: Mesa %d está bloqueada (estado=%d, descripción='%s')", s.ID, mesaID, estado.Estado, estado.DescripcionEstado)
	return false, nil
}

// SendFrabricationOrder envía una orden de fabricación al sistema de paletizaje
func (s *Sorter) SendFrabricationOrder(salida *shared.Salida, sku models.SKU, client *pallet.Client) {
	log.Printf("🚚 Sorter #%d: Iniciando envío de orden de fabricación para mesa %d", s.ID, salida.MesaID)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Valores fijos
	const (
		NumeroPalesFijo = 0 // Fijo
	)

	// Obtener datos dinámicos de FX_Sync usando el código de embalaje
	var cajasPerPale int
	var cajasPerCapa int
	var codigoTipoEnvase string
	var codigoTipoPale string
	var flejado int

	log.Printf("ℹ️ Sorter #%d: Verificando FXSyncManager...", s.ID)
	if s.fxSyncManager != nil {
		// Type assertion para usar el método
		// Usamos interface{} genérica ya que OrdenFabricacionData está en package db
		type FXSyncQuerier interface {
			GetOFData(ctx context.Context, codigoEmbalaje string) (interface{}, error)
		}

		if fxSync, ok := s.fxSyncManager.(FXSyncQuerier); ok {
			log.Printf("🔍 Sorter #%d: Consultando V_Danish en FX_Sync con embalaje '%s'", s.ID, sku.Embalaje)
			ofDataRaw, err := fxSync.GetOFData(ctx, sku.Embalaje)
			if err != nil {
				log.Printf("❌ Sorter #%d: Error al obtener datos de V_Danish para embalaje '%s': %v",
					s.ID, sku.Embalaje, err)
				log.Printf("⚠️  Sorter #%d: No se puede crear orden sin datos de V_Danish", s.ID)
				return
			}

			// Extraer campos usando reflection ya que viene como interface{}
			// Usamos interface con métodos getter para acceder a los campos
			type OFDataGetter interface {
				GetCajasPerPale() int
				GetCajasPerCapa() int
				GetCodigoTipoEnvase() string
				GetCodigoTipoPale() string
				GetFlejado() int64
			}

			// Intentar con interface de métodos
			if ofGetter, ok := ofDataRaw.(OFDataGetter); ok {
				cajasPerPale = ofGetter.GetCajasPerPale()
				cajasPerCapa = ofGetter.GetCajasPerCapa()
				codigoTipoEnvase = strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(ofGetter.GetCodigoTipoEnvase(), "\n", ""), "\r", ""))
				codigoTipoPale = strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(ofGetter.GetCodigoTipoPale(), "\n", ""), "\r", ""))
				flejado = int(ofGetter.GetFlejado())
			} else {
				// Fallback: usar reflection para extraer campos
				// Esto funciona con cualquier struct que tenga los campos correctos
				v := reflect.ValueOf(ofDataRaw)
				if v.Kind() == reflect.Ptr {
					v = v.Elem()
				}

				if v.Kind() != reflect.Struct {
					log.Printf("❌ Sorter #%d: Error al convertir datos de FX_Sync (tipo: %T, kind: %v)",
						s.ID, ofDataRaw, v.Kind())
					return
				}

				cajasPerPale = int(v.FieldByName("CajasPerPale").Int())
				cajasPerCapa = int(v.FieldByName("CajasPerCapa").Int())
				codigoTipoEnvase = strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(v.FieldByName("CodigoTipoEnvase").String(), "\n", ""), "\r", ""))
				codigoTipoPale = strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(v.FieldByName("CodigoTipoPale").String(), "\n", ""), "\r", ""))
				flejado = int(v.FieldByName("Flejado").Int())
			}

			log.Printf("📊 Sorter #%d: Datos obtenidos de V_Danish para '%s': %d cajas/palé, %d cajas/capa, envase='%s', palé='%s', flejado=%d",
				s.ID, sku.Embalaje, cajasPerPale, cajasPerCapa, codigoTipoEnvase, codigoTipoPale, flejado)
		} else {
			log.Printf("❌ Sorter #%d: FXSyncManager no implementa la interfaz FXSyncQuerier", s.ID)
			return
		}
	} else {
		log.Printf("❌ Sorter #%d: FXSyncManager no está inicializado", s.ID)
		return
	}

	// Crear orden con datos obtenidos
	orden := pallet.OrdenFabricacionRequest{
		NumeroPales:       NumeroPalesFijo,  // FIJO: 5
		CajasPerPale:      cajasPerPale,     // DINÁMICO: de FX_Sync
		CajasPerCapa:      cajasPerCapa,     // DINÁMICO: de FX_Sync
		CodigoTipoEnvase:  codigoTipoEnvase, // DINÁMICO: de FX_Sync
		CodigoTipoPale:    codigoTipoPale,   // DINÁMICO: de FX_Sync
		IDProgramaFlejado: flejado,          // DINAMICO: de FX_Sync
	}

	log.Printf("📋 Sorter #%d: Creando orden en mesa %d: %d palés × %d cajas (envase: %s, palé: %s, flejado: %d)",
		s.ID, salida.MesaID, orden.NumeroPales, orden.CajasPerPale, orden.CodigoTipoEnvase, orden.CodigoTipoPale, orden.IDProgramaFlejado)

	// 1. Enviar orden a Serfruit
	log.Printf("➡️  Sorter #%d: Enviando orden a Serfruit para mesa %d...", s.ID, salida.MesaID)
	err := client.CrearOrdenFabricacion(ctx, salida.MesaID, orden)
	if err != nil {
		log.Printf("❌ Sorter #%d: Error al crear orden en mesa %d: %v", s.ID, salida.MesaID, err)
		return
	}

	log.Printf("✅ Sorter #%d: Orden de fabricación creada exitosamente en mesa %d (Serfruit)", s.ID, salida.MesaID)

	// 2. Insertar orden en PostgreSQL y obtener ID
	log.Printf("ℹ️ Sorter #%d: Verificando dbManager para registrar orden...", s.ID)
	if s.dbManager != nil {
		// Type assertion para acceder al método InsertOrdenFabricacion
		type OrdenInserter interface {
			InsertOrdenFabricacion(ctx context.Context, mesaID, numeroPales, cajasPerPale, cajasPerCapa int, codigoEnvase, codigoPale string, idProgramaFlejado int) (int, error)
		}

		if psql, ok := s.dbManager.(OrdenInserter); ok {
			log.Printf("✍️  Sorter #%d: Registrando orden en PostgreSQL para mesa %d...", s.ID, salida.MesaID)
			ordenID, err := psql.InsertOrdenFabricacion(
				ctx,
				salida.MesaID,
				orden.NumeroPales,
				orden.CajasPerPale,
				orden.CajasPerCapa,
				orden.CodigoTipoEnvase,
				orden.CodigoTipoPale,
				orden.IDProgramaFlejado,
			)

			if err != nil {
				// CRÍTICO: Si no se puede registrar la orden, no continuar.
				log.Printf("❌ CRÍTICO: Sorter #%d: Error al registrar orden en PostgreSQL (mesa %d): %v. La orden no se procesará.", s.ID, salida.MesaID, err)
				return // Detener la ejecución de esta goroutine.
			}

			// Guardar ID de orden en la salida
			salida.IDOrdenActiva = ordenID
			log.Printf("✅ Sorter #%d: Orden ID=%d registrada en PostgreSQL y guardada en salida #%d", s.ID, ordenID, salida.ID)
		} else {
			log.Printf("❌ Sorter #%d: dbManager no implementa la interfaz OrdenInserter", s.ID)
		}
	} else {
		log.Printf("❌ Sorter #%d: dbManager no está inicializado, no se puede registrar la orden.", s.ID)
	}
}

// RemoveSKUFromSalida elimina una SKU específica de una salida
func (s *Sorter) RemoveSKUFromSalida(skuID uint32, salidaID int) (calibre, variedad, embalaje string, dark int, err error) {
	// PROTECCIÓN: NO permitir eliminar SKU REJECT (ID=0)
	if skuID == 0 {
		return "", "", "", 0, fmt.Errorf("no se puede eliminar SKU REJECT (ID=0), está protegida")
	}

	var targetSalida *shared.Salida
	var salidaIndex int

	for i := range s.Salidas {
		if s.Salidas[i].ID == salidaID {
			targetSalida = &s.Salidas[i]
			salidaIndex = i
			break
		}
	}

	if targetSalida == nil {
		return "", "", "", 0, fmt.Errorf("salida con ID %d no encontrada en sorter #%d", salidaID, s.ID)
	}

	skuFound := false
	var removedSKU models.SKU
	newSKUs := make([]models.SKU, 0, len(targetSalida.SKUs_Actuales))

	for _, sku := range targetSalida.SKUs_Actuales {
		currentSKUID := uint32(sku.GetNumericID())
		if currentSKUID == skuID {
			skuFound = true
			removedSKU = sku
			continue
		}
		newSKUs = append(newSKUs, sku)
	}

	if !skuFound {
		return "", "", "", 0, fmt.Errorf("SKU con ID %d no encontrada en salida %d del sorter #%d", skuID, salidaID, s.ID)
	}

	s.Salidas[salidaIndex].SKUs_Actuales = newSKUs

	for i := range s.assignedSKUs {
		if uint32(s.assignedSKUs[i].ID) == skuID {
			s.assignedSKUs[i].IsAssigned = false
			break
		}
	}

	log.Printf("🗑️  Sorter #%d: SKU '%s' (ID=%d) eliminada de salida '%s' (ID=%d)",
		s.ID, removedSKU.SKU, skuID, targetSalida.Salida_Sorter, salidaID)

	s.UpdateSKUs(s.assignedSKUs)

	// Si es salida automática, ejecutar secuencia de vaciado
	// Normalizar: tanto "automatico" como "automatica" son válidos
	tipoSalida := s.Salidas[salidaIndex].Tipo
	if tipoSalida == "automatico" || tipoSalida == "automatica" {
		log.Printf("🔄 Sorter #%d: Iniciando secuencia de vaciado para salida automática %d", s.ID, salidaID)
		go s.SecuenciaVaciado(targetSalida)
	}

	return removedSKU.Calibre, removedSKU.Variedad, removedSKU.Embalaje, removedSKU.Dark, nil
}

func (s *Sorter) SecuenciaVaciado(salida *shared.Salida) {
	log.Printf("🔄 Sorter #%d: Iniciando secuencia de vaciado para salida %d (mesa %d, tipo: %s)",
		s.ID, salida.ID, salida.MesaID, salida.Tipo)

	// Validar que tenemos lo necesario
	if s.plcManager == nil {
		log.Printf("❌ Sorter #%d: No se puede ejecutar secuencia - PLCManager no disponible", s.ID)
		return
	}

	if salida.BloqueoNode == "" {
		log.Printf("⚠️  Sorter #%d: Salida %d no tiene nodo de bloqueo configurado, continuando sin bloqueo PLC",
			s.ID, salida.ID)
	}

	// Crear cliente de paletizado
	client := pallet.NewClient(s.PaletHost, s.PaletPort, 10*time.Second)
	defer client.Close()

	// PASO 1: Bloquear salida en PLC
	if salida.BloqueoNode != "" {
		log.Printf("🔒 Sorter #%d: Bloqueando salida %d (nodo: %s)", s.ID, salida.ID, salida.BloqueoNode)
		err := s.plcManager.LockSalida(s.ID, salida.BloqueoNode)
		if err != nil {
			log.Printf("❌ Sorter #%d: Error al bloquear salida %d en PLC: %v", s.ID, salida.ID, err)
			log.Printf("⚠️  Sorter #%d: Continuando secuencia a pesar del error de bloqueo", s.ID)
		} else {
			salida.SetBloqueo(true) // Actualizar estado en memoria
			log.Printf("✅ Sorter #%d: Salida %d bloqueada exitosamente", s.ID, salida.ID)
		}
	}

	// PASO 2: Esperar 2 segundos para que entren las últimas cajas
	log.Printf("⏳ Sorter #%d: Esperando 2 segundos para que entren últimas cajas...", s.ID)
	time.Sleep(2 * time.Second)

	// PASO 3: Vaciar mesa en servidor de paletizado (modo 2 = finalizar orden)
	log.Printf("🧹 Sorter #%d: Enviando orden de vaciado a mesa %d (modo 2: finalizar orden)",
		s.ID, salida.MesaID)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := client.VaciarMesa(ctx, salida.MesaID, 2)
	if err != nil {
		log.Printf("❌ Sorter #%d: Error al vaciar mesa %d: %v", s.ID, salida.MesaID, err)
		log.Printf("⚠️  Sorter #%d: La mesa podría no haberse vaciado correctamente", s.ID)
	} else {
		log.Printf("✅ Sorter #%d: Mesa %d vaciada exitosamente (orden finalizada)", s.ID, salida.MesaID)
	}

	// PASO 4: Esperar 3 segundos para que el paletizador procese
	log.Printf("⏳ Sorter #%d: Esperando 3 segundos para que paletizador procese...", s.ID)
	time.Sleep(3 * time.Second)

	// PASO 5: Desbloquear salida en PLC
	if salida.BloqueoNode != "" {
		log.Printf("🔓 Sorter #%d: Desbloqueando salida %d (nodo: %s)", s.ID, salida.ID, salida.BloqueoNode)
		err := s.plcManager.UnlockSalida(s.ID, salida.BloqueoNode)
		if err != nil {
			log.Printf("❌ Sorter #%d: Error al desbloquear salida %d en PLC: %v", s.ID, salida.ID, err)
			log.Printf("🚨 Sorter #%d: CRÍTICO - Salida %d quedó bloqueada, requiere intervención manual",
				s.ID, salida.ID)
		} else {
			salida.SetBloqueo(false) // Actualizar estado en memoria
			log.Printf("✅ Sorter #%d: Salida %d desbloqueada exitosamente", s.ID, salida.ID)
		}
	}

	log.Printf("✅ Sorter #%d: Secuencia de vaciado completada para salida %d (total: 5 segundos)",
		s.ID, salida.ID)
}

// RemoveAllSKUsFromSalida elimina TODAS las SKUs de una salida específica
// y re-inserta automáticamente la SKU REJECT
func (s *Sorter) RemoveAllSKUsFromSalida(salidaID int) ([]models.SKU, error) {
	var targetSalida *shared.Salida
	var salidaIndex int

	for i := range s.Salidas {
		if s.Salidas[i].ID == salidaID {
			targetSalida = &s.Salidas[i]
			salidaIndex = i
			break
		}
	}

	if targetSalida == nil {
		return nil, fmt.Errorf("salida con ID %d no encontrada en sorter #%d", salidaID, s.ID)
	}

	// Guardar todas las SKUs actuales antes de borrar
	removedSKUs := make([]models.SKU, len(targetSalida.SKUs_Actuales))
	copy(removedSKUs, targetSalida.SKUs_Actuales)

	// Marcar todas como no asignadas
	for _, sku := range removedSKUs {
		skuID := uint32(sku.GetNumericID())
		for i := range s.assignedSKUs {
			if uint32(s.assignedSKUs[i].ID) == skuID {
				s.assignedSKUs[i].IsAssigned = false
			}
		}
	}

	// 🧹 BORRAR TODO
	s.Salidas[salidaIndex].SKUs_Actuales = []models.SKU{}

	// ♻️ RE-INSERTAR SKU REJECT automáticamente
	rejectSKU := models.SKU{
		SKU:      "REJECT",
		Calibre:  "REJECT",
		Variedad: "REJECT",
		Embalaje: "REJECT",
	}
	s.Salidas[salidaIndex].SKUs_Actuales = append(s.Salidas[salidaIndex].SKUs_Actuales, rejectSKU)

	log.Printf("🧹 Sorter #%d: Eliminadas %d SKUs de salida '%s' (ID=%d)",
		s.ID, len(removedSKUs), targetSalida.Salida_Sorter, salidaID)
	log.Printf("♻️  Sorter #%d: SKU REJECT re-insertada automáticamente en salida %d", s.ID, salidaID)

	s.UpdateSKUs(s.assignedSKUs)

	return removedSKUs, nil
}

// findSalidaByID busca una salida por ID
func (s *Sorter) findSalidaByID(salidaID int) *shared.Salida {
	for i := range s.Salidas {
		if s.Salidas[i].ID == salidaID {
			return &s.Salidas[i]
		}
	}
	return nil
}

// findSKUByID busca una SKU por ID
func (s *Sorter) findSKUByID(skuID uint32) *models.SKUAssignable {
	for i := range s.assignedSKUs {
		if uint32(s.assignedSKUs[i].ID) == skuID {
			return &s.assignedSKUs[i]
		}
	}
	return nil
}
