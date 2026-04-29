(function () {
  var saved = localStorage.getItem('ci-theme') || 'dark';
  if (saved === 'light') document.body.classList.add('light');

  var btn      = document.getElementById('themeToggle');
  var iconSun  = document.getElementById('iconSun');
  var iconMoon = document.getElementById('iconMoon');

  function syncIcons() {
    var isLight = document.body.classList.contains('light');
    iconSun.style.display  = isLight ? 'none' : '';
    iconMoon.style.display = isLight ? ''     : 'none';
  }
  syncIcons();

  if (btn) {
    btn.addEventListener('click', function () {
      document.body.classList.toggle('light');
      var theme = document.body.classList.contains('light') ? 'light' : 'dark';
      localStorage.setItem('ci-theme', theme);
      syncIcons();
    });
  }
})();

// Helpers
function fmtDur(secs) {
  if (!secs) return '—';
  if (secs < 60) return Math.round(secs) + 's';
  var h = Math.floor(secs / 3600);
  var m = Math.floor((secs % 3600) / 60);
  var s = Math.round(secs % 60);
  if (h > 0) return m > 0 ? h + 'h ' + m + 'm' : h + 'h';
  return s > 0 ? m + 'm ' + s + 's' : m + 'm';
}

function fmtDate(iso) {
  if (!iso) return '';
  var d = new Date(iso);
  return d.toLocaleDateString('en-US', { day: '2-digit', month: 'short', year: 'numeric' })
    + ' ' + d.toLocaleTimeString('en-US', { hour: '2-digit', minute: '2-digit' });
}

function conclusionHtml(c) {
  if (!c) return '<span class="conclusion unknown">—</span>';
  var cls = ['success', 'failure', 'skipped', 'action_required'].includes(c) ? c : 'unknown';
  return '<span class="conclusion ' + cls + '">' + c.replace(/_/g, ' ') + '</span>';
}

function rateColorClass(r) { return r >= 80 ? 'green' : r >= 50 ? 'orange' : 'red'; }
function rateBarColor(r)   { return r >= 80 ? 'var(--green)' : r >= 50 ? 'var(--orange)' : 'var(--red)'; }
function healthColor(h)    { return h >= 80 ? 'var(--green)' : h >= 50 ? 'var(--orange)' : 'var(--red)'; }

function esc(s) {
  return String(s || '')
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;');
}

function cssVar(name) {
  return getComputedStyle(document.documentElement).getPropertyValue(name).trim();
}

function hexToRgba(hex, alpha) {
  hex = hex.replace('#', '');
  if (hex.length === 3) hex = hex.split('').map(function (c) { return c + c; }).join('');
  var r = parseInt(hex.slice(0, 2), 16);
  var g = parseInt(hex.slice(2, 4), 16);
  var b = parseInt(hex.slice(4, 6), 16);
  return 'rgba(' + r + ',' + g + ',' + b + ',' + alpha + ')';
}

function getChartPalette(count) {
  var root = getComputedStyle(document.body);
  var cssColors = [
    root.getPropertyValue('--chart-1').trim(),
    root.getPropertyValue('--chart-2').trim(),
    root.getPropertyValue('--chart-3').trim(),
    root.getPropertyValue('--chart-4').trim(),
    root.getPropertyValue('--chart-5').trim(),
    root.getPropertyValue('--chart-6').trim(),
    root.getPropertyValue('--chart-7').trim(),
    root.getPropertyValue('--chart-8').trim()
  ].filter(Boolean);

  var colors = cssColors.slice();
  while (colors.length < count) {
    var hue = Math.round((colors.length * 137.5) % 360);
    colors.push('hsl(' + hue + ', 62%, 56%)');
  }
  return colors.slice(0, count);
}

var allWorkflows = [];
var activeFilter = 'all';
var searchQuery  = '';
var chartsBuilt  = false;

// Charts
function buildCharts(workflows) {
  if (chartsBuilt) return;
  chartsBuilt = true;

  var active = workflows.filter(function (w) { return w.total_runs > 0; });

  var labels       = active.map(function (w) { return w.name; });
  var failRates    = active.map(function (w) { return parseFloat(w.failure_rate.toFixed(1)); });
  var successRates = active.map(function (w) { return parseFloat((100 - w.failure_rate).toFixed(1)); });
  var totalRuns    = active.map(function (w) { return w.total_runs; });
  var failedRuns   = active.map(function (w) { return w.failed_runs; });

  var palette = getChartPalette(labels.length);

  function gridColor()  { return cssVar('--border') || '#30363d'; }
  function mutedColor() { return cssVar('--muted')  || '#8b949e'; }

  Chart.defaults.global.defaultFontFamily = "'JetBrains Mono', monospace";
  Chart.defaults.global.defaultFontColor  = mutedColor();
  Chart.defaults.global.defaultFontSize   = 11;

  new Chart(document.getElementById('chartPie'), {
    type: 'pie',
    data: {
      labels: labels,
      datasets: [{ data: failRates, backgroundColor: palette, borderWidth: 0 }]
    },
    options: {
      responsive: true,
      maintainAspectRatio: true,
      legend: { position: 'right', labels: { boxWidth: 12, fontSize: 11, fontColor: mutedColor() } },
      tooltips: {
        callbacks: {
          label: function (item, data) {
            return ' ' + data.labels[item.index] + ': ' + data.datasets[0].data[item.index] + '%';
          }
        }
      }
    }
  });

  new Chart(document.getElementById('chartBar'), {
    type: 'bar',
    data: {
      labels: labels,
      datasets: [
        { label: 'Total Runs',  data: totalRuns,  backgroundColor: hexToRgba(cssVar('--blue'), 0.75) },
        { label: 'Failed Runs', data: failedRuns, backgroundColor: hexToRgba(cssVar('--red'),  0.75) }
      ]
    },
    options: {
      responsive: true,
      maintainAspectRatio: true,
      legend: { labels: { fontSize: 11, boxWidth: 12, fontColor: mutedColor() } },
      scales: {
        xAxes: [{ gridLines: { color: gridColor(), zeroLineColor: gridColor() }, ticks: { fontColor: mutedColor(), fontSize: 11 } }],
        yAxes: [{ gridLines: { color: gridColor(), zeroLineColor: gridColor() }, ticks: { beginAtZero: true, fontColor: mutedColor(), fontSize: 11 } }]
      }
    }
  });

  var durationLabels = [];
  var durationValues = [];
  active.forEach(function (w) {
    if (w.avg_duration_secs > 0) {
      durationLabels.push(w.name);
      durationValues.push(parseFloat((w.avg_duration_secs / 60).toFixed(2)));
    }
  });
  var durationColors = getChartPalette(durationLabels.length);

  new Chart(document.getElementById('chartDuration'), {
    type: 'doughnut',
    data: {
      labels: durationLabels,
      datasets: [{ data: durationValues, backgroundColor: durationColors, borderWidth: 0 }]
    },
    options: {
      responsive: true,
      maintainAspectRatio: true,
      legend: { position: 'right', labels: { boxWidth: 12, fontSize: 11, fontColor: mutedColor() } },
      tooltips: {
        callbacks: {
          label: function (item, data) {
            return ' ' + data.labels[item.index] + ': ' + data.datasets[0].data[item.index] + ' min';
          }
        }
      }
    }
  });

  new Chart(document.getElementById('chartSuccess'), {
    type: 'bar',
    data: {
      labels: labels,
      datasets: [{
        label: 'Success Rate %',
        data: successRates,
        backgroundColor: successRates.map(function (r) {
          return r >= 80
            ? hexToRgba(cssVar('--green'),  0.75)
            : r >= 50
              ? hexToRgba(cssVar('--orange'), 0.75)
              : hexToRgba(cssVar('--red'),    0.75);
        })
      }]
    },
    options: {
      responsive: true,
      maintainAspectRatio: true,
      legend: { display: false },
      scales: {
        xAxes: [{ gridLines: { color: gridColor(), zeroLineColor: gridColor() }, ticks: { fontColor: mutedColor(), fontSize: 11 } }],
        yAxes: [{
          gridLines: { color: gridColor(), zeroLineColor: gridColor() },
          ticks: { beginAtZero: true, max: 100, fontColor: mutedColor(), fontSize: 11, callback: function (v) { return v + '%'; } }
        }]
      },
      tooltips: { callbacks: { label: function (item) { return ' ' + item.yLabel + '%'; } } }
    }
  });
}

// Filter helpers
function hasRecentRun(wf) {
  if (!wf.last_run) return false;
  var age = Date.now() - new Date(wf.last_run.created_at).getTime();
  return age < 7 * 24 * 60 * 60 * 1000;
}

function isRecentFailure(wf) {
  if (!wf.last_run) return false;
  if (wf.last_run.conclusion !== 'failure') return false;
  var age = Date.now() - new Date(wf.last_run.created_at).getTime();
  return age < 7 * 24 * 60 * 60 * 1000;
}

function matchesFilter(wf) {
  switch (activeFilter) {
    case 'failing':        return wf.failure_rate > 20;
    case 'passing':        return wf.failure_rate <= 20 && wf.total_runs > 0;
    case 'critical':       return wf.critical === true;
    case 'recent-run':     return hasRecentRun(wf);
    case 'recent-failure': return isRecentFailure(wf);
    case 'no-runs':        return wf.total_runs === 0;
    default:               return true;
  }
}

function matchesSearch(wf) {
  if (!searchQuery) return true;
  var q = searchQuery.toLowerCase();
  return wf.name.toLowerCase().includes(q)
      || (wf.description || '').toLowerCase().includes(q);
}

// Sort helper
function sortList(list) {
  switch (activeFilter) {
    case 'recent-run':
      return list.slice().sort(function (a, b) {
        return new Date(b.last_run.created_at) - new Date(a.last_run.created_at);
      });
    case 'recent-failure':
      return list.slice().sort(function (a, b) {
        return new Date(b.last_run.created_at) - new Date(a.last_run.created_at);
      });
    case 'failing':
      return list.slice().sort(function (a, b) {
        return b.failure_rate - a.failure_rate;
      });
    case 'passing':
      return list.slice().sort(function (a, b) {
        return a.failure_rate - b.failure_rate;
      });
    default:
      return list.slice().sort(function (a, b) {
        return a.name.localeCompare(b.name);
      });
  }
}

// Apply filters + re render
function applyFilters() {
  var filtered = allWorkflows.filter(function (wf) {
    return matchesFilter(wf) && matchesSearch(wf);
  });

  var sorted = sortList(filtered);

  renderTable(sorted);

  document.getElementById('filterCount').textContent =
    sorted.length + ' of ' + allWorkflows.length + ' workflows';
}

// Table renderer
function renderTable(list) {
  var tbody = document.getElementById('wfBody');
  tbody.innerHTML = '';
  if (list.length === 0) {
    tbody.innerHTML = '<tr class="empty-row"><td colspan="6">No workflows match this filter.</td></tr>';
    return;
  }

  list.forEach(function (wf, idx) {
    var safeId   = wf.name.replace(/\W+/g, '-');
    var drawerId = 'drawer-' + safeId;

    var tr = document.createElement('tr');
    tr.className = 'wf-row';
    tr.style.animationDelay = (idx * 30) + 'ms';

    var critBadge = wf.critical ? '<span class="badge badge-critical">critical</span>' : '';
    var lr        = wf.last_run;
    var rate      = wf.failure_rate != null ? (100 - wf.failure_rate) : 0;
    var rc        = rateColorClass(rate);
    var rateDisplay = (rate % 1 === 0) ? rate : rate.toFixed(1);

    var dotsHtml = (wf.weather_history || []).map(function (h) {
      return '<div class="dot ' + h + '" title="' + h + '"></div>';
    }).join('');

    tr.innerHTML =
      '<td>'
        + '<div class="wf-name-wrap">'
        + '<svg class="chevron" viewBox="0 0 16 16" fill="currentColor"><path d="M6.22 3.22a.75.75 0 0 1 1.06 0l4.25 4.25a.75.75 0 0 1 0 1.06l-4.25 4.25a.749.749 0 0 1-1.06-1.06L10 8 6.22 4.22a.75.75 0 0 1 0-1Z"/></svg>'
        + '<span class="wf-name">' + esc(wf.name) + critBadge + '</span>'
        + '</div>'
        + '<div class="wf-desc">' + esc(wf.description || '') + '</div>'
      + '</td>'
      + '<td>'
        + (lr
            ? conclusionHtml(lr.conclusion) + '<div class="run-date">#' + lr.run_number + ' · ' + fmtDate(lr.created_at) + '</div>'
            : '<span class="conclusion unknown">no runs</span>')
      + '</td>'
      + '<td><div class="rate-wrap">'
        + '<div class="rate-bar-track"><div class="rate-bar-fill" style="width:' + Math.min(rate, 100) + '%;background:' + rateBarColor(rate) + '"></div></div>'
        + '<span class="rate-num ' + rc + '">' + rateDisplay + '%</span>'
      + '</div></td>'
      + '<td><div class="history-dots">' + (dotsHtml || '<span class="no-link">—</span>') + '</div></td>'
      + '<td><span class="dur">' + fmtDur(wf.avg_duration_secs) + '</span></td>'
      + '<td>'
        + (lr
            ? '<a class="run-link" href="' + esc(lr.html_url) + '" target="_blank" onclick="event.stopPropagation()">↗ View</a>'
            : '<span class="no-link">—</span>')
      + '</td>';

    tbody.appendChild(tr);

    // Drawer row
    var drawerTr = document.createElement('tr');
    drawerTr.className     = 'drawer-row';
    drawerTr.style.display = 'none';

    var drawerTd  = document.createElement('td');
    drawerTd.colSpan = 6;
    var drawerDiv = document.createElement('div');
    drawerDiv.className = 'drawer';
    drawerDiv.id        = drawerId;
    drawerTd.appendChild(drawerDiv);
    drawerTr.appendChild(drawerTd);
    tbody.appendChild(drawerTr);

    tr.addEventListener('click', function () {
      var open     = tr.classList.toggle('open');
      var drawerEl = document.getElementById(drawerId);
      drawerTr.style.display = open ? '' : 'none';
      if (open) {
        drawerEl.classList.add('open');
        if (!drawerEl.dataset.rendered) {
          drawerEl.dataset.rendered = '1';
          renderDrawer(wf, drawerEl);
        }
      } else {
        drawerEl.classList.remove('open');
      }
    });
  });
}

// Chip & search listeners
document.querySelectorAll('.chip').forEach(function (chip) {
  chip.addEventListener('click', function () {
    document.querySelectorAll('.chip').forEach(function (c) { c.classList.remove('active'); });
    chip.classList.add('active');
    activeFilter = chip.dataset.filter;

    var chartsPanel = document.getElementById('chartsPanel');
    var tableWrap   = document.getElementById('tableWrap');

    if (activeFilter === 'chart') {
      chartsPanel.classList.add('open');
      tableWrap.style.display = 'none';
      if (allWorkflows.length > 0) buildCharts(allWorkflows);
      document.querySelectorAll('.chart-card').forEach(function (card, i) {
        card.style.animation = 'none';
        card.offsetHeight;
        card.style.animation = '';
        card.style.animationDelay = (i * 80) + 'ms';
      });
    } else {
      chartsPanel.classList.remove('open');
      tableWrap.style.display = '';
      applyFilters();
    }
  });
});

document.querySelectorAll('.clickable').forEach(function (card) {
  card.addEventListener('click', function () {
    var filter = card.dataset.filter;
    if (!filter) return;

    document.querySelectorAll('.chip').forEach(function (c) { c.classList.remove('active'); });
    var target = document.querySelector('.chip[data-filter="' + filter + '"]');
    if (target) target.classList.add('active');

    document.getElementById('chartsPanel').classList.remove('open');
    document.getElementById('tableWrap').style.display = '';

    activeFilter = filter;
    applyFilters();

    document.querySelector('.filter-bar').scrollIntoView({ behavior: 'smooth', block: 'start' });
  });
});


document.getElementById('searchInput').addEventListener('input', function (e) {
  searchQuery = e.target.value.trim();
  applyFilters();
});

// Drawer renderer
function renderDrawer(wf, drawerEl) {
  var runs = wf.recent_runs || [];
  if (runs.length === 0) {
    drawerEl.innerHTML = '<p class="no-runs-msg">No runs stored yet.</p>';
    return;
  }

  var html = '<div class="runs-title">Past Runs (' + runs.length + ')</div>';
  html += '<table class="runs-table"><thead><tr>'
    + '<th>#</th><th>Conclusion</th><th>Started</th>'
    + '<th>Duration</th><th>Link</th><th>Failure Logs</th>'
    + '</tr></thead><tbody>';

  runs.forEach(function (run, ri) {
    var dur        = (new Date(run.updated_at) - new Date(run.created_at)) / 1000;
    var safeWfName = wf.name.replace(/\W+/g, '-');
    var logPanelId = 'logpanel-' + safeWfName + '-' + ri;
    var hasFailed  = run.conclusion === 'failure' && run.failed_jobs && run.failed_jobs.length > 0;

    html += '<tr>'
      + '<td style="color:var(--muted);font-family:\'JetBrains Mono\',monospace">#' + run.run_number + '</td>'
      + '<td>' + conclusionHtml(run.conclusion) + '</td>'
      + '<td style="color:var(--muted);font-family:\'JetBrains Mono\',monospace;font-size:12px">' + fmtDate(run.created_at) + '</td>'
      + '<td><span class="dur">' + fmtDur(dur) + '</span></td>'
      + '<td><a class="run-link" href="' + esc(run.html_url) + '" target="_blank">↗ View</a></td>'
      + '<td>';

    if (hasFailed) {
      html += '<button class="log-toggle" onclick="toggleLog(\'' + logPanelId + '\',this)">▶ Show logs</button>';
    } else {
      html += '<span class="no-link">—</span>';
    }
    html += '</td></tr>';

    if (hasFailed) {
      html += '<tr><td colspan="6" style="padding:0 12px 0">'
        + '<div class="log-panel" id="' + logPanelId + '">';

      run.failed_jobs.forEach(function (job) {
        html += '<div class="log-job-block">'
          + '<div class="log-job-header">'
          + '<span class="log-job-name">✗ ' + esc(job.name) + '</span>'
          + '<a class="log-job-link" href="' + esc(job.html_url) + '" target="_blank">Open in GitHub ↗</a>'
          + '</div>'
          + '<div class="log-body">'
          + (job.log_snippet
              ? esc(job.log_snippet)
              : '<span style="color:var(--muted)">No log output captured.</span>')
          + '</div></div>';
      });

      html += '</div></td></tr>';
    }
  });

  html += '</tbody></table>';
  drawerEl.innerHTML = html;
}

function toggleLog(panelId, btn) {
  var panel = document.getElementById(panelId);
  if (!panel) return;
  var open = panel.classList.toggle('open');
  btn.textContent = open ? '▼ Hide logs' : '▶ Show logs';
}

// Data fetch
fetch('stats.json')
  .then(function (r) { return r.json(); })
  .then(function (data) {
    var repo    = data.repo || '';
    var repoUrl = 'https://github.com/' + repo;
    document.getElementById('footerLink').href        = repoUrl;
    document.getElementById('footerLink').textContent = repo;

    var generatedAt   = data.generated_at ? new Date(data.generated_at) : null;
    var lastUpdatedEl = document.getElementById('lastUpdated');
    if (generatedAt && lastUpdatedEl) {
      var diff    = Math.floor((Date.now() - generatedAt) / 60000);
      var timeStr = generatedAt.toLocaleTimeString('en-GB', { hour: '2-digit', minute: '2-digit' });
      var dateStr = generatedAt.toLocaleDateString('en-GB', { day: '2-digit', month: 'short' });
      var agoStr  = diff < 1 ? 'just now' : diff < 60 ? diff + 'm ago' : Math.floor(diff / 60) + 'h ago';
      lastUpdatedEl.textContent = 'updated ' + dateStr + ' ' + timeStr + ' (' + agoStr + ')';
      lastUpdatedEl.title = generatedAt.toISOString();
    }

    var health = data.overall_health || 0;
    document.getElementById('healthVal').textContent = health.toFixed(1) + '%';
    var fill = document.getElementById('healthBarFill');
    fill.style.width      = health + '%';
    fill.style.background = healthColor(health);

    allWorkflows = data.workflows || [];

    document.getElementById('totalVal').textContent = allWorkflows.length;
    document.getElementById('passVal').textContent  = allWorkflows.filter(function (w) { return w.failure_rate <= 20 && w.total_runs > 0; }).length;
    document.getElementById('failVal').textContent  = allWorkflows.filter(function (w) { return w.failure_rate > 20; }).length;

    // Last 24h stats
    var WINDOW_MS   = 24 * 60 * 60 * 1000;
    var windowStart = generatedAt
      ? new Date(generatedAt.getTime() - WINDOW_MS)
      : new Date(Date.now() - WINDOW_MS);

    var todayRuns = 0, todayPass = 0, todayFail = 0;

    allWorkflows.forEach(function (wf) {
      (wf.recent_runs || []).forEach(function (run) {
        var t = new Date(run.created_at);
        if (t < windowStart) return;
        todayRuns++;
        switch (run.conclusion) {
          case 'success':                                       todayPass++; break;
          case 'failure': case 'timed_out': case 'action_required': todayFail++; break;
        }
      });
    });

    var healthToday = (todayPass + todayFail) > 0
      ? (todayPass / (todayPass + todayFail)) * 100
      : 0;
    document.getElementById('runsTodayVal').textContent   = todayRuns;
    document.getElementById('passTodayVal').textContent   = todayPass;
    document.getElementById('failTodayVal').textContent   = todayFail;
    document.getElementById('healthTodayVal').textContent = healthToday.toFixed(1) + '%';
    var todayFill = document.getElementById('healthTodayBarFill');
    todayFill.style.width      = healthToday + '%';
    todayFill.style.background = healthColor(healthToday);
    if (activeFilter === 'chart') {
      buildCharts(allWorkflows);
    } else {
      applyFilters();
    }
  })
  .catch(function (e) {
    document.getElementById('wfBody').innerHTML =
      '<tr class="loading-row"><td colspan="6">Error loading stats.json: ' + e + '</td></tr>';
  });