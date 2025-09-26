package listeners

import (
    "github.com/gin-gonic/gin"
    "net/http"
)

type HTTPFrontend struct {
    router *gin.Engine
    port   string
}

func NewHTTPFrontend(port string) *HTTPFrontend {
    return &HTTPFrontend{
        router: gin.Default(),
        port:   port,
    }
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
}

func (h *HTTPFrontend) Start() error {
    h.setupRoutes()
    return h.router.Run(":" + h.port)
}

func (h *HTTPFrontend) GetRouter() *gin.Engine {
    return h.router
}