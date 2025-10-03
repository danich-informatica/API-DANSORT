package main

import (
	"API-GREENEX/internal/db"
	"API-GREENEX/internal/models"
	"bufio"
	"context"
	"fmt"
	"log"
	"math/rand"
	"net"
	"strings"
	"time"
)

type CognexListener struct {
	host        string
	port        int
	scan_method string // "QR" o "DATAMATRIX"
	listener    net.Listener
	ctx         context.Context
	cancel      context.CancelFunc
	dbManager   *db.PostgresManager
}

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

func NewCognexListener(host string, port int, scan_method string, dbManager *db.PostgresManager) *CognexListener {
	ctx, cancel := context.WithCancel(context.Background())
	return &CognexListener{
		host:        host,
		port:        port,
		ctx:         ctx,
		scan_method: scan_method,
		cancel:      cancel,
		dbManager:   dbManager,
	}
}

// String implementa la interfaz fmt.Stringer
func (c *CognexListener) String() string {
	return fmt.Sprintf("CognexListener{host: %s, port: %d}", c.host, c.port)
}

// Start inicia el servidor TCP para escuchar mensajes de Cognex
func (c *CognexListener) Start() error {
	address := fmt.Sprintf("%s:%d", c.host, c.port)

	listener, err := net.Listen("tcp", address)
	if err != nil {
		return fmt.Errorf("error al crear listener: %w", err)
	}

	c.listener = listener
	log.Printf("✓ CognexListener escuchando en %s\n", address)

	// Aceptar conexiones en una goroutine
	go c.acceptConnections()

	return nil
}

// acceptConnections acepta nuevas conexiones de la cámara Cognex
func (c *CognexListener) acceptConnections() {
	for {
		select {
		case <-c.ctx.Done():
			log.Println("CognexListener: deteniendo aceptación de conexiones")
			return
		default:
			// Establecer timeout para Accept
			c.listener.(*net.TCPListener).SetDeadline(time.Now().Add(1 * time.Second))

			conn, err := c.listener.Accept()
			if err != nil {
				// Si es timeout, continuar el loop
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					continue
				}
				log.Printf("Error al aceptar conexión: %v\n", err)
				continue
			}

			log.Printf("✓ Nueva conexión desde: %s\n", conn.RemoteAddr().String())

			// Manejar cada conexión en su propia goroutine
			go c.handleConnection(conn)
		}
	}
}

// handleConnection maneja los mensajes de una conexión Cognex
func (c *CognexListener) handleConnection(conn net.Conn) {
	defer conn.Close()

	reader := bufio.NewReader(conn)

	for {
		select {
		case <-c.ctx.Done():
			log.Printf("Cerrando conexión con %s\n", conn.RemoteAddr().String())
			return
		default:
			// Establecer timeout de lectura
			conn.SetReadDeadline(time.Now().Add(30 * time.Second))

			// Leer mensaje (Cognex suele terminar líneas con \r\n)
			message, err := reader.ReadString('\n')
			if err != nil {
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					continue
				}
				log.Printf("Conexión cerrada o error de lectura: %v\n", err)
				return
			}

			// Procesar el mensaje recibido
			c.processMessage(message, conn)
		}
	}
}

// processMessage procesa los mensajes recibidos de Cognex
func (c *CognexListener) processMessage(message string, conn net.Conn) {
	log.Printf("📦 Mensaje recibido de %s: %s", conn.RemoteAddr().String(), message)

	message_splitted := strings.Split(message, ";")
	if c.scan_method == "QR" {
		// Validar que el mensaje tiene el formato correcto (6 partes)
		if len(message_splitted) < 6 {
			log.Printf("❌ Mensaje inválido (partes insuficientes): %s (tiene %d partes, necesita 6)", message, len(message_splitted))
			response := "NACK\r\n"
			conn.Write([]byte(response))
			return
		}

		// Extraer componentes
		especie := strings.TrimSpace(message_splitted[0])
		calibre := strings.TrimSpace(message_splitted[1])
		embalaje := strings.TrimSpace(message_splitted[3])
		variedad := strings.TrimSpace(message_splitted[5])

		// Validar que los componentes no estén vacíos
		if especie == "" || calibre == "" || embalaje == "" || variedad == "" {
			log.Printf("❌ Componentes vacíos en mensaje: %s", message)
			response := "NACK\r\n"
			conn.Write([]byte(response))
			return
		}

		// Generar SKU para validación
		sku, err := models.RequestSKU(variedad, calibre, embalaje)
		if err != nil {
			log.Printf("❌ SKU inválido generado desde mensaje: %s | Error: %v", message, err)
			response := "NACK\r\n"
			conn.Write([]byte(response))
			return
		}

		// Correlativo ahora es string
		correlativo, err := c.dbManager.InsertNewBox(
			context.Background(),
			especie,
			variedad,
			calibre,
			embalaje,
		)

		if err != nil {
			log.Printf("❌ Error al insertar caja en DB desde mensaje: %s | Error: %v", message, err)
			response := "NACK\r\n"
			conn.Write([]byte(response))
			return
		}

		log.Printf("✅ Caja insertada | Correlativo: %s | SKU: %s | Especie: %s", correlativo, sku.SKU, especie)
	} else if c.scan_method == "DATAMATRIX" {
		log.Printf("✅ Escaneo DataMatrix válido: %s ", strings.TrimSpace(message))
	} else {
		log.Printf("❌ Método de escaneo desconocido: %s", c.scan_method)
		response := "NACK\r\n"
		conn.Write([]byte(response))
		return
	}

	// Enviar confirmación (ACK)
	response := "ACK\r\n"
	_, err := conn.Write([]byte(response))
	if err != nil {
		log.Printf("Error al enviar respuesta: %v\n", err)
	}
}

// Stop detiene el listener
func (c *CognexListener) Stop() error {
	log.Println("Deteniendo CognexListener...")
	c.cancel()

	if c.listener != nil {
		return c.listener.Close()
	}

	return nil
}

func simular_escaneo() {
	// Esperar 1 segundo para que el listener esté listo
	time.Sleep(1 * time.Second)

	// Conectar al listener
	conn, err := net.Dial("tcp", "localhost:8080")
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

		message := qrCode + "\r\n"

		// Enviar mensaje
		_, err := conn.Write([]byte(message))
		if err != nil {
			log.Printf("❌ Error al enviar: %v", err)
			return
		}

		log.Printf("📤 [Escaneo #%d] Enviado: %s", contador, qrCode)
		contador++

		// Leer respuesta (ACK)
		buffer := make([]byte, 1024)
		conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		n, err := conn.Read(buffer)
		if err != nil {
			log.Printf("⚠️  Error al leer respuesta: %v", err)
			continue
		}

		log.Printf("📥 Respuesta recibida: %s", string(buffer[:n]))
	}
}

func main() {
	// 1. Crear manager de PostgreSQL
	dbManager, err := db.GetPostgresManager(context.Background())
	if err != nil {
		log.Fatalf("❌ Error al inicializar PostgreSQL: %v", err)
	}
	defer dbManager.Close()

	log.Println("✅ Base de datos PostgreSQL inicializada correctamente")

	// 2. Crear listener
	cognex := NewCognexListener("127.0.0.1", 8080, "QR", dbManager)

	// 3. Iniciar listener
	if err := cognex.Start(); err != nil {
		log.Fatalf("❌ Error al iniciar CognexListener: %v", err)
	}

	// 4. Iniciar simulación de escaneo
	go simular_escaneo()

	// 5. Mantener el programa corriendo
	log.Println("Presiona Ctrl+C para detener...")
	select {}
}
