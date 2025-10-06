# API-Greenex-Rust

Implementación en Rust del sistema API-Greenex con funcionalidad completa de lectura/escritura OPC UA para WAGO PLCs.

## 🚀 Características

- ✅ Cliente OPC UA completo con reconexión automática
- ✅ Lectura y escritura de nodos escalares WAGO (Boolean, Byte, Int16, Float, String, UInt16)
- ✅ Suscripción a nodos vectoriales WAGO
- ✅ Logs detallados con información de tipos de datos
- ✅ Servidor HTTP con Axum para API REST
- ✅ Procesamiento asíncrono con Tokio
- ✅ Manejo robusto de errores

## 📋 Requisitos

- Rust 1.70+ (rustc y cargo)
- Servidor OPC UA (WAGO PLC o simulador)

## 🔧 Instalación

```bash
cd /home/arbaiter/Documents/Arbeit/2025/API-Greenex-Rust
cargo build --release
```

## ⚙️ Configuración

Crea un archivo `.env` en la raíz del proyecto:

```env
OPCUA_ENDPOINT=opc.tcp://192.168.1.100:4840
HTTP_PORT=8080
RUST_LOG=info
```

## 🏃 Ejecución

### Modo desarrollo (con logs detallados):
```bash
RUST_LOG=debug cargo run
```

### Modo producción:
```bash
cargo run --release
```

## 📡 API Endpoints

### Estado del sistema
```bash
GET http://localhost:8080/status
```

Respuesta:
```json
{
  "status": "OK",
  "service": "API-Greenex-Rust",
  "opcua": {
    "connected": true
  }
}
```

### Escribir en variable WAGO

```bash
# Boolean
POST http://localhost:8080/wago/boleano
Content-Type: application/json
{"value": true}

# Byte (0-255)
POST http://localhost:8080/wago/byte
Content-Type: application/json
{"value": 123}

# Entero (Int16)
POST http://localhost:8080/wago/entero
Content-Type: application/json
{"value": 1500}

# Real (Float)
POST http://localhost:8080/wago/real
Content-Type: application/json
{"value": 25.5}

# String
POST http://localhost:8080/wago/string
Content-Type: application/json
{"value": "Greenex-Test"}

# Word (UInt16, 0-65535)
POST http://localhost:8080/wago/word
Content-Type: application/json
{"value": 30000}
```

## 🔄 Funcionalidades Automáticas

### WAGO Loop
Cada 10 segundos ejecuta:
- Escritura automática en todos los nodos escalares WAGO
- Lectura de vectores WAGO (VectorBool, VectorInt, VectorWord)
- Logs detallados con DataType esperado vs enviado

### Suscripciones
- **Vectores WAGO**: Monitorea cambios en arrays y actualiza nodos escalares
- **Nodos por defecto**: Suscripción a métodos de segregación

### Reconexión Automática
- Keep-alive cada 5 segundos
- Reconexión automática si se pierde la conexión
- Logs claros del estado de conexión

## 📊 Logs

Los logs incluyen información detallada:

```
┌─────────────────────────────────────────────────────────
│ 📝 ESCRITURA WAGO
│ Nodo: ns=4;s=|var|WAGO TEST.Application.DB_OPC.StringTest
│ Tipo esperado (DataType): NodeId(0, 12)
│ Valor a escribir: String("Greenex-123")
│ Tipo Variant: String
└─────────────────────────────────────────────────────────
✅ ÉXITO | Nodo: ns=4;s=|var|WAGO TEST.Application.DB_OPC.StringTest | Valor: String("Greenex-123") | Duración: 5.2ms
```

## 🛠️ Estructura del Proyecto

```
src/
├── main.rs              # Punto de entrada
├── http.rs              # Servidor HTTP y handlers
├── models/
│   ├── mod.rs
│   └── constants.rs     # Constantes OPC UA y WAGO
├── listeners/
│   ├── mod.rs
│   └── opcua.rs         # Cliente OPC UA completo
└── flow/
    ├── mod.rs
    ├── wago.rs          # WAGO Loop (lectura/escritura)
    └── subscription.rs  # Manager de suscripciones
```

## 🔍 Debugging

Para ver todos los logs internos de OPC UA:

```bash
RUST_LOG=opcua=debug,api_greenex_rust=debug cargo run
```

## 🆚 Diferencias con Go

### Ventajas de Rust:
- ✅ **Performance**: ~2-3x más rápido en operaciones I/O
- ✅ **Memoria**: Sin garbage collector, uso de memoria predecible
- ✅ **Seguridad**: Compilador previene race conditions y null pointers
- ✅ **Concurrencia**: Sistema de ownership garantiza thread-safety

### Sistema de Tipos:
```rust
// Rust - Tipos explícitos y seguros
Variant::Boolean(true)
Variant::Int16(1500)
Variant::String("test".into())

// vs Go - Interfaces genéricas
interface{}
```

### Async/Await:
```rust
// Rust - Async nativo con Tokio
async fn write_node(&self, node_id: &str, value: Variant) -> Result<()>

// vs Go - Goroutines
go func() { writeNode(nodeID, value) }()
```

## 📝 Notas de Migración desde Go

1. **Variants**: En Rust usamos `opcua::types::Variant` enum fuertemente tipado
2. **Channels**: `async-channel` reemplaza channels de Go
3. **Mutexes**: `tokio::sync::RwLock` para acceso concurrente
4. **Errors**: `anyhow::Result` para manejo de errores con contexto

## 🐛 Troubleshooting

### Error de conexión OPC UA
```
Error: StatusBadCertificateInvalid
```
**Solución**: Verifica que `trust_server_certs(true)` esté habilitado en el cliente.

### Type Mismatch al escribir
```
Error: StatusBadTypeMismatch
```
**Solución**: Los logs mostrarán el tipo esperado vs enviado. Usa el Variant correcto:
- `Boolean` → `Variant::Boolean`
- `Byte` → `Variant::Byte`
- `Int16` → `Variant::Int16`
- `Float` → `Variant::Float`
- `String` → `Variant::String`
- `UInt16` → `Variant::UInt16`

## 📄 Licencia

Proyecto interno - API-Greenex 2025
