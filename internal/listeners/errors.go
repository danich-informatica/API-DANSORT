package listeners

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// ErrorResponse representa la estructura estándar de errores
type ErrorResponse struct {
	Success   bool        `json:"success"`
	Error     ErrorDetail `json:"error"`
	Timestamp string      `json:"timestamp"`
	Path      string      `json:"path"`
	Method    string      `json:"method"`
}

// ErrorDetail contiene los detalles del error
type ErrorDetail struct {
	Code    string      `json:"code"`
	Message string      `json:"message"`
	Details interface{} `json:"details,omitempty"`
	Hint    string      `json:"hint,omitempty"`
}

// SuccessResponse representa la estructura estándar de respuestas exitosas
type SuccessResponse struct {
	Success   bool        `json:"success"`
	Data      interface{} `json:"data"`
	Message   string      `json:"message,omitempty"`
	Timestamp string      `json:"timestamp"`
}

// Códigos de error estandarizados
const (
	// Client Errors (4xx)
	ErrCodeBadRequest          = "BAD_REQUEST"
	ErrCodeUnauthorized        = "UNAUTHORIZED"
	ErrCodeForbidden           = "FORBIDDEN"
	ErrCodeNotFound            = "NOT_FOUND"
	ErrCodeMethodNotAllowed    = "METHOD_NOT_ALLOWED"
	ErrCodeConflict            = "CONFLICT"
	ErrCodeUnprocessableEntity = "UNPROCESSABLE_ENTITY"
	ErrCodeTooManyRequests     = "TOO_MANY_REQUESTS"

	// Server Errors (5xx)
	ErrCodeInternalServer = "INTERNAL_SERVER_ERROR"
	ErrCodeNotImplemented = "NOT_IMPLEMENTED"
	ErrCodeServiceUnavail = "SERVICE_UNAVAILABLE"
	ErrCodeGatewayTimeout = "GATEWAY_TIMEOUT"

	// Business Logic Errors
	ErrCodeSorterNotFound      = "SORTER_NOT_FOUND"
	ErrCodeSealerNotFound      = "SEALER_NOT_FOUND"
	ErrCodeSKUNotFound         = "SKU_NOT_FOUND"
	ErrCodeSKUAlreadyAssigned  = "SKU_ALREADY_ASSIGNED"
	ErrCodeInvalidSKU          = "INVALID_SKU"
	ErrCodeSealerBlocked       = "SEALER_BLOCKED"
	ErrCodeSealerOffline       = "SEALER_OFFLINE"
	ErrCodeRejectNotAllowed    = "REJECT_SKU_NOT_ALLOWED"
	ErrCodeDatabaseError       = "DATABASE_ERROR"
	ErrCodeValidationError     = "VALIDATION_ERROR"
)

// RespondWithError envía una respuesta de error estandarizada
func RespondWithError(c *gin.Context, statusCode int, errorCode, message string, details interface{}, hint string) {
	c.JSON(statusCode, ErrorResponse{
		Success: false,
		Error: ErrorDetail{
			Code:    errorCode,
			Message: message,
			Details: details,
			Hint:    hint,
		},
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Path:      c.Request.URL.Path,
		Method:    c.Request.Method,
	})
}

// RespondWithSuccess envía una respuesta exitosa estandarizada
func RespondWithSuccess(c *gin.Context, statusCode int, data interface{}, message string) {
	c.JSON(statusCode, SuccessResponse{
		Success:   true,
		Data:      data,
		Message:   message,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	})
}

// Funciones helper para errores comunes

// BadRequest - Error 400
func BadRequest(c *gin.Context, message string, details interface{}) {
	RespondWithError(c, http.StatusBadRequest, ErrCodeBadRequest, message, details,
		"Verifica que los parámetros de la solicitud sean correctos")
}

// NotFound - Error 404
func NotFound(c *gin.Context, message string, details interface{}) {
	RespondWithError(c, http.StatusNotFound, ErrCodeNotFound, message, details,
		"Verifica que el recurso existe o que el ID sea correcto")
}

// UnprocessableEntity - Error 422
func UnprocessableEntity(c *gin.Context, message string, details interface{}) {
	RespondWithError(c, http.StatusUnprocessableEntity, ErrCodeUnprocessableEntity, message, details,
		"La solicitud está bien formada pero contiene errores semánticos")
}

// InternalServerError - Error 500
func InternalServerError(c *gin.Context, message string, details interface{}) {
	RespondWithError(c, http.StatusInternalServerError, ErrCodeInternalServer, message, details,
		"Contacta al equipo de desarrollo si el error persiste")
}

// SorterNotFound - Error de negocio: Sorter no encontrado
func SorterNotFound(c *gin.Context, sorterID string) {
	RespondWithError(c, http.StatusNotFound, ErrCodeSorterNotFound,
		"🔍 Sorter no encontrado",
		gin.H{
			"sorter_id": sorterID,
			"reason":    "El sorter especificado no está registrado en el sistema",
		},
		"Verifica que el sorter_id sea correcto. Usa GET /sorters para listar los sorters disponibles")
}

// SealerNotFound - Error de negocio: Salida no encontrada
func SealerNotFound(c *gin.Context, sealerID int) {
	RespondWithError(c, http.StatusNotFound, ErrCodeSealerNotFound,
		"🔍 Salida (sealer) no encontrada",
		gin.H{
			"sealer_id": sealerID,
			"reason":    "La salida especificada no existe en ningún sorter",
		},
		"Verifica que el sealer_id sea correcto. Usa GET /sku/assigned/:sorter_id para ver las salidas disponibles")
}

// SKUNotFound - Error de negocio: SKU no encontrada
func SKUNotFound(c *gin.Context, skuID interface{}, sorterID string) {
	RespondWithError(c, http.StatusNotFound, ErrCodeSKUNotFound,
		"🔍 SKU no encontrada",
		gin.H{
			"sku_id":    skuID,
			"sorter_id": sorterID,
			"reason":    "La SKU no existe o no está disponible para este sorter",
		},
		"Usa GET /sku/assignables/:sorter_id para ver las SKUs disponibles")
}

// RejectSKUNotAllowed - Error de negocio: No se puede asignar REJECT a salida automática
func RejectSKUNotAllowed(c *gin.Context, sealerID int, sealerName string) {
	RespondWithError(c, http.StatusUnprocessableEntity, ErrCodeRejectNotAllowed,
		"⛔ No se puede asignar SKU REJECT a salida automática",
		gin.H{
			"sku_id":      0,
			"sku_name":    "REJECT",
			"sealer_id":   sealerID,
			"sealer_name": sealerName,
			"sealer_type": "automatico",
			"reason":      "SKU REJECT solo puede asignarse a salidas de tipo 'manual'",
		},
		"Asigna la SKU REJECT a una salida manual (descarte) en lugar de una salida automática")
}

// ValidationError - Error de validación genérico
func ValidationError(c *gin.Context, field string, message string) {
	RespondWithError(c, http.StatusBadRequest, ErrCodeValidationError,
		"❌ Error de validación",
		gin.H{
			"field":  field,
			"reason": message,
		},
		"Verifica que todos los campos requeridos estén presentes y sean del tipo correcto")
}

// DatabaseError - Error de base de datos
func DatabaseError(c *gin.Context, operation string, err error) {
	RespondWithError(c, http.StatusInternalServerError, ErrCodeDatabaseError,
		"💾 Error de base de datos",
		gin.H{
			"operation": operation,
			"error":     err.Error(),
		},
		"Verifica la conectividad con la base de datos. Contacta al administrador si el error persiste")
}

// Success - Respuesta exitosa genérica
func Success(c *gin.Context, data interface{}, message string) {
	RespondWithSuccess(c, http.StatusOK, data, message)
}

// Created - Recurso creado exitosamente (201)
func Created(c *gin.Context, data interface{}, message string) {
	RespondWithSuccess(c, http.StatusCreated, data, message)
}

// NoContent - Operación exitosa sin contenido (204)
func NoContent(c *gin.Context) {
	c.Status(http.StatusNoContent)
}
