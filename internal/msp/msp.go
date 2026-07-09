package msp

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
)

const MSPURL = "https://coberturasalud.msp.gob.ec/"

var (
	chromiumOnce sync.Once
	chromiumBin  string
	chromiumErr  error
)

func findChromeIn(root string) string {
	entries, err := os.ReadDir(root)
	if err != nil {
		return ""
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		candidate := filepath.Join(root, e.Name(), "chrome")
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return ""
}

func EnsureChromium() (string, error) {
	chromiumOnce.Do(func() {
		home, _ := os.UserHomeDir()
		cacheDir := filepath.Join(home, ".cache", "cobertura_msp", "chromium")

		fmt.Fprintf(os.Stderr, "[CHROMIUM] Buscando en caché: %s\n", cacheDir)
		if found := findChromeIn(cacheDir); found != "" {
			chromiumBin = found
			fmt.Fprintf(os.Stderr, "[CHROMIUM] ✓ Encontrado: %s\n", found)
			return
		}

		fmt.Fprintf(os.Stderr, "[CHROMIUM] Buscando en default: %s\n", launcher.DefaultBrowserDir)
		if found := findChromeIn(launcher.DefaultBrowserDir); found != "" {
			chromiumBin = found
			fmt.Fprintf(os.Stderr, "[CHROMIUM] ✓ Encontrado en default: %s\n", found)
			return
		}

		fmt.Fprintf(os.Stderr, "[CHROMIUM] Descargando (~150MB)...\n")
		if err := os.MkdirAll(cacheDir, 0755); err != nil {
			chromiumErr = fmt.Errorf("crear caché: %w", err)
			return
		}
		b := launcher.NewBrowser()
		b.RootDir = cacheDir
		bin, err := b.Get()
		if err != nil {
			chromiumErr = fmt.Errorf("descarga chromium: %w", err)
			return
		}
		chromiumBin = bin
		fmt.Fprintf(os.Stderr, "[CHROMIUM] ✓ Descargado: %s\n", bin)
	})
	return chromiumBin, chromiumErr
}

func StartBrowser() (*rod.Browser, error) {
	fmt.Fprintf(os.Stderr, "[BROWSER] Iniciando navegador...\n")
	bin, err := EnsureChromium()
	if err != nil {
		return nil, err
	}
	fmt.Fprintf(os.Stderr, "[BROWSER] Chromium bin: %s\n", bin)

	l := launcher.New().Headless(true).NoSandbox(true).
		Set("disable-gpu").Set("disable-dev-shm-usage").
		Set("js-flags", "--max-old-space-size=128").
		Set("disable-features", "TranslateUI,OptimizationHints")
	if bin != "" {
		l = l.Bin(bin)
	}
	url, err := l.Launch()
	if err != nil {
		return nil, fmt.Errorf("launch: %w", err)
	}
	browser := rod.New().ControlURL(url)
	if err := browser.Connect(); err != nil {
		return nil, fmt.Errorf("connect: %w", err)
	}
	fmt.Fprintf(os.Stderr, "[BROWSER] ✓ Navegador listo\n")
	return browser, nil
}

type Generator struct {
	OutputDir  string
	MaxRetries int
	browser    *rod.Browser
	ownsBrowser bool
}

func NewGenerator(outputDir string, browser *rod.Browser) *Generator {
	os.MkdirAll(outputDir, 0755)
	return &Generator{OutputDir: outputDir, MaxRetries: 8, browser: browser}
}

func (g *Generator) Start() error {
	if g.browser != nil {
		return nil
	}
	b, err := StartBrowser()
	if err != nil {
		return err
	}
	g.browser = b
	g.ownsBrowser = true
	return nil
}

func (g *Generator) Close() {
	if g.ownsBrowser && g.browser != nil {
		g.browser.Close()
	}
}

// GenerateAll procesa todos los PDFs de una cobertura reutilizando la misma página
func (g *Generator) GenerateAll(cedulas, fechas []string, pdfStart int) []Result {
	var results []Result
	os.MkdirAll(g.OutputDir, 0755)

	page, err := g.browser.Page(proto.TargetCreateTarget{URL: MSPURL})
	if err != nil {
		for i := range cedulas {
			results = append(results, Result{Cedula: cedulas[i], Fecha: fechas[i], PDFNum: pdfStart + i, Error: fmt.Errorf("open page: %w", err)})
		}
		return results
	}
	defer func() { _ = page.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	page = page.Context(ctx)

	if err := page.WaitLoad(); err != nil {
		for i := range cedulas {
			results = append(results, Result{Cedula: cedulas[i], Fecha: fechas[i], PDFNum: pdfStart + i, Error: fmt.Errorf("wait load: %w", err)})
		}
		return results
	}
	page.WaitStable(300)

	for i := range cedulas {
		r := g.generateOneOnPage(page, cedulas[i], fechas[i], pdfStart+i)
		results = append(results, r)
	}
	return results
}

// generateOneOnPage genera un PDF en una página ya abierta
func (g *Generator) generateOneOnPage(page *rod.Page, cedula, fecha string, pdfNum int) Result {
	r := Result{Cedula: cedula, Fecha: fecha, PDFNum: pdfNum}
	for attempt := 1; attempt <= g.MaxRetries; attempt++ {
		r.Retries = attempt
		pdfPath, err := g.tryGenerateOnPage(page, cedula, fecha, pdfNum)
		if err == nil {
			r.PDFPath = pdfPath
			return r
		}
		r.Error = err
		// Recargar la página para reintentar
		page.Navigate(MSPURL)
		page.WaitLoad()
		page.WaitStable(300)
		time.Sleep(1 * time.Second)
	}
	return r
}

type Result struct {
	Cedula  string
	Fecha   string
	PDFNum  int
	PDFPath string
	Error   error
	Retries int
}

func (g *Generator) GenerateOne(cedula, fecha string, pdfNum int) *Result {
	r := &Result{Cedula: cedula, Fecha: fecha, PDFNum: pdfNum}
	for attempt := 1; attempt <= g.MaxRetries; attempt++ {
		r.Retries = attempt
		pdfPath, err := g.tryGenerate(cedula, fecha, pdfNum)
		if err == nil {
			r.PDFPath = pdfPath
			return r
		}
		r.Error = err
		time.Sleep(2 * time.Second)
	}
	return r
}

func (g *Generator) tryGenerate(cedula, fecha string, pdfNum int) (retPath string, retErr error) {
	defer func() {
		if r := recover(); r != nil {
			retErr = fmt.Errorf("panic: %v", r)
		}
	}()

	page, err := g.browser.Page(proto.TargetCreateTarget{URL: MSPURL})
	if err != nil {
		return "", fmt.Errorf("open page: %w", err)
	}
	defer func() { _ = page.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	page = page.Context(ctx)

	if err := page.WaitLoad(); err != nil {
		return "", fmt.Errorf("wait load: %w", err)
	}
	page.WaitStable(300)
	time.Sleep(500 * time.Millisecond)

	el, err := page.Element("input")
	if err != nil {
		return "", fmt.Errorf("cedula: %w", err)
	}
	if err := el.Focus(); err != nil {
		return "", fmt.Errorf("cedula focus: %w", err)
	}
	if err := el.Input(cedula); err != nil {
		return "", fmt.Errorf("cedula input: %w", err)
	}
	time.Sleep(200 * time.Millisecond)

	inputs, err := page.Elements("input")
	if err != nil || len(inputs) < 2 {
		return "", fmt.Errorf("date input: %w", err)
	}
	if err := inputs[1].Click(proto.InputMouseButtonLeft, 1); err != nil {
		return "", fmt.Errorf("date click: %w", err)
	}
	time.Sleep(300 * time.Millisecond)

	parts := strings.Split(fecha, "-")
	if len(parts) != 3 {
		return "", fmt.Errorf("bad date: %s", fecha)
	}
	targetDay := strings.TrimLeft(parts[0], "0")

	page.Eval(`() => {
		const dialog = document.querySelector('.MuiDialog-root');
		if (!dialog) return;
		const arrows = dialog.querySelectorAll('.MuiPickersArrowSwitcher-root svg');
		if (arrows.length >= 2) { arrows[0].closest('button').click(); return; }
		const btns = dialog.querySelectorAll('button');
		if (btns.length >= 1) btns[0].click();
	}`)
	time.Sleep(200 * time.Millisecond)

	page.Eval(`(day) => {
		const dialog = document.querySelector('.MuiDialog-root');
		if (!dialog) return;
		for (const btn of dialog.querySelectorAll('button')) {
			if (btn.textContent.trim() === day) { btn.click(); return; }
		}
	}`, targetDay)
	time.Sleep(200 * time.Millisecond)

	page.Eval(`() => {
		const dialog = document.querySelector('.MuiDialog-root');
		if (!dialog) return;
		const buttons = dialog.querySelectorAll('button');
		for (const btn of buttons) {
			if (btn.textContent.trim().toLowerCase() === 'ok' || btn.textContent.trim().toLowerCase() === 'aceptar') {
				btn.click(); return;
			}
		}
		buttons[buttons.length - 1]?.click();
	}`)
	time.Sleep(200 * time.Millisecond)

	btns, err := page.Elements("button")
	if err != nil || len(btns) == 0 {
		return "", fmt.Errorf("no buttons")
	}
	if err := btns[0].Click(proto.InputMouseButtonLeft, 1); err != nil {
		return "", fmt.Errorf("consultar click: %w", err)
	}

	// Esperar a que el MSP procese (polling cada 1s, máximo 15s)
	fmt.Fprintf(os.Stderr, "[MSP] Esperando respuesta del MSP...\n")
	var lastStatus string
	for i := 0; i < 15; i++ {
		time.Sleep(1 * time.Second)
		status, err := page.Eval(`() => {
			const embed = document.querySelector('embed');
			if (embed && embed.src && embed.src.startsWith('blob:')) return 'PDF_FOUND';
			const body = document.body.innerText || '';
			if (body.includes('no registra') || body.includes('No se encontr')) return 'NO_DATA';
			return 'WAIT';
		}`)
		if err != nil {
			lastStatus = "EVAL_ERR"
			continue
		}
		lastStatus = status.Value.Str()
		if lastStatus == "PDF_FOUND" {
			pdfPath := filepath.Join(g.OutputDir, fmt.Sprintf("CC_%02d.pdf", pdfNum))
			return savePDF(page, pdfPath)
		}
		if lastStatus == "NO_DATA" {
			return "", fmt.Errorf("no data")
		}
		fmt.Fprintf(os.Stderr, "[MSP]   ... esperando (%ds)\n", i+1)
	}

	return "", fmt.Errorf("timeout esperando MSP (último estado: %s)", lastStatus)
}

// tryGenerateOnPage es igual a tryGenerate pero reutiliza una página ya abierta
func (g *Generator) tryGenerateOnPage(page *rod.Page, cedula, fecha string, pdfNum int) (retPath string, retErr error) {
	defer func() {
		if r := recover(); r != nil {
			retErr = fmt.Errorf("panic: %v", r)
		}
	}()

	// Limpiar y llenar cédula
	el, err := page.Element("input")
	if err != nil {
		return "", fmt.Errorf("cedula: %w", err)
	}
	// Seleccionar todo el texto y escribir el nuevo
	el.Focus()
	el.Eval(`(el, val) => { el.value = ''; el.dispatchEvent(new Event('input', {bubbles:true})); }`)
	time.Sleep(100 * time.Millisecond)
	if err := el.Input(cedula); err != nil {
		return "", fmt.Errorf("cedula input: %w", err)
	}
	time.Sleep(200 * time.Millisecond)

	// Click en el date picker (segundo input)
	inputs, err := page.Elements("input")
	if err != nil || len(inputs) < 2 {
		return "", fmt.Errorf("date input: %w", err)
	}
	if err := inputs[1].Click(proto.InputMouseButtonLeft, 1); err != nil {
		return "", fmt.Errorf("date click: %w", err)
	}
	time.Sleep(300 * time.Millisecond)

	// Seleccionar día
	parts := strings.Split(fecha, "-")
	if len(parts) != 3 {
		return "", fmt.Errorf("bad date: %s", fecha)
	}
	targetDay := strings.TrimLeft(parts[0], "0")

	page.Eval(`() => {
		const dialog = document.querySelector('.MuiDialog-root');
		if (!dialog) return;
		const arrows = dialog.querySelectorAll('.MuiPickersArrowSwitcher-root svg');
		if (arrows.length >= 2) { arrows[0].closest('button').click(); return; }
		const btns = dialog.querySelectorAll('button');
		if (btns.length >= 1) btns[0].click();
	}`)
	time.Sleep(200 * time.Millisecond)

	page.Eval(`(day) => {
		const dialog = document.querySelector('.MuiDialog-root');
		if (!dialog) return;
		for (const btn of dialog.querySelectorAll('button')) {
			if (btn.textContent.trim() === day) { btn.click(); return; }
		}
	}`, targetDay)
	time.Sleep(200 * time.Millisecond)

	page.Eval(`() => {
		const dialog = document.querySelector('.MuiDialog-root');
		if (!dialog) return;
		const buttons = dialog.querySelectorAll('button');
		for (const btn of buttons) {
			if (btn.textContent.trim().toLowerCase() === 'ok' || btn.textContent.trim().toLowerCase() === 'aceptar') {
				btn.click(); return;
			}
		}
		buttons[buttons.length - 1]?.click();
	}`)
	time.Sleep(200 * time.Millisecond)

	// Click consultar
	btns, err := page.Elements("button")
	if err != nil || len(btns) == 0 {
		return "", fmt.Errorf("no buttons")
	}
	if err := btns[0].Click(proto.InputMouseButtonLeft, 1); err != nil {
		return "", fmt.Errorf("consultar click: %w", err)
	}

	// Polling esperando respuesta del MSP
	fmt.Fprintf(os.Stderr, "[MSP] Esperando respuesta del MSP...\n")
	var lastStatus string
	for i := 0; i < 15; i++ {
		time.Sleep(1 * time.Second)
		status, err := page.Eval(`() => {
			const embed = document.querySelector('embed');
			if (embed && embed.src && embed.src.startsWith('blob:')) return 'PDF_FOUND';
			const body = document.body.innerText || '';
			if (body.includes('no registra') || body.includes('No se encontr')) return 'NO_DATA';
			return 'WAIT';
		}`)
		if err != nil {
			lastStatus = "EVAL_ERR"
			continue
		}
		lastStatus = status.Value.Str()
		if lastStatus == "PDF_FOUND" {
			pdfPath := filepath.Join(g.OutputDir, fmt.Sprintf("CC_%02d.pdf", pdfNum))
			return savePDF(page, pdfPath)
		}
		if lastStatus == "NO_DATA" {
			return "", fmt.Errorf("no data")
		}
		fmt.Fprintf(os.Stderr, "[MSP]   ... esperando (%ds)\n", i+1)
	}

	return "", fmt.Errorf("timeout esperando MSP (último estado: %s)", lastStatus)
}

func savePDF(page *rod.Page, path string) (string, error) {
	b64Result, err := page.Eval(`async () => {
		const embed = document.querySelector('embed');
		const resp = await fetch(embed.src);
		const blob = await resp.blob();
		const reader = new FileReader();
		const b64 = await new Promise(r => {
			reader.onload = e => r(e.target.result);
			reader.readAsDataURL(blob);
		});
		const comma = b64.indexOf(',');
		return 'SIZE:' + blob.size + '|' + b64.substring(comma + 1);
	}`)
	if err != nil {
		return "", fmt.Errorf("read blob: %w", err)
	}
	b64Str := b64Result.Value.Str()
	if len(b64Str) > 5 && b64Str[:5] == "SIZE:" {
		parts := strings.SplitN(b64Str, "|", 2)
		if len(parts) != 2 {
			return "", fmt.Errorf("malformed b64")
		}
		pdfBytes, err := base64.StdEncoding.DecodeString(parts[1])
		if err != nil {
			return "", fmt.Errorf("decode: %w", err)
		}
		if err := os.WriteFile(path, pdfBytes, 0644); err != nil {
			return "", fmt.Errorf("write: %w", err)
		}
		return path, nil
	}
	return "", fmt.Errorf("unexpected: %s", b64Str[:min(50, len(b64Str))])
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
