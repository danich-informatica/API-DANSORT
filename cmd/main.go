package main

import (
	"context"
	"fmt"
	"log"
	"math/rand/v2"
	"os"
	"time"

	"API-DANSORT/internal/communication/pallet"
	"API-DANSORT/internal/communication/plc"
	"API-DANSORT/internal/config"
	"API-DANSORT/internal/db"
	"API-DANSORT/internal/flow"
	"API-DANSORT/internal/listeners"
	"API-DANSORT/internal/models"
	"API-DANSORT/internal/monitoring"
	"API-DANSORT/internal/shared"
	"API-DANSORT/internal/sorter"

	"github.com/joho/godotenv"
)

func main() {
	// Configurar logger sin timestamps para el banner
	log.SetOutput(os.Stdout)
	log.SetFlags(0)

	// Tu hermoso banner ASCII
	log.Println("")
	log.Println(`
		░█████╗░██████╗░██╗░░░░░░██████╗░░█████╗░███╗░░██╗░██████╗░█████╗░██████╗░████████╗
		██╔══██╗██╔══██╗██║░░░░░░██╔══██╗██╔══██╗████╗░██║██╔════╝██╔══██╗██╔══██╗╚══██╔══╝
		███████║██████╔╝██║█████╗██║░░██║███████║██╔██╗██║╚█████╗░██║░░██║██████╔╝░░░██║░░░
		██╔══██║██╔═══╝░██║╚════╝██║░░██║██╔══██║██║╚████║░╚═══██╗██║░░██║██╔══██╗░░░██║░░░
		██║░░██║██║░░░░░██║░░░░░░██████╔╝██║░░██║██║░╚███║██████╔╝╚█████╔╝██║░░██║░░░██║░░░
		╚═╝░░╚═╝╚═╝░░░░░╚═╝░░░░░░╚═════╝░╚═╝░░╚═╝╚═╝░░╚══╝╚═════╝░░╚════╝░╚═╝░░╚═╝░░░╚═╝░░░
	`)
	log.Println("")
	log.Println("Iniciando API-Greenex...")
	log.Println("")

	// Ahora activar fecha/hora para los logs normales
	log.SetFlags(log.Ldate | log.Ltime)

	// Inicializar gestor de canales compartidos (Singleton)
	channelMgr := shared.GetChannelManager()
	defer channelMgr.CloseAll()
	log.Println("✅ Gestor de canales inicializado")
	log.Println("")

	// Cargar archivo .env para obtener ruta del config
	if err := godotenv.Load(); err != nil {
		log.Println("⚠️  Archivo .env no encontrado, usando valores por defecto")
	}

	// Cargar configuración desde YAML
	configPath := os.Getenv("CONFIG_FILE")
	if configPath == "" {
		configPath = "config/config.yaml"
	}

	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		log.Fatalf("❌ Error al cargar configuración: %v", err)
	}
	log.Printf("✅ Configuración cargada desde: %s", configPath)

	// Inicializar la conexión a PostgreSQL usando config YAML
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	connectTimeout, _ := cfg.Database.Postgres.GetConnectTimeoutDuration()
	healthCheckInterval, _ := cfg.Database.Postgres.GetHealthcheckIntervalDuration()

	dbManager, err := db.GetPostgresManagerWithURL(
		ctx,
		cfg.Database.Postgres.URL,
		int32(cfg.Database.Postgres.MinConns),
		int32(cfg.Database.Postgres.MaxConns),
		connectTimeout,
		healthCheckInterval,
	)
	if err != nil {
		log.Fatalf("❌ Error al inicializar PostgreSQL: %v", err)
	}
	defer dbManager.Close()
	log.Println("✅ Base de datos PostgreSQL inicializada correctamente")

	// Inicializar FX6Manager para lecturas DataMatrix
	log.Println("")
	log.Println("📊 Inicializando conexión a SQL Server FX6...")
	fx6Manager, err := db.GetFX6Manager(ctx, cfg)
	if err != nil {
		log.Printf("⚠️  Error al inicializar FX6Manager: %v (continuando sin soporte DataMatrix)", err)
		fx6Manager = nil
	} else {
		defer fx6Manager.Close()
		log.Println("✅ FX6Manager inicializado correctamente (tabla PKG_Pallets_Externos)")
	}
	log.Println("")

	log.Println("📊 Inicializando conexión a SQL Server FX_Sync...")
	fxSyncManager, err := db.GetFXSyncManager(ctx, cfg)
	if err != nil {
		log.Printf("⚠️  Error al inicializar FXSyncManager: %v (continuando sin soporte orden de fabricación)", err)
		fxSyncManager = nil
	} else {
		defer fxSyncManager.Close()
		log.Println("✅ FXSyncManager inicializado correctamente (vista V_Danish)")
	}
	log.Println("")

	// Inicializar SKUManager para gestión eficiente con streaming
	skuManager, err := flow.NewSKUManager(ctx, dbManager)
	if err != nil {
		log.Printf("⚠️  Error al inicializar SKUManager: %v (continuando sin caché de SKUs)", err)
		skuManager = nil
	}

	// Crear el servidor HTTP con endpoints (antes de crear sorters)
	httpHost := cfg.HTTP.Host
	if httpHost == "" {
		httpHost = "0.0.0.0" // Default si no está configurado
	}
	httpPort := cfg.HTTP.Port
	httpAddr := fmt.Sprintf("%s:%d", httpHost, httpPort)

	httpService := listeners.NewHTTPFrontend(httpAddr)
	httpService.SetPostgresManager(dbManager)

	// Vincular SKUManager si está disponible para endpoints de streaming
	if skuManager != nil {
		httpService.SetSKUManager(skuManager)
	}

	log.Printf("✅ Servidor HTTP creado en %s", httpAddr)
	log.Println("")

	// Crear Device Monitor para monitoreo de dispositivos con heartbeat
	deviceMonitor := monitoring.NewDeviceMonitor(5*time.Second, 3*time.Second)
	httpService.SetDeviceMonitor(deviceMonitor)
	log.Println("📡 Device Monitor creado (heartbeat: 5s, timeout: 3s)")
	log.Println("")

	// Crear listeners de Cognex (NO iniciarlos aún, los sorters lo harán)
	log.Println("")
	log.Printf("📷 Configurando %d dispositivo(s) Cognex...", len(cfg.CognexDevices))
	var cognexListeners []*listeners.CognexListener
	ssmsManager, _ := db.GetManagerWithConfig(ctx, cfg.Database.SQLServer)

	// Inicializar BoxCacheManager para optimizar lecturas ID-DB
	var boxCacheManager *flow.BoxCacheManager
	if cfg.BoxCache.Enabled {
		log.Println("")
		log.Println("📦 Inicializando BoxCacheManager (optimización ID-DB)...")
		cacheSize := cfg.BoxCache.Size
		if cacheSize <= 0 {
			cacheSize = 500 // Default
		}
		refreshInterval := cfg.BoxCache.GetRefreshInterval()

		boxCacheManager = flow.NewBoxCacheManager(ctx, ssmsManager, cacheSize, refreshInterval)
		boxCacheManager.Start()
		defer boxCacheManager.Stop()
		log.Println("✅ BoxCacheManager iniciado correctamente")
		log.Println("")
	} else {
		log.Println("⚠️  BoxCache desactivado en configuración (se usarán queries directas)")
		log.Println("")
	}

	for _, cognexCfg := range cfg.CognexDevices {
		log.Println("  ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		log.Printf("  📌 Cognex #%d: %s", cognexCfg.ID, cognexCfg.Name)
		log.Printf("     Ubicación: %s", cognexCfg.Ubicacion)
		log.Printf("     Host: %s:%d", cognexCfg.Host, cognexCfg.Port)
		log.Printf("     Método: %s", cognexCfg.ScanMethod)

		cognexListener := listeners.NewCognexListener(
			cognexCfg.ID,
			cognexCfg.Host,
			cognexCfg.Port,
			cognexCfg.ScanMethod,
			dbManager,
			ssmsManager,
			boxCacheManager, // NUEVO: Pasar caché de cajas
		)

		cognexListeners = append(cognexListeners, cognexListener)
		log.Printf("     [OK] CognexListener created (will be started by sorter)")
	}
	log.Println("  ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	log.Println("")

	// Crear y conectar PLC Manager
	log.Println("[Init] Initializing PLC Manager...")
	plcManager := plc.NewManager(cfg)

	plcCtx, plcCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer plcCancel()

	if err := plcManager.ConnectAll(plcCtx); err != nil {
		log.Printf("[Init] Initial PLC connection error: %v", err)
		log.Println("[Init] PLC Manager will continue attempting automatic reconnection")
		// NO ponemos plcManager = nil, dejamos que intente reconectar
	} else {
		log.Println("[Init] PLC Manager connected successfully")
	}

	// Siempre registrar defer para cerrar conexiones al salir
	if plcManager != nil {
		defer plcManager.CloseAll(context.Background())
	}
	log.Println("")

	// Inicializar Sorters (si hay configurados)
	var sorters []*sorter.Sorter
	if len(cfg.Sorters) > 0 {
		log.Printf("🔀 Inicializando %d Sorter(s)...", len(cfg.Sorters))
		log.Println("")

		for _, sorterCfg := range cfg.Sorters {
			log.Println("  ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
			log.Printf("  📦 Sorter #%d: %s", sorterCfg.ID, sorterCfg.Name)
			log.Printf("     PLC Endpoint: %s", sorterCfg.PLCEndpoint)

			// Insertar sorter en la base de datos si no existe
			if err := dbManager.InsertSorterIfNotExists(ctx, sorterCfg.ID, sorterCfg.Name); err != nil {
				log.Printf("     ⚠️  Error al insertar sorter en DB: %v", err)
			} else {
				log.Printf("     ✅ Sorter #%d insertado en DB (o ya existía)", sorterCfg.ID)
			}

			// Buscar el CognexListener correspondiente para QR/SKU (asumiendo 1 Cognex principal por sorter)
			var cognexListener *listeners.CognexListener
			if sorterCfg.ID > 0 && sorterCfg.ID <= len(cognexListeners) {
				cognexListener = cognexListeners[sorterCfg.ID-1]
			}

			if cognexListener == nil && len(cognexListeners) > 0 {
				log.Printf("     ⚠️  Usando primer Cognex disponible para Sorter #%d", sorterCfg.ID)
				cognexListener = cognexListeners[0]
			}

			// Crear mapa de cámaras DataMatrix para este sorter (basado en cognex_id en salidas)
			cognexDevices := make(map[int]*listeners.CognexListener)
			ssms_manager, _ := db.GetManagerWithConfig(ctx, cfg.Database.SQLServer)
			for _, salidaCfg := range sorterCfg.Salidas {
				if salidaCfg.CognexID > 0 {
					// Esta salida tiene una cámara DataMatrix asignada
					// Buscar la configuración del Cognex correspondiente
					for _, cognexCfg := range cfg.CognexDevices {
						if cognexCfg.ID == salidaCfg.CognexID && cognexCfg.ScanMethod == "DATAMATRIX" {
							// Crear listener solo si no existe ya en el mapa
							if _, exists := cognexDevices[cognexCfg.ID]; !exists {
								dmListener := listeners.NewCognexListener(
									cognexCfg.ID,
									cognexCfg.Host,
									cognexCfg.Port,
									cognexCfg.ScanMethod,
									dbManager,
									ssms_manager,
									nil, // DataMatrix no usa boxCache (solo para ID-DB)
								)
								cognexDevices[cognexCfg.ID] = dmListener
								log.Printf("     📷 Cámara DataMatrix Cognex #%d → Salida #%d (%s:%d)",
									cognexCfg.ID, salidaCfg.ID, cognexCfg.Host, cognexCfg.Port)
							}
							break
						}
					}
				}
			}
			if len(cognexDevices) > 0 {
				log.Printf("     ✅ %d cámara(s) DataMatrix configurada(s) para Sorter #%d", len(cognexDevices), sorterCfg.ID)
			}

			// Crear salidas desde config YAML e insertarlas en la base de datos
			var salidas []shared.Salida
			for _, salidaCfg := range sorterCfg.Salidas {
				physicalID := salidaCfg.PhysicalID
				if physicalID <= 0 {
					physicalID = salidaCfg.ID // Usar ID como PhysicalID si no está configurado
				}

				// Normalizar tipo de salida desde YAML
				tipo := salidaCfg.Tipo
				if tipo == "" {
					tipo = "automatico" // Default si no está especificado
				}
				// Normalizar "automatica" -> "automatico" (plural a singular)
				if tipo == "automatica" {
					tipo = "automatico"
				}

				// Crear salida con todos los parámetros incluyendo CognexID desde config
				var salida shared.Salida
				if salidaCfg.CognexID > 0 {
					// Usar el CognexID especificado en el config
					salida = shared.GetNewSalidaComplete(salidaCfg.ID, physicalID, salidaCfg.CognexID, salidaCfg.Nombre, tipo, salidaCfg.MesaID, salidaCfg.BatchSize)
					log.Printf("           📷 CognexID=%d asignado a salida", salidaCfg.CognexID)
				} else {
					salida = shared.GetNewSalidaWithPhysicalID(salidaCfg.ID, physicalID, salidaCfg.Nombre, tipo, salidaCfg.MesaID, salidaCfg.BatchSize)
				}

				salida.EstadoNode = salidaCfg.PLC.EstadoNodeID
				salida.BloqueoNode = salidaCfg.PLC.BloqueoNodeID

				// Vincular FX6Manager ANTES de añadir al slice (para evitar copiar el mutex)
				if fx6Manager != nil {
					salida.SetFX6Manager(fx6Manager)

					// Inicializar números de caja disponibles para DataMatrix
					boxNumbers := []int{
						20000058, 20000059, 20000060, 20000061, 20000062,
						20000063, 20000064, 20000065, 20000066, 20000067,
						20000068, 20000069, 20000070, 20000071, 20000072,
						20000073, 20000074, 20000075, 20000076, 20000077,
						20000078,
					}
					salida.InitializeBoxNumbers(boxNumbers)

					log.Printf("           ✅ FX6Manager vinculado (DataMatrix habilitado)")
					log.Printf("           📦 %d números de caja configurados", len(boxNumbers))
				}

				// Vincular cliente de pallet para salidas automáticas
				if tipo == "automatico" && sorterCfg.PaletAutomatico.Host != "" && sorterCfg.PaletAutomatico.Port > 0 {
					palletClient := pallet.NewClient(sorterCfg.PaletAutomatico.Host, sorterCfg.PaletAutomatico.Port, 10*time.Second)
					salida.SetPalletClient(palletClient)
					log.Printf("           ✅ Cliente Serfruit vinculado (Host: %s:%d)",
						sorterCfg.PaletAutomatico.Host, sorterCfg.PaletAutomatico.Port)
				}

				// Importante: añadir la salida después de configurarla completamente
				salidas = append(salidas, salida)

				log.Printf("       ↳ Salida %d: %s [%s] (physical_id=%d)", salidaCfg.ID, salidaCfg.Nombre, tipo, physicalID)
				if salidaCfg.PLC.EstadoNodeID != "" {
					log.Printf("           EstadoNode: %s", salidaCfg.PLC.EstadoNodeID)
				}
				if salidaCfg.PLC.BloqueoNodeID != "" {
					log.Printf("           BloqueoNode: %s", salidaCfg.PLC.BloqueoNodeID)
				}

				// Insertar salida en la base de datos si no existe
				if err := dbManager.InsertSalidaIfNotExists(ctx, salidaCfg.ID, sorterCfg.ID, physicalID, true); err != nil {
					log.Printf("         ⚠️  Error al sincronizar salida en DB: %v", err)
				} else {
					log.Printf("         ✅ Salida %d sincronizada en DB (physical_id=%d)", salidaCfg.ID, physicalID)
				}
			}

			// CRÍTICO: Asegurar que SKU REJECT existe en la tabla sku para que JOIN funcione
			rejectSQL := `INSERT INTO sku (calibre, variedad, embalaje, dark, estado) 
			              VALUES ('REJECT', 'REJECT', 'REJECT', 0, true) 
			              ON CONFLICT (calibre, variedad, embalaje, dark) DO NOTHING`
			if _, err := dbManager.Pool().Exec(ctx, rejectSQL); err != nil {
				log.Printf("     ⚠️  Error al insertar REJECT en tabla sku: %v", err)
			}

			// Cargar SKUs asignadas desde la base de datos
			skusBySalida, err := dbManager.LoadAssignedSKUsForSorter(ctx, sorterCfg.ID)
			if err != nil {
				log.Printf("     ⚠️  Error al cargar SKUs desde BD: %v", err)
			} else if len(skusBySalida) > 0 {
				log.Printf("     📥 Cargando SKUs persistidas desde BD...")
				totalSKUsLoaded := 0
				for i := range salidas {
					salidaID := salidas[i].ID
					if skus, existe := skusBySalida[salidaID]; existe && len(skus) > 0 {
						salidas[i].SKUs_Actuales = skus
						totalSKUsLoaded += len(skus)
						for _, sku := range skus {
							log.Printf("        ✓ Salida %d: SKU '%s' (Calibre:%s, Variedad:%s, Embalaje:%s)",
								salidaID, sku.SKU, sku.Calibre, sku.Variedad, sku.Embalaje)
						}
					}
				}
				log.Printf("     ✅ Total: %d SKU(s) cargada(s) desde BD", totalSKUsLoaded)
			}

			// Información de nodos PLC
			if sorterCfg.PLC.InputNodeID != "" && sorterCfg.PLC.OutputNodeID != "" {
				log.Printf("     🔌 PLC Configurado:")
				log.Printf("        ↳ Input Node:  %s", sorterCfg.PLC.InputNodeID)
				log.Printf("        ↳ Output Node: %s", sorterCfg.PLC.OutputNodeID)
			}

			s := sorter.GetNewSorter(sorterCfg.ID, sorterCfg.Name, sorterCfg.PLC.InputNodeID, sorterCfg.PLC.OutputNodeID, sorterCfg.PaletAutomatico.Host, sorterCfg.PaletAutomatico.Port, salidas, cognexListener, cognexDevices, httpService.GetWebSocketHub(), dbManager, plcManager, fxSyncManager)
			sorters = append(sorters, s)

			log.Printf("     ✅ Sorter #%d creado y registrado", sorterCfg.ID)

			// Sincronizar mesas de paletizado automático en PostgreSQL
			log.Printf("     🔄 Sincronizando mesas en PostgreSQL...")
			mesasSincronizadas := 0
			for _, salida := range salidas {
				if salida.Tipo == "automatico" || salida.Tipo == "automatica" {
					err := dbManager.InsertMesa(ctx, salida.MesaID, salida.ID)
					if err != nil {
						log.Printf("        ❌ Error sincronizando mesa %d (salida %d): %v", salida.MesaID, salida.ID, err)
					} else {
						log.Printf("        ✅ Mesa %d sincronizada (salida %d)", salida.MesaID, salida.ID)
						mesasSincronizadas++
					}
				}
			}
			if mesasSincronizadas > 0 {
				log.Printf("     ✅ %d mesa(s) sincronizada(s) en PostgreSQL", mesasSincronizadas)
			}

			// Registrar dispositivos del sorter en el monitor
			// Registrar PLC
			if sorterCfg.PLCEndpoint != "" {
				// Extraer host y puerto del endpoint OPC UA
				// Formato: opc.tcp://host:port
				plcHost, plcPort := "", 4840
				if len(sorterCfg.PLCEndpoint) > 10 {
					endpoint := sorterCfg.PLCEndpoint[10:] // Quitar "opc.tcp://"
					if colonIdx := 0; colonIdx < len(endpoint) {
						for i, c := range endpoint {
							if c == ':' {
								colonIdx = i
								break
							}
						}
						if colonIdx > 0 {
							plcHost = endpoint[:colonIdx]
							fmt.Sscanf(endpoint[colonIdx+1:], "%d", &plcPort)
						} else {
							plcHost = endpoint
						}
					}
				}

				if plcHost != "" {
					plcDevice := &models.DeviceStatus{
						ID:             sorterCfg.ID*100 + 1, // ID único: sorterID * 100 + 1 para PLC
						DeviceName:     fmt.Sprintf("PLC Sorter #%d", sorterCfg.ID),
						DeviceType:     models.DeviceTypePLC,
						IP:             plcHost,
						Port:           plcPort,
						SectionID:      sorterCfg.ID,
						IsDisconnected: false,
					}
					deviceMonitor.RegisterDevice(plcDevice)
				}
			}

			// Registrar Cognex del sorter
			if cognexListener != nil {
				cognexCfg := cfg.CognexDevices[sorterCfg.ID-1] // Asumiendo relación 1:1
				cognexDevice := &models.DeviceStatus{
					ID:             sorterCfg.ID*100 + 2, // ID único: sorterID * 100 + 2 para Cognex
					DeviceName:     cognexCfg.Name,
					DeviceType:     models.DeviceTypeCognex,
					IP:             cognexCfg.Host,
					Port:           cognexCfg.Port,
					SectionID:      sorterCfg.ID,
					IsDisconnected: false,
				}
				deviceMonitor.RegisterDevice(cognexDevice)
			}
		}

		log.Println("  ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		log.Println("")

		// Cargar y publicar SKUs activos a TODOS los sorters
		if skuManager != nil {
			log.Println("🔄 Cargando SKUs activos para todos los sorters...")
			activeSKUs := skuManager.GetActiveSKUs()

			// Convertir SKUs a formato assignable con hash
			assignableSKUs := make([]models.SKUAssignable, 0, len(activeSKUs)+1)

			// Agregar SKU REJECT (ID 0) al inicio
			assignableSKUs = append(assignableSKUs, models.GetRejectSKU())

			// Convertir SKUs activos usando hash como ID
			for _, sku := range activeSKUs {
				assignableSKUs = append(assignableSKUs, sku.ToAssignableWithHash())
			}

			// Publicar SKUs a cada sorter usando su método UpdateSKUs
			for _, s := range sorters {
				s.UpdateSKUs(assignableSKUs)
				log.Printf("   ✅ Sorter #%d: %d SKUs publicados", s.ID, len(assignableSKUs))
			}
			log.Println("")
		} else {
			log.Println("⚠️  SKUManager no disponible, sorters sin SKUs iniciales")
			log.Println("")
		}

		// Asignar REJECT a salidas manuales aleatorias (solo en memoria)
		log.Println("🎲 Asignando SKU REJECT a salidas de descarte...")
		for _, s := range sorters {
			salidas := s.GetSalidas()

			// Buscar salidas manuales
			var manualSalidas []shared.Salida
			for _, salida := range salidas {
				if salida.Tipo == "manual" {
					manualSalidas = append(manualSalidas, salida)
				}
			}

			if len(manualSalidas) > 0 {
				// Elegir salida manual aleatoria
				randomIndex := rand.IntN(len(manualSalidas))
				targetSalida := manualSalidas[randomIndex]

				// Asignar REJECT (ID=0) a esta salida
				_, _, _, _, err := s.AssignSKUToSalida(0, targetSalida.ID)
				if err != nil {
					log.Printf("   ⚠️  Error al asignar REJECT en Sorter #%d: %v", s.ID, err)
				} else {
					log.Printf("   ✅ Sorter #%d: REJECT asignado a '%s' (ID=%d)",
						s.ID, targetSalida.Salida_Sorter, targetSalida.ID)
				}
			} else {
				log.Printf("   ⚠️  Sorter #%d: No hay salidas manuales para REJECT", s.ID)
			}
		}
		log.Println("")

		// Iniciar todos los sorters
		log.Println("🚀 Iniciando sorters...")
		for _, s := range sorters {
			if err := s.Start(); err != nil {
				log.Printf("   ⚠️  Error al iniciar Sorter #%d: %v", s.ID, err)
			} else {
				log.Printf("   ✅ Sorter #%d iniciado correctamente", s.ID)
			}
		}
		log.Println("")

		// Iniciar sistema de estadísticas de flujo para cada sorter
		log.Println("📊 Iniciando sistema de estadísticas de flujo...")
		calculationInterval := cfg.Statistics.GetFlowCalculationInterval()
		windowDuration := cfg.Statistics.GetFlowWindowDuration()

		log.Printf("   ⏱️  Intervalo de cálculo: %v", calculationInterval)
		log.Printf("   🪟  Ventana de tiempo: %v", windowDuration)

		for _, s := range sorters {
			go s.StartFlowStatistics(calculationInterval, windowDuration)
			log.Printf("   ✅ Sorter #%d: Sistema de flow stats iniciado", s.ID)
		}
		log.Println("")

		// Iniciar monitor de dispositivos
		log.Println("📡 Iniciando monitoreo de dispositivos...")
		go deviceMonitor.Start()
		log.Println("   ✅ Device Monitor iniciado (heartbeat continuo)")
		log.Println("")
	}

	// Cerrar todos los listeners y sorters al finalizar
	defer func() {
		for _, cl := range cognexListeners {
			cl.Stop()
		}
	}()

	// Iniciar SKU Sync Worker si está disponible
	if skuManager != nil && len(sorters) > 0 {
		log.Println("🔄 Inicializando SKU Sync Worker...")
		log.Printf("   📍 Vista UNITEC: VW_INT_DANICH_ENVIVO")

		// Pasar directamente la configuración del YAML
		sqlServerMgr, err := db.GetManagerWithConfig(ctx, cfg.Database.SQLServer)
		if err != nil {
			log.Printf("   ❌ ERROR: No se pudo conectar a SQL Server UNITEC: %v", err)
			log.Println("   ⚠️  Sync Worker DESHABILITADO - SKUs NO se sincronizarán desde UNITEC")
			log.Println("   ⚠️  Solo se usarán SKUs ya persistidas en PostgreSQL")
		} else {
			log.Println("   ✅ Conexión a SQL Server UNITEC exitosa")

			syncInterval := cfg.Database.SQLServer.GetSKUSyncInterval()

			// Convertir []*sorter.Sorter a []shared.SorterInterface
			sorterInterfaces := make([]shared.SorterInterface, len(sorters))
			for i, s := range sorters {
				sorterInterfaces[i] = s
			}

			syncWorker := flow.NewSKUSyncWorker(
				context.Background(),
				sqlServerMgr,
				dbManager,
				skuManager,
				sorterInterfaces,
				syncInterval,
			)

			syncWorker.Start()
			defer syncWorker.Stop()

			log.Printf("   ✅ Sync Worker iniciado (intervalo: %v)", syncInterval)
		}
		log.Println("")
	}

	// Registrar todos los sorters para acceso directo desde HTTP
	for _, s := range sorters {
		httpService.RegisterSorter(s)
	}

	// Configurar WebSocket para cada sorter
	if len(sorters) > 0 {
		wsHub := httpService.GetWebSocketHub()

		// Crear rooms de WebSocket
		sorterIDs := make([]int, len(sorters))
		for i, s := range sorters {
			sorterIDs[i] = s.ID
		}
		wsHub.CreateRoomsForSorters(sorterIDs)

		// Suscribir WebSocket Hub a los canales de cada sorter
		for _, s := range sorters {
			wsHub.SubscribeToSorter(s)
		}

		log.Printf("🔌 WebSocket: %d room(s) creadas y suscritas para notificaciones en tiempo real", len(sorterIDs))
		for _, id := range sorterIDs {
			log.Printf("   📦 Room: assignment_%d (Sorter #%d)", id, id)
		}
		log.Println("")
	}

	log.Printf("🌐 Servidor HTTP iniciando en %s...", httpAddr)
	log.Println("📊 Endpoints disponibles:")
	log.Println("   GET  /Mesa/Estado")
	log.Println("   POST /Mesa")
	log.Println("   POST /Mesa/Vaciar")
	log.Println("   GET  /status")
	log.Println("   POST /assignment")
	log.Println("   DELETE /assignment/:sealer_id/:sku_id")
	log.Println("   DELETE /assignment/:sealer_id")
	log.Printf("   GET  /skus/assignables/:sorter_id (acceso directo sin bloqueo, %d sorters registrados)", len(sorters))
	log.Printf("   GET  /sku/assigned/:sorter_id")
	log.Printf("   GET  /assignment/sorter/:sorter_id/history")
	log.Println("")
	log.Println("📡 Monitoring endpoints:")
	log.Println("   GET  /monitoring/devices/sections")
	log.Println("   GET  /monitoring/devices/:section_id")
	log.Println("   GET  /monitoring/devices")
	log.Println("")
	log.Println("🔌 WebSocket endpoints:")
	log.Println("   WS   /ws/:room (ej: ws://host/ws/assignment_1)")
	log.Println("   GET  /ws/stats (estadísticas de conexiones)")
	log.Println("")

	// Iniciar servidor estático para frontend (dist/) en puerto 3001
	staticPort := 3001
	staticAddr := fmt.Sprintf("%s:%d", httpHost, staticPort)

	go func() {
		log.Printf("🎨 Servidor estático iniciando en %s (sirviendo carpeta dist/)...", staticAddr)
		if err := listeners.StartStaticFileServer(staticAddr, "dist"); err != nil {
			log.Printf("❌ Error al iniciar servidor estático: %v", err)
		}
	}()

	// Iniciar servidor HTTP con las rutas configuradas
	if err := httpService.Start(); err != nil {
		log.Fatalf("❌ Error al iniciar servidor HTTP: %v", err)
	}
}
