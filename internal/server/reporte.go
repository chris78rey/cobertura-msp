package server

import (
	"archive/zip"
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

type ConsistencyRow struct {
	DIGID                int64  `json:"dig_id"`
	DIGTramite           int64  `json:"dig_tramite"`
	DIGExpediente        string `json:"dig_expediente"`
	DIGServicio          string `json:"dig_servicio"`
	FEPlaAniomes         string `json:"fe_pla_aniomes"`
	DIGFechaPlanilla     string `json:"dig_fecha_planilla"`
	SISFechaPlanilla     string `json:"sis_fecha_planilla"`
	DIGFechaHasta        string `json:"dig_fecha_hasta"`
	SISFechaHasta        string `json:"sis_fecha_hasta"`
	DIGFechaAlta         string `json:"dig_fecha_alta"`
	SISFechaAlta         string `json:"sis_fecha_alta"`
	CantidadCuentas      int64  `json:"cantidad_cuentas"`
	CantidadPermanencias int64  `json:"cantidad_permanencias"`
	EstadoGeneral        string `json:"estado_general"`
}

const reportHTML = `<!DOCTYPE html>
<html lang="es">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Reporte de consistencia HE-1</title>
<style>
*{box-sizing:border-box;margin:0;padding:0}
body{font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,sans-serif;background:#0f172a;color:#e2e8f0;min-height:100vh}
.container{max-width:1400px;margin:0 auto;padding:20px}
h1{color:#38bdf8;margin-bottom:8px;font-size:1.55em}
.subtitle{color:#94a3b8;font-size:.9em;margin-bottom:16px}
.card{background:#1e293b;border-radius:12px;padding:20px;margin-bottom:16px;border:1px solid #334155}
.card h2{color:#7dd3fc;font-size:1.1em;margin-bottom:12px}
label{display:block;color:#94a3b8;font-size:.85em;margin-bottom:4px;margin-top:10px}
input{width:100%;padding:10px;border-radius:8px;border:1px solid #475569;background:#0f172a;color:#e2e8f0;font-size:.95em}
input:focus{outline:none;border-color:#38bdf8}
button,a.btn{padding:11px 18px;border-radius:8px;border:none;font-size:.95em;font-weight:600;cursor:pointer;text-decoration:none;display:inline-block}
.btn-primary{background:#2563eb;color:#fff}.btn-primary:hover{background:#1d4ed8}
.btn-success{background:#16a34a;color:#fff}.btn-success:hover{background:#15803d}
.btn-warning{background:#d97706;color:#fff}.btn-warning:hover{background:#b45309}
.btn-warning:disabled,.btn-success:disabled,.btn-primary:disabled{background:#475569;cursor:not-allowed}
.row{display:flex;gap:12px;flex-wrap:wrap;align-items:end}
.row>div{flex:1;min-width:200px}
.summary{display:flex;gap:12px;flex-wrap:wrap;margin-top:12px}
.kpi{background:#0f172a;border:1px solid #334155;border-radius:10px;padding:10px 14px;min-width:180px}
.kpi .label{color:#94a3b8;font-size:.75em;text-transform:uppercase;letter-spacing:.05em}
.kpi .value{font-size:1.1em;font-weight:700;margin-top:4px}
.kpi.ok .value{color:#4ade80}.kpi.warn .value{color:#facc15}.kpi.bad .value{color:#f87171}
.table-wrap{overflow:auto;border:1px solid #334155;border-radius:10px}
.table{width:100%;border-collapse:collapse;font-size:.82em;min-width:1250px}
.table th{position:sticky;top:0;background:#334155;color:#7dd3fc;padding:8px 10px;text-align:left;white-space:nowrap}
.table td{padding:7px 10px;border-bottom:1px solid #1e293b;white-space:nowrap;vertical-align:top}
.table tr:hover{background:#1e293b}
.badge{padding:2px 8px;border-radius:4px;font-size:.8em;font-weight:600;display:inline-block}
.badge-ok{background:#16a34a33;color:#4ade80}.badge-bad{background:#dc262633;color:#f87171}
.toast{position:fixed;top:20px;right:20px;padding:12px 20px;border-radius:8px;font-size:.9em;z-index:999;animation:slideIn .3s}
.toast-ok{background:#16a34a;color:#fff}.toast-err{background:#dc2626;color:#fff}
@keyframes slideIn{from{transform:translateX(100%);opacity:0}to{transform:translateX(0);opacity:1}}
.small{font-size:.82em;color:#94a3b8}
</style>
</head>
<body>
<div class="container">
  <h1>📊 Reporte de consistencia de coberturas</h1>
  <p class="subtitle">Validación entre DIGITALIZACION y SIS <a href="/" style="color:#38bdf8;margin-left:10px">← Volver</a></p>

  <div class="card">
    <h2>Filtrar por período</h2>
    <div class="row">
      <div>
        <label>P_FE_PLA_ANIOMES</label>
        <input type="text" id="periodo" placeholder="YYYYMM" maxlength="6" oninput="toggleSyncButton()" onkeydown="if(event.key==='Enter')loadReport()">
        <div class="small">Ejemplo: 202606</div>
      </div>
      <div style="flex:0;min-width:180px">
        <label>Máx. registros</label>
        <input type="number" id="limite" min="1" max="2000" step="1" value="200" onkeydown="if(event.key==='Enter')loadReport()">
        <div class="small">Para evitar el scroll largo</div>
      </div>
      <div style="flex:0;min-width:180px">
        <button class="btn-primary" onclick="loadReport()">Consultar</button>
      </div>
      <div style="flex:0;min-width:220px">
        <button class="btn-success" id="btn-excel" onclick="downloadExcel()" disabled>Descargar Excel</button>
      </div>
      <div style="flex:0;min-width:220px">
        <button class="btn-warning" id="btn-sync" onclick="syncReport()" disabled>Sincronizar fecha alta</button>
      </div>
    </div>
    <div class="summary" id="summary"></div>
    <div class="small" id="sync-status" style="margin-top:10px"></div>
  </div>

  <div class="card">
    <h2>Resultados</h2>
    <div class="table-wrap">
      <table class="table">
        <thead>
          <tr>
            <th>DIG_ID</th><th>TRAMITE</th><th>EXPEDIENTE</th><th>SERVICIO</th><th>PERÍODO</th>
            <th>PLANILLA DIG</th><th>PLANILLA SIS</th><th>HASTA DIG</th><th>HASTA SIS</th><th>ALTA DIG</th><th>ALTA SIS</th>
            <th>CUENTAS</th><th>PERMANENCIAS</th><th>ESTADO</th>
          </tr>
        </thead>
        <tbody id="results-body">
          <tr><td colspan="14" style="color:#94a3b8;padding:16px">Ingresá un período para consultar.</td></tr>
        </tbody>
      </table>
    </div>
  </div>
</div>

<script>
let currentPeriodo = '';
let currentLimit = 200;

function setSyncState(loading, msg) {
  const btn = document.getElementById('btn-sync');
  const st = document.getElementById('sync-status');
  if (btn) {
    btn.disabled = loading || !document.getElementById('periodo').value.trim();
    btn.textContent = loading ? 'Sincronizando...' : 'Sincronizar fecha alta';
  }
  if (st) st.textContent = msg || '';
}

function toggleSyncButton() {
  const btn = document.getElementById('btn-sync');
  if (!btn) return;
  btn.disabled = !document.getElementById('periodo').value.trim();
}

function toast(msg, ok) {
  const d = document.createElement('div');
  d.className = 'toast ' + (ok ? 'toast-ok' : 'toast-err');
  d.textContent = msg;
  document.body.appendChild(d);
  setTimeout(() => d.remove(), 4000);
}

function fmtCell(v) {
  return (v === null || v === undefined || v === '') ? '—' : String(v);
}

function clsForEstado(v) {
  return v === 'CONSISTENTE' ? 'badge badge-ok' : 'badge badge-bad';
}

function renderSummary(rows) {
  const total = rows.length;
  const consistentes = rows.filter(r => r.estado_general === 'CONSISTENTE').length;
  const inconsistentes = total - consistentes;
  const sinCuentas = rows.filter(r => r.estado_general === 'INCONSISTENTE: SIN CUENTAS EN SIS').length;
  const diffPlanilla = rows.filter(r => r.estado_general === 'INCONSISTENTE: FECHA PLANILLA').length;
  const diffHasta = rows.filter(r => r.estado_general === 'INCONSISTENTE: FECHA HASTA').length;
  const diffAlta = rows.filter(r => r.estado_general === 'INCONSISTENTE: FECHA ALTA').length;
  const el = document.getElementById('summary');
  el.innerHTML = [
    ['Total', total, 'ok'],
    ['Consistentes', consistentes, 'ok'],
    ['Inconsistentes', inconsistentes, inconsistentes > 0 ? 'warn' : 'ok'],
    ['Sin cuentas', sinCuentas, sinCuentas > 0 ? 'bad' : 'ok'],
    ['Dif. planilla', diffPlanilla, diffPlanilla > 0 ? 'warn' : 'ok'],
    ['Dif. hasta', diffHasta, diffHasta > 0 ? 'warn' : 'ok'],
    ['Dif. alta', diffAlta, diffAlta > 0 ? 'warn' : 'ok'],
  ].map(([label, value, kind]) => '<div class="kpi ' + kind + '"><div class="label">' + label + '</div><div class="value">' + value + '</div></div>').join('');
}

async function loadReport() {
  const periodo = document.getElementById('periodo').value.trim();
  const limite = parseInt(document.getElementById('limite').value.trim() || '200', 10);
  if (!periodo) { toast('Ingresá el período', false); return; }
  if (!Number.isInteger(limite) || limite < 1 || limite > 2000) { toast('El límite debe estar entre 1 y 2000', false); return; }
  currentPeriodo = periodo;
  currentLimit = limite;
  toggleSyncButton();
  const tbody = document.getElementById('results-body');
  tbody.innerHTML = '<tr><td colspan="14" style="color:#94a3b8;padding:16px">Consultando...</td></tr>';
  try {
    const resp = await fetch('/api/reporte?periodo=' + encodeURIComponent(periodo) + '&limit=' + encodeURIComponent(limite));
    const data = await resp.json();
    if (!resp.ok) throw new Error(data.error || 'Error consultando el reporte');
    if (!Array.isArray(data) || data.length === 0) {
      tbody.innerHTML = '<tr><td colspan="14" style="color:#facc15;padding:16px">No se encontraron registros para ese período.</td></tr>';
      document.getElementById('btn-excel').disabled = true;
      renderSummary([]);
      toast('Sin resultados', false);
      return;
    }
    document.getElementById('btn-excel').disabled = false;
    renderSummary(data);
    tbody.innerHTML = '';
    data.forEach(r => {
      const tr = document.createElement('tr');
      tr.innerHTML =
        '<td>' + fmtCell(r.dig_id) + '</td>' +
        '<td>' + fmtCell(r.dig_tramite) + '</td>' +
        '<td>' + fmtCell(r.dig_expediente) + '</td>' +
        '<td>' + fmtCell(r.dig_servicio) + '</td>' +
        '<td>' + fmtCell(r.fe_pla_aniomes) + '</td>' +
        '<td>' + fmtCell(r.dig_fecha_planilla) + '</td>' +
        '<td>' + fmtCell(r.sis_fecha_planilla) + '</td>' +
        '<td>' + fmtCell(r.dig_fecha_hasta) + '</td>' +
        '<td>' + fmtCell(r.sis_fecha_hasta) + '</td>' +
        '<td>' + fmtCell(r.dig_fecha_alta) + '</td>' +
        '<td>' + fmtCell(r.sis_fecha_alta) + '</td>' +
        '<td>' + fmtCell(r.cantidad_cuentas) + '</td>' +
        '<td>' + fmtCell(r.cantidad_permanencias) + '</td>' +
        '<td><span class="' + clsForEstado(r.estado_general) + '">' + fmtCell(r.estado_general) + '</span></td>';
      tbody.appendChild(tr);
    });
    toast('Reporte cargado: ' + data.length + ' filas', true);
  } catch (e) {
    tbody.innerHTML = '<tr><td colspan="14" style="color:#f87171;padding:16px">' + fmtCell(e.message) + '</td></tr>';
    document.getElementById('btn-excel').disabled = true;
    toast(e.message || 'Error', false);
  }
}

function downloadExcel() {
  if (!currentPeriodo) { toast('Primero consultá un período', false); return; }
  window.location = '/api/reporte/excel?periodo=' + encodeURIComponent(currentPeriodo) + '&limit=' + encodeURIComponent(currentLimit || 200);
}

async function syncReport() {
  const periodo = document.getElementById('periodo').value.trim() || currentPeriodo;
  if (!periodo) { toast('Ingresá el período', false); return; }
  currentPeriodo = periodo;
  setSyncState(true, 'Ejecutando PR_EJECUTA_DIGITALIZACION_SIS para ' + periodo + '...');
  try {
    const resp = await fetch('/api/reporte/sync?periodo=' + encodeURIComponent(periodo), { method: 'POST' });
    const data = await resp.json().catch(() => ({}));
    if (!resp.ok) throw new Error(data.error || 'Error sincronizando');
    toast('Sincronización completada', true);
    setSyncState(false, 'Sincronización lista. Refrescando reporte...');
    await loadReport();
    setSyncState(false, 'Sincronizado para el período ' + periodo + '.');
  } catch (e) {
    setSyncState(false, '');
    toast(e.message || 'Error', false);
  }
}
</script>
</body>
</html>`

func (s *Server) handleConsistencyPage(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/reporte" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(reportHTML))
}

func (s *Server) handleConsistencyReport(w http.ResponseWriter, r *http.Request) {
	periodo := strings.TrimSpace(r.URL.Query().Get("periodo"))
	if periodo == "" {
		http.Error(w, `{"error":"parámetro periodo requerido"}`, http.StatusBadRequest)
		return
	}
	limit, err := parseReportLimit(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	rows, err := s.queryConsistencyReport(r, periodo, limit)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(rows)
}

func (s *Server) handleConsistencyReportExcel(w http.ResponseWriter, r *http.Request) {
	periodo := strings.TrimSpace(r.URL.Query().Get("periodo"))
	if periodo == "" {
		http.Error(w, "parámetro periodo requerido", http.StatusBadRequest)
		return
	}
	limit, err := parseReportLimit(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	rows, err := s.queryConsistencyReport(r, periodo, limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	b, err := buildConsistencyReportXLSX(periodo, rows)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	fname := fmt.Sprintf("reporte_consistencia_%s.xlsx", sanitizeFileName(periodo))
	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", fname))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(b)
}


func (s *Server) handleConsistencySync(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "método no permitido", http.StatusMethodNotAllowed)
		return
	}
	periodo := strings.TrimSpace(r.URL.Query().Get("periodo"))
	if periodo == "" {
		http.Error(w, `{"error":"parámetro periodo requerido"}`, http.StatusBadRequest)
		return
	}
	if err := s.syncConsistencyFechaAlta(r); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"ok": true,
		"periodo": periodo,
	})
}

func (s *Server) syncConsistencyFechaAlta(r *http.Request) error {
	_, err := s.db.Exec(`BEGIN PR_EJECUTA_DIGITALIZACION_SIS; END;`)
	if err != nil {
		return err
	}
	return nil
}

func parseReportLimit(r *http.Request) (int, error) {
	limit := 200
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		v, err := strconv.Atoi(raw)
		if err != nil {
			return 0, fmt.Errorf("límite inválido")
		}
		limit = v
	}
	if limit < 1 || limit > 2000 {
		return 0, fmt.Errorf("el límite debe estar entre 1 y 2000")
	}
	return limit, nil
}

func (s *Server) queryConsistencyReport(r *http.Request, periodo string, limit int) ([]ConsistencyRow, error) {
	q := fmt.Sprintf(`SELECT * FROM (
WITH D_FILTRADO AS (
    SELECT /*+ MATERIALIZE */
           D.DIG_ID,
           D.DIG_TRAMITE,
           D.DIG_EXPEDIENTE,
           D.DIG_SERVICIO,
           D.FE_PLA_ANIOMES,
           TRUNC(D.DIG_FECHA_PLANILLA) AS DIG_FECHA_PLANILLA,
           TRUNC(D.DIG_FECHA_HASTA) AS DIG_FECHA_HASTA,
           TRUNC(D.DIG_FECHA_ALTA) AS DIG_FECHA_ALTA
      FROM %s.DIGITALIZACION D
     WHERE D.FE_PLA_ANIOMES = :1
       AND (
            REGEXP_LIKE(UPPER(TRIM(D.DIG_EXPEDIENTE)), '^(HSP|HSD|QRE|EMR|URG|OBE)(0[1-9]|1[0-2])$')
            OR UPPER(TRIM(D.DIG_SERVICIO)) IN (
                'HOSPITALIZACION', 'HOSPITALIZACIÓN', 'EMERGENCIA', 'URGENCIAS',
                'URGENCIA', 'OBSERVACION', 'OBSERVACIÓN'
            )
       )
),
TRAMITES AS (
    SELECT /*+ MATERIALIZE */ DISTINCT DIG_TRAMITE
      FROM D_FILTRADO
     WHERE DIG_TRAMITE IS NOT NULL
),
DATOS_SIS AS (
    SELECT CTA.PLA_NUMERO_PLANILLA AS DIG_TRAMITE,
           MIN(TRUNC(CTA.FECHA)) AS SIS_FECHA_PLANILLA,
           MAX(TRUNC(CTA.FECHA)) AS SIS_FECHA_HASTA,
           MAX(TRUNC(PYA.FECHA_ALTA)) AS SIS_FECHA_ALTA,
           COUNT(*) AS CANTIDAD_CUENTAS,
           COUNT(DISTINCT CTA.NUMERO_PERMANENCIA) AS CANTIDAD_PERMANENCIAS
      FROM TRAMITES T
      JOIN SIS.CUENTAS CTA ON CTA.PLA_NUMERO_PLANILLA = T.DIG_TRAMITE AND CTA.ESTADO = 'PND'
      LEFT JOIN SIS.PERMANENCIAS_Y_ATENCIONES PYA ON PYA.NUMERO = CTA.NUMERO_PERMANENCIA
  GROUP BY CTA.PLA_NUMERO_PLANILLA
),
COMPARACION AS (
    SELECT D.DIG_ID,
           D.DIG_TRAMITE,
           D.DIG_EXPEDIENTE,
           D.DIG_SERVICIO,
           D.FE_PLA_ANIOMES,
           D.DIG_FECHA_PLANILLA,
           D.DIG_FECHA_HASTA,
           D.DIG_FECHA_ALTA,
           S.SIS_FECHA_PLANILLA,
           S.SIS_FECHA_HASTA,
           S.SIS_FECHA_ALTA,
           NVL(S.CANTIDAD_CUENTAS, 0) AS CANTIDAD_CUENTAS,
           NVL(S.CANTIDAD_PERMANENCIAS, 0) AS CANTIDAD_PERMANENCIAS,
           CASE
             WHEN S.DIG_TRAMITE IS NULL THEN 'INCONSISTENTE: SIN CUENTAS EN SIS'
             WHEN NVL(D.DIG_FECHA_PLANILLA, DATE '1900-01-01') <> NVL(S.SIS_FECHA_PLANILLA, DATE '1900-01-01') THEN 'INCONSISTENTE: FECHA PLANILLA'
             WHEN NVL(D.DIG_FECHA_HASTA, DATE '1900-01-01') <> NVL(S.SIS_FECHA_HASTA, DATE '1900-01-01') THEN 'INCONSISTENTE: FECHA HASTA'
             WHEN NVL(D.DIG_FECHA_ALTA, DATE '1900-01-01') <> NVL(S.SIS_FECHA_ALTA, DATE '1900-01-01') THEN 'INCONSISTENTE: FECHA ALTA'
             ELSE 'CONSISTENTE'
           END AS ESTADO_GENERAL
      FROM D_FILTRADO D
      LEFT JOIN DATOS_SIS S ON S.DIG_TRAMITE = D.DIG_TRAMITE
)
SELECT DIG_ID,
       DIG_TRAMITE,
       DIG_EXPEDIENTE,
       DIG_SERVICIO,
       FE_PLA_ANIOMES,
       DIG_FECHA_PLANILLA,
       SIS_FECHA_PLANILLA,
       DIG_FECHA_HASTA,
       SIS_FECHA_HASTA,
       DIG_FECHA_ALTA,
       SIS_FECHA_ALTA,
       CANTIDAD_CUENTAS,
       CANTIDAD_PERMANENCIAS,
       ESTADO_GENERAL
  FROM COMPARACION
 WHERE ESTADO_GENERAL <> 'CONSISTENTE'
 ORDER BY ESTADO_GENERAL, DIG_EXPEDIENTE, DIG_TRAMITE
) WHERE ROWNUM <= :2`, s.schema)

	rows, err := s.getDB(r).Query(q, periodo, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []ConsistencyRow
	for rows.Next() {
		var row ConsistencyRow
		var expediente, servicio, feP, dPlanilla, sPlanilla, dHasta, sHasta, dAlta, sAlta, estado sql.NullString
		if err := rows.Scan(
			&row.DIGID,
			&row.DIGTramite,
			&expediente,
			&servicio,
			&feP,
			&dPlanilla,
			&sPlanilla,
			&dHasta,
			&sHasta,
			&dAlta,
			&sAlta,
			&row.CantidadCuentas,
			&row.CantidadPermanencias,
			&estado,
		); err != nil {
			return nil, err
		}
		row.DIGExpediente = ns(expediente)
		row.DIGServicio = ns(servicio)
		row.FEPlaAniomes = ns(feP)
		row.DIGFechaPlanilla = ns(dPlanilla)
		row.SISFechaPlanilla = ns(sPlanilla)
		row.DIGFechaHasta = ns(dHasta)
		row.SISFechaHasta = ns(sHasta)
		row.DIGFechaAlta = ns(dAlta)
		row.SISFechaAlta = ns(sAlta)
		row.EstadoGeneral = ns(estado)
		out = append(out, row)
	}
	return out, rows.Err()
}

func buildConsistencyReportXLSX(periodo string, rows []ConsistencyRow) ([]byte, error) {
	headers := []string{"DIG_ID", "DIG_TRAMITE", "DIG_EXPEDIENTE", "DIG_SERVICIO", "FE_PLA_ANIOMES", "DIG_FECHA_PLANILLA", "SIS_FECHA_PLANILLA", "DIG_FECHA_HASTA", "SIS_FECHA_HASTA", "DIG_FECHA_ALTA", "SIS_FECHA_ALTA", "CANTIDAD_CUENTAS", "CANTIDAD_PERMANENCIAS", "ESTADO_GENERAL"}

	var sheet bytes.Buffer
	sheet.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`)
	sheet.WriteString(`<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">`)
	sheet.WriteString(`<sheetData>`)
	writeXLSXRow(&sheet, 1, headers)
	for i, r := range rows {
		rowNum := i + 2
		vals := []string{
			strconv.FormatInt(r.DIGID, 10),
			strconv.FormatInt(r.DIGTramite, 10),
			r.DIGExpediente,
			r.DIGServicio,
			r.FEPlaAniomes,
			r.DIGFechaPlanilla,
			r.SISFechaPlanilla,
			r.DIGFechaHasta,
			r.SISFechaHasta,
			r.DIGFechaAlta,
			r.SISFechaAlta,
			strconv.FormatInt(r.CantidadCuentas, 10),
			strconv.FormatInt(r.CantidadPermanencias, 10),
			r.EstadoGeneral,
		}
		writeXLSXRow(&sheet, rowNum, vals)
	}
	sheet.WriteString(`</sheetData></worksheet>`)

	var out bytes.Buffer
	zw := zip.NewWriter(&out)
	files := map[string]string{
		"[Content_Types].xml": contentTypesXML(),
		"_rels/.rels": relsXML(),
		"xl/workbook.xml": workbookXML(),
		"xl/_rels/workbook.xml.rels": workbookRelsXML(),
		"xl/styles.xml": stylesXML(),
		"xl/worksheets/sheet1.xml": sheet.String(),
	}
	for name, content := range files {
		f, err := zw.Create(name)
		if err != nil {
			return nil, err
		}
		if _, err := f.Write([]byte(content)); err != nil {
			return nil, err
		}
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func writeXLSXRow(buf *bytes.Buffer, rowNum int, values []string) {
	buf.WriteString(`<row r="`)
	buf.WriteString(strconv.Itoa(rowNum))
	buf.WriteString(`">`)
	for i, v := range values {
		cellRef := fmt.Sprintf("%s%d", xlsxColName(i+1), rowNum)
		buf.WriteString(`<c r="`)
		buf.WriteString(cellRef)
		buf.WriteString(`" t="inlineStr"><is><t>`)
		buf.WriteString(xlsxEscape(v))
		buf.WriteString(`</t></is></c>`)
	}
	buf.WriteString(`</row>`)
}

func xlsxColName(n int) string {
	if n <= 0 {
		return "A"
	}
	name := ""
	for n > 0 {
		n--
		name = string(rune('A'+(n%26))) + name
		n /= 26
	}
	return name
}

func xlsxEscape(s string) string {
	r := strings.NewReplacer(`&`, `&amp;`, `<`, `&lt;`, `>`, `&gt;`, `"`, `&quot;`, `'`, `&apos;`)
	return r.Replace(s)
}

func workbookXML() string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
  <sheets>
    <sheet name="Reporte" sheetId="1" r:id="rId1"/>
  </sheets>
</workbook>`
}

func workbookRelsXML() string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/>
</Relationships>`
}

func stylesXML() string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<styleSheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">
  <fonts count="1"><font><sz val="11"/><name val="Calibri"/></font></fonts>
  <fills count="1"><fill><patternFill patternType="none"/></fill></fills>
  <borders count="1"><border/></borders>
  <cellStyleXfs count="1"><xf numFmtId="0" fontId="0" fillId="0" borderId="0"/></cellStyleXfs>
  <cellXfs count="1"><xf numFmtId="0" fontId="0" fillId="0" borderId="0" xfId="0"/></cellXfs>
</styleSheet>`
}

func contentTypesXML() string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/>
  <Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/>
  <Override PartName="/xl/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.styles+xml"/>
</Types>`
}

func relsXML() string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/>
</Relationships>`
}

func sanitizeFileName(s string) string {
	s = strings.TrimSpace(s)
	s = strings.Map(func(r rune) rune {
		switch r {
		case '/', '\\', ':', '*', '?', '"', '<', '>', '|':
			return -1
		default:
			return r
		}
	}, s)
	if s == "" {
		return "reporte"
	}
	return s
}

func ns(v sql.NullString) string {
	if v.Valid {
		return v.String
	}
	return ""
}
