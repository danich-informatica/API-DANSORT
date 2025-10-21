# API-Greenex - Sistema de Clasificación Automática

## Descripción General

API-Greenex es un sistema de clasificación automática en tiempo real para líneas de producción agrícola. El sistema procesa lecturas de códigos QR desde dispositivos Cognex, identifica productos (SKUs) y los direcciona automáticamente a salidas específicas basándose en reglas de negocio configurables.

## Arquitectura del Sistema

### Componentes Principales

```
┌─────────────────┐
│  Cognex Reader  │ ──► TCP Socket (puerto 2001)
└─────────────────┘
         │
         ▼
┌─────────────────┐
│ CognexListener  │ ──► Procesa mensajes y valida formato
└─────────────────┘
         │
         ▼
┌─────────────────┐
│  Sorter Engine  │ ──► Determina salida según SKU
└─────────────────┘
         │
         ├──► PostgreSQL (persistencia)
         ├──► WebSocket (visualización tiempo real)
         └──► HTTP REST API (configuración)
```

### Stack Tecnológico

- **Backend**: Go 1.22+
- **Base de Datos**: PostgreSQL 14+
- **Comunicación Tiempo Real**: WebSockets (gorilla/websocket)
- **API REST**: Gin Framework
- **Protocolo Cognex**: TCP Socket raw

## Modelo de Datos

### Estructura de Lectura (LecturaEvent)

```go
type LecturaEvent struct {
    Timestamp   time.Time
    Exitoso     bool
    SKU         string      // Formato: CALIBRE-VARIEDAD-EMBALAJE
    Especie     string
    Calibre     string
    Variedad    string
    Embalaje    string
    Correlativo string      // ID único de la caja
    Mensaje     string      // Mensaje original de Cognex
    Error       error
    Dispositivo string
}
```

### Formato de Mensaje Cognex

Formato esperado: `ESPECIE;CALIBRE;NUMERO;EMBALAJE;MARCA;VARIEDAD`

Ejemplo: `E001;XL;0;CECDCAM5;GREENEX;V018`

- **ESPECIE**: Código de especie (E001)
- **CALIBRE**: Tamaño del producto (XL, L, M, S, etc.)
- **NUMERO**: Número de serie (ignorado)
- **EMBALAJE**: Tipo de empaque (código de 8 caracteres)
- **MARCA**: Identificador de marca (GREENEX)
- **VARIEDAD**: Código de variedad (V018)

SKU generado: `XL-V018-CECDCAM5`

## Lógica de Clasificación

### 1. Recepción de Lectura

El `CognexListener` recibe mensajes TCP del dispositivo Cognex:

```
1. Conexión TCP establecida en puerto 2001
2. Cognex envía mensaje terminado en \r o \n
3. Sistema parsea el mensaje según formato esperado
4. Valida estructura y genera SKU
5. Emite evento LecturaEvent al canal del Sorter
```

### 2. Procesamiento de Evento

El `Sorter` consume eventos del canal y ejecuta la siguiente lógica:

```go
func procesarEventosCognex() {
    for evento := range cognexChannel {
        if evento.Exitoso {
            // Lectura exitosa
            salida := determinarSalida(evento.SKU, evento.Calibre)
            registrarLectura(evento.SKU)  // Para estadísticas
            PublishLecturaEvent(evento, salida, true)  // WebSocket
            // TODO: Activar actuadores físicos
        } else {
            // Lectura fallida (NO_READ, formato inválido, etc.)
            salida := GetDiscardSalida()
            PublishLecturaEvent(evento, salida, false)
        }
    }
}
```

### 3. Determinación de Salida

Algoritmo de asignación de salida:

```
1. SKU == "REJECT" o "NO_READ"?
   └─► Enviar a salida manual (tipo: manual)

2. Existe distribución por lotes para esta SKU?
   └─► Aplicar algoritmo round-robin con batch_size
       Ejemplo: Salida A (batch_size=10), Salida B (batch_size=5)
       - Primeras 10 cajas → Salida A
       - Siguientes 5 cajas → Salida B
       - Siguientes 10 cajas → Salida A
       - (ciclo se repite)

3. SKU está asignada a alguna salida?
   └─► Enviar a esa salida específica

4. SKU no configurada?
   └─► Enviar a salida de descarte (tipo: manual)
```

Implementación del distribuidor por lotes:

```go
type BatchDistributor struct {
    Salidas      []int           // IDs de salidas
    CurrentIndex int             // Índice actual en el ciclo
    CurrentCount int             // Contador del lote actual
    BatchSizes   map[int]int     // Tamaño de lote por salida
}

func (bd *BatchDistributor) NextSalida() int {
    salidaID := bd.Salidas[bd.CurrentIndex]
    batchSize := bd.BatchSizes[salidaID]

    bd.CurrentCount++

    if bd.CurrentCount >= batchSize {
        bd.CurrentIndex = (bd.CurrentIndex + 1) % len(bd.Salidas)
        bd.CurrentCount = 0
    }

    return salidaID
}
```

## Sistema de Estadísticas en Tiempo Real

### Ventana Deslizante

El sistema mantiene estadísticas de flujo usando una ventana temporal deslizante:

```
Configuración predeterminada:
- Ventana: 20 segundos (configurable)
- Intervalo de cálculo: 5 segundos (configurable)
```

### Cálculo de Porcentajes

```go
func calcularFlowStatistics(windowDuration time.Duration) FlowStatistics {
    // Filtrar lecturas dentro de la ventana
    now := time.Now()
    cutoff := now.Add(-windowDuration)

    skuCounts := make(map[string]int)
    totalLecturas := 0

    for _, record := range lecturaRecords {
        if record.Timestamp.After(cutoff) {
            skuCounts[record.SKU]++
            totalLecturas++
        }
    }

    // Calcular porcentajes
    for sku, count := range skuCounts {
        percentage := (count / totalLecturas) * 100.0
        // Redondear a entero
    }
}
```

Ejemplo de salida:

```json
{
  "sorter_id": 2,
  "timestamp": "2025-10-09T16:30:00Z",
  "total_lecturas": 45,
  "window_seconds": 20,
  "stats": [
    { "sku": "XL-V018-CECDCAM5", "lecturas": 30, "porcentaje": 67 },
    { "sku": "L-V022-CEMGKAM5", "lecturas": 15, "porcentaje": 33 }
  ]
}
```

## API WebSocket

### Conexión

```
URL: ws://host:port/ws/assignment_{sorter_id}
```

Ejemplo: `ws://localhost:8080/ws/assignment_2`

### Eventos Emitidos

#### 1. lectura_procesada

Evento emitido por cada lectura individual procesada:

```json
{
  "type": "lectura_procesada",
  "qr_code": "E001;XL;0;CECDCAM5;GREENEX;V018",
  "sku": "XL-V018-CECDCAM5",
  "salida_id": 8,
  "salida_name": "Premium",
  "exitoso": true,
  "timestamp": "2025-10-09T16:23:02Z",
  "sorter_id": 2,
  "correlativo": "10888",
  "calibre": "XL",
  "variedad": "V018",
  "embalaje": "CECDCAM5"
}
```

#### 2. sku_assigned

Evento emitido cuando cambia la configuración de SKUs:

```json
{
  "type": "sku_assigned",
  "sorter_id": 2,
  "salidas": [
    {
      "id": 8,
      "nombre": "Premium",
      "tipo": "automatico",
      "batch_size": 10,
      "skus": ["XL-V018-CECDCAM5", "XL-V022-CEMGKAM5"],
      "count": 245
    }
  ]
}
```

#### 3. sku_flow_stats

Estadísticas agregadas cada 5 segundos:

```json
{
  "type": "sku_flow_stats",
  "sorter_id": 2,
  "total_lecturas": 45,
  "window_seconds": 20,
  "stats": [
    { "sku": "XL-V018-CECDCAM5", "lecturas": 30, "porcentaje": 67 },
    { "sku": "L-V022-CEMGKAM5", "lecturas": 15, "porcentaje": 33 }
  ]
}
```

## API REST Endpoints

### Gestión de SKUs

**GET /sku/assigned/:sorter_id**

- Obtiene SKUs asignados a un sorter con porcentajes actualizados

**POST /sku/assign**

```json
{
  "sorter_id": 2,
  "sku_id": 12345,
  "salida_id": 8
}
```

**DELETE /sku/unassign**

```json
{
  "sorter_id": 2,
  "sku_id": 12345,
  "salida_id": 8
}
```

**DELETE /sku/clear_salida/:sorter_id/:salida_id**

- Elimina todas las SKUs de una salida específica

### Gestión de Lecturas

**GET /lecturas/sorter/:sorter_id**

- Parámetros: limit, offset, start_date, end_date
- Obtiene historial de lecturas con paginación

**GET /lecturas/correlativo/:correlativo**

- Busca lectura específica por correlativo

### Streaming en Tiempo Real

**GET /stream/lecturas/sorter/:sorter_id**

- Server-Sent Events (SSE)
- Stream continuo de lecturas en tiempo real

**GET /stream/sku/flow/:sorter_id**

- Stream de estadísticas de flujo cada 5 segundos

## Base de Datos

### Tablas Principales

#### sorters

```sql
CREATE TABLE sorters (
    id INTEGER PRIMARY KEY,
    ubicacion VARCHAR(255) NOT NULL
);
```

#### salidas

```sql
CREATE TABLE salidas (
    id INTEGER PRIMARY KEY,
    sorter_id INTEGER REFERENCES sorters(id),
    numero_salida INTEGER NOT NULL,
    activo BOOLEAN DEFAULT true
);
```

#### sku

```sql
CREATE TABLE sku (
    sku VARCHAR(100) PRIMARY KEY,
    calibre VARCHAR(50),
    variedad VARCHAR(50),
    embalaje VARCHAR(50),
    estado BOOLEAN DEFAULT true
);
```

#### salida_sku (relación muchos a muchos)

```sql
CREATE TABLE salida_sku (
    salida_id INTEGER REFERENCES salidas(id),
    sorter_id INTEGER REFERENCES sorters(id),
    calibre VARCHAR(50),
    variedad VARCHAR(50),
    embalaje VARCHAR(50),
    PRIMARY KEY (salida_id, sorter_id, calibre, variedad, embalaje)
);
```

#### caja (registro de lecturas)

```sql
CREATE TABLE caja (
    id SERIAL PRIMARY KEY,
    correlativo VARCHAR(50) UNIQUE,
    qr_content TEXT,
    sku VARCHAR(100),
    calibre VARCHAR(50),
    variedad VARCHAR(50),
    embalaje VARCHAR(50),
    salida_id INTEGER REFERENCES salidas(id),
    sorter_id INTEGER REFERENCES sorters(id),
    timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    exitoso BOOLEAN DEFAULT true,
    scan_method VARCHAR(50)
);
```

## Configuración

### Archivo config.yaml

```yaml
database:
  postgres:
    url: "postgresql://user:pass@host:5432/greenex"
    min_conns: 2
    max_conns: 10
    connect_timeout: "10s"
    healthcheck_interval: "30s"

http:
  host: "0.0.0.0"
  port: 8080

cognex_devices:
  - id: 1
    name: "Cognex-Sorter-1"
    ubicacion: "Linea-A"
    host: "192.168.1.100"
    port: 2001
    scan_method: "DATAMAN"

sorters:
  - id: 1
    name: "Sorter Principal"
    ubicacion: "Linea-A"
    cognex_id: 1
    salidas:
      - id: 1
        name: "Premium"
        tipo: "automatico"
        batch_size: 10
      - id: 2
        name: "Standard"
        tipo: "automatico"
        batch_size: 5
      - id: 3
        name: "Descarte"
        tipo: "manual"
        batch_size: 1
```

## Compilación e Instalación

### Requisitos

- Go 1.22 o superior
- PostgreSQL 14 o superior
- Node.js 18+ (para desarrollo frontend, opcional)

### Compilación

```bash
# Compilar binario principal
go build -o bin/api-greenex cmd/main.go

# Compilar simulador de Cognex (testing)
go build -o bin/cognex-simulator cmd/cognex_simulator/main.go

# Compilar simulador web de Cognex
go build -o bin/cognex-web-simulator cmd/cognex_web_simulator/main.go
```

### Inicialización de Base de Datos

```bash
# Crear base de datos
psql -U postgres -f DB/db_create.sql

# Crear tablas
psql -U postgres -d greenex -f DB/greendex_db_table_create.sql
```

### Variables de Entorno

Crear archivo `.env` en la raíz del proyecto:

```env
CONFIG_FILE=config/config.yaml
DB_URL=postgresql://user:pass@localhost:5432/greenex
```

### Ejecución

```bash
# Iniciar servidor principal
./bin/api-greenex

# Iniciar simulador de Cognex (puerto 2001)
./bin/cognex-simulator

# Iniciar simulador web (puerto 9090)
./bin/cognex-web-simulator
```

## Visualizador Web

URL: `http://localhost:8080/web/visualizer.html?sorter=1`

Características:

- Visualización en tiempo real de cajas procesadas
- Animación de cinta transportadora
- Resaltado de salidas activas
- Contadores de lecturas exitosas/fallidas
- Display de última lectura de Cognex
- Grid de salidas con SKUs asignados

## Gestión de Concurrencia

### Canales Go

El sistema utiliza canales buffered para comunicación entre goroutines:

```go
// Canal de eventos de Cognex
EventChan chan LecturaEvent  // Buffer: 100

// Canal de SKUs assignables
skuChannel chan []SKUAssignable  // Buffer: 10

// Canal de estadísticas de flujo
flowStatsChannel chan FlowStatistics  // Buffer: 5
```

### Mutex para Datos Compartidos

```go
// Protección de estadísticas de flujo
flowMutex sync.RWMutex

// Protección de distribuidores de lote
batchMutex sync.RWMutex
```

### Goroutines Principales

1. **procesarEventosCognex()**: Procesa eventos del canal de Cognex
2. **StartFlowStatistics()**: Calcula estadísticas cada 5 segundos
3. **CognexListener.Start()**: Lee socket TCP de Cognex
4. **WebSocketHub.Run()**: Gestiona conexiones WebSocket

## Manejo de Errores

### Tipos de Lecturas Fallidas

```go
const (
    LecturaExitosa     = "EXITOSA"
    LecturaNoRead      = "NO_READ"        // Cognex no pudo leer QR
    LecturaFormato     = "ERROR_FORMATO"  // Formato de mensaje inválido
    LecturaSKU         = "ERROR_SKU"      // SKU no existe en BD
    LecturaDB          = "ERROR_DB"       // Error de base de datos
)
```

### Estrategia de Recuperación

- **NO_READ**: Enviar a salida de descarte
- **ERROR_FORMATO**: Enviar a salida de descarte
- **ERROR_SKU**: Enviar a salida de descarte
- **ERROR_DB**: Log error, continuar procesamiento

## Logs del Sistema

Formato de logs:

```
[timestamp] [nivel] Componente: Mensaje

Ejemplo:
2025/10/09 16:23:02 [Sorter] Lectura #10888 | SKU: XL-V018-CECDCAM5 | Salida: Premium (ID: 8) | Razon: sort por SKU
```

Niveles de log:

- Proceso normal (sin prefijo)
- Advertencia (prefijo de preocupación visual)
- Error (prefijo de error visual)
- Debug (comentarios de desarrollo)

## Testing

### Simulador de Cognex

El sistema incluye un simulador que genera lecturas sintéticas:

```bash
./bin/cognex-simulator --host localhost --port 2001 --interval 500ms
```

Parámetros:

- `--host`: Host del servidor (default: localhost)
- `--port`: Puerto TCP (default: 2001)
- `--interval`: Intervalo entre lecturas (default: 1s)
- `--error-rate`: Porcentaje de lecturas fallidas (default: 5%)

### Unit Tests

```bash
# Ejecutar todos los tests
go test ./...

# Test con cobertura
go test -cover ./...

# Test verboso
go test -v ./internal/sorter/...
```

## Monitoreo y Métricas

### Métricas Disponibles

- Lecturas exitosas totales
- Lecturas fallidas totales
- Tasa de éxito (%)
- Lecturas por segundo
- Distribución de SKUs (%)
- Cajas por salida

### Health Check

**GET /health**

```json
{
  "status": "healthy",
  "database": "connected",
  "cognex": "connected",
  "uptime_seconds": 3600
}
```

## Troubleshooting

### Problema: WebSocket no conecta (404)

**Solución**: Verificar que la URL incluye el sorter_id correcto:

```
ws://host:port/ws/assignment_{sorter_id}
```

### Problema: Cognex no envía datos

**Diagnóstico**:

1. Verificar conexión TCP: `telnet <cognex_ip> 2001`
2. Revisar logs del CognexListener
3. Verificar formato de mensaje esperado

### Problema: SKUs no se asignan correctamente

**Diagnóstico**:

1. Verificar que SKU existe en tabla `sku`
2. Verificar tipo de salida (manual vs automatico)
3. Revisar logs de determinación de salida

### Problema: Alta latencia en WebSocket

**Solución**:

1. Aumentar buffer de canales
2. Reducir frecuencia de estadísticas
3. Verificar carga de base de datos

## Mantenimiento

### Limpieza de Datos Antiguos

```sql
-- Eliminar lecturas más antiguas de 90 días
DELETE FROM caja
WHERE timestamp < NOW() - INTERVAL '90 days';

-- Vacuum para recuperar espacio
VACUUM ANALYZE caja;
```
