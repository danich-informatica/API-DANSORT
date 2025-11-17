package listeners

import (
	"API-DANSORT/internal/db"
	"API-DANSORT/internal/flow"
	"API-DANSORT/internal/models"
	"context"
	"fmt"
	"log"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"
)

// logTs imprime log con timestamp de microsegundos para debugging preciso
func logTs(format string, args ...interface{}) {
	ts := time.Now().Format("2006-01-02T15:04:05.000000")
	log.Printf("[%s] "+format, append([]interface{}{ts}, args...)...)
}

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

// CognexMetrics contiene contadores de eventos para diagnóstico
type CognexMetrics struct {
	mu                  sync.RWMutex
	MessagesReceived    int64 // Total de mensajes recibidos (incluye NO_READ, QR, ID-DB)
	MessagesNoRead      int64 // Mensajes NO_READ
	MessagesQR          int64 // Mensajes con formato QR
	MessagesIDDB        int64 // Mensajes ID-DB (numéricos)
	MessagesInvalid     int64 // Mensajes con formato inválido
	EventsGenerated     int64 // Eventos generados (exitosos + fallidos)
	EventsSuccess       int64 // Eventos de lectura exitosa
	EventsFailed        int64 // Eventos de lectura fallida
	LastMessageTime     time.Time
	LastMessageReceived string
}

// CognexListener maneja la conexión y el procesamiento de datos para un dispositivo Cognex.
type CognexListener struct {
	ID             int
	Name           string
	RemoteHost     string
	Port           int
	ScanMethod     string
	dbManager      *db.PostgresManager
	ssmsManager    *db.Manager           // Restaurado para consultas a UNITEC
	boxCache       *flow.BoxCacheManager // NUEVO: Caché optimizado para ID-DB
	conn           net.Conn
	mu             sync.Mutex
	isRunning      bool
	ctx            context.Context
	cancel         context.CancelFunc
	listener       net.Listener
	EventChan      chan models.LecturaEvent
	DataMatrixChan chan models.DataMatrixEvent
	insertChan     chan insertRequest
	dispositivo    string
	isStopped      chan struct{}
	reconnectChan  chan struct{}
	messageChannel chan string
	metrics        CognexMetrics // NUEVO: Métricas de diagnóstico
}

func NewCognexListener(id int, remoteHost string, port int, scanMethod string, dbManager *db.PostgresManager, ssmsManager *db.Manager, boxCache *flow.BoxCacheManager) *CognexListener {
	log.Printf("[Cognex-%d] NewCognexListener creado con método: %s", id, scanMethod)
	ctx, cancel := context.WithCancel(context.Background())
	dispositivo := fmt.Sprintf("Cognex-%d:%d", id, port)
	cl := &CognexListener{
		ID:             id,
		Name:           fmt.Sprintf("Cognex-%d", id),
		RemoteHost:     remoteHost,
		Port:           port,
		ctx:            ctx,
		ScanMethod:     scanMethod,
		cancel:         cancel,
		dbManager:      dbManager,
		ssmsManager:    ssmsManager, // Asignar el gestor de SQL Server
		boxCache:       boxCache,    // NUEVO: Asignar caché de cajas
		EventChan:      make(chan models.LecturaEvent, 100),
		DataMatrixChan: make(chan models.DataMatrixEvent, 100),
		insertChan:     make(chan insertRequest, 200),
		dispositivo:    dispositivo,
		isStopped:      make(chan struct{}),
		reconnectChan:  make(chan struct{}, 1),
		messageChannel: make(chan string, 100),
	}

	// Iniciar worker para inserciones asíncronas
	go cl.insertWorker()

	// Iniciar logger de métricas periódico (cada 30 segundos)
	go cl.metricsLogger()

	return cl
}

// GetMetrics devuelve una copia de las métricas actuales
func (c *CognexListener) GetMetrics() CognexMetrics {
	c.metrics.mu.RLock()
	defer c.metrics.mu.RUnlock()
	return c.metrics
}

// metricsLogger imprime métricas periódicamente para monitoreo
func (c *CognexListener) metricsLogger() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-c.ctx.Done():
			return
		case <-ticker.C:
			c.metrics.mu.RLock()
			m := c.metrics
			c.metrics.mu.RUnlock()

			if m.MessagesReceived > 0 {
				log.Printf("📊 [Cognex-%d] Métricas:")
				log.Printf("   📦 Mensajes recibidos: %d (NO_READ: %d, QR: %d, ID-DB: %d, Inválidos: %d)",
					m.MessagesReceived, m.MessagesNoRead, m.MessagesQR, m.MessagesIDDB, m.MessagesInvalid)
				log.Printf("   ✅ Eventos: Total=%d | Exitosos=%d | Fallidos=%d | Tasa=%.1f%%",
					m.EventsGenerated, m.EventsSuccess, m.EventsFailed,
					float64(m.EventsSuccess)/float64(m.EventsGenerated)*100)
				if !m.LastMessageTime.IsZero() {
					log.Printf("   🕒 Último mensaje: %s (hace %s)",
						m.LastMessageTime.Format("15:04:05"),
						time.Since(m.LastMessageTime).Round(time.Second))
				}
			}
		}
	}
}

// fetchSKUFromUnitec consulta la base de datos de UNITEC para obtener los datos de un SKU.
func (c *CognexListener) fetchSKUFromUnitec(ctx context.Context, codCaja string) (string, string, string, string, int, error) {
	var especie, calibre, variedad, embalaje string
	var dark int

	if c.ssmsManager == nil {
		return "", "", "", "", 0, fmt.Errorf("el gestor de SQL Server no está inicializado")
	}

	row := c.ssmsManager.QueryRow(ctx, db.SELECT_UNITEC_DB_DBO_SKU_FROM_CODIGO_CAJA, codCaja)
	err := row.Scan(&especie, &variedad, &calibre, &embalaje, &dark)
	if err != nil {
		return "", "", "", "", 1, fmt.Errorf("error al escanear el resultado de la consulta: %w", err)
	}

	return especie, calibre, variedad, embalaje, dark, nil
}

// fetchSKUFromCache intenta búsqueda en caché primero, con fallback a query directa
// Incluye logs detallados de tiempos para observabilidad
func (c *CognexListener) fetchSKUFromCache(ctx context.Context, codCaja string) (string, string, string, string, int, error) {
	startTotal := time.Now()

	// Si no hay caché configurado, usar método directo
	if c.boxCache == nil {
		log.Printf("⚠️  [Cognex-%d] BoxCache no configurado, usando query directa", c.ID)
		return c.fetchSKUFromUnitec(ctx, codCaja)
	}

	// Intentar búsqueda en caché
	boxData, found := c.boxCache.GetBoxData(codCaja)

	if found {
		// CACHE HIT - Retornar inmediatamente
		totalDuration := time.Since(startTotal)
		log.Printf("⚡ [Cognex-%d] Fetch completado vía CACHÉ en %.3fms", c.ID, totalDuration.Seconds()*1000)
		return boxData.Especie, boxData.Calibre, boxData.Variedad, boxData.Embalaje, boxData.Dark, nil
	}

	// CACHE MISS - Fallback a query directa
	log.Printf("🔄 [Cognex-%d] Fallback a query directa...", c.ID)
	startQuery := time.Now()

	especie, calibre, variedad, embalaje, dark, err := c.fetchSKUFromUnitec(ctx, codCaja)

	queryDuration := time.Since(startQuery)
	totalDuration := time.Since(startTotal)

	if err != nil {
		log.Printf("❌ [Cognex-%d] Query directa falló en %.2fms: %v", c.ID, queryDuration.Seconds()*1000, err)
		return "", "", "", "", 0, err
	}

	log.Printf("✅ [Cognex-%d] Query directa completada en %.2fms", c.ID, queryDuration.Seconds()*1000)
	log.Printf("⚡ [Cognex-%d] Fetch completado vía QUERY en %.2fms total", c.ID, totalDuration.Seconds()*1000)

	return especie, calibre, variedad, embalaje, dark, nil
}

// fetchSKUFromUnitecQR consulta la base de datos de UNITEC para obtener los datos de un SKU a partir de un código QR.
func (c *CognexListener) fetchSKUFromUnitecQR(ctx context.Context, qrCode string) (string, string, string, string, int, error) {
	var especie, calibre, variedad, embalaje string
	var dark int

	if c.ssmsManager == nil {
		return "", "", "", "", 0, fmt.Errorf("el gestor de SQL Server no está inicializado")
	}

	row := c.ssmsManager.QueryRow(ctx, db.SELECT_UNITEC_DB_DBO_SKU_FROM_CODIGO_CAJA, qrCode)
	err := row.Scan(&especie, &variedad, &calibre, &embalaje, &dark)
	if err != nil {
		return "", "", "", "", 0, fmt.Errorf("error al escanear el resultado de la consulta QR: %w", err)
	}

	return especie, calibre, variedad, embalaje, dark, nil
}

// String implementa la interfaz fmt.Stringer
func (c *CognexListener) String() string {
	return fmt.Sprintf("CognexListener{remote: %s, port: %d}", c.RemoteHost, c.Port)
}

// GetID retorna el ID del Cognex
func (c *CognexListener) GetID() int {
	return c.ID
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
	startInsert := time.Now() // ⏱️ Medición completa de inserción
	ctx := context.Background()

	startDB := time.Now()
	correlativo, err := c.dbManager.InsertNewBox(
		ctx,
		req.especie,
		req.variedad,
		req.calibre,
		req.embalaje,
		req.dark,
	)
	dbDuration := time.Since(startDB)

	// Obtener nombre de variedad para construir SKU correctamente
	startVariedad := time.Now()
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
	variedadDuration := time.Since(startVariedad)

	totalInsertDuration := time.Since(startInsert)

	if err == nil {
		log.Printf("⏱️  [Cognex-%d] INSERCIÓN DB: InsertBox=%.2fms + GetVariedad=%.2fms = TOTAL %.2fms",
			c.ID, dbDuration.Seconds()*1000, variedadDuration.Seconds()*1000, totalInsertDuration.Seconds()*1000)
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
	address := fmt.Sprintf("0.0.0.0:%d", c.Port)

	listener, err := net.Listen("tcp", address)
	if err != nil {
		return fmt.Errorf("error al crear listener: %w", err)
	}

	c.listener = listener
	log.Printf("✓ CognexListener escuchando en %s (esperando conexiones desde %s)\n", address, c.RemoteHost)

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

	// ⚡ OPTIMIZACIÓN: TCP Keepalive + NoDelay para latencia mínima
	if tcpConn, ok := conn.(*net.TCPConn); ok {
		tcpConn.SetKeepAlive(true)
		tcpConn.SetKeepAlivePeriod(30 * time.Second)
		tcpConn.SetNoDelay(true) // Deshabilitar algoritmo de Nagle
		tcpConn.SetReadBuffer(256 * 1024)
		tcpConn.SetWriteBuffer(256 * 1024)
	}

	buffer := make([]byte, 4096) // Buffer para lectura directa

	for {
		select {
		case <-c.ctx.Done():
			log.Printf("Cerrando conexión con %s\n", conn.RemoteAddr().String())
			return
		default:
			// ⚡ SIN TIMEOUT - TCP Keepalive detecta conexiones muertas
			// La goroutine NUNCA se cierra por inactividad
			n, err := conn.Read(buffer)
			if err != nil {
				log.Printf("Conexión cerrada o error de lectura: %v\n", err)
				return
			}

			if n > 0 {
				// Convertir a string y procesar
				message := string(buffer[:n])
				logTs("📦 Datos recibidos (%d bytes): %s", n, message)
				c.processMessage(message, conn)
			}
		}
	}
}

// processMessage procesa los mensajes recibidos de Cognex
func (c *CognexListener) processMessage(message string, conn net.Conn) {
	// 📊 Incrementar contador de mensajes recibidos
	c.metrics.mu.Lock()
	c.metrics.MessagesReceived++
	c.metrics.LastMessageTime = time.Now()
	c.metrics.LastMessageReceived = message
	totalReceived := c.metrics.MessagesReceived
	c.metrics.mu.Unlock()

	logTs("📦 Mensaje recibido de %s: %s [Total: %d]", conn.RemoteAddr().String(), message, totalReceived)

	message = strings.TrimSpace(message)
	switch c.ScanMethod {
	case "QR":
		message_splitted := strings.Split(message, ";")
		if message == "" {
			log.Printf("❌ Mensaje vacío recibido")
			response := "NACK\r\n"

			c.metrics.mu.Lock()
			c.metrics.MessagesInvalid++
			c.metrics.EventsFailed++
			c.metrics.EventsGenerated++
			c.metrics.mu.Unlock()

			c.EventChan <- models.NewLecturaFallida(fmt.Errorf("mensaje vacío"), message, c.dispositivo)

			conn.Write([]byte(response))
			return
		}

		if strings.TrimSpace(message) == models.NO_READ_CODE {
			log.Printf("❌ Código NO_READ recibido")
			response := "NACK\r\n"

			c.metrics.mu.Lock()
			c.metrics.MessagesNoRead++
			c.metrics.EventsFailed++
			c.metrics.EventsGenerated++
			c.metrics.mu.Unlock()

			c.EventChan <- models.NewLecturaFallida(fmt.Errorf("NO_READ"), message, c.dispositivo)
			conn.Write([]byte(response))
			return
		}
		// Validar que el mensaje tiene el formato correcto (5 partes)
		if len(message_splitted) < 5 {
			log.Printf("❌ Mensaje inválido (partes insuficientes): %s (tiene %d partes, necesita 5)", message, len(message_splitted))
			response := "NACK\r\n"

			c.metrics.mu.Lock()
			c.metrics.MessagesInvalid++
			c.metrics.EventsFailed++
			c.metrics.EventsGenerated++
			c.metrics.mu.Unlock()

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

		// Incrementar contador de mensajes QR
		c.metrics.mu.Lock()
		c.metrics.MessagesQR++
		c.metrics.mu.Unlock()

		// Enviar al worker (no bloqueante si hay buffer disponible)
		select {
		case c.insertChan <- insertReq:
			// Enviado al worker, responder inmediatamente ACK
			response := "ACK\r\n"
			conn.Write([]byte(response))
			logTs("✅ ACK enviado (async insert en cola)")

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
	case "ID-DB":
		startTotal := time.Now() // ⏱️ INICIO medición flujo completo

		if message == "" {
			log.Printf("❌ Mensaje vacío recibido")
			response := "NACK\r\n"

			c.metrics.mu.Lock()
			c.metrics.MessagesInvalid++
			c.metrics.EventsFailed++
			c.metrics.EventsGenerated++
			c.metrics.mu.Unlock()

			c.EventChan <- models.NewLecturaFallida(fmt.Errorf("mensaje vacío"), message, c.dispositivo)

			conn.Write([]byte(response))
			return
		}

		if message == models.NO_READ_CODE {
			log.Printf("❌ Código NO_READ recibido")
			response := "NACK\r\n"

			c.metrics.mu.Lock()
			c.metrics.MessagesNoRead++
			c.metrics.EventsFailed++
			c.metrics.EventsGenerated++
			c.metrics.mu.Unlock()

			c.EventChan <- models.NewLecturaFallida(fmt.Errorf("NO_READ"), message, c.dispositivo)
			conn.Write([]byte(response))
			return
		}

		_, err := strconv.Atoi(message)

		// El "truco" de Go es verificar si el error es 'nil' (nulo/vacío)
		if err != nil {
			fmt.Printf("ALERTA con '%s': solo se aceptan numeros enteros.\n", message)
			response := "NACK\r\n"

			c.metrics.mu.Lock()
			c.metrics.MessagesInvalid++
			c.metrics.EventsFailed++
			c.metrics.EventsGenerated++
			c.metrics.mu.Unlock()

			c.EventChan <- models.NewLecturaFallida(fmt.Errorf("formato inválido, no es un número entero"), message, c.dispositivo)
			conn.Write([]byte(response))
			return
		}

		// Incrementar contador de mensajes ID-DB válidos
		c.metrics.mu.Lock()
		c.metrics.MessagesIDDB++
		c.metrics.mu.Unlock()

		// El mensaje es un ID de caja, lo usamos para consultar la DB
		startFetch := time.Now()
		especie, calibre, variedad, embalaje, dark, err := c.fetchSKUFromCache(context.Background(), message)

		// limpiamos los datos recibidos
		especie = strings.TrimSpace(especie)
		calibre = strings.TrimSpace(calibre)
		variedad = strings.TrimSpace(variedad)
		embalaje = strings.TrimSpace(embalaje)

		fetchDuration := time.Since(startFetch)
		log.Printf("🔧 [Cognex-%d] Fetch de datos desde UNITEC completado SKU: %q-%q-%q-%q en %.3fms", c.ID, calibre, variedad, embalaje, dark, fetchDuration.Seconds()*1000)

		if err != nil {
			log.Printf("❌ Error al obtener SKU de UNITEC desde ID: %v", err)
			response := "NACK\r\n"
			c.EventChan <- models.NewLecturaFallida(fmt.Errorf("error en UNITEC ID: %w", err), message, c.dispositivo)
			conn.Write([]byte(response))
			return
		}

		startSKU := time.Now()
		sku, err := models.RequestSKU(strings.TrimSpace(variedad), strings.TrimSpace(calibre), strings.TrimSpace(embalaje), dark)
		skuDuration := time.Since(startSKU)
		log.Printf("🔧 [Cognex-%d] Generación de SKU: %s (Variedad=%q, Calibre=%q, Embalaje=%q, Dark=%q)", c.ID, sku.SKU, sku.Variedad, sku.Calibre, sku.Embalaje, sku.Dark)
		if err != nil {
			log.Printf("❌ SKU inválido generado desde datos de UNITEC ID: %s | Error: %v", message, err)
			response := "NACK\r\n"
			c.EventChan <- models.NewLecturaFallida(fmt.Errorf("SKU inválido de UNITEC ID"), message, c.dispositivo)
			conn.Write([]byte(response))
			return
		}

		log.Printf("🔧 [Cognex-%d] Validación SKU completada en %.3fms", c.ID, skuDuration.Seconds()*1000)

		resultCh := make(chan insertResult, 1)
		insertReq := insertRequest{
			especie:  strings.TrimSpace(especie),
			variedad: strings.TrimSpace(variedad),
			calibre:  strings.TrimSpace(calibre),
			embalaje: strings.TrimSpace(embalaje),
			dark:     dark,
			sku:      strings.TrimSpace(sku.SKU),
			message:  strings.TrimSpace(message),
			resultCh: resultCh,
		}

		select {
		case c.insertChan <- insertReq:
			response := "ACK\r\n"
			startACK := time.Now()
			conn.Write([]byte(response))
			ackDuration := time.Since(startACK)

			totalDuration := time.Since(startTotal)
			logTs(fmt.Sprintf("✅ ACK enviado en %.3fms (async insert en cola para ID-DB)", ackDuration.Seconds()*1000))
			log.Printf("⏱️  [Cognex-%d] FLUJO COMPLETO ID-DB: Fetch=%.3fms + SKU=%.3fms + ACK=%.3fms = TOTAL %.3fms",
				c.ID, fetchDuration.Seconds()*1000, skuDuration.Seconds()*1000, ackDuration.Seconds()*1000, totalDuration.Seconds()*1000)

			go func() {
				startInsert := time.Now() // ⏱️ Medición de inserción async
				result := <-resultCh
				insertDuration := time.Since(startInsert)

				if result.err != nil {
					log.Printf("❌ Error al insertar caja en DB desde ID-DB: %s | Error: %v (tardó %.2fms)", message, result.err, insertDuration.Seconds()*1000)

					c.metrics.mu.Lock()
					c.metrics.EventsFailed++
					c.metrics.EventsGenerated++
					c.metrics.mu.Unlock()

					c.EventChan <- models.NewLecturaFallidaConDatos(
						fmt.Errorf("error al insertar: %w", result.err),
						especie, calibre, variedad, embalaje, message, c.dispositivo,
					)
				} else {
					skuFinal := fmt.Sprintf("%s-%s-%s-%d", calibre, strings.ToUpper(result.nombreVariedad), embalaje, dark)
					log.Printf("📦 Correlativo de caja insertado: %s (inserción DB: %.2fms)", result.correlativo, insertDuration.Seconds()*1000)
					log.Printf("✅ Caja insertada | Correlativo: %s | SKU: %s | Especie: %s", result.correlativo, skuFinal, especie)

					c.metrics.mu.Lock()
					c.metrics.EventsSuccess++
					c.metrics.EventsGenerated++
					c.metrics.mu.Unlock()

					c.EventChan <- models.NewLecturaExitosa(
						skuFinal, especie, calibre, variedad, embalaje, result.correlativo, message, c.dispositivo,
					)
				}
			}()
		default:
			log.Printf("⚠️  Buffer de inserciones lleno, procesando síncronamente (ID-DB)")
			ctx := context.Background()
			correlativo, err := c.dbManager.InsertNewBox(
				ctx,
				especie,
				variedad,
				calibre,
				embalaje,
				dark, // Usar el dark extraído del ID
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
		logTs("📊 DataMatrix detectado: %s", strings.TrimSpace(message))
		message = strings.TrimSpace(message)

		// Crear y enviar evento DataMatrix a un canal dedicado
		// Este flujo es SEPARADO del flujo QR/SKU original
		if message == models.NO_READ_CODE {
			message = ""
		}
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
		log.Printf("❌ Método de escaneo desconocido: %s", c.ScanMethod)
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
func (c *CognexListener) Stop() {
	log.Printf("Deteniendo CognexListener %d...", c.ID)
	c.cancel()

	if c.listener != nil {
		c.listener.Close()
	}
	// No cerramos los canales aquí para evitar panics si todavía hay goroutines escribiendo.
	// El contexto cancelado debería ser suficiente para detener todo.
	log.Printf("CognexListener %d detenido.", c.ID)
}
