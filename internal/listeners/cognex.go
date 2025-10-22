package listeners

import (
	"API-GREENEX/internal/db"
	"API-GREENEX/internal/models"
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
	dark     int
	sku      string
	message  string
	resultCh chan insertResult
}

type insertResult struct {
	correlativo    string
	nombreVariedad string // Nombre de variedad para construir SKU correcto
	err            error
}

type CognexListener struct {
	id             int    // ID numérico del Cognex (del config)
	remoteHost     string // Host remoto de donde viene la cámara (solo informativo)
	port           int
	scan_method    string // "QR" o "DATAMATRIX"
	listener       net.Listener
	ctx            context.Context
	cancel         context.CancelFunc
	dbManager      *db.PostgresManager
	EventChan      chan models.LecturaEvent    // Canal para lecturas QR/SKU (flujo original)
	DataMatrixChan chan models.DataMatrixEvent // Canal para lecturas DataMatrix (nuevo flujo)
	insertChan     chan insertRequest          // Canal para inserciones asíncronas
	dispositivo    string
}

func NewCognexListener(id int, remoteHost string, port int, scan_method string, dbManager *db.PostgresManager) *CognexListener {
	ctx, cancel := context.WithCancel(context.Background())
	dispositivo := fmt.Sprintf("Cognex-%d:%d", id, port)
	cl := &CognexListener{
		id:             id,
		remoteHost:     remoteHost,
		port:           port,
		ctx:            ctx,
		scan_method:    scan_method,
		cancel:         cancel,
		dbManager:      dbManager,
		EventChan:      make(chan models.LecturaEvent, 100),    // buffer para 100 eventos QR/SKU
		DataMatrixChan: make(chan models.DataMatrixEvent, 100), // buffer para 100 eventos DataMatrix
		insertChan:     make(chan insertRequest, 200),          // buffer para 200 inserciones pendientes
		dispositivo:    dispositivo,
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
	ctx := context.Background()
	correlativo, err := c.dbManager.InsertNewBox(
		ctx,
		req.especie,
		req.variedad,
		req.calibre,
		req.embalaje,
		req.dark,
	)

	// Obtener nombre de variedad para construir SKU correctamente
	nombreVariedad := ""
	if err == nil {
		nombreVar, errNombre := c.dbManager.GetNombreVariedad(ctx, req.variedad)
		if errNombre == nil && nombreVar != "" {
			nombreVariedad = nombreVar
		} else {
			// Si no se encuentra nombre, usar código
			nombreVariedad = req.variedad
		}
	}

	req.resultCh <- insertResult{
		correlativo:    correlativo,
		nombreVariedad: nombreVariedad,
		err:            err,
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
// handleConnection maneja los mensajes de una conexión Cognex
func (c *CognexListener) handleConnection(conn net.Conn) {
	defer conn.Close()

	buffer := make([]byte, 4096) // Buffer para lectura directa

	for {
		select {
		case <-c.ctx.Done():
			log.Printf("Cerrando conexión con %s\n", conn.RemoteAddr().String())
			return
		default:
			// Establecer timeout de lectura
			conn.SetReadDeadline(time.Now().Add(30 * time.Second))

			// Leer datos del socket
			n, err := conn.Read(buffer)
			if err != nil {
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					continue
				}
				log.Printf("Conexión cerrada o error de lectura: %v\n", err)
				return
			}

			if n > 0 {
				// Convertir a string y procesar
				message := string(buffer[:n])
				log.Printf("� Datos recibidos (%d bytes): %s\n", n, message)
				c.processMessage(message, conn)
			}
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
		// Validar que el mensaje tiene el formato correcto (5 partes)
		if len(message_splitted) < 5 {
			log.Printf("❌ Mensaje inválido (partes insuficientes): %s (tiene %d partes, necesita 5)", message, len(message_splitted))
			response := "NACK\r\n"
			c.EventChan <- models.NewLecturaFallida(fmt.Errorf("formato inválido"), message, c.dispositivo)
			conn.Write([]byte(response))
			return
		}

		// Extraer componentes (formato: Especie;Calibre;Dark;Embalaje;Variedad)
		especie := strings.TrimSpace(message_splitted[0])
		calibre := strings.TrimSpace(message_splitted[1])
		darkStr := strings.TrimSpace(message_splitted[2])
		embalaje := strings.TrimSpace(message_splitted[3])
		// message_splitted[4] es etiqueta/marca (no se usa)
		variedad := strings.TrimSpace(message_splitted[4])

		// Validar que los componentes no estén vacíos
		if especie == "" || calibre == "" || embalaje == "" || variedad == "" {
			log.Printf("❌ Componentes vacíos en mensaje: %s", message)
			response := "NACK\r\n"
			c.EventChan <- models.NewLecturaFallida(fmt.Errorf("componentes vacíos"), message, c.dispositivo)
			conn.Write([]byte(response))
			return
		}

		// Parsear el valor de dark (esperamos "0" o "1")
		var dark int
		if darkStr == "0" || darkStr == "" {
			dark = 0
		} else if darkStr == "1" {
			dark = 1
		} else {
			log.Printf("⚠️  Valor dark inválido '%s', usando 0 por defecto", darkStr)
			dark = 0
		}

		// Generar SKU para validación
		sku, err := models.RequestSKU(variedad, calibre, embalaje, dark)
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
			dark:     dark, // Usar el dark extraído del QR
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
					// Reconstruir SKU con nombre de variedad en vez de código
					skuFinal := fmt.Sprintf("%s-%s-%s-%d",
						calibre,
						strings.ToUpper(result.nombreVariedad),
						embalaje,
						dark)

					log.Printf("📦 Correlativo de caja insertado: %s", result.correlativo)
					log.Printf("✅ Caja insertada | Correlativo: %s | SKU: %s | Especie: %s", result.correlativo, skuFinal, especie)
					c.EventChan <- models.NewLecturaExitosa(
						skuFinal,
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
			ctx := context.Background()
			correlativo, err := c.dbManager.InsertNewBox(
				ctx,
				especie,
				variedad,
				calibre,
				embalaje,
				dark, // Usar el dark extraído del QR
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

			// Obtener nombre de variedad para construir SKU correctamente
			nombreVariedad := variedad
			nombreVar, errNombre := c.dbManager.GetNombreVariedad(ctx, variedad)
			if errNombre == nil && nombreVar != "" {
				nombreVariedad = nombreVar
			}

			// Reconstruir SKU con nombre de variedad en vez de código
			skuFinal := fmt.Sprintf("%s-%s-%s-%d",
				calibre,
				strings.ToUpper(nombreVariedad),
				embalaje,
				dark)

			response := "ACK\r\n"
			conn.Write([]byte(response))
			log.Printf("📦 Correlativo de caja insertado: %s", correlativo)
			log.Printf("✅ Caja insertada | Correlativo: %s | SKU: %s | Especie: %s", correlativo, skuFinal, especie)
			c.EventChan <- models.NewLecturaExitosa(
				skuFinal,
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
			if _, err := conn.Write([]byte(response)); err != nil {
				log.Printf("Error al enviar respuesta NACK: %v\n", err)
			}
			return
		}

		// Crear y enviar evento DataMatrix al nuevo canal dedicado
		// Este flujo es SEPARADO del flujo QR/SKU original
		dmEvent := models.NewDataMatrixEvent(message, c.dispositivo, c.id, message)
		log.Printf("✅ [Cognex#%d] DataMatrix → Canal dedicado | Código: %s", c.id, message)

		select {
		case c.DataMatrixChan <- dmEvent:
			log.Printf("   ✓ Evento DataMatrix enviado correctamente")
		case <-time.After(2 * time.Second):
			log.Printf("   ⚠️  Timeout enviando evento DataMatrix (canal lleno?)")
		}

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
