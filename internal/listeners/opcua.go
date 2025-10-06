package listeners

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"API-GREENEX/internal/models"

	"github.com/gopcua/opcua"
	"github.com/gopcua/opcua/ua"
	"github.com/joho/godotenv"
)

// OPCUAWriter define una interfaz para encolar escrituras, permitiendo el desacoplamiento.
type OPCUAWriter interface {
	QueueWriteRequest(nodeID string, value interface{})
}

// Interfaces para los managers (para evitar dependencias circulares)
type SubscriptionManagerInterface interface {
	OnSubscriptionData(subscriptionName, nodeID string, data *NodeData)
	SetOPCUAWriter(writer OPCUAWriter)
	Start()
}

type MethodManagerInterface interface {
	OnMethodRead(nodeID string, result *NodeData, err error, duration time.Duration)
	OnMethodWrite(nodeID string, value interface{}, err error, duration time.Duration)
	OnMethodCall(nodeID string, inputArgs interface{}, result interface{}, err error, duration time.Duration)
	Start()
}

// Usar directamente tus estructuras existentes
type OPCUAService struct {
	client           *opcua.Client
	config           *OPCUAConfig
	subscriptions    map[string]*opcua.Subscription
	isConnected      bool
	ctx              context.Context
	cancel           context.CancelFunc
	dataCallbacks    map[string]func(interface{})
	isRunning        bool
	writeRequestChan chan WriteRequest // Canal para solicitudes de escritura

	// Managers para manejar los datos
	subscriptionManager SubscriptionManagerInterface
	methodManager       MethodManagerInterface
}

// Estructura para una solicitud de escritura
type WriteRequest struct {
	NodeID string
	Value  interface{}
}

type OPCUAConfig struct {
	Endpoint             string
	Username             string
	Password             string
	SecurityPolicy       string
	SecurityMode         string
	CertificatePath      string
	PrivateKeyPath       string
	ConnectionTimeout    time.Duration
	SessionTimeout       time.Duration
	SubscriptionInterval time.Duration
	KeepAliveCount       uint32
	LifetimeCount        uint32
	MaxNotifications     uint32
}

type NodeData struct {
	NodeID    string
	Value     interface{}
	Timestamp time.Time
	Quality   ua.StatusCode
}

// ReadNodeDataType lee el tipo de dato (DataType) de un nodo OPC UA
func (s *OPCUAService) ReadNodeDataType(nodeID string) (interface{}, error) {
	if !s.isConnected {
		return nil, fmt.Errorf("cliente no está conectado")
	}
	id, err := ua.ParseNodeID(nodeID)
	if err != nil {
		return nil, fmt.Errorf("error parseando NodeID %s: %v", nodeID, err)
	}
	req := &ua.ReadRequest{
		NodesToRead: []*ua.ReadValueID{
			{
				NodeID:      id,
				AttributeID: ua.AttributeIDDataType, // 25: DataType
			},
		},
	}
	resp, err := s.client.Read(s.ctx, req)
	if err != nil {
		return nil, fmt.Errorf("error leyendo DataType de %s: %v", nodeID, err)
	}
	if len(resp.Results) == 0 {
		return nil, fmt.Errorf("respuesta vacía al leer DataType de %s", nodeID)
	}
	return resp.Results[0].Value, nil
}

// NewOPCUAService crea una nueva instancia del servicio
func NewOPCUAService() *OPCUAService {
	// Cargar variables de entorno
	if err := godotenv.Load(); err != nil {
		log.Println("Advertencia: No se pudo cargar el archivo .env:", err)
	}

	config, err := loadOPCUAConfig()
	if err != nil {
		log.Printf("Error al cargar configuración OPC UA: %v", err)
		return nil
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &OPCUAService{
		config:           config,
		subscriptions:    make(map[string]*opcua.Subscription),
		dataCallbacks:    make(map[string]func(interface{})),
		ctx:              ctx,
		cancel:           cancel,
		isRunning:        false,
		writeRequestChan: make(chan WriteRequest, 50), // Inicializar el canal
	}
}

// SetSubscriptionManager asigna el manager de suscripciones
func (s *OPCUAService) SetSubscriptionManager(manager SubscriptionManagerInterface) {
	s.subscriptionManager = manager
	log.Println("Subscription Manager vinculado al servicio OPC UA")
}

// SetMethodManager asigna el manager de métodos
func (s *OPCUAService) SetMethodManager(manager MethodManagerInterface) {
	s.methodManager = manager
	log.Println("Method Manager vinculado al servicio OPC UA")
}

// loadOPCUAConfig carga la configuración desde variables de entorno
func loadOPCUAConfig() (*OPCUAConfig, error) {
	config := &OPCUAConfig{
		Endpoint:        getEnv("OPCUA_ENDPOINT", "opc.tcp://localhost:4840"),
		Username:        getEnv("OPCUA_USERNAME", ""),
		Password:        getEnv("OPCUA_PASSWORD", ""),
		SecurityPolicy:  getEnv("OPCUA_SECURITY_POLICY", "Basic256Sha256"), // Cambiado de "None"
		SecurityMode:    getEnv("OPCUA_SECURITY_MODE", "SignAndEncrypt"),   // Cambiado de "None"
		CertificatePath: getEnv("OPCUA_CERTIFICATE_PATH", ""),
		PrivateKeyPath:  getEnv("OPCUA_PRIVATE_KEY_PATH", ""),
	} // Usar constantes para valores por defecto
	var err error
	defaultTimeout := fmt.Sprintf("%ds", models.OPCUA_TIMEOUT)
	config.ConnectionTimeout, err = time.ParseDuration(getEnv("OPCUA_CONNECTION_TIMEOUT", defaultTimeout))
	if err != nil {
		return nil, fmt.Errorf("error parseando connection_timeout: %v", err)
	}

	config.SessionTimeout, err = time.ParseDuration(getEnv("OPCUA_SESSION_TIMEOUT", "60s"))
	if err != nil {
		return nil, fmt.Errorf("error parseando session_timeout: %v", err)
	}

	config.SubscriptionInterval, err = time.ParseDuration(getEnv("OPCUA_SUBSCRIPTION_INTERVAL", models.DEFAULT_SUBSCRIPTION_INTERVAL))
	if err != nil {
		return nil, fmt.Errorf("error parseando subscription_interval: %v", err)
	}

	// Parsear valores uint32
	keepAlive, err := strconv.ParseUint(getEnv("OPCUA_KEEPALIVE_COUNT", "3"), 10, 32)
	if err != nil {
		return nil, fmt.Errorf("error parseando keepalive_count: %v", err)
	}
	config.KeepAliveCount = uint32(keepAlive)

	lifetime, err := strconv.ParseUint(getEnv("OPCUA_LIFETIME_COUNT", "300"), 10, 32)
	if err != nil {
		return nil, fmt.Errorf("error parseando lifetime_count: %v", err)
	}
	config.LifetimeCount = uint32(lifetime)

	maxNotif, err := strconv.ParseUint(getEnv("OPCUA_MAX_NOTIFICATIONS", "1000"), 10, 32)
	if err != nil {
		return nil, fmt.Errorf("error parseando max_notifications: %v", err)
	}
	config.MaxNotifications = uint32(maxNotif)

	return config, nil
}

// getSecurityModeFromString convierte un string a su constante ua.MessageSecurityMode
func getSecurityModeFromString(mode string) ua.MessageSecurityMode {
	switch mode {
	case "None":
		return ua.MessageSecurityModeNone
	case "Sign":
		return ua.MessageSecurityModeSign
	case "SignAndEncrypt":
		return ua.MessageSecurityModeSignAndEncrypt
	default:
		return ua.MessageSecurityModeInvalid
	}
}

// Start inicia el servicio OPC UA como listener
func (s *OPCUAService) Start() {
	log.Println("Iniciando servicio OPC UA...")
	s.isRunning = true

	// Intentar conectar
	var suscripcionOk bool = false
	if err := s.Connect(); err != nil {
		log.Printf("No se pudo conectar a OPC UA: %v", err)
		log.Println("Servicio OPC UA funcionará en modo desconectado")
	} else {
		log.Printf("OPC UA conectado exitosamente: %s", s.config.Endpoint)
		errSub := s.setupWagoTestSubscriptions() // VECTORES por suscripción
		if errSub != nil {
			log.Printf("Error al suscribirse a los vectores de WAGO: %v", errSub)
			suscripcionOk = false
		} else {
			suscripcionOk = true
		}
		log.Println("WAGO: Vectores configurados por SUSCRIPCIÓN - Solo modo LECTURA por ahora")
	}

	// Iniciar el procesador de escrituras en background
	go s.processWriteRequests()

	// Si la suscripción falla, activar bucle de lectura manual cada segundo
	go func() {
		if !suscripcionOk {
			// Modo simple desactivado - WagoLoop maneja escrituras
		}
	}()

	// Listener infinito - mantiene el servicio corriendo
	for s.isRunning {
		time.Sleep(5 * time.Second)
		// Auto-reconexión si se pierde la conexión
		if !s.isConnected {
			log.Println("Intentando reconectar OPC UA...")
			if err := s.Connect(); err == nil {
				log.Println("OPC UA reconectado exitosamente")
				s.setupWagoTestSubscriptions() // VECTORES por suscripción
			}
		}
	}
}

// processWriteRequests procesa las solicitudes de escritura del canal
func (s *OPCUAService) processWriteRequests() {
	log.Println("Procesador de escrituras OPC UA iniciado.")
	for req := range s.writeRequestChan {
		if !s.isConnected {
			log.Printf("Descartando escritura en %s: cliente no conectado.", req.NodeID)
			continue
		}
		log.Printf("Procesando escritura en %s con valor %v", req.NodeID, req.Value)
		if err := s.WriteNode(req.NodeID, req.Value); err != nil {
			log.Printf("Error al procesar escritura desde canal: %v", err)
		}
	}
	log.Println("Procesador de escrituras OPC UA detenido.")
}

// QueueWriteRequest encola una solicitud de escritura para ser procesada
func (s *OPCUAService) QueueWriteRequest(nodeID string, value interface{}) {
	if !s.isRunning {
		log.Printf("Servicio no está corriendo, descartando escritura en %s", nodeID)
		return
	}
	select {
	case s.writeRequestChan <- WriteRequest{NodeID: nodeID, Value: value}:
		// Solicitud encolada
	default:
		log.Printf("Canal de escritura lleno. Descartando escritura en %s", nodeID)
	}
}

// Connect establece la conexión con el servidor OPC UA
func (s *OPCUAService) Connect() error {
	if s.isConnected {
		return nil // Ya está conectado, no es un error.
	}

	// 1. Obtener los endpoints disponibles del servidor.
	endpoints, err := opcua.GetEndpoints(s.ctx, s.config.Endpoint)
	if err != nil {
		return fmt.Errorf("error obteniendo endpoints: %w", err)
	}

	log.Println("Endpoints disponibles en el servidor:")
	for _, ep := range endpoints {
		log.Printf("  - URL: %s, Policy: %s, Mode: %s", ep.EndpointURL, ep.SecurityPolicyURI, ep.SecurityMode)
	}

	// 2. Seleccionar el mejor endpoint disponible.
	// Intentar con la política configurada primero.
	ep, err := opcua.SelectEndpoint(endpoints, s.config.SecurityPolicy, getSecurityModeFromString(s.config.SecurityMode))
	if err != nil {
		log.Printf("No se pudo seleccionar el endpoint configurado (%s, %s): %v. Intentando con política 'None'...", s.config.SecurityPolicy, s.config.SecurityMode, err)
		// Si falla, intentar con None para diagnóstico.
		ep, err = opcua.SelectEndpoint(endpoints, "None", ua.MessageSecurityModeNone)
		if err != nil {
			return fmt.Errorf("no se pudo encontrar un endpoint compatible con las políticas intentadas (configurada o 'None'): %w", err)
		}
	}

	log.Printf("Endpoint seleccionado para la conexión: %s (Policy: %s, Mode: %s)", ep.EndpointURL, ep.SecurityPolicyURI, ep.SecurityMode)

	// 3. Configurar las opciones del cliente usando SOLO las mínimas (como Python)
	opts := []opcua.Option{}

	// Solo añadir el certificado si la política de seguridad no es 'None'
	if ep.SecurityPolicyURI != "http://opcfoundation.org/UA/SecurityPolicy#None" {
		opts = append(opts, opcua.Certificate(ep.ServerCertificate))
		log.Println("Certificado del servidor añadido para política de seguridad con encriptación.")
	} else {
		log.Println("Sin certificado del servidor (política de seguridad None).")
	}

	log.Println("Usando configuración mínima (sin opciones adicionales, como Python).")

	// 4. Crear el cliente con las opciones.
	c, err := opcua.NewClient(s.config.Endpoint, opts...)
	if err != nil {
		return fmt.Errorf("error creando el cliente OPC UA: %w", err)
	}
	s.client = c

	// 5. Conectar al servidor.
	if err := s.client.Connect(s.ctx); err != nil {
		s.isConnected = false
		return fmt.Errorf("error conectando al servidor OPC UA: %w", err)
	}

	s.isConnected = true
	return nil
}

// Disconnect cierra la conexión con el servidor OPC UA
func (s *OPCUAService) Disconnect() error {
	if !s.isConnected {
		return nil
	}

	// Cancelar contexto para detener suscripciones
	s.cancel()

	// Cerrar todas las suscripciones
	for name, sub := range s.subscriptions {
		if err := sub.Cancel(s.ctx); err != nil {
			log.Printf("Error cancelando suscripción %s: %v", name, err)
		}
	}

	// Cerrar cliente
	if err := s.client.Close(s.ctx); err != nil {
		log.Printf("Error cerrando cliente OPC UA: %v", err)
		return err
	}

	s.isConnected = false
	s.subscriptions = make(map[string]*opcua.Subscription)
	log.Println("Desconectado del servidor OPC UA")

	return nil
}

// ReadNode lee el valor de un nodo específico - INTEGRADO CON METHOD MANAGER
func (s *OPCUAService) ReadNode(nodeID string) (*NodeData, error) {
	startTime := time.Now()

	if !s.isConnected {
		err := fmt.Errorf("cliente no está conectado")
		// Notificar al method manager sobre el error
		if s.methodManager != nil {
			s.methodManager.OnMethodRead(nodeID, nil, err, time.Since(startTime))
		}
		return nil, err
	}

	id, err := ua.ParseNodeID(nodeID)
	if err != nil {
		err = fmt.Errorf("error parseando NodeID %s: %v", nodeID, err)
		if s.methodManager != nil {
			s.methodManager.OnMethodRead(nodeID, nil, err, time.Since(startTime))
		}
		return nil, err
	}

	req := &ua.ReadRequest{
		MaxAge: 2000,
		NodesToRead: []*ua.ReadValueID{
			{NodeID: id},
		},
	}

	resp, err := s.client.Read(s.ctx, req)
	if err != nil {
		err = fmt.Errorf("error leyendo nodo %s: %v", nodeID, err)
		if s.methodManager != nil {
			s.methodManager.OnMethodRead(nodeID, nil, err, time.Since(startTime))
		}
		return nil, err
	}

	if len(resp.Results) == 0 {
		err = fmt.Errorf("no se recibieron resultados para el nodo %s", nodeID)
		if s.methodManager != nil {
			s.methodManager.OnMethodRead(nodeID, nil, err, time.Since(startTime))
		}
		return nil, err
	}

	result := resp.Results[0]
	if result.Status != ua.StatusOK {
		err = fmt.Errorf("error en lectura del nodo %s: %v", nodeID, result.Status)
		if s.methodManager != nil {
			s.methodManager.OnMethodRead(nodeID, nil, err, time.Since(startTime))
		}
		return nil, err
	}

	nodeData := &NodeData{
		NodeID:    nodeID,
		Value:     result.Value.Value(),
		Timestamp: result.ServerTimestamp,
		Quality:   result.Status,
	}

	// Notificar al method manager sobre el éxito
	if s.methodManager != nil {
		s.methodManager.OnMethodRead(nodeID, nodeData, nil, time.Since(startTime))
	}

	return nodeData, nil
}

// WriteNode escribe un valor a un nodo específico - INTEGRADO CON METHOD MANAGER
func (s *OPCUAService) WriteNode(nodeID string, value interface{}) error {
	startTime := time.Now()

	if !s.isConnected {
		err := fmt.Errorf("cliente no está conectado")
		log.Printf("❌ WRITE ERROR | Nodo: %s | Valor: %v (%T) | ERROR: %v", nodeID, value, value, err)
		if s.methodManager != nil {
			s.methodManager.OnMethodWrite(nodeID, value, err, time.Since(startTime))
		}
		return err
	}

	id, err := ua.ParseNodeID(nodeID)
	if err != nil {
		err = fmt.Errorf("error parseando NodeID %s: %v", nodeID, err)
		log.Printf("❌ WRITE ERROR | Nodo: %s | Valor: %v (%T) | ERROR: %v", nodeID, value, value, err)
		if s.methodManager != nil {
			s.methodManager.OnMethodWrite(nodeID, value, err, time.Since(startTime))
		}
		return err
	}

	// Leer el DataType esperado del nodo para diagnóstico
	dataType, errDataType := s.ReadNodeDataType(nodeID)
	var dataTypeInfo string
	if errDataType == nil {
		if dtNodeID, ok := dataType.(*ua.NodeID); ok {
			dataTypeInfo = fmt.Sprintf("ns=%d;i=%d", dtNodeID.Namespace(), dtNodeID.IntID())
		} else {
			dataTypeInfo = fmt.Sprintf("%v", dataType)
		}
	} else {
		dataTypeInfo = "Desconocido"
	}

	variant, err := ua.NewVariant(value)
	if err != nil {
		err = fmt.Errorf("error creando variant para valor %v: %v", value, err)
		log.Printf("❌ WRITE ERROR | Nodo: %s | Valor: %v (%T) | DataType esperado: %s | ERROR: %v",
			nodeID, value, value, dataTypeInfo, err)
		if s.methodManager != nil {
			s.methodManager.OnMethodWrite(nodeID, value, err, time.Since(startTime))
		}
		return err
	}

	req := &ua.WriteRequest{
		NodesToWrite: []*ua.WriteValue{
			{
				NodeID:      id,
				AttributeID: ua.AttributeIDValue,
				Value: &ua.DataValue{
					Value: variant,
				},
			},
		},
	}

	resp, err := s.client.Write(s.ctx, req)
	duration := time.Since(startTime)

	if err != nil {
		err = fmt.Errorf("error escribiendo nodo %s: %v", nodeID, err)
		log.Printf("❌ WRITE FAILED | Nodo: %s | Valor: %v (%T) | DataType: %s | Duración: %v | ERROR: %v",
			nodeID, value, value, dataTypeInfo, duration, err)
		if s.methodManager != nil {
			s.methodManager.OnMethodWrite(nodeID, value, err, duration)
		}
		return err
	}

	if len(resp.Results) > 0 && resp.Results[0] != ua.StatusOK {
		statusCode := resp.Results[0]
		err = fmt.Errorf("error en escritura del nodo %s: %v", nodeID, statusCode)

		log.Printf("┌─────────────────────────────────────────────────────────")
		log.Printf("│ ❌ WRITE STATUS ERROR")
		log.Printf("│ Nodo: %s", nodeID)
		log.Printf("│ Valor enviado: %v", value)
		log.Printf("│ Tipo Go: %T", value)
		log.Printf("│ DataType esperado: %s", dataTypeInfo)
		log.Printf("│ Variant Type: %v", variant.Type())
		log.Printf("│ Status Code: %v", statusCode)
		log.Printf("│ Duración: %v", duration)
		log.Printf("└─────────────────────────────────────────────────────────")

		if s.methodManager != nil {
			s.methodManager.OnMethodWrite(nodeID, value, err, duration)
		}
		return err
	}

	// Notificar al method manager sobre el éxito
	if s.methodManager != nil {
		s.methodManager.OnMethodWrite(nodeID, value, nil, duration)
	}

	return nil
}

// SubscribeToNodes crea una suscripción para monitorear cambios en nodos - INTEGRADO CON SUBSCRIPTION MANAGER
func (s *OPCUAService) SubscribeToNodes(subscriptionName string, nodeIDs []string, callback func(string, *NodeData)) error {
	if !s.isConnected {
		return fmt.Errorf("cliente no está conectado")
	}

	// Crear channel para notificaciones
	notifChan := make(chan *opcua.PublishNotificationData, 100)

	// Crear suscripción con parámetros MÍNIMOS (máxima compatibilidad WAGO)
	sub, err := s.client.Subscribe(s.ctx, nil, notifChan)

	if err != nil {
		return fmt.Errorf("error creando suscripción %s: %v", subscriptionName, err)
	}

	// Crear elementos monitoreados
	var items []*ua.MonitoredItemCreateRequest
	for _, nodeID := range nodeIDs {
		id, err := ua.ParseNodeID(nodeID)
		if err != nil {
			log.Printf("Error parseando NodeID %s: %v", nodeID, err)
			continue
		}

		items = append(items, &ua.MonitoredItemCreateRequest{
			ItemToMonitor: &ua.ReadValueID{
				NodeID:      id,
				AttributeID: ua.AttributeIDValue,
			},
			MonitoringMode:      ua.MonitoringModeReporting,
			RequestedParameters: nil, // Usar parámetros por defecto del servidor
		})
	}

	_, err = sub.Monitor(s.ctx, ua.TimestampsToReturnBoth, items...)
	if err != nil {
		return fmt.Errorf("error creando elementos monitoreados para suscripción %s: %v", subscriptionName, err)
	}

	// Manejar notificaciones en una goroutine separada
	go func() {
		defer close(notifChan)
		for s.isRunning {
			select {
			case msg := <-notifChan:
				if msg == nil {
					return
				}
				if msg.Error != nil {
					log.Printf("Error en suscripción %s: %v", subscriptionName, msg.Error)
					continue
				}

				// Procesar notificaciones de cambio de datos
				switch notification := msg.Value.(type) {
				case *ua.DataChangeNotification:
					for _, item := range notification.MonitoredItems {
						nodeID := findNodeIDByClientHandle(nodeIDs, item.ClientHandle)
						if nodeID == "" {
							continue
						}

						nodeData := &NodeData{
							NodeID:    nodeID,
							Value:     item.Value.Value.Value(),
							Timestamp: item.Value.ServerTimestamp,
							Quality:   item.Value.Status,
						}

						// PRIMERO: Enviar al subscription manager
						if s.subscriptionManager != nil {
							s.subscriptionManager.OnSubscriptionData(subscriptionName, nodeID, nodeData)
						}

						// SEGUNDO: Ejecutar callback personalizado si existe
						if callback != nil {
							callback(nodeID, nodeData)
						}
					}
				}
			case <-s.ctx.Done():
				return
			}
		}
	}()

	s.subscriptions[subscriptionName] = sub
	log.Printf("Suscripción %s creada exitosamente para %d nodos", subscriptionName, len(nodeIDs))

	return nil
}

// UnsubscribeFromNodes cancela una suscripción específica
func (s *OPCUAService) UnsubscribeFromNodes(subscriptionName string) error {
	sub, exists := s.subscriptions[subscriptionName]
	if !exists {
		return fmt.Errorf("suscripción %s no existe", subscriptionName)
	}

	if err := sub.Cancel(s.ctx); err != nil {
		return fmt.Errorf("error cancelando suscripción %s: %v", subscriptionName, err)
	}

	delete(s.subscriptions, subscriptionName)
	log.Printf("Suscripción %s cancelada exitosamente", subscriptionName)

	return nil
}

// setupWagoTestSubscriptions se suscribe a los nodos de prueba de WAGO
func (s *OPCUAService) setupWagoTestSubscriptions() error {
	if !s.isConnected {
		return fmt.Errorf("cliente no está conectado")
	}

	wagoVectorNodes := []string{
		models.WAGO_VectorBool,
		models.WAGO_VectorInt,
		models.WAGO_VectorWord,
	}

	err := s.SubscribeToNodes("wago_vector_subscription", wagoVectorNodes, nil)
	if err != nil {
		log.Printf("Error al suscribirse a los vectores de WAGO: %v", err)
		return err
	} else {
		log.Printf("Suscripción a vectores WAGO configurada para %d nodos.", len(wagoVectorNodes))
		return nil
	}
}

// setupDefaultSubscriptions configurado con las constantes correctas
func (s *OPCUAService) setupDefaultSubscriptions() {
	if !s.isConnected {
		return
	}

	// Usar las constantes definidas en lugar de valores hardcoded
	nodeIDs := []string{
		models.BuildNodeID(models.OPCUA_SEGREGATION_METHOD), // ns=4;i=2
		// models.BuildNodeID(models.OPCUA_HEARTBEAT_METHOD),   // ns=4;i=3 (descomentado si se activa)
	}

	// Crear suscripción usando las constantes
	err := s.SubscribeToNodes(models.DEFAULT_SUBSCRIPTION, nodeIDs, nil)

	if err != nil {
		log.Printf("Error en suscripción por defecto: %v", err)
	} else {
		log.Printf("Suscripción por defecto configurada para %d nodos (namespace %d)",
			len(nodeIDs), models.OPCUA_NAMESPACE)
	}
}

// Stop detiene el servicio
func (s *OPCUAService) Stop() {
	s.isRunning = false
	s.Disconnect()
	log.Println("Servicio OPC UA detenido")
}

// IsConnected retorna el estado de conexión
func (s *OPCUAService) IsConnected() bool {
	return s.isConnected
}

// GetEndpoint retorna el endpoint configurado
func (s *OPCUAService) GetEndpoint() string {
	if s.config == nil {
		return "No configurado"
	}
	return s.config.Endpoint
}

// Helper functions
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func findNodeIDByClientHandle(nodeIDs []string, handle uint32) string {
	if int(handle) < len(nodeIDs) {
		return nodeIDs[handle]
	}
	return ""
}

// startWagoSimulator escribe valores constantemente a las variables WAGO con gran frecuencia
func (s *OPCUAService) startWagoSimulator() {
	log.Println("🚀 Iniciando simulador WAGO - Escritura constante con gran frecuencia...")

	contador := 0
	boleano := false

	for s.isRunning && s.isConnected {
		contador++

		// Alternar booleano cada ciclo
		boleano = !boleano

		// Escribir a todas las variables WAGO constantemente
		variables := map[string]interface{}{
			"ns=4;s=|var|WAGO TEST.Application.DB_OPC.BoleanoTest": boleano,
			"ns=4;s=|var|WAGO TEST.Application.DB_OPC.ByteTest":    uint8(contador % 256),
			"ns=4;s=|var|WAGO TEST.Application.DB_OPC.EnteroTest":  int16(contador % 32767),
			"ns=4;s=|var|WAGO TEST.Application.DB_OPC.RealTest":    float32(contador) * 3.14159,
			"ns=4;s=|var|WAGO TEST.Application.DB_OPC.StringTest":  fmt.Sprintf("Ciclo_%d", contador),
			"ns=4;s=|var|WAGO TEST.Application.DB_OPC.WordTest":    uint16(contador % 65536),
		}

		for nodeID, valor := range variables {
			if err := s.WriteNode(nodeID, valor); err != nil {
				log.Printf("🔥 Error simulador en %s: %v", nodeID, err)
			} else {
				log.Printf("✍️  Simulador: %s = %v", nodeID, valor)
			}
		}

		// Frecuencia alta: cada 100ms
		time.Sleep(100 * time.Millisecond)

		// Log cada 50 ciclos para no spam
		if contador%50 == 0 {
			log.Printf("🔄 Simulador WAGO - Ciclo %d completado", contador)
		}
	}

	log.Println("🛑 Simulador WAGO detenido")
}
