package main

import (
	"fmt"
	"log"
	"math/rand"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

// Código NO_READ constante
const NO_READ_CODE = "NO_READ"

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

func simular_escaneo(host string, intervaloMs int, noReadPercentage int) {
	// Esperar 1 segundo para asegurar que el listener esté listo
	time.Sleep(5 * time.Second)

	// Conectar al listener
	log.Printf("📡 Conectando a %s...", host)
	conn, err := net.Dial("tcp", host)
	if err != nil {
		log.Fatalf("❌ Error al conectar: %v", err)
	}
	defer conn.Close()

	log.Printf("✅ Conectado al CognexListener en %s", host)
	log.Printf("⚙️  Intervalo: %dms | NO_READ: %d%%", intervaloMs, noReadPercentage)
	log.Println("🚀 Iniciando simulación de escaneo QR...")
	log.Println("")

	// Simular envío de códigos QR
	ticker := time.NewTicker(time.Duration(intervaloMs) * time.Millisecond)
	defer ticker.Stop()

	// Canal para detectar Ctrl+C
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	contador := 1

	for {
		select {
		case <-sigChan:
			log.Println("")
			log.Println("🛑 Deteniendo simulador...")
			return

		case <-ticker.C:
			var message string

			// 🎲 Simular NO_READ según el porcentaje configurado
			if rand.Intn(100) < noReadPercentage {
				// Enviar mensaje NO_READ
				message = NO_READ_CODE + "\r\n"
				log.Printf("📤 #%-4d → %s (simulando lectura fallida)", contador, NO_READ_CODE)
			} else {
				// Seleccionar una combinación válida de SKU
				skuCombo := skusCombinaciones[rand.Intn(len(skusCombinaciones))]

				// Generar código QR con la estructura: E003;XL;0;CECDCAM5;CHLION;V018
				especie := especies[rand.Intn(len(especies))]
				calibre := skuCombo.Calibre
				invertirColores := "0"
				embalaje := skuCombo.Embalaje
				marca := marcas[rand.Intn(len(marcas))]
				variedad := skuCombo.Variedad

				// Construir el código QR
				qrCode := fmt.Sprintf("%s;%s;%s;%s;%s;%s",
					especie, calibre, invertirColores, embalaje, marca, variedad)

				message = qrCode + "\r\n"

				// Construir el SKU para el log
				sku := fmt.Sprintf("%s-%s-%s", calibre, variedad, embalaje)
				log.Printf("📤 #%-4d → %s (SKU: %s)", contador, qrCode, sku)
			}

			// Enviar mensaje
			_, err := conn.Write([]byte(message))
			if err != nil {
				log.Printf("❌ Error al enviar: %v", err)
				log.Println("🔄 Intentando reconectar...")
				time.Sleep(5 * time.Second)

				// Intentar reconectar
				conn, err = net.Dial("tcp", host)
				if err != nil {
					log.Fatalf("❌ No se pudo reconectar: %v", err)
				}
				log.Printf("✅ Reconectado a %s", host)
				continue
			}

			// Leer respuesta (ACK/NACK) con timeout
			buffer := make([]byte, 1024)
			conn.SetReadDeadline(time.Now().Add(2 * time.Second))
			n, err := conn.Read(buffer)
			if err != nil {
				log.Printf("      ⚠️  No se recibió respuesta (timeout o error): %v", err)
			} else {
				response := strings.TrimSpace(string(buffer[:n]))
				if response == "NACK" {
					log.Printf("      📥 ❌ NACK (mensaje rechazado)")
				} else {
					log.Printf("      📥 ✅ %s", response)
				}
			}

			contador++
		}
	}
}

func main() {
	// Configuración
	host := "localhost:8085"
	intervaloMs := 250     // Milisegundos entre mensajes
	noReadPercentage := 15 // Porcentaje de NO_READ (0-100)

	// Parsear argumentos (opcional)
	if len(os.Args) > 1 {
		host = os.Args[1]
	}

	// Banner
	log.Println("")
	log.Println("╔═══════════════════════════════════════════════╗")
	log.Println("║   🎯 SIMULADOR COGNEX - API GREENEX          ║")
	log.Println("╚═══════════════════════════════════════════════╝")
	log.Println("")

	// Inicializar generador de números aleatorios
	rand.Seed(time.Now().UnixNano())

	// Iniciar simulación
	simular_escaneo(host, intervaloMs, noReadPercentage)

	log.Println("👋 Simulador detenido correctamente")
}
