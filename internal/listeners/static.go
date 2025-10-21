package listeners

import (
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
)

// StartStaticFileServer inicia un servidor HTTP para servir archivos estáticos
// desde un directorio específico (típicamente frontend builds como Nuxt, React, etc.)
func StartStaticFileServer(addr string, distPath string) error {
	// Verificar que el directorio existe
	if _, err := os.Stat(distPath); os.IsNotExist(err) {
		log.Printf("⚠️  Directorio de frontend no encontrado: %s", distPath)
		log.Println("   El servidor de frontend no se iniciará")
		return err
	}

	// Crear router Gin para el frontend
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery())

	// Servir archivos estáticos
	router.NoRoute(func(c *gin.Context) {
		path := c.Request.URL.Path

		// Rutas especiales de Nuxt
		if strings.HasPrefix(path, "/_nuxt/") ||
			strings.HasPrefix(path, "/img/") ||
			path == "/favicon.ico" ||
			path == "/robots.txt" {
			c.File(filepath.Join(distPath, path))
			return
		}

		// Para rutas HTML (SPA routing)
		// Intentar servir el archivo específico
		fullPath := filepath.Join(distPath, path)

		// Si es un directorio, buscar index.html
		if info, err := os.Stat(fullPath); err == nil && info.IsDir() {
			indexPath := filepath.Join(fullPath, "index.html")
			if _, err := os.Stat(indexPath); err == nil {
				c.File(indexPath)
				return
			}
		}

		// Si el archivo existe, servirlo
		if _, err := os.Stat(fullPath); err == nil {
			c.File(fullPath)
			return
		}

		// Si no existe, intentar con .html
		htmlPath := fullPath + ".html"
		if _, err := os.Stat(htmlPath); err == nil {
			c.File(htmlPath)
			return
		}

		// Si no encuentra nada, servir index.html (SPA fallback)
		indexPath := filepath.Join(distPath, "index.html")
		if _, err := os.Stat(indexPath); err == nil {
			c.File(indexPath)
			return
		}

		// Si tampoco hay index.html, 404
		c.File(filepath.Join(distPath, "404.html"))
	})

	log.Printf("✅ Servidor de frontend listo en http://%s", addr)
	log.Printf("   📁 Archivos estáticos: %s", distPath)
	log.Println("")

	// Iniciar servidor
	return router.Run(addr)
}
