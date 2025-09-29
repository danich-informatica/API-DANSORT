package flow

import (
	"log"
	"time"
	"API-GREENEX/internal/listeners"
	"API-GREENEX/internal/models"
	"API-GREENEX/internal/shared"
	"github.com/gopcua/opcua/ua"
)

const WAGO_LOOP_INTERVAL_MS = 10000

// WagoLoop ejecuta escritura y lectura periódica sobre los nodos WAGO
func WagoLoop(service *listeners.OPCUAService) {
	for service.IsConnected() {
		// Escritura en nodos escalares
		   // Generar valores válidos y del tipo exacto
		   boleano := (time.Now().UnixNano()%2 == 0)
		   byteVal := uint8(time.Now().UnixNano() % 256)
		   entero := int16(time.Now().UnixNano() % 32768)
		   realVal := float32(time.Now().UnixNano()%10000) / 100.0
		   strVal := "LoopTest"
		   wordVal := uint16(time.Now().UnixNano() % 65536)

		   // Escritura en nodos escalares con tipo explícito
		   writeOps := []struct {
			   nodeID string
			   value  interface{}
		   }{
	           {models.WAGO_BoleanoTest, ua.MustVariant(shared.CastWagoValue(models.WAGO_BoleanoTest, boleano))},
	           {models.WAGO_ByteTest, ua.MustVariant(shared.CastWagoValue(models.WAGO_ByteTest, byteVal))},
	           {models.WAGO_EnteroTest, ua.MustVariant(shared.CastWagoValue(models.WAGO_EnteroTest, entero))},
	           {models.WAGO_RealTest, ua.MustVariant(shared.CastWagoValue(models.WAGO_RealTest, realVal))},
	           {models.WAGO_StringTest, ua.MustVariant(shared.CastWagoValue(models.WAGO_StringTest, strVal))},
	           {models.WAGO_WordTest, ua.MustVariant(shared.CastWagoValue(models.WAGO_WordTest, wordVal))},
		   }
		   for _, op := range writeOps {
			   // Diagnóstico: leer el tipo de dato del nodo antes de escribir
			   dataType, errType := service.ReadNodeDataType(op.nodeID)
			   if errType != nil {
				   log.Printf("[DIAG] Nodo: %s | ERROR leyendo tipo de dato: %v", op.nodeID, errType)
			   } else {
				   log.Printf("[DIAG] Nodo: %s | Tipo de dato esperado (DataType): %v", op.nodeID, dataType)
			   }

			   err := service.WriteNode(op.nodeID, op.value)
			   if err != nil {
				   log.Printf("[WRITE] Nodo: %s | Valor: %v | ERROR: %v", op.nodeID, op.value, err)
			   } else {
				   log.Printf("[WRITE] Nodo: %s | Valor: %v | OK", op.nodeID, op.value)
			   }
		   }

		// Lectura de nodos vectoriales
		vectorNodes := []string{
			models.WAGO_VectorBool,
			models.WAGO_VectorInt,
			models.WAGO_VectorWord,
		}
		for _, nodeID := range vectorNodes {
			nodeData, err := service.ReadNode(nodeID)
			if err != nil {
				log.Printf("[READ] Nodo: %s | ERROR: %v", nodeID, err)
			} else {
				log.Printf("[READ] Nodo: %s | Valor: %v | Calidad: %v", nodeID, nodeData.Value, nodeData.Quality)
			}
		}

		time.Sleep(WAGO_LOOP_INTERVAL_MS * time.Millisecond)
	}
}
