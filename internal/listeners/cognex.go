package listeners

import (
	"API-GREENEX/internal/db"
	"API-GREENEX/internal/models"
	"bufio"
	"context"
	"fmt"
	"log"
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
	EventChan   chan models.LecturaEvent
	dispositivo string
}

func NewCognexListener(host string, port int, scan_method string, dbManager *db.PostgresManager) *CognexListener {
	ctx, cancel := context.WithCancel(context.Background())
	dispositivo := fmt.Sprintf("Cognex-%s:%d", host, port)
	return &CognexListener{
		host:        host,
		port:        port,
		ctx:         ctx,
		scan_method: scan_method,
		cancel:      cancel,
		dbManager:   dbManager,
		EventChan:   make(chan models.LecturaEvent, 100), // buffer para 100 eventos
		dispositivo: dispositivo,
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

	message = strings.TrimSpace(message)
	switch c.scan_method {
	case "QR":
		message_splitted := strings.Split(message, ";")
		if message == "" {
			log.Printf("❌ Mensaje vacío recibido")
			response := "NACK\r\n"

			c.EventChan <- models.NewLecturaFallida(fmt.Errorf("mensaje vacío"), message, c.dispositivo)

			conn.Write([]byte(response))
			return
		}

		if strings.TrimSpace(message) == models.NO_READ_CODE {
			log.Printf("❌ Código NO_READ recibido")
			response := "NACK\r\n"
			c.EventChan <- models.NewLecturaFallida(fmt.Errorf("NO_READ"), message, c.dispositivo)
			conn.Write([]byte(response))
			return
		}
		// Validar que el mensaje tiene el formato correcto (6 partes)
		if len(message_splitted) < 6 {
			log.Printf("❌ Mensaje inválido (partes insuficientes): %s (tiene %d partes, necesita 6)", message, len(message_splitted))
			response := "NACK\r\n"
			c.EventChan <- models.NewLecturaFallida(fmt.Errorf("formato inválido"), message, c.dispositivo)
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
			c.EventChan <- models.NewLecturaFallida(fmt.Errorf("componentes vacíos"), message, c.dispositivo)
			conn.Write([]byte(response))
			return
		}

		// Generar SKU para validación
		sku, err := models.RequestSKU(variedad, calibre, embalaje)
		if err != nil {
			log.Printf("❌ SKU inválido generado desde mensaje: %s | Error: %v", message, err)
			response := "NACK\r\n"
			c.EventChan <- models.NewLecturaFallidaConDatos(
				err,
				especie,
				calibre,
				variedad,
				embalaje,
				message,
				c.dispositivo,
			)
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
			c.EventChan <- models.NewLecturaFallidaConDatos(
				fmt.Errorf("error al insertar: %w", err),
				especie,
				calibre,
				variedad,
				embalaje,
				message,
				c.dispositivo,
			)
			conn.Write([]byte(response))
			return
		}

		log.Printf("✅ Caja insertada | Correlativo: %s | SKU: %s | Especie: %s", correlativo, sku.SKU, especie)
		c.EventChan <- models.NewLecturaExitosa(
			sku.SKU,
			especie,
			calibre,
			variedad,
			embalaje,
			correlativo,
			message,
			c.dispositivo,
		)
	case "DATAMATRIX":
		log.Printf("✅ Escaneo DataMatrix válido: %s ", strings.TrimSpace(message))
	default:
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

	close(c.EventChan)

	return nil
}
