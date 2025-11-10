package main

import (
	"context"
	"log"
	"time"

	"api-dansort/internal/config"
	"api-dansort/internal/db"
)

func main() {
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds)
	log.Println("🧪 Test: Flujo completo DataMatrix (Cognex → Sorter → Salida → FX6)")
	log.Println("")

	ctx := context.Background()

	// Cargar configuración
	cfg, err := config.LoadConfig("/home/arbaiter/Documents/Arbeit/2025/API-Greenex/bin/config/config.yaml")
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

	// Test 1: Insertar lecturas DataMatrix
	// IMPORTANTE: Correlativo = Número de Caja (son el mismo valor)
	log.Println("📝 Test 1: Insertar 3 lecturas DataMatrix")
	log.Println("          (Correlativo = Número de Caja)")
	log.Println("─────────────────────────────────────────")

	tests := []struct {
		salida      int
		correlativo int64
	}{
		{1, 50001}, // Salida 1, Correlativo/Caja 50001
		{1, 50002}, // Salida 1, Correlativo/Caja 50002
		{7, 60001}, // Salida 7, Correlativo/Caja 60001
	}

	for i, test := range tests {
		// El número de caja ES el correlativo
		numeroCaja := int(test.correlativo)

		log.Printf("  [%d/%d] Insertando: Salida=%d, Correlativo=%d (Caja=%d)",
			i+1, len(tests), test.salida, test.correlativo, numeroCaja)

		err := fx6Manager.InsertLecturaDataMatrix(
			ctx,
			test.salida,
			test.correlativo,
			numeroCaja,
			time.Now(),
		)

		if err != nil {
			log.Printf("    ❌ Error: %v", err)
		} else {
			log.Printf("    ✅ Insertado correctamente")
		}
		time.Sleep(100 * time.Millisecond)
	}

	log.Println("")

	// Test 2: Consultar lecturas recientes por salida
	log.Println("📚 Test 2: Consultar lecturas recientes")
	log.Println("─────────────────────────────────────────")

	for _, salidaID := range []int{1, 7} {
		lecturas, err := fx6Manager.GetLecturasRecientes(ctx, salidaID, 10)
		if err != nil {
			log.Printf("❌ Error consultando Salida %d: %v", salidaID, err)
			continue
		}

		log.Printf("📦 Salida %d - Últimas %d lecturas:", salidaID, len(lecturas))
		for _, lect := range lecturas {
			log.Printf("    • Caja #%d | Correlativo: %d | Fecha: %s",
				lect.NumeroCaja, lect.Correlativo, lect.FechaLectura.Format("15:04:05"))
		}
	}

	log.Println("")

	// Test 3: Consultar contador actual
	log.Println("🔢 Test 3: Consultar contador de cajas actual")
	log.Println("─────────────────────────────────────────")

	for _, salidaID := range []int{1, 7} {
		contador, err := fx6Manager.GetContadorActual(ctx, salidaID)
		if err != nil {
			log.Printf("❌ Error consultando contador Salida %d: %v", salidaID, err)
			continue
		}

		log.Printf("📊 Salida %d → Última caja registrada: #%d", salidaID, contador)
	}

	log.Println("")
	log.Println("✅ Test completado exitosamente")
}
