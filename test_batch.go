package main

import "fmt"

type BatchDistributor struct {
	Salidas      []int
	CurrentIndex int
	CurrentCount int
	BatchSizes   map[int]int
}

func (bd *BatchDistributor) NextSalida() int {
	if len(bd.Salidas) == 0 {
		return -1
	}

	if len(bd.Salidas) == 1 {
		return bd.Salidas[0]
	}

	// Obtener salida actual
	salidaID := bd.Salidas[bd.CurrentIndex]
	batchSize := bd.BatchSizes[salidaID]

	// Incrementar contador
	bd.CurrentCount++

	// Si alcanzamos el batch size, avanzar al siguiente
	if bd.CurrentCount >= batchSize {
		bd.CurrentIndex = (bd.CurrentIndex + 1) % len(bd.Salidas)
		bd.CurrentCount = 0
	}

	return salidaID
}

func main() {
	bd := &BatchDistributor{
		Salidas:      []int{7, 4, 1},
		CurrentIndex: 0,
		CurrentCount: 0,
		BatchSizes:   map[int]int{7: 1, 4: 1, 1: 1},
	}

	fmt.Println("Simulando 10 cajas:")
	for i := 1; i <= 10; i++ {
		salida := bd.NextSalida()
		fmt.Printf("Caja %d → Salida %d (Index: %d, Count: %d)\n",
			i, salida, bd.CurrentIndex, bd.CurrentCount)
	}
}
