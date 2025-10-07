# Fix para escritura OPC UA en WAGO/Codesys

## 🐛 Problema

El código Go no podía escribir en nodos WAGO debido a un error `StatusBadTypeMismatch`.

## 🔍 Causa Raíz

Faltaba el campo **`EncodingMask`** en el `DataValue` al escribir. WAGO PLCs con Codesys **requieren** este campo para validar correctamente el tipo de dato.

## ✅ Solución Aplicada

### Antes (❌ INCORRECTO):

```go
Value: &ua.DataValue{
    Value: variant,
},
```

### Después (✅ CORRECTO):

```go
Value: &ua.DataValue{
    EncodingMask: ua.DataValueValue, // ← CRÍTICO para WAGO/Codesys
    Value:        variant,
},
```

## 📝 Detalles Técnicos

### ¿Qué es EncodingMask?

`EncodingMask` es un campo bit-mask que indica qué campos del `DataValue` están presentes:

```go
const (
    DataValueValue            = 0x01  // Value presente
    DataValueStatusCode       = 0x02  // StatusCode presente
    DataValueSourceTimestamp  = 0x04  // SourceTimestamp presente
    DataValueServerTimestamp  = 0x08  // ServerTimestamp presente
    DataValueSourcePicoseconds = 0x10  // SourcePicoseconds presente
    DataValueServerPicoseconds = 0x20  // ServerPicoseconds presente
)
```

### ¿Por qué WAGO lo requiere?

Los PLCs WAGO con runtime Codesys son **estrictos** con el protocolo OPC UA:

- Validan que el `EncodingMask` corresponda con los campos enviados
- Sin `EncodingMask`, el PLC no sabe interpretar el `DataValue` correctamente
- Retorna `StatusBadTypeMismatch` al no poder validar el tipo

## 🧪 Ejemplos Funcionando

### Escribir Boolean

```go
variant, _ := ua.NewVariant(true)
req := &ua.WriteRequest{
    NodesToWrite: []*ua.WriteValue{{
        NodeID:      id,
        AttributeID: ua.AttributeIDValue,
        Value: &ua.DataValue{
            EncodingMask: ua.DataValueValue,
            Value:        variant,
        },
    }},
}
```

### Escribir Int16

```go
variant, _ := ua.NewVariant(int16(1500))
req := &ua.WriteRequest{
    NodesToWrite: []*ua.WriteValue{{
        NodeID:      id,
        AttributeID: ua.AttributeIDValue,
        Value: &ua.DataValue{
            EncodingMask: ua.DataValueValue,
            Value:        variant,
        },
    }},
}
```

### Escribir String

```go
variant, _ := ua.NewVariant("Greenex-Test")
req := &ua.WriteRequest{
    NodesToWrite: []*ua.WriteValue{{
        NodeID:      id,
        AttributeID: ua.AttributeIDValue,
        Value: &ua.DataValue{
            EncodingMask: ua.DataValueValue,
            Value:        variant,
        },
    }},
}
```

### Escribir Array ([]int16)

```go
arr := []int16{10, 20, 30, 40, 50}
variant, _ := ua.NewVariant(arr)
req := &ua.WriteRequest{
    NodesToWrite: []*ua.WriteValue{{
        NodeID:      id,
        AttributeID: ua.AttributeIDValue,
        Value: &ua.DataValue{
            EncodingMask: ua.DataValueValue,
            Value:        variant,
        },
    }},
}
```

## 🎯 Mapeo de Tipos WAGO ↔ Go

| WAGO/Codesys  | Go Type    | Variant Constructor                  |
| ------------- | ---------- | ------------------------------------ |
| BOOL          | `bool`     | `ua.NewVariant(true)`                |
| BYTE          | `uint8`    | `ua.NewVariant(uint8(123))`          |
| INT           | `int16`    | `ua.NewVariant(int16(1500))`         |
| REAL          | `float32`  | `ua.NewVariant(float32(25.5))`       |
| STRING        | `string`   | `ua.NewVariant("text")`              |
| WORD          | `uint16`   | `ua.NewVariant(uint16(30000))`       |
| ARRAY OF BOOL | `[]bool`   | `ua.NewVariant([]bool{true, false})` |
| ARRAY OF INT  | `[]int16`  | `ua.NewVariant([]int16{1, 2, 3})`    |
| ARRAY OF WORD | `[]uint16` | `ua.NewVariant([]uint16{100, 200})`  |

## 📊 Logs Mejorados

Con el fix aplicado, los logs ahora muestran:

```
┌─────────────────────────────────────────────────────────
│ 📝 ESCRITURA WAGO
│ Nodo: ns=4;s=|var|WAGO TEST.Application.DB_OPC.StringTest
│ Tipo esperado (DataType): ns=0;i=12
│ Valor a escribir: Greenex-468
│ Tipo Go: string
└─────────────────────────────────────────────────────────
✅ ÉXITO | Nodo: ns=4;s=|var|WAGO TEST.Application.DB_OPC.StringTest | Valor: Greenex-468 (string)
```

## 🚀 Código de Referencia Completo

Ver archivo de ejemplo funcional: [internal/listeners/opcua.go](../internal/listeners/opcua.go) líneas 493-507

## 📚 Referencias

- OPC UA Specification Part 6 (Mappings): Section 5.2.2.13 DataValue
- gopcua library: https://github.com/gopcua/opcua
- WAGO Codesys OPC UA Server Documentation

## ✨ Resultado

- ✅ Escrituras exitosas en todos los tipos WAGO
- ✅ Sin errores `StatusBadTypeMismatch`
- ✅ Logs detallados para debugging
- ✅ Compatible con WAGO PLCs y Codesys runtime

---

**Fecha de Fix**: 6 de Octubre 2025  
**Archivo Modificado**: `internal/listeners/opcua.go`  
**Línea**: 501  
**Cambio**: Agregado `EncodingMask: ua.DataValueValue`
