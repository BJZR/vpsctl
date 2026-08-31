function SettingsPage(container) {

  container.innerHTML = `
    <div class="page-header">
      <h2>Settings</h2>
    </div>
    <div class="page-content">
      <div class="settings-grid">
        <!-- Change Password -->
        <div class="settings-section">
          <div class="section-header">Change Password</div>
          <div class="section-body">
            <form id="settings-pw-form">
              <div class="form-group">
                <label for="pw-old">Current Password</label>
                <input type="password" id="pw-old" class="form-input" autocomplete="current-password" required>
              </div>
              <div class="form-group">
                <label for="pw-new">New Password</label>
                <input type="password" id="pw-new" class="form-input" autocomplete="new-password" required minlength="8">
              </div>
              <div class="form-group">
                <label for="pw-confirm">Confirm New Password</label>
                <input type="password" id="pw-confirm" class="form-input" autocomplete="new-password" required minlength="8">
              </div>
              <button type="submit" class="btn btn-primary">Update Password</button>
            </form>
          </div>
        </div>

        <!-- TOTP 2FA -->
        <div class="settings-section">
          <div class="section-header">Two-Factor Authentication (TOTP)</div>
          <div class="section-body" id="totp-section">
            <div class="spinner-center"><div class="spinner"></div></div>
          </div>
        </div>

        <!-- Appearance -->
        <div class="settings-section">
          <div class="section-header">Appearance</div>
          <div class="section-body">
            <div class="flex items-center justify-between">
              <div>
                <div style="font-weight:500;color:var(--text-primary)">Dark Mode</div>
                <div class="text-sm text-muted mt-8">Toggle between dark and light themes</div>
              </div>
              <label class="toggle-switch">
                <input type="checkbox" id="theme-toggle" checked>
                <span class="toggle-slider"></span>
              </label>
            </div>
          </div>
        </div>

        <!-- System Info -->
        <div class="settings-section">
          <div class="section-header">System Information</div>
          <div class="section-body" id="sysinfo-section">
            <div class="spinner-center"><div class="spinner"></div></div>
          </div>
        </div>

        <!-- Sessions -->
        <div class="settings-section" style="grid-column: 1 / -1">
          <div class="section-header">Active Sessions</div>
          <div class="section-body" id="sessions-section">
            <div class="spinner-center"><div class="spinner"></div></div>
          </div>
        </div>
      </div>
    </div>
  `;

  // Password form
  document.getElementById('settings-pw-form').addEventListener('submit', async (e) => {
    e.preventDefault();
    const oldPw = document.getElementById('pw-old').value;
    const newPw = document.getElementById('pw-new').value;
    const confirmPw = document.getElementById('pw-confirm').value;

    if (newPw !== confirmPw) {
      App.Toast.error('Passwords do not match');
      return;
    }
    if (newPw.length < 8) {
      App.Toast.error('Password must be at least 8 characters');
      return;
    }

    try {
      await App.api.post('/auth/password', { old_password: oldPw, new_password: newPw });
      App.Toast.success('Password updated successfully');
      document.getElementById('pw-old').value = '';
      document.getElementById('pw-new').value = '';
      document.getElementById('pw-confirm').value = '';
    } catch (err) {
      App.Toast.error('Failed: ' + err.message);
    }
  });

  // Theme toggle
  const themeToggle = document.getElementById('theme-toggle');
  const savedTheme = localStorage.getItem('vpsctl-theme');
  if (savedTheme === 'light') {
    themeToggle.checked = false;
    document.documentElement.setAttribute('data-theme', 'light');
  }

  themeToggle.addEventListener('change', () => {
    if (themeToggle.checked) {
      localStorage.setItem('vpsctl-theme', 'dark');
      document.documentElement.removeAttribute('data-theme');
    } else {
      localStorage.setItem('vpsctl-theme', 'light');
      document.documentElement.setAttribute('data-theme', 'light');
    }
  });

  // Load TOTP info
  async function loadTOTP() {
    const sec = document.getElementById('totp-section');
    try {
      const data = await App.api.get('/auth/totp');
      if (data.enabled) {
        sec.innerHTML = `
          <div class="flex items-center gap-12 mb-12">
            <span class="badge badge-success">Enabled</span>
            <span class="text-secondary text-sm">Two-factor authentication is active</span>
          </div>
          <button class="btn btn-danger btn-sm" id="totp-disable">Disable 2FA</button>
        `;
        document.getElementById('totp-disable').addEventListener('click', async () => {
          const ok = await App.confirm('Disable 2FA', 'Are you sure you want to disable two-factor authentication?', 'Disable');
          if (!ok) return;
          try {
            await App.api.post('/auth/totp/disable');
            App.Toast.success('2FA disabled');
            loadTOTP();
          } catch (err) {
            App.Toast.error('Failed: ' + err.message);
          }
        });
      } else {
        sec.innerHTML = `
          <p class="text-secondary mb-12">Set up two-factor authentication using an authenticator app.</p>
          <button class="btn btn-primary btn-sm" id="totp-setup">Setup 2FA</button>
          <div id="totp-setup-area"></div>
        `;
        document.getElementById('totp-setup').addEventListener('click', async () => {
          try {
            const data = await App.api.post('/auth/totp/setup');
            const area = document.getElementById('totp-setup-area');
            area.innerHTML = `
              <div class="mt-16">
                <div class="totp-qr">
                  ${data.qr_code ? `<img src="data:image/png;base64,${data.qr_code}" alt="QR Code">` : ''}
                  ${data.secret ? `<div class="totp-secret">${data.secret}</div>` : ''}
                </div>
                <p class="text-sm text-secondary mb-8">Scan the QR code with your authenticator app, then enter the 6-digit code below:</p>
                <form id="totp-verify-form">
                  <div class="form-group">
                    <input type="text" id="totp-verify-code" class="form-input form-input-mono" placeholder="000000" maxlength="6" pattern="[0-9]{6}" required>
                  </div>
                  <button type="submit" class="btn btn-success btn-sm">Verify & Enable</button>
                </form>
              </div>
            `;
            document.getElementById('totp-verify-form').addEventListener('submit', async (e) => {
              e.preventDefault();
              const code = document.getElementById('totp-verify-code').value.trim();
              try {
                await App.api.post('/auth/totp/verify', { code });
                App.Toast.success('2FA enabled successfully');
                loadTOTP();
              } catch (err) {
                App.Toast.error('Verification failed: ' + err.message);
              }
            });
          } catch (err) {
            App.Toast.error('Failed to start setup: ' + err.message);
          }
        });
      }
    } catch {
      sec.innerHTML = '<p class="text-muted">Unable to load 2FA settings</p>';
    }
  }

  // Load system info
  async function loadSysInfo() {
    const sec = document.getElementById('sysinfo-section');
    try {
      const data = await App.api.get('/system/info');
      sec.innerHTML = `
        <table style="width:100%">
          <tbody>
            ${Object.entries(data).map(([k, v]) => `
              <tr>
                <td style="padding:6px 0;color:var(--text-secondary);width:140px">${App.escapeHtml(k)}</td>
                <td class="mono text-sm" style="padding:6px 0">${App.escapeHtml(String(v))}</td>
              </tr>
            `).join('')}
          </tbody>
        </table>
      `;
    } catch {
      sec.innerHTML = '<p class="text-muted">Unable to load system info</p>';
    }
  }

  // Load sessions
  async function loadSessions() {
    const sec = document.getElementById('sessions-section');
    try {
      const data = await App.api.get('/auth/sessions');
      const sessions = data.sessions || data || [];
      if (!sessions.length) {
        sec.innerHTML = '<p class="text-muted">No active sessions</p>';
        return;
      }
      sec.innerHTML = `
        <ul class="session-list">
          ${sessions.map(s => `
            <li class="session-item">
              <div>
                <span class="session-ip">${App.escapeHtml(s.ip || s.address || '-')}</span>
                <span class="text-muted text-sm" style="margin-left:8px">${App.escapeHtml(s.user_agent || s.agent || '')}</span>
              </div>
              <span class="session-time">${App.formatDate(s.created || s.last_active)}</span>
            </li>
          `).join('')}
        </ul>
      `;
    } catch {
      sec.innerHTML = '<p class="text-muted">Unable to load sessions</p>';
    }
  }

  loadTOTP();
  loadSysInfo();
  loadSessions();

  return function cleanup() {};
}
