# 🎯 Guía Rápida: Consultas al Servidor Dummy Serfruit

## 🚀 Iniciar el Servidor

```bash
# Opción 1: Ejecutar directamente
go run cmd/serfruit/main.go

# Opción 2: Compilar y ejecutar
go build -o bin/serfruit cmd/serfruit/main.go
./bin/serfruit
```

**Puerto:** 9093  
**URL Base:** http://localhost:9093  
**Mesas disponibles:** 6 (IDs: 1, 2, 3, 4, 5, 6)

---

## 📝 Ejemplos de Consultas

### 1️⃣ **Consultar Todas las Mesas**

```bash
curl -s http://localhost:9093/Mesa/Estado?id=0 | jq '.'
```

**Respuesta:**
```json
[
  {
    "idMesa": 1,
    "estado": 1,
    "descripcionEstado": "Libre",
    "estadoPLC": 3,
    "descripcionEstadoPLC": "Automático",
    "datosProduccion": {
      "numeroPaleActual": 0,
      "numeroCajasEnPale": 0,
      "totalPalesFinalizados": 0,
      "totalCajasPaletizadas": 0
    },
    "datosPaletizado": {
      "cajasPorPale": 0,
      "codigoTipoEnvase": "",
      "codigoTipoPale": "",
      "cajasPorCapa": 0
    }
  },
  ...
]
```

### 2️⃣ **Consultar Mesa Específica (Libre)**

```bash
curl -s http://localhost:9093/Mesa/Estado?id=1 | jq '.'
```

**Respuesta (mesa sin orden):**
```json
{
  "message": "Mesa NO tiene OF activa"
}
```
**Código HTTP:** 202

### 3️⃣ **Crear Orden de Fabricación**

```bash
curl -X POST http://localhost:9093/Mesa?id=1 \
  -H "Content-Type: application/json" \
  -d '{
    "numeroPales": 5,
    "cajasPerPale": 24,
    "cajasPerCapa": 4,
    "codigoTipoEnvase": "C0068",
    "codigoTipoPale": "C0082",
    "idProgramaFlejado": 1
  }' | jq '.'
```

**Respuesta:**
```json
{
  "mensaje": "Orden creada exitosamente",
  "status": "exito"
}
```

### 4️⃣ **Consultar Mesa con Orden Activa**

```bash
curl -s http://localhost:9093/Mesa/Estado?id=1 | jq '.'
```

**Respuesta:**
```json
{
  "idMesa": 1,
  "estado": 2,
  "descripcionEstado": "Bloqueado (orden activa)",
  "estadoPLC": 3,
  "descripcionEstadoPLC": "Automático",
  "datosProduccion": {
    "numeroPaleActual": 0,
    "numeroCajasEnPale": 0,
    "totalPalesFinalizados": 0,
    "totalCajasPaletizadas": 0
  },
  "datosPaletizado": {
    "cajasPorPale": 24,
    "codigoTipoEnvase": "C0068",
    "codigoTipoPale": "C0082",
    "cajasPorCapa": 4
  }
}
```

### 5️⃣ **Registrar Caja**

```bash
curl -X POST http://localhost:9093/Mesa/NuevaCaja?idMesa=1 \
  -H "Content-Type: application/json" \
  -d '{"idCaja": "CAJA001"}' | jq '.'
```

**Respuesta:**
```json
{
  "mensaje": "La caja ha sido registrada correctamente en la mesa",
  "status": "exito"
}
```

### 6️⃣ **Registrar Múltiples Cajas**

```bash
# Loop para registrar 10 cajas
for i in {1..10}; do
  curl -s -X POST http://localhost:9093/Mesa/NuevaCaja?idMesa=1 \
    -H "Content-Type: application/json" \
    -d "{\"idCaja\": \"CAJA$(printf %03d $i)\"}" | jq -r '.mensaje'
done
```

### 7️⃣ **Ver Progreso en Tiempo Real**

```bash
# Ver solo datos de producción
curl -s http://localhost:9093/Mesa/Estado?id=1 | jq '.datosProduccion'
```

**Respuesta:**
```json
{
  "numeroPaleActual": 0,
  "numeroCajasEnPale": 10,
  "totalPalesFinalizados": 0,
  "totalCajasPaletizadas": 10
}
```

### 8️⃣ **Completar un Palé**

```bash
# Registrar 24 cajas (completa un palé si cajasPerPale=24)
for i in {1..24}; do
  curl -s -X POST http://localhost:9093/Mesa/NuevaCaja?idMesa=1 \
    -H "Content-Type: application/json" \
    -d "{\"idCaja\": \"CAJA$(printf %03d $i)\"}" > /dev/null
done

# Ver resultado
curl -s http://localhost:9093/Mesa/Estado?id=1 | jq '.datosProduccion'
```

**Respuesta (palé completado):**
```json
{
  "numeroPaleActual": 1,
  "numeroCajasEnPale": 0,
  "totalPalesFinalizados": 1,
  "totalCajasPaletizadas": 24
}
```

### 9️⃣ **Vaciar Mesa (Continuar)**

```bash
# Modo 1: Vacía el palé pero continúa la orden
curl -X POST "http://localhost:9093/Mesa/Vaciar?id=1&modo=1" | jq '.'
```

**Respuesta:**
```json
{
  "mensaje": "Solicitud de vaciado registrada correctamente en la mesa",
  "status": "exito"
}
```

### 🔟 **Vaciar Mesa (Finalizar)**

```bash
# Modo 2: Finaliza la orden y libera la mesa
curl -X POST "http://localhost:9093/Mesa/Vaciar?id=1&modo=2" | jq '.'
```

**Respuesta:**
```json
{
  "mensaje": "Solicitud de vaciado registrada correctamente en la mesa",
  "status": "exito"
}
```

---

## 🔍 Consultas Avanzadas con jq

### Ver Solo Estado de Todas las Mesas

```bash
curl -s http://localhost:9093/Mesa/Estado?id=0 | \
  jq '.[] | {idMesa, estado: .descripcionEstado}'
```

### Ver Progreso de Todas las Mesas

```bash
curl -s http://localhost:9093/Mesa/Estado?id=0 | \
  jq '.[] | {
    idMesa, 
    estado: .descripcionEstado,
    paleActual: .datosProduccion.numeroPaleActual,
    cajasEnPale: .datosProduccion.numeroCajasEnPale,
    palesCompletados: .datosProduccion.totalPalesFinalizados
  }'
```

### Filtrar Mesas Ocupadas

```bash
curl -s http://localhost:9093/Mesa/Estado?id=0 | \
  jq '.[] | select(.estado == 2) | {idMesa, descripcionEstado}'
```

### Filtrar Mesas Libres

```bash
curl -s http://localhost:9093/Mesa/Estado?id=0 | \
  jq '.[] | select(.estado == 1) | {idMesa, descripcionEstado}'
```

---

## 🧪 Script de Pruebas Automatizado

```bash
# Ejecutar todas las pruebas
./test_serfruit_dummy.sh
```

**El script prueba:**
- ✅ Consultar todas las mesas
- ✅ Crear órdenes
- ✅ Registrar cajas
- ✅ Completar palés
- ✅ Vaciar mesas
- ✅ Casos de error
- ✅ Múltiples mesas simultáneas

---

## 🌐 Interfaz Web

Abre en tu navegador:
```
http://localhost:9093
```

Verás una página HTML con:
- Descripción del servidor
- Lista de endpoints disponibles
- Estado actual del sistema

---

## 📊 Casos de Uso Comunes

### Flujo Completo: Crear y Completar Orden

```bash
# 1. Verificar mesa libre
curl -s http://localhost:9093/Mesa/Estado?id=1

# 2. Crear orden
curl -X POST http://localhost:9093/Mesa?id=1 \
  -H "Content-Type: application/json" \
  -d '{
    "numeroPales": 2,
    "cajasPerPale": 12,
    "cajasPerCapa": 3,
    "codigoTipoEnvase": "5KG",
    "codigoTipoPale": "EURO",
    "idProgramaFlejado": 1
  }'

# 3. Registrar 12 cajas (completa primer palé)
for i in {1..12}; do
  curl -s -X POST http://localhost:9093/Mesa/NuevaCaja?idMesa=1 \
    -H "Content-Type: application/json" \
    -d "{\"idCaja\": \"BATCH1_$(printf %03d $i)\"}" > /dev/null
done

# 4. Vaciar primer palé (continuar)
curl -X POST "http://localhost:9093/Mesa/Vaciar?id=1&modo=1"

# 5. Registrar 12 cajas más (segundo palé)
for i in {13..24}; do
  curl -s -X POST http://localhost:9093/Mesa/NuevaCaja?idMesa=1 \
    -H "Content-Type: application/json" \
    -d "{\"idCaja\": \"BATCH2_$(printf %03d $i)\"}" > /dev/null
done

# 6. Finalizar orden
curl -X POST "http://localhost:9093/Mesa/Vaciar?id=1&modo=2"

# 7. Verificar mesa libre
curl -s http://localhost:9093/Mesa/Estado?id=1
```

### Probar con Cliente Go

```go
package main

import (
    "context"
    "log"
    "time"
    
    "api-dansort/internal/communication/pallet"
)

func main() {
    client := pallet.NewClient("127.0.0.1", 9093, 10*time.Second)
    defer client.Close()
    
    ctx := context.Background()
    
    // Consultar estado
    estados, err := client.GetEstadoMesa(ctx, 1)
    if err != nil {
        log.Fatal(err)
    }
    
    log.Printf("Mesa 1: %+v", estados[0])
    
    // Crear orden
    orden := pallet.OrdenFabricacionRequest{
        NumeroPales:       5,
        CajasPerPale:      24,
        CajasPerCapa:      4,
        CodigoTipoEnvase:  "C0068",
        CodigoTipoPale:    "C0082",
        IDProgramaFlejado: 1,
    }
    
    err = client.CrearOrdenFabricacion(ctx, 1, orden)
    if err != nil {
        log.Fatal(err)
    }
    
    log.Println("Orden creada exitosamente")
}
```

---

## 🐛 Troubleshooting

### El servidor no responde
```bash
# Verificar si está corriendo
curl http://localhost:9093
# Si falla, iniciar el servidor
go run cmd/serfruit/main.go
```

### Puerto 9093 en uso
```bash
# Ver qué proceso usa el puerto
lsof -i :9093
# O en Linux alternativo
netstat -tulpn | grep 9093
```

### jq no instalado
```bash
# Ubuntu/Debian
sudo apt-get install jq

# macOS
brew install jq
```

---

## 📝 Logs del Servidor

El servidor muestra logs detallados:

```
🚀 Servidor dummy API Paletizado Serfruit iniciado en http://localhost:9093
📋 Endpoints disponibles:
   GET  /Mesa/Estado?id={id}
   POST /Mesa/NuevaCaja?idMesa={id}
   POST /Mesa?id={id}
   POST /Mesa/Vaciar?id={id}&modo={1|2}

➡️  POST /Mesa?id=1 
📋 Orden creada en mesa 1: 5 palés × 24 cajas
⬅️  POST /Mesa?id=1 - 2.5ms

➡️  POST /Mesa/NuevaCaja?idMesa=1 
📦 Caja registrada: CAJA001 (Mesa 1, Palé 1, Caja 1/24)
⬅️  POST /Mesa/NuevaCaja?idMesa=1 - 1.2ms
```

---

**¡Listo para usar! 🎉**

Para empezar:
1. `go run cmd/serfruit/main.go`
2. `./test_serfruit_dummy.sh`
3. O usa los ejemplos de curl de arriba

