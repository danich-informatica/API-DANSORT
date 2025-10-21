package main

import (
	"fmt"
	"log"
	"math/rand"
	"time"
)

func main() {
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds)
	log.Println("🧪 Test SIMULADO: Inserts DataMatrix")
	log.Println("    Correlativos: Números aleatorios terminados en 1")
	log.Println("    Números de Caja: De la lista específica")
	log.Println("")

	// Lista de números de caja válidos (según especificación)
	numerosCaja := []int{
		20000058, 20000059, 20000060, 20000061, 20000062,
		20000063, 20000064, 20000065, 20000066, 20000067,
		20000068, 20000069, 20000070, 20000071, 20000072,
		20000073, 20000074, 20000075, 20000076, 20000077,
		20000078,
	}

	log.Println("📦 Números de caja disponibles (21 total):")
	log.Printf("    Desde: %d", numerosCaja[0])
	log.Printf("    Hasta: %d", numerosCaja[len(numerosCaja)-1])
	log.Println("")

	// Generar correlativo aleatorio que termine en 1
	generarCorrelativo := func() int64 {
		// Generar número aleatorio entre 10000 y 99999
		base := rand.Intn(90000) + 10000
		// Asegurar que termine en 1
		correlativo := (base / 10 * 10) + 1
		return int64(correlativo)
	}

	// Verificar que termina en 1
	verificarCorrelativo := func(correlativo int64) string {
		if correlativo%10 == 1 {
			return "✅ Termina en 1"
		}
		return "❌ NO termina en 1"
	}

	// Simulación: Insertar 15 lecturas de prueba
	log.Println("📝 SIMULACIÓN: 15 inserts de prueba")
	log.Println("═══════════════════════════════════════════════════════════════════════════════")

	salidas := []int{1, 1, 7, 1, 7, 1, 7, 1, 1, 7, 1, 7, 1, 7, 1} // Alternando salidas

	for i := 0; i < 15; i++ {
		salida := salidas[i]

		// Generar correlativo que termine en 1
		correlativo := generarCorrelativo()

		// Seleccionar número de caja de la lista (rotación)
		numeroCaja := numerosCaja[i%len(numerosCaja)]

		fechaLectura := time.Now()

		// Simular el SQL que se ejecutaría
		sqlSimulado := fmt.Sprintf(
			"INSERT INTO PKG_Pallets_Externos (Salida, Correlativo, Numero_Caja, Fecha_Lectura, Terminado) VALUES (%d, %d, %d, '%s', 0)",
			salida,
			correlativo,
			numeroCaja,
			fechaLectura.Format("2006-01-02 15:04:05"),
		)

		log.Printf("\n[%2d/15] Salida=%d", i+1, salida)
		log.Printf("        Correlativo: %d %s", correlativo, verificarCorrelativo(correlativo))
		log.Printf("        Número Caja: %d", numeroCaja)
		log.Printf("        SQL: %s", sqlSimulado)
		log.Printf("        ✅ Insertado correctamente (simulado)")

		time.Sleep(100 * time.Millisecond)
	}

	log.Println("")
	log.Println("═══════════════════════════════════════════════════════════════════════════════")
	log.Println("")

	// Resumen
	log.Println("📊 Resumen de la prueba:")
	log.Println("─────────────────────────────────────────────────────────────")
	log.Printf("✅ Total de inserts simulados: 15")
	log.Printf("✅ Correlativos generados: Todos terminan en 1")
	log.Printf("✅ Números de caja usados: %d a %d (primeros 15 de la lista)",
		numerosCaja[0], numerosCaja[14])
	log.Printf("✅ Salidas usadas: 1 y 7")
	log.Println("")

	// Mostrar ejemplos de validación
	log.Println("🔍 Ejemplos de validación:")
	log.Println("─────────────────────────────────────────────────────────────")

	ejemplos := []int64{12341, 99991, 54321, 11111, 87651}
	for _, ej := range ejemplos {
		log.Printf("  Correlativo %d → %s", ej, verificarCorrelativo(ej))
	}

	log.Println("")
	log.Println("💡 Lógica en producción:")
	log.Println("─────────────────────────────────────────────────────────────")
	log.Println("  1. Cognex lee DataMatrix")
	log.Println("  2. Se genera/extrae Correlativo (debe terminar en 1)")
	log.Println("  3. Se asigna Número de Caja de la lista disponible")
	log.Println("  4. INSERT en FX6: PKG_Pallets_Externos")
	log.Println("  5. WebSocket notifica al frontend")
	log.Println("")

	log.Println("✅ Test de simulación completado exitosamente")
}
