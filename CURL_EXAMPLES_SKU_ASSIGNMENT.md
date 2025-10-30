# 📚 Ejemplos de curl para Sistema de Asignación de SKUs

BASE_URL="http://localhost:8081"

## ═══════════════════════════════════════════════════════════════
## 1. CONSULTAR SKUs DISPONIBLES
## ═══════════════════════════════════════════════════════════════

### 1.1 Obtener SKUs asignables para un sorter
```bash
# Sorter #1
curl -s "$BASE_URL/sku/assignables/1" | jq '.'

# Sorter #2
curl -s "$BASE_URL/sku/assignables/2" | jq '.'
```

**Respuesta esperada:**
```json
{
  "data": {
    "sorter_id": 1,
    "skus": [
      {
        "id": 123456789,
        "sku": "28-30-V018-5KG",
        "calibre": "28-30",
        "variedad": "V018",
        "nombre_variedad": "LAPINS",
        "embalaje": "5KG",
        "dark": 0,
        "linea": "L1",
        "percentage": 0.0,
        "is_assigned": false,
        "is_master_case": false
      },
      {
        "id": 0,
        "sku": "REJECT",
        "dark": 0,
        "percentage": 0.0,
        "is_assigned": false,
        "is_master_case": false
      }
    ]
  },
  "message": "SKUs disponibles obtenidas exitosamente"
}
```

### 1.2 Obtener SKUs ya asignados en un sorter
```bash
curl -s "$BASE_URL/sku/assigned/1" | jq '.'
```

**Filtrar solo SKUs asignados:**
```bash
curl -s "$BASE_URL/sku/assigned/1" | jq '.data.skus[] | select(.is_assigned == true)'
```

**Filtrar solo SKUs disponibles:**
```bash
curl -s "$BASE_URL/sku/assigned/1" | jq '.data.skus[] | select(.is_assigned == false)'
```

---

## ═══════════════════════════════════════════════════════════════
## 2. ASIGNAR SKU A SALIDA
## ═══════════════════════════════════════════════════════════════

### 2.1 Asignar SKU a salida automática
```bash
curl -X POST "$BASE_URL/assignment" \
  -H "Content-Type: application/json" \
  -d '{
    "sku_id": 123456789,
    "sealer_id": 2
  }'
```

**Respuesta esperada:**
```json
{
  "data": {
    "sku_id": 123456789,
    "sealer_id": 2,
    "sorter_id": 1,
    "sku_details": {
      "calibre": "28-30",
      "variedad": "V018",
      "embalaje": "5KG"
    }
  },
  "message": "✅ SKU asignada exitosamente a salida #2"
}
```

### 2.2 Asignar REJECT a salida manual
```bash
curl -X POST "$BASE_URL/assignment" \
  -H "Content-Type: application/json" \
  -d '{
    "sku_id": 0,
    "sealer_id": 1
  }'
```

### 2.3 Asignar múltiples SKUs a la misma salida
```bash
# SKU #1
curl -X POST "$BASE_URL/assignment" \
  -H "Content-Type: application/json" \
  -d '{"sku_id": 123456789, "sealer_id": 2}'

# SKU #2
curl -X POST "$BASE_URL/assignment" \
  -H "Content-Type: application/json" \
  -d '{"sku_id": 987654321, "sealer_id": 2}'

# SKU #3
curl -X POST "$BASE_URL/assignment" \
  -H "Content-Type: application/json" \
  -d '{"sku_id": 111222333, "sealer_id": 2}'
```

### 2.4 Asignar mismo SKU a múltiples salidas (para balance)
```bash
# Asignar SKU a Salida #2
curl -X POST "$BASE_URL/assignment" \
  -H "Content-Type: application/json" \
  -d '{"sku_id": 555666777, "sealer_id": 2}'

# Asignar MISMO SKU a Salida #3
curl -X POST "$BASE_URL/assignment" \
  -H "Content-Type: application/json" \
  -d '{"sku_id": 555666777, "sealer_id": 3}'

# Ahora el SKU se distribuirá entre ambas salidas con round-robin
```

---

## ═══════════════════════════════════════════════════════════════
## 3. ELIMINAR SKU DE SALIDA
## ═══════════════════════════════════════════════════════════════

### 3.1 Eliminar SKU específico de una salida
```bash
# Formato: DELETE /assignment/:sealer_id/:sku_id
curl -X DELETE "$BASE_URL/assignment/2/987654321"
```

**Respuesta esperada:**
```json
{
  "data": {
    "sku_id": 987654321,
    "sealer_id": 2,
    "sorter_id": 1,
    "sku_details": {
      "calibre": "32-34",
      "variedad": "V020",
      "embalaje": "10KG"
    }
  },
  "message": "🗑️ SKU eliminada exitosamente de salida #2"
}
```

### 3.2 Verificar que SKU fue eliminado
```bash
curl -s "$BASE_URL/sku/assigned/1" | \
  jq '.data.skus[] | select(.id == 987654321) | {id, sku, is_assigned}'
```

---

## ═══════════════════════════════════════════════════════════════
## 4. VACIAR TODAS LAS SKUs DE UNA SALIDA
## ═══════════════════════════════════════════════════════════════

### 4.1 Vaciar salida completa
```bash
# Formato: DELETE /assignment/:sealer_id
curl -X DELETE "$BASE_URL/assignment/2"
```

**Respuesta esperada:**
```json
{
  "data": {
    "sealer_id": 2,
    "sorter_id": 1,
    "removed_skus": [
      {
        "calibre": "28-30",
        "variedad": "V018",
        "embalaje": "5KG",
        "dark": 0
      },
      {
        "calibre": "32-34",
        "variedad": "V020",
        "embalaje": "10KG",
        "dark": 0
      }
    ],
    "count": 2
  },
  "message": "🧹 Todas las SKUs eliminadas de salida #2 (REJECT reinsertado automáticamente)"
}
```

**⚠️ Nota:** REJECT se reinserta automáticamente después de vaciar

---

## ═══════════════════════════════════════════════════════════════
## 5. CASOS DE ERROR (PARA PRUEBAS)
## ═══════════════════════════════════════════════════════════════

### 5.1 Intentar asignar REJECT a salida automática (debe fallar)
```bash
curl -X POST "$BASE_URL/assignment" \
  -H "Content-Type: application/json" \
  -d '{
    "sku_id": 0,
    "sealer_id": 2
  }'
```

**Respuesta esperada (ERROR):**
```json
{
  "error": {
    "code": "unprocessable_entity",
    "message": "no se puede asignar SKU REJECT (ID=0) a salida automática 'Salida_2' (ID=2)",
    "details": {
      "sku_id": 0,
      "sealer_id": 2
    }
  }
}
```

### 5.2 Intentar eliminar REJECT (debe fallar)
```bash
curl -X DELETE "$BASE_URL/assignment/1/0"
```

**Respuesta esperada (ERROR):**
```json
{
  "error": {
    "code": "forbidden",
    "message": "no se puede eliminar SKU REJECT (ID=0), está protegida"
  }
}
```

### 5.3 SKU inexistente
```bash
curl -X POST "$BASE_URL/assignment" \
  -H "Content-Type: application/json" \
  -d '{
    "sku_id": 999999,
    "sealer_id": 2
  }'
```

**Respuesta esperada (ERROR):**
```json
{
  "error": {
    "code": "not_found",
    "message": "SKU con ID 999999 no encontrada en las SKUs disponibles del sorter #1"
  }
}
```

### 5.4 Salida inexistente
```bash
curl -X POST "$BASE_URL/assignment" \
  -H "Content-Type: application/json" \
  -d '{
    "sku_id": 123456789,
    "sealer_id": 999
  }'
```

**Respuesta esperada (ERROR):**
```json
{
  "error": {
    "code": "not_found",
    "message": "Salida con ID 999 no encontrada"
  }
}
```

### 5.5 Formato JSON inválido
```bash
curl -X POST "$BASE_URL/assignment" \
  -H "Content-Type: application/json" \
  -d '{
    "invalid_field": 12345
  }'
```

**Respuesta esperada (ERROR):**
```json
{
  "error": {
    "code": "validation_error",
    "message": "sku_id es requerido"
  }
}
```

---

## ═══════════════════════════════════════════════════════════════
## 6. CONSULTAS AVANZADAS CON JQ
## ═══════════════════════════════════════════════════════════════

### 6.1 Listar solo IDs y nombres de SKUs
```bash
curl -s "$BASE_URL/sku/assignables/1" | \
  jq '.data.skus[] | {id, sku, nombre_variedad}'
```

### 6.2 Contar SKUs asignados vs disponibles
```bash
curl -s "$BASE_URL/sku/assigned/1" | \
  jq '{
    total: .data.skus | length,
    asignados: [.data.skus[] | select(.is_assigned == true)] | length,
    disponibles: [.data.skus[] | select(.is_assigned == false)] | length
  }'
```

### 6.3 Filtrar SKUs por calibre
```bash
curl -s "$BASE_URL/sku/assignables/1" | \
  jq '.data.skus[] | select(.calibre == "28-30")'
```

### 6.4 Filtrar SKUs por variedad
```bash
curl -s "$BASE_URL/sku/assignables/1" | \
  jq '.data.skus[] | select(.variedad == "V018")'
```

### 6.5 Mostrar SKUs con porcentaje de flujo
```bash
curl -s "$BASE_URL/sku/assigned/1" | \
  jq '.data.skus[] | select(.percentage > 0) | {sku, percentage}'
```

### 6.6 Ver solo SKUs de una salida específica
```bash
# Esto requiere modificar el backend para incluir info de salida
# Por ahora, inferir desde is_assigned
curl -s "$BASE_URL/sku/assigned/1" | \
  jq '.data.skus[] | select(.is_assigned == true) | {id, sku, calibre, variedad}'
```

---

## ═══════════════════════════════════════════════════════════════
## 7. WEBSOCKET (PRUEBAS EN TIEMPO REAL)
## ═══════════════════════════════════════════════════════════════

### 7.1 Conectarse al WebSocket del Sorter #1
```bash
# Instalar wscat si no lo tienes
npm install -g wscat

# Conectar
wscat -c ws://localhost:8081/ws/sorter-1
```

**Mensajes que recibirás:**
```json
{
  "type": "sku_assigned",
  "sorter_id": 1,
  "timestamp": "2025-10-30T10:30:00Z",
  "data": {
    "skus": [
      {
        "id": 123456789,
        "sku": "28-30-LAPINS-5KG",
        "is_assigned": true,
        "percentage": 45.2
      }
    ]
  }
}
```

### 7.2 Room de estadísticas globales
```bash
wscat -c ws://localhost:8081/ws/stats
```

### 7.3 Probar actualización en tiempo real
```bash
# Terminal 1: Conectar WebSocket
wscat -c ws://localhost:8081/ws/sorter-1

# Terminal 2: Hacer asignación
curl -X POST "$BASE_URL/assignment" \
  -H "Content-Type: application/json" \
  -d '{"sku_id": 123456789, "sealer_id": 2}'

# Terminal 1 recibirá notificación automáticamente
```

---

## ═══════════════════════════════════════════════════════════════
## 8. PRUEBAS DE CARGA (MÚLTIPLES ASIGNACIONES)
## ═══════════════════════════════════════════════════════════════

### 8.1 Asignar múltiples SKUs en paralelo
```bash
# Asignar 5 SKUs simultáneamente
for i in {1..5}; do
  curl -X POST "$BASE_URL/assignment" \
    -H "Content-Type: application/json" \
    -d "{\"sku_id\": $((100000000 + i)), \"sealer_id\": 2}" &
done
wait
```

### 8.2 Ciclo de asignación y eliminación
```bash
SKU_ID=123456789
SEALER_ID=2

# Asignar
curl -X POST "$BASE_URL/assignment" \
  -H "Content-Type: application/json" \
  -d "{\"sku_id\": $SKU_ID, \"sealer_id\": $SEALER_ID}"

sleep 1

# Eliminar
curl -X DELETE "$BASE_URL/assignment/$SEALER_ID/$SKU_ID"

sleep 1

# Asignar de nuevo
curl -X POST "$BASE_URL/assignment" \
  -H "Content-Type: application/json" \
  -d "{\"sku_id\": $SKU_ID, \"sealer_id\": $SEALER_ID}"
```

---

## ═══════════════════════════════════════════════════════════════
## 9. VERIFICACIÓN DE CONSISTENCIA
## ═══════════════════════════════════════════════════════════════

### 9.1 Verificar que REJECT siempre está presente
```bash
curl -s "$BASE_URL/sku/assigned/1" | \
  jq '.data.skus[] | select(.sku == "REJECT")'
```

### 9.2 Verificar estado antes y después de vaciar
```bash
# Antes
echo "ANTES DE VACIAR:"
curl -s "$BASE_URL/sku/assigned/1" | \
  jq '.data.skus[] | select(.is_assigned == true) | {sku}'

# Vaciar salida
curl -X DELETE "$BASE_URL/assignment/2"

# Después
echo "DESPUÉS DE VACIAR:"
curl -s "$BASE_URL/sku/assigned/1" | \
  jq '.data.skus[] | select(.is_assigned == true) | {sku}'
```

### 9.3 Comparar SKUs entre sorters
```bash
echo "=== SORTER 1 ==="
curl -s "$BASE_URL/sku/assigned/1" | \
  jq '.data.skus[] | select(.is_assigned == true) | .sku'

echo "=== SORTER 2 ==="
curl -s "$BASE_URL/sku/assigned/2" | \
  jq '.data.skus[] | select(.is_assigned == true) | .sku'
```

---

## ═══════════════════════════════════════════════════════════════
## 10. DEBUGGING Y TROUBLESHOOTING
## ═══════════════════════════════════════════════════════════════

### 10.1 Ver respuesta completa con headers
```bash
curl -v "$BASE_URL/sku/assignables/1"
```

### 10.2 Ver solo código de respuesta HTTP
```bash
curl -s -o /dev/null -w "%{http_code}" "$BASE_URL/sku/assignables/1"
```

### 10.3 Guardar respuesta en archivo
```bash
curl -s "$BASE_URL/sku/assigned/1" > response.json
cat response.json | jq '.'
```

### 10.4 Timing de request
```bash
curl -w "\nTiempo: %{time_total}s\n" -s "$BASE_URL/sku/assignables/1" | jq '.'
```

### 10.5 Ver todos los headers de respuesta
```bash
curl -I "$BASE_URL/sku/assignables/1"
```

---

## 🎯 EJEMPLOS DE FLUJOS COMPLETOS

### Flujo 1: Configurar Salida desde Cero
```bash
# 1. Ver SKUs disponibles
curl -s "$BASE_URL/sku/assignables/1" | jq '.data.skus[0:5]'

# 2. Asignar primer SKU
curl -X POST "$BASE_URL/assignment" \
  -H "Content-Type: application/json" \
  -d '{"sku_id": 123456789, "sealer_id": 2}'

# 3. Asignar segundo SKU
curl -X POST "$BASE_URL/assignment" \
  -H "Content-Type: application/json" \
  -d '{"sku_id": 987654321, "sealer_id": 2}'

# 4. Verificar configuración
curl -s "$BASE_URL/sku/assigned/1" | \
  jq '.data.skus[] | select(.is_assigned == true)'
```

### Flujo 2: Balance entre Múltiples Salidas
```bash
# 1. Asignar SKU a Salida #2
curl -X POST "$BASE_URL/assignment" \
  -H "Content-Type: application/json" \
  -d '{"sku_id": 555666777, "sealer_id": 2}'

# 2. Asignar MISMO SKU a Salida #3
curl -X POST "$BASE_URL/assignment" \
  -H "Content-Type: application/json" \
  -d '{"sku_id": 555666777, "sealer_id": 3}'

# 3. Verificar que está en ambas
curl -s "$BASE_URL/sku/assigned/1" | \
  jq '.data.skus[] | select(.id == 555666777)'
```

### Flujo 3: Reconfigurar Salida
```bash
# 1. Vaciar salida actual
curl -X DELETE "$BASE_URL/assignment/2"

# 2. Asignar nuevos SKUs
curl -X POST "$BASE_URL/assignment" \
  -H "Content-Type: application/json" \
  -d '{"sku_id": 111222333, "sealer_id": 2}'

curl -X POST "$BASE_URL/assignment" \
  -H "Content-Type: application/json" \
  -d '{"sku_id": 444555666, "sealer_id": 2}'

# 3. Verificar nueva configuración
curl -s "$BASE_URL/sku/assigned/1" | \
  jq '.data.skus[] | select(.is_assigned == true) | {id, sku}'
```

---

**📝 Notas:**
- Reemplaza `$BASE_URL` con tu URL real si es diferente
- Asegúrate de que el servidor esté corriendo en el puerto 8081 (configurado en config.yaml)
- Instala `jq` para formatear JSON: `sudo apt-get install jq` (Linux) o `brew install jq` (macOS)
- Los IDs de SKU son ejemplos, usa los IDs reales de tu sistema

