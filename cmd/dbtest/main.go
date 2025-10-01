package main

import (
	"context"
	"database/sql"
	"log"
	"time"

	"API-GREENEX/internal/db"
	"API-GREENEX/internal/models"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	manager, err := db.GetManager(ctx)
	if err != nil {
		log.Fatalf("db: no fue posible inicializar el pool: %v", err)
	}
	defer manager.Close()

	if err := manager.Ping(ctx); err != nil {
		log.Fatalf("db: ping fallido: %v", err)
	}
	log.Println("db: conexión verificada correctamente")

	rows, err := db.FetchSegregazioneProgramma(ctx, manager)
	if err != nil {
		log.Fatalf("db: error al ejecutar la consulta: %v", err)
	}

	if len(rows) == 0 {
		log.Println("db: la consulta no devolvió resultados")
		return
	}

	for idx, row := range rows {
		skuObj, err := models.RequestSKU(nullToString(row.Variedad), nullToString(row.Calibre), nullToString(row.Embalaje))
		if err != nil {
			log.Printf("fila %d: sku inválido: %v", idx, err)
			continue
		}
		log.Printf("SKU: %s", skuObj.SKU)
	}
}

func nullToString(v sql.NullString) string {
	if v.Valid {
		return v.String
	}
	return "(NULL)"
}
