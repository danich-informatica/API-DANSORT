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

// Estructura para inserción asíncrona en DB
type insertRequest struct {
	especie  string
	variedad string
	calibre  string
	embalaje string
	sku      string
	message  string
	resultCh chan insertResult
}

type insertResult struct {
	correlativo string
	err         error
}

type CognexListener struct {
	remoteHost  string // Host remoto de donde viene la cámara (solo informativo)
	port        int
	scan_method string // "QR" o "DATAMATRIX"
	listener    net.Listener
	ctx         context.Context
	cancel      context.CancelFunc
	dbManager   *db.PostgresManager
	EventChan   chan models.LecturaEvent
	insertChan  chan insertRequest // Canal para inserciones asíncronas
	dispositivo string
}

func NewCognexListener(remoteHost string, port int, scan_method string, dbManager *db.PostgresManager) *CognexListener {
	ctx, cancel := context.WithCancel(context.Background())
	dispositivo := fmt.Sprintf("Cognex-%s:%d", remoteHost, port)
	cl := &CognexListener{
		remoteHost:  remoteHost,
		port:        port,
		ctx:         ctx,
		scan_method: scan_method,
		cancel:      cancel,
		dbManager:   dbManager,
		EventChan:   make(chan models.LecturaEvent, 100), // buffer para 100 eventos
		insertChan:  make(chan insertRequest, 200),       // buffer para 200 inserciones pendientes
		dispositivo: dispositivo,
	}

	// Iniciar worker para inserciones asíncronas
	go cl.insertWorker()

	return cl
}

// String implementa la interfaz fmt.Stringer
func (c *CognexListener) String() string {
	return fmt.Sprintf("CognexListener{remote: %s, port: %d}", c.remoteHost, c.port)
}

// insertWorker procesa inserciones a la base de datos de forma asíncrona
func (c *CognexListener) insertWorker() {
	for {
		select {
		case <-c.ctx.Done():
			// Procesar requests pendientes antes de salir
			close(c.insertChan)
			for req := range c.insertChan {
				c.processInsert(req)
			}
			return

		case req := <-c.insertChan:
			c.processInsert(req)
		}
	}
}

// processInsert realiza la inserción real en la base de datos
func (c *CognexListener) processInsert(req insertRequest) {
	correlativo, err := c.dbManager.InsertNewBox(
		context.Background(),
		req.especie,
		req.variedad,
		req.calibre,
		req.embalaje,
	)

	req.resultCh <- insertResult{
		correlativo: correlativo,
		err:         err,
	}
}

// Start inicia el servidor TCP para escuchar mensajes de Cognex
func (c *CognexListener) Start() error {
	// Siempre escuchar en todas las interfaces (0.0.0.0)
	address := fmt.Sprintf("0.0.0.0:%d", c.port)

	listener, err := net.Listen("tcp", address)
	if err != nil {
		return fmt.Errorf("error al crear listener: %w", err)
	}

	c.listener = listener
	log.Printf("✓ CognexListener escuchando en %s (esperando conexiones desde %s)\n", address, c.remoteHost)

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

		// Inserción asíncrona en DB para máxima velocidad
		resultCh := make(chan insertResult, 1)
		insertReq := insertRequest{
			especie:  especie,
			variedad: variedad,
			calibre:  calibre,
			embalaje: embalaje,
			sku:      sku.SKU,
			message:  message,
			resultCh: resultCh,
		}

		// Enviar al worker (no bloqueante si hay buffer disponible)
		select {
		case c.insertChan <- insertReq:
			// Enviado al worker, responder inmediatamente ACK
			response := "ACK\r\n"
			conn.Write([]byte(response))

			// Procesar resultado en goroutine para no bloquear
			go func() {
				result := <-resultCh
				if result.err != nil {
					log.Printf("❌ Error al insertar caja en DB desde mensaje: %s | Error: %v", message, result.err)
					c.EventChan <- models.NewLecturaFallidaConDatos(
						fmt.Errorf("error al insertar: %w", result.err),
						especie,
						calibre,
						variedad,
						embalaje,
						message,
						c.dispositivo,
					)
				} else {
					log.Printf("📦 Correlativo de caja insertado: %s", result.correlativo)
					log.Printf("✅ Caja insertada | Correlativo: %s | SKU: %s | Especie: %s", result.correlativo, sku.SKU, especie)
					c.EventChan <- models.NewLecturaExitosa(
						sku.SKU,
						especie,
						calibre,
						variedad,
						embalaje,
						result.correlativo,
						message,
						c.dispositivo,
					)
				}
			}()

		default:
			// Buffer lleno, responder NACK y procesar síncrono
			log.Printf("⚠️  Buffer de inserciones lleno, procesando síncronamente")
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

			response := "ACK\r\n"
			conn.Write([]byte(response))
			log.Printf("📦 Correlativo de caja insertado: %s", correlativo)
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
		}
	case "DATAMATRIX":
		log.Printf("📊 DataMatrix detectado: %s", strings.TrimSpace(message))
		message = strings.TrimSpace(message)

		if message == "" {
			log.Printf("❌ Mensaje DataMatrix vacío recibido")
			response := "NACK\r\n"
			c.EventChan <- models.NewLecturaFallida(fmt.Errorf("mensaje datamatrix vacío"), message, c.dispositivo)
			if _, err := conn.Write([]byte(response)); err != nil {
				log.Printf("Error al enviar respuesta NACK: %v\n", err)
			}
			return
		}

		// Crear y enviar un evento específico para DataMatrix.
		// Este evento será recogido por el Sorter, que buscará la salida asociada
		// al dispositivo y ejecutará la lógica correspondiente con el código leído.
		// NOTA: Se asume la existencia de 'NewLecturaDataMatrix(dispositivo, codigo)' en el paquete 'models'.
		log.Printf("✅ Enviando evento DataMatrix para dispositivo '%s' con código '%s'", c.dispositivo, message)
		c.EventChan <- models.NewLecturaDataMatrix(c.dispositivo, message)

		// Enviar confirmación inmediata a la cámara
		response := "ACK\r\n"
		if _, err := conn.Write([]byte(response)); err != nil {
			log.Printf("Error al enviar respuesta ACK: %v\n", err)
		}
		return

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
