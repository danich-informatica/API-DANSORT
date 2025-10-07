package listeners

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"

	"API-GREENEX/internal/db"
	"API-GREENEX/internal/models"
)

type HTTPFrontend struct {
	router       *gin.Engine
	port         string
	opcuaService *OPCUAService
	postgresMgr  interface{}
}

func NewHTTPFrontend(port string) *HTTPFrontend {
	return &HTTPFrontend{
		router: gin.Default(),
		port:   port,
	}
}

// SetOPCUAService vincula el servicio OPC UA al frontend HTTP
func (h *HTTPFrontend) SetOPCUAService(service *OPCUAService) {
	h.opcuaService = service
}

// SetPostgresManager vincula el manager de PostgreSQL al frontend HTTP
func (h *HTTPFrontend) SetPostgresManager(mgr interface{}) {
	h.postgresMgr = mgr
}

func (h *HTTPFrontend) setupRoutes() {
	// Ruta GET - ya está correcta
	h.router.GET("/Mesa/Estado", func(c *gin.Context) {
		id := c.Query("id")

		var jsonResponse map[string]interface{}
		if err := json.Unmarshal([]byte(models.MESA_ESTADO_JSON_RESPONSE), &jsonResponse); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":  "Error parsing JSON response",
				"detail": err.Error(),
			})
			return
		}

		if id != "" {
			if dataList, ok := jsonResponse["dataList"].([]interface{}); ok && len(dataList) > 0 {
				if mesa, ok := dataList[0].(map[string]interface{}); ok {
					mesa["idMesaConsultada"] = id
				}
			}
		}

		c.JSON(http.StatusOK, jsonResponse)
	})

	// Ruta POST - CORREGIDA para usar la constante JSON
	h.router.POST("/Mesa", func(c *gin.Context) {
		id := c.Query("id")

		// Parsear el cuerpo JSON si existe
		var requestBody map[string]interface{}
		c.ShouldBindJSON(&requestBody)

		// Usar la constante JSON de fabricación
		var jsonResponse map[string]interface{}
		if err := json.Unmarshal([]byte(models.MESA_FABRICAION_JSON_RESPONSE), &jsonResponse); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":  "Error parsing JSON response",
				"detail": err.Error(),
			})
			return
		}

		// Agregar información adicional si es necesario
		if id != "" {
			jsonResponse["idMesaProcesada"] = id
		}
		if requestBody != nil {
			jsonResponse["requestData"] = requestBody
		}

		c.JSON(http.StatusCreated, jsonResponse)
	})

	// Ruta POST Vaciar - CORREGIDA para usar la constante JSON
	h.router.POST("/Mesa/Vaciar", func(c *gin.Context) {
		id := c.Query("id")
		modo := c.Query("modo")

		// Parsear el cuerpo JSON si existe
		var requestBody map[string]interface{}
		c.ShouldBindJSON(&requestBody)

		// Usar la constante JSON de vaciado
		var jsonResponse map[string]interface{}
		if err := json.Unmarshal([]byte(models.MESA_VACIADO_JSON_RESPONSE), &jsonResponse); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":  "Error parsing JSON response",
				"detail": err.Error(),
			})
			return
		}

		// Agregar información adicional
		if id != "" {
			jsonResponse["idMesaVaciada"] = id
		}
		if modo != "" {
			jsonResponse["modoVaciado"] = modo
		}
		if requestBody != nil {
			jsonResponse["requestData"] = requestBody
		}

		c.JSON(http.StatusOK, jsonResponse)
	})

	// Endpoint GET /skus/assignables/:sorter_id
	h.router.GET("/skus/assignables/:sorter_id", func(c *gin.Context) {
		sorterID := c.Param("sorter_id")

		// Verificar que tengamos el dbManager
		if h.postgresMgr == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"error": "Base de datos no disponible",
			})
			return
		}

		// Cast a PostgresManager
		dbManager, ok := h.postgresMgr.(*db.PostgresManager)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Error de configuración del manager",
			})
			return
		}

		// Obtener SKUs activos
		skus, err := dbManager.GetActiveSKUs(context.Background())
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Error al obtener SKUs: " + err.Error(),
			})
			return
		}

		response := make([]map[string]interface{}, 0, len(skus)+1) // +1 para REJECT

		// Agregar el SKU REJECT primero (ID 0)
		rejectEntry := map[string]interface{}{
			"id":             0,
			"sku":            "REJECT",
			"percentage":     0.0,
			"is_assigned":    false,
			"is_master_case": false,
		}
		response = append(response, rejectEntry)

		// Agregar las demás SKUs con IDs secuenciales (empezando desde 1)
		for i, sku := range skus {
			skuEntry := map[string]interface{}{
				"id":             i + 1, // Empieza en 1 porque REJECT es 0
				"sku":            sku.SKU,
				"percentage":     0.0,
				"is_assigned":    false,
				"is_master_case": false,
			}
			response = append(response, skuEntry)
		}

		c.JSON(http.StatusOK, gin.H{
			"sorter_id": sorterID,
			"skus":      response,
		})
	})
}

func (h *HTTPFrontend) Start() error {
	h.setupRoutes()
	return h.router.Run(":" + h.port)
}

func (h *HTTPFrontend) GetRouter() *gin.Engine {
	return h.router
}

// readWagoVariable lee variables escalares WAGO por MÉTODOS
func (h *HTTPFrontend) readWagoVariable(c *gin.Context) {
	variable := c.Param("variable")

	if h.opcuaService == nil || !h.opcuaService.IsConnected() {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Servicio OPC UA no disponible",
		})
		return
	}

	// Mapear nombres de variables a NodeIDs
	nodeID := h.getWagoNodeID(variable)
	if nodeID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Variable WAGO no válida. Disponibles: boleano, byte, entero, real, string, word",
		})
		return
	}

	// Leer el valor usando métodos OPC UA
	nodeData, err := h.opcuaService.ReadNode(nodeID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Error leyendo variable WAGO: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"variable":  variable,
		"nodeID":    nodeID,
		"value":     nodeData.Value,
		"timestamp": nodeData.Timestamp,
		"quality":   uint32(nodeData.Quality),
	})
}

// writeWagoVariable escribe variables escalares WAGO por MÉTODOS
func (h *HTTPFrontend) writeWagoVariable(c *gin.Context) {
	variable := c.Param("variable")

	if h.opcuaService == nil || !h.opcuaService.IsConnected() {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Servicio OPC UA no disponible",
		})
		return
	}

	// Mapear nombres de variables a NodeIDs
	nodeID := h.getWagoNodeID(variable)
	if nodeID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Variable WAGO no válida. Disponibles: boleano, byte, entero, real, string, word",
		})
		return
	}

	// Obtener el valor del body JSON
	var request struct {
		Value interface{} `json:"value"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "JSON inválido: " + err.Error(),
		})
		return
	}

	// Convertir el valor al tipo OPC UA correcto según la variable
	opcuaValue, err := h.convertToOPCUAType(variable, request.Value)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Error de tipo de datos: " + err.Error(),
		})
		return
	}

	// Escribir el valor usando métodos OPC UA con tipo correcto
	err = h.opcuaService.WriteNode(nodeID, opcuaValue)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Error escribiendo variable WAGO: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"variable": variable,
		"nodeID":   nodeID,
		"value":    request.Value,
		"status":   "written successfully",
	})
}

// getWagoNodeID mapea nombres de variables a NodeIDs WAGO
func (h *HTTPFrontend) getWagoNodeID(variable string) string {
	switch variable {
	case "boleano":
		return "ns=4;s=|var|WAGO TEST.Application.DB_OPC.BoleanoTest"
	case "byte":
		return "ns=4;s=|var|WAGO TEST.Application.DB_OPC.ByteTest"
	case "entero":
		return "ns=4;s=|var|WAGO TEST.Application.DB_OPC.EnteroTest"
	case "real":
		return "ns=4;s=|var|WAGO TEST.Application.DB_OPC.RealTest"
	case "string":
		return "ns=4;s=|var|WAGO TEST.Application.DB_OPC.StringTest"
	case "word":
		return "ns=4;s=|var|WAGO TEST.Application.DB_OPC.WordTest"
	default:
		return ""
	}
}

// convertToOPCUAType convierte valores JSON a tipos OPC UA específicos para WAGO
func (h *HTTPFrontend) convertToOPCUAType(variable string, value interface{}) (interface{}, error) {
	switch variable {
	case "boleano":
		if v, ok := value.(bool); ok {
			return v, nil
		}
		return nil, fmt.Errorf("boleano requiere tipo boolean, recibido: %T", value)

	case "byte":
		if v, ok := value.(float64); ok && v >= 0 && v <= 255 {
			return uint8(v), nil
		}
		return nil, fmt.Errorf("byte requiere entero 0-255, recibido: %v", value)

	case "entero":
		if v, ok := value.(float64); ok {
			return int16(v), nil // WAGO típicamente usa INT16 para enteros
		}
		return nil, fmt.Errorf("entero requiere número entero, recibido: %T", value)

	case "real":
		if v, ok := value.(float64); ok {
			return float32(v), nil // WAGO usa REAL (float32)
		}
		return nil, fmt.Errorf("real requiere número decimal, recibido: %T", value)

	case "string":
		if v, ok := value.(string); ok {
			return v, nil
		}
		return nil, fmt.Errorf("string requiere texto, recibido: %T", value)

	case "word":
		if v, ok := value.(float64); ok && v >= 0 && v <= 65535 {
			return uint16(v), nil
		}
		return nil, fmt.Errorf("word requiere entero 0-65535, recibido: %v", value)

	default:
		return nil, fmt.Errorf("variable desconocida: %s", variable)
	}
}
