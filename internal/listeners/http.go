package listeners

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"API-GREENEX/internal/models"
	"API-GREENEX/internal/shared"
)

// SKUStreamer es una interfaz para streaming de SKUs sin crear import cycle
type SKUStreamer interface {
	StreamActiveSKUsWithLimit(ctx context.Context, skuChan models.SKUChannel, limit int)
}

// SorterInterface define la interfaz para acceder a SKUs de un Sorter
type SorterInterface = shared.SorterInterface

type HTTPFrontend struct {
	router      *gin.Engine
	port        string
	postgresMgr interface{}
	skuManager  interface{}                // Para usar SKUManager sin import cycle
	sorters     map[string]SorterInterface // Mapa de sorters por ID
}

func NewHTTPFrontend(port string) *HTTPFrontend {
	return &HTTPFrontend{
		router:  gin.Default(),
		port:    port,
		sorters: make(map[string]SorterInterface),
	}
}

// SetPostgresManager vincula el manager de PostgreSQL al frontend HTTP
func (h *HTTPFrontend) SetPostgresManager(mgr interface{}) {
	h.postgresMgr = mgr
}

// SetSKUManager vincula el SKU manager al frontend HTTP
func (h *HTTPFrontend) SetSKUManager(mgr interface{}) {
	h.skuManager = mgr
}

// RegisterSorter registra un sorter para acceso desde HTTP
func (h *HTTPFrontend) RegisterSorter(sorter SorterInterface) {
	sorterID := fmt.Sprintf("%d", sorter.GetID())
	h.sorters[sorterID] = sorter
}

func (h *HTTPFrontend) setupRoutes() {
	// Endpoint GET /sku/assigned/:sorter_id
	h.router.GET("/sku/assigned/:sorter_id", func(c *gin.Context) {
		sorterID := c.Param("sorter_id")
		sorter, exists := h.sorters[sorterID]
		if !exists {
			c.JSON(http.StatusNotFound, gin.H{
				"error": fmt.Sprintf("Sorter #%s no encontrado o no registrado", sorterID),
			})
			return
		}
		var result []map[string]interface{}
		for _, salida := range sorter.GetSalidas() {
			assignments := []map[string]interface{}{}
			for _, sku := range salida.SKUs_Actuales {
				assignments = append(assignments, map[string]interface{}{
					"sku_id":         sku.GetNumericID(),
					"sku_name":       sku.SKU,
					"is_master_case": false,
				})
			}
			result = append(result, map[string]interface{}{
				"sealer_db_id":       salida.ID,
				"sealer_physical_id": salida.SealerPhysicalID,
				"sorter_id":          sorter.GetID(),
				"sorter_name":        "",
				"mode":               salida.Tipo,
				"assignments":        assignments,
				"is_enabled":         salida.IsEnabled,
			})
		}
		c.JSON(http.StatusOK, result)
	})
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
	// Accede directamente a las SKUs cacheadas en el sorter (no usa canales)
	h.router.GET("/skus/assignables/:sorter_id", func(c *gin.Context) {
		sorterID := c.Param("sorter_id")

		// Validar sorter_id
		_, err := strconv.Atoi(sorterID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "sorter_id debe ser un número válido",
			})
			return
		}

		// Obtener sorter directamente desde el mapa
		sorter, exists := h.sorters[sorterID]
		if !exists {
			c.JSON(http.StatusNotFound, gin.H{
				"error": fmt.Sprintf("Sorter #%s no encontrado o no registrado", sorterID),
			})
			return
		}

		// Obtener SKUs directamente del sorter (inmediato, sin bloqueo)
		skus := sorter.GetCurrentSKUs()
		c.JSON(http.StatusOK, skus)
	})

	// Endpoint POST /assignment
	// Asigna una SKU a una salida específica (sealer)
	// Body: { "sku_id": uint32, "sealer_id": int }
	h.router.POST("/assignment", func(c *gin.Context) {
		var request struct {
			SKUID    *uint32 `json:"sku_id" binding:"required"` // Pointer para aceptar 0
			SealerID int     `json:"sealer_id" binding:"required"`
		}

		if err := c.ShouldBindJSON(&request); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Formato de body inválido. Se requiere: {sku_id: number, sealer_id: number}",
			})
			return
		}

		if request.SKUID == nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "sku_id es requerido",
			})
			return
		}

		skuID := *request.SKUID

		// Buscar en qué sorter está la salida (sealer_id es único globalmente)
		var targetSorter SorterInterface
		var assignError error
		var calibre, variedad, embalaje string
		for _, sorter := range h.sorters {
			cal, var_, emb, err := sorter.AssignSKUToSalida(skuID, request.SealerID)
			if err == nil {
				targetSorter = sorter
				calibre = cal
				variedad = var_
				embalaje = emb
				break
			}
			// Guardar el error más relevante
			if assignError == nil || err.Error() != fmt.Sprintf("salida con ID %d no encontrada en sorter #%d", request.SealerID, sorter.GetID()) {
				assignError = err
			}
		}

		if targetSorter == nil {
			// Si hubo error específico (ej: REJECT en automática), mostrarlo
			if assignError != nil {
				c.JSON(http.StatusUnprocessableEntity, gin.H{
					"error":     assignError.Error(),
					"sku_id":    skuID,
					"sealer_id": request.SealerID,
				})
				return
			}

			c.JSON(http.StatusNotFound, gin.H{
				"error":     fmt.Sprintf("No se encontró salida con ID %d en ningún sorter", request.SealerID),
				"sealer_id": request.SealerID,
			})
			return
		}

		// Insertar asignación en la base de datos
		if h.postgresMgr != nil {
			if dbMgr, ok := h.postgresMgr.(interface {
				InsertSalidaSKU(ctx context.Context, salidaID int, calibre, variedad, embalaje string) error
			}); ok {
				ctx := c.Request.Context()
				if err := dbMgr.InsertSalidaSKU(ctx, request.SealerID, calibre, variedad, embalaje); err != nil {
					// Log el error pero no fallar el request (asignación en memoria ya está hecha)
					fmt.Printf("⚠️  Error al insertar asignación en DB: %v\n", err)
				}
			}
		}

		c.JSON(http.StatusOK, gin.H{
			"message":   "SKU asignada exitosamente",
			"sku_id":    skuID,
			"sealer_id": request.SealerID,
			"sorter_id": targetSorter.GetID(),
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
