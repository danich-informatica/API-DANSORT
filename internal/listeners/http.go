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

type HTTPFrontend struct {
	router      *gin.Engine
	addr        string // Dirección completa host:port
	postgresMgr interface{}
	skuManager  interface{}                       // Para usar SKUManager sin import cycle
	sorters     map[string]shared.SorterInterface // Mapa de sorters por ID
	wsHub       *WebSocketHub                     // Hub de WebSocket
}

func NewHTTPFrontend(addr string) *HTTPFrontend {
	router := gin.Default()

	// Configurar CORS para permitir todas las peticiones
	router.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE, PATCH")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	})

	// Manejador personalizado para rutas 404
	router.NoRoute(func(c *gin.Context) {
		RespondWithError(c, http.StatusNotFound, ErrCodeNotFound,
			"🤔 La ruta que buscas no existe en este servidor",
			gin.H{
				"available_endpoints": gin.H{
					"frontend": []string{
						"GET /visualizer",
						"GET /simulator",
					},
					"sku_management": []string{
						"GET /sku/assigned/:sorter_id",
						"GET /sku/assignables/:sorter_id",
					},
					"assignments": []string{
						"POST /assignment",
						"DELETE /assignment/:sealer_id/:sku_id",
						"DELETE /assignment/:sealer_id",
					},
					"websocket": []string{
						"GET /ws/:room",
						"GET /ws/stats",
					},
					"legacy_api": []string{
						"GET /Mesa/Estado",
						"POST /Mesa",
						"POST /Mesa/Vaciar",
					},
				},
			},
			"Revisa la documentación en MANUAL_API.md o contacta al equipo de desarrollo")
	})

	// Crear e iniciar WebSocket Hub
	wsHub := NewWebSocketHub()
	go wsHub.Run()

	return &HTTPFrontend{
		router:  router,
		addr:    addr,
		sorters: make(map[string]shared.SorterInterface),
		wsHub:   wsHub,
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
func (h *HTTPFrontend) RegisterSorter(sorter shared.SorterInterface) {
	sorterID := fmt.Sprintf("%d", sorter.GetID())
	h.sorters[sorterID] = sorter
}

// GetWebSocketHub retorna el hub de WebSocket
func (h *HTTPFrontend) GetWebSocketHub() *WebSocketHub {
	return h.wsHub
}

func (h *HTTPFrontend) setupRoutes() {
	// Servir archivos estáticos (visualizador) sin caché
	h.router.GET("/visualizer", func(c *gin.Context) {
		c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
		c.Header("Pragma", "no-cache")
		c.Header("Expires", "0")
		c.File("./web/visualizer.html")
	})
	h.router.GET("/simulator", func(c *gin.Context) {
		c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
		c.Header("Pragma", "no-cache")
		c.Header("Expires", "0")
		c.File("./web/simulator.html")
	})

	// Endpoint GET /sku/assigned/:sorter_id
	h.router.GET("/sku/assigned/:sorter_id", func(c *gin.Context) {
		sorterID := c.Param("sorter_id")
		sorter, exists := h.sorters[sorterID]
		if !exists {
			SorterNotFound(c, sorterID)
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

	// Endpoint GET /sku/assignables/:sorter_id (alias sin 's' para frontend)
	h.router.GET("/sku/assignables/:sorter_id", func(c *gin.Context) {
		sorterID := c.Param("sorter_id")

		// Validar sorter_id
		_, err := strconv.Atoi(sorterID)
		if err != nil {
			ValidationError(c, "sorter_id", "debe ser un número válido")
			return
		}

		// Obtener sorter directamente desde el mapa
		sorter, exists := h.sorters[sorterID]
		if !exists {
			SorterNotFound(c, sorterID)
			return
		}

		// Obtener SKUs directamente del sorter (inmediato, sin bloqueo)
		skus := sorter.GetCurrentSKUs()
		Success(c, skus, "✅ SKUs obtenidas exitosamente")
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
			BadRequest(c, "Formato de body inválido",
				gin.H{
					"required_format": gin.H{
						"sku_id":    "number (uint32)",
						"sealer_id": "number (int)",
					},
					"error": err.Error(),
				})
			return
		}

		if request.SKUID == nil {
			ValidationError(c, "sku_id", "es requerido")
			return
		}

		skuID := *request.SKUID

		// Buscar en qué sorter está la salida (sealer_id es único globalmente)
		var targetSorter shared.SorterInterface
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
				UnprocessableEntity(c, assignError.Error(),
					gin.H{
						"sku_id":    skuID,
						"sealer_id": request.SealerID,
					})
				return
			}

			SealerNotFound(c, request.SealerID)
			return
		}

		// Insertar asignación en la base de datos (solo si NO es REJECT)
		// REJECT (ID=0) solo existe en memoria, no en BD
		if skuID != 0 && h.postgresMgr != nil {
			if dbMgr, ok := h.postgresMgr.(interface {
				InsertSalidaSKU(ctx context.Context, salidaID int, calibre, variedad, embalaje string) error
			}); ok {
				ctx := c.Request.Context()
				if err := dbMgr.InsertSalidaSKU(ctx, request.SealerID, calibre, variedad, embalaje); err != nil {
					// Log el error pero no fallar el request (asignación en memoria ya está hecha)
					fmt.Printf("⚠️  Error al insertar asignación en DB: %v\n", err)
				}
			}
		} else if skuID == 0 {
			fmt.Printf("ℹ️  SKU REJECT asignada solo en memoria (no se persiste en BD)\n")
		}

		// 🔔 Respuesta exitosa
		Created(c, gin.H{
			"sku_id":    skuID,
			"sealer_id": request.SealerID,
			"sorter_id": targetSorter.GetID(),
			"sku_details": gin.H{
				"calibre":  calibre,
				"variedad": variedad,
				"embalaje": embalaje,
			},
		}, fmt.Sprintf("✅ SKU asignada exitosamente a salida #%d", request.SealerID))
	})

	// Endpoint DELETE /assignment/:sealer_id/:sku_id
	// Elimina una SKU específica de una salida (sealer)
	// Params: sealer_id (int), sku_id (uint32)
	h.router.DELETE("/assignment/:sealer_id/:sku_id", func(c *gin.Context) {
		// Parsear parámetros
		sealerIDStr := c.Param("sealer_id")
		skuIDStr := c.Param("sku_id")

		sealerID, err := strconv.Atoi(sealerIDStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "sealer_id debe ser un número entero válido",
			})
			return
		}

		skuIDInt, err := strconv.ParseUint(skuIDStr, 10, 32)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "sku_id debe ser un número entero válido",
			})
			return
		}
		skuID := uint32(skuIDInt)

		// Buscar en qué sorter está la salida
		var targetSorter shared.SorterInterface
		var calibre, variedad, embalaje string
		var removeError error

		for _, sorter := range h.sorters {
			cal, var_, emb, err := sorter.RemoveSKUFromSalida(skuID, sealerID)
			if err == nil {
				targetSorter = sorter
				calibre = cal
				variedad = var_
				embalaje = emb
				break
			}
			// Guardar el error más relevante
			if removeError == nil || err.Error() != fmt.Sprintf("salida con ID %d no encontrada en sorter #%d", sealerID, sorter.GetID()) {
				removeError = err
			}
		}

		if targetSorter == nil {
			if removeError != nil {
				c.JSON(http.StatusNotFound, gin.H{
					"error":     removeError.Error(),
					"sku_id":    skuID,
					"sealer_id": sealerID,
				})
				return
			}

			c.JSON(http.StatusNotFound, gin.H{
				"error":     fmt.Sprintf("No se encontró salida con ID %d en ningún sorter", sealerID),
				"sealer_id": sealerID,
			})
			return
		}

		// Eliminar de la base de datos (solo si NO es REJECT)
		// REJECT (ID=0) solo existe en memoria, no en BD
		if skuID != 0 && h.postgresMgr != nil {
			if dbMgr, ok := h.postgresMgr.(interface {
				DeleteSalidaSKU(ctx context.Context, salidaID int, calibre, variedad, embalaje string) error
			}); ok {
				ctx := c.Request.Context()
				if err := dbMgr.DeleteSalidaSKU(ctx, sealerID, calibre, variedad, embalaje); err != nil {
					// Si la SKU no existe en BD, no es un error crítico
					// (puede haber sido asignada solo en memoria)
					fmt.Printf("⚠️  Error al eliminar asignación de DB: %v\n", err)
					// NO retornar error - la eliminación en memoria ya está hecha
				}
			}
		} else if skuID == 0 {
			fmt.Printf("ℹ️  SKU REJECT eliminada solo de memoria (no existe en BD)\n")
		}

		// 🔔 Notificar a clientes WebSocket
		c.JSON(http.StatusOK, gin.H{
			"message":   "SKU eliminada exitosamente",
			"sku_id":    skuID,
			"sealer_id": sealerID,
			"sorter_id": targetSorter.GetID(),
			"removed_sku": gin.H{
				"calibre":  calibre,
				"variedad": variedad,
				"embalaje": embalaje,
			},
		})
	})

	// Endpoint DELETE /assignment/:sealer_id
	// Elimina TODAS las SKUs de una salida (sealer)
	// Params: sealer_id (int)
	h.router.DELETE("/assignment/:sealer_id", func(c *gin.Context) {
		// Parsear parámetro
		sealerIDStr := c.Param("sealer_id")
		sealerID, err := strconv.Atoi(sealerIDStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "sealer_id debe ser un número entero válido",
			})
			return
		}

		// Buscar en qué sorter está la salida
		var targetSorter shared.SorterInterface
		var removedSKUs []models.SKU
		var removeError error

		for _, sorter := range h.sorters {
			skus, err := sorter.RemoveAllSKUsFromSalida(sealerID)
			if err == nil {
				targetSorter = sorter
				removedSKUs = skus
				break
			}
			// Guardar el error más relevante
			if removeError == nil || err.Error() != fmt.Sprintf("salida con ID %d no encontrada en sorter #%d", sealerID, sorter.GetID()) {
				removeError = err
			}
		}

		if targetSorter == nil {
			if removeError != nil {
				c.JSON(http.StatusNotFound, gin.H{
					"error":     removeError.Error(),
					"sealer_id": sealerID,
				})
				return
			}

			c.JSON(http.StatusNotFound, gin.H{
				"error":     fmt.Sprintf("No se encontró salida con ID %d en ningún sorter", sealerID),
				"sealer_id": sealerID,
			})
			return
		}

		// 🧹 Borrar TODAS las SKUs de la DB (incluyendo REJECT)
		var rowsDeleted int64
		if h.postgresMgr != nil {
			if dbMgr, ok := h.postgresMgr.(interface {
				DeleteAllSalidaSKUs(ctx context.Context, salidaID int) (int64, error)
				InsertSalidaSKU(ctx context.Context, salidaID int, calibre, variedad, embalaje string) error
			}); ok {
				ctx := c.Request.Context()

				// 1. Borrar TODO
				rowsDeleted, err = dbMgr.DeleteAllSalidaSKUs(ctx, sealerID)
				if err != nil {
					fmt.Printf("⚠️  Error al eliminar SKUs de DB: %v\n", err)
					c.JSON(http.StatusInternalServerError, gin.H{
						"warning": "SKUs eliminadas de memoria pero falló eliminación en base de datos",
						"error":   err.Error(),
					})
					return
				}

				// 2. ♻️ Re-insertar REJECT automáticamente
				err = dbMgr.InsertSalidaSKU(ctx, sealerID, "REJECT", "REJECT", "REJECT")
				if err != nil {
					fmt.Printf("⚠️  Error al re-insertar REJECT en DB: %v\n", err)
				} else {
					fmt.Printf("♻️  [DB] SKU REJECT re-insertada en salida %d\n", sealerID)
				}
			}
		}

		response := gin.H{
			"message":             fmt.Sprintf("Eliminadas %d SKUs de salida %d (REJECT re-insertada automáticamente)", len(removedSKUs), sealerID),
			"sealer_id":           sealerID,
			"sorter_id":           targetSorter.GetID(),
			"skus_removed_memory": len(removedSKUs),
			"skus_removed_db":     rowsDeleted,
			"removed_skus":        removedSKUs,
			"note":                "SKU REJECT re-insertada automáticamente en memoria y DB",
		}

		c.JSON(http.StatusOK, response)
	})
}

func (h *HTTPFrontend) Start() error {
	h.setupRoutes()

	// Configurar rutas de WebSocket
	SetupWebSocketRoutes(h.router, h.wsHub)

	return h.router.Run(h.addr)
}

func (h *HTTPFrontend) GetRouter() *gin.Engine {
	return h.router
}
