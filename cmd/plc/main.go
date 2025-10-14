package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"

	"API-GREENEX/internal/communication/plc"
	"API-GREENEX/internal/config"

	"github.com/gopcua/opcua/ua"
)

// MenuNodeInfo es una estructura para facilitar la selección de nodos en el menú.
type MenuNodeInfo struct {
	SorterID   int
	NodeID     string
	Name       string
	SorterName string
}

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

	// --- BUCLE INTERACTIVO ---
	interactiveMenu(manager, cfg)

	log.Println("\n✅ Programa finalizado exitosamente")
}

// interactiveMenu muestra el menú principal y gestiona las acciones del usuario.
func interactiveMenu(manager *plc.Manager, cfg *config.Config) {
	reader := bufio.NewReader(os.Stdin)
	nodes := getAllNodesForMenu(cfg)

	for {
		fmt.Println("\n" + strings.Repeat("═", 40))
		fmt.Println("          MENÚ INTERACTIVO OPC UA")
		fmt.Println(strings.Repeat("─", 40))
		fmt.Println("  1. Leer un nodo específico")
		fmt.Println("  2. Escribir en un nodo específico")
		fmt.Println("  3. Mostrar estado de todos los nodos")
		fmt.Println("  4. Agregar elemento a array (Input/Output)")
		fmt.Println("  5. Explorar nodos (Browse)")
		fmt.Println("  6. Llamar método OPC UA")
		fmt.Println("  7. Ver estadísticas de cache")
		fmt.Println("  8. Monitorear nodo en tiempo real (Suscripción)")
		fmt.Println("  9. Salir")
		fmt.Println(strings.Repeat("═", 40))
		fmt.Print("Seleccione una opción: ")

		input, _ := reader.ReadString('\n')
		choice, err := strconv.Atoi(strings.TrimSpace(input))
		if err != nil {
			log.Println("⚠️ Opción inválida. Por favor, ingrese un número.")
			continue
		}

		switch choice {
		case 1:
			handleReadNode(manager, reader, nodes)
		case 2:
			handleWriteNode(manager, reader, nodes)
		case 3:
			handleReadAllNodes(manager)
		case 4:
			handleAppendToArray(manager, reader, nodes)
		case 5:
			handleBrowseNode(manager, reader)
		case 6:
			handleCallMethod(manager, reader)
		case 7:
			handleCacheStats(manager)
		case 8:
			handleMonitorNode(manager, reader, nodes)
		case 9:
			return
		default:
			log.Println("⚠️ Opción no válida.")
		}
	}
}

// handleReadNode gestiona la lectura de un nodo seleccionado por el usuario.
func handleReadNode(plcManager *plc.Manager, reader *bufio.Reader, nodes []MenuNodeInfo) {
	selectedNode, err := selectNode(reader, nodes)
	if err != nil {
		if err.Error() != "back" {
			log.Printf("❌ Error al seleccionar el nodo: %v", err)
		}
		return
	}

	log.Printf("🔄 Leyendo nodo '%s' (%s)...", selectedNode.Name, selectedNode.NodeID)
	readCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := plcManager.ReadNode(readCtx, selectedNode.SorterID, selectedNode.NodeID)
	if err != nil {
		log.Printf("❌ Error al leer el nodo: %v", err)
		return
	}

	valueStr := formatValue(result.Value, result.Error)
	fmt.Printf("\n✅ Lectura exitosa de '%s':\n", selectedNode.Name)
	fmt.Printf("   - Valor: %s\n", valueStr)
	fmt.Printf("   - Tipo:  %s\n", result.ValueType)
}

// handleWriteNode gestiona la escritura en un nodo seleccionado por el usuario.
func handleWriteNode(plcManager *plc.Manager, reader *bufio.Reader, nodes []MenuNodeInfo) {
	selectedNode, err := selectNode(reader, nodes)
	if err != nil {
		if err.Error() != "back" {
			log.Printf("❌ Error al seleccionar el nodo: %v", err)
		}
		return
	}

	log.Printf("🔄 Obteniendo tipo de dato del nodo '%s'...", selectedNode.Name)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var varType ua.TypeID
	var valueTypeString string

	// Intenta leer el nodo para determinar su tipo.
	nodeInfo, err := plcManager.ReadNode(ctx, selectedNode.SorterID, selectedNode.NodeID)
	if err != nil {
		// Si la lectura falla porque el nodo no es legible (común en nodos de solo escritura).
		if strings.Contains(err.Error(), ua.StatusBadNotReadable.Error()) {
			// Ofrecer opciones de tipo común
			fmt.Println("\n⚠️  El nodo no es legible (solo escritura).")
			fmt.Println("Seleccione el tipo de dato:")
			fmt.Println("  1. bool")
			fmt.Println("  2. int16")
			fmt.Println("  3. uint16")
			fmt.Println("  4. int32")
			fmt.Println("  5. uint32")
			fmt.Println("  6. float32")
			fmt.Println("  7. string")
			fmt.Print("Opción: ")

			typeChoice, _ := reader.ReadString('\n')
			typeChoice = strings.TrimSpace(typeChoice)

			switch typeChoice {
			case "1":
				valueTypeString = "bool"
			case "2":
				valueTypeString = "int16"
			case "3":
				valueTypeString = "uint16"
			case "4":
				valueTypeString = "int32"
			case "5":
				valueTypeString = "uint32"
			case "6":
				valueTypeString = "float32"
			case "7":
				valueTypeString = "string"
			default:
				log.Println("❌ Tipo no válido. Operación cancelada.")
				return
			}
		} else {
			// Otro tipo de error durante la lectura.
			log.Printf("❌ No se pudo determinar el tipo del nodo debido a un error de lectura: %v", err)
			return
		}
	} else {
		varType = nodeInfo.DataType
		valueTypeString = nodeInfo.ValueType
		log.Printf("✅ Tipo de dato obtenido: %s", valueTypeString)
	}

	// Validar si es escribible (solo si tenemos varType)
	if valueTypeString == "" && !isWritableType(varType) {
		log.Printf("❌ El tipo de dato '%s' del nodo '%s' no es soportado para escritura.", valueTypeString, selectedNode.Name)
		return
	}

	fmt.Printf("➡️  Ingrese el valor a escribir para el nodo '%s' (tipo: %s): ", selectedNode.Name, valueTypeString)
	text, _ := reader.ReadString('\n')
	text = strings.TrimSpace(text)

	ctxWrite, cancelWrite := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelWrite()

	// Usar WriteNodeTyped con conversión explícita
	err = plcManager.WriteNodeTyped(ctxWrite, selectedNode.SorterID, selectedNode.NodeID, text, valueTypeString)
	if err != nil {
		log.Printf("❌ Error al escribir en el nodo: %v", err)
	} else {
		log.Printf("✅ Escritura exitosa en el nodo '%s'.", selectedNode.Name)
	}
}

func handleReadAllNodes(plcManager *plc.Manager) {
	log.Println("🔄 Leyendo el estado de todos los nodos (lectura masiva)...")
	var builder strings.Builder

	separator := strings.Repeat("═", 100)
	builder.WriteString("\n" + separator + "\n")
	builder.WriteString(fmt.Sprintf("%-100s\n", fmt.Sprintf("%*s%s", (100-26)/2, "", "ESTADO DE NODOS OPC UA")))
	builder.WriteString(separator + "\n")

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second) // Aumentado a 20s
	defer cancel()

	allNodesData, err := plcManager.ReadAllSorterNodes(ctx)
	if err != nil {
		log.Printf("❌ Error al leer todos los nodos: %v", err)
		return
	}

	for _, sorterData := range allNodesData {
		builder.WriteString(fmt.Sprintf("\n╔═══ SORTER %d: %s (%s) ═══\n", sorterData.SorterID, sorterData.SorterName, sorterData.Endpoint))
		builder.WriteString("╠" + strings.Repeat("─", 99) + "\n")

		builder.WriteString("║ 🔌 NODOS PRINCIPALES:\n")
		if len(sorterData.Nodes) > 0 {
			for _, nodeData := range sorterData.Nodes {
				builder.WriteString(fmt.Sprintf("║   - %-8s: %-30s (ID: %s)\n", nodeData.Name, formatValue(nodeData.Value, nodeData.Error), nodeData.NodeID))
			}
		} else {
			builder.WriteString("║   (No hay nodos principales configurados)\n")
		}
		builder.WriteString("║\n")

		builder.WriteString("║ 📦 SALIDAS:\n")
		if len(sorterData.Salidas) > 0 {
			for _, salidaData := range sorterData.Salidas {
				builder.WriteString(fmt.Sprintf("║   - Salida %d: %-20s (PhysicalID: %d)\n", salidaData.ID, salidaData.Name, salidaData.PhysicalID))
				for _, nodeData := range salidaData.Nodes {
					builder.WriteString(fmt.Sprintf("║     └ %-8s: %-30s (ID: %s)\n", nodeData.Name, formatValue(nodeData.Value, nodeData.Error), nodeData.NodeID))
				}
			}
		} else {
			builder.WriteString("║   (No hay salidas configuradas)\n")
		}
		builder.WriteString("╚" + strings.Repeat("═", 99) + "\n")
	}

	fmt.Println(builder.String())
}

// formatValue formatea el valor para una visualización clara.
func formatValue(value interface{}, err error) string {
	if err != nil {
		// Formateo conciso de errores comunes
		errStr := err.Error()
		if strings.Contains(errStr, ua.StatusBadNotReadable.Error()) {
			return "⚠️  (Sin permiso de lectura)"
		}
		if strings.Contains(errStr, ua.StatusBadNodeIDUnknown.Error()) {
			return "⚠️  (Nodo no encontrado)"
		}
		return fmt.Sprintf("⚠️  %s", errStr)
	}
	if value == nil {
		return "null"
	}

	switch v := value.(type) {
	case *ua.ExtensionObject:
		if v == nil {
			return "null"
		}
		jsonBytes, err := json.MarshalIndent(v.Value, "", "  ")
		if err != nil {
			return fmt.Sprintf(`{"error": "no se pudo serializar a JSON: %v"}`, err)
		}
		return string(jsonBytes)
	case []*ua.ExtensionObject:
		if len(v) == 0 {
			return "[]"
		}
		var decodedObjects []interface{}
		for _, extObj := range v {
			if extObj != nil {
				decodedObjects = append(decodedObjects, extObj.Value)
			}
		}
		finalJSON, err := json.MarshalIndent(decodedObjects, "", "  ")
		if err != nil {
			return fmt.Sprintf(`[{"error": "no se pudo serializar a JSON: %v"}]`, err)
		}
		return string(finalJSON)
	default:
		return fmt.Sprintf("%v", v)
	}
}

func isWritableType(varType ua.TypeID) bool {
	switch varType {
	case ua.TypeIDBoolean, ua.TypeIDInt16, ua.TypeIDUint16, ua.TypeIDInt32, ua.TypeIDUint32, ua.TypeIDInt64, ua.TypeIDUint64, ua.TypeIDFloat, ua.TypeIDDouble, ua.TypeIDString, ua.TypeIDByte, ua.TypeIDSByte:
		return true
	default:
		return false
	}
}

func parseValue(valueStr string, nodeType string) (interface{}, error) {
	// nodeType puede venir como "Boolean" o "boolean", normalizamos
	switch strings.ToLower(nodeType) {
	case "boolean":
		return strconv.ParseBool(valueStr)
	case "int16":
		v, err := strconv.ParseInt(valueStr, 10, 16)
		return int16(v), err
	case "uint16":
		v, err := strconv.ParseUint(valueStr, 10, 16)
		return uint16(v), err
	case "int32":
		v, err := strconv.ParseInt(valueStr, 10, 32)
		return int32(v), err
	case "uint32":
		v, err := strconv.ParseUint(valueStr, 10, 32)
		return uint32(v), err
	case "int64":
		return strconv.ParseInt(valueStr, 10, 64)
	case "uint64":
		return strconv.ParseUint(valueStr, 10, 64)
	case "float", "float32":
		v, err := strconv.ParseFloat(valueStr, 32)
		return float32(v), err
	case "double", "float64":
		return strconv.ParseFloat(valueStr, 64)
	case "string":
		return valueStr, nil
	case "byte":
		v, err := strconv.ParseUint(valueStr, 10, 8)
		return byte(v), err
	case "sbyte":
		v, err := strconv.ParseInt(valueStr, 10, 8)
		return int8(v), err
	default:
		return nil, fmt.Errorf("tipo de dato '%s' no soportado para escritura", nodeType)
	}
}

func getAllNodesForMenu(cfg *config.Config) []MenuNodeInfo {
	var nodes []MenuNodeInfo
	for _, sorter := range cfg.Sorters {
		if sorter.PLC.InputNodeID != "" {
			nodes = append(nodes, MenuNodeInfo{
				SorterID:   sorter.ID,
				NodeID:     sorter.PLC.InputNodeID,
				Name:       fmt.Sprintf("%s - Input", sorter.Name),
				SorterName: sorter.Name,
			})
		}
		if sorter.PLC.OutputNodeID != "" {
			nodes = append(nodes, MenuNodeInfo{
				SorterID:   sorter.ID,
				NodeID:     sorter.PLC.OutputNodeID,
				Name:       fmt.Sprintf("%s - Output", sorter.Name),
				SorterName: sorter.Name,
			})
		}
		// Añadir nodos de las salidas
		for _, salida := range sorter.Salidas {
			if salida.PLC.EstadoNodeID != "" {
				nodes = append(nodes, MenuNodeInfo{
					SorterID:   sorter.ID,
					NodeID:     salida.PLC.EstadoNodeID,
					Name:       fmt.Sprintf("%s - Estado", salida.Nombre),
					SorterName: sorter.Name,
				})
			}
			if salida.PLC.BloqueoNodeID != "" {
				nodes = append(nodes, MenuNodeInfo{
					SorterID:   sorter.ID,
					NodeID:     salida.PLC.BloqueoNodeID,
					Name:       fmt.Sprintf("%s - Bloqueo", salida.Nombre),
					SorterName: sorter.Name,
				})
			}
		}
	}
	return nodes
}

func selectNode(reader *bufio.Reader, nodes []MenuNodeInfo) (MenuNodeInfo, error) {
	fmt.Println("\n" + strings.Repeat("─", 50))
	fmt.Printf("%-50s\n", fmt.Sprintf("%*s%s", (50-18)/2, "", "SELECCIONE UN NODO"))
	fmt.Println(strings.Repeat("─", 50))

	sorterGroups := make(map[string][]MenuNodeInfo)
	var sorterOrder []string
	sorterNameMap := make(map[string]bool)

	for _, node := range nodes {
		if !sorterNameMap[node.SorterName] {
			sorterOrder = append(sorterOrder, node.SorterName)
			sorterNameMap[node.SorterName] = true
		}
		sorterGroups[node.SorterName] = append(sorterGroups[node.SorterName], node)
	}

	var flatNodeList []MenuNodeInfo
	nodeCounter := 1
	for _, sorterName := range sorterOrder {
		fmt.Printf("\n--- %s ---\n", sorterName)
		groupNodes := sorterGroups[sorterName]
		for _, node := range groupNodes {
			fmt.Printf("  %d. %s (%s)\n", nodeCounter, node.Name, node.NodeID)
			flatNodeList = append(flatNodeList, node)
			nodeCounter++
		}
	}

	fmt.Println(strings.Repeat("─", 50))
	fmt.Print("Seleccione un número de nodo (o 'b' para volver): ")

	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)

	if strings.ToLower(input) == "b" {
		return MenuNodeInfo{}, fmt.Errorf("back")
	}

	choice, err := strconv.Atoi(input)
	if err != nil || choice < 1 || choice > len(flatNodeList) {
		return MenuNodeInfo{}, fmt.Errorf("selección inválida")
	}

	return flatNodeList[choice-1], nil
}

// handleAppendToArray gestiona agregar un elemento a un array de ExtensionObject
func handleAppendToArray(plcManager *plc.Manager, reader *bufio.Reader, nodes []MenuNodeInfo) {
	// Filtrar solo nodos de tipo Input/Output (que contienen arrays)
	var arrayNodes []MenuNodeInfo
	for _, node := range nodes {
		if strings.Contains(node.Name, "Input") || strings.Contains(node.Name, "Output") {
			arrayNodes = append(arrayNodes, node)
		}
	}

	if len(arrayNodes) == 0 {
		log.Println("❌ No hay nodos de tipo array (Input/Output) disponibles")
		return
	}

	fmt.Println("\n" + strings.Repeat("─", 50))
	fmt.Println("      SELECCIONE NODO DE ARRAY (Input/Output)")
	fmt.Println(strings.Repeat("─", 50))

	for i, node := range arrayNodes {
		fmt.Printf("  %d. %s (%s)\n", i+1, node.Name, node.NodeID)
	}

	fmt.Println(strings.Repeat("─", 50))
	fmt.Print("Seleccione un número (o 'b' para volver): ")

	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)

	if strings.ToLower(input) == "b" {
		return
	}

	choice, err := strconv.Atoi(input)
	if err != nil || choice < 1 || choice > len(arrayNodes) {
		log.Println("❌ Selección inválida")
		return
	}

	selectedNode := arrayNodes[choice-1]

	// Primero leer el array actual para detectar el tipo
	log.Printf("🔄 Leyendo estructura del array '%s'...", selectedNode.Name)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	nodeInfo, err := plcManager.ReadNode(ctx, selectedNode.SorterID, selectedNode.NodeID)
	if err != nil {
		log.Printf("❌ Error al leer el array: %v", err)
		return
	}

	log.Printf("🔍 Tipo detectado: %T", nodeInfo.Value)

	// Detectar el tipo de array y procesar según corresponda
	switch arr := nodeInfo.Value.(type) {
	case []uint32:
		handleAppendUInt32(plcManager, reader, selectedNode, arr)
	case []*ua.ExtensionObject:
		handleAppendExtensionObject(plcManager, reader, selectedNode, arr)
	default:
		log.Printf("❌ Tipo de array no soportado: %T", nodeInfo.Value)
	}
}

func handleAppendUInt32(plcManager *plc.Manager, reader *bufio.Reader, selectedNode MenuNodeInfo, currentArray []uint32) {
	if len(currentArray) == 0 {
		log.Printf("❌ El array está vacío, no hay plantilla disponible")
		return
	}

	log.Printf("📋 Array de uint32 con %d elemento(s)", len(currentArray))
	log.Printf("💡 Valores actuales: %v", currentArray)

	// Solicitar el nuevo valor
	fmt.Print("\n  Ingrese el nuevo valor uint32: ")
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)

	value, err := strconv.ParseUint(input, 10, 32)
	if err != nil {
		log.Printf("❌ Valor inválido: %v", err)
		return
	}

	newValue := uint32(value)
	log.Printf("➕ Agregando valor %d al array '%s'...", newValue, selectedNode.Name)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err = plcManager.AppendToUInt32Array(ctx, selectedNode.SorterID, selectedNode.NodeID, newValue)
	if err != nil {
		log.Printf("❌ Error al agregar elemento: %v", err)
		return
	}

	log.Printf("✅ Valor %d agregado exitosamente al array '%s'", newValue, selectedNode.Name)
}

func handleAppendExtensionObject(plcManager *plc.Manager, reader *bufio.Reader, selectedNode MenuNodeInfo, currentArray []*ua.ExtensionObject) {
	if len(currentArray) == 0 {
		log.Printf("❌ El array está vacío, no hay plantilla disponible")
		return
	}

	// Usar el primer elemento como plantilla
	template := currentArray[0]
	log.Printf("✅ Plantilla obtenida del primer elemento del array")

	// Mostrar la estructura JSON para debug
	if jsonBytes, err := json.Marshal(template.Value); err == nil {
		log.Printf("🔍 Estructura plantilla: %s", string(jsonBytes))
	}
	log.Printf("🔍 Tipo de Value: %T", template.Value)

	// Solicitar solo el nombre del nuevo elemento
	fmt.Println("\n📝 Ingrese los datos del nuevo elemento:")
	fmt.Print("  Nombre (ej: 'OUT2', 'IN2'): ")
	name, _ := reader.ReadString('\n')
	name = strings.TrimSpace(name)

	if name == "" {
		log.Println("❌ El nombre no puede estar vacío")
		return
	}

	// Intentar clonar usando reflexión para preservar tipos
	templateValue := reflect.ValueOf(template.Value)

	// Si es un puntero, necesitamos trabajar con el elemento apuntado
	var newValue interface{}
	if templateValue.Kind() == reflect.Ptr && !templateValue.IsNil() {
		// Crear una nueva instancia del mismo tipo
		elemType := templateValue.Elem().Type()
		newValueReflect := reflect.New(elemType)

		// Copiar todos los campos del original
		newValueReflect.Elem().Set(templateValue.Elem())

		// Intentar modificar el campo Name
		nameField := newValueReflect.Elem().FieldByName("Name")
		if nameField.IsValid() && nameField.CanSet() && nameField.Kind() == reflect.String {
			nameField.SetString(name)
			log.Printf("✅ Nombre actualizado a: %s", name)
		} else {
			log.Printf("⚠️ No se pudo modificar el campo Name")
		}

		newValue = newValueReflect.Interface()
	} else {
		// Fallback: usar el valor original
		log.Printf("⚠️ Tipo no es puntero o es nil, usando valor original")
		newValue = template.Value
	}

	// Crear nuevo ExtensionObject con el valor clonado
	newElement := &ua.ExtensionObject{
		TypeID: template.TypeID,
		Value:  newValue,
	}

	log.Printf("➕ Agregando elemento '%s' al nodo '%s'...", name, selectedNode.Name)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := plcManager.AppendToArrayNode(ctx, selectedNode.SorterID, selectedNode.NodeID, newElement)
	if err != nil {
		log.Printf("❌ Error al agregar elemento al array: %v", err)
		return
	}

	log.Printf("✅ Elemento agregado exitosamente al array '%s'", selectedNode.Name)
}

// handleBrowseNode explora los nodos hijos de un nodo específico
func handleBrowseNode(plcManager *plc.Manager, reader *bufio.Reader) {
	// Seleccionar sorter
	fmt.Println("\n" + strings.Repeat("─", 50))
	fmt.Println("      EXPLORAR NODOS - SELECCIONE SORTER")
	fmt.Println(strings.Repeat("─", 50))
	fmt.Println("  1. Sorter Principal (ID: 1)")
	fmt.Println("  2. Sorter Secundario (ID: 2)")
	fmt.Println(strings.Repeat("─", 50))
	fmt.Print("Seleccione sorter (o 'b' para volver): ")

	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)

	if strings.ToLower(input) == "b" {
		return
	}

	choice, err := strconv.Atoi(input)
	if err != nil || choice < 1 || choice > 2 {
		log.Println("❌ Selección inválida")
		return
	}

	sorterID := choice

	// Solicitar NodeID a explorar
	fmt.Print("\nIngrese NodeID a explorar (ej: ns=4;i=1, ns=4;i=21): ")
	nodeID, _ := reader.ReadString('\n')
	nodeID = strings.TrimSpace(nodeID)

	if nodeID == "" {
		log.Println("❌ NodeID no puede estar vacío")
		return
	}

	log.Printf("🔄 Explorando nodo '%s' en Sorter %d...", nodeID, sorterID)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	results, err := plcManager.BrowseNode(ctx, sorterID, nodeID)
	if err != nil {
		log.Printf("❌ Error al explorar nodo: %v", err)
		return
	}

	if len(results) == 0 {
		log.Println("ℹ️ No se encontraron nodos hijos")
		return
	}

	// Mostrar resultados en tabla
	fmt.Println("\n" + strings.Repeat("═", 120))
	fmt.Printf("%-40s %-30s %-20s %-15s\n", "NodeID", "BrowseName", "DisplayName", "NodeClass")
	fmt.Println(strings.Repeat("─", 120))

	for _, result := range results {
		nodeClassStr := nodeClassToString(result.NodeClass)
		fmt.Printf("%-40s %-30s %-20s %-15s\n",
			truncate(result.NodeID, 40),
			truncate(result.BrowseName, 30),
			truncate(result.DisplayName, 20),
			nodeClassStr)
	}
	fmt.Println(strings.Repeat("═", 120))

	log.Printf("✅ Se encontraron %d nodos hijos", len(results))
}

func nodeClassToString(nc ua.NodeClass) string {
	switch nc {
	case ua.NodeClassVariable:
		return "Variable"
	case ua.NodeClassMethod:
		return "Method"
	case ua.NodeClassObject:
		return "Object"
	case ua.NodeClassObjectType:
		return "ObjectType"
	case ua.NodeClassDataType:
		return "DataType"
	case ua.NodeClassReferenceType:
		return "ReferenceType"
	case ua.NodeClassVariableType:
		return "VariableType"
	case ua.NodeClassView:
		return "View"
	default:
		return fmt.Sprintf("Unknown(%d)", nc)
	}
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// handleCallMethod invoca un método OPC UA
func handleCallMethod(plcManager *plc.Manager, reader *bufio.Reader) {
	// Seleccionar sorter
	fmt.Println("\n" + strings.Repeat("─", 50))
	fmt.Println("      LLAMAR MÉTODO OPC UA - SELECCIONE SORTER")
	fmt.Println(strings.Repeat("─", 50))
	fmt.Println("  1. Sorter Principal (ID: 1)")
	fmt.Println("  2. Sorter Secundario (ID: 2)")
	fmt.Println(strings.Repeat("─", 50))
	fmt.Print("Seleccione sorter (o 'b' para volver): ")

	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)

	if strings.ToLower(input) == "b" {
		return
	}

	choice, err := strconv.Atoi(input)
	if err != nil || choice < 1 || choice > 2 {
		log.Println("❌ Selección inválida")
		return
	}

	sorterID := choice

	// Para simplificar, usar los valores de configuración conocidos
	var objectID, methodID string
	if sorterID == 1 {
		objectID = "ns=4;i=1"
		methodID = "ns=4;i=21"
		fmt.Println("\n📌 Usando método DB_OPC_S7000 del Sorter Principal")
		fmt.Printf("   ObjectID: %s\n", objectID)
		fmt.Printf("   MethodID: %s\n", methodID)
	} else {
		log.Println("❌ El Sorter Secundario no tiene métodos configurados")
		return
	}

	// Primero, leer los InputArguments para mostrar qué espera el método
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	log.Printf("🔄 Leyendo InputArguments del método...")
	inputArgsNodeID := "ns=4;i=22"
	nodeInfo, err := plcManager.ReadNode(ctx, sorterID, inputArgsNodeID)
	if err != nil {
		log.Printf("❌ Error al leer InputArguments: %v", err)
		return
	}

	inputArgsArray, ok := nodeInfo.Value.([]*ua.ExtensionObject)
	if !ok || len(inputArgsArray) == 0 {
		log.Printf("❌ No se pudo obtener la definición de argumentos")
		return
	}

	fmt.Println("\n📋 Argumentos de entrada requeridos:")
	for i, extObj := range inputArgsArray {
		if arg, ok := extObj.Value.(*ua.Argument); ok {
			fmt.Printf("  %d. %s (Tipo: %s, ValueRank: %d)\n", i+1, arg.Name, arg.DataType.String(), arg.ValueRank)
		}
	}

	// Solicitar valores para cada argumento
	fmt.Println("\n📝 Ingrese los valores para los argumentos:")
	var inputVariants []*ua.Variant

	for i, extObj := range inputArgsArray {
		arg, ok := extObj.Value.(*ua.Argument)
		if !ok {
			log.Printf("❌ Error: argumento %d no es válido", i+1)
			return
		}

		fmt.Printf("\n  Argumento %d: %s\n", i+1, arg.Name)
		fmt.Printf("  Tipo esperado: %s\n", arg.DataType.String())
		fmt.Print("  Valor: ")

		valueStr, _ := reader.ReadString('\n')
		valueStr = strings.TrimSpace(valueStr)

		// Intentar convertir según el tipo
		var variant *ua.Variant
		dataTypeStr := arg.DataType.String()

		switch {
		case strings.Contains(dataTypeStr, "i=4"): // Int32 (estándar OPC UA)
			val, err := strconv.ParseInt(valueStr, 10, 32)
			if err != nil {
				log.Printf("❌ Valor inválido para Int32: %v", err)
				return
			}
			variant, _ = ua.NewVariant(int32(val))

		case strings.Contains(dataTypeStr, "i=5"): // UInt32 (estándar OPC UA)
			val, err := strconv.ParseUint(valueStr, 10, 32)
			if err != nil {
				log.Printf("❌ Valor inválido para UInt32: %v", err)
				return
			}
			variant, _ = ua.NewVariant(uint32(val))

		case strings.Contains(dataTypeStr, "i=6"): // Int64 (estándar OPC UA)
			val, err := strconv.ParseInt(valueStr, 10, 64)
			if err != nil {
				log.Printf("❌ Valor inválido para Int64: %v", err)
				return
			}
			variant, _ = ua.NewVariant(int64(val))

		case strings.Contains(dataTypeStr, "i=1"): // Boolean
			val, err := strconv.ParseBool(valueStr)
			if err != nil {
				log.Printf("❌ Valor inválido para Boolean: %v", err)
				return
			}
			variant, _ = ua.NewVariant(val)

		case strings.Contains(dataTypeStr, "i=12"): // String
			variant, _ = ua.NewVariant(valueStr)

		default:
			// Por defecto, intentar como string
			log.Printf("⚠️ Tipo desconocido %s, usando como string", dataTypeStr)
			variant, _ = ua.NewVariant(valueStr)
		}

		inputVariants = append(inputVariants, variant)
	}

	// Confirmar antes de ejecutar
	fmt.Println("\n⚠️  ¿Está seguro de que desea ejecutar este método? (s/n): ")
	confirm, _ := reader.ReadString('\n')
	confirm = strings.TrimSpace(strings.ToLower(confirm))

	if confirm != "s" && confirm != "y" {
		log.Println("❌ Operación cancelada")
		return
	}

	// Ejecutar el método
	log.Printf("🚀 Llamando método %s...", methodID)

	ctx2, cancel2 := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel2()

	outputValues, err := plcManager.CallMethod(ctx2, sorterID, objectID, methodID, inputVariants)
	if err != nil {
		log.Printf("❌ Error al llamar método: %v", err)
		return
	}

	// Mostrar resultados
	fmt.Println("\n✅ Método ejecutado exitosamente")
	if len(outputValues) > 0 {
		fmt.Println("\n📤 Valores de salida:")
		for i, outVal := range outputValues {
			fmt.Printf("  %d. %v (tipo: %T)\n", i+1, outVal, outVal)
		}
	} else {
		fmt.Println("  (Sin valores de retorno)")
	}
}

// handleCacheStats muestra las estadísticas de la cache LRU
func handleCacheStats(plcManager *plc.Manager) {
	fmt.Println("\n" + strings.Repeat("═", 50))
	fmt.Println("           ESTADÍSTICAS DE CACHE LRU")
	fmt.Println(strings.Repeat("═", 50))

	// Obtener stats de cada cliente
	stats := plcManager.GetCacheStats()

	if len(stats) == 0 {
		fmt.Println("⚠️  No hay clientes conectados")
		return
	}

	for endpoint, cacheLen := range stats {
		fmt.Printf("\n📊 PLC: %s\n", endpoint)
		fmt.Printf("   Entradas en cache: %d / 1000\n", cacheLen)
		fmt.Printf("   TTL configurado:   100ms\n")
		fmt.Printf("   Estado:            %s\n", func() string {
			if cacheLen > 800 {
				return "🔴 Cerca del límite"
			} else if cacheLen > 500 {
				return "🟡 Uso moderado"
			}
			return "🟢 Uso óptimo"
		}())
	}

	fmt.Println("\n💡 Tip: La cache mejora 100-200x el rendimiento de lecturas repetidas")
	fmt.Println("   Se invalida automáticamente después de cada escritura")
	fmt.Println(strings.Repeat("═", 50))
}

// handleMonitorNode crea una suscripción para monitorear cambios en un nodo en tiempo real
func handleMonitorNode(plcManager *plc.Manager, reader *bufio.Reader, nodes []MenuNodeInfo) {
	selectedNode, err := selectNode(reader, nodes)
	if err != nil {
		if err.Error() != "back" {
			log.Printf("❌ Error al seleccionar el nodo: %v", err)
		}
		return
	}

	fmt.Println("\n" + strings.Repeat("─", 50))
	fmt.Printf("📡 Iniciando monitoreo del nodo '%s'\n", selectedNode.Name)
	fmt.Printf("   NodeID: %s\n", selectedNode.NodeID)
	fmt.Println(strings.Repeat("─", 50))
	fmt.Println("Presiona Ctrl+C o escribe 'q' + Enter para detener")
	fmt.Println(strings.Repeat("─", 50))

	// Crear contexto con cancelación
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Iniciar suscripción (intervalo de 100ms)
	notifChan, cancelSub, err := plcManager.MonitorNode(ctx, selectedNode.SorterID, selectedNode.NodeID, 100*time.Millisecond)
	if err != nil {
		log.Printf("❌ Error al crear suscripción: %v", err)
		return
	}
	defer cancelSub()

	// Canal para detectar 'q' en input
	quitChan := make(chan bool)
	go func() {
		reader.ReadString('\n')
		quitChan <- true
	}()

	fmt.Println("\n📊 VALORES EN TIEMPO REAL:")
	fmt.Println(strings.Repeat("─", 50))

	// Contador de cambios
	changeCount := 0

	for {
		select {
		case nodeInfo, ok := <-notifChan:
			if !ok {
				log.Println("⚠️ Suscripción cerrada")
				return
			}
			changeCount++
			timestamp := time.Now().Format("15:04:05.000")
			fmt.Printf("[%s] #%d - Valor: %v (tipo: %s)\n",
				timestamp, changeCount, nodeInfo.Value, nodeInfo.ValueType)

		case <-quitChan:
			log.Printf("\n✅ Monitoreo detenido. Total de cambios detectados: %d", changeCount)
			return

		case <-time.After(30 * time.Second):
			if changeCount == 0 {
				fmt.Println("\n⏱️  No se detectaron cambios en 30 segundos.")
				fmt.Println("   El nodo puede ser estático o la suscripción no está funcionando.")
				return
			}
		}
	}
}
