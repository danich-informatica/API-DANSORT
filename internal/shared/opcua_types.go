package shared

import (
	"strconv"
	"strings"
    "time"
    "github.com/gopcua/opcua/ua"
)

// SubscriptionData representa datos de suscripción
type SubscriptionData struct {
	SubscriptionName string
	NodeID           string
	ReceivedAt       time.Time
}

// WriteRequest representa una solicitud de escritura
type WriteRequest struct {
	NodeID string
	Value  interface{}
}

type WagoDataTypes struct {
	VectorBool []bool    // Array de Boolean
	VectorInt  []int16   // Array de Int16
	VectorWord []uint16  // Array de UInt16
	
	BoleanoTest bool      // Boolean escalar
	ByteTest    uint8      // Byte escalar
	EnteroTest  int16     // Int16 escalar
	RealTest    float32   // Real escalar (float32)
	StringTest  string    // String escalar
	WordTest    uint16    // UInt16 escalar
}

// ...existing code...

// CastWagoValue castea el valor al tipo Go correcto según el nombre del nodo WAGO
func CastWagoValue(nodeID string, value interface{}) interface{} {
	switch {
	case contains(nodeID, "BoleanoTest"):
		if v, ok := value.(bool); ok { return v }
		if v, ok := value.(int); ok { return v != 0 }
	case contains(nodeID, "ByteTest"):
		switch v := value.(type) {
		case uint8: return v
		case int: return uint8(v)
		case float64: return uint8(v)
		}
	case contains(nodeID, "EnteroTest"):
		switch v := value.(type) {
		case int16: return v
		case int: return int16(v)
		case float64: return int16(v)
		}
	case contains(nodeID, "RealTest"):
		switch v := value.(type) {
		case float32: return v
		case float64: return float32(v)
		case int: return float32(v)
		}
	case contains(nodeID, "StringTest"):
		if v, ok := value.(string); ok { return v }
	case contains(nodeID, "WordTest"):
		switch v := value.(type) {
		case uint16: return v
		case int: return uint16(v)
		case float64: return uint16(v)
		}
	}
	return value
}

// CastByDataType convierte un valor genérico (value) al tipo Go EXACTO requerido 
// por el PLC, definido por el ua.NodeID del DataType.
func CastByDataType(dataType interface{}, value interface{}) interface{} {
    
	v, ok := dataType.(*ua.NodeID)
	if !ok {
		// El 'dataType' no es un *ua.NodeID, devuelve el valor sin cambios.
		return value
	}

	// Extraer Namespace y Built-in ID del tipo de dato.
	// El Built-in ID (i=X) es el número que define el tipo (ej. i=1 para Boolean)
	// No todos los Built-in IDs están en el namespace 0, pero la mayoría de los básicos sí.
	ns := v.Namespace()
	idUint32 := v.IntID()
    
	// Nota: Los IDs de 0 a 25 son los Built-in Types (BOOL, INT16, etc.)
    // Los namespaces 0, 3 y 4 son comunes.
	if ns >= 0 {
		switch idUint32 {
        
        // --- BOOLEAN (ID 1) ---
		case 1, 2: 
			if b, ok := value.(bool); ok { return b }
            // Intentar coerción de int, float64 o string
			if i, ok := value.(int); ok { return i != 0 }
			if f, ok := value.(float64); ok { return f != 0.0 }
            if s, ok := value.(string); ok { 
                s = strings.ToLower(s)
                return s == "true" || s == "1" 
            }
        
        // --- INT16 (ID 4) ---
		case 4: 
			if i, ok := value.(int16); ok { return i }
			if i, ok := value.(int); ok { 
                // Convertir int genérico (int32/int64) a int16.
                return int16(i) 
            }
            if f, ok := value.(float64); ok { 
                // Convertir float64 a int16 (truncado)
                return int16(f) 
            }

        // --- UINT16 (ID 5) ---
		case 5: 
			if u, ok := value.(uint16); ok { return u }
			if i, ok := value.(int); ok { return uint16(i) }
            if f, ok := value.(float64); ok { return uint16(f) }

        // --- FLOAT32 (ID 10/11) ---
        // ID 10 es Float (usado en algunos sistemas), ID 11 es Double
		case 10: 
			if f, ok := value.(float32); ok { return f }
			if f, ok := value.(float64); ok { return float32(f) }
			if i, ok := value.(int); ok { return float32(i) }
            
        // --- STRING (ID 12) ---
		case 12, 3013: // 3013 es un ID común en WAGO para Strings
			if s, ok := value.(string); ok { return s }
            // Intentar convertir números a string (útil si el input viene de un JSON)
            if i, ok := value.(int); ok { return strconv.Itoa(i) }

        // --- BYTE/UINT8 (ID 3) ---
        case 3: 
            if u, ok := value.(uint8); ok { return u }
			if i, ok := value.(int); ok { return uint8(i) }
            if f, ok := value.(float64); ok { return uint8(f) }
            
        // --- INT32 (ID 6) ---
        case 6: 
            if i, ok := value.(int32); ok { return i }
			if i, ok := value.(int); ok { return int32(i) }
            if f, ok := value.(float64); ok { return int32(f) }
            
        // --- UINT32 (ID 7) ---
        case 7: 
            if u, ok := value.(uint32); ok { return u }
			if i, ok := value.(int); ok { return uint32(i) }
            if f, ok := value.(float64); ok { return uint32(f) }
            
        // --- DOUBLE (ID 11) ---
        case 11: 
            if f, ok := value.(float64); ok { return f }
			if i, ok := value.(int); ok { return float64(i) }

		}
	}
    
    // Si no se encuentra un tipo coincidente o una conversión válida,
    // se devuelve el valor original.
	return value
}

// contains verifica si substr está en s
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || (len(s) > len(substr) && (index(s, substr) >= 0)))
}

// index retorna el índice de substr en s, -1 si no está
func index(s, substr string) int {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}