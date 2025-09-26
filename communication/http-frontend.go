package communication

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
    // Ruta GET de prueba
    h.router.GET("/ping", func(c *gin.Context) {
        c.JSON(http.StatusOK, gin.H{
            "message": "pong",
        })
    })

    // Ruta POST de ejemplo
    h.router.POST("/echo", func(c *gin.Context) {
        var json map[string]interface{}
        if err := c.ShouldBindJSON(&json); err != nil {
            c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
            return
        }
        c.JSON(http.StatusOK, json)
    })
}

func (h *HTTPFrontend) Start() error {
    h.setupRoutes()
    return h.router.Run(":" + h.port)
}

func (h *HTTPFrontend) GetRouter() *gin.Engine {
    return h.router
}