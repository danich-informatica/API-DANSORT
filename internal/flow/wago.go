package flow

import (
	"API-GREENEX/internal/listeners"
	"API-GREENEX/internal/models"
	"API-GREENEX/internal/shared"
	"log"
	"time"

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

		// Escritura en nodos escalares con tipo explícito (sin doble variant)
		writeOps := []struct {
			nodeID string
			value  interface{}
		}{
			{models.WAGO_BoleanoTest, shared.CastWagoValue(models.WAGO_BoleanoTest, boleano)},
			{models.WAGO_ByteTest, shared.CastWagoValue(models.WAGO_ByteTest, byteVal)},
			{models.WAGO_EnteroTest, shared.CastWagoValue(models.WAGO_EnteroTest, entero)},
			{models.WAGO_RealTest, shared.CastWagoValue(models.WAGO_RealTest, realVal)},
			{models.WAGO_StringTest, shared.CastWagoValue(models.WAGO_StringTest, strVal)},
			{models.WAGO_WordTest, shared.CastWagoValue(models.WAGO_WordTest, wordVal)},
		}
		for _, op := range writeOps {
			// Diagnóstico detallado: leer el tipo de dato del nodo antes de escribir
			dataType, errType := service.ReadNodeDataType(op.nodeID)
			if errType != nil {
				log.Printf("┌─────────────────────────────────────────────────────────")
				log.Printf("│ 📝 ESCRITURA WAGO")
				log.Printf("│ Nodo: %s", op.nodeID)
				log.Printf("│ ⚠️  ERROR leyendo DataType: %v", errType)
				log.Printf("└─────────────────────────────────────────────────────────")
			} else {
				// Extraer detalles del DataType
				dataTypeNodeID, ok := dataType.(*ua.NodeID)
				var dataTypeInfo string
				if ok {
					dataTypeInfo = "Namespace=" + string(rune(dataTypeNodeID.Namespace())) + ", ID=" + string(rune(dataTypeNodeID.IntID()))
				} else {
					dataTypeInfo = "Desconocido"
				}

				log.Printf("┌─────────────────────────────────────────────────────────")
				log.Printf("│ 📝 ESCRITURA WAGO")
				log.Printf("│ Nodo: %s", op.nodeID)
				log.Printf("│ Tipo esperado (DataType): %v", dataType)
				log.Printf("│ DataType detalle: %s", dataTypeInfo)
				log.Printf("│ Valor a escribir: %v", op.value)
				log.Printf("│ Tipo Go del valor: %T", op.value)
				log.Printf("└─────────────────────────────────────────────────────────")
			}

			err := service.WriteNode(op.nodeID, op.value)
			if err != nil {
				log.Printf("❌ FALLO | Nodo: %s | Valor: %v (%T) | Error: %v", op.nodeID, op.value, op.value, err)
			} else {
				log.Printf("✅ ÉXITO | Nodo: %s | Valor: %v (%T)", op.nodeID, op.value, op.value)
			}
		}

		// Lectura de nodos vectoriales (sin logs)

		time.Sleep(WAGO_LOOP_INTERVAL_MS * time.Millisecond)
	}
}
