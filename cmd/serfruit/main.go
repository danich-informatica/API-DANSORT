package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"
)

// EstadoMesa representa el estado de una mesa de paletizado
type EstadoMesa struct {
	IDMesa               int    `json:"idMesa"`
	Estado               int    `json:"estado"` // 1 = libre, 2 = bloqueado
	DescripcionEstado    string `json:"descripcionEstado"`
	EstadoPLC            int    `json:"estadoPLC"` // 1-5
	DescripcionEstadoPLC string `json:"descripcionEstadoPLC"`
	NumeroCajaPale       int    `json:"numeroCajaPale"`
	NumeroPaleActual     int    `json:"numeroPaleActual"`
	CodigoTipoCaja       string `json:"codigoTipoCaja"`
	CodigoTipoPale       string `json:"codigoTipoPale"`
	CajasPerCapa         int    `json:"cajasPerCapa"`
	NumeroPales          int    `json:"numeroPales"`
}

// OrdenFabricacion representa una orden activa
type OrdenFabricacion struct {
	NumeroPales       int      `json:"numeroPales"`
	CajasPerPale      int      `json:"cajasPerPale"`
	CajasPerCapa      int      `json:"cajasPerCapa"`
	CodigoEnvase      string   `json:"codigoEnvase"`
	CodigoPale        string   `json:"codigoPale"`
	IDProgramaFlejado int      `json:"idProgramaFlejado"`
	CajasRegistradas  []string `json:"-"`
	PaleActual        int      `json:"-"`
	CajasEnPale       int      `json:"-"`
}

// ServerState mantiene el estado del servidor dummy
type ServerState struct {
	mu      sync.RWMutex
	mesas   map[int]*EstadoMesa
	ordenes map[int]*OrdenFabricacion
}

var state = &ServerState{
	mesas:   make(map[int]*EstadoMesa),
	ordenes: make(map[int]*OrdenFabricacion),
}

func init() {
	// Inicializar 3 mesas de ejemplo
	for i := 1; i <= 3; i++ {
		state.mesas[i] = &EstadoMesa{
			IDMesa:               i,
			Estado:               1, // Libre
			DescripcionEstado:    "Libre",
			EstadoPLC:            3, // Automático
			DescripcionEstadoPLC: "Automático",
			NumeroCajaPale:       0,
			NumeroPaleActual:     0,
			CodigoTipoCaja:       "",
			CodigoTipoPale:       "",
			CajasPerCapa:         0,
			NumeroPales:          0,
		}
	}
}

func main() {
	mux := http.NewServeMux()

	// Endpoints
	mux.HandleFunc("/Mesa/Estado", handleGetEstado)
	mux.HandleFunc("/Mesa/NuevaCaja", handlePostNuevaCaja)
	mux.HandleFunc("/Mesa", handlePostOrden)
	mux.HandleFunc("/Mesa/Vaciar", handlePostVaciar)

	// Root endpoint
	mux.HandleFunc("/", handleRoot)

	addr := ":9093"
	log.Printf("🚀 Servidor dummy API Paletizado Serfruit iniciado en http://localhost%s", addr)
	log.Printf("📋 Endpoints disponibles:")
	log.Printf("   GET  /Mesa/Estado?id={id}")
	log.Printf("   POST /Mesa/NuevaCaja?idMesa={id}")
	log.Printf("   POST /Mesa?id={id}")
	log.Printf("   POST /Mesa/Vaciar?id={id}&modo={1|2}")
	log.Printf("")

	if err := http.ListenAndServe(addr, loggingMiddleware(mux)); err != nil {
		log.Fatalf("Error iniciando servidor: %v", err)
	}
}

// Middleware para logging
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		log.Printf("➡️  %s %s %s", r.Method, r.URL.Path, r.URL.RawQuery)
		next.ServeHTTP(w, r)
		log.Printf("⬅️  %s %s - %v", r.Method, r.URL.Path, time.Since(start))
	})
}

func handleRoot(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	html := `<!DOCTYPE html>
<html>
<head>
    <title>API Paletizado Serfruit - Dummy Server</title>
    <style>
        body { font-family: Arial, sans-serif; max-width: 800px; margin: 50px auto; padding: 20px; }
        h1 { color: #2c3e50; }
        .endpoint { background: #f8f9fa; padding: 15px; margin: 10px 0; border-radius: 5px; }
        .method { display: inline-block; padding: 5px 10px; border-radius: 3px; font-weight: bold; }
        .get { background: #61affe; color: white; }
        .post { background: #49cc90; color: white; }
        code { background: #e9ecef; padding: 2px 6px; border-radius: 3px; }
    </style>
</head>
<body>
    <h1>🏭 API Paletizado Serfruit - Servidor Dummy</h1>
    <p>Servidor de prueba para desarrollo del cliente HTTP</p>
    
    <h2>Endpoints Disponibles:</h2>
    
    <div class="endpoint">
        <span class="method get">GET</span>
        <code>/Mesa/Estado?id={id}</code>
        <p>Consultar estado de mesa(s). id=0 para todas.</p>
    </div>
    
    <div class="endpoint">
        <span class="method post">POST</span>
        <code>/Mesa/NuevaCaja?idMesa={id}</code>
        <p>Registrar nueva caja. Body: <code>{"idCaja": "código"}</code></p>
    </div>
    
    <div class="endpoint">
        <span class="method post">POST</span>
        <code>/Mesa?id={id}</code>
        <p>Crear orden de fabricación.</p>
    </div>
    
    <div class="endpoint">
        <span class="method post">POST</span>
        <code>/Mesa/Vaciar?id={id}&modo={1|2}</code>
        <p>Vaciar mesa. Modo 1: Continuar, Modo 2: Finalizar</p>
    </div>
    
    <h3>Estado Actual:</h3>
    <p>Mesas activas: 3 (IDs: 1, 2, 3)</p>
</body>
</html>`

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, html)
}

// GET /Mesa/Estado?id={id}
func handleGetEstado(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Método no permitido", http.StatusMethodNotAllowed)
		return
	}

	idStr := r.URL.Query().Get("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{
			"error": "ID inválido",
		})
		return
	}

	state.mu.RLock()
	defer state.mu.RUnlock()

	// id=0 significa todas las mesas
	if id == 0 {
		mesas := make([]EstadoMesa, 0, len(state.mesas))
		for _, mesa := range state.mesas {
			mesas = append(mesas, *mesa)
		}
		respondJSON(w, http.StatusOK, mesas)
		return
	}

	// Mesa específica
	mesa, exists := state.mesas[id]
	if !exists {
		respondJSON(w, http.StatusNotFound, map[string]string{
			"error": "Mesa no encontrada",
		})
		return
	}

	// Si la mesa no tiene orden activa, devolver 202
	if mesa.Estado == 1 {
		respondJSON(w, 202, map[string]string{
			"message": "Mesa NO tiene OF activa",
		})
		return
	}

	respondJSON(w, http.StatusOK, mesa)
}

// POST /Mesa/NuevaCaja?idMesa={id}
func handlePostNuevaCaja(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Método no permitido", http.StatusMethodNotAllowed)
		return
	}

	idStr := r.URL.Query().Get("idMesa")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{
			"error": "ID de mesa inválido",
		})
		return
	}

	var req struct {
		IDCaja string `json:"idCaja"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{
			"error": "El formato de la petición enviada es incorrecto. Caja NO registrada",
		})
		return
	}

	state.mu.Lock()
	defer state.mu.Unlock()

	mesa, exists := state.mesas[id]
	if !exists {
		respondJSON(w, http.StatusNotFound, map[string]string{
			"error": "Mesa no encontrada",
		})
		return
	}

	// Verificar que hay orden activa
	orden, tieneOrden := state.ordenes[id]
	if !tieneOrden || mesa.Estado == 1 {
		respondJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "No se ha podido registrar la caja en la mesa",
		})
		return
	}

	// Verificar duplicados
	for _, caja := range orden.CajasRegistradas {
		if caja == req.IDCaja {
			respondJSON(w, 202, map[string]string{
				"message": "Lectura duplicada. La caja ya existe",
			})
			return
		}
	}

	// Registrar caja
	orden.CajasRegistradas = append(orden.CajasRegistradas, req.IDCaja)
	orden.CajasEnPale++

	// Si se completó el palé actual, avanzar al siguiente
	if orden.CajasEnPale >= orden.CajasPerPale {
		orden.PaleActual++
		orden.CajasEnPale = 0
		mesa.NumeroPaleActual = orden.PaleActual
		log.Printf("✅ Palé %d/%d completado en mesa %d", orden.PaleActual, orden.NumeroPales, id)
	}

	log.Printf("📦 Caja registrada: %s (Mesa %d, Palé %d, Caja %d/%d)",
		req.IDCaja, id, orden.PaleActual+1, orden.CajasEnPale, orden.CajasPerPale)

	respondJSON(w, http.StatusOK, map[string]string{
		"message": "La caja ha sido registrada correctamente en la mesa",
	})
}

// POST /Mesa?id={id}
func handlePostOrden(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Método no permitido", http.StatusMethodNotAllowed)
		return
	}

	idStr := r.URL.Query().Get("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{
			"error": "ID de mesa inválido",
		})
		return
	}

	var orden OrdenFabricacion
	if err := json.NewDecoder(r.Body).Decode(&orden); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{
			"error": "El formato de la orden enviada es incorrecto",
		})
		return
	}

	state.mu.Lock()
	defer state.mu.Unlock()

	mesa, exists := state.mesas[id]
	if !exists {
		respondJSON(w, http.StatusNotFound, map[string]string{
			"error": "Mesa, envase o palé no encontrados",
		})
		return
	}

	// Verificar que la mesa esté libre
	if mesa.Estado == 2 {
		respondJSON(w, 209, map[string]string{
			"message": "La mesa no está disponible para iniciar una orden",
		})
		return
	}

	// Crear orden
	orden.CajasRegistradas = make([]string, 0)
	orden.PaleActual = 0
	orden.CajasEnPale = 0
	state.ordenes[id] = &orden

	// Actualizar estado de la mesa
	mesa.Estado = 2
	mesa.DescripcionEstado = "Bloqueado (orden activa)"
	mesa.NumeroCajaPale = orden.CajasPerPale
	mesa.NumeroPaleActual = 0
	mesa.CodigoTipoCaja = orden.CodigoEnvase
	mesa.CodigoTipoPale = orden.CodigoPale
	mesa.CajasPerCapa = orden.CajasPerCapa
	mesa.NumeroPales = orden.NumeroPales

	log.Printf("📋 Orden creada en mesa %d: %d palés × %d cajas", id, orden.NumeroPales, orden.CajasPerPale)

	respondJSON(w, http.StatusOK, map[string]string{
		"message": "Orden creada exitosamente",
	})
}

// POST /Mesa/Vaciar?id={id}&modo={1|2}
func handlePostVaciar(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Método no permitido", http.StatusMethodNotAllowed)
		return
	}

	idStr := r.URL.Query().Get("id")
	modoStr := r.URL.Query().Get("modo")

	id, err := strconv.Atoi(idStr)
	if err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{
			"error": "ID de mesa inválido",
		})
		return
	}

	modo, err := strconv.Atoi(modoStr)
	if err != nil || (modo != 1 && modo != 2) {
		respondJSON(w, http.StatusBadRequest, map[string]string{
			"error": "El formato de la petición enviada es incorrecto o el modo de vaciado indicado no existe",
		})
		return
	}

	state.mu.Lock()
	defer state.mu.Unlock()

	mesa, exists := state.mesas[id]
	if !exists {
		respondJSON(w, http.StatusNotFound, map[string]string{
			"error": "Mesa no encontrada",
		})
		return
	}

	// Verificar que la mesa tiene algo
	if mesa.Estado == 1 {
		respondJSON(w, 202, map[string]string{
			"message": "La mesa ya está vacía",
		})
		return
	}

	orden := state.ordenes[id]
	if modo == 1 {
		// Modo 1: Vaciar y continuar
		log.Printf("🔄 Mesa %d vaciada (modo continuar) - Palé %d completado", id, orden.PaleActual)
		orden.CajasEnPale = 0
		mesa.NumeroPaleActual = orden.PaleActual
	} else {
		// Modo 2: Vaciar y finalizar
		log.Printf("🏁 Mesa %d vaciada (modo finalizar) - Orden completada", id)
		delete(state.ordenes, id)
		mesa.Estado = 1
		mesa.DescripcionEstado = "Libre"
		mesa.NumeroCajaPale = 0
		mesa.NumeroPaleActual = 0
		mesa.CodigoTipoCaja = ""
		mesa.CodigoTipoPale = ""
		mesa.CajasPerCapa = 0
		mesa.NumeroPales = 0
	}

	respondJSON(w, http.StatusOK, map[string]string{
		"message": "Solicitud de vaciado registrada correctamente en la mesa",
	})
}

func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("Error encoding JSON: %v", err)
	}
}
