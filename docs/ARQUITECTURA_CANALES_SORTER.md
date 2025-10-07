# Arquitectura de Canales por Sorter

## Descripción General

Cada **sorter** tiene su propio canal dedicado de SKUs (`chan []models.SKUAssignable`). Esto permite:

1. **Actualización en tiempo real**: Cada sorter puede actualizar su lista de SKUs sin afectar a otros sorters
2. **Escalabilidad**: Soporta millones de SKUs con arquitectura basada en canales (no bloqueante)
3. **Aislamiento**: Cada sorter mantiene su propia lista de SKUs asignados
4. **Performance**: El HTTP endpoint consume directamente del canal específico del sorter

## Flujo de Datos

```
┌─────────────────┐
│   SKUManager    │  ← Carga SKUs desde PostgreSQL
└────────┬────────┘
         │
         │ GetActiveSKUs()
         ▼
┌─────────────────────────────────────────────┐
│  ChannelManager (Singleton)                 │
│  ┌──────────────────────────────────────┐  │
│  │ sorterSKUChannels                    │  │
│  │   map[string]chan []SKUAssignable    │  │
│  │                                       │  │
│  │   "1" → chan (buffer: 10)            │  │
│  │   "2" → chan (buffer: 10)            │  │
│  │   "3" → chan (buffer: 10)            │  │
│  └──────────────────────────────────────┘  │
└──────────┬──────────────┬──────────────────┘
           │              │
           │              │
     ┌─────▼────┐   ┌─────▼────┐
     │ Sorter#1 │   │ Sorter#2 │
     └─────┬────┘   └─────┬────┘
           │              │
           │ UpdateSKUs() │ UpdateSKUs()
           ▼              ▼
     skuChannel     skuChannel
           │              │
           │              │
     ┌─────▼──────────────▼────────┐
     │  HTTP GET /skus/assignables │
     │         /:sorter_id          │
     └──────────────────────────────┘
```

## Componentes Principales

### 1. ChannelManager (`internal/shared/channels.go`)

**Singleton** que gestiona todos los canales de la aplicación:

```go
type ChannelManager struct {
    cognexQRChannel   chan models.LecturaEvent
    skuStreamChannel  models.SKUChannel
    sorterSKUChannels map[string]chan []models.SKUAssignable  // Canal por sorter
    mu                sync.RWMutex
}
```

**Métodos clave:**

- `RegisterSorterSKUChannel(sorterID string, bufferSize int)`: Registra canal de un sorter
- `GetSorterSKUChannel(sorterID string)`: Obtiene canal de un sorter específico
- `IsSorterRegistered(sorterID string)`: Verifica si el sorter está registrado
- `GetAllSorterIDs()`: Lista todos los sorters registrados

### 2. Sorter (`internal/sorter/sorter.go`)

Cada sorter mantiene:

```go
type Sorter struct {
    ID           int
    skuChannel   chan []models.SKUAssignable  // Canal dedicado
    assignedSKUs []models.SKUAssignable        // Lista local de SKUs
    // ...
}
```

**Métodos clave:**

- `UpdateSKUs(skus []SKUAssignable)`: Actualiza SKUs y publica al canal (no bloqueante)
- `GetSKUChannel()`: Retorna el canal de este sorter
- `GetCurrentSKUs()`: Retorna la lista actual de SKUs
- `StartSKUPublisher(intervalSeconds int, skuSource func())`: Publicador periódico de SKUs

### 3. HTTP Endpoint (`internal/listeners/http.go`)

```go
GET /skus/assignables/:sorter_id?timeout=5
```

**Funcionamiento:**

1. Valida el `sorter_id`
2. Obtiene el canal del sorter desde `ChannelManager`
3. Espera SKUs del canal con timeout configurable (default: 5 segundos)
4. Retorna la lista de SKUs o error 404/timeout

**Respuesta:**

```json
[
  {
    "id": 0,
    "sku": "REJECT"
  },
  {
    "id": 1234567890,
    "sku": "J-V022-CEMGKAM5"
  },
  ...
]
```

### 4. Inicialización (`cmd/main.go`, `cmd/http/main.go`)

Al iniciar la aplicación:

1. Se carga el `ChannelManager` (Singleton)
2. Se crea el `SKUManager` (caché de SKUs con streaming)
3. Se cargan todos los SKUs activos desde PostgreSQL
4. Se convierten a formato `SKUAssignable` con hash-based IDs (FNV-1a)
5. Se publica la lista inicial a cada canal de sorter

```go
// Cargar SKUs activos
activeSKUs := skuManager.GetActiveSKUs()

// Convertir a formato assignable
assignableSKUs := make([]models.SKUAssignable, 0, len(activeSKUs)+1)
assignableSKUs = append(assignableSKUs, models.GetRejectSKU())  // ID 0
for _, sku := range activeSKUs {
    assignableSKUs = append(assignableSKUs, sku.ToAssignableWithHash())
}

// Publicar a cada sorter
for _, sorterCfg := range cfg.Sorters {
    sorterID := fmt.Sprintf("%d", sorterCfg.ID)
    sorterChannel := channelMgr.GetSorterSKUChannel(sorterID)

    select {
    case sorterChannel <- assignableSKUs:
        log.Printf("✅ Sorter #%s: %d SKUs publicados", sorterID, len(assignableSKUs))
    default:
        log.Printf("⚠️ Sorter #%s: Canal lleno", sorterID)
    }
}
```

## Ventajas de esta Arquitectura

### 1. **Desacoplamiento Total**

- HTTP no depende de lógica de negocio del Sorter
- Cada componente trabaja de forma independiente
- Comunicación asíncrona vía canales

### 2. **Escalabilidad**

- Soporta millones de SKUs sin bloqueos
- Canales con buffer (10 mensajes por sorter)
- Publicación no bloqueante con `select`

### 3. **Actualización en Tiempo Real**

- Cada sorter puede actualizar su lista dinámicamente
- Los cambios se propagan instantáneamente al HTTP endpoint
- Ideal para sistemas con alta frecuencia de cambios

### 4. **Performance**

- IDs numéricos basados en hash FNV-1a (determinísticos)
- Sin acceso a base de datos en cada request HTTP
- Consumo directo desde canal (O(1) para lectura)

### 5. **Tolerancia a Fallos**

- Si un sorter no está registrado: HTTP devuelve 404
- Timeout configurable para evitar bloqueos infinitos
- Publicación no bloqueante (si canal lleno, log warning)

## Parámetros Configurables

### Buffer del Canal

```go
// En GetNewSorter()
channelMgr.RegisterSorterSKUChannel(sorterID, 10)  // Buffer de 10 mensajes
```

### Timeout HTTP

```http
GET /skus/assignables/1?timeout=10  # 10 segundos de timeout
```

### Publicador Periódico

```go
// Publicar SKUs cada 30 segundos
sorter.StartSKUPublisher(30, func() []models.SKUAssignable {
    return skuManager.GetActiveSKUs()
})
```

## Casos de Uso

### 1. Obtener SKUs de un Sorter

```bash
curl http://localhost:8080/skus/assignables/1
```

### 2. Actualizar SKUs de un Sorter (desde código)

```go
newSKUs := []models.SKUAssignable{
    {ID: 0, SKU: "REJECT"},
    {ID: 123456, SKU: "A-B001-C002"},
}
sorter.UpdateSKUs(newSKUs)
```

### 3. Publicador Automático

```go
// En la inicialización del sorter
sorter.StartSKUPublisher(60, func() []models.SKUAssignable {
    // Obtener SKUs desde alguna fuente
    return getUpdatedSKUs()
})
```

## Mantenimiento

### Agregar Nuevo Sorter

1. Agregar configuración en `config/config.yaml`
2. El ChannelManager registra automáticamente el canal en `GetNewSorter()`
3. Los SKUs iniciales se publican al iniciar la aplicación

### Debugging

```go
// Ver sorters registrados
sorterIDs := channelMgr.GetAllSorterIDs()
log.Printf("Sorters registrados: %v", sorterIDs)

// Verificar si un sorter existe
if channelMgr.IsSorterRegistered("1") {
    log.Println("Sorter #1 está registrado")
}
```

### Monitoreo

- Logs de publicación: `✅ Sorter #X: Y SKUs publicados`
- Logs de canal lleno: `⚠️ Sorter #X: Canal lleno`
- HTTP 404: Sorter no encontrado
- HTTP 408: Timeout esperando SKUs

## Referencias

- `internal/shared/channels.go` - ChannelManager (Singleton)
- `internal/sorter/sorter.go` - Sorter con canal dedicado
- `internal/listeners/http.go` - Endpoint HTTP con consumo desde canal
- `internal/models/sku.go` - Modelos SKU y SKUAssignable
- `cmd/main.go` - Inicialización completa
- `cmd/http/main.go` - Servidor HTTP aislado
