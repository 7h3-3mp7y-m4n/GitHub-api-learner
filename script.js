function healthColor(score) {
    if (score >= 80) return "var(--green)"
    if (score >= 50) return "var(--yellow)"
    return "var(--red)"
  }
  
  function rateClass(rate) {
    if (rate <= 10) return "rate-good"
    if (rate <= 25) return "rate-warning"
    return "rate-bad"
  }
  
  function formatDuration(secs) {
    if (!secs) return "—"
    if (secs < 60)   return `${Math.round(secs)}s`
    if (secs < 3600) return `${Math.floor(secs/60)}m ${Math.round(secs%60)}s`
    return `${Math.floor(secs/3600)}h ${Math.floor((secs%3600)/60)}m`
  }
  
  function timeAgo(dateStr) {
    if (!dateStr) return "—"
    const diff = Math.floor((Date.now() - new Date(dateStr)) / 1000)
    if (diff < 60)    return `${diff}s ago`
    if (diff < 3600)  return `${Math.floor(diff/60)}m ago`
    if (diff < 86400) return `${Math.floor(diff/3600)}h ago`
    return `${Math.floor(diff/86400)}d ago`
  }
  
  function weatherDot(day) {
    const status = typeof day === "string" ? day : day.status
    const date   = typeof day === "object" ? day.date : ""
    const flaky  = typeof day === "object" ? day.is_flaky : false
  
    let color = "var(--muted)"
    let label = date || ""
  
    if (flaky) {
      color = "var(--yellow)"
      label += ": flaky (passed on retry)"
    } else if (status === "success") {
      color = "var(--green)"
      label += ": passed"
    } else if (status === "failure") {
      color = "var(--red)"
      label += ": failed"
    } else {
      label += ": no data"
    }
  
    return `<span title="${label}" style="
      display: inline-block;
      width: 12px;
      height: 12px;
      border-radius: 3px;
      background: ${color};
      margin-right: 3px;
      cursor: default;
    "></span>`
  }
  
  function render(data) {
    try {
      document.getElementById("header-meta").textContent =
        `${data.repo} · updated ${timeAgo(data.generated_at)}`
  
      // health score
      const score = Math.round(data.overall_health || 0)
      const scoreEl = document.getElementById("health-score")
      scoreEl.textContent = `${score}%`
      scoreEl.style.color = healthColor(score)
      let workflows = []
  
      if (Array.isArray(data.workflows) && data.workflows.length > 0) {
        workflows = data.workflows
      } else if (Array.isArray(data.sections) && data.sections.length > 0) {
        workflows = data.sections.flatMap(s => s.workflows || [])
      }

      document.getElementById("stat-workflows").textContent = workflows.length
  
      const requiredCount = (data.required_tests || []).length
      document.getElementById("stat-required").textContent = requiredCount
  
      const failing = workflows.filter(
        w => w.last_run?.conclusion === "failure"
      ).length
      const failingEl = document.getElementById("stat-failing")
      failingEl.textContent = failing
      if (failing > 0) failingEl.style.color = "var(--red)"
  
      const tbody = document.getElementById("workflow-tbody")
  
      if (workflows.length === 0) {
        tbody.innerHTML = `
          <tr>
            <td colspan="6" class="loading">
              No workflow data found. Check config.yaml workflow names.
            </td>
          </tr>`
        return
      }
  
      tbody.innerHTML = workflows.map(wf => {
        const dots = (wf.weather_history || []).map(weatherDot).join("")
        const conc = wf.last_run?.conclusion || "unknown"
        const dotClass = conc === "success" ? "dot-success"
                       : conc === "failure" ? "dot-failure"
                       : "dot-unknown"
        const runURL = wf.last_run?.url || wf.last_run?.html_url || wf.last_run?.HTMLURL
        const flakyBadge = wf.is_flaky
          ? `<span style="
              font-size:11px;
              background:rgba(210,153,34,.15);
              color:var(--yellow);
              border:1px solid rgba(210,153,34,.3);
              border-radius:10px;
              padding:1px 6px;
              margin-left:6px;
            ">flaky</span>`
          : ""
  
        return `
          <tr>
            <td>
              <div style="font-weight:600">${wf.name}${flakyBadge}</div>
              <div style="font-size:11px;color:var(--muted)">${wf.description || ""}</div>
            </td>
            <td>
              <span class="status-dot ${dotClass}"></span>
              <span style="font-size:12px;color:var(--muted)">
                ${timeAgo(wf.last_run?.created_at)}
              </span>
            </td>
            <td class="${rateClass(wf.failure_rate || 0)}" style="font-weight:600;font-family:var(--mono)">
              ${(wf.failure_rate || 0).toFixed(1)}%
              <div style="font-size:11px;color:var(--muted);font-weight:normal">
                ${wf.failed_runs || 0}/${wf.total_runs || 0}
              </div>
            </td>
            <td>${dots}</td>
            <td style="font-family:var(--mono);font-size:12px;color:var(--muted)">
              ${formatDuration(wf.avg_duration_secs)}
            </td>
            <td>
              ${runURL
                ? `<a href="${runURL}" target="_blank" style="color:var(--blue);font-size:12px">
                    #${wf.last_run?.run_number || "view"}
                  </a>`
                : "—"}
            </td>
          </tr>`
      }).join("")
  
    } catch (err) {
      console.error("render error:", err)
      document.getElementById("error-box").style.display = "block"
      document.getElementById("error-box").textContent =
        `Render error: ${err.message} — check console for details`
    }
  }
  
  async function loadData() {
    try {
      const resp = await fetch(`stats.json?t=${Date.now()}`)
      if (!resp.ok) throw new Error(`HTTP ${resp.status}`)
      const data = await resp.json()
      console.log("Loaded data:", data)
      render(data)
    } catch (err) {
      console.error("load error:", err)
      document.getElementById("error-box").style.display = "block"
      document.getElementById("error-box").textContent =
        `Failed to load stats.json: ${err.message}`
      document.getElementById("header-meta").textContent = "error loading data"
    }
  }
  
  loadData()
  setInterval(loadData, 1 * 60 * 60 * 1000)