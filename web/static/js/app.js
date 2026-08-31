const App = (() => {
  let currentUser = null;

  // ============ API ============
  const api = {
    async request(method, path, body) {
      const opts = {
        method,
        headers: {},
        credentials: 'same-origin',
      };
      if (body && !(body instanceof FormData)) {
        opts.headers['Content-Type'] = 'application/json';
        opts.body = JSON.stringify(body);
      } else if (body) {
        opts.body = body;
      }
      const res = await fetch('/api' + path, opts);
      if (res.status === 401) {
        currentUser = null;
        showLogin();
        Toast.error('Session expired. Please sign in again.');
        throw new Error('Unauthorized');
      }
      if (!res.ok) {
        let msg = `Request failed (${res.status})`;
        try {
          const data = await res.json();
          msg = data.error || data.message || msg;
        } catch {}
        throw new Error(msg);
      }
      const ct = res.headers.get('content-type') || '';
      if (ct.includes('application/json')) return res.json();
      return res;
    },
    get(path) { return this.request('GET', path); },
    post(path, body) { return this.request('POST', path, body); },
    put(path, body) { return this.request('PUT', path, body); },
    del(path) { return this.request('DELETE', path); },
  };

  // ============ TOAST ============
  const Toast = (() => {
    const container = document.getElementById('toast-container');
    function show(type, message, duration = 4000) {
      const icons = { success: '\u2713', error: '\u2717', warning: '\u26A0', info: '\u2139' };
      const el = document.createElement('div');
      el.className = `toast toast-${type}`;
      el.innerHTML = `<span class="toast-icon">${icons[type] || ''}</span><span class="toast-message">${escapeHtml(message)}</span>`;
      container.appendChild(el);
      setTimeout(() => {
        el.classList.add('removing');
        setTimeout(() => el.remove(), 300);
      }, duration);
    }
    return {
      success: (msg) => show('success', msg),
      error: (msg) => show('error', msg, 6000),
      warning: (msg) => show('warning', msg, 5000),
      info: (msg) => show('info', msg),
    };
  })();

  // ============ CONFIRM ============
  function confirm(title, message, dangerLabel) {
    return new Promise((resolve) => {
      const overlay = document.getElementById('confirm-overlay');
      document.getElementById('confirm-title').textContent = title;
      document.getElementById('confirm-message').textContent = message;
      const okBtn = document.getElementById('confirm-ok');
      okBtn.textContent = dangerLabel || 'Confirm';
      overlay.classList.add('visible');
      const cleanup = (val) => {
        overlay.classList.remove('visible');
        resolve(val);
      };
      document.getElementById('confirm-cancel').onclick = () => cleanup(false);
      okBtn.onclick = () => cleanup(true);
      overlay.onclick = (e) => { if (e.target === overlay) cleanup(false); };
    });
  }

  // ============ HELPERS ============
  function escapeHtml(s) {
    const d = document.createElement('div');
    d.textContent = s;
    return d.innerHTML;
  }

  function formatBytes(bytes) {
    if (bytes == null || bytes === 0) return '0 B';
    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
    const i = Math.floor(Math.log(Math.abs(bytes)) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
  }

  function formatDuration(seconds) {
    if (!seconds || seconds < 0) return '-';
    const d = Math.floor(seconds / 86400);
    const h = Math.floor((seconds % 86400) / 3600);
    const m = Math.floor((seconds % 3600) / 60);
    const parts = [];
    if (d > 0) parts.push(d + 'd');
    if (h > 0) parts.push(h + 'h');
    if (m > 0) parts.push(m + 'm');
    if (parts.length === 0) parts.push(Math.floor(seconds) + 's');
    return parts.join(' ');
  }

  function formatDate(dateStr) {
    if (!dateStr) return '-';
    const d = new Date(dateStr);
    return d.toLocaleString();
  }

  function formatPercent(val, total) {
    if (!total) return '0%';
    return ((val / total) * 100).toFixed(1) + '%';
  }

  function getPercentColor(pct) {
    if (pct >= 90) return 'red';
    if (pct >= 70) return 'yellow';
    return 'green';
  }

  // ============ AUTH ============
  async function login(username, password, totp) {
    const body = { username, password };
    if (totp) body.totp = totp;
    return api.post('/auth/login', body);
  }

  async function logout() {
    try { await api.post('/auth/logout'); } catch {}
    currentUser = null;
    showLogin();
  }

  async function checkAuth() {
    try {
      const data = await api.get('/auth/me');
      currentUser = data.user || data;
      showApp();
      return true;
    } catch {
      showLogin();
      return false;
    }
  }

  function showLogin() {
    document.getElementById('login-page').style.display = '';
    document.getElementById('app-layout').style.display = 'none';
    document.getElementById('login-username').focus();
  }

  function showApp() {
    document.getElementById('login-page').style.display = 'none';
    document.getElementById('app-layout').style.display = '';
    if (currentUser) {
      const name = currentUser.username || 'User';
      document.getElementById('user-name').textContent = name;
      document.getElementById('user-avatar').textContent = name.charAt(0).toUpperCase();
    }
    Router.navigate(getCurrentRoute() || '/dashboard');
  }

  function getCurrentRoute() {
    const hash = location.hash.slice(1);
    return hash || '/dashboard';
  }

  // ============ ROUTER ============
  const Router = (() => {
    const routes = {};
    let currentCleanup = null;

    function register(path, handler) {
      routes[path] = handler;
    }

    function navigate(path) {
      if (currentCleanup) {
        currentCleanup();
        currentCleanup = null;
      }
      location.hash = '#' + path;
      render(path);
    }

    function render(path) {
      const main = document.getElementById('main-content');
      // Update active nav
      document.querySelectorAll('.nav-link').forEach(link => {
        link.classList.toggle('active', link.dataset.route === path);
      });
      // Close mobile sidebar
      document.getElementById('sidebar').classList.remove('open');
      document.getElementById('sidebar-overlay').classList.remove('visible');

      const handler = routes[path];
      if (handler) {
        currentCleanup = handler(main);
      } else {
        main.innerHTML = '<div class="page-content"><div class="empty-state"><div class="empty-icon">&#128269;</div><h3>Page not found</h3><p>The page you\'re looking for doesn\'t exist.</p></div></div>';
      }
    }

    function init() {
      window.addEventListener('hashchange', () => {
        const path = getCurrentRoute();
        render(path);
      });
    }

    return { register, navigate, init };
  })();

  // ============ INIT ============
  function init() {
    // Sidebar nav clicks
    document.querySelectorAll('.nav-link').forEach(link => {
      link.addEventListener('click', (e) => {
        e.preventDefault();
        Router.navigate(link.dataset.route);
      });
    });

    // Hamburger
    document.getElementById('hamburger').addEventListener('click', () => {
      document.getElementById('sidebar').classList.toggle('open');
      document.getElementById('sidebar-overlay').classList.toggle('visible');
    });
    document.getElementById('sidebar-overlay').addEventListener('click', () => {
      document.getElementById('sidebar').classList.remove('open');
      document.getElementById('sidebar-overlay').classList.remove('visible');
    });

    // Logout
    document.getElementById('btn-logout').addEventListener('click', logout);

    // Login form
    document.getElementById('login-form').addEventListener('submit', async (e) => {
      e.preventDefault();
      const username = document.getElementById('login-username').value.trim();
      const password = document.getElementById('login-password').value;
      const totp = document.getElementById('login-totp').value.trim();
      const submitBtn = document.getElementById('login-submit');
      const btnText = document.getElementById('login-btn-text');
      const btnSpinner = document.getElementById('login-btn-spinner');
      const errorEl = document.getElementById('login-error');

      submitBtn.disabled = true;
      btnText.textContent = 'Signing in...';
      btnSpinner.style.display = '';
      errorEl.classList.remove('visible');

      try {
        await login(username, password, totp);
        await checkAuth();
      } catch (err) {
        errorEl.textContent = err.message;
        errorEl.classList.add('visible');
        // Show TOTP field if needed
        if (err.message.toLowerCase().includes('totp') || err.message.toLowerCase().includes('2fa') || err.message.toLowerCase().includes('authenticator')) {
          document.getElementById('totp-group').style.display = '';
          document.getElementById('login-totp').focus();
        }
      } finally {
        submitBtn.disabled = false;
        btnText.textContent = 'Sign In';
        btnSpinner.style.display = 'none';
      }
    });

    // Register routes
    Router.register('/dashboard', DashboardPage);
    Router.register('/terminal', TerminalPage);
    Router.register('/files', FilesPage);
    Router.register('/docker', DockerPage);
    Router.register('/services', ServicesPage);
    Router.register('/settings', SettingsPage);

    Router.init();
    checkAuth();
  }

  document.addEventListener('DOMContentLoaded', init);

  return { api, Toast, confirm, Router, formatBytes, formatDuration, formatDate, formatPercent, getPercentColor, escapeHtml, get currentUser() { return currentUser; } };
})();
