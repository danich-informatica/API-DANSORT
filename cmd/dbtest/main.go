package main

import (
	"context"
	"database/sql"
	"log"
	"time"

	"api-dansort/internal/db"
	"api-dansort/internal/models"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// Inicializar conexión a SQL Server
	manager, err := db.GetManager(ctx)
	if err != nil {
		log.Fatalf("db: no fue posible inicializar el pool de SQL Server: %v", err)
	}
	defer manager.Close()

	if err := manager.Ping(ctx); err != nil {
		log.Fatalf("db: ping a SQL Server fallido: %v", err)
	}
	log.Println("db: conexión a SQL Server verificada correctamente")

	// Inicializar conexión a PostgreSQL
	pgManager, err := db.GetPostgresManager(ctx)
	if err != nil {
		log.Fatalf("db: no fue posible inicializar el pool de PostgreSQL: %v", err)
	}
	defer pgManager.Close()

	log.Println("db: conexión a PostgreSQL verificada correctamente")

	// Obtener datos de SQL Server
	rows, err := db.FetchSegregazioneProgramma(ctx, manager)
	if err != nil {
		log.Fatalf("db: error al ejecutar la consulta: %v", err)
	}

	if len(rows) == 0 {
		log.Println("db: la consulta no devolvió resultados")
		return
	}

	log.Printf("db: se obtuvieron %d registros de SQL Server", len(rows))

	// Preparar SKUs para inserción en batch
	var skuList []struct {
		Calibre  string
		Variedad string
		Embalaje string
	}

	for idx, row := range rows {
		// Generar SKU para validación y logging
		skuObj, err := models.RequestSKU(nullToString(row.Variedad), nullToString(row.Calibre), nullToString(row.Embalaje))
		if err != nil {
			log.Printf("fila %d: sku inválido: %v", idx+1, err)
			continue
		}
		log.Printf("SKU generada: %s", skuObj.SKU)

		// Agregar a la lista para inserción
		skuList = append(skuList, struct {
			Calibre  string
			Variedad string
			Embalaje string
		}{
			Calibre:  skuObj.Calibre,
			Variedad: skuObj.Variedad,
			Embalaje: skuObj.Embalaje,
		})
	}

	// Insertar SKUs en PostgreSQL
	log.Println("\n=== Iniciando inserción de SKUs en PostgreSQL ===")
	inserted, skipped, errors := pgManager.InsertSKUsBatch(ctx, skuList)

	// Mostrar resumen
	log.Println("\n=== Resumen de Inserción ===")
	log.Printf("✅ SKUs insertadas correctamente: %d", inserted)
	log.Printf("⚠️  SKUs omitidas (nulas/inválidas/duplicadas): %d", skipped)
	log.Printf("❌ Errores durante inserción: %d", len(errors.([]error)))

	errList, ok := errors.([]error)
	if ok && len(errList) > 0 {
		log.Println("\n=== Detalle de Errores ===")
		for _, err := range errList {
			log.Println(err)
		}
	}

	log.Println("\n✅ Proceso completado")
}

func nullToString(v sql.NullString) string {
	if v.Valid {
		return v.String
	}
	return "(NULL)"
}
