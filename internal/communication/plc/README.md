# Módulo de Comunicación PLC (OPC UA)

## 📁 Estructura

```
internal/communication/plc/
├── client.go    - Cliente OPC UA (conexión, lectura, escritura)
├── manager.go   - Manager que coordina múltiples PLCs
└── types.go     - Estructuras de datos

cmd/plc/
└── main.go      - Programa de prueba de consola
```

## 🚀 Compilación

```bash
# Compilar programa de prueba
go build -o bin/plc-test cmd/plc/main.go

# Compilar programa principal
go build -o bin/api-greenex cmd/main.go
```

## ⚙️ Configuración

### Agregar Endpoint OPC UA en config.yaml

Cada sorter necesita su endpoint OPC UA configurado:

```yaml
sorters:
  - id: 1
    name: "Sorter Principal"
    plc_endpoint: "opc.tcp://192.168.120.100:4840" # ← ENDPOINT OPC UA
    plc_input_node: "ns=4;i=22"
    plc_output_node: "ns=4;i=23"
    salidas:
      - id: 1
        estado_node: "ns=4;i=28"
        bloqueo_node: "ns=4;i=27"
```

**Notas importantes:**

- Cada sorter puede tener su propio endpoint o compartir el mismo
- Los nodos (input, output, estado, bloqueo) son NodeIDs dentro del servidor OPC UA
- `bloqueo_node` es opcional (puede estar vacío `""` si la salida no lo soporta)

## 🧪 Programa de Prueba

### Ejecutar

```bash
# Usar configuración por defecto (config/config.yaml)
./bin/plc-test

# Usar archivo de configuración específico
./bin/plc-test /path/to/config.yaml
```

### Qué Hace

1. **Carga la configuración** desde el archivo YAML
2. **Conecta a todos los PLCs** (endpoints únicos configurados)
3. **Lee TODOS los nodos** configurados:
   - Nodos del sorter (input, output)
   - Nodos de cada salida (estado, bloqueo si existe)
4. **Muestra los resultados** en consola con formato legible

### Ejemplo de Salida

```
================================================================================
                     ESTADO DE NODOS OPC UA
================================================================================

╔═══ SORTER 1: Sorter Principal ═══
║ Endpoint: opc.tcp://192.168.120.100:4840
╠═══════════════════════════════════════════════════════════════
║
║ 🔌 NODOS DEL PLC:
║     Input : 🔢 0
║     Input   NodeID: ns=4;i=22 | Tipo: int16
║     Output: 🔢 0
║     Output  NodeID: ns=4;i=23 | Tipo: int16
║
║ 📦 SALIDAS:
║
║   ┌─ Salida 1: Exportación (Physical ID: 1)
║   │     Estado : 🔢 1
║   │     Estado   NodeID: ns=4;i=28 | Tipo: int16
║   │     Bloqueo: ❌ false
║   │     Bloqueo  NodeID: ns=4;i=27 | Tipo: bool
║   └─────────────────────────────────────────────
```

## 📚 API del Módulo

### Crear Manager

```go
import (
    "API-GREENEX/internal/communication/plc"
    "API-GREENEX/internal/config"
)

// Cargar configuración
cfg, err := config.LoadConfig("config/config.yaml")
if err != nil {
    log.Fatal(err)
}

// Crear manager
manager := plc.NewManager(cfg)
```

### Conectar a PLCs

```go
ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
defer cancel()

// Conecta a todos los endpoints únicos configurados
if err := manager.ConnectAll(ctx); err != nil {
    log.Fatal(err)
}
defer manager.CloseAll(context.Background())
```

### Leer Nodos

```go
// Leer TODOS los sorters
sortersData, err := manager.ReadAllSorterNodes(ctx)
if err != nil {
    log.Fatal(err)
}

// Leer UN sorter específico
sorterData, err := manager.ReadSorterNodes(ctx, sorterID)
if err != nil {
    log.Fatal(err)
}
```

### Estructura de Datos Retornada

```go
type SorterNodes struct {
    SorterID    int            // ID del sorter
    SorterName  string         // Nombre del sorter
    Endpoint    string         // Endpoint OPC UA
    InputNode   *NodeInfo      // Nodo de entrada del PLC
    OutputNode  *NodeInfo      // Nodo de salida del PLC
    SalidaNodes []SalidaNodes  // Nodos de cada salida
}

type NodeInfo struct {
    NodeID      string         // "ns=4;i=22"
    Description string         // "PLC Input"
    Value       interface{}    // Valor leído (bool, int16, string, etc.)
    ValueType   string         // "bool", "int16", etc.
    ReadTime    time.Time      // Momento de lectura
    Error       error          // nil si OK
}
```

## 🔧 Siguientes Pasos (Fase 2 - NO IMPLEMENTADO AÚN)

Las siguientes funcionalidades están planificadas pero NO implementadas:

- ✅ **Lectura de nodos** (IMPLEMENTADO)
- ⏳ **Escritura de nodos** (pendiente)
- ⏳ **SendCommandAndWait** - Escribir y esperar confirmación (pendiente)
- ⏳ **Integración con main.go** - Usar en producción (pendiente)
- ⏳ **Monitoreo continuo** - Suscripciones a cambios (pendiente)

## 🐛 Troubleshooting

### Error: "no hay endpoints OPC UA configurados"

**Causa:** Los sorters en el YAML no tienen `plc_endpoint` configurado.  
**Solución:** Agrega `plc_endpoint: "opc.tcp://IP:PUERTO"` a cada sorter.

### Error: "error al conectar a opc.tcp://..."

**Causa:** El servidor OPC UA no está accesible.  
**Solución:**

- Verifica que el PLC está encendido y conectado a la red
- Verifica la IP y puerto correctos
- Verifica firewall/conectividad de red

### Error: "nodeID inválido"

**Causa:** El formato del NodeID es incorrecto.  
**Solución:** Los NodeIDs deben tener formato:

- `ns=4;i=22` (numérico)
- `ns=4;s=|var|DB_Test.Valor` (string)

### Nodo con [ERROR] al leer

**Causa:** El nodo no existe en el servidor OPC UA o no tienes permisos.  
**Solución:**

- Verifica el NodeID con un browser OPC UA (UaExpert, Prosys, etc.)
- Verifica que el tipo de dato coincide
- Verifica permisos de lectura

## 📖 Referencias

- [gopcua - Cliente OPC UA en Go](https://github.com/gopcua/opcua)
- [OPC UA Specification](https://opcfoundation.org/developer-tools/specifications-unified-architecture)
