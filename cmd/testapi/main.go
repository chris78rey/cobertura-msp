package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"cobertura_msp/internal/msp"
)

func main() {
	cedula := "1760930154"
	if len(os.Args) > 1 {
		cedula = os.Args[1]
	}
	fecha := "2026-06-15"
	if len(os.Args) > 2 {
		fecha = os.Args[2]
	}

	start := time.Now()
	result, err := msp.QueryAPI(cedula, fecha)
	elapsed := time.Since(start)

	if err != nil {
		fmt.Printf("ERROR (%.1fs): %v\n", elapsed.Seconds(), err)
		os.Exit(1)
	}

	fmt.Printf("OK (%.1fs)\n", elapsed.Seconds())
	fmt.Printf("Cedula: %s  Fecha: %s\n", result.Cedula, result.Fecha)

	var pretty interface{}
	json.Unmarshal(result.Data, &pretty)
	b, _ := json.MarshalIndent(pretty, "", "  ")
	if len(b) > 800 {
		b = b[:800]
	}
	fmt.Printf("Data: %s...\n", string(b))
}
