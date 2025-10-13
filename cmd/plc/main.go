package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"API-GREENEX/internal/communication/plc"
	"API-GREENEX/internal/config"

	"github.com/gopcua/opcua/ua"
)

func main() {
	log.Println("🔧 Iniciando programa de prueba OPC UA...")

	// --- 1. CARGAR CONFIGURACIÓN ---
	configPath := "config/config.yaml"
	if len(os.Args) > 1 {
		configPath = os.Args[1]
	}

	log.Printf("📄 Cargando configuración desde: %s", configPath)
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		log.Fatalf("❌ Error cargando configuración: %v", err)
	}

	log.Printf("✅ Configuración cargada: %d sorter(s) configurados", len(cfg.Sorters))

	// --- 2. CREAR MANAGER Y CONECTAR ---
	manager := plc.NewManager(cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := manager.ConnectAll(ctx); err != nil {
		log.Fatalf("❌ Error conectando a PLCs: %v", err)
	}
	defer manager.CloseAll(context.Background())

	// --- 3. LEER TODOS LOS NODOS ---
	log.Println("\n📊 Leyendo todos los nodos configurados...")

	sortersData, err := manager.ReadAllSorterNodes(ctx)
	if err != nil {
		log.Fatalf("❌ Error leyendo nodos: %v", err)
	}

	// --- 4. MOSTRAR RESULTADOS ---
	printResults(sortersData)

	// --- 5. INSPECCIONAR ESTRUCTURA DE MÉTODOS ---
	log.Println("\n🔍 Inspeccionando estructura de métodos...")
	inspectMethodStructure(ctx, manager, cfg)

	log.Println("\n✅ Programa finalizado exitosamente")
}

// printResults muestra los resultados en formato legible
func printResults(sortersData []plc.SorterNodes) {
	separator := strings.Repeat("═", 80)
	fmt.Println("\n" + separator)
	fmt.Println("                     ESTADO DE NODOS OPC UA")
	fmt.Println(separator)

	for _, sorter := range sortersData {
		printSorterSection(sorter)
	}

	fmt.Println(separator)
}

// printSorterSection imprime la sección de un sorter
func printSorterSection(sorter plc.SorterNodes) {
	fmt.Printf("\n╔═══ SORTER %d: %s ═══\n", sorter.SorterID, sorter.SorterName)
	fmt.Printf("║ Endpoint: %s\n", sorter.Endpoint)
	fmt.Println("╠═══════════════════════════════════════════════════════════════")

	// Nodos del PLC
	fmt.Println("║")
	fmt.Println("║ 🔌 NODOS DEL PLC:")
	if sorter.InputNode != nil {
		printNodeInfo("    Input ", sorter.InputNode)
	}
	if sorter.OutputNode != nil {
		printNodeInfo("    Output", sorter.OutputNode)
	}

	// Salidas
	if len(sorter.SalidaNodes) > 0 {
		fmt.Println("║")
		fmt.Println("║ 📦 SALIDAS:")
		for _, salida := range sorter.SalidaNodes {
			printSalidaSection(salida)
		}
	}

	fmt.Println("╚═══════════════════════════════════════════════════════════════")
}

// printSalidaSection imprime la información de una salida
func printSalidaSection(salida plc.SalidaNodes) {
	fmt.Printf("║\n")
	fmt.Printf("║   ┌─ Salida %d: %s (Physical ID: %d)\n",
		salida.SalidaID, salida.SalidaName, salida.PhysicalID)

	if salida.EstadoNode != nil {
		printNodeInfo("│     Estado ", salida.EstadoNode)
	}

	if salida.BloqueoNode != nil {
		printNodeInfo("│     Bloqueo", salida.BloqueoNode)
	} else {
		fmt.Println("║   │     Bloqueo: (no configurado)")
	}

	fmt.Printf("║   └─────────────────────────────────────────────\n")
}

// printNodeInfo imprime la información de un nodo
func printNodeInfo(prefix string, node *plc.NodeInfo) {
	if node.Error != nil {
		errorMsg := formatError(node.Error)
		fmt.Printf("║   %s: ⚠️  %s\n", prefix, errorMsg)
		fmt.Printf("║   %s  NodeID: %s\n", prefix, node.NodeID)
		return
	}

	// Formatear valor según tipo
	valueStr := formatValue(node.Value, node.ValueType)

	fmt.Printf("║   %s: %s\n", prefix, valueStr)
	fmt.Printf("║   %s  NodeID: %s | Tipo: %s\n",
		prefix, node.NodeID, node.ValueType)
}

// formatError formatea mensajes de error de forma concisa
func formatError(err error) string {
	errStr := err.Error()

	// Detectar tipos comunes de errores de OPC UA
	if strings.Contains(errStr, "StatusBadNotReadable") {
		return "Sin permisos de lectura"
	}
	if strings.Contains(errStr, "StatusBadNodeIDUnknown") {
		return "Nodo no existe"
	}
	if strings.Contains(errStr, "StatusBadNotConnected") {
		return "No conectado"
	}
	if strings.Contains(errStr, "StatusBadTimeout") {
		return "Timeout"
	}
	if strings.Contains(errStr, "StatusBad") {
		// Extraer solo el nombre del status si es muy largo
		if idx := strings.Index(errStr, "StatusBad"); idx != -1 {
			statusEnd := idx + 20
			if statusEnd > len(errStr) {
				statusEnd = len(errStr)
			}
			return errStr[idx:statusEnd]
		}
	}

	// Si el error es muy largo, truncar
	if len(errStr) > 40 {
		return errStr[:37] + "..."
	}

	return errStr
}

// formatValue formatea el valor según su tipo
func formatValue(value interface{}, valueType string) string {
	if value == nil {
		return "<nil>"
	}

	switch v := value.(type) {
	case bool:
		if v {
			return "✅ true"
		}
		return "❌ false"
	case int16:
		return fmt.Sprintf("🔢 %d", v)
	case int32:
		return fmt.Sprintf("🔢 %d", v)
	case int64:
		return fmt.Sprintf("🔢 %d", v)
	case float32:
		return fmt.Sprintf("📊 %.2f", v)
	case float64:
		return fmt.Sprintf("📊 %.2f", v)
	case string:
		return fmt.Sprintf("📝 \"%s\"", v)
	default:
		return fmt.Sprintf("%v", v)
	}
}

// inspectMethodStructure inspecciona la estructura de métodos OPC UA
func inspectMethodStructure(ctx context.Context, manager *plc.Manager, cfg *config.Config) {
	separator := strings.Repeat("═", 80)
	fmt.Println("\n" + separator)
	fmt.Println("                   ESTRUCTURA DE MÉTODOS OPC UA")
	fmt.Println(separator)

	for _, sorterCfg := range cfg.Sorters {
		fmt.Printf("\n╔═══ SORTER %d: %s ═══\n", sorterCfg.ID, sorterCfg.Name)

		// Verificar si tiene método configurado
		if sorterCfg.PLCInputNode == "" || sorterCfg.PLCOutputNode == "" {
			fmt.Println("║ ⚠️  No hay nodos Input/Output configurados")
			fmt.Println("╚═══════════════════════════════════════════════════════════════")
			continue
		}

		fmt.Printf("║ Input NodeID:  %s\n", sorterCfg.PLCInputNode)
		fmt.Printf("║ Output NodeID: %s\n", sorterCfg.PLCOutputNode)
		fmt.Println("║")

		// Leer InputArguments
		inputArgs, inputErr := readMethodArguments(ctx, cfg, manager, sorterCfg.ID, sorterCfg.PLCInputNode)
		if inputErr != nil {
			fmt.Printf("║ ❌ Error leyendo InputArguments: %s\n", formatError(inputErr))
		} else {
			fmt.Println("║ 📥 INPUT ARGUMENTS:")
			printArguments(inputArgs, "   ")
		}

		// Leer OutputArguments
		outputArgs, outputErr := readMethodArguments(ctx, cfg, manager, sorterCfg.ID, sorterCfg.PLCOutputNode)
		if outputErr != nil {
			fmt.Printf("║ ❌ Error leyendo OutputArguments: %s\n", formatError(outputErr))
		} else {
			fmt.Println("║")
			fmt.Println("║ 📤 OUTPUT ARGUMENTS:")
			printArguments(outputArgs, "   ")
		}

		if sorterCfg.PLCObjectID != "" && sorterCfg.PLCMethodID != "" {
			fmt.Println("║")
			fmt.Println("║ 🔧 CONFIGURACIÓN DE MÉTODO:")
			fmt.Printf("║    Object ID: %s\n", sorterCfg.PLCObjectID)
			fmt.Printf("║    Method ID: %s\n", sorterCfg.PLCMethodID)
		}

		fmt.Println("╚═══════════════════════════════════════════════════════════════")
	}

	fmt.Println(separator)
}

// readMethodArguments lee y decodifica los argumentos de un método
func readMethodArguments(ctx context.Context, cfg *config.Config, manager *plc.Manager, sorterID int, nodeID string) ([]*ua.Argument, error) {
	_ = cfg // No se usa en esta versión simplificada

	// Leer el nodo usando el manager
	result, err := manager.ReadNode(ctx, sorterID, nodeID)
	if err != nil {
		return nil, err
	}

	// Verificar tipos de argumentos
	var arguments []*ua.Argument

	// Caso 1: Array de ExtensionObjects (lo más común)
	if extArr, ok := result.Value.([]*ua.ExtensionObject); ok {
		for _, extObj := range extArr {
			if arg, ok := extObj.Value.(*ua.Argument); ok {
				arguments = append(arguments, arg)
			}
		}
		return arguments, nil
	}

	// Caso 2: Array de interfaces
	if arr, ok := result.Value.([]interface{}); ok {
		for _, item := range arr {
			if extObj, ok := item.(*ua.ExtensionObject); ok {
				if arg, ok := extObj.Value.(*ua.Argument); ok {
					arguments = append(arguments, arg)
				}
			}
		}
		return arguments, nil
	}

	// Caso 3: Un solo ExtensionObject
	if extObj, ok := result.Value.(*ua.ExtensionObject); ok {
		if arg, ok := extObj.Value.(*ua.Argument); ok {
			return []*ua.Argument{arg}, nil
		}
	}

	return nil, fmt.Errorf("valor no es ExtensionObject, es %T", result.Value)
}

// printArguments imprime la lista de argumentos
func printArguments(args []*ua.Argument, indent string) {
	if len(args) == 0 {
		fmt.Printf("║%s(ninguno)\n", indent)
		return
	}

	for _, arg := range args {
		typeStr := getTypeString(arg.DataType)
		valueRankStr := getValueRankString(arg.ValueRank)

		fmt.Printf("║%s• %s (%s, %s)\n", indent, arg.Name, typeStr, valueRankStr)
		if arg.Description.Text != "" {
			fmt.Printf("║%s  Descripción: %s\n", indent, arg.Description.Text)
		}
	}
}

// getTypeString convierte un NodeID de tipo a string legible
func getTypeString(typeID *ua.NodeID) string {
	if typeID == nil {
		return "desconocido"
	}

	// Mapeo de tipos comunes de OPC UA
	typeMap := map[uint32]string{
		1:  "Boolean",
		2:  "SByte",
		3:  "Byte",
		4:  "Int16",
		5:  "UInt16",
		6:  "Int32",
		7:  "UInt32",
		8:  "Int64",
		9:  "UInt64",
		10: "Float",
		11: "Double",
		12: "String",
		13: "DateTime",
		15: "Guid",
		20: "LocalizedText",
	}

	if typeID.Type() == ua.NodeIDTypeNumeric {
		if typeName, ok := typeMap[typeID.IntID()]; ok {
			return typeName
		}
		return fmt.Sprintf("Type%d", typeID.IntID())
	}

	return typeID.String()
}

// getValueRankString convierte el ValueRank a string legible
func getValueRankString(valueRank int32) string {
	switch valueRank {
	case -3:
		return "escalar o array"
	case -2:
		return "escalar o array de cualquier dimensión"
	case -1:
		return "escalar"
	case 0:
		return "array de cualquier dimensión"
	case 1:
		return "array unidimensional"
	default:
		return fmt.Sprintf("array de %d dimensiones", valueRank)
	}
}
