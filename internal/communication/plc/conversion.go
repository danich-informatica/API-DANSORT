package plc

import (
	"fmt"
	"strconv"
)

// ConvertValueForWrite convierte un valor de Go al tipo apropiado para escritura OPC UA
// Basado en el código de dantrack que funciona correctamente
func ConvertValueForWrite(value interface{}, dataType string) (interface{}, error) {
	switch dataType {
	case "bool":
		switch v := value.(type) {
		case bool:
			return v, nil
		case int:
			return v != 0, nil
		case string:
			return v == "true" || v == "1", nil
		default:
			return nil, fmt.Errorf("no se puede convertir %T a bool para escritura", value)
		}

	case "byte", "uint8":
		switch v := value.(type) {
		case uint8:
			return v, nil
		case int:
			return uint8(v), nil
		case int64:
			return uint8(v), nil
		default:
			return nil, fmt.Errorf("no se puede convertir %T a byte para escritura", value)
		}

	case "int16":
		switch v := value.(type) {
		case int16:
			return v, nil
		case int:
			return int16(v), nil
		case int32:
			return int16(v), nil
		case int64:
			return int16(v), nil
		case string:
			// Convertir string a int16
			parsed, err := strconv.ParseInt(v, 10, 16)
			if err != nil {
				return nil, fmt.Errorf("no se puede convertir string '%s' a int16: %w", v, err)
			}
			return int16(parsed), nil
		default:
			return nil, fmt.Errorf("no se puede convertir %T a int16 para escritura", value)
		}

	case "uint16":
		switch v := value.(type) {
		case uint16:
			return v, nil
		case int:
			return uint16(v), nil
		case uint32:
			return uint16(v), nil
		case uint64:
			return uint64(v), nil
		case string:
			parsed, err := strconv.ParseUint(v, 10, 16)
			if err != nil {
				return nil, fmt.Errorf("no se puede convertir string '%s' a uint16: %w", v, err)
			}
			return uint16(parsed), nil
		default:
			return nil, fmt.Errorf("no se puede convertir %T a uint16 para escritura", value)
		}

	case "int32":
		switch v := value.(type) {
		case int32:
			return v, nil
		case int:
			return int32(v), nil
		case int64:
			return int32(v), nil
		case string:
			parsed, err := strconv.ParseInt(v, 10, 32)
			if err != nil {
				return nil, fmt.Errorf("no se puede convertir string '%s' a int32: %w", v, err)
			}
			return int32(parsed), nil
		default:
			return nil, fmt.Errorf("no se puede convertir %T a int32 para escritura", value)
		}

	case "uint32":
		switch v := value.(type) {
		case uint32:
			return v, nil
		case int:
			return uint32(v), nil
		case uint64:
			return uint32(v), nil
		case string:
			parsed, err := strconv.ParseUint(v, 10, 32)
			if err != nil {
				return nil, fmt.Errorf("no se puede convertir string '%s' a uint32: %w", v, err)
			}
			return uint32(parsed), nil
		default:
			return nil, fmt.Errorf("no se puede convertir %T a uint32 para escritura", value)
		}

	case "float32":
		switch v := value.(type) {
		case float32:
			return v, nil
		case float64:
			return float32(v), nil
		case int:
			return float32(v), nil
		case string:
			parsed, err := strconv.ParseFloat(v, 32)
			if err != nil {
				return nil, fmt.Errorf("no se puede convertir string '%s' a float32: %w", v, err)
			}
			return float32(parsed), nil
		default:
			return nil, fmt.Errorf("no se puede convertir %T a float32 para escritura", value)
		}

	case "float64":
		switch v := value.(type) {
		case float64:
			return v, nil
		case float32:
			return float64(v), nil
		case int:
			return float64(v), nil
		default:
			return nil, fmt.Errorf("no se puede convertir %T a float64 para escritura", value)
		}

	case "string":
		return fmt.Sprintf("%v", value), nil

	case "variant":
		// Para valores que ya son del tipo correcto
		return value, nil

	default:
		// Para tipos no reconocidos, intentar devolver el valor tal como está
		return value, nil
	}
}

// ConvertValue convierte un valor leído desde OPC UA a Go según el tipo de datos configurado
func ConvertValue(value interface{}, dataType string) (interface{}, error) {
	switch dataType {
	case "bool":
		switch v := value.(type) {
		case bool:
			return v, nil
		case uint8:
			return v != 0, nil
		case int32:
			return v != 0, nil
		default:
			return nil, fmt.Errorf("no se puede convertir %T a bool", value)
		}

	case "byte", "uint8":
		switch v := value.(type) {
		case uint8:
			return v, nil
		case int32:
			return uint8(v), nil
		default:
			return nil, fmt.Errorf("no se puede convertir %T a byte", value)
		}

	case "int16":
		switch v := value.(type) {
		case int16:
			return v, nil
		case int32:
			return int16(v), nil
		default:
			return nil, fmt.Errorf("no se puede convertir %T a int16", value)
		}

	case "uint16":
		switch v := value.(type) {
		case uint16:
			return v, nil
		case int32:
			return uint16(v), nil
		default:
			return nil, fmt.Errorf("no se puede convertir %T a uint16", value)
		}

	case "int32":
		switch v := value.(type) {
		case int32:
			return v, nil
		case int16:
			return int32(v), nil
		default:
			return nil, fmt.Errorf("no se puede convertir %T a int32", value)
		}

	case "uint32":
		switch v := value.(type) {
		case uint32:
			return v, nil
		case int32:
			return uint32(v), nil
		default:
			return nil, fmt.Errorf("no se puede convertir %T a uint32", value)
		}

	case "float32":
		switch v := value.(type) {
		case float32:
			return v, nil
		case float64:
			return float32(v), nil
		default:
			return nil, fmt.Errorf("no se puede convertir %T a float32", value)
		}

	case "float64":
		switch v := value.(type) {
		case float64:
			return v, nil
		case float32:
			return float64(v), nil
		default:
			return nil, fmt.Errorf("no se puede convertir %T a float64", value)
		}

	case "string":
		return fmt.Sprintf("%v", value), nil

	case "array_bool":
		switch v := value.(type) {
		case []bool:
			return v, nil
		case []uint8:
			bools := make([]bool, len(v))
			for i, b := range v {
				bools[i] = b != 0
			}
			return bools, nil
		default:
			return nil, fmt.Errorf("no se puede convertir %T a []bool", value)
		}

	case "array_int16":
		switch v := value.(type) {
		case []int16:
			return v, nil
		case []int32:
			int16s := make([]int16, len(v))
			for i, val := range v {
				int16s[i] = int16(val)
			}
			return int16s, nil
		default:
			return nil, fmt.Errorf("no se puede convertir %T a []int16", value)
		}

	case "array_uint16":
		switch v := value.(type) {
		case []uint16:
			return v, nil
		case []int32:
			uint16s := make([]uint16, len(v))
			for i, val := range v {
				uint16s[i] = uint16(val)
			}
			return uint16s, nil
		default:
			return nil, fmt.Errorf("no se puede convertir %T a []uint16", value)
		}

	default:
		// Para tipos no reconocidos, devolver el valor tal como está
		return value, nil
	}
}
