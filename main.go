package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strings"
	"syscall"

	"cobertura_msp/internal/oracle"
	"cobertura_msp/internal/server"
)

func main() {
	port := flag.Int("port", 9899, "Puerto del servidor web")
	outDir := flag.String("out", "./coberturas", "Directorio de salida de PDFs")
	hosts := flag.String("hosts", "172.16.60.20,172.16.60.21", "Hosts Oracle RAC")
	service := flag.String("service", "prdsgh", "Servicio Oracle")
	schema := flag.String("schema", "digitalizacion", "Schema Oracle")
	password := flag.String("password", "digitalizacion", "Password Oracle")
	noOpen := flag.Bool("no-open", false, "No abrir navegador automáticamente")
	flag.Parse()

	conn := oracle.ConnParams{
		Hosts:   strings.Split(*hosts, ","),
		Service: *service,
		Schema:  *schema,
		Pass:    *password,
	}

	fmt.Println("╔══════════════════════════════════════════╗")
	fmt.Println("║    🏥 Cobertura MSP - Descargador        ║")
	fmt.Println("╚══════════════════════════════════════════╝")
	fmt.Println()

	fmt.Printf("Conectando a Oracle RAC: %s @ %s ...\n", conn.Schema, strings.Join(conn.Hosts, ","))
	db, err := oracle.Connect(conn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR Oracle: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()
	fmt.Println("✓ Conectado a Oracle")

	os.MkdirAll(*outDir, 0755)

	srv := server.New(db, *schema, *outDir)
	fmt.Fprintf(srv.LogWriter(), "[INIT] Servidor iniciando...\n")

	addr := fmt.Sprintf(":%d", *port)
	url := fmt.Sprintf("http://localhost:%d", *port)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		fmt.Println("\nCerrando...")
		db.Close()
		os.Exit(0)
	}()

	if !*noOpen {
		go openBrowser(url)
	}

	fmt.Printf("\n  Abre tu navegador en: %s\n\n", url)
	if err := srv.Start(addr); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR servidor: %v\n", err)
		os.Exit(1)
	}
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	cmd.Start()
}
