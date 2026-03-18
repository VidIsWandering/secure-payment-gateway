package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// DashboardUI serves the embedded single-page merchant dashboard application.
func DashboardUI(c *gin.Context) {
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(dashboardHTML))
}

const dashboardHTML = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1.0" />
  <title>Secure Payment Gateway — Dashboard</title>
  <link rel="preconnect" href="https://fonts.googleapis.com" />
  <link rel="preconnect" href="https://fonts.gstatic.com" crossorigin />
  <link href="https://fonts.googleapis.com/css2?family=Inter:wght@300;400;500;600;700&display=swap" rel="stylesheet" />
  <script src="https://cdn.jsdelivr.net/npm/chart.js@4.4.0/dist/chart.umd.min.js"></script>
  <style>
    *, *::before, *::after { box-sizing: border-box; margin: 0; padding: 0; }

    :root {
      --bg:        #0b0f1a;
      --surface:   #111827;
      --surface2:  #1a2234;
      --border:    rgba(255,255,255,0.07);
      --accent:    #6366f1;
      --accent2:   #818cf8;
      --green:     #10b981;
      --red:       #f43f5e;
      --yellow:    #f59e0b;
      --blue:      #3b82f6;
      --text:      #f1f5f9;
      --muted:     #64748b;
      --radius:    14px;
    }

    html, body { height: 100%; font-family: 'Inter', sans-serif; background: var(--bg); color: var(--text); }

    /* ── LOGIN ── */
    #login-view {
      display: flex; align-items: center; justify-content: center;
      min-height: 100vh;
      background: radial-gradient(ellipse at 60% 40%, #1e1b4b 0%, var(--bg) 70%);
    }
    .login-card {
      width: 400px; padding: 2.5rem 2rem;
      background: var(--surface); border: 1px solid var(--border);
      border-radius: var(--radius); box-shadow: 0 25px 60px rgba(0,0,0,0.5);
    }
    .login-logo {
      display: flex; align-items: center; gap: .6rem;
      margin-bottom: 2rem;
    }
    .login-logo svg { width: 32px; height: 32px; }
    .login-logo span { font-size: 1.1rem; font-weight: 700; color: var(--text); }
    .login-card h2 { font-size: 1.5rem; font-weight: 700; margin-bottom: .3rem; }
    .login-card p { color: var(--muted); font-size: .85rem; margin-bottom: 1.5rem; }
    label { display: block; font-size: .8rem; font-weight: 500; color: var(--muted); margin-bottom: .4rem; }
    input[type=text], input[type=password] {
      width: 100%; padding: .65rem .9rem;
      background: var(--bg); border: 1px solid var(--border);
      border-radius: 8px; color: var(--text); font-size: .9rem;
      outline: none; transition: border-color .2s;
    }
    input:focus { border-color: var(--accent); }
    .field { margin-bottom: 1rem; }
    .btn-primary {
      width: 100%; padding: .75rem; margin-top: .5rem;
      background: var(--accent); color: #fff; border: none;
      border-radius: 8px; font-size: .95rem; font-weight: 600;
      cursor: pointer; transition: background .2s, transform .1s;
    }
    .btn-primary:hover { background: var(--accent2); }
    .btn-primary:active { transform: scale(0.98); }
    .login-error { color: var(--red); font-size: .8rem; margin-top: .75rem; text-align: center; min-height: 1rem; }

    /* ── APP LAYOUT ── */
    #app-view { display: none; flex-direction: column; min-height: 100vh; }
    header {
      display: flex; align-items: center; justify-content: space-between;
      padding: 1rem 2rem; background: var(--surface);
      border-bottom: 1px solid var(--border);
      position: sticky; top: 0; z-index: 100;
    }
    .header-left { display: flex; align-items: center; gap: 1rem; }
    .header-logo { display: flex; align-items: center; gap: .5rem; font-size: .95rem; font-weight: 700; }
    .header-logo svg { width: 26px; height: 26px; }
    .badge {
      padding: .25rem .65rem; border-radius: 999px; font-size: .75rem; font-weight: 600;
    }
    .badge-blue { background: rgba(99,102,241,.2); color: var(--accent2); }
    .header-right { display: flex; align-items: center; gap: 1rem; }
    .balance-chip {
      display: flex; align-items: center; gap: .5rem;
      padding: .4rem .9rem; background: var(--surface2);
      border: 1px solid var(--border); border-radius: 999px; font-size: .82rem;
    }
    .balance-chip strong { color: var(--green); font-size: .9rem; }
    .btn-logout {
      padding: .4rem .9rem; background: transparent; border: 1px solid var(--border);
      border-radius: 8px; color: var(--muted); font-size: .82rem; cursor: pointer;
      transition: border-color .2s, color .2s;
    }
    .btn-logout:hover { border-color: var(--red); color: var(--red); }
    .merchant-name { font-size: .85rem; color: var(--muted); }

    main { flex: 1; padding: 2rem; max-width: 1400px; margin: 0 auto; width: 100%; }
    .page-header { display: flex; align-items: center; justify-content: space-between; margin-bottom: 1.8rem; }
    .page-header h1 { font-size: 1.4rem; font-weight: 700; }
    .period-tabs { display: flex; gap: .3rem; }
    .period-btn {
      padding: .4rem .85rem; border: 1px solid var(--border); border-radius: 8px;
      background: transparent; color: var(--muted); font-size: .8rem; cursor: pointer;
      transition: all .2s;
    }
    .period-btn.active { background: var(--accent); border-color: var(--accent); color: #fff; font-weight: 600; }

    /* ── KPI CARDS ── */
    .kpi-grid { display: grid; grid-template-columns: repeat(4,1fr); gap: 1rem; margin-bottom: 1.5rem; }
    @media(max-width:900px){ .kpi-grid { grid-template-columns: repeat(2,1fr); } }
    .kpi-card {
      background: var(--surface); border: 1px solid var(--border);
      border-radius: var(--radius); padding: 1.2rem 1.4rem;
      transition: box-shadow .2s;
    }
    .kpi-card:hover { box-shadow: 0 8px 30px rgba(0,0,0,.3); }
    .kpi-label { font-size: .75rem; font-weight: 500; color: var(--muted); margin-bottom: .5rem; text-transform: uppercase; letter-spacing: .05em; }
    .kpi-value { font-size: 1.7rem; font-weight: 700; }
    .kpi-icon { font-size: 1.5rem; float: right; opacity: 0.6; }
    .kpi-sub { font-size: .78rem; color: var(--muted); margin-top: .4rem; }

    /* ── CHART ── */
    .charts-row { display: grid; grid-template-columns: 2fr 1fr; gap: 1rem; margin-bottom: 1.5rem; }
    @media(max-width:900px){ .charts-row { grid-template-columns: 1fr; } }
    .card {
      background: var(--surface); border: 1px solid var(--border);
      border-radius: var(--radius); padding: 1.4rem;
    }
    .card-header { display: flex; align-items: center; justify-content: space-between; margin-bottom: 1rem; }
    .card-header h3 { font-size: .95rem; font-weight: 600; }
    .card-header span { font-size: .78rem; color: var(--muted); }
    .chart-wrap { position: relative; height: 220px; }
    .donut-wrap { display: flex; flex-direction: column; align-items: center; }
    .donut-wrap canvas { max-width: 160px; max-height: 160px; margin-bottom: 1rem; }
    .legend { width: 100%; }
    .legend-item { display: flex; justify-content: space-between; align-items: center; font-size: .8rem; padding: .3rem 0; border-bottom: 1px solid var(--border); }
    .legend-item:last-child { border-bottom: none; }
    .legend-dot { width: 10px; height: 10px; border-radius: 50%; margin-right: .5rem; display: inline-block; }
    .legend-left { display: flex; align-items: center; color: var(--muted); }

    /* ── TABLE ── */
    .table-card { background: var(--surface); border: 1px solid var(--border); border-radius: var(--radius); overflow: hidden; }
    .table-toolbar {
      display: flex; align-items: center; gap: .75rem; padding: 1rem 1.4rem;
      border-bottom: 1px solid var(--border); flex-wrap: wrap;
    }
    .table-toolbar h3 { font-size: .95rem; font-weight: 600; flex: 1; }
    .filter-select {
      padding: .4rem .7rem; background: var(--bg); border: 1px solid var(--border);
      border-radius: 8px; color: var(--text); font-size: .8rem; cursor: pointer;
    }
    .btn-refresh {
      padding: .4rem .8rem; background: var(--surface2); border: 1px solid var(--border);
      border-radius: 8px; color: var(--text); font-size: .8rem; cursor: pointer;
      display: flex; align-items: center; gap: .4rem; transition: background .2s;
    }
    .btn-refresh:hover { background: var(--border); }
    table { width: 100%; border-collapse: collapse; font-size: .84rem; }
    th { padding: .8rem 1.2rem; text-align: left; color: var(--muted); font-weight: 500; font-size: .75rem; text-transform: uppercase; letter-spacing: .05em; background: var(--surface2); }
    td { padding: .9rem 1.2rem; border-top: 1px solid var(--border); vertical-align: middle; }
    tr:hover td { background: rgba(255,255,255,0.02); }
    .tx-id { font-family: monospace; font-size: .78rem; color: var(--muted); }
    .pill {
      display: inline-block; padding: .2rem .6rem; border-radius: 999px; font-size: .72rem; font-weight: 600;
    }
    .pill-success { background: rgba(16,185,129,.15); color: var(--green); }
    .pill-fail    { background: rgba(244,63,94,.15);  color: var(--red); }
    .pill-reversed{ background: rgba(245,158,11,.15); color: var(--yellow); }
    .pill-payment { background: rgba(99,102,241,.15); color: var(--accent2); }
    .pill-refund  { background: rgba(244,63,94,.12);  color: var(--red); }
    .pill-topup   { background: rgba(16,185,129,.12); color: var(--green); }
    .amount-pos { color: var(--green); font-weight: 600; }
    .amount-neg { color: var(--red); font-weight: 600; }
    .pagination { display: flex; align-items: center; justify-content: space-between; padding: .9rem 1.4rem; border-top: 1px solid var(--border); font-size: .82rem; color: var(--muted); }
    .page-btns { display: flex; gap: .4rem; }
    .page-btn {
      padding: .3rem .65rem; background: transparent; border: 1px solid var(--border);
      border-radius: 6px; color: var(--muted); cursor: pointer; font-size: .8rem; transition: all .2s;
    }
    .page-btn:hover:not(:disabled) { border-color: var(--accent); color: var(--accent); }
    .page-btn:disabled { opacity: 0.35; cursor: not-allowed; }
    .page-btn.current { background: var(--accent); border-color: var(--accent); color: #fff; }

    .empty-row td { text-align: center; padding: 2.5rem; color: var(--muted); }
    .spinner { display:inline-block; width:16px; height:16px; border:2px solid var(--border); border-top-color: var(--accent); border-radius:50%; animation: spin .6s linear infinite; }
    @keyframes spin { to { transform:rotate(360deg); } }

    .live-dot { width:8px; height:8px; border-radius:50%; background:var(--green); display:inline-block; animation: pulse 1.8s ease-in-out infinite; }
    @keyframes pulse { 0%,100%{ opacity:1; box-shadow:0 0 0 0 rgba(16,185,129,.4);} 50%{ opacity:.8; box-shadow:0 0 0 5px rgba(16,185,129,0);} }
  </style>
</head>
<body>

<!-- ── LOGIN VIEW ── -->
<div id="login-view">
  <div class="login-card">
    <div class="login-logo">
      <svg viewBox="0 0 32 32" fill="none" xmlns="http://www.w3.org/2000/svg">
        <rect width="32" height="32" rx="8" fill="#6366f1"/>
        <path d="M8 20v-8l8-4 8 4v8l-8 4-8-4z" stroke="#fff" stroke-width="1.5" stroke-linejoin="round"/>
        <path d="M16 8v16M8 12l8 4 8-4" stroke="#fff" stroke-width="1.5"/>
      </svg>
      <span>Secure Payment Gateway</span>
    </div>
    <h2>Merchant Dashboard</h2>
    <p>Sign in to your merchant account to continue.</p>
    <div class="field">
      <label for="login-username">Username</label>
      <input id="login-username" type="text" placeholder="Enter your username" autocomplete="username" />
    </div>
    <div class="field">
      <label for="login-password">Password</label>
      <input id="login-password" type="password" placeholder="Enter your password" autocomplete="current-password" />
    </div>
    <button class="btn-primary" id="login-btn">Sign In</button>
    <div class="login-error" id="login-error"></div>
  </div>
</div>

<!-- ── APP VIEW ── -->
<div id="app-view">
  <header>
    <div class="header-left">
      <div class="header-logo">
        <svg viewBox="0 0 32 32" fill="none" xmlns="http://www.w3.org/2000/svg">
          <rect width="32" height="32" rx="8" fill="#6366f1"/>
          <path d="M8 20v-8l8-4 8 4v8l-8 4-8-4z" stroke="#fff" stroke-width="1.5" stroke-linejoin="round"/>
          <path d="M16 8v16M8 12l8 4 8-4" stroke="#fff" stroke-width="1.5"/>
        </svg>
        <span>Payment Gateway</span>
      </div>
      <span class="badge badge-blue">Merchant Dashboard</span>
    </div>
    <div class="header-right">
      <div class="balance-chip">
        <span class="live-dot"></span>
        <span>Balance:</span>
        <strong id="header-balance">—</strong>
      </div>
      <span class="merchant-name" id="header-merchant">—</span>
      <button class="btn-logout" id="logout-btn">Sign out</button>
    </div>
  </header>

  <main>
    <div class="page-header">
      <h1>Overview</h1>
      <div class="period-tabs">
        <button class="period-btn active" data-period="all">All time</button>
        <button class="period-btn" data-period="month">Month</button>
        <button class="period-btn" data-period="week">Week</button>
        <button class="period-btn" data-period="day">Today</button>
      </div>
    </div>

    <!-- KPI -->
    <div class="kpi-grid">
      <div class="kpi-card">
        <div class="kpi-icon">💰</div>
        <div class="kpi-label">Total Revenue</div>
        <div class="kpi-value" id="kpi-revenue">—</div>
        <div class="kpi-sub">Successful payments</div>
      </div>
      <div class="kpi-card">
        <div class="kpi-icon">✅</div>
        <div class="kpi-label">Successful Txns</div>
        <div class="kpi-value" id="kpi-success">—</div>
        <div class="kpi-sub" id="kpi-rate">—</div>
      </div>
      <div class="kpi-card">
        <div class="kpi-icon">🔄</div>
        <div class="kpi-label">Refunds</div>
        <div class="kpi-value" id="kpi-refunds">—</div>
        <div class="kpi-sub" id="kpi-refunded-amt">—</div>
      </div>
      <div class="kpi-card">
        <div class="kpi-icon">📊</div>
        <div class="kpi-label">Total Transactions</div>
        <div class="kpi-value" id="kpi-total">—</div>
        <div class="kpi-sub" id="kpi-failed-sub">—</div>
      </div>
    </div>

    <!-- Charts row -->
    <div class="charts-row">
      <div class="card">
        <div class="card-header">
          <h3>Transaction Volume</h3>
          <span id="chart-sub-label">Breakdown by type</span>
        </div>
        <div class="chart-wrap"><canvas id="bar-chart"></canvas></div>
      </div>
      <div class="card">
        <div class="card-header">
          <h3>Status Distribution</h3>
          <span>By outcome</span>
        </div>
        <div class="donut-wrap">
          <canvas id="donut-chart"></canvas>
          <div class="legend" id="donut-legend"></div>
        </div>
      </div>
    </div>

    <!-- Transaction Table -->
    <div class="table-card">
      <div class="table-toolbar">
        <h3>Transaction History</h3>
        <select class="filter-select" id="filter-status">
          <option value="">All statuses</option>
          <option value="SUCCESS">Success</option>
          <option value="FAILED">Failed</option>
          <option value="REVERSED">Reversed</option>
        </select>
        <select class="filter-select" id="filter-type">
          <option value="">All types</option>
          <option value="PAYMENT">Payment</option>
          <option value="REFUND">Refund</option>
          <option value="TOPUP">Top-up</option>
        </select>
        <button class="btn-refresh" id="refresh-btn">
          <span>&#8635;</span> Refresh
        </button>
      </div>
      <table>
        <thead>
          <tr>
            <th>Reference ID</th>
            <th>Type</th>
            <th>Amount (VND)</th>
            <th>Status</th>
            <th>Date</th>
            <th>Tx ID</th>
          </tr>
        </thead>
        <tbody id="tx-tbody">
          <tr class="empty-row"><td colspan="6"><div class="spinner"></div></td></tr>
        </tbody>
      </table>
      <div class="pagination">
        <span id="page-info">—</span>
        <div class="page-btns" id="page-btns"></div>
      </div>
    </div>
  </main>
</div>

<script>
(function () {
  'use strict';

  const API = '/api/v1';
  let token = null;
  let barChart = null;
  let donutChart = null;
  let currentPeriod = 'all';
  let currentPage = 1;
  const PAGE_SIZE = 15;

  // ─── UTILS ────────────────────────────────────────────────────────────────
  const fmt = (n) => new Intl.NumberFormat('vi-VN').format(n);
  const fmtDate = (s) => {
    const d = new Date(s);
    return d.toLocaleString('en-GB', { day: '2-digit', month: 'short', year: 'numeric', hour: '2-digit', minute: '2-digit' });
  };
  const pct = (a, b) => b ? ((a / b) * 100).toFixed(1) + '%' : '0%';

  async function api(path, opts = {}) {
    const headers = { 'Content-Type': 'application/json' };
    if (token) headers['Authorization'] = 'Bearer ' + token;
    const r = await fetch(API + path, { headers, ...opts });
    const data = await r.json();
    if (!r.ok) throw new Error(data?.error?.message || 'Request failed');
    return data.data ?? data;
  }

  // ─── AUTH ─────────────────────────────────────────────────────────────────
  document.getElementById('login-btn').addEventListener('click', doLogin);
  document.getElementById('login-password').addEventListener('keydown', e => { if (e.key === 'Enter') doLogin(); });

  async function doLogin() {
    const btn = document.getElementById('login-btn');
    const errEl = document.getElementById('login-error');
    errEl.textContent = '';
    btn.disabled = true; btn.textContent = 'Signing in…';
    try {
      const username = document.getElementById('login-username').value.trim();
      const password = document.getElementById('login-password').value;
      if (!username || !password) throw new Error('Please enter username and password.');
      const res = await api('/auth/login', { method: 'POST', body: JSON.stringify({ username, password }) });
      token = res.token;
      localStorage.setItem('spg_token', token);
      localStorage.setItem('spg_username', username);
      showApp(username);
    } catch (e) {
      errEl.textContent = e.message;
    } finally {
      btn.disabled = false; btn.textContent = 'Sign In';
    }
  }

  document.getElementById('logout-btn').addEventListener('click', () => {
    token = null;
    localStorage.removeItem('spg_token');
    localStorage.removeItem('spg_username');
    document.getElementById('app-view').style.display = 'none';
    document.getElementById('login-view').style.display = 'flex';
    if (barChart) { barChart.destroy(); barChart = null; }
    if (donutChart) { donutChart.destroy(); donutChart = null; }
  });

  // ─── STARTUP ──────────────────────────────────────────────────────────────
  const savedToken = localStorage.getItem('spg_token');
  const savedUser  = localStorage.getItem('spg_username');
  if (savedToken) { token = savedToken; showApp(savedUser || 'Merchant'); }

  function showApp(username) {
    document.getElementById('login-view').style.display = 'none';
    const appEl = document.getElementById('app-view');
    appEl.style.display = 'flex';
    document.getElementById('header-merchant').textContent = username;
    loadAll();
  }

  // ─── LOAD EVERYTHING ──────────────────────────────────────────────────────
  function loadAll() {
    loadBalance();
    loadStats();
    loadTransactions(1);
  }

  // Period tabs
  document.querySelectorAll('.period-btn').forEach(btn => {
    btn.addEventListener('click', () => {
      document.querySelectorAll('.period-btn').forEach(b => b.classList.remove('active'));
      btn.classList.add('active');
      currentPeriod = btn.dataset.period;
      loadStats();
    });
  });

  // Filters & refresh
  document.getElementById('filter-status').addEventListener('change', () => loadTransactions(1));
  document.getElementById('filter-type').addEventListener('change', () => loadTransactions(1));
  document.getElementById('refresh-btn').addEventListener('click', loadAll);

  // ─── BALANCE ──────────────────────────────────────────────────────────────
  async function loadBalance() {
    try {
      const d = await api('/wallets/balance');
      document.getElementById('header-balance').textContent = fmt(d.balance) + ' ' + d.currency;
    } catch (e) {
      if (e.message.includes('token') || e.message.includes('401')) signOut();
    }
  }

  // ─── STATS + CHARTS ───────────────────────────────────────────────────────
  async function loadStats() {
    try {
      const d = await api('/dashboard/stats?period=' + currentPeriod);
      document.getElementById('kpi-revenue').textContent   = fmt(d.total_revenue) + ' ₫';
      document.getElementById('kpi-success').textContent   = fmt(d.successful);
      document.getElementById('kpi-rate').textContent      = 'Success rate: ' + pct(d.successful, d.total_transactions);
      document.getElementById('kpi-refunds').textContent   = fmt(d.reversed);
      document.getElementById('kpi-refunded-amt').textContent = 'Refunded: ' + fmt(d.total_refunded) + ' ₫';
      document.getElementById('kpi-total').textContent     = fmt(d.total_transactions);
      document.getElementById('kpi-failed-sub').textContent = fmt(d.failed) + ' failed';

      renderBarChart(d);
      renderDonut(d);
    } catch (e) {
      console.error(e);
    }
  }

  function renderBarChart(d) {
    const labels = ['Payments', 'Refunds', 'Top-ups'];
    const values = [d.total_revenue || 0, d.total_refunded || 0, d.total_topup || 0];
    const ctx = document.getElementById('bar-chart').getContext('2d');

    if (barChart) { barChart.data.datasets[0].data = values; barChart.update(); return; }

    barChart = new Chart(ctx, {
      type: 'bar',
      data: {
        labels,
        datasets: [{
          label: 'Amount (VND)',
          data: values,
          backgroundColor: ['rgba(99,102,241,0.7)', 'rgba(244,63,94,0.7)', 'rgba(16,185,129,0.7)'],
          borderColor:     ['rgba(99,102,241,1)',   'rgba(244,63,94,1)',   'rgba(16,185,129,1)'],
          borderWidth: 1.5,
          borderRadius: 6,
        }]
      },
      options: {
        responsive: true, maintainAspectRatio: false,
        plugins: {
          legend: { display: false },
          tooltip: { callbacks: { label: ctx => ' ' + fmt(ctx.parsed.y) + ' ₫' } }
        },
        scales: {
          x: { grid: { color: 'rgba(255,255,255,0.05)' }, ticks: { color: '#64748b' } },
          y: { grid: { color: 'rgba(255,255,255,0.05)' }, ticks: { color: '#64748b', callback: v => fmt(v) } }
        }
      }
    });
  }

  function renderDonut(d) {
    const labels = ['Successful', 'Failed', 'Reversed'];
    const values = [d.successful || 0, d.failed || 0, d.reversed || 0];
    const colors = ['#10b981', '#f43f5e', '#f59e0b'];
    const ctx = document.getElementById('donut-chart').getContext('2d');

    if (donutChart) {
      donutChart.data.datasets[0].data = values;
      donutChart.update();
    } else {
      donutChart = new Chart(ctx, {
        type: 'doughnut',
        data: { labels, datasets: [{ data: values, backgroundColor: colors, borderWidth: 0, hoverOffset: 6 }] },
        options: {
          responsive: true, cutout: '72%',
          plugins: { legend: { display: false } }
        }
      });
    }

    const legendEl = document.getElementById('donut-legend');
    const total = values.reduce((a, b) => a + b, 0);
    legendEl.innerHTML = labels.map((l, i) => '<div class="legend-item">'
      + '<span class="legend-left"><span class="legend-dot" style="background:' + colors[i] + '"></span>' + l + '</span>'
      + '<span>' + values[i] + ' <span style="color:var(--muted);font-size:.72rem">('+pct(values[i],total)+')</span></span>'
      + '</div>').join('');
  }

  // ─── TRANSACTIONS TABLE ───────────────────────────────────────────────────
  async function loadTransactions(page) {
    currentPage = page;
    const tbody = document.getElementById('tx-tbody');
    tbody.innerHTML = '<tr class="empty-row"><td colspan="6"><div class="spinner"></div></td></tr>';

    const status = document.getElementById('filter-status').value;
    const type   = document.getElementById('filter-type').value;
    let qs = '?page=' + page + '&page_size=' + PAGE_SIZE;
    if (status) qs += '&status=' + status;
    if (type)   qs += '&type='   + type;

    try {
      const d = await api('/transactions' + qs);
      renderTable(d.items || []);
      renderPagination(d.page, d.total_pages, d.total);
    } catch (e) {
      tbody.innerHTML = '<tr class="empty-row"><td colspan="6">Failed to load transactions.</td></tr>';
    }
  }

  function renderTable(items) {
    const tbody = document.getElementById('tx-tbody');
    if (!items.length) {
      tbody.innerHTML = '<tr class="empty-row"><td colspan="6">No transactions found.</td></tr>';
      return;
    }
    tbody.innerHTML = items.map(tx => {
      const isNeg = tx.transaction_type === 'PAYMENT';
      const amtClass = isNeg ? 'amount-neg' : 'amount-pos';
      const amtPrefix = isNeg ? '−' : '+';
      const typePill = {
        'PAYMENT': '<span class="pill pill-payment">PAYMENT</span>',
        'REFUND':  '<span class="pill pill-refund">REFUND</span>',
        'TOPUP':   '<span class="pill pill-topup">TOPUP</span>',
      }[tx.transaction_type] || tx.transaction_type;
      const statusPill = {
        'SUCCESS':  '<span class="pill pill-success">SUCCESS</span>',
        'FAILED':   '<span class="pill pill-fail">FAILED</span>',
        'REVERSED': '<span class="pill pill-reversed">REVERSED</span>',
      }[tx.status] || tx.status;

      return '<tr>'
        + '<td style="max-width:180px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap" title="' + tx.reference_id + '">' + tx.reference_id + '</td>'
        + '<td>' + typePill + '</td>'
        + '<td class="' + amtClass + '">' + amtPrefix + fmt(tx.amount) + ' ₫</td>'
        + '<td>' + statusPill + '</td>'
        + '<td style="white-space:nowrap;color:var(--muted)">' + fmtDate(tx.created_at) + '</td>'
        + '<td class="tx-id" style="max-width:120px;overflow:hidden;text-overflow:ellipsis" title="' + tx.id + '">' + tx.id.slice(0,8) + '…</td>'
        + '</tr>';
    }).join('');
  }

  function renderPagination(page, totalPages, total) {
    document.getElementById('page-info').textContent =
      'Page ' + page + ' of ' + totalPages + ' (' + fmt(total) + ' total)';

    const btnsEl = document.getElementById('page-btns');
    const pages = [];

    // always show first/last, and ±2 around current
    const add = (p) => { if (p > 0 && p <= totalPages && !pages.includes(p)) pages.push(p); };
    add(1); add(page - 2); add(page - 1); add(page); add(page + 1); add(page + 2); add(totalPages);
    pages.sort((a, b) => a - b);

    let html = '';
    let prev = null;
    for (const p of pages) {
      if (prev && p - prev > 1) html += '<button class="page-btn" disabled>…</button>';
      html += '<button class="page-btn ' + (p === page ? 'current' : '') + '" data-page="' + p + '">' + p + '</button>';
      prev = p;
    }
    btnsEl.innerHTML = html;
    btnsEl.querySelectorAll('[data-page]').forEach(btn => {
      btn.addEventListener('click', () => loadTransactions(+btn.dataset.page));
    });
  }

})();
</script>
</body>
</html>
`
