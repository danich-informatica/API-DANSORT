package main

import (
	"database/sql"
	"fmt"
	"log"

	_ "github.com/microsoft/go-mssqldb"
)

func main() {
	// Configuración desde config.yaml
	host := "190.110.163.22"
	port := 1433
	user := "Danich"
	password := "yb*;NC&5r9U2"

	// Probar varias combinaciones de DSN
	dsns := []string{
		// Opción 1: Con encrypt=disable
		fmt.Sprintf("server=%s;port=%d;user id=%s;password=%s;encrypt=disable;TrustServerCertificate=true;connection timeout=15",
			host, port, user, password),

		// Opción 2: Sin encrypt
		fmt.Sprintf("server=%s;port=%d;user id=%s;password=%s;TrustServerCertificate=true;connection timeout=15",
			host, port, user, password),

		// Opción 3: Con database master
		fmt.Sprintf("server=%s;port=%d;user id=%s;password=%s;database=master;encrypt=disable;TrustServerCertificate=true;connection timeout=15",
			host, port, user, password),
	}

	for i, dsn := range dsns {
		fmt.Printf("\n=== Prueba %d ===\n", i+1)
		safeDSN := fmt.Sprintf("server=%s;port=%d;user id=%s;password=***;...", host, port, user)
		fmt.Printf("DSN: %s\n", safeDSN)

		db, err := sql.Open("sqlserver", dsn)
		if err != nil {
			log.Printf("❌ Error abriendo conexión: %v\n", err)
			continue
		}
		defer db.Close()

		err = db.Ping()
		if err != nil {
			log.Printf("❌ Error en Ping: %v\n", err)
			continue
		}

		fmt.Println("✅ Conexión exitosa!")

		// Intentar una query simple
		var version string
		err = db.QueryRow("SELECT @@VERSION").Scan(&version)
		if err != nil {
			log.Printf("❌ Error ejecutando query: %v\n", err)
			continue
		}

		fmt.Printf("✅ SQL Server Version: %.100s...\n", version)

		// Intentar ver la vista
		rows, err := db.Query("SELECT TOP 1 * FROM VW_INT_DANICH_ENVIVO")
		if err != nil {
			log.Printf("⚠️  Error accediendo a vista VW_INT_DANICH_ENVIVO: %v\n", err)
		} else {
			rows.Close()
			fmt.Println("✅ Vista VW_INT_DANICH_ENVIVO accesible!")
		}

		break
	}
}
