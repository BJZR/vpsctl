function FilesPage(container) {
  let currentPath = '/';
  let files = [];
  let sortField = 'name';
  let sortDir = 1;

  container.innerHTML = `
    <div class="page-header">
      <h2>File Manager</h2>
      <div class="actions">
        <button class="btn btn-ghost btn-sm" id="files-upload-btn">&#8593; Upload</button>
        <button class="btn btn-ghost btn-sm" id="files-mkdir-btn">&#128193;+ New Folder</button>
      </div>
    </div>
    <div class="page-content">
      <div class="breadcrumb" id="files-breadcrumb"></div>
      <div class="card">
        <div class="card-body-np">
          <div class="table-wrapper">
            <table>
              <thead>
                <tr>
                  <th class="sortable" data-sort="name">Name <span class="sort-arrow"></span></th>
                  <th class="sortable" data-sort="size">Size <span class="sort-arrow"></span></th>
                  <th class="sortable" data-sort="modified">Modified <span class="sort-arrow"></span></th>
                  <th>Permissions</th>
                  <th style="width:120px">Actions</th>
                </tr>
              </thead>
              <tbody id="files-tbody">
                <tr><td colspan="5" class="text-center text-muted" style="padding:20px">Loading...</td></tr>
              </tbody>
            </table>
          </div>
        </div>
      </div>
    </div>
    <!-- Editor Modal -->
    <div class="modal-overlay" id="file-editor-overlay">
      <div class="modal" style="max-width:800px">
        <div class="modal-header">
          <h3 id="editor-filename">Edit File</h3>
          <button class="modal-close" id="editor-close">&times;</button>
        </div>
        <div class="modal-body modal-body-np">
          <textarea class="editor-textarea" id="editor-content" spellcheck="false"></textarea>
        </div>
        <div class="modal-footer">
          <button class="btn btn-ghost" id="editor-cancel">Cancel</button>
          <button class="btn btn-primary" id="editor-save">Save</button>
        </div>
      </div>
    </div>
    <!-- Hidden file input -->
    <input type="file" id="files-file-input" style="display:none">
  `;

  const tbody = document.getElementById('files-tbody');
  const breadcrumb = document.getElementById('files-breadcrumb');
  const editorOverlay = document.getElementById('file-editor-overlay');
  const editorContent = document.getElementById('editor-content');
  const editorFilename = document.getElementById('editor-filename');
  const fileInput = document.getElementById('files-file-input');

  let editingPath = '';

  function renderBreadcrumb() {
    const parts = currentPath.split('/').filter(Boolean);
    let html = `<span class="breadcrumb-item ${parts.length === 0 ? 'current' : ''}" data-path="/">/</span>`;
    let accumulated = '';
    parts.forEach((part, i) => {
      accumulated += '/' + part;
      const isLast = i === parts.length - 1;
      html += `<span class="breadcrumb-sep">/</span>`;
      html += `<span class="breadcrumb-item ${isLast ? 'current' : ''}" data-path="${App.escapeHtml(accumulated)}">${App.escapeHtml(part)}</span>`;
    });
    breadcrumb.innerHTML = html;
    breadcrumb.querySelectorAll('.breadcrumb-item').forEach(el => {
      el.addEventListener('click', () => {
        if (!el.classList.contains('current')) {
          navigateTo(el.dataset.path);
        }
      });
    });
  }

  function fileIcon(entry) {
    if (entry.is_dir) return '&#128193;';
    const ext = (entry.name || '').split('.').pop().toLowerCase();
    const icons = {
      js: '&#128309;', ts: '&#128309;', py: '&#128013;', go: '&#128013;',
      html: '&#127760;', css: '&#127912;', json: '&#128196;', md: '&#128221;',
      sh: '&#9881;', bash: '&#9881;', yml: '&#128196;', yaml: '&#128196;',
      txt: '&#128196;', log: '&#128196;', conf: '&#128196;', cfg: '&#128196;',
      png: '&#127748;', jpg: '&#127748;', jpeg: '&#127748;', gif: '&#127748;', svg: '&#127748;',
      zip: '&#128230;', tar: '&#128230;', gz: '&#128230;',
    };
    return icons[ext] || '&#128196;';
  }

  function renderFiles() {
    const sorted = [...files].sort((a, b) => {
      // Directories first
      if (a.is_dir && !b.is_dir) return -1;
      if (!a.is_dir && b.is_dir) return 1;
      let va = a[sortField], vb = b[sortField];
      if (sortField === 'size') return sortDir * ((va || 0) - (vb || 0));
      if (sortField === 'modified') return sortDir * (new Date(va || 0) - new Date(vb || 0));
      return sortDir * String(va || '').localeCompare(String(vb || ''));
    });

    if (!sorted.length) {
      tbody.innerHTML = '<tr><td colspan="5" class="text-center text-muted" style="padding:40px">Empty directory</td></tr>';
      return;
    }

    tbody.innerHTML = sorted.map(f => {
      const name = f.name || '';
      const encodedPath = encodeURIComponent(currentPath === '/' ? '/' + name : currentPath + '/' + name);
      return `
        <tr data-name="${App.escapeHtml(name)}" data-isdir="${f.is_dir}">
          <td>
            <span class="file-type-icon">${fileIcon(f)}</span>
            <span style="cursor:${f.is_dir ? 'pointer' : 'default'};color:${f.is_dir ? 'var(--accent)' : 'var(--text-primary)'}" class="file-name-link" data-name="${App.escapeHtml(name)}" data-isdir="${f.is_dir}">${App.escapeHtml(name)}</span>
          </td>
          <td class="mono text-muted">${f.is_dir ? '-' : App.formatBytes(f.size)}</td>
          <td class="text-muted">${App.formatDate(f.modified)}</td>
          <td class="mono text-muted text-sm">${App.escapeHtml(f.permissions || '-')}</td>
          <td>
            <div class="file-actions">
              ${!f.is_dir ? `<button class="btn btn-ghost btn-sm btn-icon file-edit" data-path="${encodedPath}" data-name="${App.escapeHtml(name)}" title="Edit">&#9998;</button>` : ''}
              ${!f.is_dir ? `<button class="btn btn-ghost btn-sm btn-icon file-download" data-path="${encodedPath}" title="Download">&#8615;</button>` : ''}
              <button class="btn btn-ghost btn-sm btn-icon file-rename" data-name="${App.escapeHtml(name)}" title="Rename">&#9998;&#8634;</button>
              <button class="btn btn-ghost btn-sm btn-icon file-delete" data-name="${App.escapeHtml(name)}" title="Delete" style="color:var(--danger)">&#10005;</button>
            </div>
          </td>
        </tr>`;
    }).join('');

    // Bind events
    tbody.querySelectorAll('.file-name-link').forEach(el => {
      el.addEventListener('click', () => {
        if (el.dataset.isdir === 'true') {
          const newName = el.dataset.name;
          navigateTo(currentPath === '/' ? '/' + newName : currentPath + '/' + newName);
        } else {
          // Open in editor
          const encodedPath = encodeURIComponent(currentPath === '/' ? '/' + el.dataset.name : currentPath + '/' + el.dataset.name);
          openEditor(encodedPath, el.dataset.name);
        }
      });
    });

    tbody.querySelectorAll('.file-edit').forEach(btn => {
      btn.addEventListener('click', () => openEditor(btn.dataset.path, btn.dataset.name));
    });

    tbody.querySelectorAll('.file-download').forEach(btn => {
      btn.addEventListener('click', () => {
        window.open('/api/files/download?path=' + btn.dataset.path, '_blank');
      });
    });

    tbody.querySelectorAll('.file-rename').forEach(btn => {
      btn.addEventListener('click', () => handleRename(btn.dataset.name));
    });

    tbody.querySelectorAll('.file-delete').forEach(btn => {
      btn.addEventListener('click', () => handleDelete(btn.dataset.name));
    });
  }

  async function loadFiles(path) {
    currentPath = path || '/';
    renderBreadcrumb();
    tbody.innerHTML = '<tr><td colspan="5" class="text-center" style="padding:20px"><div class="spinner-center"><div class="spinner"></div></div></td></tr>';
    try {
      const data = await App.api.get('/files/list?path=' + encodeURIComponent(currentPath));
      files = data.files || data || [];
      renderFiles();
    } catch (err) {
      tbody.innerHTML = `<tr><td colspan="5" class="text-center text-danger" style="padding:20px">${App.escapeHtml(err.message)}</td></tr>`;
    }
  }

  function navigateTo(path) {
    currentPath = path;
    loadFiles(path);
  }

  async function openEditor(encodedPath, name) {
    editorFilename.textContent = name;
    editorContent.value = 'Loading...';
    editorOverlay.classList.add('visible');
    editingPath = decodeURIComponent(encodedPath);
    try {
      const res = await App.api.get('/files/content?path=' + encodedPath);
      const text = await res.text();
      editorContent.value = text;
    } catch (err) {
      editorContent.value = 'Error loading file: ' + err.message;
    }
    editorContent.focus();
  }

  function closeEditor() {
    editorOverlay.classList.remove('visible');
  }

  document.getElementById('editor-close').addEventListener('click', closeEditor);
  document.getElementById('editor-cancel').addEventListener('click', closeEditor);
  editorOverlay.addEventListener('click', (e) => { if (e.target === editorOverlay) closeEditor(); });

  document.getElementById('editor-save').addEventListener('click', async () => {
    try {
      await App.api.post('/files/write', { path: editingPath, content: editorContent.value });
      App.Toast.success('File saved successfully');
      closeEditor();
      loadFiles(currentPath);
    } catch (err) {
      App.Toast.error('Failed to save: ' + err.message);
    }
  });

  // Upload
  document.getElementById('files-upload-btn').addEventListener('click', () => fileInput.click());
  fileInput.addEventListener('change', async () => {
    const file = fileInput.files[0];
    if (!file) return;
    const formData = new FormData();
    formData.append('file', file);
    formData.append('path', currentPath);
    try {
      await App.api.post('/files/upload', formData);
      App.Toast.success('Uploaded ' + file.name);
      loadFiles(currentPath);
    } catch (err) {
      App.Toast.error('Upload failed: ' + err.message);
    }
    fileInput.value = '';
  });

  // Mkdir
  document.getElementById('files-mkdir-btn').addEventListener('click', async () => {
    const name = prompt('New folder name:');
    if (!name) return;
    try {
      await App.api.post('/files/mkdir', { path: currentPath, name: name });
      App.Toast.success('Folder created');
      loadFiles(currentPath);
    } catch (err) {
      App.Toast.error('Failed: ' + err.message);
    }
  });

  async function handleRename(name) {
    const newName = prompt('Rename to:', name);
    if (!newName || newName === name) return;
    try {
      const oldPath = currentPath === '/' ? '/' + name : currentPath + '/' + name;
      const newPath = currentPath === '/' ? '/' + newName : currentPath + '/' + newName;
      await App.api.post('/files/rename', { old_path: oldPath, new_path: newPath });
      App.Toast.success('Renamed to ' + newName);
      loadFiles(currentPath);
    } catch (err) {
      App.Toast.error('Rename failed: ' + err.message);
    }
  }

  async function handleDelete(name) {
    const ok = await App.confirm('Delete', `Delete "${name}"? This cannot be undone.`, 'Delete');
    if (!ok) return;
    try {
      const path = currentPath === '/' ? '/' + name : currentPath + '/' + name;
      await App.api.del('/files/delete?path=' + encodeURIComponent(path));
      App.Toast.success('Deleted ' + name);
      loadFiles(currentPath);
    } catch (err) {
      App.Toast.error('Delete failed: ' + err.message);
    }
  }

  // Sort headers
  container.querySelectorAll('thead th.sortable').forEach(th => {
    th.addEventListener('click', () => {
      const field = th.dataset.sort;
      if (sortField === field) sortDir *= -1;
      else { sortField = field; sortDir = 1; }
      container.querySelectorAll('thead th.sortable').forEach(h => h.classList.remove('sorted'));
      th.classList.add('sorted');
      renderFiles();
    });
  });

  loadFiles('/');

  return function cleanup() {};
}
