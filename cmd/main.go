package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"time"

	"api-dansort/internal/communication/pallet"
	"api-dansort/internal/communication/plc"
	"api-dansort/internal/config"
	"api-dansort/internal/db"
	"api-dansort/internal/flow"
	"api-dansort/internal/listeners"
	"api-dansort/internal/models"
	"api-dansort/internal/monitoring"
	"api-dansort/internal/shared"
	"api-dansort/internal/sorter"

	"github.com/joho/godotenv"
)

func main() {
	// Configurar logger sin timestamps para el banner
	log.SetOutput(os.Stdout)
	log.SetFlags(0)

	// Tu hermoso banner ASCII
	log.Println("    ░█████╗░██████╗░██╗░░░░░░░██████╗░██████╗░███████╗███████╗███╗░░██╗███████╗██╗░░██╗")
	log.Println("    ██╔══██╗██╔══██╗██║░░░░░░██╔════╝░██╔══██╗██╔════╝██╔════╝████╗░██║██╔════╝╚██╗██╔╝")
	log.Println("    ███████║██████╔╝██║█████╗██║░░██╗░██████╔╝█████╗░░█████╗░░██╔██╗██║█████╗░░░╚███╔╝░")
	log.Println("    ██╔══██║██╔═══╝░██║╚════╝██║░░╚██╗██╔══██╗██╔══╝░░██╔══╝░░██║╚████║██╔══╝░░░██╔██╗░")
	log.Println("    ██║░░██║██║░░░░░██║░░░░░░╚██████╔╝██║░░██║███████╗███████╗██║░╚███║███████╗██╔╝╚██╗")
	log.Println("    ╚═╝░░╚═╝╚═╝░░░░░╚═╝░░░░░░░╚═════╝░╚═╝░░╚═╝╚══════╝╚══════╝╚═╝░░╚══╝╚══════╝╚═╝░░╚═╝")
	log.Println("")

	// Restaurar flags del logger
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	// Inicializar gestor de canales
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

	// Crear la conexión a SQL Server (Unitec) de forma síncrona y centralizada
	log.Println("📊 Inicializando conexión a SQL Server UNITEC...")
	sqlServerMgr, err := db.GetManagerWithConfig(ctx, cfg.Database.SQLServer)
	if err != nil {
		// A diferencia de otros managers, si este falla, es crítico.
		log.Fatalf("❌ ERROR CRÍTICO: No se pudo conectar a SQL Server UNITEC: %v. El worker de SKUs y la validación de DataMatrix no funcionarán.", err)
	}
	defer sqlServerMgr.Close()
	log.Println("✅ Conexión a SQL Server UNITEC exitosa")

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
	cognexListeners := make(map[int]*listeners.CognexListener)

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
			cfg.SKUFormat,
			dbManager,
		)

		cognexListeners[cognexCfg.ID] = cognexListener
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
	} else {
		log.Println("[Init] PLC Manager connected successfully")
	}

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

			if err := dbManager.InsertSorterIfNotExists(ctx, sorterCfg.ID, sorterCfg.Name); err != nil {
				log.Printf("     ⚠️  Error al insertar sorter en DB: %v", err)
			} else {
				log.Printf("     ✅ Sorter #%d insertado en DB (o ya existía)", sorterCfg.ID)
			}

			cognexListener := cognexListeners[sorterCfg.CognexPrincipalID]

			cognexDevices := make(map[int]*listeners.CognexListener)
			for _, salidaCfg := range sorterCfg.Salidas {
				if listener, ok := cognexListeners[salidaCfg.CognexID]; ok {
					cognexDevices[salidaCfg.CognexID] = listener
				}
			}
			log.Printf("     ✅ %d cámara(s) DataMatrix configurada(s) para Sorter #%d", len(cognexDevices), sorterCfg.ID)

			var salidas []shared.Salida
			for _, salidaCfg := range sorterCfg.Salidas {
				physicalID := salidaCfg.PhysicalID
				if physicalID == 0 {
					physicalID = salidaCfg.ID
				}

				tipo := salidaCfg.Tipo
				if tipo == "" || tipo == "automatica" {
					tipo = "automatico"
				}

				salida := shared.GetNewSalida(salidaCfg.ID, salidaCfg.Nombre, tipo, salidaCfg.MesaID, salidaCfg.BatchSize)
				salida.SealerPhysicalID = physicalID
				salida.CognexID = salidaCfg.CognexID
				salida.EstadoNode = salidaCfg.PLC.EstadoNodeID
				salida.BloqueoNode = salidaCfg.PLC.BloqueoNodeID

				if sqlServerMgr != nil {
					salida.SetSSMSManager(sqlServerMgr)
				}

				if fx6Manager != nil {
					salida.SetFX6Manager(fx6Manager)
					boxNumbers := []int{
						20000058, 20000059, 20000060, 20000061, 20000062,
						20000063, 20000064, 20000065, 20000066, 20000067,
						20000078,
					}
					salida.InitializeBoxNumbers(boxNumbers)
				}

				if (tipo == "automatico") && sorterCfg.PaletAutomatico.Host != "" && sorterCfg.PaletAutomatico.Port > 0 {
					palletClient := pallet.NewClient(sorterCfg.PaletAutomatico.Host, sorterCfg.PaletAutomatico.Port, 10*time.Second)
					salida.SetPalletClient(palletClient)
				}

				salidas = append(salidas, salida)

				log.Printf("       ↳ Salida %d: %s [%s] (physical_id=%d)", salidaCfg.ID, salidaCfg.Nombre, tipo, physicalID)
				if salidaCfg.PLC.EstadoNodeID != "" {
					log.Printf("           EstadoNode: %s", salidaCfg.PLC.EstadoNodeID)
				}
				if salidaCfg.PLC.BloqueoNodeID != "" {
					log.Printf("           BloqueoNode: %s", salidaCfg.PLC.BloqueoNodeID)
				}

				if err := dbManager.InsertSalidaIfNotExists(ctx, salidaCfg.ID, sorterCfg.ID, physicalID, true); err != nil {
					log.Printf("         ⚠️  Error al sincronizar salida en DB: %v", err)
				} else {
					log.Printf("         ✅ Salida %d sincronizada en DB (physical_id=%d)", salidaCfg.ID, physicalID)
				}
			}

			rejectSQL := `INSERT INTO sku (calibre, variedad, embalaje, dark, estado)
			              VALUES ('REJECT', 'REJECT', 'REJECT', 0, true) 
			              ON CONFLICT (calibre, variedad, embalaje, dark) DO NOTHING`
			if _, err := dbManager.Pool().Exec(ctx, rejectSQL); err != nil {
				log.Printf("     ⚠️  Error al insertar REJECT en tabla sku: %v", err)
			}

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

			if sorterCfg.PLC.InputNodeID != "" && sorterCfg.PLC.OutputNodeID != "" {
				log.Printf("     🔌 PLC Configurado:")
				log.Printf("        ↳ Input Node:  %s", sorterCfg.PLC.InputNodeID)
				log.Printf("        ↳ Output Node: %s", sorterCfg.PLC.OutputNodeID)
			}

			s := sorter.GetNewSorter(sorterCfg.ID, sorterCfg.Name, sorterCfg.PLC.InputNodeID, sorterCfg.PLC.OutputNodeID, sorterCfg.PaletAutomatico.Host, sorterCfg.PaletAutomatico.Port, salidas, cognexListener, cognexDevices, httpService.GetWebSocketHub(), dbManager, plcManager, fxSyncManager)
			sorters = append(sorters, s)

			log.Printf("     ✅ Sorter #%d creado y registrado", sorterCfg.ID)

			log.Printf("     🔄 Sincronizando mesas en PostgreSQL...")
			mesasSincronizadas := 0
			for _, salida := range salidas {
				if salida.Tipo == "automatico" {
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

			if sorterCfg.PLCEndpoint != "" {
				plcHost, plcPort := "", 4840
				if len(sorterCfg.PLCEndpoint) > 10 {
					endpoint := sorterCfg.PLCEndpoint[10:]
					if colonIdx := strings.Index(endpoint, ":"); colonIdx != -1 {
						plcHost = endpoint[:colonIdx]
						if port, err := strconv.Atoi(endpoint[colonIdx+1:]); err == nil {
							plcPort = port
						}
					} else {
						plcHost = endpoint
					}
				}

				if plcHost != "" {
					plcDevice := &models.DeviceStatus{
						ID:             sorterCfg.ID*100 + 1,
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

			if cognexListener != nil {
				var foundCognexCfg *config.CognexDevice
				for i := range cfg.CognexDevices {
					if cfg.CognexDevices[i].ID == sorterCfg.CognexPrincipalID {
						foundCognexCfg = &cfg.CognexDevices[i]
						break
					}
				}

				if foundCognexCfg != nil {
					cognexDevice := &models.DeviceStatus{
						ID:             sorterCfg.ID*100 + 2,
						DeviceName:     foundCognexCfg.Name,
						DeviceType:     models.DeviceTypeCognex,
						IP:             foundCognexCfg.Host,
						Port:           foundCognexCfg.Port,
						SectionID:      sorterCfg.ID,
						IsDisconnected: false,
					}
					deviceMonitor.RegisterDevice(cognexDevice)
				}
			}
		}
		log.Println("  ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		log.Println("")

		if skuManager != nil {
			log.Println("🔄 Cargando SKUs activos para todos los sorters...")
			activeSKUs := skuManager.GetActiveSKUs()
			assignableSKUs := make([]models.SKUAssignable, 0, len(activeSKUs)+1)
			assignableSKUs = append(assignableSKUs, models.GetRejectSKU())
			for _, sku := range activeSKUs {
				assignableSKUs = append(assignableSKUs, sku.ToAssignableWithHash())
			}
			for _, s := range sorters {
				s.UpdateSKUs(assignableSKUs)
				log.Printf("   ✅ Sorter #%d: %d SKUs publicados", s.ID, len(assignableSKUs))
			}
			log.Println("")
		} else {
			log.Println("⚠️  SKUManager no disponible, sorters sin SKUs iniciales")
			log.Println("")
		}

		log.Println("🎲 Asignando SKU REJECT a salidas de descarte...")
		for _, s := range sorters {
			salidas := s.GetSalidas()
			var manualSalidas []shared.Salida
			for _, salida := range salidas {
				if salida.Tipo == "manual" {
					manualSalidas = append(manualSalidas, salida)
				}
			}

			if len(manualSalidas) > 0 {
				randomIndex := rand.Intn(len(manualSalidas))
				targetSalida := manualSalidas[randomIndex]
				_, _, _, _, err := s.AssignSKUToSalida(0, targetSalida.ID)
				if err != nil {
					log.Printf("   ⚠️  Error al asignar REJECT en Sorter #%d: %v", s.ID, err)
				} else {
					log.Printf("   ✅ Sorter #%d: REJECT asignado a '%s' (ID=%d)", s.ID, targetSalida.Salida_Sorter, targetSalida.ID)
				}
			} else {
				log.Printf("   ⚠️  Sorter #%d no tiene salidas manuales para asignar REJECT", s.ID)
			}
		}
		log.Println("")

		for _, s := range sorters {
			if err := s.Start(); err != nil {
				log.Printf("❌ Error al iniciar Sorter #%d: %v", s.ID, err)
			}
		}
		log.Println("")

		if skuManager != nil && sqlServerMgr != nil {
			startSKUSyncWorker(context.Background(), cfg, dbManager, skuManager, sorters, sqlServerMgr)
		}

		for _, s := range sorters {
			httpService.RegisterSorter(s)
		}

		if len(sorters) > 0 {
			wsHub := httpService.GetWebSocketHub()
			sorterIDs := make([]int, len(sorters))
			for i, s := range sorters {
				sorterIDs[i] = s.ID
			}
			wsHub.CreateRoomsForSorters(sorterIDs)
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

		staticPort := 3001
		staticAddr := fmt.Sprintf("%s:%d", httpHost, staticPort)

		go func() {
			log.Printf("🎨 Servidor estático iniciando en %s (sirviendo carpeta dist/)...", staticAddr)
			if err := listeners.StartStaticFileServer(staticAddr, "dist"); err != nil {
				log.Printf("❌ Error al iniciar servidor estático: %v", err)
			}
		}()

		if err := httpService.Start(); err != nil {
			log.Fatalf("❌ Error al iniciar servidor HTTP: %v", err)
		}
	}
}

func startSKUSyncWorker(ctx context.Context, cfg *config.Config, dbManager *db.PostgresManager, skuManager *flow.SKUManager, sorters []*sorter.Sorter, sqlServerMgr *db.Manager) {
	log.Println("🔄 Iniciando SKU Sync Worker...")
	log.Printf("   📍 Vista UNITEC: VW_INT_DANICH_ENVIVO")

	for _, s := range sorters {
		s.SetSSMSManagerForAllSalidas(sqlServerMgr)
	}
	log.Println("   ✅ Conexión a SQL Server inyectada en todas las salidas para consultas DataMatrix")

	syncInterval := cfg.Database.SQLServer.GetSKUSyncInterval()

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
		cfg.SKUFormat,
	)
	go syncWorker.Start()
	log.Printf("   ✅ Sync Worker iniciado (intervalo: %v)", syncInterval)
}
