package main

import (
	"fmt"
	"log"
	"math/rand"
	"net"
	"strings"
	"time"

	"api-dansort/internal/models"
)

// Combinaciones válidas de SKUs basadas en la tabla de la base de datos
var skusCombinaciones = []struct {
	Calibre  string
	Variedad string
	Embalaje string
}{
	{"XL", "V018", "CECDCAM5"},
	{"4J", "V018", "CEMDSAM4"},
	{"2J", "V018", "CEMGKAM5"},
	{"2J", "V018", "CEMDSAM4"},
	{"XLD", "V018", "CECGKAM5"},
	{"2J", "V022", "CEMGKAM5"},
	{"3J", "V022", "CEMGKAM5"},
	{"J", "V022", "CEMGKAM5"},
	{"JD", "V022", "CEMGKAM5"},
}

var (
	especies = []string{"E003", "E001", "E005", "E007"}
	marcas   = []string{"CHLION", "GREENEX", "PREMIUM", "SELECT"}
)

func simular_escaneo() {
	// Esperar 1 segundo para que el listener esté listo
	time.Sleep(1 * time.Second)

	// Conectar al listener
	conn, err := net.Dial("tcp", "localhost:8085")
	if err != nil {
		log.Fatalf("Error al conectar: %v", err)
	}
	defer conn.Close()

	log.Println("✓ Conectado al CognexListener - Iniciando simulación de escaneo QR")

	// Simular envío de códigos QR cada 250 milisegundos
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()

	contador := 1

	for range ticker.C {
		var message string

		// 🎲 Simular NO_READ aproximadamente el 15% de las veces
		if rand.Intn(100) < 15 {
			// Enviar mensaje NO_READ
			message = models.NO_READ_CODE + "\r\n"
			log.Printf("📤 [Escaneo #%d] Enviado: %s (simulando lectura fallida)", contador, models.NO_READ_CODE)
		} else {
			// Seleccionar una combinación válida de SKU de la tabla
			skuCombo := skusCombinaciones[rand.Intn(len(skusCombinaciones))]

			// Generar código QR con la estructura: E003;XL;0;CECDCAM5;CHLION;V018
			especie := especies[rand.Intn(len(especies))]
			calibre := skuCombo.Calibre
			invertirColores := "0" // Puede ser "0" o "1" según necesites
			embalaje := skuCombo.Embalaje
			marca := marcas[rand.Intn(len(marcas))]
			variedad := skuCombo.Variedad

			// Construir el código QR
			qrCode := fmt.Sprintf("%s;%s;%s;%s;%s;%s",
				especie, calibre, invertirColores, embalaje, marca, variedad)

			message = qrCode + "\r\n"
			log.Printf("📤 [Escaneo #%d] Enviado: %s", contador, qrCode)
		}

		// Enviar mensaje
		_, err := conn.Write([]byte(message))
		if err != nil {
			log.Printf("❌ Error al enviar: %v", err)
			return
		}

		contador++

		// Leer respuesta (ACK/NACK)
		buffer := make([]byte, 1024)
		conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		n, err := conn.Read(buffer)
		if err != nil {
			log.Printf("⚠️  Error al leer respuesta: %v", err)
			continue
		}

		response := strings.TrimSpace(string(buffer[:n]))
		if response == "NACK" {
			log.Printf("📥 Respuesta: ❌ NACK (mensaje rechazado)")
		} else {
			log.Printf("📥 Respuesta: ✅ %s", response)
		}
	}
}

func main() {
	log.Println("🎯 Simulador de Cognex - API Greenex")
	log.Println("Conectando al listener en localhost:8085...")

	// Iniciar simulación de escaneo
	simular_escaneo()
}
