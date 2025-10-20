package main

import (
	"context"
	"log"
	"os"
	"time"

	"API-GREENEX/internal/config"
	"API-GREENEX/internal/db"

	"github.com/joho/godotenv"
)

func main() {
	log.SetFlags(log.Ldate | log.Ltime)
	log.Println("🔧 Creando tabla lectura_datamatrix en FX6...")
	log.Println("")

	// Cargar .env y config
	godotenv.Load()
	configPath := os.Getenv("CONFIG_FILE")
	if configPath == "" {
		configPath = "config/config.yaml"
	}

	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		log.Fatalf("❌ Error al cargar config: %v", err)
	}

	// Conectar a FX6
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	fx6Manager, err := db.NewFX6Manager(ctx, cfg.Database.SQLServer)
	if err != nil {
		log.Fatalf("❌ Error al conectar FX6: %v", err)
	}
	defer fx6Manager.Close()

	log.Println("✅ Conectado a FX6")
	log.Println("")

	// Script SQL de creación
	createTableSQL := `
-- Eliminar si existe
IF OBJECT_ID('dbo.lectura_datamatrix', 'U') IS NOT NULL
BEGIN
    DROP TABLE dbo.lectura_datamatrix;
    PRINT '⚠️  Tabla anterior eliminada';
END

-- Crear tabla
CREATE TABLE dbo.lectura_datamatrix (
    Salida         INT          NOT NULL,
    Correlativo    BIGINT       NOT NULL,
    Numero_Caja    INT          NOT NULL,
    Fecha_Lectura  DATETIME     NOT NULL DEFAULT GETDATE(),
    Terminado      INT          NOT NULL DEFAULT 0,
    
    INDEX IX_Salida (Salida),
    INDEX IX_FechaLectura (Fecha_Lectura DESC),
    INDEX IX_Terminado (Terminado)
);

PRINT '✅ Tabla lectura_datamatrix creada exitosamente';
`

	log.Println("📝 Ejecutando script de creación...")
	_, err = fx6Manager.Exec(ctx, createTableSQL)
	if err != nil {
		log.Fatalf("❌ Error al crear tabla: %v", err)
	}

	log.Println("✅ Tabla creada exitosamente")
	log.Println("")

	// Insertar datos de prueba
	log.Println("📦 Insertando datos de prueba...")

	testInserts := []struct {
		salida      int
		correlativo int64
		numeroCaja  int
	}{
		{1, 1001, 1},
		{1, 1002, 2},
		{7, 2001, 1},
	}

	for _, test := range testInserts {
		err := fx6Manager.InsertLecturaDataMatrix(
			ctx,
			test.salida,
			test.correlativo,
			test.numeroCaja,
			time.Now(),
		)
		if err != nil {
			log.Printf("  ⚠️  Error insertando prueba: %v", err)
		} else {
			log.Printf("  ✅ Insertado: Salida=%d, Correlativo=%d, Caja=%d",
				test.salida, test.correlativo, test.numeroCaja)
		}
	}

	log.Println("")
	log.Println("🎉 Configuración completada")
	log.Println("")
	log.Println("📊 Puedes verificar con:")
	log.Println("   SELECT * FROM lectura_datamatrix")
}
