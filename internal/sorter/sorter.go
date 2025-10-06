package sorter

import (
	"API-GREENEX/internal/listeners"
	"API-GREENEX/internal/models"
	"context"
	"log"
)

type Sorter struct {
	ID        int                       `json:"id"`
	Ubicacion string                    `json:"ubicacion"`
	Salidas   []Salida                  `json:"salidas"`
	Cognex    *listeners.CognexListener `json:"cognex"`
	ctx       context.Context
	cancel    context.CancelFunc
	// Estadísticas
	LecturasExitosas int
	LecturasFallidas int
}

func GetNewSorter(ID int, ubicacion string, salidas []Salida, cognex *listeners.CognexListener) *Sorter {
	ctx, cancel := context.WithCancel(context.Background())
	return &Sorter{
		ID:        ID,
		Ubicacion: ubicacion,
		Salidas:   salidas,
		Cognex:    cognex,
		ctx:       ctx,
		cancel:    cancel,
	}
}

func (s *Sorter) Start() error {
	log.Printf("🚀 Iniciando Sorter #%d en %s", s.ID, s.Ubicacion)
	log.Printf("📍 Salidas configuradas: %d", len(s.Salidas))

	for _, salida := range s.Salidas {
		log.Printf("  ↳ Salida %d: %s", salida.ID, salida.Salida_Sorter)
	}

	// Iniciar el listener de Cognex
	err := s.Cognex.Start()
	if err != nil {
		return err
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
				log.Printf("✅ [Sorter] Lectura #%d | %s", s.LecturasExitosas, evento.String())

				// Determinar salida
				salida := s.determinarSalida(evento.SKU, evento.Calibre)
				log.Printf("📤 [Sorter] → Salida #%d (%s) | Calibre: %s",
					salida.ID, salida.Salida_Sorter, evento.Calibre)

				// TODO: Activar actuadores/PLC para enviar a la salida

			} else {
				s.LecturasFallidas++
				log.Printf("❌ [Sorter] Fallo #%d | Tipo: %s | %s",
					s.LecturasFallidas, tipoLectura, evento.String())

				// Manejar según tipo de error
				switch tipoLectura {
				case models.LecturaNoRead:
					log.Printf("⚠️  [Sorter] NO_READ detectado - Continuando...")
				case models.LecturaFormato:
					log.Printf("⚠️  [Sorter] Error de formato - Enviando a rechazo")
					// TODO: Enviar a salida de rechazo
				case models.LecturaSKU:
					log.Printf("⚠️  [Sorter] SKU inválido - Enviando a rechazo")
					// TODO: Enviar a salida de rechazo
				case models.LecturaDB:
					log.Printf("⚠️  [Sorter] Error de base de datos - Alerta")
					// TODO: Generar alerta al operador
				}
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
func (s *Sorter) determinarSalida(sku, calibre string) Salida {
	// Lógica de ejemplo: asignar por calibre
	switch calibre {
	case "XL", "XLD", "2XL":
		if len(s.Salidas) > 0 {
			return s.Salidas[0]
		}
	case "2J", "3J", "4J":
		if len(s.Salidas) > 1 {
			return s.Salidas[1]
		}
	default:
		if len(s.Salidas) > 2 {
			return s.Salidas[2]
		}
	}

	// Fallback a primera salida
	return s.Salidas[0]
}

func (s *Sorter) Stop() error {
	log.Printf("🛑 Deteniendo Sorter #%d", s.ID)
	s.cancel()

	if s.Cognex != nil {
		return s.Cognex.Stop()
	}

	return nil
}
