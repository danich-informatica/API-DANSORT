package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"api-dansort/internal/config"
	"api-dansort/internal/db"

	"github.com/joho/godotenv"
)

func main() {
	log.SetFlags(log.Ldate | log.Ltime)
	log.Println("🔍 Diagnóstico de Tablas en FX6...")
	log.Println("")

	godotenv.Load()
	configPath := os.Getenv("CONFIG_FILE")
	if configPath == "" {
		configPath = "config/config.yaml"
	}

	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		log.Fatalf("❌ Error al cargar config: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	fx6Manager, err := db.NewFX6Manager(ctx, cfg.Database.SQLServer)
	if err != nil {
		log.Fatalf("❌ Error al conectar FX6: %v", err)
	}
	defer fx6Manager.Close()

	log.Println("✅ Conectado a FX6_packing_Garate_Operaciones")
	log.Println("")

	// Listar todas las tablas de la base de datos
	query := `
		SELECT 
			TABLE_SCHEMA,
			TABLE_NAME
		FROM INFORMATION_SCHEMA.TABLES
		WHERE TABLE_TYPE = 'BASE TABLE'
		AND TABLE_NAME LIKE '%datamatrix%'
		ORDER BY TABLE_SCHEMA, TABLE_NAME
	`

	log.Println("📋 Buscando tablas que contengan 'datamatrix'...")
	rows, err := fx6Manager.Query(ctx, query)
	if err != nil {
		log.Fatalf("❌ Error al consultar: %v", err)
	}
	defer rows.Close()

	found := false
	for rows.Next() {
		var schema, tableName string
		if err := rows.Scan(&schema, &tableName); err != nil {
			log.Printf("❌ Error al leer fila: %v", err)
			continue
		}
		log.Printf("  ✅ Encontrada: %s.%s", schema, tableName)
		found = true
	}

	if !found {
		log.Println("  ⚠️  No se encontraron tablas con 'datamatrix'")
		log.Println("")
		log.Println("📋 Listando TODAS las tablas de la base de datos:")

		queryAll := `
			SELECT TOP 50
				TABLE_SCHEMA,
				TABLE_NAME
			FROM INFORMATION_SCHEMA.TABLES
			WHERE TABLE_TYPE = 'BASE TABLE'
			ORDER BY TABLE_SCHEMA, TABLE_NAME
		`

		rowsAll, err := fx6Manager.Query(ctx, queryAll)
		if err != nil {
			log.Fatalf("❌ Error al consultar todas las tablas: %v", err)
		}
		defer rowsAll.Close()

		for rowsAll.Next() {
			var schema, tableName string
			if err := rowsAll.Scan(&schema, &tableName); err != nil {
				continue
			}
			log.Printf("  • %s.%s", schema, tableName)
		}
	} else {
		log.Println("")
		log.Println("📝 Verificando estructura de la tabla encontrada...")

		queryColumns := `
			SELECT 
				COLUMN_NAME,
				DATA_TYPE,
				IS_NULLABLE,
				CHARACTER_MAXIMUM_LENGTH
			FROM INFORMATION_SCHEMA.COLUMNS
			WHERE TABLE_SCHEMA = 'dbo'
			  AND TABLE_NAME LIKE '%datamatrix%'
			ORDER BY ORDINAL_POSITION
		`

		rowsCols, err := fx6Manager.Query(ctx, queryColumns)
		if err != nil {
			log.Printf("❌ Error al consultar columnas: %v", err)
		} else {
			defer rowsCols.Close()
			log.Println("")
			log.Println("Columnas encontradas:")
			for rowsCols.Next() {
				var colName, dataType, nullable string
				var maxLen *int
				if err := rowsCols.Scan(&colName, &dataType, &nullable, &maxLen); err != nil {
					continue
				}
				lenStr := ""
				if maxLen != nil {
					lenStr = fmt.Sprintf("(%d)", *maxLen)
				}
				log.Printf("  • %s: %s%s (NULL=%s)", colName, dataType, lenStr, nullable)
			}
		}
	}

	log.Println("")
	log.Println("🎉 Diagnóstico completado")
}
