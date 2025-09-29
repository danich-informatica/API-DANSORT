package listeners

import (
    "fmt"
    "github.com/gin-gonic/gin"
    "net/http"
)

type HTTPFrontend struct {
    router     *gin.Engine
    port       string
    opcuaService *OPCUAService
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

func (h *HTTPFrontend) setupRoutes() {
    // Ruta GET 
    h.router.GET("/ping", func(c *gin.Context) {
        c.JSON(http.StatusOK, gin.H{
            "message": "pong",
        })
    })

    h.router.GET("/Mesa/Estado", func(c *gin.Context) {
        id := c.Query("id")
        c.JSON(http.StatusOK, gin.H{
            "message": "Estado de la mesa",
            "id":      id,
        })
    })
    
    // Ruta POST 
    h.router.POST("/echo", func(c *gin.Context) {
        var json map[string]interface{}
        if err := c.ShouldBindJSON(&json); err != nil {
            c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
            return
        }
        c.JSON(http.StatusOK, json)
    })

    h.router.POST("/Mesa", func(c *gin.Context) {
        id := c.Query("id")
        var json map[string]interface{}
        if err := c.ShouldBindJSON(&json); err != nil {
            c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
            return
        }
        c.JSON(http.StatusOK, gin.H{
            "message": "Mesa envio orden",
            "id":      id,
            "data":    json,
        })
    })

    h.router.POST("/Mesa/Vaciar", func(c *gin.Context) {
        id := c.Query("id")
        modo := c.Query("modo")
        var json map[string]interface{}
        if err := c.ShouldBindJSON(&json); err != nil {
            c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
            return
        }
        c.JSON(http.StatusOK, gin.H{
            "message": "Mesa vaciada",
            "id":      id,
            "modo":    modo,
            "data":    json,
        })
    })

    // === RUTAS WAGO ESPECÍFICAS ===
    // Leer variables escalares WAGO (por MÉTODOS)
    h.router.GET("/wago/:variable", h.readWagoVariable)
    
    // Escribir variables escalares WAGO (por MÉTODOS) 
    h.router.POST("/wago/:variable", h.writeWagoVariable)
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
        "variable": variable,
        "nodeID":   nodeID,
        "value":    nodeData.Value,
        "timestamp": nodeData.Timestamp,
        "quality":  uint32(nodeData.Quality),
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