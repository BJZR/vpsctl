function DockerPage(container) {
  let refreshTimer = null;
  let containers = [];
  let filterText = '';

  container.innerHTML = `
    <div class="page-header">
      <h2>Docker</h2>
      <div class="actions">
        <div class="search-input-wrapper">
          <span class="search-icon">&#128269;</span>
          <input type="text" class="form-input search-input" id="docker-search" placeholder="Filter containers..." style="width:200px;padding-left:32px">
        </div>
        <button class="btn btn-ghost btn-sm" id="docker-refresh">&#8635; Refresh</button>
      </div>
    </div>
    <div class="page-content">
      <div class="card mb-16">
        <div class="card-header">
          <span>Containers</span>
          <span class="badge badge-info" id="docker-count">0</span>
        </div>
        <div class="card-body">
          <div class="docker-grid" id="docker-containers">
            <div class="spinner-center"><div class="spinner"></div></div>
          </div>
        </div>
      </div>
      <div class="card">
        <div class="card-header">
          <span>Images</span>
        </div>
        <div class="card-body-np">
          <div class="table-wrapper">
            <table>
              <thead>
                <tr>
                  <th>Repository</th>
                  <th>Tag</th>
                  <th>ID</th>
                  <th>Size</th>
                </tr>
              </thead>
              <tbody id="docker-images">
                <tr><td colspan="4" class="text-center text-muted" style="padding:20px">Loading...</td></tr>
              </tbody>
            </table>
          </div>
        </div>
      </div>
    </div>
    <!-- Logs Modal -->
    <div class="modal-overlay" id="docker-logs-overlay">
      <div class="modal" style="max-width:900px;height:80vh">
        <div class="modal-header">
          <h3 id="docker-logs-title">Container Logs</h3>
          <button class="modal-close" id="docker-logs-close">&times;</button>
        </div>
        <div class="modal-body modal-body-np" style="flex:1;overflow:hidden;display:flex;flex-direction:column">
          <div class="journal-output" id="docker-logs-output" style="flex:1;overflow-y:auto;background:#000;color:var(--text-primary);font-size:12px"></div>
        </div>
        <div class="modal-footer">
          <button class="btn btn-ghost btn-sm" id="docker-logs-auto">&#9654; Auto-refresh</button>
          <button class="btn btn-ghost btn-sm" id="docker-logs-close-btn">Close</button>
        </div>
      </div>
    </div>
  `;

  const containersGrid = document.getElementById('docker-containers');
  const imagesTbody = document.getElementById('docker-images');
  const logsOverlay = document.getElementById('docker-logs-overlay');
  const logsOutput = document.getElementById('docker-logs-output');
  let logsRefresh = null;
  let logsContainerId = null;

  function statusBadge(status) {
    if (!status) return '<span class="badge badge-neutral">Unknown</span>';
    const s = status.toLowerCase();
    if (s.includes('running')) return '<span class="badge badge-success">Running</span>';
    if (s.includes('exited') || s.includes('stopped') || s.includes('dead')) return '<span class="badge badge-danger">' + App.escapeHtml(status) + '</span>';
    if (s.includes('restarting') || s.includes('paused')) return '<span class="badge badge-warning">' + App.escapeHtml(status) + '</span>';
    return '<span class="badge badge-neutral">' + App.escapeHtml(status) + '</span>';
  }

  function renderContainers() {
    const filtered = containers.filter(c => {
      if (!filterText) return true;
      const t = filterText.toLowerCase();
      return (c.name || '').toLowerCase().includes(t) || (c.image || '').toLowerCase().includes(t) || (c.id || '').toLowerCase().includes(t);
    });

    document.getElementById('docker-count').textContent = filtered.length + '/' + containers.length;

    if (!filtered.length) {
      containersGrid.innerHTML = '<div class="empty-state"><div class="empty-icon">&#9881;</div><h3>No containers found</h3></div>';
      return;
    }

    containersGrid.innerHTML = filtered.map(c => {
      const running = (c.status || '').toLowerCase().includes('running');
      return `
        <div class="container-card">
          <div class="container-header">
            <span class="container-name" title="${App.escapeHtml(c.name || c.id)}">${App.escapeHtml(c.name || c.id)}</span>
            ${statusBadge(c.status)}
          </div>
          <div class="container-image" title="${App.escapeHtml(c.image || '')}">${App.escapeHtml(c.image || '-')}</div>
          <div class="container-meta">
            ${c.ports ? `<span>Ports: ${App.escapeHtml(c.ports)}</span>` : ''}
            ${c.cpu != null ? `<span>CPU: ${(c.cpu || 0).toFixed(1)}%</span>` : ''}
            ${c.memory != null ? `<span>MEM: ${App.formatBytes(c.memory || 0)}</span>` : ''}
          </div>
          <div class="container-actions">
            ${running
              ? `<button class="btn btn-warning btn-sm" data-action="stop" data-id="${c.id}">&#9632; Stop</button>
                 <button class="btn btn-ghost btn-sm" data-action="restart" data-id="${c.id}">&#8635; Restart</button>`
              : `<button class="btn btn-success btn-sm" data-action="start" data-id="${c.id}">&#9654; Start</button>`}
            <button class="btn btn-ghost btn-sm" data-action="logs" data-id="${c.id}" data-name="${App.escapeHtml(c.name || c.id)}">&#128196; Logs</button>
            <button class="btn btn-danger btn-sm" data-action="delete" data-id="${c.id}">&#10005;</button>
          </div>
        </div>`;
    }).join('');

    containersGrid.querySelectorAll('[data-action]').forEach(btn => {
      btn.addEventListener('click', () => handleAction(btn.dataset.action, btn.dataset.id, btn.dataset.name));
    });
  }

  function renderImages(images) {
    if (!images || !images.length) {
      imagesTbody.innerHTML = '<tr><td colspan="4" class="text-center text-muted">No images</td></tr>';
      return;
    }
    imagesTbody.innerHTML = images.map(img => `
      <tr>
        <td>${App.escapeHtml(img.repository || '-')}</td>
        <td>${App.escapeHtml(img.tag || '-')}</td>
        <td class="mono text-muted text-sm">${App.escapeHtml(img.id || '-')}</td>
        <td class="text-muted">${App.formatBytes(img.size || 0)}</td>
      </tr>
    `).join('');
  }

  async function fetchContainers() {
    try {
      const data = await App.api.get('/docker/containers');
      containers = data.containers || data || [];
      renderContainers();
    } catch (err) {
      containersGrid.innerHTML = `<div class="empty-state"><div class="empty-icon">&#9888;</div><h3>Error loading containers</h3><p>${App.escapeHtml(err.message)}</p></div>`;
    }
  }

  async function fetchImages() {
    try {
      const data = await App.api.get('/docker/images');
      renderImages(data.images || data || []);
    } catch {
      imagesTbody.innerHTML = '<tr><td colspan="4" class="text-center text-muted">Failed to load</td></tr>';
    }
  }

  async function handleAction(action, id, name) {
    if (action === 'delete') {
      const ok = await App.confirm('Delete Container', `Permanently delete container ${name || id}?`, 'Delete');
      if (!ok) return;
    }
    if (action === 'logs') {
      openLogs(id, name);
      return;
    }
    try {
      await App.api.post(`/docker/containers/${id}/${action}`);
      App.Toast.success(`Container ${action} successful`);
      fetchContainers();
    } catch (err) {
      App.Toast.error(`Failed to ${action}: ${err.message}`);
    }
  }

  async function openLogs(id, name) {
    logsContainerId = id;
    document.getElementById('docker-logs-title').textContent = 'Logs: ' + (name || id);
    logsOutput.textContent = 'Loading...';
    logsOverlay.classList.add('visible');
    await fetchLogs();
  }

  async function fetchLogs() {
    if (!logsContainerId) return;
    try {
      const data = await App.api.get('/docker/containers/' + logsContainerId + '/logs?tail=200');
      const text = data.logs || data || '';
      logsOutput.textContent = text;
      logsOutput.scrollTop = logsOutput.scrollHeight;
    } catch (err) {
      logsOutput.textContent = 'Error: ' + err.message;
    }
  }

  function closeLogs() {
    logsOverlay.classList.remove('visible');
    logsContainerId = null;
    if (logsRefresh) {
      clearInterval(logsRefresh);
      logsRefresh = null;
    }
  }

  document.getElementById('docker-logs-close').addEventListener('click', closeLogs);
  document.getElementById('docker-logs-close-btn').addEventListener('click', closeLogs);
  logsOverlay.addEventListener('click', (e) => { if (e.target === logsOverlay) closeLogs(); });

  document.getElementById('docker-logs-auto').addEventListener('click', function() {
    if (logsRefresh) {
      clearInterval(logsRefresh);
      logsRefresh = null;
      this.textContent = '\u25B6 Auto-refresh';
    } else {
      logsRefresh = setInterval(fetchLogs, 2000);
      this.textContent = '\u23F8 Pause';
    }
  });

  document.getElementById('docker-search').addEventListener('input', (e) => {
    filterText = e.target.value;
    renderContainers();
  });

  document.getElementById('docker-refresh').addEventListener('click', () => {
    fetchContainers();
    fetchImages();
  });

  fetchContainers();
  fetchImages();
  refreshTimer = setInterval(fetchContainers, 10000);

  return function cleanup() {
    if (refreshTimer) {
      clearInterval(refreshTimer);
      refreshTimer = null;
    }
    if (logsRefresh) {
      clearInterval(logsRefresh);
      logsRefresh = null;
    }
  };
}
