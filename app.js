function fmtDur(secs) {
  if (!secs) return '—';
  if (secs < 60) return Math.round(secs) + 's';
  var m = Math.floor(secs / 60), s = Math.round(secs % 60);
  return s ? m + 'm ' + s + 's' : m + 'm';
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

function rateColorClass(r) { return r === 0 ? 'green' : r <= 30 ? 'orange' : 'red'; }
function rateBarColor(r)   { return r === 0 ? 'var(--green)' : r <= 30 ? 'var(--orange)' : 'var(--red)'; }
function healthColor(h)    { return h >= 80 ? 'var(--green)' : h >= 50 ? 'var(--orange)' : 'var(--red)'; }

function esc(s) {
  return String(s || '')
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;');
}

var allWorkflows = [];
var activeFilter = 'all';
var searchQuery  = '';

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

function applyFilters() {
  var tbody     = document.getElementById('wfBody');
  var wfRows    = tbody.querySelectorAll('tr.wf-row');
  var drawerRows = tbody.querySelectorAll('tr.drawer-row');
  var shown = 0;

  wfRows.forEach(function(tr) {
    var idx  = parseInt(tr.dataset.wfIdx, 10);
    var wf   = allWorkflows[idx];
    var show = matchesFilter(wf) && matchesSearch(wf);

    tr.style.display = show ? '' : 'none';

    if (!show) {
      var drawerTr = drawerRows[idx];
      if (drawerTr) drawerTr.style.display = 'none';
      tr.classList.remove('open');
      var drawerEl = document.getElementById('drawer-' + idx);
      if (drawerEl) drawerEl.classList.remove('open');
    }

    if (show) shown++;
  });

  var emptyRow = document.getElementById('emptyRow');
  if (shown === 0) {
    if (!emptyRow) {
      emptyRow = document.createElement('tr');
      emptyRow.id = 'emptyRow';
      emptyRow.className = 'empty-row';
      emptyRow.innerHTML = '<td colspan="6">No workflows match this filter.</td>';
      tbody.appendChild(emptyRow);
    }
    emptyRow.style.display = '';
  } else if (emptyRow) {
    emptyRow.style.display = 'none';
  }

  document.getElementById('filterCount').textContent =
    shown + ' of ' + allWorkflows.length + ' workflows';
}

document.querySelectorAll('.chip').forEach(function(chip) {
  chip.addEventListener('click', function() {
    document.querySelectorAll('.chip').forEach(function(c) { c.classList.remove('active'); });
    chip.classList.add('active');
    activeFilter = chip.dataset.filter;
    applyFilters();
  });
});

document.getElementById('searchInput').addEventListener('input', function(e) {
  searchQuery = e.target.value.trim();
  applyFilters();
});

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

  runs.forEach(function(run, ri) {
    var dur        = (new Date(run.updated_at) - new Date(run.created_at)) / 1000;
    var logPanelId = 'logpanel-' + wf.name.replace(/\W/g, '') + '-' + ri;
    var hasFailed  = run.conclusion === 'failure' && run.failed_jobs && run.failed_jobs.length > 0;

    html += '<tr>'
      + '<td style="color:var(--muted);font-family:\'JetBrains Mono\',monospace">#' + run.run_number + '</td>'
      + '<td>' + conclusionHtml(run.conclusion) + '</td>'
      + '<td style="color:var(--muted);font-family:\'JetBrains Mono\',monospace;font-size:11px">' + fmtDate(run.created_at) + '</td>'
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
      html += '<tr><td colspan="6" style="padding:0 12px 12px">'
        + '<div class="log-panel" id="' + logPanelId + '">';

      run.failed_jobs.forEach(function(job) {
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

fetch('stats.json')
  .then(function(r) { return r.json(); })
  .then(function(data) {
    var repo    = data.repo || '';
    var repoUrl = 'https://github.com/' + repo;
    document.getElementById('footerLink').href        = repoUrl;
    document.getElementById('footerLink').textContent = repo;
    var generatedAt    = data.generated_at ? new Date(data.generated_at) : null;
    var lastUpdatedEl  = document.getElementById('lastUpdated');
    if (generatedAt && lastUpdatedEl) {
      var diff    = Math.floor((Date.now() - generatedAt) / 60000);
      var timeStr = generatedAt.toLocaleTimeString('en-GB', { hour: '2-digit', minute: '2-digit' });
      var dateStr = generatedAt.toLocaleDateString('en-GB', { day: '2-digit', month: 'short' });
      var agoStr  = diff < 1 ? 'just now' : diff < 60 ? diff + 'm ago' : Math.floor(diff / 60) + 'h ago';
      lastUpdatedEl.textContent = 'updated ' + dateStr + ' ' + timeStr + ' (' + agoStr + ')';
      lastUpdatedEl.title = generatedAt.toISOString();
    }

    // Health bar
    var health = data.overall_health || 0;
    document.getElementById('healthVal').textContent = health.toFixed(1) + '%';
    var fill = document.getElementById('healthBarFill');
    fill.style.width      = health + '%';
    fill.style.background = healthColor(health);

    allWorkflows = data.workflows || [];
    document.getElementById('totalVal').textContent = allWorkflows.length;
    document.getElementById('passVal').textContent  = allWorkflows.filter(function(w) { return w.failure_rate <= 20 && w.total_runs > 0; }).length;
    document.getElementById('failVal').textContent  = allWorkflows.filter(function(w) { return w.failure_rate > 20; }).length;

    var tbody = document.getElementById('wfBody');
    tbody.innerHTML = '';

    allWorkflows.forEach(function(wf, idx) {
      var drawerId = 'drawer-' + idx;

      var tr = document.createElement('tr');
      tr.className       = 'wf-row';
      tr.dataset.wfIdx   = idx;

      var critBadge   = wf.critical ? '<span class="badge badge-critical">critical</span>' : '';
      var lr          = wf.last_run;
      var rate        = wf.failure_rate || 0;
      var rc          = rateColorClass(rate);
      var rateDisplay = (rate % 1 === 0) ? rate : rate.toFixed(1);
      var dotsHtml    = (wf.weather_history || []).map(function(h) {
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

      // ── Drawer row ──
      var drawerTr = document.createElement('tr');
      drawerTr.className     = 'drawer-row';
      drawerTr.style.display = 'none';
      drawerTr.dataset.wfIdx = idx;

      var drawerTd  = document.createElement('td');
      drawerTd.colSpan = 6;
      var drawerDiv = document.createElement('div');
      drawerDiv.className = 'drawer';
      drawerDiv.id        = drawerId;
      drawerTd.appendChild(drawerDiv);
      drawerTr.appendChild(drawerTd);
      tbody.appendChild(drawerTr);

      tr.addEventListener('click', function() {
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

    document.getElementById('filterCount').textContent =
      allWorkflows.length + ' of ' + allWorkflows.length + ' workflows';
  })
  .catch(function(e) {
    document.getElementById('wfBody').innerHTML =
      '<tr class="loading-row"><td colspan="6">Error loading stats.json: ' + e + '</td></tr>';
  });