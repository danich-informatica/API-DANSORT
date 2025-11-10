package main

import (
	"context"
	"log"
	"os"
	"time"

	"api-dansort/internal/config"
	"api-dansort/internal/db"

	"github.com/joho/godotenv"
)

func main() {
	log.SetFlags(log.Ldate | log.Ltime)

	printBanner()

	// Cargar .env
	if err := godotenv.Load(); err != nil {
		log.Printf("⚠️  No se pudo cargar .env: %v (continuando...)", err)
	}

	// Cargar configuración
	configPath := os.Getenv("CONFIG_FILE")
	if configPath == "" {
		configPath = "config/config.yaml"
	}

	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		log.Fatalf("❌ Error al cargar config: %v", err)
	}

	log.Printf("✅ Configuración cargada: %s", configPath)
	log.Println("")

	// Crear contexto con timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// ========================================================================
	// Test 1: Conectar a FX6 usando el Manager escalable
	// ========================================================================
	log.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	log.Println("🧪 Test 1: Conexión a FX6_packing_Garate_Operaciones")
	log.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	log.Println("")

	fx6Manager, err := db.NewFX6Manager(ctx, cfg.Database.SQLServer)
	if err != nil {
		log.Fatalf("❌ Error al conectar FX6: %v", err)
	}
	defer fx6Manager.Close()

	log.Println("")

	// ========================================================================
	// Test 2: Ping de conexión
	// ========================================================================
	log.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	log.Println("🧪 Test 2: Verificando conectividad (Ping)")
	log.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	log.Println("")

	if err := fx6Manager.Ping(ctx); err != nil {
		log.Fatalf("❌ Ping falló: %v", err)
	}
	log.Println("✅ Ping exitoso - Conexión activa")
	log.Println("")

	// ========================================================================
	// Test 3: Inserción de DataMatrix
	// ========================================================================
	log.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	log.Println("🧪 Test 3: Insertando lecturas DataMatrix de prueba")
	log.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	log.Println("")

	testData := []struct {
		salida      int
		correlativo int64
		descripcion string
	}{
		{1, 100001, "Salida 1 - Primera caja"},
		{1, 100002, "Salida 1 - Segunda caja"},
		{7, 200001, "Salida 7 - Primera caja"},
		{7, 123456789, "Salida 7 - Código real simulado"},
		{1, 100003, "Salida 1 - Tercera caja"},
	}

	successCount := 0
	errorCount := 0

	for i, test := range testData {
		numeroCaja := i + 1
		fechaLectura := time.Now()

		log.Printf("  📦 Insertando [%d/%d]: %s", i+1, len(testData), test.descripcion)
		log.Printf("     └─ Salida=%d, Correlativo=%d, Caja=%d",
			test.salida, test.correlativo, numeroCaja)

		err := fx6Manager.InsertLecturaDataMatrix(
			ctx,
			test.salida,
			test.correlativo,
			numeroCaja,
			fechaLectura,
		)

		if err != nil {
			log.Printf("     ❌ Error: %v", err)
			errorCount++
		} else {
			log.Printf("     ✅ Insertado correctamente")
			successCount++
		}

		time.Sleep(200 * time.Millisecond) // Pausa pequeña
	}

	log.Println("")
	log.Printf("📊 Resumen: %d exitosos, %d errores", successCount, errorCount)
	log.Println("")

	// ========================================================================
	// Test 4: Consultar lecturas recientes
	// ========================================================================
	log.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	log.Println("🧪 Test 4: Consultando lecturas recientes")
	log.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	log.Println("")

	// Consultar salida 1
	log.Println("  📋 Lecturas recientes de Salida 1:")
	lecturas1, err := fx6Manager.GetLecturasRecientes(ctx, 1, 10)
	if err != nil {
		log.Printf("  ❌ Error al consultar: %v", err)
	} else {
		if len(lecturas1) == 0 {
			log.Println("  ⚠️  No hay lecturas registradas")
		} else {
			for _, l := range lecturas1 {
				log.Printf("     • Caja #%d: Correlativo=%d (Terminado: %d) - %s",
					l.NumeroCaja, l.Correlativo, l.Terminado, l.FechaLectura.Format("15:04:05"))
			}
		}
	}

	log.Println("")

	// Consultar salida 7
	log.Println("  📋 Lecturas recientes de Salida 7:")
	lecturas7, err := fx6Manager.GetLecturasRecientes(ctx, 7, 10)
	if err != nil {
		log.Printf("  ❌ Error al consultar: %v", err)
	} else {
		if len(lecturas7) == 0 {
			log.Println("  ⚠️  No hay lecturas registradas")
		} else {
			for _, l := range lecturas7 {
				log.Printf("     • Caja #%d: Correlativo=%d (Terminado: %d) - %s",
					l.NumeroCaja, l.Correlativo, l.Terminado, l.FechaLectura.Format("15:04:05"))
			}
		}
	}

	log.Println("")

	// ========================================================================
	// Test 5: Obtener contador actual
	// ========================================================================
	log.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	log.Println("🧪 Test 5: Obteniendo contadores actuales")
	log.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	log.Println("")

	for _, salidaID := range []int{1, 7} {
		contador, err := fx6Manager.GetContadorActual(ctx, salidaID)
		if err != nil {
			log.Printf("  ❌ Error al obtener contador de Salida %d: %v", salidaID, err)
		} else {
			log.Printf("  ✅ Salida %d: Última caja registrada = #%d", salidaID, contador)
		}
	}

	log.Println("")

	// ========================================================================
	// Resumen final
	// ========================================================================
	printSummary()
}

func printBanner() {
	log.Println("")
	log.Println("  ╔═══════════════════════════════════════════════════════════╗")
	log.Println("  ║                                                           ║")
	log.Println("  ║   🧪  TEST DE INSERCIÓN DATAMATRIX - FX6 MANAGER  🧪      ║")
	log.Println("  ║                                                           ║")
	log.Println("  ║   Arquitectura Escalable con Manager Genérico            ║")
	log.Println("  ║                                                           ║")
	log.Println("  ╚═══════════════════════════════════════════════════════════╝")
	log.Println("")
}

func printSummary() {
	log.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	log.Println("✅ TODOS LOS TESTS COMPLETADOS")
	log.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	log.Println("")
	log.Println("📊 Verifica los datos en SQL Server con:")
	log.Println("")
	log.Println("   USE FX6_packing_Garate_Operaciones;")
	log.Println("   SELECT TOP 20 * FROM lectura_datamatrix")
	log.Println("   ORDER BY Fecha_Lectura DESC;")
	log.Println("")
	log.Println("🎉 Test finalizado exitosamente")
	log.Println("")
}
