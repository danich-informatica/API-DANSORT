package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

const baseURL = "http://localhost:8080"

func main() {
	log.Println("Iniciando cliente de prueba HTTP...")

	// Bucle infinito para probar los endpoints continuamente
	for {
		log.Println("--- Nuevo ciclo de pruebas ---")

		// Probar cada endpoint
		testPing()
		testMesaEstado("mesa-01")
		testEcho()
		testMesaPost("mesa-02")
		testMesaVaciar("mesa-03", "automatico")

		// Esperar antes del siguiente ciclo
		log.Println("--- Fin del ciclo, esperando 5 segundos ---")
		time.Sleep(5 * time.Second)
	}
}

// testPing prueba el endpoint GET /ping
func testPing() {
	log.Println("Probando: GET /ping")
	resp, err := http.Get(baseURL + "/ping")
	if err != nil {
		log.Printf("Error en /ping: %v\n", err)
		return
	}
	defer resp.Body.Close()
	printResponse(resp)
}

// testMesaEstado prueba el endpoint GET /Mesa/Estado
func testMesaEstado(id string) {
	url := fmt.Sprintf("%s/Mesa/Estado?id=%s", baseURL, id)
	log.Printf("Probando: GET %s", url)
	resp, err := http.Get(url)
	if err != nil {
		log.Printf("Error en /Mesa/Estado: %v\n", err)
		return
	}
	defer resp.Body.Close()
	printResponse(resp)
}

// testEcho prueba el endpoint POST /echo
func testEcho() {
	log.Println("Probando: POST /echo")
	jsonData := `{"clave": "valor", "numero": 123}`
	resp, err := http.Post(baseURL+"/echo", "application/json", bytes.NewBufferString(jsonData))
	if err != nil {
		log.Printf("Error en /echo: %v\n", err)
		return
	}
	defer resp.Body.Close()
	printResponse(resp)
}

// testMesaPost prueba el endpoint POST /Mesa
func testMesaPost(id string) {
	url := fmt.Sprintf("%s/Mesa?id=%s", baseURL, id)
	log.Printf("Probando: POST %s", url)
	jsonData := `{"producto": "caja_grande", "cantidad": 5}`
	resp, err := http.Post(url, "application/json", bytes.NewBufferString(jsonData))
	if err != nil {
		log.Printf("Error en /Mesa: %v\n", err)
		return
	}
	defer resp.Body.Close()
	printResponse(resp)
}

// testMesaVaciar prueba el endpoint POST /Mesa/Vaciar
func testMesaVaciar(id, modo string) {
	url := fmt.Sprintf("%s/Mesa/Vaciar?id=%s&modo=%s", baseURL, id, modo)
	log.Printf("Probando: POST %s", url)
	jsonData := `{"motivo": "fin_de_turno"}`
	resp, err := http.Post(url, "application/json", bytes.NewBufferString(jsonData))
	if err != nil {
		log.Printf("Error en /Mesa/Vaciar: %v\n", err)
		return
	}
	defer resp.Body.Close()
	printResponse(resp)
}

// printResponse imprime el código de estado y el cuerpo de la respuesta
func printResponse(resp *http.Response) {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Error al leer la respuesta: %v\n", err)
		return
	}
	log.Printf("Respuesta: %s | Cuerpo: %s\n", resp.Status, string(body))
}
