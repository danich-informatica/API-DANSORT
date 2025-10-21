package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/url"

	_ "github.com/microsoft/go-mssqldb"
)

func main() {
	// Configuración desde config.yaml
	host := "190.110.163.22"
	port := 1433
	user := "Danich"
	password := "yb*;NC&5r9U2"

	// Probar con URL encoding de la contraseña
	query := url.Values{}
	query.Add("database", "master")
	query.Add("encrypt", "disable")
	query.Add("TrustServerCertificate", "true")
	query.Add("connection timeout", "15")

	u := &url.URL{
		Scheme:   "sqlserver",
		User:     url.UserPassword(user, password),
		Host:     fmt.Sprintf("%s:%d", host, port),
		RawQuery: query.Encode(),
	}

	connString := u.String()

	fmt.Println("=== Probando con URL encoding ===")
	fmt.Printf("Usuario: %s\n", user)
	fmt.Printf("Host: %s:%d\n", host, port)

	db, err := sql.Open("sqlserver", connString)
	if err != nil {
		log.Fatalf("❌ Error abriendo conexión: %v\n", err)
	}
	defer db.Close()

	fmt.Println("⏳ Intentando conectar...")
	err = db.Ping()
	if err != nil {
		log.Fatalf("❌ Error en Ping: %v\n", err)
	}

	fmt.Println("✅ Conexión exitosa!")

	// Intentar una query simple
	var version string
	err = db.QueryRow("SELECT @@VERSION").Scan(&version)
	if err != nil {
		log.Fatalf("❌ Error ejecutando query: %v\n", err)
	}

	fmt.Printf("✅ SQL Server Version: %.100s...\n", version)

	// Listar bases de datos disponibles
	fmt.Println("\n=== Bases de datos disponibles ===")
	dbRows, err := db.Query("SELECT name FROM sys.databases WHERE name NOT IN ('master', 'tempdb', 'model', 'msdb') ORDER BY name")
	if err != nil {
		log.Printf("⚠️  Error listando bases de datos: %v\n", err)
	} else {
		defer dbRows.Close()
		var databases []string
		for dbRows.Next() {
			var dbName string
			if err := dbRows.Scan(&dbName); err == nil {
				databases = append(databases, dbName)
				fmt.Printf("  - %s\n", dbName)
			}
		}
		fmt.Printf("  ✅ Total: %d bases de datos\n\n", len(databases))

		// Buscar la vista en cada base de datos
		if len(databases) > 0 {
			fmt.Println("=== Buscando vista VW_INT_DANICH_ENVIVO en cada DB ===")
			for _, dbName := range databases {
				queryStr := fmt.Sprintf(`
					SELECT COUNT(*) 
					FROM [%s].sys.views 
					WHERE name = 'VW_INT_DANICH_ENVIVO'
				`, dbName)

				var count int
				err := db.QueryRow(queryStr).Scan(&count)
				if err != nil {
					fmt.Printf("  %s: ❌ Error: %v\n", dbName, err)
				} else if count > 0 {
					fmt.Printf("  %s: ✅ ¡VISTA ENCONTRADA!\n", dbName)

					// Probar acceso
					testQuery := fmt.Sprintf("SELECT TOP 3 * FROM [%s].dbo.VW_INT_DANICH_ENVIVO", dbName)
					rows, err := db.Query(testQuery)
					if err != nil {
						fmt.Printf("    ⚠️  Error accediendo: %v\n", err)
					} else {
						cols, _ := rows.Columns()
						fmt.Printf("    Columnas: %v\n", cols)
						rows.Close()
					}
				} else {
					fmt.Printf("  %s: ⚪ No encontrada\n", dbName)
				}
			}
		}
	}

	// Listar todas las vistas disponibles
	fmt.Println("\n=== Vistas disponibles ===")
	rows, err := db.Query(`
		SELECT 
			SCHEMA_NAME(schema_id) AS schema_name,
			name AS view_name
		FROM sys.views
		WHERE name LIKE '%DANICH%' OR name LIKE '%ENVIVO%' OR name LIKE '%VW%'
		ORDER BY schema_name, name
	`)
	if err != nil {
		log.Printf("⚠️  Error listando vistas: %v\n", err)
	} else {
		defer rows.Close()
		count := 0
		for rows.Next() {
			var schemaName, viewName string
			if err := rows.Scan(&schemaName, &viewName); err != nil {
				log.Printf("Error leyendo fila: %v\n", err)
				continue
			}
			fmt.Printf("  - %s.%s\n", schemaName, viewName)
			count++
		}
		if count == 0 {
			fmt.Println("  ⚠️  No se encontraron vistas con DANICH, ENVIVO o VW en el nombre")
		} else {
			fmt.Printf("  ✅ Total: %d vistas encontradas\n", count)
		}
	}

	// Intentar ver la vista con schema dbo
	fmt.Println("\n=== Probando acceso a vista ===")
	testViews := []string{
		"VW_INT_DANICH_ENVIVO",
		"dbo.VW_INT_DANICH_ENVIVO",
		"DANICH.VW_INT_DANICH_ENVIVO",
	}

	for _, viewName := range testViews {
		fmt.Printf("Probando: %s ... ", viewName)
		rows, err := db.Query(fmt.Sprintf("SELECT TOP 1 * FROM %s", viewName))
		if err != nil {
			fmt.Printf("❌ %v\n", err)
		} else {
			rows.Close()
			fmt.Printf("✅ ACCESIBLE!\n")

			// Mostrar columnas
			rows2, _ := db.Query(fmt.Sprintf("SELECT TOP 0 * FROM %s", viewName))
			if rows2 != nil {
				cols, _ := rows2.Columns()
				fmt.Printf("  Columnas: %v\n", cols)
				rows2.Close()
			}
			break
		}
	}
}
