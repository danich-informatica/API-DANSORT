# API-Greenex - Manual de Uso Completo

## 📋 Índice

1. [Introducción](#introducción)
2. [Arquitectura del Sistema](#arquitectura-del-sistema)
3. [Protocolos de Comunicación](#protocolos-de-comunicación)
4. [API REST HTTP](#api-rest-http)
5. [API WebSocket](#api-websocket)
6. [Protocolo TCP Cognex](#protocolo-tcp-cognex)
7. [Protocolo OPC UA (PLC)](#protocolo-opc-ua-plc)
8. [Base de Datos PostgreSQL](#base-de-datos-postgresql)
9. [Configuración](#configuración)
10. [Ejemplos de Uso](#ejemplos-de-uso)

---

## 🎯 Introducción

API-Greenex es un sistema de clasificación automática en tiempo real para líneas de producción agrícola que procesa lecturas de códigos QR desde dispositivos Cognex y direcciona automáticamente los productos a salidas específicas mediante señales OPC UA a PLCs.

### Componentes del Sistema

```
┌──────────────┐      ┌───────────────┐      ┌──────────────┐
│   Cognex     │─TCP─►│  API-Greenex  │◄─OPC─│     PLC      │
│   Reader     │      │   (Servidor)  │  UA  │  (Siemens)   │
└──────────────┘      └───────┬───────┘      └──────────────┘
                              │
                    ┌─────────┼─────────┐
                    │         │         │
                ┌───▼───┐ ┌───▼──┐  ┌──▼─────┐
                │  HTTP │ │  WS  │  │  PSQL  │
                │  API  │ │      │  │   DB   │
                └───────┘ └──────┘  └────────┘
```

---

## 🏗️ Arquitectura del Sistema

### Stack Tecnológico

| Component     | Tecnología        | Versión | Puerto |
| ------------- | ----------------- | ------- | ------ |
| Backend       | Go                | 1.22+   | -      |
| API REST      | Gin Framework     | 1.9+    | 8080   |
| WebSocket     | gorilla/websocket | 1.5+    | 8080   |
| Base de Datos | PostgreSQL        | 14+     | 5432   |
| OPC UA        | gopcua            | 0.5+    | 4840   |
| TCP Cognex    | net/tcp           | stdlib  | 2001   |

### Flujo de Datos

```mermaid
sequenceDiagram
    Cognex->>API: TCP Socket (QR Code)
    API->>DB: Validar SKU
    DB-->>API: SKU Info
    API->>API: Determinar Salida
    API->>PLC: OPC UA (Activar Salida)
    API->>WebSocket: Broadcast Evento
    API->>DB: Registrar Lectura
```

---

## 🔌 Protocolos de Comunicación

### 1. HTTP REST API

**Base URL**: `http://localhost:8080`

**Content-Type**: `application/json`

**Autenticación**: Ninguna (desarrollo)

### 2. WebSocket

**URL Pattern**: `ws://localhost:8080/ws/{room}`

**Protocol**: WebSocket RFC 6455

**Subprotocol**: Ninguno

### 3. TCP Raw Socket (Cognex)

**Protocol**: TCP

**Puerto**: 2001

**Encoding**: ASCII

**Terminador**: `\r\n`

### 4. OPC UA (PLC)

**Protocol**: OPC UA Binary

**Puerto**: 4840

**Security Mode**: None

**Security Policy**: None

---

## 🌐 API REST HTTP

### Gestión de Asignaciones SKU

#### 📥 GET `/sku/assigned/:sorter_id`

Obtiene todas las SKUs asignadas a un sorter con porcentajes de flujo actualizados.

**Parámetros de Ruta**:

- `sorter_id` (int): ID del sorter

**Respuesta Exitosa** (200 OK):

```json
{
  "sorter_id": 2,
  "skus": [
    {
      "id": 12345,
      "sku": "XL-V018-CECDCAM5",
      "calibre": "XL",
      "variedad": "V018",
      "embalaje": "CECDCAM5",
      "is_assigned": true,
      "percentage": 67.0
    },
    {
      "id": 12346,
      "sku": "L-V022-CEMGKAM5",
      "calibre": "L",
      "variedad": "V022",
      "embalaje": "CEMGKAM5",
      "is_assigned": true,
      "percentage": 33.0
    }
  ]
}
```

**Ejemplo cURL**:

```bash
curl -X GET http://localhost:8080/sku/assigned/2
```

**Ejemplo JavaScript**:

```javascript
fetch("http://localhost:8080/sku/assigned/2")
  .then((response) => response.json())
  .then((data) => console.log(data));
```

---

#### 📤 POST `/sku/assign`

Asigna una SKU a una salida específica de un sorter.

**Body Request**:

```json
{
  "sorter_id": 2,
  "sku_id": 12345,
  "salida_id": 8
}
```

**Validaciones**:

- `sorter_id`: Debe existir en la BD
- `sku_id`: Debe existir en tabla `sku`
- `salida_id`: Debe pertenecer al sorter especificado
- ⚠️ **CRÍTICO**: No se puede asignar SKU REJECT (ID=0) a salida automática

**Respuesta Exitosa** (200 OK):

```json
{
  "message": "SKU XL-V018-CECDCAM5 asignada a salida Premium (ID: 8)",
  "sorter_id": 2,
  "sku_id": 12345,
  "salida_id": 8
}
```

**Error 400** (Bad Request):

```json
{
  "error": "no se puede asignar SKU REJECT (ID=0) a salida automática 'Premium' (ID=8)"
}
```

**Error 404** (Not Found):

```json
{
  "error": "SKU con ID 12345 no encontrada en las SKUs disponibles del sorter #2"
}
```

**Ejemplo cURL**:

```bash
curl -X POST http://localhost:8080/sku/assign \
  -H "Content-Type: application/json" \
  -d '{
    "sorter_id": 2,
    "sku_id": 12345,
    "salida_id": 8
  }'
```

**Ejemplo JavaScript**:

```javascript
fetch("http://localhost:8080/sku/assign", {
  method: "POST",
  headers: { "Content-Type": "application/json" },
  body: JSON.stringify({
    sorter_id: 2,
    sku_id: 12345,
    salida_id: 8,
  }),
})
  .then((response) => response.json())
  .then((data) => console.log(data));
```

---

#### 🗑️ DELETE `/sku/unassign`

Elimina una SKU de una salida específica.

**Body Request**:

```json
{
  "sorter_id": 2,
  "sku_id": 12345,
  "salida_id": 8
}
```

**Respuesta Exitosa** (200 OK):

```json
{
  "message": "SKU XL-V018-CECDCAM5 (ID=12345) eliminada de salida Premium (ID=8)",
  "calibre": "XL",
  "variedad": "V018",
  "embalaje": "CECDCAM5"
}
```

**Error 404**:

```json
{
  "error": "SKU con ID 12345 no encontrada en salida 8 del sorter #2"
}
```

**Ejemplo cURL**:

```bash
curl -X DELETE http://localhost:8080/sku/unassign \
  -H "Content-Type: application/json" \
  -d '{
    "sorter_id": 2,
    "sku_id": 12345,
    "salida_id": 8
  }'
```

---

#### 🧹 DELETE `/sku/clear_salida/:sorter_id/:salida_id`

Elimina **TODAS** las SKUs de una salida específica.

**Parámetros de Ruta**:

- `sorter_id` (int): ID del sorter
- `salida_id` (int): ID de la salida

**Respuesta Exitosa** (200 OK):

```json
{
  "message": "Eliminadas 5 SKUs de salida Premium (ID=8)",
  "removed_skus": ["XL-V018-CECDCAM5", "L-V022-CEMGKAM5", "M-V024-CECGKAM5"]
}
```

**Ejemplo cURL**:

```bash
curl -X DELETE http://localhost:8080/sku/clear_salida/2/8
```

---

### Consulta de Lecturas

#### 📊 GET `/lecturas/sorter/:sorter_id`

Obtiene historial de lecturas con paginación y filtros.

**Parámetros de Ruta**:

- `sorter_id` (int): ID del sorter

**Query Parameters**:

- `limit` (int): Máximo de registros (default: 100, max: 1000)
- `offset` (int): Offset para paginación (default: 0)
- `start_date` (string): Fecha inicio ISO 8601 (opcional)
- `end_date` (string): Fecha fin ISO 8601 (opcional)
- `exitoso` (bool): Filtrar por exitosas/fallidas (opcional)

**Respuesta Exitosa** (200 OK):

```json
{
  "sorter_id": 2,
  "total": 1540,
  "limit": 100,
  "offset": 0,
  "lecturas": [
    {
      "id": 10888,
      "correlativo": "10888",
      "qr_content": "E001;XL;0;CECDCAM5;GREENEX;V018",
      "sku": "XL-V018-CECDCAM5",
      "calibre": "XL",
      "variedad": "V018",
      "embalaje": "CECDCAM5",
      "salida_id": 8,
      "salida_nombre": "Premium",
      "timestamp": "2025-10-14T15:23:02Z",
      "exitoso": true,
      "scan_method": "DATAMAN"
    }
  ]
}
```

**Ejemplos**:

```bash
# Últimas 100 lecturas
curl "http://localhost:8080/lecturas/sorter/2?limit=100"

# Lecturas de hoy
curl "http://localhost:8080/lecturas/sorter/2?start_date=2025-10-14T00:00:00Z"

# Solo lecturas exitosas
curl "http://localhost:8080/lecturas/sorter/2?exitoso=true"

# Paginación
curl "http://localhost:8080/lecturas/sorter/2?limit=50&offset=100"
```

---

#### 🔍 GET `/lecturas/correlativo/:correlativo`

Busca una lectura específica por su correlativo único.

**Parámetros de Ruta**:

- `correlativo` (string): ID único de la caja

**Respuesta Exitosa** (200 OK):

```json
{
  "id": 10888,
  "correlativo": "10888",
  "qr_content": "E001;XL;0;CECDCAM5;GREENEX;V018",
  "sku": "XL-V018-CECDCAM5",
  "calibre": "XL",
  "variedad": "V018",
  "embalaje": "CECDCAM5",
  "salida_id": 8,
  "salida_nombre": "Premium",
  "salida_physical_id": 1,
  "timestamp": "2025-10-14T15:23:02Z",
  "exitoso": true,
  "scan_method": "DATAMAN"
}
```

**Error 404**:

```json
{
  "error": "lectura con correlativo 10888 no encontrada"
}
```

**Ejemplo cURL**:

```bash
curl http://localhost:8080/lecturas/correlativo/10888
```

---

### Server-Sent Events (SSE)

#### 📡 GET `/stream/lecturas/sorter/:sorter_id`

Stream continuo de lecturas en tiempo real usando SSE.

**Parámetros de Ruta**:

- `sorter_id` (int): ID del sorter

**Response Headers**:

```
Content-Type: text/event-stream
Cache-Control: no-cache
Connection: keep-alive
```

**Formato de Eventos**:

```
event: lectura
data: {"id":10888,"correlativo":"10888",...}

event: lectura
data: {"id":10889,"correlativo":"10889",...}
```

**Ejemplo JavaScript (EventSource)**:

```javascript
const evtSource = new EventSource(
  "http://localhost:8080/stream/lecturas/sorter/2"
);

evtSource.addEventListener("lectura", (event) => {
  const lectura = JSON.parse(event.data);
  console.log("Nueva lectura:", lectura);
});

evtSource.onerror = (err) => {
  console.error("Error en SSE:", err);
  evtSource.close();
};
```

---

#### 📈 GET `/stream/sku/flow/:sorter_id`

Stream de estadísticas de flujo de SKUs cada 5 segundos.

**Parámetros de Ruta**:

- `sorter_id` (int): ID del sorter

**Formato de Eventos**:

```
event: flow_stats
data: {"sorter_id":2,"total_lecturas":45,"stats":[...]}
```

**Ejemplo JavaScript**:

```javascript
const flowSource = new EventSource("http://localhost:8080/stream/sku/flow/2");

flowSource.addEventListener("flow_stats", (event) => {
  const stats = JSON.parse(event.data);
  console.log("Estadísticas:", stats);
  // Actualizar gráficos, dashboard, etc.
});
```

---

### Health Check

#### 🏥 GET `/health`

Verifica el estado de salud del sistema.

**Respuesta** (200 OK):

```json
{
  "status": "healthy",
  "components": {
    "database": "connected",
    "cognex": "connected",
    "opc_ua": "connected"
  },
  "uptime_seconds": 3600,
  "version": "1.0.0"
}
```

**Ejemplo cURL**:

```bash
curl http://localhost:8080/health
```

---

## 🔌 API WebSocket

### Conexión

**URL Pattern**: `ws://host:port/ws/assignment_{sorter_id}`

**Ejemplo**: `ws://localhost:8080/ws/assignment_2`

### Protocolo de Mensajes

Todos los mensajes son JSON con campo `type` que identifica el tipo de evento.

### Eventos Emitidos por el Servidor

#### 1️⃣ lectura_procesada

Emitido **inmediatamente** después de procesar cada lectura individual.

**Estructura**:

```json
{
  "type": "lectura_procesada",
  "qr_code": "E001;XL;0;CECDCAM5;GREENEX;V018",
  "sku": "XL-V018-CECDCAM5",
  "salida_id": 8,
  "salida_name": "Premium",
  "exitoso": true,
  "timestamp": "2025-10-14T15:23:02Z",
  "sorter_id": 2,
  "correlativo": "10888",
  "calibre": "XL",
  "variedad": "V018",
  "embalaje": "CECDCAM5"
}
```

**Casos de Uso**:

- Animación en tiempo real de cajas en cinta transportadora
- Contador de cajas procesadas
- Historial de últimas lecturas
- Alertas de lecturas fallidas

---

#### 2️⃣ sku_assigned

Emitido cuando cambia la configuración de SKUs (asignación/desasignación) o cuando se actualizan porcentajes de flujo.

**Frecuencia**: Cada 5 segundos (con actualización de flow stats) + eventos de cambio de configuración

**Estructura**:

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
      "estado": 1,
      "bloqueo": false,
      "skus": [
        {
          "sku": "XL-V018-CECDCAM5",
          "percentage": 67.0
        },
        {
          "sku": "L-V022-CEMGKAM5",
          "percentage": 33.0
        }
      ],
      "count": 245
    }
  ]
}
```

**Campos Importantes**:

- `estado`: 0=APAGADO, 1=ANDANDO, 2=FALLA
- `bloqueo`: `true` si salida bloqueada
- `percentage`: Porcentaje de flujo en ventana de 20 segundos
- `count`: Total de cajas procesadas en esta salida

**Casos de Uso**:

- Mostrar configuración actual de salidas
- Indicadores visuales de estado de salidas
- Gráficos de distribución de SKUs
- Dashboard de gestión

---

#### 3️⃣ sku_flow_stats

Emitido cada **5 segundos** con estadísticas agregadas de flujo.

**Estructura**:

```json
{
  "type": "sku_flow_stats",
  "sorter_id": 2,
  "timestamp": "2025-10-14T15:30:00Z",
  "total_lecturas": 45,
  "window_seconds": 20,
  "stats": [
    {
      "sku": "XL-V018-CECDCAM5",
      "lecturas": 30,
      "porcentaje": 67
    },
    {
      "sku": "L-V022-CEMGKAM5",
      "lecturas": 15,
      "porcentaje": 33
    }
  ]
}
```

**Cálculo de Porcentajes**:

- Ventana deslizante de **20 segundos** (configurable)
- Solo considera lecturas dentro de la ventana
- Porcentajes redondeados a entero
- Se recalcula cada 5 segundos

**Casos de Uso**:

- Gráficos de barras/torta de distribución
- Monitoreo de balance de línea
- Detección de desequilibrios
- KPIs en tiempo real

---

### Ejemplo de Cliente WebSocket

#### JavaScript (Browser)

```javascript
// Conectar al WebSocket
const socket = new WebSocket("ws://localhost:8080/ws/assignment_2");

// Evento: Conexión establecida
socket.onopen = () => {
  console.log("✅ Conectado al WebSocket del Sorter #2");
};

// Evento: Mensaje recibido
socket.onmessage = (event) => {
  const message = JSON.parse(event.data);

  switch (message.type) {
    case "lectura_procesada":
      // Procesar lectura individual
      console.log("📦 Nueva caja:", message.sku, "→", message.salida_name);
      actualizarAnimacion(message);
      incrementarContador(message.exitoso);
      break;

    case "sku_assigned":
      // Actualizar configuración de salidas
      console.log("⚙️ Configuración actualizada");
      actualizarGridSalidas(message.salidas);
      break;

    case "sku_flow_stats":
      // Actualizar estadísticas
      console.log("📊 Stats:", message.stats);
      actualizarGraficos(message.stats);
      break;

    default:
      console.warn("⚠️ Tipo de mensaje desconocido:", message.type);
  }
};

// Evento: Error
socket.onerror = (error) => {
  console.error("❌ Error WebSocket:", error);
};

// Evento: Desconexión
socket.onclose = (event) => {
  console.log("🔌 Desconectado:", event.code, event.reason);
  // Reconectar después de 5 segundos
  setTimeout(() => {
    console.log("🔄 Reconectando...");
    location.reload();
  }, 5000);
};

// Funciones auxiliares
function actualizarAnimacion(lectura) {
  const caja = crearElementoCaja(lectura);
  cinta.appendChild(caja);
  animarHastaSalida(caja, lectura.salida_id);
}

function actualizarGridSalidas(salidas) {
  salidas.forEach((salida) => {
    const elem = document.querySelector(`#salida-${salida.id}`);
    elem.classList.toggle("bloqueada", salida.bloqueo);
    elem.classList.toggle("falla", salida.estado === 2);
    elem.querySelector(".count").textContent = salida.count;
  });
}

function actualizarGraficos(stats) {
  const labels = stats.map((s) => s.sku);
  const valores = stats.map((s) => s.porcentaje);
  chartDeTorta.data.labels = labels;
  chartDeTorta.data.datasets[0].data = valores;
  chartDeTorta.update();
}
```

#### Node.js

```javascript
const WebSocket = require("ws");

const ws = new WebSocket("ws://localhost:8080/ws/assignment_2");

ws.on("open", () => {
  console.log("✅ Conectado");
});

ws.on("message", (data) => {
  const message = JSON.parse(data);

  if (message.type === "lectura_procesada") {
    console.log(`📦 ${message.sku} → ${message.salida_name}`);
  }
});

ws.on("error", (error) => {
  console.error("❌ Error:", error);
});

ws.on("close", () => {
  console.log("🔌 Desconectado");
});
```

#### Python

```python
import websocket
import json

def on_message(ws, message):
    data = json.loads(message)

    if data['type'] == 'lectura_procesada':
        print(f"📦 {data['sku']} → {data['salida_name']}")
    elif data['type'] == 'sku_flow_stats':
        print(f"📊 Total lecturas: {data['total_lecturas']}")

def on_error(ws, error):
    print(f"❌ Error: {error}")

def on_close(ws, close_status_code, close_msg):
    print("🔌 Desconectado")

def on_open(ws):
    print("✅ Conectado")

ws = websocket.WebSocketApp(
    "ws://localhost:8080/ws/assignment_2",
    on_open=on_open,
    on_message=on_message,
    on_error=on_error,
    on_close=on_close
)

ws.run_forever()
```

---

## 🔧 Protocolo TCP Cognex

### Especificación del Protocolo

**Transport**: TCP Socket

**Puerto**: 2001 (configurable)

**Encoding**: ASCII

**Terminador de Línea**: `\r\n` o `\n`

**Timeout de Lectura**: 30 segundos

### Formato de Mensaje

```
ESPECIE;CALIBRE;NUMERO;EMBALAJE;MARCA;VARIEDAD
```

**Campos**:

| Campo    | Posición | Descripción                  | Ejemplo             | Requerido |
| -------- | -------- | ---------------------------- | ------------------- | --------- |
| ESPECIE  | 0        | Código de especie (4 chars)  | `E001`              | ✅        |
| CALIBRE  | 1        | Tamaño del producto          | `XL`, `L`, `M`, `S` | ✅        |
| NUMERO   | 2        | Número de serie (ignorado)   | `0`                 | ❌        |
| EMBALAJE | 3        | Código de empaque (8 chars)  | `CECDCAM5`          | ✅        |
| MARCA    | 4        | Identificador de marca       | `GREENEX`           | ✅        |
| VARIEDAD | 5        | Código de variedad (4 chars) | `V018`              | ✅        |

### Ejemplos de Mensajes Válidos

```
E001;XL;0;CECDCAM5;GREENEX;V018
E003;L;0;CEMGKAM5;SELECT;V022
E005;M;0;CEMDSAM4;PREMIUM;V024
E007;S;0;CECGKAM5;CHLION;V018
```

### Mensaje de No-Lectura

Cuando el lector Cognex no puede leer el código QR:

```
NO_READ
```

### Generación de SKU

El sistema convierte el mensaje Cognex en un SKU único:

```
SKU = CALIBRE + "-" + VARIEDAD + "-" + EMBALAJE
```

**Ejemplos**:

- `E001;XL;0;CECDCAM5;GREENEX;V018` → `XL-V018-CECDCAM5`
- `E003;L;0;CEMGKAM5;SELECT;V022` → `L-V022-CEMGKAM5`

### Validación de Mensajes

El sistema valida automáticamente:

1. **Número de campos**: Debe tener exactamente 6 campos separados por `;`
2. **Longitud de ESPECIE**: 4 caracteres
3. **Longitud de EMBALAJE**: 8 caracteres
4. **Longitud de VARIEDAD**: 4 caracteres
5. **CALIBRE válido**: Debe existir en tabla `calibres` de BD

### Tipos de Error

| Tipo            | Descripción               | Acción                 |
| --------------- | ------------------------- | ---------------------- |
| `NO_READ`       | Cognex no pudo leer QR    | → Salida de descarte   |
| `ERROR_FORMATO` | Mensaje no cumple formato | → Salida de descarte   |
| `ERROR_SKU`     | SKU no existe en BD       | → Salida de descarte   |
| `ERROR_DB`      | Error al consultar BD     | → Log error, continuar |

### Simulador de Cognex

El sistema incluye un simulador para testing:

**Iniciar Simulador Web**:

```bash
go run cmd/cognex_web_simulator/main.go
```

**URL**: http://localhost:9091

**Características**:

- Envío manual de mensajes
- Modo ráfaga (burst) configurable
- Simulación de NO_READ aleatorio
- Control de intervalo entre mensajes

---

## ⚙️ Protocolo OPC UA (PLC)

### Especificación

**Protocol**: OPC UA Binary

**Puerto**: 4840 (default)

**Security Mode**: None (desarrollo)

**Security Policy**: None (desarrollo)

**Endpoint**: `opc.tcp://<plc_ip>:4840`

### Operaciones Soportadas

#### 1. Lectura de Nodos (Read)

**Método**: `ReadNode(ctx, nodeID)`

**Ejemplo**:

```go
nodeInfo, err := client.ReadNode(ctx, "ns=3;s=DB_Sorter1.Salida_1.Estado")

// Resultado
nodeInfo.NodeID    // "ns=3;s=DB_Sorter1.Salida_1.Estado"
nodeInfo.Value     // 1 (int16)
nodeInfo.ValueType // "int16"
```

**Nodos Monitoreados**:

- `ns=3;s=DB_Sorter{N}.Salida_{M}.Estado` - Estado salida (0=OFF, 1=ON, 2=FALLA)
- `ns=3;s=DB_Sorter{N}.Salida_{M}.Bloqueo` - Bloqueo salida (bool)

#### 2. Escritura de Nodos (Write)

**Método**: `WriteNode(ctx, nodeID, value)`

**Ejemplo**:

```go
// Activar salida física #5 para Sorter #2
err := client.WriteNode(ctx, "ns=3;s=DB_Sorter2.Entrada.Carril", int16(5))
```

**Nodos de Escritura**:

- `ns=3;s=DB_Sorter{N}.Entrada.Carril` - Número de carril a activar (int16)

#### 3. Suscripciones (Monitor)

El sistema usa **UNA suscripción compartida** para monitorear múltiples nodos simultáneamente.

**Configuración**:

- Intervalo de muestreo: 100ms
- Nodes por suscripción: ~26 nodos (13 salidas × 2 tipos)

**Ventajas**:

- ✅ Evita error `StatusBadTooManySubscriptions`
- ✅ Reduce overhead de red
- ✅ Menor latencia promedio

**Ejemplo de Uso**:

```go
nodeIDs := []string{
    "ns=3;s=DB_Sorter1.Salida_1.Estado",
    "ns=3;s=DB_Sorter1.Salida_1.Bloqueo",
    "ns=3;s=DB_Sorter1.Salida_2.Estado",
    "ns=3;s=DB_Sorter1.Salida_2.Bloqueo",
    // ... más nodos
}

dataChan, cancelFunc, err := client.MonitorMultipleNodes(ctx, nodeIDs, 100*time.Millisecond)

// Procesar notificaciones
for nodeInfo := range dataChan {
    fmt.Printf("Nodo %s cambió a %v\n", nodeInfo.NodeID, nodeInfo.Value)
}
```

### Mapeo de Salidas Físicas

Cada salida lógica del sistema se mapea a una salida física del PLC:

**Tabla salidas**:

```sql
CREATE TABLE salidas (
    id INTEGER PRIMARY KEY,                    -- ID lógico (1, 2, 3...)
    sealer_physical_id INTEGER NOT NULL,       -- ID físico del PLC (1-13)
    salida_sorter VARCHAR(100) NOT NULL,       -- Nombre ("Premium", "Standard")
    tipo VARCHAR(50) NOT NULL,                 -- "automatico" o "manual"
    estado_node VARCHAR(255),                  -- NodeID de estado OPC UA
    bloqueo_node VARCHAR(255)                  -- NodeID de bloqueo OPC UA
);
```

**Ejemplo de Configuración**:

```yaml
salidas:
  - id: 8 # ID lógico
    nombre: "Premium"
    tipo: "automatico"
    sealer_physical_id: 1 # Canal físico del PLC
    estado_node: "ns=3;s=DB_Sorter1.Salida_1.Estado"
    bloqueo_node: "ns=3;s=DB_Sorter1.Salida_1.Bloqueo"
```

### Flujo de Señalización

```mermaid
sequenceDiagram
    Sorter->>Sorter: Determinar Salida (ID: 8)
    Sorter->>Salida: Obtener sealer_physical_id (1)
    Sorter->>PLC: WriteNode(Carril, 1)
    PLC-->>Actuador: Activar Carril #1
    Note over PLC,Actuador: Señal física para<br/>desviar caja
```

### Monitoreo de Estado

El sistema monitorea continuamente el estado de cada salida:

**Estados Posibles**:

- `0 = APAGADO`: Salida no operativa
- `1 = ANDANDO`: Salida funcionando normalmente
- `2 = FALLA`: Salida en estado de error

**Logs de Cambio de Estado**:

```
🔄 Sorter #1: Salida 8 (Premium) - ESTADO cambió: 1→2 (FALLA)
🔒 Sorter #1: Salida 8 (Premium) - BLOQUEO cambió: false→true (BLOQUEADA)
```

### Lógica de Disponibilidad

Una salida se considera **disponible** si:

```go
func (s *Salida) IsAvailable() bool {
    estado := s.GetEstado()
    bloqueo := s.GetBloqueo()
    return estado == 1 && !bloqueo  // ANDANDO y NO bloqueada
}
```

**Comportamiento del Sistema**:

- ✅ Salida disponible: Se asigna normalmente
- ⚠️ Salida no disponible: Se busca alternativa o se envía a descarte

---

## 💾 Base de Datos PostgreSQL

### Conexión

**Driver**: `pgx/v5`

**URL Format**: `postgresql://user:password@host:port/database?sslmode=disable`

**Pool Configuration**:

```yaml
database:
  postgres:
    url: "postgresql://greenex:password@localhost:5432/greenex?sslmode=disable"
    min_conns: 2
    max_conns: 10
    connect_timeout: "10s"
    healthcheck_interval: "30s"
```

### Esquema de Tablas

#### Tabla: `sorters`

```sql
CREATE TABLE sorters (
    id INTEGER PRIMARY KEY,
    ubicacion VARCHAR(255) NOT NULL
);
```

**Ejemplo de Datos**:

```sql
INSERT INTO sorters (id, ubicacion) VALUES
(1, 'Linea-A'),
(2, 'Linea-B');
```

---

#### Tabla: `salidas`

```sql
CREATE TABLE salidas (
    id INTEGER PRIMARY KEY,
    sorter_id INTEGER REFERENCES sorters(id),
    sealer_physical_id INTEGER NOT NULL,
    salida_sorter VARCHAR(100) NOT NULL,
    tipo VARCHAR(50) NOT NULL,         -- 'automatico' o 'manual'
    batch_size INTEGER DEFAULT 1,
    activo BOOLEAN DEFAULT true,
    estado_node VARCHAR(255),
    bloqueo_node VARCHAR(255)
);
```

**Ejemplo de Datos**:

```sql
INSERT INTO salidas VALUES
(8, 2, 1, 'Premium', 'automatico', 10, true,
 'ns=3;s=DB_Sorter2.Salida_1.Estado',
 'ns=3;s=DB_Sorter2.Salida_1.Bloqueo'),
(9, 2, 2, 'Standard', 'automatico', 5, true,
 'ns=3;s=DB_Sorter2.Salida_2.Estado',
 'ns=3;s=DB_Sorter2.Salida_2.Bloqueo');
```

---

#### Tabla: `sku`

```sql
CREATE TABLE sku (
    sku VARCHAR(100) PRIMARY KEY,
    calibre VARCHAR(50) NOT NULL,
    variedad VARCHAR(50) NOT NULL,
    embalaje VARCHAR(50) NOT NULL,
    estado BOOLEAN DEFAULT true
);
```

**Ejemplo de Datos**:

```sql
INSERT INTO sku VALUES
('XL-V018-CECDCAM5', 'XL', 'V018', 'CECDCAM5', true),
('L-V022-CEMGKAM5', 'L', 'V022', 'CEMGKAM5', true);
```

---

#### Tabla: `salida_sku` (Relación N:M)

```sql
CREATE TABLE salida_sku (
    salida_id INTEGER REFERENCES salidas(id),
    sorter_id INTEGER REFERENCES sorters(id),
    calibre VARCHAR(50) NOT NULL,
    variedad VARCHAR(50) NOT NULL,
    embalaje VARCHAR(50) NOT NULL,
    PRIMARY KEY (salida_id, sorter_id, calibre, variedad, embalaje),
    FOREIGN KEY (calibre, variedad, embalaje)
        REFERENCES sku(calibre, variedad, embalaje)
);
```

**Ejemplo de Datos**:

```sql
INSERT INTO salida_sku VALUES
(8, 2, 'XL', 'V018', 'CECDCAM5'),  -- Premium recibe XL-V018-CECDCAM5
(8, 2, 'L', 'V022', 'CEMGKAM5'),   -- Premium recibe L-V022-CEMGKAM5
(9, 2, 'M', 'V024', 'CEMDSAM4');   -- Standard recibe M-V024-CEMDSAM4
```

---

#### Tabla: `caja` (Registro de Lecturas)

```sql
CREATE TABLE caja (
    id SERIAL PRIMARY KEY,
    correlativo VARCHAR(50) UNIQUE NOT NULL,
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

CREATE INDEX idx_caja_correlativo ON caja(correlativo);
CREATE INDEX idx_caja_sorter_timestamp ON caja(sorter_id, timestamp DESC);
CREATE INDEX idx_caja_sku ON caja(sku);
```

**Ejemplo de Query**:

```sql
-- Últimas 100 lecturas del Sorter #2
SELECT * FROM caja
WHERE sorter_id = 2
ORDER BY timestamp DESC
LIMIT 100;

-- Lecturas por SKU en las últimas 24 horas
SELECT sku, COUNT(*) as total
FROM caja
WHERE sorter_id = 2
  AND timestamp > NOW() - INTERVAL '24 hours'
GROUP BY sku
ORDER BY total DESC;
```

---

#### Tabla: `salida_caja` (Registro Físico)

Registra qué caja física fue enviada a qué salida física del PLC.

```sql
CREATE TABLE salida_caja (
    id SERIAL PRIMARY KEY,
    correlativo VARCHAR(50) NOT NULL,
    salida_id INTEGER REFERENCES salidas(id),
    sealer_physical_id INTEGER NOT NULL,
    timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (correlativo) REFERENCES caja(correlativo)
);

CREATE INDEX idx_salida_caja_correlativo ON salida_caja(correlativo);
```

**Uso**:

```go
// Al activar salida física en PLC
err := pgManager.InsertSalidaCaja(ctx, correlativo, salida.ID, salida.SealerPhysicalID)
```

---

### Consultas Comunes

#### Obtener SKUs Asignadas a un Sorter

```sql
SELECT
    s.sku,
    s.calibre,
    s.variedad,
    s.embalaje,
    sal.id as salida_id,
    sal.salida_sorter as salida_nombre
FROM salida_sku ss
JOIN sku s ON (ss.calibre = s.calibre
           AND ss.variedad = s.variedad
           AND ss.embalaje = s.embalaje)
JOIN salidas sal ON ss.salida_id = sal.id
WHERE ss.sorter_id = $1;
```

#### Estadísticas de Producción

```sql
-- Total de cajas por salida (hoy)
SELECT
    sal.salida_sorter,
    COUNT(*) as total_cajas,
    COUNT(*) FILTER (WHERE c.exitoso = true) as exitosas,
    COUNT(*) FILTER (WHERE c.exitoso = false) as fallidas
FROM caja c
JOIN salidas sal ON c.salida_id = sal.id
WHERE c.sorter_id = $1
  AND c.timestamp::date = CURRENT_DATE
GROUP BY sal.salida_sorter;
```

---

## ⚙️ Configuración

### Archivo `config/config.yaml`

```yaml
# Configuración de Base de Datos
database:
  postgres:
    url: "postgresql://greenex:password@localhost:5432/greenex?sslmode=disable"
    min_conns: 2
    max_conns: 10
    connect_timeout: "10s"
    healthcheck_interval: "30s"

# Configuración de Servidor HTTP
http:
  host: "0.0.0.0"
  port: 8080
  read_timeout: "30s"
  write_timeout: "30s"

# OPC UA (PLC)
opc_ua:
  endpoints:
    - sorter_id: 1
      endpoint: "opc.tcp://192.168.1.50:4840"
    - sorter_id: 2
      endpoint: "opc.tcp://192.168.1.51:4840"

# Dispositivos Cognex
cognex_devices:
  - id: 1
    name: "Cognex-Sorter-1"
    ubicacion: "Linea-A"
    host: "192.168.1.100"
    port: 2001
    scan_method: "DATAMAN"
    timeout: "30s"

  - id: 2
    name: "Cognex-Sorter-2"
    ubicacion: "Linea-B"
    host: "192.168.1.101"
    port: 2001
    scan_method: "DATAMAN"
    timeout: "30s"

# Configuración de Sorters
sorters:
  - id: 1
    name: "Sorter Principal"
    ubicacion: "Linea-A"
    cognex_id: 1
    plc_input_node: "ns=3;s=DB_Sorter1.Entrada.Carril"
    flow_stats:
      calculation_interval: "5s"
      window_duration: "20s"
    salidas:
      - id: 1
        nombre: "Premium"
        tipo: "automatico"
        sealer_physical_id: 1
        batch_size: 10
        estado_node: "ns=3;s=DB_Sorter1.Salida_1.Estado"
        bloqueo_node: "ns=3;s=DB_Sorter1.Salida_1.Bloqueo"

      - id: 2
        nombre: "Standard"
        tipo: "automatico"
        sealer_physical_id: 2
        batch_size: 5
        estado_node: "ns=3;s=DB_Sorter1.Salida_2.Estado"
        bloqueo_node: "ns=3;s=DB_Sorter1.Salida_2.Bloqueo"

      - id: 3
        nombre: "Descarte"
        tipo: "manual"
        sealer_physical_id: 13
        batch_size: 1
        estado_node: "ns=3;s=DB_Sorter1.Salida_13.Estado"
        bloqueo_node: "ns=3;s=DB_Sorter1.Salida_13.Bloqueo"
```

### Variables de Entorno

Crear archivo `.env`:

```bash
# Configuración
CONFIG_FILE=config/config.yaml

# Base de Datos (sobreescribe config.yaml)
DB_URL=postgresql://greenex:password@localhost:5432/greenex?sslmode=disable

# Logging
LOG_LEVEL=info  # debug, info, warn, error
LOG_FORMAT=text # text, json

# Servidor
HTTP_PORT=8080
```

---

## 📚 Ejemplos de Uso

### Ejemplo 1: Asignar SKU a Salida

```bash
# Paso 1: Ver SKUs disponibles
curl http://localhost:8080/sku/assigned/2

# Paso 2: Asignar SKU ID=12345 a Salida ID=8
curl -X POST http://localhost:8080/sku/assign \
  -H "Content-Type: application/json" \
  -d '{
    "sorter_id": 2,
    "sku_id": 12345,
    "salida_id": 8
  }'

# Respuesta:
# {
#   "message": "SKU XL-V018-CECDCAM5 asignada a salida Premium (ID: 8)"
# }

# Paso 3: Verificar asignación
curl http://localhost:8080/sku/assigned/2
```

---

### Ejemplo 2: Monitorear Lecturas en Tiempo Real (WebSocket)

```html
<!DOCTYPE html>
<html>
  <head>
    <title>Monitor de Sorter #2</title>
  </head>
  <body>
    <h1>Sorter #2 - Monitor en Vivo</h1>

    <div id="stats">
      <p>Total Lecturas: <span id="total">0</span></p>
      <p>Exitosas: <span id="exitosas">0</span></p>
      <p>Fallidas: <span id="fallidas">0</span></p>
    </div>

    <div id="ultimas-lecturas"></div>

    <script>
      let totalLecturas = 0;
      let exitosas = 0;
      let fallidas = 0;

      const socket = new WebSocket("ws://localhost:8080/ws/assignment_2");

      socket.onmessage = (event) => {
        const msg = JSON.parse(event.data);

        if (msg.type === "lectura_procesada") {
          totalLecturas++;

          if (msg.exitoso) {
            exitosas++;
          } else {
            fallidas++;
          }

          // Actualizar contadores
          document.getElementById("total").textContent = totalLecturas;
          document.getElementById("exitosas").textContent = exitosas;
          document.getElementById("fallidas").textContent = fallidas;

          // Mostrar última lectura
          const div = document.getElementById("ultimas-lecturas");
          const lectura = document.createElement("div");
          lectura.innerHTML = `
          <strong>${msg.sku}</strong> → ${msg.salida_name}
          <small>${new Date(msg.timestamp).toLocaleTimeString()}</small>
        `;
          div.prepend(lectura);

          // Mantener solo últimas 10
          if (div.children.length > 10) {
            div.removeChild(div.lastChild);
          }
        }
      };
    </script>
  </body>
</html>
```

---

### Ejemplo 3: Dashboard con Estadísticas (SSE)

```javascript
// dashboard.js
const sorterID = 2;

// Conectar a stream de lecturas
const lecturaSource = new EventSource(
  `http://localhost:8080/stream/lecturas/sorter/${sorterID}`
);

lecturaSource.addEventListener("lectura", (event) => {
  const lectura = JSON.parse(event.data);
  agregarATabla(lectura);
});

// Conectar a stream de estadísticas
const statsSource = new EventSource(
  `http://localhost:8080/stream/sku/flow/${sorterID}`
);

statsSource.addEventListener("flow_stats", (event) => {
  const stats = JSON.parse(event.data);
  actualizarGrafico(stats);
});

function agregarATabla(lectura) {
  const tabla = document.getElementById("tabla-lecturas");
  const fila = tabla.insertRow(0);
  fila.innerHTML = `
    <td>${lectura.correlativo}</td>
    <td>${lectura.sku}</td>
    <td>${lectura.salida_nombre}</td>
    <td>${lectura.timestamp}</td>
    <td>${lectura.exitoso ? "✅" : "❌"}</td>
  `;
}

function actualizarGrafico(stats) {
  const labels = stats.stats.map((s) => s.sku);
  const valores = stats.stats.map((s) => s.porcentaje);

  chart.data.labels = labels;
  chart.data.datasets[0].data = valores;
  chart.update();
}
```

---

### Ejemplo 4: Consultar Historial

```bash
# Lecturas de las últimas 24 horas
curl "http://localhost:8080/lecturas/sorter/2?start_date=$(date -u -d '24 hours ago' +%Y-%m-%dT%H:%M:%SZ)"

# Solo lecturas fallidas
curl "http://localhost:8080/lecturas/sorter/2?exitoso=false&limit=50"

# Buscar por correlativo
curl "http://localhost:8080/lecturas/correlativo/10888"
```

---

### Ejemplo 5: Integración con Python

```python
import requests
import json

# Configuración
BASE_URL = "http://localhost:8080"
SORTER_ID = 2

# Obtener SKUs asignadas
response = requests.get(f"{BASE_URL}/sku/assigned/{SORTER_ID}")
skus = response.json()

print(f"Sorter #{skus['sorter_id']} tiene {len(skus['skus'])} SKUs asignadas:")
for sku in skus['skus']:
    print(f"  - {sku['sku']}: {sku['percentage']:.0f}%")

# Asignar nueva SKU
payload = {
    "sorter_id": SORTER_ID,
    "sku_id": 12345,
    "salida_id": 8
}

response = requests.post(
    f"{BASE_URL}/sku/assign",
    json=payload,
    headers={"Content-Type": "application/json"}
)

if response.status_code == 200:
    print("✅ SKU asignada correctamente")
else:
    print(f"❌ Error: {response.json()['error']}")

# Obtener lecturas recientes
response = requests.get(f"{BASE_URL}/lecturas/sorter/{SORTER_ID}?limit=10")
lecturas = response.json()

print(f"\nÚltimas {len(lecturas['lecturas'])} lecturas:")
for lect in lecturas['lecturas']:
    status = "✅" if lect['exitoso'] else "❌"
    print(f"  {status} {lect['sku']} → {lect['salida_nombre']}")
```

---

## 🚀 Instalación y Ejecución

### Requisitos del Sistema

- **Go**: 1.22 o superior
- **PostgreSQL**: 14 o superior
- **PLC Siemens** (opcional para desarrollo)
- **Cognex DataMan** (opcional - puede usar simulador)

### Paso 1: Clonar Repositorio

```bash
git clone https://github.com/tu-repo/API-Greenex.git
cd API-Greenex
```

### Paso 2: Configurar Base de Datos

```bash
# Crear base de datos
psql -U postgres -c "CREATE DATABASE greenex;"

# Crear tablas
psql -U postgres -d greenex -f DB/greendex_db_table_create.sql

# Cargar datos de prueba (opcional)
psql -U postgres -d greenex -f DB/sample_data.sql
```

### Paso 3: Configurar Variables de Entorno

```bash
cp .env.example .env
nano .env

# Editar valores:
DB_URL=postgresql://postgres:password@localhost:5432/greenex?sslmode=disable
HTTP_PORT=8080
```

### Paso 4: Compilar

```bash
# Compilar servidor principal
go build -o bin/api-greenex cmd/main.go

# Compilar simulador de Cognex
go build -o bin/cognex-web-simulator cmd/cognex_web_simulator/main.go
```

### Paso 5: Ejecutar

```bash
# Terminal 1: Servidor principal
./bin/api-greenex

# Terminal 2: Simulador Cognex (opcional)
./bin/cognex-web-simulator
```

### Paso 6: Verificar

```bash
# Health check
curl http://localhost:8080/health

# Abrir visualizador
open http://localhost:8080/web/visualizer.html?sorter=1
```

---

## 📊 Logs del Sistema

### Formato de Logs

```
[YYYY/MM/DD HH:MM:SS] [Componente] Mensaje
```

**Ejemplos**:

```
2025/10/14 15:23:02 🚀 Sorter #2: Iniciando en Linea-B
2025/10/14 15:23:02 📍 Sorter #2: Salidas configuradas: 13
2025/10/14 15:23:02 ✅ Sorter #2: Iniciado y escuchando eventos de Cognex
2025/10/14 15:23:05 ✅ Sorter #2: Lectura #10888 | SKU: XL-V018-CECDCAM5 | Salida: Premium (ID: 8)
2025/10/14 15:23:05 📤 [Sorter #2] Señal PLC enviada → Salida 8 (PhysicalID=1)
2025/10/14 15:23:06 🔄 Sorter #2: Salida 8 (Premium) - ESTADO cambió: 1→1 (ANDANDO)
2025/10/14 15:23:10 📊 Sorter #2: Stats: Total=10 | Exitosas=9 | Fallidas=1 | Tasa=90.0%
```

### Niveles de Log

| Emoji | Nivel | Descripción           |
| ----- | ----- | --------------------- |
| 🚀    | INFO  | Inicio de componentes |
| ✅    | INFO  | Operación exitosa     |
| 📍    | INFO  | Configuración         |
| 📊    | INFO  | Estadísticas          |
| 🔄    | INFO  | Cambio de estado      |
| ⚠️    | WARN  | Advertencia           |
| ❌    | ERROR | Error                 |
| 🔒    | INFO  | Bloqueo/desbloqueo    |

### Filtrar Logs por Sorter

```bash
# Ver solo logs del Sorter #2
./bin/api-greenex 2>&1 | grep "Sorter #2:"

# Ver solo lecturas
./bin/api-greenex 2>&1 | grep "Lectura #"

# Ver solo errores
./bin/api-greenex 2>&1 | grep "❌"
```

---

## 🔧 Troubleshooting

### Problema: No se conecta a PLC

**Síntomas**:

```
❌ Error al conectar a opc.tcp://192.168.1.50:4840: dial tcp: connection refused
```

**Solución**:

1. Verificar que PLC esté encendido y en red
2. Hacer ping: `ping 192.168.1.50`
3. Verificar firewall en PLC
4. Verificar configuración en `config.yaml`

---

### Problema: WebSocket retorna 404

**Síntomas**:

```javascript
WebSocket connection failed: 404 Not Found
```

**Solución**:

- Verificar URL correcta: `ws://host:port/ws/assignment_{sorter_id}`
- Reemplazar `{sorter_id}` con número real: `ws://localhost:8080/ws/assignment_2`

---

### Problema: SKU no se puede asignar

**Error**:

```json
{
  "error": "no se puede asignar SKU REJECT (ID=0) a salida automática"
}
```

**Solución**:

- SKU REJECT solo puede ir a salidas manuales
- Asignar a salida tipo `manual` en lugar de `automatico`

---

### Problema: Alta latencia en WebSocket

**Síntomas**:

- Mensajes llegan con retraso
- Canal lleno: `⚠️ Sorter #2: Canal lleno, SKUs no publicados`

**Solución**:

1. Aumentar buffer de canales en código:
   ```go
   skuChannel := make(chan []models.SKUAssignable, 50) // Era 10
   ```
2. Reducir frecuencia de estadísticas en `config.yaml`:
   ```yaml
   flow_stats:
     calculation_interval: "10s" # Era 5s
   ```
