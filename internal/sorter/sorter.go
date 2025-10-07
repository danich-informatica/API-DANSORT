// (Eliminado: función duplicada antes de package)
package sorter

import (
	"API-GREENEX/internal/listeners"
	"API-GREENEX/internal/models"
	"API-GREENEX/internal/shared"
	"context"
	"fmt"
	"log"
	"time"
)

type Sorter struct {
	ID        int                       `json:"id"`
	Ubicacion string                    `json:"ubicacion"`
	Salidas   []shared.Salida           `json:"salidas"`
	Cognex    *listeners.CognexListener `json:"cognex"`
	ctx       context.Context
	cancel    context.CancelFunc

	// Canal de SKUs assignables para este sorter
	skuChannel chan []models.SKUAssignable

	// SKUs actualmente asignados
	assignedSKUs []models.SKUAssignable

	// Estadísticas
	LecturasExitosas int
	LecturasFallidas int
}

// GetSalidas retorna todas las salidas del sorter
func (s *Sorter) GetSalidas() []shared.Salida {
	return s.Salidas
}

func GetNewSorter(ID int, ubicacion string, salidas []shared.Salida, cognex *listeners.CognexListener) *Sorter {
	ctx, cancel := context.WithCancel(context.Background())

	// Registrar canal de SKUs para este sorter
	channelMgr := shared.GetChannelManager()
	sorterID := fmt.Sprintf("%d", ID)
	skuChannel := channelMgr.RegisterSorterSKUChannel(sorterID, 10) // Buffer de 10

	return &Sorter{
		ID:           ID,
		Ubicacion:    ubicacion,
		Salidas:      salidas,
		Cognex:       cognex,
		ctx:          ctx,
		cancel:       cancel,
		skuChannel:   skuChannel,
		assignedSKUs: make([]models.SKUAssignable, 0),
	}
}

func (s *Sorter) Start() error {
	log.Printf("🚀 Iniciando Sorter #%d en %s", s.ID, s.Ubicacion)
	log.Printf("📍 Salidas configuradas: %d", len(s.Salidas))

	for _, salida := range s.Salidas {
		log.Printf("  ↳ Salida %d: %s", salida.ID, salida.Salida_Sorter)
	}

	// Iniciar el listener de Cognex
	if s.Cognex != nil {
		err := s.Cognex.Start()
		if err != nil {
			return err
		}
	}

	// 🆕 Iniciar goroutine para procesar eventos de lectura
	go s.procesarEventosCognex()

	log.Printf("✅ Sorter iniciado y escuchando eventos de Cognex")

	return nil
}

// 🆕 Procesar eventos de lectura de Cognex
func (s *Sorter) procesarEventosCognex() {
	log.Println("👂 Sorter: Escuchando eventos de Cognex...")

	for {
		select {
		case <-s.ctx.Done():
			log.Println("🛑 Sorter: Deteniendo procesamiento de eventos")
			return

		case evento, ok := <-s.Cognex.EventChan:
			if !ok {
				log.Println("⚠️  Sorter: Canal de eventos cerrado")
				return
			}

			// Procesar el evento según su tipo
			tipoLectura := evento.GetTipo()

			if evento.Exitoso {
				s.LecturasExitosas++
				salida := s.determinarSalida(evento.SKU, evento.Calibre)
				log.Printf("✅ [Sorter] Lectura #%d | SKU: %s | Salida: %s (ID: %d) | Razón: sort por SKU",
					s.LecturasExitosas, evento.SKU, salida.Salida_Sorter, salida.ID)
				// TODO: Activar actuadores/PLC para enviar a la salida
			} else {
				s.LecturasFallidas++
				var salida shared.Salida
				razon := ""
				switch tipoLectura {
				case models.LecturaNoRead:
					salidaPtr := s.GetDiscardSalida()
					if salidaPtr != nil {
						salida = *salidaPtr
					}
					razon = "sort por descarte (NO_READ)"
				case models.LecturaFormato, models.LecturaSKU:
					salidaPtr := s.GetDiscardSalida()
					if salidaPtr != nil {
						salida = *salidaPtr
					}
					razon = "sort por descarte (formato/SKU inválido)"
				case models.LecturaDB:
					razon = "error de base de datos"
				}
				log.Printf("❌ [Sorter] Fallo #%d | SKU: %s | Salida: %s (ID: %d) | Razón: %s | %s",
					s.LecturasFallidas, evento.SKU, salida.Salida_Sorter, salida.ID, razon, evento.String())
			}

			// Mostrar estadísticas cada 10 lecturas
			total := s.LecturasExitosas + s.LecturasFallidas
			if total%10 == 0 && total > 0 {
				tasaExito := float64(s.LecturasExitosas) / float64(total) * 100
				log.Printf("📊 [Sorter] Stats: Total=%d | Exitosas=%d | Fallidas=%d | Tasa=%.1f%%",
					total, s.LecturasExitosas, s.LecturasFallidas, tasaExito)
			}
		}
	}
}

// Determinar a qué salida debe ir la caja según el SKU/Calibre
func (s *Sorter) determinarSalida(sku, calibre string) shared.Salida {
	// Si el SKU es REJECT (id=0 o texto 'REJECT'), buscar salida manual o de descarte
	if sku == "REJECT" || sku == "0" {
		for _, salida := range s.Salidas {
			if salida.Tipo == "manual" {
				return salida
			}
		}
		// Si no hay salida manual, usar la última
		if len(s.Salidas) > 0 {
			return s.Salidas[len(s.Salidas)-1]
		}
	}

	// Buscar la salida que tenga configurado ese SKU
	for _, salida := range s.Salidas {
		for _, skuConfig := range salida.SKUs_Actuales {
			if skuConfig.SKU == sku {
				return salida
			}
		}
	}

	// Si no está configurado en ninguna salida, enviar a descarte
	for _, salida := range s.Salidas {
		if salida.Tipo == "manual" {
			return salida
		}
	}
	// Si no hay salida manual, usar la última
	if len(s.Salidas) > 0 {
		return s.Salidas[len(s.Salidas)-1]
	}

	// Si no hay salidas configuradas, devolver un valor vacío
	return shared.Salida{}
}

func (s *Sorter) Stop() error {
	log.Printf("🛑 Deteniendo Sorter #%d", s.ID)
	s.cancel()

	if s.Cognex != nil {
		return s.Cognex.Stop()
	}

	return nil
}

// UpdateSKUs actualiza la lista de SKUs asignados a este sorter y publica al canal
func (s *Sorter) UpdateSKUs(skus []models.SKUAssignable) {
	s.assignedSKUs = skus

	// Publicar al canal (no bloqueante)
	select {
	case s.skuChannel <- skus:
		log.Printf("📦 Sorter #%d: SKUs actualizados (%d SKUs)", s.ID, len(skus))
	default:
		log.Printf("⚠️  Sorter #%d: Canal lleno, SKUs no publicados", s.ID)
	}
}

// GetSKUChannel retorna el canal de SKUs de este sorter
// GetID retorna el ID del sorter (implementa SorterInterface)
func (s *Sorter) GetID() int {
	return s.ID
}

func (s *Sorter) GetSKUChannel() chan []models.SKUAssignable {
	return s.skuChannel
}

// GetCurrentSKUs retorna la lista actual de SKUs asignados (implementa SorterInterface)
func (s *Sorter) GetCurrentSKUs() []models.SKUAssignable {
	return s.assignedSKUs
}

// AssignSKUToSalida asigna una SKU a una salida específica
// Retorna los datos del SKU asignado (calibre, variedad, embalaje) para insertar en BD
// Retorna error si:
// - La salida no existe
// - La SKU es REJECT (ID=0) y la salida es automática
// - La SKU no existe en assignedSKUs
func (s *Sorter) AssignSKUToSalida(skuID uint32, salidaID int) (calibre, variedad, embalaje string, err error) {
	// Buscar la salida
	var targetSalida *shared.Salida
	for i := range s.Salidas {
		if s.Salidas[i].ID == salidaID {
			targetSalida = &s.Salidas[i]
			break
		}
	}

	if targetSalida == nil {
		return "", "", "", fmt.Errorf("salida con ID %d no encontrada en sorter #%d", salidaID, s.ID)
	}

	// VALIDACIÓN CRÍTICA: REJECT no puede ir a salida automática
	if skuID == 0 && targetSalida.Tipo == "automatico" {
		return "", "", "", fmt.Errorf("no se puede asignar SKU REJECT (ID=0) a salida automática '%s' (ID=%d)",
			targetSalida.Salida_Sorter, salidaID)
	}

	// Buscar la SKU en assignedSKUs
	var targetSKU *models.SKUAssignable
	for i := range s.assignedSKUs {
		if uint32(s.assignedSKUs[i].ID) == skuID {
			targetSKU = &s.assignedSKUs[i]
			break
		}
	}

	if targetSKU == nil {
		return "", "", "", fmt.Errorf("SKU con ID %d no encontrada en las SKUs disponibles del sorter #%d", skuID, s.ID)
	}

	// Convertir SKUAssignable a SKU para agregar a la salida
	sku := models.SKU{
		SKU:      targetSKU.SKU,
		Variedad: targetSKU.Variedad,
		Calibre:  targetSKU.Calibre,
		Embalaje: targetSKU.Embalaje,
		Estado:   true,
	}

	// Agregar SKU a la salida
	targetSalida.SKUs_Actuales = append(targetSalida.SKUs_Actuales, sku)

	// Marcar SKU como asignada
	targetSKU.IsAssigned = true

	log.Printf("✅ Sorter #%d: SKU '%s' (ID=%d) asignada a salida '%s' (ID=%d, tipo=%s)",
		s.ID, targetSKU.SKU, skuID, targetSalida.Salida_Sorter, salidaID, targetSalida.Tipo)

	return targetSKU.Calibre, targetSKU.Variedad, targetSKU.Embalaje, nil
}

// FindSalidaForSKU busca en qué salida está asignada una SKU específica
// Retorna el ID de la salida, o -1 si no está asignada (debe ir a descarte)
func (s *Sorter) FindSalidaForSKU(skuText string) int {
	for _, salida := range s.Salidas {
		for _, sku := range salida.SKUs_Actuales {
			if sku.SKU == skuText {
				return salida.ID
			}
		}
	}
	return -1 // No encontrada, debe ir a descarte
}

// GetDiscardSalida retorna una salida de descarte (tipo manual)
// Si no hay ninguna configurada, asigna una aleatoriamente
func (s *Sorter) GetDiscardSalida() *shared.Salida {
	// Buscar salida que tenga SKU REJECT
	for i := range s.Salidas {
		for _, sku := range s.Salidas[i].SKUs_Actuales {
			if sku.SKU == "REJECT" {
				return &s.Salidas[i]
			}
		}
	}

	// Si no hay salida con REJECT, buscar primera salida manual
	for i := range s.Salidas {
		if s.Salidas[i].Tipo == "manual" {
			log.Printf("⚠️  Sorter #%d: Asignando salida manual '%s' (ID=%d) como descarte automático",
				s.ID, s.Salidas[i].Salida_Sorter, s.Salidas[i].ID)
			return &s.Salidas[i]
		}
	}

	// Último recurso: usar la última salida (no debería pasar)
	if len(s.Salidas) > 0 {
		log.Printf("🚨 Sorter #%d: ADVERTENCIA - No hay salida manual, usando última salida como descarte", s.ID)
		return &s.Salidas[len(s.Salidas)-1]
	}

	return nil
}

// StartSKUPublisher inicia un publicador periódico de SKUs (cada N segundos)
func (s *Sorter) StartSKUPublisher(intervalSeconds int, skuSource func() []models.SKUAssignable) {
	go func() {
		ticker := time.NewTicker(time.Duration(intervalSeconds) * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-s.ctx.Done():
				log.Printf("⏸️  Sorter #%d: Publicador de SKUs detenido", s.ID)
				return
			case <-ticker.C:
				skus := skuSource()
				s.UpdateSKUs(skus)
			}
		}
	}()
	log.Printf("⏰ Sorter #%d: Publicador de SKUs iniciado (cada %ds)", s.ID, intervalSeconds)
}
