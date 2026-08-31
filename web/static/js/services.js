function ServicesPage(container) {
  let refreshTimer = null;
  let services = [];
  let filterText = '';
  let selectedService = null;
  let journalRefresh = null;

  container.innerHTML = `
    <div class="page-header">
      <h2>Services</h2>
      <div class="actions">
        <div class="search-input-wrapper">
          <span class="search-icon">&#128269;</span>
          <input type="text" class="form-input search-input" id="svc-search" placeholder="Filter services..." style="width:220px;padding-left:32px">
        </div>
        <button class="btn btn-ghost btn-sm" id="svc-refresh">&#8635; Refresh</button>
      </div>
    </div>
    <div class="page-content">
      <div class="card">
        <div class="card-body-np">
          <div class="table-wrapper">
            <table>
              <thead>
                <tr>
                  <th>Name</th>
                  <th>Status</th>
                  <th>Description</th>
                  <th style="width:200px">Actions</th>
                </tr>
              </thead>
              <tbody id="svc-tbody">
                <tr><td colspan="4" class="text-center" style="padding:20px"><div class="spinner-center"><div class="spinner"></div></div></td></tr>
              </tbody>
            </table>
          </div>
        </div>
      </div>
      <div id="svc-detail"></div>
    </div>
  `;

  const tbody = document.getElementById('svc-tbody');
  const detailDiv = document.getElementById('svc-detail');

  function statusBadge(status) {
    if (!status) return '<span class="badge badge-neutral">Unknown</span>';
    const s = status.toLowerCase();
    if (s === 'active') return '<span class="badge badge-success">Active</span>';
    if (s === 'inactive' || s === 'dead') return '<span class="badge badge-neutral">' + App.escapeHtml(status) + '</span>';
    if (s === 'failed') return '<span class="badge badge-danger">Failed</span>';
    if (s.includes('activating') || s.includes('reloading')) return '<span class="badge badge-warning">' + App.escapeHtml(status) + '</span>';
    return '<span class="badge badge-neutral">' + App.escapeHtml(status) + '</span>';
  }

  function renderServices() {
    const filtered = services.filter(s => {
      if (!filterText) return true;
      const t = filterText.toLowerCase();
      return (s.name || '').toLowerCase().includes(t) || (s.description || '').toLowerCase().includes(t);
    });

    if (!filtered.length) {
      tbody.innerHTML = '<tr><td colspan="4" class="text-center text-muted" style="padding:20px">No services found</td></tr>';
      return;
    }

    tbody.innerHTML = filtered.map(s => `
      <tr data-name="${App.escapeHtml(s.name)}">
        <td>
          <span style="color:var(--accent);cursor:pointer;font-weight:500" class="svc-name-link" data-name="${App.escapeHtml(s.name)}">${App.escapeHtml(s.name)}</span>
        </td>
        <td>${statusBadge(s.status)}</td>
        <td class="text-secondary" style="max-width:300px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap">${App.escapeHtml(s.description || '-')}</td>
        <td>
          <div class="flex gap-4">
            ${s.status !== 'active' ? `<button class="btn btn-success btn-sm" data-action="start" data-name="${App.escapeHtml(s.name)}">&#9654; Start</button>` : ''}
            ${s.status === 'active' ? `<button class="btn btn-warning btn-sm" data-action="stop" data-name="${App.escapeHtml(s.name)}">&#9632; Stop</button>` : ''}
            <button class="btn btn-ghost btn-sm" data-action="restart" data-name="${App.escapeHtml(s.name)}">&#8635; Restart</button>
          </div>
        </td>
      </tr>
    `).join('');

    tbody.querySelectorAll('.svc-name-link').forEach(el => {
      el.addEventListener('click', () => showServiceDetail(el.dataset.name));
    });

    tbody.querySelectorAll('[data-action]').forEach(btn => {
      btn.addEventListener('click', () => handleServiceAction(btn.dataset.action, btn.dataset.name));
    });
  }

  async function fetchServices() {
    try {
      const data = await App.api.get('/services');
      services = data.services || data || [];
      renderServices();
    } catch (err) {
      tbody.innerHTML = `<tr><td colspan="4" class="text-center text-danger" style="padding:20px">${App.escapeHtml(err.message)}</td></tr>`;
    }
  }

  async function handleServiceAction(action, name) {
    try {
      await App.api.post(`/services/${encodeURIComponent(name)}/${action}`);
      App.Toast.success(`Service ${action} successful`);
      fetchServices();
      if (selectedService === name) {
        showServiceDetail(name);
      }
    } catch (err) {
      App.Toast.error(`Failed to ${action}: ${err.message}`);
    }
  }

  async function showServiceDetail(name) {
    selectedService = name;
    detailDiv.innerHTML = `
      <div class="service-detail">
        <div class="service-detail-header">
          <h3>${App.escapeHtml(name)} &mdash; Journal</h3>
          <div class="flex gap-8">
            <button class="btn btn-ghost btn-sm" id="svc-journal-refresh">&#8635;</button>
            <button class="btn btn-ghost btn-sm" id="svc-journal-close">&#10005;</button>
          </div>
        </div>
        <div class="journal-output" id="svc-journal-output">
          <div class="spinner-center"><div class="spinner"></div></div>
        </div>
      </div>
    `;

    document.getElementById('svc-journal-close').addEventListener('click', () => {
      detailDiv.innerHTML = '';
      selectedService = null;
      if (journalRefresh) {
        clearInterval(journalRefresh);
        journalRefresh = null;
      }
    });

    document.getElementById('svc-journal-refresh').addEventListener('click', () => fetchJournal(name));

    await fetchJournal(name);

    if (journalRefresh) clearInterval(journalRefresh);
    journalRefresh = setInterval(() => fetchJournal(name), 5000);
  }

  async function fetchJournal(name) {
    const output = document.getElementById('svc-journal-output');
    if (!output) return;
    try {
      const data = await App.api.get('/services/' + encodeURIComponent(name) + '/journal?lines=100');
      const lines = data.lines || data.log || [];
      if (typeof lines === 'string') {
        output.textContent = lines;
      } else if (Array.isArray(lines)) {
        output.innerHTML = lines.map(line => {
          const text = typeof line === 'string' ? line : (line.message || line.text || JSON.stringify(line));
          let cls = 'journal-line';
          const lower = text.toLowerCase();
          if (lower.includes('error') || lower.includes('failed')) cls += ' error';
          else if (lower.includes('warn')) cls += ' warn';
          return `<div class="${cls}">${App.escapeHtml(text)}</div>`;
        }).join('');
      }
      output.scrollTop = output.scrollHeight;
    } catch (err) {
      output.textContent = 'Error loading journal: ' + err.message;
    }
  }

  document.getElementById('svc-search').addEventListener('input', (e) => {
    filterText = e.target.value;
    renderServices();
  });

  document.getElementById('svc-refresh').addEventListener('click', fetchServices);

  fetchServices();
  refreshTimer = setInterval(fetchServices, 5000);

  return function cleanup() {
    if (refreshTimer) {
      clearInterval(refreshTimer);
      refreshTimer = null;
    }
    if (journalRefresh) {
      clearInterval(journalRefresh);
      journalRefresh = null;
    }
  };
}
