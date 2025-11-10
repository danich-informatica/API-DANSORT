package main

import (
"context"
"log"
"time"

"api-dansort/internal/communication/plc"
"api-dansort/internal/config"
)

func main() {
log.Println("🔧 Test de AssignLaneToBox (estilo Rust)")

cfg, err := config.LoadConfig("config/config.yaml")
if err != nil {
log.Fatalf("❌ Error cargando config: %v", err)
}

manager := plc.NewManager(cfg)
ctx := context.Background()

if err := manager.ConnectAll(ctx); err != nil {
log.Fatalf("❌ Error conectando: %v", err)
}

log.Println("✅ Conexión establecida\n")

log.Println("═══════════════════════════════════════")
log.Println("TEST: Asignar caja a salida 5")
log.Println("═══════════════════════════════════════")

err = manager.AssignLaneToBox(context.Background(), 1, 5)
if err != nil {
log.Printf("❌ Error final: %v\n", err)
}

time.Sleep(1 * time.Second)
log.Println("\n✅ Test completado")
}
