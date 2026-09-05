package server

const indexHTML = `<!DOCTYPE html>
<html lang="es">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Cobertura de Salud HE-1</title>
<style>
*{box-sizing:border-box;margin:0;padding:0}
body{font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,sans-serif;background:#0f172a;color:#e2e8f0;min-height:100vh}
.container{max-width:960px;margin:0 auto;padding:20px}
h1{color:#38bdf8;margin-bottom:5px;font-size:1.5em}
.subtitle{color:#94a3b8;font-size:.85em;margin-bottom:4px}
.credit{color:#475569;font-size:.75em;margin-bottom:20px}
.card{background:#1e293b;border-radius:12px;padding:20px;margin-bottom:16px;border:1px solid #334155}
.card h2{color:#7dd3fc;font-size:1.1em;margin-bottom:12px}
.guide{display:flex;gap:16px;margin-bottom:20px;flex-wrap:wrap}
.guide-step{flex:1;min-width:160px;background:#1e293b;border-radius:10px;padding:14px 16px;border:1px solid #334155;text-align:center}
.guide-step .num{display:inline-block;width:28px;height:28px;line-height:28px;border-radius:50%;background:#2563eb;color:#fff;font-size:.85em;font-weight:700;margin-bottom:6px}
.guide-step p{font-size:.78em;color:#94a3b8;line-height:1.4}
.guide-step strong{color:#7dd3fc}
.tabs{display:flex;gap:4px;margin-bottom:16px}
.tab{padding:8px 20px;border-radius:8px 8px 0 0;background:#1e293b;color:#94a3b8;border:1px solid #334155;border-bottom:none;cursor:pointer;font-size:.9em;font-weight:600}
.tab.active{background:#2563eb;color:#fff;border-color:#2563eb}
.tab-content{display:none}
.tab-content.active{display:block}
label{display:block;color:#94a3b8;font-size:.85em;margin-bottom:4px;margin-top:10px}
input,select{width:100%;padding:10px;border-radius:8px;border:1px solid #475569;background:#0f172a;color:#e2e8f0;font-size:.95em}
input:focus,select:focus{outline:none;border-color:#38bdf8}
select option{background:#1e293b;color:#e2e8f0}
.row{display:flex;gap:12px}
.row>*{flex:1}
button{padding:12px 24px;border-radius:8px;border:none;font-size:1em;font-weight:600;cursor:pointer;transition:all .2s}
.btn-primary{background:#2563eb;color:#fff}
.btn-primary:hover{background:#1d4ed8}
.btn-primary:disabled{background:#475569;cursor:not-allowed}
.btn-success{background:#16a34a;color:#fff}
.btn-success:hover{background:#15803d}
.btn-success:disabled{background:#475569;cursor:not-allowed}
.btn-sm{padding:6px 14px;font-size:.85em;border-radius:6px}
.results-table{width:100%;border-collapse:collapse;margin-top:12px;font-size:.85em}
.results-table th{background:#334155;color:#7dd3fc;padding:8px 12px;text-align:left}
.results-table td{padding:8px 12px;border-bottom:1px solid #1e293b}
.results-table tr:hover{background:#1e293b}
.badge{padding:2px 8px;border-radius:4px;font-size:.8em;font-weight:600}
.badge-ok{background:#16a34a33;color:#4ade80}
.badge-partial{background:#eab30833;color:#facc15}
.badge-none{background:#dc262633;color:#f87171}
.toast{position:fixed;top:20px;right:20px;padding:12px 20px;border-radius:8px;font-size:.9em;z-index:999;animation:slideIn .3s}
.toast-ok{background:#16a34a;color:#fff}
.toast-err{background:#dc2626;color:#fff}
@keyframes slideIn{from{transform:translateX(100%);opacity:0}to{transform:translateX(0);opacity:1}}
.hidden{display:none}
.folder-link{color:#38bdf8;text-decoration:none;font-size:.85em;background:#1e293b;padding:8px 16px;border-radius:6px;border:1px solid #334155;display:inline-block;margin-top:8px}
.folder-link:hover{background:#334155}
.disk-bar{display:flex;align-items:center;gap:10px;margin-bottom:16px;font-size:.82em}
.disk-bar .bar-bg{flex:1;height:8px;background:#334155;border-radius:4px;overflow:hidden}
.disk-bar .bar-fill{height:100%;border-radius:4px;transition:width .5s,background .5s}
.disk-bar .bar-ok{background:#16a34a}
.disk-bar .bar-warn{background:#eab308}
.disk-bar .bar-crit{background:#dc2626}
.disk-warning{background:#7f1d1d;border:1px solid #dc2626;border-radius:8px;padding:12px 16px;margin-bottom:16px;color:#fca5a5;font-size:.85em;display:none}
.disk-warning.show{display:block}
#btn-generate:disabled{opacity:.6}
.card.active-generation{border-color:#38bdf8;box-shadow:0 0 20px rgba(56,189,248,0.15)}
.progress-bar{height:10px;background:#334155;border-radius:5px;overflow:hidden;margin-top:8px}
.progress-fill{height:100%;background:linear-gradient(90deg,#2563eb,#38bdf8);transition:width .3s;border-radius:5px}
.progress-bar.pulse .progress-fill{animation:progressPulse 1.5s ease-in-out infinite}
@keyframes progressPulse{0%,100%{opacity:1}50%{opacity:.7}}
.step-row{display:flex;gap:12px;align-items:end;flex-wrap:wrap}
.step-row>div{flex:1;min-width:180px}
.step-label{font-size:.75em;color:#38bdf8;margin-bottom:2px;font-weight:600}
.gen-count{font-size:.75em;color:#94a3b8;margin-left:6px}
</style>
</head>
<body>
<div class="container">

  <div id="login-screen" style="max-width:400px;margin:60px auto">
    <div class="card" style="text-align:center">
      <h2>🔐 Iniciar Sesión</h2>
      <p style="color:#94a3b8;font-size:.85em;margin-bottom:16px">Ingresá tus credenciales de Oracle</p>
      <div style="text-align:left">
        <label>Usuario</label>
        <input type="text" id="login-user" placeholder="Tu usuario Oracle" onkeydown="if(event.key==='Enter')doLogin()">
        <label>Contraseña</label>
        <input type="password" id="login-pass" placeholder="Tu contraseña" onkeydown="if(event.key==='Enter')doLogin()">
      </div>
      <p id="login-error" style="color:#f87171;font-size:.8em;margin-top:8px;display:none"></p>
      <button class="btn-primary" onclick="doLogin()" style="margin-top:12px;width:100%">Ingresar</button>
    </div>
  </div>

  <div id="main-app" class="hidden">
  <h1>🏥 Cobertura de Salud HE-1</h1>
  <p class="subtitle">Consulta y descarga tus documentos de cobertura <span id="user-display" style="color:#4ade80"></span> <a href="#" onclick="openReporte();return false" style="color:#38bdf8;font-size:.8em;margin-left:10px">📊 Reporte</a> <a href="#" onclick="doLogout();return false" style="color:#f87171;font-size:.8em;margin-left:10px">Salir</a></p>
  <p class="credit" style="color:#38bdf8;font-size:.9em;font-weight:600;margin-bottom:20px;letter-spacing:1px">✦ Elaborado por DTIC HE1 ✦</p>

  <div class="disk-bar">
    <span>💾 Espacio:</span>
    <div class="bar-bg"><div class="bar-fill" id="disk-fill" style="width:70%"></div></div>
    <span id="disk-text" style="color:#94a3b8">-- GB libres</span>
  </div>

  <div class="tabs">
    <div class="tab active" onclick="switchTab('periodo')">📅 Por período</div>
    <div class="tab" onclick="switchTab('rango')">🔢 Por rango</div>
  </div>

  <div id="tab-periodo" class="tab-content active">
    <div class="card">
      <h2>📅 Buscar por período</h2>
      <div class="step-row">
        <div>
          <div class="step-label">PASO 1 — Período</div>
          <select id="periodo-select" onchange="onPeriodoChange()"><option value="">Cargando...</option></select>
        </div>
        <div>
          <div class="step-label">PASO 2 — 
            <select id="tipo-select" onchange="onPeriodoChange()" style="width:auto;display:inline;padding:2px 6px;background:transparent;border:none;color:#38bdf8;font-weight:600;cursor:pointer">
              <option value="generacion">Generación</option>
              <option value="idtramite">ID Trámite</option>
            </select>
            <span id="gen-info" class="gen-count"></span>
          </div>
          <select id="generacion-select" onchange="onGeneracionChange()" disabled><option value="">Primero elegí un período</option></select>
        </div>
        <div style="flex:0">
          <button class="btn-primary" id="btn-load-planillas" onclick="loadPlanillas()" disabled>🔍 Buscar</button>
        </div>
      </div>
    </div>
  </div>

  <div id="tab-rango" class="tab-content">
    <div class="card">
      <h2>🔍 Buscar por rango</h2>
      <div class="row">
        <div><label>Desde (N° Planilla)</label><input type="number" id="from" placeholder="6019091"></div>
        <div><label>Hasta (N° Planilla)</label><input type="number" id="to" placeholder="6019105"></div>
        <div style="flex:0;align-self:end"><button class="btn-primary" onclick="doQuery()">🔍 Buscar</button></div>
      </div>
    </div>
  </div>

  <div id="results-card" class="card hidden">
    <h2>📊 Resultados <span id="results-count"></span></h2>
    <div style="margin-bottom:12px;display:flex;gap:8px;flex-wrap:wrap">
      <button class="btn-primary btn-sm" onclick="selectAll()">Todas</button>
      <button class="btn-primary btn-sm" onclick="selectIncomplete()">Pendientes</button>
      <button class="btn-primary btn-sm" onclick="selectNone()">Ninguna</button>
      <button class="btn-success" onclick="doGenerate()" id="btn-generate">📥 Descargar documentos</button>
      <span id="time-estimate" style="color:#94a3b8;font-size:.8em;align-self:center"></span>
    </div>
    <table class="results-table">
      <thead><tr><th><input type="checkbox" id="check-all" onchange="toggleAll(this)"></th><th>N° Planilla</th><th>Cédula</th><th>¿Menor?</th><th>Períodos</th><th>Docs</th><th>Estado</th></tr></thead>
      <tbody id="results-body"></tbody>
    </table>
  </div>

  <div id="status-card" class="card hidden">
    <h2>📥 Descargando documentos <span id="elapsed" style="color:#94a3b8;font-size:.8em;font-weight:normal"></span></h2>
    <div class="progress-bar"><div class="progress-fill" id="progress" style="width:0%"></div></div>
    <p id="progress-text" style="margin-top:8px;color:#94a3b8;font-size:.85em"></p>
    <div id="download-all" class="hidden" style="margin-top:12px"></div>
  </div>

  </div><!-- /main-app -->
</div><!-- /container -->

<script>
let coberturas = [];
let pollInterval = null;
let diskOK = true;
let sessionToken = localStorage.getItem('oracle_token') || sessionStorage.getItem('oracle_token') || '';
let sessionUser = localStorage.getItem('oracle_user') || sessionStorage.getItem('oracle_user') || '';

// ========== LOGIN ==========
async function doLogin() {
  const user = document.getElementById('login-user').value.trim();
  const pass = document.getElementById('login-pass').value.trim();
  const errEl = document.getElementById('login-error');
  if (!user || !pass) { errEl.textContent = 'Ingresá usuario y contraseña'; errEl.style.display = 'block'; return; }
  errEl.style.display = 'none';
  try {
    const resp = await fetch('/api/login', {method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({username:user,password:pass})});
    if (!resp.ok) { errEl.textContent = await resp.text(); errEl.style.display = 'block'; return; }
    const data = await resp.json();
    sessionToken = data.token; sessionUser = data.username;
    localStorage.setItem('oracle_token', sessionToken);
    localStorage.setItem('oracle_user', sessionUser);
    sessionStorage.setItem('oracle_token', sessionToken);
    sessionStorage.setItem('oracle_user', sessionUser);
    document.getElementById('login-screen').style.display = 'none';
    document.getElementById('main-app').classList.remove('hidden');
    document.getElementById('user-display').textContent = ' | ' + sessionUser;
    if (periodosLoaded) onPeriodoChange();
  } catch(e) { errEl.textContent = 'Error de conexión'; errEl.style.display = 'block'; }
}

function openReporte() {
  window.location = '/reporte';
}

function doLogout() {
  fetch('/api/logout?token=' + sessionToken);
  sessionToken = ''; sessionUser = '';
  localStorage.removeItem('oracle_token'); localStorage.removeItem('oracle_user');
  sessionStorage.removeItem('oracle_token'); sessionStorage.removeItem('oracle_user');
  document.getElementById('main-app').classList.add('hidden');
  document.getElementById('login-screen').style.display = '';
  document.getElementById('login-pass').value = '';
}

if (sessionToken) {
  document.getElementById('login-screen').style.display = 'none';
  document.getElementById('main-app').classList.remove('hidden');
  document.getElementById('user-display').textContent = ' | ' + sessionUser;
}

function withToken(url) { return sessionToken ? url + (url.includes('?')?'&':'?') + 'token=' + sessionToken : url; }

// ========== TABS ==========
function switchTab(tab) {
  document.querySelectorAll('.tab').forEach(t => t.classList.remove('active'));
  document.querySelectorAll('.tab-content').forEach(c => c.classList.remove('active'));
  document.querySelector('.tab:nth-child(' + (tab==='periodo'?1:2) + ')').classList.add('active');
  document.getElementById('tab-' + tab).classList.add('active');
  if (tab === 'periodo' && !periodosLoaded) loadPeriodos();
}

// ========== DISCO ==========
async function updateDisk() {
  try {
    const resp = await fetch(withToken('/api/disk')); const d = await resp.json();
    const fill = document.getElementById('disk-fill'), text = document.getElementById('disk-text'), warn = document.getElementById('disk-warning');
    const pct = d.total_gb > 0 ? (d.free_gb/d.total_gb*100) : 70;
    fill.style.width = pct + '%';
    if (d.ok) { fill.className='bar-fill bar-ok'; text.textContent = d.free_gb.toFixed(1)+' GB libres de '+d.total_gb.toFixed(0)+' GB'; warn.classList.remove('show'); diskOK=true; }
    else { fill.className = d.free_gb>7?'bar-fill bar-warn':'bar-fill bar-crit'; text.textContent='⚠️ '+d.free_gb.toFixed(1)+' GB libres'; warn.classList.add('show'); diskOK=false; }
  } catch(e) {}
}
updateDisk(); setInterval(updateDisk, 30000);

// ========== PERIODO/GENERACION ==========
let periodosLoaded = false;
async function loadPeriodos() {
  try {
    const resp = await fetch(withToken('/api/periodos')); const periodos = await resp.json();
    const sel = document.getElementById('periodo-select');
    sel.innerHTML = '<option value="">Seleccioná un período...</option>';
    periodos.forEach(p => { sel.innerHTML += '<option value="'+p+'">'+p+'</option>'; });
    periodosLoaded = true;
  } catch(e) { toast('Error cargando períodos', false); }
}
loadPeriodos();

async function onPeriodoChange() {
  const periodo = document.getElementById('periodo-select').value;
  const genSel = document.getElementById('generacion-select'), btn = document.getElementById('btn-load-planillas'), info = document.getElementById('gen-info');
  const tipo = document.getElementById('tipo-select').value, label = tipo==='idtramite'?'ID Trámite':'Generación';
  if (!periodo) { genSel.innerHTML='<option value="">Primero elegí un período</option>'; genSel.disabled=true; btn.disabled=true; info.textContent=''; return; }
  genSel.innerHTML = '<option value="">Cargando...</option>'; genSel.disabled = true; btn.disabled = true;
  try {
    const url = withToken('/api/generaciones?periodo='+periodo+(tipo==='idtramite'?'&tipo=idtramite':''));
    const resp = await fetch(url); const gens = await resp.json();
    genSel.innerHTML = '<option value="">Todos ('+gens.length+')</option>';
    gens.forEach(g => { genSel.innerHTML += '<option value="'+g+'">'+label+' '+g+'</option>'; });
    genSel.disabled = false; btn.disabled = false; info.textContent = '('+gens.length+' disponibles)';
  } catch(e) { genSel.innerHTML='<option value="">Error</option>'; }
}

function onGeneracionChange() {}

async function loadPlanillas() {
  const periodo = document.getElementById('periodo-select').value, gen = document.getElementById('generacion-select').value;
  const tipo = document.getElementById('tipo-select').value;
  if (!periodo) { toast('Seleccioná un período', false); return; }
  let url = withToken('/api/planillas?periodo='+periodo); if (gen) url += '&generacion='+gen; if (tipo==='idtramite') url += '&tipo=idtramite';
  try {
    const resp = await fetch(url);
    if (!resp.ok) { toast('Error al consultar', false); return; }
    coberturas = await resp.json();
    renderResults();
    document.getElementById('results-card').classList.remove('hidden');
    if (coberturas.length===0) toast('No se encontraron planillas', false);
    else toast(coberturas.length+' planillas encontradas', true);
  } catch(e) { toast('Error de conexión', false); }
}

// ========== RANGO ==========
async function doQuery() {
  const from = parseInt(document.getElementById('from').value), to = parseInt(document.getElementById('to').value);
  if (!from||!to||from>to) { toast('Rango inválido', false); return; }
  try {
    const resp = await fetch('/api/query', {method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({from,to})});
    coberturas = await resp.json(); renderResults();
    document.getElementById('results-card').classList.remove('hidden');
    if (coberturas.length===0) toast('No se encontraron coberturas', false);
    else toast(coberturas.length+' coberturas encontradas', true);
  } catch(e) { toast('Error de conexión', false); }
}

// ========== RESULTADOS ==========
let completadas = new Set();
function toast(msg, ok) { const d=document.createElement('div'); d.className='toast '+(ok?'toast-ok':'toast-err'); d.textContent=msg; document.body.appendChild(d); setTimeout(()=>d.remove(),4000); }

function renderResults() {
  const tbody = document.getElementById('results-body'); tbody.innerHTML = '';
  document.getElementById('results-count').textContent = '('+coberturas.length+')';
  coberturas.forEach(t => {
    const tr = document.createElement('tr');
    const complete = t.has_pdfs && t.pdf_count>=t.total_pdfs, partial = t.has_pdfs && t.pdf_count<t.total_pdfs;
    let badge, badgeClass;
    if (complete) { badge='✅ '+t.pdf_count+'/'+t.total_pdfs; badgeClass='badge-ok'; }
    else if (partial) { badge='⚠️ '+t.pdf_count+'/'+t.total_pdfs; badgeClass='badge-partial'; }
    else { badge='Pendiente'; badgeClass='badge-none'; }
    tr.innerHTML = '<td><input type="checkbox" class="tram-check" value="'+t.tramite+'" onchange="updateEstimate()"></td>'+
      '<td>'+t.tramite+'</td><td>'+t.cedula+'</td><td>'+(t.menor==='S'?'✅':'—')+'</td><td>'+t.tandas+'</td><td>'+t.total_pdfs+'</td>'+
      '<td><span class="badge '+badgeClass+'">'+badge+'</span>'+(t.has_pdfs?' <a href="#" onclick="downloadAll('+t.tramite+');return false" style="color:#38bdf8;font-size:.8em;margin-left:4px">📥 Descargar</a>':'')+'</td>';
    tbody.appendChild(tr);
  });
  updateEstimate();
}

function updateEstimate() {
  const selected = [...document.querySelectorAll('.tram-check:checked')].map(c=>parseInt(c.value));
  const el = document.getElementById('time-estimate');
  if (selected.length===0) { el.textContent=''; return; }
  const totalSecs = selected.length*45, mins=Math.floor(totalSecs/60), secs=totalSecs%60;
  el.textContent = mins>0?'⏱ ~'+mins+' min'+(secs>0?' '+secs+'s':'')+' para '+selected.length+' coberturas':'⏱ ~'+secs+'s';
}
function selectAll() { document.querySelectorAll('.tram-check').forEach(c=>c.checked=true); document.getElementById('check-all').checked=true; updateEstimate(); }
function selectNone() { document.querySelectorAll('.tram-check').forEach(c=>c.checked=false); document.getElementById('check-all').checked=false; updateEstimate(); }
function selectIncomplete() { document.querySelectorAll('.tram-check').forEach((c,i)=>{const t=coberturas[i];c.checked=!t.has_pdfs||t.pdf_count<t.total_pdfs;}); updateEstimate(); }
function toggleAll(el) { document.querySelectorAll('.tram-check').forEach(c=>c.checked=el.checked); updateEstimate(); }

// ========== GENERAR ==========
async function doGenerate() {
  const selected = [...document.querySelectorAll('.tram-check:checked')].map(c=>parseInt(c.value));
  if (selected.length===0) { toast('Seleccioná al menos una cobertura', false); return; }
  if (selected.length>2000) { toast('Máximo 2000 coberturas. Seleccionaste '+selected.length, false); return; }
  if (!diskOK) { toast('Espacio insuficiente', false); return; }

  document.getElementById('btn-generate').disabled = true;
  const sc = document.getElementById('status-card');
  sc.classList.remove('hidden'); sc.classList.add('active-generation');
  sc.scrollIntoView({behavior:'smooth',block:'start'});
  document.getElementById('download-all').classList.add('hidden');
  document.getElementById('download-all').innerHTML = '';
  document.getElementById('progress').style.width = '0%';
  document.getElementById('progress-text').textContent = 'Consultando el MSP...';
  document.getElementById('elapsed').textContent = '00:00';
  completadas.clear();

  try {
    const resp = await fetch('/api/generate', {method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({tramites:selected,workers:1,client_id:clientId,token:sessionToken})});
    if (!resp.ok) { const errText = await resp.text(); toast(errText.includes('espacio')?'Espacio insuficiente':'Error', false); document.getElementById('btn-generate').disabled=false; return; }
    const data = await resp.json();
    let msg = 'Descargando '+data.job_ids.length+' coberturas...';
    if (data.skipped>0) msg += ' ('+data.skipped+' ya tenían documentos)';
    toast(msg, true);
    startPolling();
  } catch(e) { toast('Error de conexión', false); document.getElementById('btn-generate').disabled=false; }
}

let clientId = sessionStorage.getItem('cobertura_client_id');
if (!clientId) { clientId='cli_'+Date.now()+'_'+Math.random().toString(36).substr(2,6); sessionStorage.setItem('cobertura_client_id',clientId); }

let genStartTime = null;
function startPolling() { if(pollInterval)clearInterval(pollInterval); genStartTime=Date.now(); pollInterval=setInterval(pollStatus,1000); document.querySelector('.progress-bar').classList.add('pulse'); }
function fmtElapsed(ms) { const s=Math.floor(ms/1000),m=Math.floor(s/60),sec=s%60; return String(m).padStart(2,'0')+':'+String(sec).padStart(2,'0'); }

function playDone() { try { const ctx=new(window.AudioContext||window.webkitAudioContext)(); [800,1000].forEach((freq,i)=>{const o=ctx.createOscillator(),g=ctx.createGain();o.type='sine';o.frequency.value=freq;g.gain.value=0.15;g.gain.exponentialRampToValueAtTime(0.001,ctx.currentTime+0.3);o.connect(g);g.connect(ctx.destination);o.start(ctx.currentTime+i*0.15);o.stop(ctx.currentTime+i*0.15+0.3);}); } catch(e) {} }

async function pollStatus() {
  try {
    const resp = await fetch('/api/status?client='+clientId); const jobs = await resp.json();
    let totalAll=0, doneAll=0, failedAll=0, allDone=true;
    let totalCob=jobs.length, doneCob=0;
    jobs.forEach(j => { totalAll+=j.total; doneAll+=j.completed; failedAll+=j.failed; if(j.status==='running'||j.status==='pending')allDone=false; if(j.status==='done')doneCob++; if(j.status==='done'&&j.completed>0)completadas.add(j.tramite); });
    const pct = totalAll>0?Math.round((doneAll/totalAll)*100):0;
    document.getElementById('progress').style.width = pct+'%';
    if(genStartTime) document.getElementById('elapsed').textContent = '⏱ '+fmtElapsed(Date.now()-genStartTime);
    document.getElementById('progress-text').textContent = doneAll+' de '+totalAll+' documentos · '+doneCob+' de '+totalCob+' coberturas ('+pct+'%)'+(failedAll>0?' · '+failedAll+' no se pudo':'');
    if(!allDone&&pct>0&&genStartTime) { const eta=((Date.now()-genStartTime)/pct)*(100-pct); document.getElementById('progress-text').textContent+=' · ⏳ ~'+fmtElapsed(eta)+' restante'; }
    if(allDone&&jobs.length>0) {
      clearInterval(pollInterval); pollInterval=null; genStartTime=null;
      document.querySelector('.progress-bar').classList.remove('pulse');
      document.getElementById('status-card').classList.remove('active-generation');
      document.getElementById('elapsed').textContent='✅ Completado';
      document.getElementById('btn-generate').disabled=false;
      playDone(); toast('¡Listo! '+doneAll+' documentos en '+totalCob+' coberturas descargados', true);
      if(completadas.size>0) { const dlAll=document.getElementById('download-all'); dlAll.classList.remove('hidden'); dlAll.innerHTML='<div style="display:flex;gap:8px"><a href="/api/download-zip" class="folder-link" style="background:#16a34a;color:#fff;border-color:#16a34a;font-size:.95em;padding:10px 20px">📦 Descargar ZIP ('+totalCob+' coberturas / '+doneAll+' documentos)</a></div>'; }
    }
  } catch(e) {}
}

async function downloadAll(tramite) { try { const resp=await fetch('/api/list-pdfs?tramite='+tramite); const pdfs=await resp.json(); if(pdfs.length===0){toast('No hay documentos',false);return;} pdfs.forEach(pdf=>{window.open('/api/download/'+tramite+'/'+pdf,'_blank');}); } catch(e) {} }

document.addEventListener('keydown', e => { if(e.key==='Escape'){document.getElementById('results-card').classList.add('hidden');} });
</script>
</body>
</html>`
