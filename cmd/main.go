package main

import (
	"context"
	"fmt"
	"log"
	"math/rand/v2"
	"os"
	"time"

	"API-GREENEX/internal/config"
	"API-GREENEX/internal/db"
	"API-GREENEX/internal/flow"
	"API-GREENEX/internal/listeners"
	"API-GREENEX/internal/models"
	"API-GREENEX/internal/shared"
	"API-GREENEX/internal/sorter"

	"github.com/joho/godotenv"
)

func main() {
	// Configurar logger sin timestamps para el banner
	log.SetOutput(os.Stdout)
	log.SetFlags(0)

	// Tu hermoso banner ASCII
	log.Println("")
	log.Println("    ░█████╗░██████╗░██╗░░░░░░░██████╗░██████╗░███████╗███████╗███╗░░██╗███████╗██╗░░██╗")
	log.Println("    ██╔══██╗██╔══██╗██║░░░░░░██╔════╝░██╔══██╗██╔════╝██╔════╝████╗░██║██╔════╝╚██╗██╔╝")
	log.Println("    ███████║██████╔╝██║█████╗██║░░██╗░██████╔╝█████╗░░█████╗░░██╔██╗██║█████╗░░░╚███╔╝░")
	log.Println("    ██╔══██║██╔═══╝░██║╚════╝██║░░╚██╗██╔══██╗██╔══╝░░██╔══╝░░██║╚████║██╔══╝░░░██╔██╗░")
	log.Println("    ██║░░██║██║░░░░░██║░░░░░░╚██████╔╝██║░░██║███████╗███████╗██║░╚███║███████╗██╔╝╚██╗")
	log.Println("    ╚═╝░░╚═╝╚═╝░░░░░╚═╝░░░░░░░╚═════╝░╚═╝░░╚═╝╚══════╝╚══════╝╚═╝░░╚══╝╚══════╝╚═╝░░╚═╝")
	log.Println("")
	log.Println("Iniciando API-Greenex...")
	log.Println("")

	// Ahora activar fecha/hora para los logs normales
	log.SetFlags(log.Ldate | log.Ltime)

	// 0. Inicializar gestor de canales compartidos (Singleton)
	channelMgr := shared.GetChannelManager()
	defer channelMgr.CloseAll()
	log.Println("✅ Gestor de canales inicializado")
	log.Println("")

	// 1. Cargar archivo .env para obtener ruta del config
	if err := godotenv.Load(); err != nil {
		log.Println("⚠️  Archivo .env no encontrado, usando valores por defecto")
	}

	// 2. Cargar configuración desde YAML
	configPath := os.Getenv("CONFIG_FILE")
	if configPath == "" {
		configPath = "config/config.yaml"
	}

	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		log.Fatalf("❌ Error al cargar configuración: %v", err)
	}
	log.Printf("✅ Configuración cargada desde: %s", configPath)

	// 3. Inicializar la conexión a PostgreSQL usando config YAML
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

	// 3.5. Inicializar SKUManager para gestión eficiente con streaming
	skuManager, err := flow.NewSKUManager(ctx, dbManager)
	if err != nil {
		log.Printf("⚠️  Error al inicializar SKUManager: %v (continuando sin caché de SKUs)", err)
		skuManager = nil
	}

	// 4. Crear listeners de Cognex (NO iniciarlos aún, los sorters lo harán)
	log.Println("")
	log.Printf("📷 Configurando %d dispositivo(s) Cognex...", len(cfg.CognexDevices))
	var cognexListeners []*listeners.CognexListener

	for _, cognexCfg := range cfg.CognexDevices {
		log.Println("  ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		log.Printf("  📌 Cognex #%d: %s", cognexCfg.ID, cognexCfg.Name)
		log.Printf("     Ubicación: %s", cognexCfg.Ubicacion)
		log.Printf("     Host: %s:%d", cognexCfg.Host, cognexCfg.Port)
		log.Printf("     Método: %s", cognexCfg.ScanMethod)

		cognexListener := listeners.NewCognexListener(
			cognexCfg.Host,
			cognexCfg.Port,
			cognexCfg.ScanMethod,
			dbManager,
		)

		cognexListeners = append(cognexListeners, cognexListener)
		log.Printf("     ✅ CognexListener creado (será iniciado por el sorter)")
	}
	log.Println("  ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	log.Println("")

	// Inicializar Sorters (si hay configurados)
	var sorters []*sorter.Sorter
	if len(cfg.Sorters) > 0 {
		log.Printf("🔀 Inicializando %d Sorter(s)...", len(cfg.Sorters))
		log.Println("")

		for _, sorterCfg := range cfg.Sorters {
			log.Println("  ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
			log.Printf("  📦 Sorter #%d: %s", sorterCfg.ID, sorterCfg.Name)
			log.Printf("     Ubicación: %s", sorterCfg.Ubicacion)
			log.Printf("     Cognex ID: %d", sorterCfg.CognexID)

			// Insertar sorter en la base de datos si no existe
			if err := dbManager.InsertSorterIfNotExists(ctx, sorterCfg.ID, sorterCfg.Ubicacion); err != nil {
				log.Printf("     ⚠️  Error al insertar sorter en DB: %v", err)
			} else {
				log.Printf("     ✅ Sorter #%d insertado en DB (o ya existía)", sorterCfg.ID)
			}

			// Buscar el CognexListener correspondiente por índice (CognexID - 1)
			var cognexListener *listeners.CognexListener
			if sorterCfg.CognexID > 0 && sorterCfg.CognexID <= len(cognexListeners) {
				cognexListener = cognexListeners[sorterCfg.CognexID-1]
			}

			if cognexListener == nil {
				log.Printf("     ⚠️  No se encontró Cognex Listener para Cognex ID %d, usando el primero disponible", sorterCfg.CognexID)
				if len(cognexListeners) > 0 {
					cognexListener = cognexListeners[0]
				}
			}

			// Crear salidas desde config YAML e insertarlas en la base de datos
			var salidas []shared.Salida
			salida_counter := 1
			for _, salidaCfg := range sorterCfg.Salidas {
				tipo := salidaCfg.Tipo
				if tipo == "" {
					tipo = "manual" // Default si no está especificado
				}
				salida := shared.GetNewSalida(salidaCfg.ID, salidaCfg.Name, tipo)
				salidas = append(salidas, salida)
				log.Printf("       ↳ Salida %d: %s (%s)", salidaCfg.ID, salidaCfg.Name, tipo)
				// Insertar salida en la base de datos si no existe
				if err := dbManager.InsertSalidaIfNotExists(ctx, salidaCfg.ID, sorterCfg.ID, salida_counter, true); err != nil {
					log.Printf("         ⚠️  Error al insertar salida en DB: %v", err)
				} else {
					log.Printf("         ✅ Salida %d insertada en DB (o ya existía)", salidaCfg.ID)
				}

				salida_counter++
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

			s := sorter.GetNewSorter(sorterCfg.ID, sorterCfg.Ubicacion, salidas, cognexListener)
			sorters = append(sorters, s)

			log.Printf("     ✅ Sorter #%d creado y registrado", sorterCfg.ID)
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

		// 🎲 Asignar REJECT a salidas manuales aleatorias (solo en memoria)
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
				_, _, _, err := s.AssignSKUToSalida(0, targetSalida.ID)
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
	}

	// Cerrar todos los listeners y sorters al finalizar
	defer func() {
		for _, cl := range cognexListeners {
			cl.Stop()
		}
		for _, s := range sorters {
			s.Stop()
		}
	}()

	// 5. Crear e iniciar el servidor HTTP con endpoints
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

	// Registrar todos los sorters para acceso directo desde HTTP
	for _, s := range sorters {
		httpService.RegisterSorter(s)
	}

	// 🔌 Configurar WebSocket para cada sorter
	if len(sorters) > 0 {
		wsHub := httpService.GetWebSocketHub()

		// Crear rooms de WebSocket
		sorterIDs := make([]int, len(sorters))
		for i, s := range sorters {
			sorterIDs[i] = s.ID
		}
		wsHub.CreateRoomsForSorters(sorterIDs)

		// 🔔 Suscribir WebSocket Hub a los canales de cada sorter
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
	log.Printf("   GET  /skus/assignables/:sorter_id (⚡ acceso directo sin bloqueo, %d sorters registrados)", len(sorters))
	log.Printf("   GET  /sku/assigned/:sorter_id")
	log.Println("")
	log.Println("🔌 WebSocket endpoints:")
	log.Println("   WS   /ws/:room (ej: ws://host/ws/assignment_1)")
	log.Println("   GET  /ws/stats (estadísticas de conexiones)")

	// Iniciar servidor HTTP con las rutas configuradas
	if err := httpService.Start(); err != nil {
		log.Fatalf("❌ Error al iniciar servidor HTTP: %v", err)
	}
}
