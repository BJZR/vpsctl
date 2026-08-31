function DashboardPage(container) {
  let refreshTimer = null;

  container.innerHTML = `
    <div class="page-header">
      <h2>Dashboard</h2>
      <div class="actions">
        <span class="text-sm text-muted" id="dash-uptime"></span>
      </div>
    </div>
    <div class="page-content">
      <div class="dashboard-grid" id="dash-grid">
        <div class="metric-card" id="card-hostname"><div class="spinner-center"><div class="spinner"></div></div></div>
        <div class="metric-card" id="card-cpu"><div class="spinner-center"><div class="spinner"></div></div></div>
        <div class="metric-card" id="card-memory"><div class="spinner-center"><div class="spinner"></div></div></div>
        <div class="metric-card full-width" id="card-disk"><div class="spinner-center"><div class="spinner"></div></div></div>
        <div class="metric-card" id="card-network"><div class="spinner-center"><div class="spinner"></div></div></div>
        <div class="metric-card" id="card-load"><div class="spinner-center"><div class="spinner"></div></div></div>
      </div>
      <div class="card mt-16" id="card-processes">
        <div class="card-header">
          <span>Top Processes</span>
          <span class="text-sm text-muted" id="proc-sort-label"></span>
        </div>
        <div class="card-body-np">
          <div class="table-wrapper">
            <table>
              <thead>
                <tr>
                  <th class="sortable" data-sort="pid">PID <span class="sort-arrow"></span></th>
                  <th class="sortable" data-sort="name">Name <span class="sort-arrow"></span></th>
                  <th class="sortable" data-sort="cpu">CPU% <span class="sort-arrow"></span></th>
                  <th class="sortable" data-sort="mem">MEM% <span class="sort-arrow"></span></th>
                  <th>Status</th>
                </tr>
              </thead>
              <tbody id="process-tbody">
                <tr><td colspan="5" class="text-center text-muted" style="padding:20px">Loading...</td></tr>
              </tbody>
            </table>
          </div>
        </div>
      </div>
    </div>
  `;

  let sortField = 'cpu';
  let sortDir = -1;
  let allProcesses = [];

  container.querySelectorAll('thead th.sortable').forEach(th => {
    th.addEventListener('click', () => {
      const field = th.dataset.sort;
      if (sortField === field) sortDir *= -1;
      else { sortField = field; sortDir = -1; }
      container.querySelectorAll('thead th.sortable').forEach(h => h.classList.remove('sorted'));
      th.classList.add('sorted');
      renderProcesses();
    });
  });

  function gaugeColor(pct) {
    if (pct >= 90) return '#f85149';
    if (pct >= 70) return '#d29922';
    return '#3fb950';
  }

  function renderGauge(pct, size = 80) {
    const r = (size - 12) / 2;
    const c = 2 * Math.PI * r;
    const offset = c * (1 - pct / 100);
    const color = gaugeColor(pct);
    return `
      <div class="gauge-circle" style="width:${size}px;height:${size}px">
        <svg width="${size}" height="${size}">
          <circle class="gauge-bg" cx="${size/2}" cy="${size/2}" r="${r}"/>
          <circle class="gauge-fill" cx="${size/2}" cy="${size/2}" r="${r}" stroke="${color}" stroke-dasharray="${c}" stroke-dashoffset="${offset}"/>
        </svg>
        <div class="gauge-text">${Math.round(pct)}%</div>
      </div>`;
  }

  function renderBar(pct) {
    const color = App.getPercentColor(pct);
    return `
      <div class="progress-bar">
        <div class="progress-bar-fill ${color}" style="width:${Math.min(pct, 100)}%"></div>
      </div>
      <div class="progress-label">
        <span>${pct.toFixed(1)}%</span>
      </div>`;
  }

  async function fetchMetrics() {
    try {
      const data = await App.api.get('/system/stats');
      renderSystemInfo(data);
      renderCPU(data.cpu);
      renderMemory(data.memory);
      renderDisk(data.disks);
      renderNetwork(data.network);
      renderLoad(data.load);
      renderProcesses(data.processes || []);
    } catch (err) {
      // Keep current state, don't crash
    }
  }

  function renderSystemInfo(data) {
    const info = data.system || {};
    const hostname = info.hostname || '-';
    const os = (info.os || '') + ' ' + (info.arch || '');
    const kernel = info.kernel || '-';
    const uptime = info.uptime || 0;

    document.getElementById('card-hostname').innerHTML = `
      <div class="metric-label">System</div>
      <div class="metric-value" style="font-size:18px">${App.escapeHtml(hostname)}</div>
      <div class="metric-sub">${App.escapeHtml(os.trim())}</div>
      <div class="metric-sub">Kernel: ${App.escapeHtml(kernel)}</div>
      <div class="metric-sub">Uptime: ${App.formatDuration(uptime)}</div>
    `;
    document.getElementById('dash-uptime').textContent = 'Uptime: ' + App.formatDuration(uptime);
  }

  function renderCPU(cpu) {
    if (!cpu) return;
    const pct = cpu.percent || 0;
    const cores = cpu.cores || 0;
    const model = cpu.model || '';
    document.getElementById('card-cpu').innerHTML = `
      <div class="metric-label">CPU</div>
      <div class="gauge-container">
        ${renderGauge(pct)}
        <div>
          <div style="font-size:13px;color:var(--text-secondary)">${cores} cores</div>
          <div style="font-size:12px;color:var(--text-muted);margin-top:4px">${App.escapeHtml(model)}</div>
        </div>
      </div>
    `;
  }

  function renderMemory(mem) {
    if (!mem) return;
    const used = mem.used || 0;
    const total = mem.total || 1;
    const pct = (used / total) * 100;
    document.getElementById('card-memory').innerHTML = `
      <div class="metric-label">Memory</div>
      <div class="gauge-container">
        ${renderGauge(pct)}
        <div style="flex:1">
          <div style="font-size:13px;color:var(--text-secondary)">${App.formatBytes(used)} / ${App.formatBytes(total)}</div>
          <div style="margin-top:8px">
            ${renderBar(pct)}
          </div>
        </div>
      </div>
    `;
  }

  function renderDisk(disks) {
    if (!disks || !disks.length) {
      document.getElementById('card-disk').innerHTML = '<div class="metric-label">Disk</div><div class="text-muted text-sm">No disk data</div>';
      return;
    }
    let html = '<div class="metric-label">Disks</div>';
    disks.forEach(d => {
      const total = d.total || 1;
      const used = d.used || 0;
      const pct = (used / total) * 100;
      html += `
        <div class="disk-card">
          <div class="flex justify-between items-center mb-8">
            <span class="disk-mount">${App.escapeHtml(d.mount || '/')}</span>
            <span class="text-sm text-muted">${App.formatBytes(used)} / ${App.formatBytes(total)}</span>
          </div>
          ${renderBar(pct)}
          <div class="disk-detail">${App.escapeHtml(d.device || '')} &middot; ${App.escapeHtml(d.fstype || '')}</div>
        </div>`;
    });
    document.getElementById('card-disk').innerHTML = html;
  }

  function renderNetwork(net) {
    if (!net || !net.length) {
      document.getElementById('card-network').innerHTML = '<div class="metric-label">Network</div><div class="text-muted text-sm">No network data</div>';
      return;
    }
    let html = '<div class="metric-label">Network</div>';
    net.forEach(iface => {
      if (iface.name === 'lo') return;
      html += `
        <div style="margin-bottom:10px;padding:8px 0;border-bottom:1px solid var(--border-color)">
          <div style="font-size:13px;font-weight:500;color:var(--text-primary)">${App.escapeHtml(iface.name)}</div>
          <div style="display:flex;gap:16px;margin-top:4px;font-size:12px;color:var(--text-secondary)">
            <span>&#8595; RX: ${App.formatBytes(iface.rx || 0)}</span>
            <span>&#8593; TX: ${App.formatBytes(iface.tx || 0)}</span>
          </div>
          ${iface.address ? `<div style="font-size:11px;color:var(--text-muted);margin-top:2px;font-family:var(--font-mono)">${App.escapeHtml(iface.address)}</div>` : ''}
        </div>`;
    });
    document.getElementById('card-network').innerHTML = html;
  }

  function renderLoad(load) {
    if (!load) return;
    const l1 = load.load1 || 0;
    const l5 = load.load5 || 0;
    const l15 = load.load15 || 0;
    const cores = load.cores || 1;
    document.getElementById('card-load').innerHTML = `
      <div class="metric-label">Load Average</div>
      <div style="display:flex;gap:20px;margin-top:8px">
        <div>
          <div style="font-size:22px;font-weight:700;color:var(--text-primary)">${l1.toFixed(2)}</div>
          <div style="font-size:11px;color:var(--text-muted)">1 min</div>
        </div>
        <div>
          <div style="font-size:22px;font-weight:700;color:var(--text-primary)">${l5.toFixed(2)}</div>
          <div style="font-size:11px;color:var(--text-muted)">5 min</div>
        </div>
        <div>
          <div style="font-size:22px;font-weight:700;color:var(--text-primary)">${l15.toFixed(2)}</div>
          <div style="font-size:11px;color:var(--text-muted)">15 min</div>
        </div>
      </div>
      <div style="font-size:12px;color:var(--text-muted);margin-top:8px">${cores} cores</div>
    `;
  }

  function renderProcesses(procs) {
    allProcesses = procs;
    const tbody = document.getElementById('process-tbody');
    if (!tbody) return;
    if (!procs.length) {
      tbody.innerHTML = '<tr><td colspan="5" class="text-center text-muted" style="padding:20px">No process data</td></tr>';
      return;
    }
    const sorted = [...procs].sort((a, b) => {
      let va = a[sortField], vb = b[sortField];
      if (typeof va === 'string') return sortDir * va.localeCompare(vb);
      return sortDir * ((va || 0) - (vb || 0));
    });
    tbody.innerHTML = sorted.map(p => `
      <tr>
        <td class="mono">${p.pid || '-'}</td>
        <td>${App.escapeHtml(p.name || p.command || '-')}</td>
        <td class="mono" style="color:${(p.cpu||0) > 50 ? 'var(--danger)' : 'var(--text-primary)'}">${(p.cpu || 0).toFixed(1)}%</td>
        <td class="mono">${(p.mem || 0).toFixed(1)}%</td>
        <td><span class="badge badge-info">${App.escapeHtml(p.status || '-')}</span></td>
      </tr>
    `).join('');
  }

  fetchMetrics();
  refreshTimer = setInterval(fetchMetrics, 3000);

  return function cleanup() {
    if (refreshTimer) {
      clearInterval(refreshTimer);
      refreshTimer = null;
    }
  };
}
