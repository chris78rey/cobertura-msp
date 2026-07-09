package msp

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/jung-kurt/gofpdf"
)

const (
	MSP_BASE_URL         = "https://coberturasalud.msp.gob.ec/"
	ACTION_GET_CAPTCHA   = "40e5613a02e25c0dfb759fd7f199149081432edf13"
	ACTION_API_CLIENT    = "70987a4dcfb783907102d476e4a450486019bbcc62"
)

// cryptoJSAESEncrypt emula CryptoJS.AES.encrypt(message, passphrase)
// Usa EVP_BytesToKey con MD5 para derivar key+IV, AES-256-CBC, PKCS7 padding.
// Output: base64("Salted__" + salt + ciphertext)
func cryptoJSAESEncrypt(plaintext, passphrase string) (string, error) {
	salt := make([]byte, 8)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}

	key, iv := evpBytesToKey([]byte(passphrase), salt, 32, 16)

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	padded := pkcs7Pad([]byte(plaintext), aes.BlockSize)
	ciphertext := make([]byte, len(padded))
	mode := cipher.NewCBCEncrypter(block, iv)
	mode.CryptBlocks(ciphertext, padded)

	// OpenSSL/CryptoJS format: "Salted__" + salt + ciphertext
	buf := make([]byte, 8+8+len(ciphertext))
	copy(buf[0:8], []byte("Salted__"))
	copy(buf[8:16], salt)
	copy(buf[16:], ciphertext)

	return base64.StdEncoding.EncodeToString(buf), nil
}

// evpBytesToKey emula EVP_BytesToKey(password, salt, keyLen, ivLen) con 1 iteración MD5
func evpBytesToKey(password, salt []byte, keyLen, ivLen int) ([]byte, []byte) {
	totalLen := keyLen + ivLen
	derived := make([]byte, 0, totalLen)
	prev := []byte{}

	for len(derived) < totalLen {
		h := md5.New()
		h.Write(prev)
		h.Write(password)
		h.Write(salt)
		prev = h.Sum(nil)
		derived = append(derived, prev...)
	}

	return derived[:keyLen], derived[keyLen : keyLen+ivLen]
}

func pkcs7Pad(data []byte, blockSize int) []byte {
	padLen := blockSize - len(data)%blockSize
	pad := bytes.Repeat([]byte{byte(padLen)}, padLen)
	return append(data, pad...)
}

// csconsulta calcula el checksum sumando dígitos en pares consecutivos
func csconsultaCalc(cedula string) string {
	var out strings.Builder
	for i := 0; i < len(cedula); i += 2 {
		a := int(cedula[i] - '0')
		b := 0
		if i+1 < len(cedula) {
			b = int(cedula[i+1] - '0')
		}
		out.WriteString(fmt.Sprintf("%d", a+b))
	}
	return out.String()
}

// toPortalDate convierte YYYY-MM-DD → DD-MM-YYYY
func toPortalDate(fechaIso string) string {
	parts := strings.Split(fechaIso, "-")
	if len(parts) != 3 {
		return fechaIso
	}
	return parts[2] + "-" + parts[1] + "-" + parts[0]
}

// rscResponse parsea una respuesta RSC de Next.js (formato texto con líneas "1:...")
type rscResponse struct {
	Payload json.RawMessage
	Raw     string
	Cookie  string
}

// postAction hace un POST a la acción RSC del MSP
func postAction(actionID string, body interface{}, cookie string) (*rscResponse, error) {
	bodyJSON, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("json marshal: %w", err)
	}

	req, err := http.NewRequest("POST", MSP_BASE_URL, bytes.NewReader(bodyJSON))
	if err != nil {
		return nil, fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("Content-Type", "text/plain;charset=UTF-8")
	req.Header.Set("Next-Action", actionID)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	if cookie != "" {
		req.Header.Set("Cookie", cookie)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http post: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	text := string(bodyBytes)

	// Extraer cookie
	var newCookie string
	for _, c := range resp.Header["Set-Cookie"] {
		if idx := strings.Index(c, ";"); idx > 0 {
			newCookie = c[:idx]
			break
		}
	}
	if newCookie == "" {
		newCookie = cookie
	}

	// Parsear payload RSC (línea que empieza con "1:")
	payload, err := parseRSCPayload(text)
	if err != nil {
		return nil, err
	}

	return &rscResponse{
		Payload: payload,
		Raw:     text,
		Cookie:  newCookie,
	}, nil
}

// parseRSCPayload busca la línea "1:" en la respuesta RSC y parsea el JSON
func parseRSCPayload(text string) (json.RawMessage, error) {
	lines := strings.Split(text, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "1:") {
			return json.RawMessage(line[2:]), nil
		}
	}
	return nil, fmt.Errorf("respuesta RSC no reconocida: %s", text[:min(200, len(text))])
}

// QueryResult es el resultado de una consulta a la API del MSP
type QueryResult struct {
	Cedula string
	Fecha  string
	Data   json.RawMessage // JSON crudo de la respuesta
	Error  error
}

// QueryAPI consulta la API interna del MSP para una cédula y fecha
func QueryAPI(cedula, fecha string) (*QueryResult, error) {
	// Validar
	if len(cedula) != 10 {
		return nil, fmt.Errorf("cédula inválida: %s", cedula)
	}

	// Paso 1: obtener captcha/token
	reqID := randomUUID()
	captchaResp, err := postAction(ACTION_GET_CAPTCHA, []string{reqID}, "")
	if err != nil {
		return nil, fmt.Errorf("getCaptcha: %w", err)
	}

	var captchaData struct {
		Success string `json:"success"`
		Data    struct {
			Token string `json:"token"`
			ReqID string `json:"reqId"`
		} `json:"data"`
	}
	if err := json.Unmarshal(captchaResp.Payload, &captchaData); err != nil {
		return nil, fmt.Errorf("parse captcha: %w", err)
	}
	if captchaData.Success != "success" {
		return nil, fmt.Errorf("captcha falló: %s", string(captchaResp.Payload)[:200])
	}
	token := captchaData.Data.Token

	// Paso 2: cifrar y consultar
	encCedula, err := cryptoJSAESEncrypt(cedula, token)
	if err != nil {
		return nil, fmt.Errorf("crypto cedula: %w", err)
	}
	encFecha, err := cryptoJSAESEncrypt(toPortalDate(fecha), token)
	if err != nil {
		return nil, fmt.Errorf("crypto fecha: %w", err)
	}

	queryBody := []interface{}{
		"cobertura",
		"POST",
		map[string]string{
			"identificacion": encCedula,
			"fechaConsulta":  encFecha,
			"token":          token,
			"csconsulta":     csconsultaCalc(cedula),
		},
	}

	resultResp, err := postAction(ACTION_API_CLIENT, queryBody, captchaResp.Cookie)
	if err != nil {
		return nil, fmt.Errorf("consulta: %w", err)
	}

	return &QueryResult{
		Cedula: cedula,
		Fecha:  fecha,
		Data:   resultResp.Payload,
	}, nil
}

// randomUUID genera un UUID v4 simple
func randomUUID() string {
	b := make([]byte, 16)
	rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// GeneratePDF usa el script de codex para generar un PDF con el formato oficial del MSP
func GeneratePDF(result *QueryResult, outputPath string) error {
	if result.Data == nil {
		return fmt.Errorf("sin datos para generar PDF")
	}

	// Buscar el script de codex
	codexDir := filepath.Join(os.Getenv("HOME"), "codex_projects", "cobertura")
	genScript := filepath.Join(codexDir, "scripts", "generate_pdf.js")
	if _, err := os.Stat(genScript); os.IsNotExist(err) {
		// Fallback: generar PDF simple con gofpdf
		return generateSimplePDF(result, outputPath)
	}

	// Crear JSON temporal con el formato que espera codex
	// La API devuelve {"data": {...}}, necesitamos {"response": {"data": {...}}}
	var apiResp struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(result.Data, &apiResp); err != nil {
		return fmt.Errorf("parse api response: %w", err)
	}

	wrapper := map[string]interface{}{
		"response": map[string]interface{}{
			"data": json.RawMessage(apiResp.Data),
		},
	}
	jsonBytes, err := json.Marshal(wrapper)
	if err != nil {
		return fmt.Errorf("json marshal: %w", err)
	}

	tmpJSON, err := os.CreateTemp("", "cobertura_*.json")
	if err != nil {
		return fmt.Errorf("tmp file: %w", err)
	}
	tmpJSON.Write(jsonBytes)
	tmpJSON.Close()
	defer os.Remove(tmpJSON.Name())

	// Directorio de salida temporal
	tmpDir, err := os.MkdirTemp("", "cobertura_pdf_")
	if err != nil {
		return fmt.Errorf("tmp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	// Llamar al script Node.js
	cmd := exec.Command("node", genScript,
		"--input_json", tmpJSON.Name(),
		"--cedula", result.Cedula,
		"--fecha", result.Fecha,
		"--output_dir", tmpDir,
	)
	cmd.Dir = codexDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("generate_pdf.js: %w\n%s", err, string(output))
	}

	// Buscar el PDF generado y moverlo al destino
	entries, err := os.ReadDir(tmpDir)
	if err != nil {
		return fmt.Errorf("read output dir: %w", err)
	}
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".pdf") {
			src := filepath.Join(tmpDir, e.Name())
			data, err := os.ReadFile(src)
			if err != nil {
				continue
			}
			return os.WriteFile(outputPath, data, 0644)
		}
	}

	return fmt.Errorf("no se encontró PDF en la salida de generate_pdf.js")
}

// generateSimplePDF es el fallback cuando no está disponible el script de codex
func generateSimplePDF(result *QueryResult, outputPath string) error {
	if result.Data == nil {
		return fmt.Errorf("sin datos para generar PDF")
	}

	var resp struct {
		Data struct {
			CoberturaSalud struct {
				CoberturaSeguros struct {
					Aseguradora []struct {
						CoberturaSalud         string `json:"CoberturaSalud"`
						EstadoCobertura        string `json:"EstadoCobertura"`
						EstadoOperacion        string `json:"EstadoOperacion"`
						MensajeServicioExterno string `json:"MensajeServicioExterno"`
					} `json:"aseguradora"`
				} `json:"CoberturaSeguros"`
			} `json:"coberturaSalud"`
			CoberturaPrivada struct {
				Mensaje string `json:"Mensaje"`
				RegistrosAsegurados struct {
					RegistroAsegurado []struct {
						ApellidosBeneficiario      string `json:"ApellidosBeneficiario"`
						IdentificacionBeneficiario string `json:"IdentificacionBeneficiario"`
						NombreBeneficiario         string `json:"NombreBeneficiario"`
						NombreFinanciador          string `json:"NombreFinanciador"`
						RucEmpresa                 string `json:"RucEmpresa"`
					} `json:"RegistroAsegurado"`
				} `json:"RegistrosAsegurados"`
			} `json:"coberturaPrivada"`
		} `json:"data"`
	}
	json.Unmarshal(result.Data, &resp)

	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.AddPage()

	// Encabezado
	pdf.SetFont("Helvetica", "B", 14)
	pdf.CellFormat(190, 10, "Cobertura de Salud", "", 1, "C", false, 0, "")
	pdf.SetFont("Helvetica", "", 9)
	pdf.CellFormat(190, 6, fmt.Sprintf("Cedula: %s    Fecha: %s", result.Cedula, result.Fecha), "", 1, "C", false, 0, "")
	pdf.Ln(4)

	// Cobertura Privada
	pdf.SetFont("Helvetica", "B", 11)
	pdf.Cell(190, 8, "Cobertura Privada")
	pdf.Ln(10)
	pdf.SetFont("Helvetica", "", 9)

	for _, reg := range resp.Data.CoberturaPrivada.RegistrosAsegurados.RegistroAsegurado {
		pdf.SetFont("Helvetica", "B", 9)
		pdf.Cell(50, 6, "Beneficiario:")
		pdf.SetFont("Helvetica", "", 9)
		pdf.CellFormat(140, 6, fmt.Sprintf("%s %s", reg.NombreBeneficiario, reg.ApellidosBeneficiario), "", 1, "L", false, 0, "")
		pdf.Cell(50, 6, "Identificacion:")
		pdf.CellFormat(140, 6, reg.IdentificacionBeneficiario, "", 1, "L", false, 0, "")
		pdf.Cell(50, 6, "Financiador:")
		pdf.CellFormat(140, 6, reg.NombreFinanciador, "", 1, "L", false, 0, "")
		pdf.Cell(50, 6, "RUC:")
		pdf.CellFormat(140, 6, reg.RucEmpresa, "", 1, "L", false, 0, "")
		pdf.Ln(3)
	}

	// Cobertura Salud (IESS)
	pdf.SetFont("Helvetica", "B", 11)
	pdf.Cell(190, 8, "Cobertura de Salud (IESS)")
	pdf.Ln(10)

	for _, as := range resp.Data.CoberturaSalud.CoberturaSeguros.Aseguradora {
		pdf.SetFont("Helvetica", "B", 9)
		pdf.Cell(50, 6, "Estado:")
		pdf.SetFont("Helvetica", "", 9)
		pdf.CellFormat(140, 6, as.EstadoCobertura, "", 1, "L", false, 0, "")
		pdf.Cell(50, 6, "Nro. Cobertura:")
		pdf.CellFormat(140, 6, as.CoberturaSalud, "", 1, "L", false, 0, "")
		if as.MensajeServicioExterno != "" {
			pdf.SetFont("Helvetica", "", 8)
			pdf.Cell(50, 6, "Detalle:")
			pdf.MultiCell(140, 5, as.MensajeServicioExterno, "", "L", false)
		}
		pdf.Ln(3)
	}

	// Pie
	pdf.SetY(-15)
	pdf.SetFont("Helvetica", "I", 7)
	pdf.CellFormat(190, 5, fmt.Sprintf("Generado: %s  |  DTIC HE1", time.Now().Format("02-01-2006 15:04")), "", 0, "C", false, 0, "")

	return pdf.OutputFileAndClose(outputPath)
}
