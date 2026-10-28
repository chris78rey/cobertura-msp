package server

import (
	"archive/zip"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"cobertura_msp/internal/msp"
	"cobertura_msp/internal/oracle"

	"github.com/go-rod/rod"
)

type Server struct {
	db            *sql.DB
	schema        string
	outDir        string
	mu            sync.Mutex
	jobs          map[string]*JobStatus
	lastTramites  []int64
	logBuf        *logBuffer
	listener      net.Listener
	downloadCount map[string]int
	warmBrowser   *rod.Browser
	warmMu        sync.Mutex
	sessions      *oracle.SessionManager
}

type logBuffer struct {
	mu      sync.Mutex
	entries []string
}

func newLogBuffer() *logBuffer {
	return &logBuffer{}
}

func (lb *logBuffer) Write(p []byte) (n int, err error) {
	lb.mu.Lock()
	defer lb.mu.Unlock()
	lb.entries = append(lb.entries, string(p))
	if len(lb.entries) > 500 {
		lb.entries = lb.entries[len(lb.entries)-500:]
	}
	return len(p), nil
}

func (lb *logBuffer) String() string {
	lb.mu.Lock()
	defer lb.mu.Unlock()
	return strings.Join(lb.entries, "")
}

type JobStatus struct {
	ID         string `json:"id"`
	Tramite    int64  `json:"tramite"`
	Total      int    `json:"total"`
	Completed  int    `json:"completed"`
	Failed     int    `json:"failed"`
	Status     string `json:"status"`
	Error      string `json:"error,omitempty"`
	StartedAt  string `json:"started_at"`
	FinishedAt string `json:"finished_at,omitempty"`
	ClientID   string `json:"client_id"`
	Token      string `json:"token,omitempty"`
}

func New(db *sql.DB, schema, outDir string) *Server {
	return &Server{db: db, schema: schema, outDir: outDir, jobs: make(map[string]*JobStatus), logBuf: newLogBuffer(), downloadCount: make(map[string]int), sessions: oracle.NewSessionManager()}
}

func (s *Server) Start(addr string) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleIndex)
	mux.HandleFunc("/api/query", s.handleQuery)
	mux.HandleFunc("/api/generate", s.handleGenerate)
	mux.HandleFunc("/api/status", s.handleStatus)
	mux.HandleFunc("/api/download/", s.handleDownload)
	mux.HandleFunc("/api/download-zip", s.handleDownloadZip)
	mux.HandleFunc("/api/list-pdfs", s.handleListPDFs)
	mux.HandleFunc("/api/logs", s.handleLogs)
	mux.HandleFunc("/api/diag", s.handleDiag)
	mux.HandleFunc("/api/disk", s.handleDisk)
	mux.HandleFunc("/reporte", s.handleConsistencyPage)
	mux.HandleFunc("/api/reporte", s.handleConsistencyReport)
	mux.HandleFunc("/api/reporte/excel", s.handleConsistencyReportExcel)
	mux.HandleFunc("/api/reporte/sync", s.handleConsistencySync)
	mux.HandleFunc("/api/login", s.handleLogin)
	mux.HandleFunc("/api/logout", s.handleLogout)
	mux.HandleFunc("/api/periodos", s.handlePeriodos)
	mux.HandleFunc("/api/generaciones", s.handleGeneraciones)
	mux.HandleFunc("/api/planillas", s.handlePlanillas)

	var err error
	s.listener, err = net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "  Servidor en http://%s\n", s.listener.Addr().String())

	go func() {
		for {
			time.Sleep(10 * time.Minute)
			s.cleanOldPDFs(1 * time.Hour)
		}
	}()

	go s.warmupBrowser()

	return http.Serve(s.listener, mux)
}

func (s *Server) Addr() string {
	if s.listener == nil {
		return ""
	}
	return s.listener.Addr().String()
}

func (s *Server) LogWriter() io.Writer {
	return s.logBuf
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(indexHTML))
}

type QueryRequest struct {
	From int64 `json:"from"`
	To   int64 `json:"to"`
}

type TramiteInfo struct {
	Tramite   int64  `json:"tramite"`
	Cedula    string `json:"cedula"`
	Menor     string `json:"menor"`
	Tandas    int    `json:"tandas"`
	TotalPDFs int    `json:"total_pdfs"`
	Planilla  string `json:"planilla"`
	Hasta     string `json:"hasta"`
	Alta      string `json:"alta"`
	Dep01     string `json:"dep01"`
	Dep02     string `json:"dep02"`
	HasPDFs   bool   `json:"has_pdfs"`
	PDFCount  int    `json:"pdf_count"`
}

func (s *Server) handleQuery(w http.ResponseWriter, r *http.Request) {
	var req QueryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	var results []TramiteInfo
	for t := req.From; t <= req.To; t++ {
		tr, err := oracle.GetTramite(s.getDB(r), s.schema, t)
		if err != nil {
			continue
		}
		jobs := tr.BuildJobs()
		pdfDir := filepath.Join(s.outDir, fmt.Sprintf("%d", t))
		pdfCount := countPDFs(pdfDir)
		results = append(results, TramiteInfo{
			Tramite: tr.DIG_TRAMITE, Cedula: tr.DIG_CEDULA, Menor: tr.DIG_MENOR_EDAD,
			Tandas: tr.Tandas, TotalPDFs: len(jobs),
			Planilla: tr.DIG_FECHA_PLANILLA, Hasta: tr.DIG_FECHA_HASTA, Alta: tr.DIG_FECHA_ALTA,
			Dep01: tr.DIG_DEPENDIENTE_01, Dep02: tr.DIG_DEPENDIENTE_02,
			HasPDFs: pdfCount > 0, PDFCount: pdfCount,
		})
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}

type GenRequest struct {
	Tramites []int64 `json:"tramites"`
	Workers  int     `json:"workers"`
	ClientID string  `json:"client_id"`
	Token    string  `json:"token"`
}

func (s *Server) handleGenerate(w http.ResponseWriter, r *http.Request) {
	var req GenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}

	if len(req.Tramites) > 2000 {
		http.Error(w, "máximo 2000 coberturas por bloque", 400)
		return
	}

	freeGB := freeDiskGB(s.outDir)
	if freeGB < 7 {
		http.Error(w, fmt.Sprintf("espacio insuficiente: %.1f GB libres (mínimo 7 GB)", freeGB), 507)
		return
	}

	req.Workers = 1

	// Usar client_id para aislar sesiones (si no viene, generar uno)
	if req.ClientID == "" {
		req.ClientID = fmt.Sprintf("auto_%d", time.Now().UnixNano())
	}

	s.mu.Lock()
	// Cada generación debe quedar aislada: limpiar trabajos previos del mismo cliente
	for id, js := range s.jobs {
		if js.ClientID == req.ClientID {
			delete(s.jobs, id)
		}
	}
	s.lastTramites = req.Tramites
	s.mu.Unlock()

	var jobIDs []string
	for _, t := range req.Tramites {
		id := fmt.Sprintf("%s_%d_%d", req.ClientID, t, time.Now().UnixNano())
		s.mu.Lock()
		s.jobs[id] = &JobStatus{ID: id, Tramite: t, Status: "pending", StartedAt: time.Now().Format(time.RFC3339), ClientID: req.ClientID, Token: req.Token}
		s.mu.Unlock()
		jobIDs = append(jobIDs, id)
	}

	go s.runGeneration(req.Tramites, req.Workers)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"job_ids": jobIDs, "started": true, "skipped": 0})
}

func (s *Server) runGeneration(tramites []int64, workers int) {
	type jobItem struct {
		tramite int64
		id      string
	}

	s.mu.Lock()
	var pending []jobItem
	for _, t := range tramites {
		for id, js := range s.jobs {
			if js.Tramite == t && js.Status == "pending" {
				js.Status = "running"
				pending = append(pending, jobItem{tramite: t, id: id})
			}
		}
	}
	s.mu.Unlock()

	jobsCh := make(chan jobItem, len(pending))
	var wg sync.WaitGroup

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			browser := s.takeWarmBrowser()
			if browser == nil {
				var err error
				browser, err = msp.StartBrowser()
				if err != nil {
					fmt.Fprintf(os.Stderr, "[Worker %d] Error iniciando browser: %v\n", workerID, err)
					for j := range jobsCh {
						s.updateJob(j.id, func(js *JobStatus) {
							js.Status = "error"
							js.Error = fmt.Sprintf("browser: %v", err)
							js.FinishedAt = time.Now().Format(time.RFC3339)
						})
					}
					return
				}
			} else {
				fmt.Fprintf(os.Stderr, "[Worker %d] Usando navegador pre-calentado\n", workerID)
			}
			defer browser.Close()

			for j := range jobsCh {
				s.processTramite(j.tramite, j.id, browser)
			}
		}(i + 1)
	}

	for _, j := range pending {
		jobsCh <- j
	}
	close(jobsCh)
	wg.Wait()
}

func (s *Server) processTramite(tramiteID int64, jobID string, browser *rod.Browser) {
	// Usar la conexión Oracle del usuario si hay sesión
	db := s.db
	s.mu.Lock()
	if js, ok := s.jobs[jobID]; ok && js.Token != "" {
		if session := s.sessions.GetSession(js.Token); session != nil {
			db = session.DB
		}
	}
	s.mu.Unlock()

	tr, err := oracle.GetTramite(db, s.schema, tramiteID)
	if err != nil {
		s.updateJob(jobID, func(js *JobStatus) {
			js.Status = "error"
			js.Error = err.Error()
			js.FinishedAt = time.Now().Format(time.RFC3339)
		})
		return
	}

	jobs := tr.BuildJobs()
	tramDir := filepath.Join(s.outDir, fmt.Sprintf("%d", tramiteID))
	os.MkdirAll(tramDir, 0755)
	// Regenerar siempre con las fechas actuales de Oracle y eliminar PDFs
	// anteriores que podrían pertenecer a otra cantidad de tandas.
	if entries, readErr := os.ReadDir(tramDir); readErr == nil {
		for _, entry := range entries {
			if strings.HasPrefix(entry.Name(), "CC_") && strings.HasSuffix(entry.Name(), ".pdf") {
				_ = os.Remove(filepath.Join(tramDir, entry.Name()))
			}
		}
	}

	s.updateJob(jobID, func(js *JobStatus) {
		js.Total = len(jobs)
		js.Status = "running"
	})

	// Usar generate_pdf.js directamente (mismo método que cobertura_integrada)
	nodeDir := os.Getenv("COBERTURA_NODE_PROJECT_DIR")
	if nodeDir == "" {
		nodeDir = filepath.Join(os.Getenv("HOME"), "codex_projects", "cobertura")
	}
	genScript := filepath.Join(nodeDir, "scripts", "generate_pdf.js")

	var lastErr string
	for _, j := range jobs {
		outputName := fmt.Sprintf("CC_%02d", j.PDFNum)

		// Convertir fecha de DD-MM-YYYY (Oracle) a YYYY-MM-DD (Node)
		fechaNode := j.Fecha
		if parts := strings.Split(j.Fecha, "-"); len(parts) == 3 {
			fechaNode = parts[2] + "-" + parts[1] + "-" + parts[0]
		}

		cmd := exec.Command("node", genScript,
			"--cedula", j.Cedula,
			"--fecha", fechaNode,
			"--output_name", outputName,
			"--output_dir", tramDir,
		)
		cmd.Dir = nodeDir
		cmd.Env = append(os.Environ(), "COBERTURA_TLS_INSECURE=true")

		output, err := cmd.CombinedOutput()
		if err != nil {
			fmt.Fprintf(os.Stderr, "[NODE] Error generate_pdf.js (%s): %v\n%s\n", j.Cedula, err, string(output[:min(300, len(output))]))
			// Fallback a Chromium
			gen := msp.NewGenerator(tramDir, browser)
			chrResult := gen.GenerateOne(j.Cedula, j.Fecha, j.PDFNum)
			if chrResult.Error != nil {
				s.updateJob(jobID, func(js *JobStatus) {
					js.Failed++
					lastErr = chrResult.Error.Error()
					js.Error = lastErr
				})
			} else {
				s.updateJob(jobID, func(js *JobStatus) { js.Completed++ })
			}
			continue
		}

		// Buscar el PDF generado y renombrarlo si es necesario
		pdfPath := filepath.Join(tramDir, outputName+".pdf")
		if _, err := os.Stat(pdfPath); os.IsNotExist(err) {
			// Buscar cualquier PDF en el directorio
			entries, _ := os.ReadDir(tramDir)
			for _, e := range entries {
				if strings.HasSuffix(e.Name(), ".pdf") && e.Name() != outputName+".pdf" {
					os.Rename(filepath.Join(tramDir, e.Name()), pdfPath)
					break
				}
			}
		}

		if _, err := os.Stat(pdfPath); err == nil {
			fmt.Fprintf(os.Stderr, "[NODE] ✅ PDF: %s\n", pdfPath)
			s.updateJob(jobID, func(js *JobStatus) { js.Completed++ })
			// Borrar JSONs residuales
			entries, _ := os.ReadDir(tramDir)
			for _, e := range entries {
				if strings.HasSuffix(e.Name(), ".json") {
					os.Remove(filepath.Join(tramDir, e.Name()))
				}
			}
		} else {
			fmt.Fprintf(os.Stderr, "[NODE] ❌ No se encontró PDF para %s\n", j.Cedula)
			s.updateJob(jobID, func(js *JobStatus) {
				js.Failed++
				lastErr = "no se generó PDF"
				js.Error = lastErr
			})
		}
	}

	s.updateJob(jobID, func(js *JobStatus) {
		js.Status = "done"
		js.FinishedAt = time.Now().Format(time.RFC3339)
	})
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (s *Server) updateJob(id string, fn func(*JobStatus)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if js, ok := s.jobs[id]; ok {
		fn(js)
	}
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	clientID := r.URL.Query().Get("client")
	s.mu.Lock()
	defer s.mu.Unlock()
	var all []*JobStatus
	for _, js := range s.jobs {
		if clientID == "" || js.ClientID == clientID {
			all = append(all, js)
		}
	}
	// Limpiar jobs viejos (más de 30 min terminados)
	cutoff := time.Now().Add(-30 * time.Minute)
	for id, js := range s.jobs {
		if js.FinishedAt != "" {
			t, err := time.Parse(time.RFC3339, js.FinishedAt)
			if err == nil && t.Before(cutoff) {
				delete(s.jobs, id)
			}
		}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(all)
}

func (s *Server) handleDownload(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/download/")
	fullPath := filepath.Join(s.outDir, path)
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filepath.Base(fullPath)))
	http.ServeFile(w, r, fullPath)

	s.mu.Lock()
	s.downloadCount[path]++
	count := s.downloadCount[path]
	s.mu.Unlock()

	fmt.Fprintf(s.logBuf, "[DOWNLOAD] %s → descarga #%d/3\n", filepath.Base(path), count)

	if count >= 3 {
		go func() {
			time.Sleep(2 * time.Second)
			if err := os.Remove(fullPath); err == nil {
				fmt.Fprintf(s.logBuf, "[CLEANUP] Borrado tras %d descargas: %s\n", count, filepath.Base(path))
			}
			dir := filepath.Dir(fullPath)
			entries, _ := os.ReadDir(dir)
			if len(entries) == 0 {
				os.Remove(dir)
			}
		}()
	}
}

func (s *Server) handleListPDFs(w http.ResponseWriter, r *http.Request) {
	tramite := r.URL.Query().Get("tramite")
	if tramite == "" {
		http.Error(w, "tramite required", 400)
		return
	}
	dir := filepath.Join(s.outDir, tramite)
	entries, err := os.ReadDir(dir)
	if err != nil {
		json.NewEncoder(w).Encode([]string{})
		return
	}
	var pdfs []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".pdf") {
			pdfs = append(pdfs, e.Name())
		}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(pdfs)
}

func (s *Server) handleDownloadZip(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	lastTramites := make([]int64, len(s.lastTramites))
	copy(lastTramites, s.lastTramites)
	s.mu.Unlock()

	if len(lastTramites) == 0 {
		http.Error(w, "no hay generación reciente", 404)
		return
	}

	type tramPDFs struct {
		name string
		pdfs []string
	}
	var tramList []tramPDFs
	for _, tID := range lastTramites {
		dirName := fmt.Sprintf("%d", tID)
		dir := filepath.Join(s.outDir, dirName)
		subEntries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		var pdfs []string
		for _, f := range subEntries {
			if !f.IsDir() && strings.HasSuffix(f.Name(), ".pdf") {
				pdfs = append(pdfs, f.Name())
			}
		}
		if len(pdfs) > 0 {
			tramList = append(tramList, tramPDFs{name: dirName, pdfs: pdfs})
		}
	}

	if len(tramList) == 0 {
		http.Error(w, "no hay PDFs para descargar", 404)
		return
	}

	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", "attachment; filename=\"coberturas_msp.zip\"")

	zw := zip.NewWriter(w)
	defer zw.Close()

	for _, t := range tramList {
		for _, pdf := range t.pdfs {
			srcPath := filepath.Join(s.outDir, t.name, pdf)
			entryName := filepath.Join(t.name, pdf)

			srcFile, err := os.Open(srcPath)
			if err != nil {
				continue
			}

			f, err := zw.Create(entryName)
			if err != nil {
				srcFile.Close()
				continue
			}
			io.Copy(f, srcFile)
			srcFile.Close()
		}
	}
}

func countPDFs(dir string) int {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}
	count := 0
	for _, e := range entries {
		if !e.IsDir() && strings.HasPrefix(e.Name(), "CC_") && strings.HasSuffix(e.Name(), ".pdf") {
			count++
		}
	}
	return count
}

func ParseRange(s string) []int64 {
	s = strings.TrimSpace(s)
	var result []int64
	if strings.Contains(s, ",") {
		for _, part := range strings.Split(s, ",") {
			result = append(result, ParseRange(part)...)
		}
		return result
	}
	if strings.Contains(s, "-") {
		parts := strings.SplitN(s, "-", 2)
		from, err1 := strconv.ParseInt(strings.TrimSpace(parts[0]), 10, 64)
		to, err2 := strconv.ParseInt(strings.TrimSpace(parts[1]), 10, 64)
		if err1 != nil || err2 != nil {
			return nil
		}
		for i := from; i <= to; i++ {
			result = append(result, i)
		}
		return result
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return nil
	}
	return []int64{n}
}

func (s *Server) cleanOldPDFs(maxAge time.Duration) {
	entries, err := os.ReadDir(s.outDir)
	if err != nil {
		return
	}
	cutoff := time.Now().Add(-maxAge)
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		tramDir := filepath.Join(s.outDir, e.Name())
		pdfs, err := os.ReadDir(tramDir)
		if err != nil {
			continue
		}
		allOld := true
		for _, pdf := range pdfs {
			pdfPath := filepath.Join(tramDir, pdf.Name())
			info, err := pdf.Info()
			if err != nil {
				continue
			}
			if info.ModTime().Before(cutoff) {
				os.Remove(pdfPath)
				fmt.Fprintf(s.logBuf, "[CLEANUP] Borrado (antiguo): %s/%s\n", e.Name(), pdf.Name())
			} else {
				allOld = false
			}
		}
		if allOld || len(pdfs) == 0 {
			if remaining, _ := os.ReadDir(tramDir); len(remaining) == 0 {
				os.Remove(tramDir)
			}
		}
	}
}

func (s *Server) handleLogs(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Write([]byte(s.logBuf.String()))
}

func (s *Server) handleDiag(w http.ResponseWriter, r *http.Request) {
	home, _ := os.UserHomeDir()
	cacheDir := filepath.Join(home, ".cache", "cobertura_msp", "chromium")

	bin, err := msp.EnsureChromium()

	diag := map[string]interface{}{
		"cache_dir":      cacheDir,
		"chrome_bin":     bin,
		"chromium_error": "",
		"out_dir":        s.outDir,
	}

	if err != nil {
		diag["chromium_error"] = err.Error()
	}

	if bin != "" {
		if info, err := os.Stat(bin); err == nil {
			diag["chrome_size_mb"] = float64(info.Size()) / 1024 / 1024
		}
	}

	if entries, err := os.ReadDir(cacheDir); err == nil {
		var dirs []string
		for _, e := range entries {
			dirs = append(dirs, e.Name())
		}
		diag["cache_contents"] = dirs
	}

	browser, berr := msp.StartBrowser()
	if berr != nil {
		diag["browser_error"] = berr.Error()
	} else {
		diag["browser_ok"] = true
		browser.Close()
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(diag)
}

func freeDiskGB(path string) float64 {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return -1
	}
	bytesFree := stat.Bavail * uint64(stat.Bsize)
	return float64(bytesFree) / (1024 * 1024 * 1024)
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "datos inválidos", 400)
		return
	}
	if req.Username == "" || req.Password == "" {
		http.Error(w, "usuario y contraseña requeridos", 400)
		return
	}

	session, err := s.sessions.Login(req.Username, req.Password)
	if err != nil {
		http.Error(w, err.Error(), 401)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "oracle_token",
		Value:    session.Token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"token":    session.Token,
		"username": session.Username,
	})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		if c, err := r.Cookie("oracle_token"); err == nil {
			token = c.Value
		}
	}
	if token != "" {
		s.sessions.Logout(token)
	}
	http.SetCookie(w, &http.Cookie{Name: "oracle_token", Value: "", Path: "/", MaxAge: -1, HttpOnly: true, SameSite: http.SameSiteLaxMode})
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"ok": true})
}

// getDB retorna la conexión a usar: la del usuario si hay sesión, o la default
func (s *Server) getDB(r *http.Request) *sql.DB {
	token := r.URL.Query().Get("token")
	if token == "" {
		// También buscar en header
		token = r.Header.Get("X-Session-Token")
	}
	if token == "" {
		if c, err := r.Cookie("oracle_token"); err == nil {
			token = c.Value
		}
	}
	if token != "" {
		if session := s.sessions.GetSession(token); session != nil {
			return session.DB
		}
	}
	return s.db
}

func (s *Server) handleDisk(w http.ResponseWriter, r *http.Request) {
	// Usar home del usuario si outDir no existe
	checkPath := s.outDir
	if _, err := os.Stat(checkPath); os.IsNotExist(err) {
		home, _ := os.UserHomeDir()
		checkPath = home
	}
	freeGB := freeDiskGB(checkPath)
	totalGB := float64(0)
	var stat syscall.Statfs_t
	if err := syscall.Statfs(checkPath, &stat); err == nil {
		totalGB = float64(stat.Blocks*uint64(stat.Bsize)) / (1024 * 1024 * 1024)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"free_gb":     freeGB,
		"total_gb":    totalGB,
		"min_free_gb": 7.0,
		"ok":          freeGB >= 7.0,
	})
}

func (s *Server) handlePeriodos(w http.ResponseWriter, r *http.Request) {
	periodos, err := oracle.GetPeriodos(s.getDB(r), s.schema)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(periodos)
}

func (s *Server) handleGeneraciones(w http.ResponseWriter, r *http.Request) {
	periodo := r.URL.Query().Get("periodo")
	tipo := r.URL.Query().Get("tipo") // "idtramite" o vacío = generacion
	if periodo == "" {
		http.Error(w, "periodo requerido", 400)
		return
	}

	var result interface{}
	var err error
	if tipo == "idtramite" {
		result, err = oracle.GetIdTramites(s.getDB(r), s.schema, periodo)
	} else {
		result, err = oracle.GetGeneraciones(s.getDB(r), s.schema, periodo)
	}
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func (s *Server) handlePlanillas(w http.ResponseWriter, r *http.Request) {
	periodo := r.URL.Query().Get("periodo")
	tipo := r.URL.Query().Get("tipo") // "idtramite" o vacío = generacion
	valorStr := r.URL.Query().Get("generacion")
	if tipo == "idtramite" {
		valorStr = r.URL.Query().Get("generacion") // usamos mismo param "generacion"
	}
	if periodo == "" {
		http.Error(w, "periodo requerido", 400)
		return
	}
	valor, err := strconv.ParseInt(valorStr, 10, 64)
	if err != nil {
		http.Error(w, "valor inválido", 400)
		return
	}

	var tramites []oracle.Tramite
	if tipo == "idtramite" {
		tramites, err = oracle.GetPlanillasByIdTramite(s.getDB(r), s.schema, periodo, valor)
	} else {
		tramites, err = oracle.GetPlanillasByGeneracion(s.getDB(r), s.schema, periodo, valor)
	}
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	// Convertir a TramiteInfo
	var results []TramiteInfo
	for _, tr := range tramites {
		jobs := tr.BuildJobs()
		pdfDir := filepath.Join(s.outDir, fmt.Sprintf("%d", tr.DIG_TRAMITE))
		pdfCount := countPDFs(pdfDir)
		results = append(results, TramiteInfo{
			Tramite: tr.DIG_TRAMITE, Cedula: tr.DIG_CEDULA, Menor: tr.DIG_MENOR_EDAD,
			Tandas: tr.Tandas, TotalPDFs: len(jobs),
			Planilla: tr.DIG_FECHA_PLANILLA, Hasta: tr.DIG_FECHA_HASTA, Alta: tr.DIG_FECHA_ALTA,
			Dep01: tr.DIG_DEPENDIENTE_01, Dep02: tr.DIG_DEPENDIENTE_02,
			HasPDFs: pdfCount > 0, PDFCount: pdfCount,
		})
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}

func (s *Server) warmupBrowser() {
	browser, err := msp.StartBrowser()
	if err != nil {
		fmt.Fprintf(os.Stderr, "[WARM] Error precalentando navegador: %v\n", err)
		return
	}
	fmt.Fprintf(os.Stderr, "[WARM] Navegador pre-calentado y listo\n")
	s.warmMu.Lock()
	s.warmBrowser = browser
	s.warmMu.Unlock()
}

func (s *Server) takeWarmBrowser() *rod.Browser {
	s.warmMu.Lock()
	b := s.warmBrowser
	s.warmBrowser = nil
	s.warmMu.Unlock()
	if b != nil {
		go s.warmupBrowser()
	}
	return b
}
