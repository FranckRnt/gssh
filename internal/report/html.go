package report

import (
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"time"

	internalssh "gssh/internal/ssh"
)

// HTMLData holds all data passed to the HTML template.
type HTMLData struct {
	GeneratedAt string
	Duration    string
	Summary     internalssh.Summary
	Results     []internalssh.Result
}

// WriteHTML generates a self-contained HTML report file.
// Returns the path to the generated file.
func WriteHTML(results []internalssh.Result, summary internalssh.Summary, elapsed time.Duration, logDir string) (string, error) {
	data := HTMLData{
		GeneratedAt: time.Now().Format("2006-01-02 15:04:05"),
		Duration:    elapsed.Round(time.Millisecond).String(),
		Summary:     summary,
		Results:     results,
	}

	baseName := fmt.Sprintf("gssh-%s.html", time.Now().Format("2006-01-02_15-04-05"))
	filePath := baseName
	if logDir != "" {
		if err := os.MkdirAll(logDir, 0750); err != nil {
			return "", fmt.Errorf("create log dir %s: %w", logDir, err)
		}
		filePath = filepath.Join(logDir, baseName)
	}

	f, err := os.Create(filePath)
	if err != nil {
		return "", fmt.Errorf("create report file: %w", err)
	}
	defer f.Close()

	if err := htmlTmpl.Execute(f, data); err != nil {
		return "", fmt.Errorf("execute template: %w", err)
	}

	return filePath, nil
}

var htmlTmpl = template.Must(template.New("report").Funcs(template.FuncMap{
	"statusClass": func(r internalssh.Result) string {
		if r.IsSuccess() {
			return "success"
		}
		return "fail"
	},
	"statusLabel": func(r internalssh.Result) string {
		if r.IsSuccess() {
			return "OK"
		}
		return "FAIL"
	},
	"durationMS": func(d time.Duration) string {
		return d.Round(time.Millisecond).String()
	},
	"successRate": func(s internalssh.Summary) string {
		if s.Total == 0 {
			return "0"
		}
		return fmt.Sprintf("%.0f", float64(s.Success)/float64(s.Total)*100)
	},
}).Parse(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>gssh Report — {{.GeneratedAt}}</title>
<link rel="preconnect" href="https://fonts.googleapis.com">
<link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
<link href="https://fonts.googleapis.com/css2?family=DM+Sans:ital,opsz,wght@0,9..40,300;0,9..40,400;0,9..40,500;0,9..40,600;1,9..40,400&family=JetBrains+Mono:wght@400;500&display=swap" rel="stylesheet">
<style>
  :root {
    --bg: #f8f9fb;
    --surface: #ffffff;
    --surface-raised: #ffffff;
    --border: #e2e5ea;
    --border-light: #eef0f3;
    --text-primary: #1a1f2b;
    --text-secondary: #5c6370;
    --text-tertiary: #8b929e;
    --accent: #2563eb;
    --accent-muted: #dbeafe;
    --success: #16a34a;
    --success-bg: #f0fdf4;
    --success-border: #bbf7d0;
    --fail: #dc2626;
    --fail-bg: #fef2f2;
    --fail-border: #fecaca;
    --warn-text: #a16207;
    --warn-bg: #fefce8;
    --warn-border: #fde68a;
    --code-bg: #f4f5f7;
    --hover-bg: #f4f6f9;
    --mono: 'JetBrains Mono', 'SF Mono', 'Cascadia Code', 'Fira Code', monospace;
    --sans: 'DM Sans', -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif;
    --shadow-sm: 0 1px 2px rgba(0,0,0,0.04);
    --shadow-md: 0 2px 8px rgba(0,0,0,0.06), 0 1px 2px rgba(0,0,0,0.04);
    --radius: 8px;
    --radius-sm: 5px;
    --logo-bg: #1a1f2b;
    --logo-fg: #ffffff;
    --active-btn-bg: #1a1f2b;
    --active-btn-fg: #ffffff;
  }

  [data-theme="dark"] {
    --bg: #0f1117;
    --surface: #181a20;
    --surface-raised: #1c1e26;
    --border: #2a2d37;
    --border-light: #23252e;
    --text-primary: #e1e3e8;
    --text-secondary: #9ca0ab;
    --text-tertiary: #6b7080;
    --accent: #5b8def;
    --accent-muted: rgba(91,141,239,0.15);
    --success: #34d058;
    --success-bg: rgba(52,208,88,0.1);
    --success-border: rgba(52,208,88,0.25);
    --fail: #f56565;
    --fail-bg: rgba(245,101,101,0.1);
    --fail-border: rgba(245,101,101,0.25);
    --warn-text: #eab308;
    --warn-bg: rgba(234,179,8,0.08);
    --warn-border: rgba(234,179,8,0.2);
    --code-bg: #14161c;
    --hover-bg: #1f2129;
    --shadow-sm: 0 1px 2px rgba(0,0,0,0.2);
    --shadow-md: 0 2px 8px rgba(0,0,0,0.3), 0 1px 2px rgba(0,0,0,0.2);
    --logo-bg: #e1e3e8;
    --logo-fg: #0f1117;
    --active-btn-bg: #e1e3e8;
    --active-btn-fg: #0f1117;
  }

  * { margin: 0; padding: 0; box-sizing: border-box; }

  body {
    font-family: var(--sans);
    background: var(--bg);
    color: var(--text-primary);
    line-height: 1.5;
    -webkit-font-smoothing: antialiased;
    -moz-osx-font-smoothing: grayscale;
  }

  /* --- Layout --- */
  .page {
    max-width: 1320px;
    margin: 0 auto;
    padding: 40px 48px 80px;
  }

  /* --- Header --- */
  .header {
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    margin-bottom: 36px;
    padding-bottom: 28px;
    border-bottom: 1px solid var(--border);
  }
  .header-left { display: flex; align-items: center; gap: 14px; }
  .logo {
    width: 36px; height: 36px;
    background: var(--logo-bg);
    border-radius: var(--radius-sm);
    display: flex; align-items: center; justify-content: center;
    color: var(--logo-fg);
    font-family: var(--mono);
    font-weight: 600;
    font-size: 14px;
    letter-spacing: -0.5px;
    flex-shrink: 0;
  }
  .header-title {
    font-size: 20px;
    font-weight: 600;
    letter-spacing: -0.3px;
    color: var(--text-primary);
  }
  .header-subtitle {
    font-size: 13px;
    color: var(--text-tertiary);
    margin-top: 1px;
  }
  .header-meta {
    text-align: right;
    font-size: 13px;
    color: var(--text-tertiary);
    line-height: 1.7;
  }
  .header-meta strong {
    color: var(--text-secondary);
    font-weight: 500;
  }
  .header-right {
    display: flex;
    align-items: center;
    gap: 14px;
  }

  /* --- Summary cards --- */
  .cards {
    display: grid;
    grid-template-columns: repeat(4, 1fr);
    gap: 16px;
    margin-bottom: 36px;
  }
  .card {
    background: var(--surface-raised);
    border: 1px solid var(--border);
    border-radius: var(--radius);
    padding: 20px 22px;
    box-shadow: var(--shadow-sm);
    position: relative;
    overflow: hidden;
  }
  .card::before {
    content: '';
    position: absolute;
    top: 0; left: 0; right: 0;
    height: 3px;
  }
  .card.card-total::before { background: var(--accent); }
  .card.card-success::before { background: var(--success); }
  .card.card-failed::before { background: var(--fail); }
  .card.card-duration::before { background: var(--text-tertiary); }
  .card-label {
    font-size: 11.5px;
    font-weight: 500;
    text-transform: uppercase;
    letter-spacing: 0.6px;
    color: var(--text-tertiary);
    margin-bottom: 8px;
  }
  .card-value {
    font-family: var(--mono);
    font-size: 28px;
    font-weight: 500;
    letter-spacing: -1px;
    line-height: 1;
  }
  .card-total .card-value { color: var(--accent); }
  .card-success .card-value { color: var(--success); }
  .card-failed .card-value { color: var(--fail); }
  .card-duration .card-value {
    color: var(--text-primary);
    font-size: 22px;
    letter-spacing: -0.5px;
  }
  .card-detail {
    font-size: 12px;
    color: var(--text-tertiary);
    margin-top: 8px;
  }

  /* --- Progress bar --- */
  .progress-bar-wrap {
    margin-bottom: 36px;
  }
  .progress-labels {
    display: flex;
    justify-content: space-between;
    margin-bottom: 6px;
    font-size: 12px;
    color: var(--text-tertiary);
    font-weight: 500;
  }
  .progress-track {
    height: 6px;
    background: var(--border-light);
    border-radius: 3px;
    overflow: hidden;
    display: flex;
  }
  .progress-fill-ok {
    background: var(--success);
    transition: width 0.4s ease;
  }
  .progress-fill-fail {
    background: var(--fail);
    transition: width 0.4s ease;
  }

  /* --- Filter bar --- */
  .toolbar {
    display: flex;
    align-items: center;
    gap: 10px;
    margin-bottom: 16px;
  }
  .search-wrap {
    position: relative;
    flex: 1;
    max-width: 340px;
  }
  .search-wrap svg {
    position: absolute;
    left: 11px;
    top: 50%;
    transform: translateY(-50%);
    color: var(--text-tertiary);
    pointer-events: none;
  }
  .search {
    width: 100%;
    padding: 8px 12px 8px 34px;
    font-family: var(--sans);
    font-size: 13px;
    border: 1px solid var(--border);
    border-radius: var(--radius-sm);
    background: var(--surface);
    color: var(--text-primary);
    outline: none;
    transition: border-color 0.15s, box-shadow 0.15s;
  }
  .search::placeholder { color: var(--text-tertiary); }
  .search:focus {
    border-color: var(--accent);
    box-shadow: 0 0 0 3px var(--accent-muted);
  }
  .filter-btn {
    padding: 8px 14px;
    font-family: var(--sans);
    font-size: 13px;
    font-weight: 500;
    border: 1px solid var(--border);
    border-radius: var(--radius-sm);
    background: var(--surface);
    color: var(--text-secondary);
    cursor: pointer;
    transition: all 0.15s;
  }
  .filter-btn:hover { border-color: var(--text-tertiary); color: var(--text-primary); }
  .filter-btn.active {
    background: var(--active-btn-bg);
    color: var(--active-btn-fg);
    border-color: var(--active-btn-bg);
  }
  .result-count {
    margin-left: auto;
    font-size: 13px;
    color: var(--text-tertiary);
    font-variant-numeric: tabular-nums;
  }

  /* --- Table --- */
  .table-wrap {
    background: var(--surface-raised);
    border: 1px solid var(--border);
    border-radius: var(--radius);
    box-shadow: var(--shadow-md);
    overflow: hidden;
  }
  table {
    width: 100%;
    border-collapse: collapse;
    font-size: 13px;
  }
  thead { position: sticky; top: 0; z-index: 1; }
  th {
    background: var(--bg);
    color: var(--text-tertiary);
    font-weight: 500;
    font-size: 11.5px;
    text-transform: uppercase;
    letter-spacing: 0.5px;
    text-align: left;
    padding: 10px 16px;
    border-bottom: 1px solid var(--border);
    cursor: pointer;
    user-select: none;
    white-space: nowrap;
    transition: color 0.15s;
  }
  th:hover { color: var(--text-primary); }
  th .sort-icon {
    display: inline-block;
    margin-left: 3px;
    font-size: 10px;
    opacity: 0.35;
    transition: opacity 0.15s;
  }
  th.sorted .sort-icon { opacity: 1; color: var(--accent); }
  td {
    padding: 10px 16px;
    border-bottom: 1px solid var(--border-light);
    vertical-align: top;
    color: var(--text-primary);
  }
  tr:last-child td { border-bottom: none; }
  tr.hidden { display: none; }
  tbody tr { transition: background-color 0.1s; }
  tbody tr:hover td { background: var(--hover-bg); }

  /* Cell types */
  .cell-host {
    font-family: var(--mono);
    font-size: 12.5px;
    font-weight: 500;
    white-space: nowrap;
  }
  .cell-cmd {
    font-family: var(--mono);
    font-size: 12px;
    color: var(--text-secondary);
    max-width: 280px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .cell-cmd:hover {
    white-space: normal;
    word-break: break-all;
  }
  .cell-exit {
    font-family: var(--mono);
    font-size: 12px;
    text-align: center;
    min-width: 44px;
  }
  .cell-dur {
    font-family: var(--mono);
    font-size: 12px;
    color: var(--text-secondary);
    white-space: nowrap;
    text-align: right;
  }

  /* Status pill */
  .pill {
    display: inline-flex;
    align-items: center;
    gap: 5px;
    font-size: 11.5px;
    font-weight: 600;
    letter-spacing: 0.3px;
    padding: 3px 10px;
    border-radius: 100px;
    white-space: nowrap;
  }
  .pill.success {
    background: var(--success-bg);
    color: var(--success);
    border: 1px solid var(--success-border);
  }
  .pill.fail {
    background: var(--fail-bg);
    color: var(--fail);
    border: 1px solid var(--fail-border);
  }
  .pill-dot {
    width: 6px; height: 6px;
    border-radius: 50%;
    flex-shrink: 0;
  }
  .pill.success .pill-dot { background: var(--success); }
  .pill.fail .pill-dot { background: var(--fail); }

  /* Output cell */
  .output-cell { max-width: 520px; min-width: 200px; }
  .output-toggle {
    font-size: 12px;
    color: var(--accent);
    cursor: pointer;
    font-weight: 500;
    display: inline-flex;
    align-items: center;
    gap: 4px;
  }
  .output-toggle:hover { text-decoration: underline; }
  .output-content { display: none; margin-top: 6px; }
  .output-content.open { display: block; }
  .output-content pre {
    font-family: var(--mono);
    font-size: 11.5px;
    line-height: 1.55;
    background: var(--code-bg);
    border: 1px solid var(--border-light);
    border-radius: var(--radius-sm);
    padding: 10px 12px;
    white-space: pre-wrap;
    word-break: break-all;
    max-height: 220px;
    overflow-y: auto;
    color: var(--text-primary);
  }
  .output-error {
    font-family: var(--mono);
    font-size: 12px;
    color: var(--fail);
    margin-bottom: 4px;
    line-height: 1.4;
  }
  .output-stderr pre {
    background: var(--warn-bg);
    color: var(--warn-text);
    border-color: var(--warn-border);
  }
  .output-preview {
    font-family: var(--mono);
    font-size: 11.5px;
    color: var(--text-tertiary);
    max-width: 100%;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  /* --- Theme toggle --- */
  .theme-toggle {
    width: 36px; height: 36px;
    display: flex; align-items: center; justify-content: center;
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: var(--radius-sm);
    cursor: pointer;
    color: var(--text-secondary);
    transition: border-color 0.15s, color 0.15s;
    flex-shrink: 0;
  }
  .theme-toggle:hover { border-color: var(--text-tertiary); color: var(--text-primary); }
  .theme-toggle svg { width: 18px; height: 18px; }

  /* --- Footer --- */
  .footer {
    margin-top: 40px;
    padding-top: 20px;
    border-top: 1px solid var(--border-light);
    font-size: 12px;
    color: var(--text-tertiary);
    display: flex;
    justify-content: space-between;
  }

  /* --- Responsive --- */
  @media (max-width: 900px) {
    .page { padding: 24px 20px 60px; }
    .cards { grid-template-columns: repeat(2, 1fr); }
    .header { flex-direction: column; gap: 12px; }
    .header-meta { text-align: left; }
  }

  /* --- Print --- */
  @media print {
    body { background: #fff; }
    .toolbar { display: none; }
    .page { padding: 20px; }
    .card { box-shadow: none; border: 1px solid #ddd; }
    .table-wrap { box-shadow: none; }
    .output-content { display: block !important; }
  }
</style>
</head>
<body>
<div class="page">

  <!-- Header -->
  <div class="header">
    <div class="header-left">
      <div class="logo">g&gt;</div>
      <div>
        <div class="header-title">Execution Report</div>
        <div class="header-subtitle">gssh — SSH at scale</div>
      </div>
    </div>
    <div class="header-right">
      <div class="header-meta">
        <div><strong>Generated</strong> {{.GeneratedAt}}</div>
        <div><strong>Duration</strong> {{.Duration}}</div>
      </div>
      <button class="theme-toggle" id="themeToggle" title="Toggle dark/light mode" aria-label="Toggle theme">
        <svg id="themeIcon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"></svg>
      </button>
    </div>
  </div>

  <!-- Summary Cards -->
  <div class="cards">
    <div class="card card-total">
      <div class="card-label">Total Hosts</div>
      <div class="card-value">{{.Summary.Total}}</div>
    </div>
    <div class="card card-success">
      <div class="card-label">Success</div>
      <div class="card-value">{{.Summary.Success}}</div>
      <div class="card-detail">{{successRate .Summary}}% success rate</div>
    </div>
    <div class="card card-failed">
      <div class="card-label">Failed</div>
      <div class="card-value">{{.Summary.Failed}}</div>
    </div>
    <div class="card card-duration">
      <div class="card-label">Total Duration</div>
      <div class="card-value">{{.Duration}}</div>
    </div>
  </div>

  <!-- Progress bar -->
  <div class="progress-bar-wrap">
    <div class="progress-labels">
      <span>Execution overview</span>
      <span>{{.Summary.Success}} passed / {{.Summary.Failed}} failed</span>
    </div>
    <div class="progress-track">
      <div class="progress-fill-ok" id="barOk"></div>
      <div class="progress-fill-fail" id="barFail"></div>
    </div>
  </div>

  <!-- Toolbar -->
  <div class="toolbar">
    <div class="search-wrap">
      <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="11" cy="11" r="8"/><line x1="21" y1="21" x2="16.65" y2="16.65"/></svg>
      <input type="text" class="search" id="searchInput" placeholder="Filter by host, command, or output...">
    </div>
    <button class="filter-btn active" data-filter="all" onclick="setFilter('all', this)">All</button>
    <button class="filter-btn" data-filter="success" onclick="setFilter('success', this)">Passed</button>
    <button class="filter-btn" data-filter="fail" onclick="setFilter('fail', this)">Failed</button>
    <span class="result-count" id="resultCount"></span>
  </div>

  <!-- Results Table -->
  <div class="table-wrap">
    <table id="results">
    <thead>
    <tr>
      <th onclick="sortTable(0)" style="width:15%">Host <span class="sort-icon">&#8597;</span></th>
      <th onclick="sortTable(1)" style="width:22%">Command <span class="sort-icon">&#8597;</span></th>
      <th onclick="sortTable(2)" style="width:8%">Status <span class="sort-icon">&#8597;</span></th>
      <th onclick="sortTable(3)" style="width:6%">Exit <span class="sort-icon">&#8597;</span></th>
      <th onclick="sortTable(4)" style="width:10%">Duration <span class="sort-icon">&#8597;</span></th>
      <th style="cursor:default">Output</th>
    </tr>
    </thead>
    <tbody>
    {{range $i, $r := .Results}}
    <tr data-status="{{statusClass $r}}" data-search="{{$r.Hostname}} {{$r.Command}} {{$r.Output}} {{$r.Error}}">
      <td class="cell-host">{{$r.Hostname}}</td>
      <td class="cell-cmd" title="{{$r.Command}}">{{$r.Command}}</td>
      <td><span class="pill {{statusClass $r}}"><span class="pill-dot"></span>{{statusLabel $r}}</span></td>
      <td class="cell-exit">{{$r.ReturnCode}}</td>
      <td class="cell-dur">{{durationMS $r.Duration}}</td>
      <td class="output-cell">
        {{- if $r.Error}}<div class="output-error">{{$r.Error}}</div>{{end -}}
        {{- if or $r.Output $r.Stderr -}}
          {{- if $r.Output}}<div class="output-preview" id="preview-{{$i}}">{{$r.Output}}</div>{{end -}}
          <span class="output-toggle" onclick="toggleOutput({{$i}})"><span id="toggleIcon-{{$i}}">&#9654;</span> details</span>
          <div class="output-content" id="output-{{$i}}">
            {{- if $r.Output}}<pre>{{$r.Output}}</pre>{{end -}}
            {{- if $r.Stderr}}<div class="output-stderr"><pre>{{$r.Stderr}}</pre></div>{{end -}}
          </div>
        {{- end -}}
      </td>
    </tr>
    {{end}}
    </tbody>
    </table>
  </div>

  <div class="footer">
    <span>gssh execution report</span>
    <span>{{.GeneratedAt}}</span>
  </div>

</div>

<script>
// Theme toggle
(function() {
  const sunPath = '<circle cx="12" cy="12" r="5"/><line x1="12" y1="1" x2="12" y2="3"/><line x1="12" y1="21" x2="12" y2="23"/><line x1="4.22" y1="4.22" x2="5.64" y2="5.64"/><line x1="18.36" y1="18.36" x2="19.78" y2="19.78"/><line x1="1" y1="12" x2="3" y2="12"/><line x1="21" y1="12" x2="23" y2="12"/><line x1="4.22" y1="19.78" x2="5.64" y2="18.36"/><line x1="18.36" y1="5.64" x2="19.78" y2="4.22"/>';
  const moonPath = '<path d="M21 12.79A9 9 0 1 1 11.21 3 7 7 0 0 0 21 12.79z"/>';
  const icon = document.getElementById('themeIcon');
  const stored = localStorage.getItem('gssh-theme');
  const prefersDark = window.matchMedia('(prefers-color-scheme: dark)').matches;
  let theme = stored || (prefersDark ? 'dark' : 'light');
  function apply(t) {
    theme = t;
    document.documentElement.setAttribute('data-theme', t);
    icon.innerHTML = t === 'dark' ? sunPath : moonPath;
    localStorage.setItem('gssh-theme', t);
  }
  apply(theme);
  document.getElementById('themeToggle').addEventListener('click', function() {
    apply(theme === 'dark' ? 'light' : 'dark');
  });
})();
(function() {
  // Progress bar
  const total = {{.Summary.Total}} || 1;
  const ok = {{.Summary.Success}};
  const fail = {{.Summary.Failed}};
  document.getElementById('barOk').style.width = (ok/total*100)+'%';
  document.getElementById('barFail').style.width = (fail/total*100)+'%';

  updateCount();

  // Toggle output
  window.toggleOutput = function(i) {
    const el = document.getElementById('output-'+i);
    const icon = document.getElementById('toggleIcon-'+i);
    const preview = document.getElementById('preview-'+i);
    const open = el.classList.toggle('open');
    icon.innerHTML = open ? '&#9660;' : '&#9654;';
    if (preview) preview.style.display = open ? 'none' : '';
  };

  // Search filter
  const input = document.getElementById('searchInput');
  let currentFilter = 'all';
  input.addEventListener('input', applyFilters);

  window.setFilter = function(f, btn) {
    currentFilter = f;
    document.querySelectorAll('.filter-btn').forEach(b => b.classList.remove('active'));
    btn.classList.add('active');
    applyFilters();
  };

  function applyFilters() {
    const q = input.value.toLowerCase();
    const rows = document.querySelectorAll('#results tbody tr');
    rows.forEach(row => {
      const matchesSearch = !q || row.getAttribute('data-search').toLowerCase().includes(q);
      const status = row.getAttribute('data-status');
      const matchesFilter = currentFilter === 'all' || status === currentFilter;
      row.classList.toggle('hidden', !(matchesSearch && matchesFilter));
    });
    updateCount();
  }

  function updateCount() {
    const visible = document.querySelectorAll('#results tbody tr:not(.hidden)').length;
    const total = document.querySelectorAll('#results tbody tr').length;
    document.getElementById('resultCount').textContent = visible === total
      ? total + ' results'
      : visible + ' of ' + total + ' results';
  }

  // Sort
  let sortDir = {};
  window.sortTable = function(col) {
    const tbody = document.querySelector('#results tbody');
    const rows = Array.from(tbody.querySelectorAll('tr'));
    sortDir[col] = !sortDir[col];

    // Update sorted state on headers
    document.querySelectorAll('#results th').forEach((th, i) => {
      th.classList.toggle('sorted', i === col);
    });

    rows.sort((a, b) => {
      let va = a.cells[col].textContent.trim();
      let vb = b.cells[col].textContent.trim();
      if (col === 3) { va = parseInt(va)||0; vb = parseInt(vb)||0; return sortDir[col] ? va-vb : vb-va; }
      if (col === 4) {
        const pa = parseFloat(va) || 0;
        const pb = parseFloat(vb) || 0;
        return sortDir[col] ? pa-pb : pb-pa;
      }
      if (va < vb) return sortDir[col] ? -1 : 1;
      if (va > vb) return sortDir[col] ? 1 : -1;
      return 0;
    });
    rows.forEach(r => tbody.appendChild(r));
  };
})();
</script>
</body>
</html>
`))
