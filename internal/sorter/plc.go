package sorter

import (
	"API-DANSORT/internal/shared"
	"log"
	"time"
)

// startPLCSubscriptions inicia UNA suscripción OPC UA para monitorear TODOS los nodos (ESTADO y BLOQUEO)
func (s *Sorter) startPLCSubscriptions() {
	log.Printf("🔔 Sorter #%d: Iniciando suscripción OPC UA compartida para Estado y Bloqueo...", s.ID)

	nodeIDs := make([]string, 0, len(s.Salidas)*2)
	nodeToSalidaMap := make(map[string]*shared.Salida)
	nodeToTypeMap := make(map[string]string)

	for i := range s.Salidas {
		salida := &s.Salidas[i]

		if salida.EstadoNode != "" {
			nodeIDs = append(nodeIDs, salida.EstadoNode)
			nodeToSalidaMap[salida.EstadoNode] = salida
			nodeToTypeMap[salida.EstadoNode] = "estado"
		}

		if salida.BloqueoNode != "" {
			nodeIDs = append(nodeIDs, salida.BloqueoNode)
			nodeToSalidaMap[salida.BloqueoNode] = salida
			nodeToTypeMap[salida.BloqueoNode] = "bloqueo"
		}
	}

	if len(nodeIDs) == 0 {
		log.Printf("⚠️  Sorter #%d: No hay nodos OPC UA configurados para monitorear", s.ID)
		return
	}

	log.Printf("📊 Sorter #%d: Monitoreando %d nodos en una sola suscripción", s.ID, len(nodeIDs))

	ctx := s.ctx
	interval := 100 * time.Millisecond

	dataChan, cancelFunc, err := s.plcManager.MonitorMultipleNodes(ctx, s.ID, nodeIDs, interval)
	if err != nil {
		log.Printf("❌ Sorter #%d: Error creando suscripción compartida: %v", s.ID, err)
		return
	}

	s.subscriptionMutex.Lock()
	s.cancelSubscriptions = append(s.cancelSubscriptions, cancelFunc)
	s.subscriptionMutex.Unlock()

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case nodeInfo, ok := <-dataChan:
				if !ok {
					log.Printf("⚠️  Sorter #%d: Canal de suscripción OPC UA cerrado", s.ID)
					return
				}

				salida, exists := nodeToSalidaMap[nodeInfo.NodeID]
				if !exists {
					continue
				}

				nodeType := nodeToTypeMap[nodeInfo.NodeID]

				if nodeType == "estado" {
					s.processEstadoChange(salida, nodeInfo.Value)
				} else if nodeType == "bloqueo" {
					s.processBloqueoChange(salida, nodeInfo.Value)
				}
			}
		}
	}()

	log.Printf("✅ Sorter #%d: Suscripción compartida iniciada para %d nodos", s.ID, len(nodeIDs))
}

// processEstadoChange procesa cambios en el estado de una salida
func (s *Sorter) processEstadoChange(salida *shared.Salida, value interface{}) {
	var estadoValor int16
	switch v := value.(type) {
	case int16:
		estadoValor = v
	case int32:
		estadoValor = int16(v)
	case int64:
		estadoValor = int16(v)
	case int:
		estadoValor = int16(v)
	default:
		log.Printf("⚠️ Sorter #%d: Tipo de valor inesperado para ESTADO de salida %d: %T",
			s.ID, salida.ID, value)
		return
	}

	estadoAnterior := salida.GetEstado()
	salida.SetEstado(estadoValor)

	if estadoAnterior != estadoValor {
		estadoNombre := "DESCONOCIDO"
		switch estadoValor {
		case 0:
			estadoNombre = "APAGADO"
		case 1:
			estadoNombre = "ANDANDO"
		case 2:
			estadoNombre = "FALLA"
		}
		log.Printf("🔄 Sorter #%d: Salida %d (%s) - ESTADO cambió: %d→%d (%s)",
			s.ID, salida.ID, salida.Salida_Sorter, estadoAnterior, estadoValor, estadoNombre)
	}
}

// processBloqueoChange procesa cambios en el bloqueo de una salida
func (s *Sorter) processBloqueoChange(salida *shared.Salida, value interface{}) {
	var bloqueoValor bool
	switch v := value.(type) {
	case bool:
		bloqueoValor = v
	case int16:
		bloqueoValor = v != 0
	case int32:
		bloqueoValor = v != 0
	case int64:
		bloqueoValor = v != 0
	case int:
		bloqueoValor = v != 0
	default:
		log.Printf("⚠️ Sorter #%d: Tipo de valor inesperado para BLOQUEO de salida %d: %T",
			s.ID, salida.ID, value)
		return
	}

	bloqueoAnterior := salida.GetBloqueo()
	salida.SetBloqueo(bloqueoValor)

	if bloqueoAnterior != bloqueoValor {
		estadoTexto := "DESBLOQUEADA"
		if bloqueoValor {
			estadoTexto = "BLOQUEADA"
		}
		log.Printf("🔒 Sorter #%d: Salida %d (%s) - BLOQUEO cambió: %t→%t (%s)",
			s.ID, salida.ID, salida.Salida_Sorter, bloqueoAnterior, bloqueoValor, estadoTexto)
	}
}

// stopPLCSubscriptions detiene todas las suscripciones OPC UA
func (s *Sorter) stopPLCSubscriptions() {
	s.subscriptionMutex.Lock()
	defer s.subscriptionMutex.Unlock()

	log.Printf("🔕 Sorter #%d: Deteniendo %d suscripciones OPC UA...", s.ID, len(s.cancelSubscriptions))

	for _, cancelFunc := range s.cancelSubscriptions {
		cancelFunc()
	}

	s.cancelSubscriptions = nil
}
