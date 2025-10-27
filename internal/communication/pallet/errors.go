package pallet

import (
	"errors"
	"fmt"
	"net/http"
)

// Error personalizado para categorizar errores de la API
type ErrorCategory int

const (
	ErrorCategoryConnection ErrorCategory = iota
	ErrorCategoryValidation
	ErrorCategoryNotFound
	ErrorCategoryConflict
	ErrorCategoryInternal
	ErrorCategoryUnknown
)

// CategorizeError categoriza un error devuelto por la API
func CategorizeError(err error) ErrorCategory {
	if err == nil {
		return ErrorCategoryUnknown
	}

	var apiErr *APIError
	if errors.As(err, &apiErr) {
		switch apiErr.StatusCode {
		case http.StatusBadRequest:
			return ErrorCategoryValidation
		case http.StatusNotFound:
			return ErrorCategoryNotFound
		case 202, 209:
			return ErrorCategoryConflict
		case http.StatusInternalServerError:
			return ErrorCategoryInternal
		default:
			return ErrorCategoryUnknown
		}
	}

	// Si no es un APIError, probablemente es un error de conexión
	return ErrorCategoryConnection
}

// IsRetryable determina si un error es reintentable
func IsRetryable(err error) bool {
	if err == nil {
		return false
	}

	category := CategorizeError(err)
	switch category {
	case ErrorCategoryConnection, ErrorCategoryInternal:
		return true
	default:
		return false
	}
}

// WrapError envuelve un error con contexto adicional
func WrapError(err error, context string) error {
	if err == nil {
		return nil
	}

	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return fmt.Errorf("%s: %w", context, err)
	}

	return fmt.Errorf("%s: %w", context, err)
}

// FormatError formatea un error para logging
func FormatError(err error) string {
	if err == nil {
		return ""
	}

	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return fmt.Sprintf("[%d] %s (endpoint: %s, timestamp: %s)",
			apiErr.StatusCode,
			apiErr.Message,
			apiErr.Endpoint,
			apiErr.Timestamp.Format("2006-01-02 15:04:05"),
		)
	}

	return err.Error()
}
