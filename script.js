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
    if (secs < 60) return `${Math.round(secs)}s`
    if (secs < 3600) return `${Math.floor(secs/60)}m ${Math.round(secs%60)}s`
    return `${Math.floor(secs/3600)}h ${Math.floor((secs%3600)/60)}m`
  }
  
  function timeAgo(dateStr) {
    if (!dateStr) return "—"
    const diff = Math.floor((Date.now() - new Date(dateStr)) / 1000)
    if (diff < 60) return `${diff}s ago`
    if (diff < 3600) return `${Math.floor(diff/60)}m ago`
    if (diff < 86400) return `${Math.floor(diff/3600)}h ago`
    return `${Math.floor(diff/86400)}d ago`
  }
  
  function render(data) {
    // header
    document.getElementById("header-meta").textContent =
      `${data.repo} · updated ${timeAgo(data.generated_at)}`
  
    // health score
    const score = Math.round(data.overall_health)
    const scoreEl = document.getElementById("health-score")
    scoreEl.textContent = `${score}%`
    scoreEl.style.color = healthColor(score)
  
    // stats
    document.getElementById("stat-workflows").textContent = data.workflows.length
    document.getElementById("stat-required").textContent = data.required_tests.length
  
    const failing = data.workflows.filter(w => w.failure_rate > 0).length
    document.getElementById("stat-failing").textContent = failing

    const tbody = document.getElementById("workflow-tbody")
    tbody.innerHTML = ""
  
    data.workflows.forEach(wf => {
      const tr = document.createElement("tr")
  
      // history dots
      const historyDots = wf.weather_history.map(h => {
        let color = "#999"
        if (h === "success") color = "green"
        else if (h === "failure") color = "red"
        else if (h === "action_required") color = "orange"
  
        return `<span style="
          display:inline-block;
          width:8px;
          height:8px;
          border-radius:50%;
          background:${color};
          margin-right:2px;
        "></span>`
      }).join("")
  
      tr.innerHTML = `
        <td>${wf.name}</td>
        <td>${timeAgo(wf.last_run?.created_at)}</td>
        <td class="${rateClass(wf.failure_rate)}">
          ${wf.failure_rate.toFixed(1)}%
        </td>
        <td>${historyDots}</td>
        <td>${formatDuration(wf.avg_duration_secs)}</td>
        <td>
          ${wf.last_run ? `<a href="${wf.last_run.html_url}" target="_blank">View</a>` : "—"}
        </td>
      `
  
      tbody.appendChild(tr)
    })
  }
  async function loadData() {
    try {
      const resp = await fetch(`stats.json?t=${Date.now()}`)
      if (!resp.ok) throw new Error(`HTTP ${resp.status}`)
      const data = await resp.json()
      render(data)
    } catch (err) {
      document.getElementById("error-box").style.display = "block"
      document.getElementById("error-box").textContent =
        `Failed to load stats.json: ${err.message}`
    }
  }
  
  loadData()
  setInterval(loadData, 1 * 60 * 60 * 1000)
