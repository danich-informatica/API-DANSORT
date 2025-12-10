package main

import (
	"context"
	"log"
	"math/rand"
	"time"

	"API-GREENEX/internal/config"
	"API-GREENEX/internal/db"
)

func main() {
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds)
	log.Println("🧪 Test: Inserts REALES DataMatrix en FX6")
	log.Println("    NUEVA LÓGICA:")
	log.Println("    • Correlativo: ÚNICO número que termina en 1 para todos los inserts")
	log.Println("    • Número de Caja: 184 números diferentes (20000001-20000184)")
	log.Println("")

	ctx := context.Background()

	// Cargar configuración
	cfg, err := config.LoadConfig("config/config.yaml")
	if err != nil {
		log.Fatalf("❌ Error al cargar configuración: %v", err)
	}

	// Crear FX6Manager
	log.Println("📊 Inicializando FX6Manager...")
	fx6Manager, err := db.GetFX6Manager(ctx, cfg)
	if err != nil {
		log.Fatalf("❌ Error al crear FX6Manager: %v", err)
	}
	defer fx6Manager.Close()
	log.Println("✅ FX6Manager inicializado")
	log.Println("")

	// Generar 184 números de caja diferentes (20000001 a 20000184)
	numerosCaja := make([]int, 184)
	for i := 0; i < 184; i++ {
		numerosCaja[i] = 20000001 + i
	}

	log.Printf("📦 Lista de números de caja: %d disponibles (%d a %d)",
		len(numerosCaja), numerosCaja[0], numerosCaja[len(numerosCaja)-1])
	log.Println("")

	// Generar UN ÚNICO correlativo que termine en 1
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	base := rng.Intn(990000) + 10000
	correlativoUnico := int64((base / 10 * 10) + 1)

	log.Printf("🔢 Correlativo único para todos los inserts: %d (termina en %d)",
		correlativoUnico, correlativoUnico%10)
	log.Println("")

	// Test: Insertar 184 lecturas de prueba
	numInserts := 184
	log.Printf("📝 Insertando %d lecturas REALES en SQL Server FX6", numInserts)
	log.Println("═══════════════════════════════════════════════════════════════════════════════")

	salidas := []int{1, 1, 7, 1, 7, 1, 7, 1, 1, 7, 1, 7, 1, 7, 1} // Alternando salidas 1 y 7
	exitosos := 0
	fallidos := 0

	for i := 0; i < numInserts; i++ {
		salida := salidas[i%len(salidas)]

		// Usar el correlativo único para todos los inserts
		correlativo := correlativoUnico

		// Usar el número de caja correspondiente (uno diferente por cada insert)
		numeroCaja := numerosCaja[i]

		log.Printf("\n[%2d/%d] INSERT en PKG_Pallets_Externos:", i+1, numInserts)
		log.Printf("        Salida      : %d", salida)
		log.Printf("        Correlativo : %d ✓ (termina en 1)", correlativo)
		log.Printf("        Número Caja : %d", numeroCaja)
		log.Printf("        Fecha       : %s", time.Now().Format("2006-01-02 15:04:05"))

		err := fx6Manager.InsertLecturaDataMatrix(
			ctx,
			salida,
			correlativo,
			numeroCaja,
			time.Now(),
		)

		if err != nil {
			log.Printf("        ❌ ERROR: %v", err)
			fallidos++
		} else {
			log.Printf("        ✅ Insertado correctamente en base de datos")
			exitosos++
		}

		time.Sleep(200 * time.Millisecond) // Delay entre inserts
	}

	log.Println("")
	log.Println("═══════════════════════════════════════════════════════════════════════════════")
	log.Println("")

	// Mostrar resumen
	log.Println("📊 RESUMEN DE INSERTS:")
	log.Println("─────────────────────────────────────────────────────────────")
	log.Printf("✅ Exitosos: %d/%d", exitosos, numInserts)
	log.Printf("❌ Fallidos: %d/%d", fallidos, numInserts)
	if exitosos == numInserts {
		log.Println("🎉 ¡Todos los inserts fueron exitosos!")
	}
	log.Println("")

	// Consultar lecturas recientes
	log.Println("📚 Consultando lecturas recientes por salida")
	log.Println("─────────────────────────────────────────────────────────────")

	for _, salidaID := range []int{1, 7} {
		lecturas, err := fx6Manager.GetLecturasRecientes(ctx, salidaID, 5)
		if err != nil {
			log.Printf("❌ Error consultando Salida %d: %v", salidaID, err)
			continue
		}

		log.Printf("\n📦 Salida %d - Últimas %d lecturas:", salidaID, len(lecturas))
		for idx, lect := range lecturas {
			// Verificar que el correlativo termina en 1
			terminaEn1 := lect.Correlativo%10 == 1
			checkMark := "✅"
			if !terminaEn1 {
				checkMark = "⚠️"
			}

			log.Printf("    [%2d] Caja: %d | Correlativo: %d %s | %s",
				idx+1,
				lect.NumeroCaja,
				lect.Correlativo,
				checkMark,
				lect.FechaLectura.Format("15:04:05"))
		}
	}

	// Estadísticas finales
	log.Println("")
	log.Println("─────────────────────────────────────────────────────────────")
	log.Println("📊 Estadísticas Finales por Salida")
	log.Println("─────────────────────────────────────────────────────────────")

	for _, salidaID := range []int{1, 7} {
		contador, err := fx6Manager.GetContadorActual(ctx, salidaID)
		if err != nil {
			log.Printf("❌ Error consultando contador Salida %d: %v", salidaID, err)
			continue
		}

		log.Printf("Salida %d → Última caja registrada: #%d", salidaID, contador)
	}

	log.Println("")
	log.Println("═══════════════════════════════════════════════════════════════════════════════")
	log.Println("✅ Test de inserts REALES completado exitosamente")
	log.Println("")
	log.Println("💡 Lógica implementada:")
	log.Printf("   • Correlativo: ÚNICO número %d (termina en 1) para todos los inserts", correlativoUnico)
	log.Println("   • Número de Caja: 184 números diferentes (20000001 a 20000184)")
	log.Println("   • Base de datos: PKG_Pallets_Externos en FX6_packing_Garate_Operaciones")
}
