package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/gopcua/opcua"
	"github.com/gopcua/opcua/ua"
)

func main() {
	endpoint := "opc.tcp://192.168.120.100:4840"
	ctx := context.Background()

	c, err := opcua.NewClient(endpoint)
	if err != nil {
		log.Fatal(err)
	}
	if err := c.Connect(ctx); err != nil {
		log.Fatal(err)
	}
	defer c.Close(ctx)
	fmt.Println("✅ Conectado al servidor OPC UA")

	objectID, _ := ua.ParseNodeID("ns=4;i=1")
	methodID, _ := ua.ParseNodeID("ns=4;i=21")

	// Probar TODOS los tipos posibles para DataType "i=4"
	tests := []struct {
		name  string
		value interface{}
	}{
		{"int8(1)", int8(1)},
		{"uint8(1)", uint8(1)},
		{"int16(1)", int16(1)},
		{"uint16(1)", uint16(1)},
		{"int32(1)", int32(1)},
		{"uint32(1)", uint32(1)},
		{"float32(1)", float32(1)},
		{"string '1'", "1"},
		{"bool true", true},
		{"NodeID ns=4;i=26", mustParseNodeID("ns=4;i=26")},
	}

	for _, test := range tests {
		fmt.Printf("\n🧪 Probando: %s\n", test.name)

		variant := ua.MustVariant(test.value)
		req := &ua.CallMethodRequest{
			ObjectID:       objectID,
			MethodID:       methodID,
			InputArguments: []*ua.Variant{variant},
		}

		ctx2, cancel := context.WithTimeout(ctx, 5*time.Second)
		resp, err := c.Call(ctx2, req)
		cancel()

		if err != nil {
			fmt.Printf("   ❌ Error: %v\n", err)
			continue
		}

		if resp.StatusCode != ua.StatusOK {
			fmt.Printf("   ❌ Status: %s\n", resp.StatusCode)
		} else {
			fmt.Printf("   ✅ ¡ÉXITO! Output: %v\n", resp.OutputArguments)
			break
		}
	}
}

func mustParseNodeID(s string) *ua.NodeID {
	id, err := ua.ParseNodeID(s)
	if err != nil {
		panic(err)
	}
	return id
}
