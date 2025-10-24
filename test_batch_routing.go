package main

import "fmt"

// Simulación del nuevo algoritmo de batch routing

type BatchDistributor struct {
	CurrentIndex int
	CurrentCount int
}

func main() {
	// Simulación: 3 salidas con diferentes batch_sizes
	salidas := []struct {
		ID        int
		BatchSize int
	}{
		{ID: 1, BatchSize: 2}, // Salida 1: 2 cajas por batch
		{ID: 4, BatchSize: 3}, // Salida 4: 3 cajas por batch
		{ID: 7, BatchSize: 1}, // Salida 7: 1 caja por batch
	}

	bd := &BatchDistributor{
		CurrentIndex: 0,
		CurrentCount: 0,
	}

	fmt.Println("🧪 Test de Batch Routing con batch_size respetado")
	fmt.Println("═══════════════════════════════════════════════════")
	fmt.Printf("Salida 1: batch_size=%d\n", salidas[0].BatchSize)
	fmt.Printf("Salida 4: batch_size=%d\n", salidas[1].BatchSize)
	fmt.Printf("Salida 7: batch_size=%d\n", salidas[2].BatchSize)
	fmt.Println("═══════════════════════════════════════════════════")
	fmt.Println()

	// Simular 20 cajas
	for caja := 1; caja <= 20; caja++ {
		// Obtener salida actual
		idx := bd.CurrentIndex
		salida := salidas[idx]

		// Incrementar contador
		bd.CurrentCount++

		// Determinar si rotamos
		rotacion := ""
		if bd.CurrentCount >= salida.BatchSize {
			bd.CurrentIndex = (idx + 1) % len(salidas)
			bd.CurrentCount = 0
			rotacion = fmt.Sprintf("→ ROTACIÓN (batch completado: %d/%d)", 
				salida.BatchSize, salida.BatchSize)
		} else {
			rotacion = fmt.Sprintf("(batch parcial: %d/%d)", 
				bd.CurrentCount, salida.BatchSize)
		}

		fmt.Printf("Caja #%2d → Salida %d %s\n", caja, salida.ID, rotacion)
	}

	fmt.Println()
	fmt.Println("✅ Patrón esperado:")
	fmt.Println("   Cajas 1-2   → Salida 1 (batch_size=2)")
	fmt.Println("   Cajas 3-5   → Salida 4 (batch_size=3)")
	fmt.Println("   Caja 6      → Salida 7 (batch_size=1)")
	fmt.Println("   Cajas 7-8   → Salida 1 (batch_size=2)")
	fmt.Println("   Cajas 9-11  → Salida 4 (batch_size=3)")
	fmt.Println("   Caja 12     → Salida 7 (batch_size=1)")
	fmt.Println("   ... y así sucesivamente")
}
