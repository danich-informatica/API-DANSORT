package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/gopcua/opcua"
	"github.com/gopcua/opcua/ua"
)

type nodePlan struct {
	ID   string
	Kind string // "bool" | "int16" | "string"
}

func mustParseNodeID(s string) *ua.NodeID {
	id, err := ua.ParseNodeID(s)
	if err != nil {
		log.Fatalf("NodeID inválido: %s (%v)", s, err)
	}
	return id
}

func readValue(ctx context.Context, c *opcua.Client, idStr string, tag string) (any, error) {
	id := mustParseNodeID(idStr)
	rr, err := c.Read(ctx, &ua.ReadRequest{
		NodesToRead: []*ua.ReadValueID{{
			NodeID:      id,
			AttributeID: ua.AttributeIDValue,
		}},
	})
	if err != nil {
		return nil, fmt.Errorf("Read %s: %w", idStr, err)
	}
	if len(rr.Results) == 0 || rr.Results[0].Status != ua.StatusOK {
		if len(rr.Results) == 0 {
			return nil, fmt.Errorf("Read %s sin resultados", idStr)
		}
		return nil, fmt.Errorf("Read %s status: %s", idStr, rr.Results[0].Status)
	}
	val := rr.Results[0].Value.Value()
	log.Printf("✅ LECTURA [%s] %s | %v (%T)", tag, idStr, val, val)
	return val, nil
}

func writeValue(ctx context.Context, c *opcua.Client, idStr string, newVal any) error {
	id := mustParseNodeID(idStr)
	v, err := ua.NewVariant(newVal)
	if err != nil {
		return fmt.Errorf("NewVariant(%T) %s: %w", newVal, idStr, err)
	}
	req := &ua.WriteRequest{
		NodesToWrite: []*ua.WriteValue{{
			NodeID:      id,
			AttributeID: ua.AttributeIDValue,
			Value: &ua.DataValue{
				EncodingMask: ua.DataValueValue, // <- esto fue clave en tu write que funcionó
				Value:        v,
			},
		}},
	}
	resp, err := c.Write(ctx, req)
	if err != nil {
		return fmt.Errorf("Write %s: %w", idStr, err)
	}
	if len(resp.Results) == 0 || resp.Results[0] != ua.StatusOK {
		if len(resp.Results) == 0 {
			return fmt.Errorf("Write %s sin resultados", idStr)
		}
		return fmt.Errorf("Write %s status: %s", idStr, resp.Results[0])
	}
	log.Printf("✅ OK ESCRITURA %s | %v (%T)", idStr, newVal, newVal)
	return nil
}

func main() {
	// --- Config ---
	endpoint := "opc.tcp://192.168.120.100:4840"

	// Nodos (según tus capturas)
	nodes := []nodePlan{
		//{ID: "ns=4;i=20", Kind: "bool"},   // DB_Test.Boleano
		//{ID: "ns=4;i=22", Kind: "string"}, // DB_Test.Texto
		{ID: "ns=4;i=21", Kind: "array_int32"},  // DB_Test.Estructura.Ent
		//{ID: "ns=4;i=23", Kind: "uint32"}, // DB_Test.Estructura.Text
		//{ID: "ns=4;i=28", Kind: "bool"},   // DB_Test.Estructura.Boo
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Cliente (mismos flags que te funcionaron)
	c, err := opcua.NewClient(endpoint,
		opcua.SecurityMode(ua.MessageSecurityModeNone),
		opcua.SecurityPolicy(ua.SecurityPolicyURINone),
		opcua.AutoReconnect(true),
	)
	if err != nil {
		log.Fatalf("Crear cliente OPC UA: %v", err)
	}
	if err := c.Connect(ctx); err != nil {
		log.Fatalf("Conexión OPC UA: %v", err)
	}
	defer c.Close(ctx)
	log.Println("Conectado.")

	// --- 1) LECTURA INICIAL ---
	log.Println("\n--- 1) LECTURA INICIAL ---")
	initial := make(map[string]any)
	for _, n := range nodes {
		val, err := readValue(ctx, c, n.ID, "INICIO")
		if err != nil {
			log.Println("READ:", err)
			continue
		}
		initial[n.ID] = val
	}

	// --- 2) ESCRITURA ---
	log.Println("\n--- 2) ESCRITURA ---")
	for _, n := range nodes {
		cur, ok := initial[n.ID]
		if !ok {
			log.Printf("Skip %s: no se pudo leer inicialmente\n", n.ID)
			continue
		}

		var desired any
		switch n.Kind {
		case "bool":
			b, ok := cur.(bool)
			if !ok {
				log.Printf("Skip %s: valor no es bool (%T)\n", n.ID, cur)
				continue
			}
			desired = !b // toggle

		case "int16":
			v, ok := cur.(int16)
			if !ok {
				// a veces llega como int32 si el servidor hace upcast
				if v32, ok2 := cur.(int32); ok2 {
					v = int16(v32)
				} else {
					log.Printf("Skip %s: valor no es int16 (%T)\n", n.ID, cur)
					continue
				}
			}
			desired = v + 1

		case "string":
			// usa un texto corto para no chocar con STRING[n]
			desired = "Go_" + time.Now().Format("150405")
			if s, ok := cur.(string); ok && s == desired {
				desired = 4
			}

		default:
			log.Printf("Skip %s: tipo no soportado: %s\n", n.ID, n.Kind)
			continue
		}

		if err := writeValue(ctx, c, n.ID, desired); err != nil {
			log.Println("WRITE:", err)
		}
	}

	// --- 3) LECTURA FINAL ---
	log.Println("\n--- 3) LECTURA FINAL ---")
	for _, n := range nodes {
		_, err := readValue(ctx, c, n.ID, "FINAL")
		if err != nil {
			log.Println("READ:", err)
		}
	}

	log.Println("\n--- DONE ---")
}

