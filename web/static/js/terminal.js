function TerminalPage(container) {
  let term = null;
  let ws = null;
  let fitAddon = null;
  let reconnectTimer = null;
  let disposed = false;

  container.innerHTML = `
    <div class="page-header">
      <h2>Terminal</h2>
      <div class="actions">
        <div class="terminal-status" id="term-status">
          <span class="dot disconnected" id="term-dot"></span>
          <span id="term-status-text">Disconnected</span>
        </div>
        <button class="btn btn-ghost btn-sm" id="term-reconnect">Reconnect</button>
      </div>
    </div>
    <div class="page-content">
      <div id="terminal-container"></div>
    </div>
  `;

  const termContainer = document.getElementById('terminal-container');
  const termDot = document.getElementById('term-dot');
  const termStatusText = document.getElementById('term-status-text');
  const reconnectBtn = document.getElementById('term-reconnect');

  function setStatus(status) {
    termDot.className = 'dot ' + status;
    const labels = { connected: 'Connected', disconnected: 'Disconnected', connecting: 'Connecting...' };
    termStatusText.textContent = labels[status] || status;
  }

  function getWsUrl() {
    const proto = location.protocol === 'https:' ? 'wss' : 'ws';
    return `${proto}://${location.host}/api/terminal`;
  }

  function initTerminal() {
    if (disposed) return;
    if (typeof Terminal === 'undefined') {
      termContainer.innerHTML = '<div class="empty-state"><div class="empty-icon">&#9000;</div><h3>xterm.js not loaded</h3><p>Check your internet connection or reload the page.</p></div>';
      return;
    }

    if (term) {
      term.dispose();
    }

    term = new Terminal({
      theme: {
        background: '#0d1117',
        foreground: '#c9d1d9',
        cursor: '#58a6ff',
        cursorAccent: '#0d1117',
        selectionBackground: '#264f78',
        black: '#484f58',
        red: '#f85149',
        green: '#3fb950',
        yellow: '#d29922',
        blue: '#58a6ff',
        magenta: '#bc8cff',
        cyan: '#39c5cf',
        white: '#c9d1d9',
        brightBlack: '#6e7681',
        brightRed: '#f85149',
        brightGreen: '#3fb950',
        brightYellow: '#d29922',
        brightBlue: '#58a6ff',
        brightMagenta: '#bc8cff',
        brightCyan: '#56d4dd',
        brightWhite: '#f0f6fc',
      },
      fontFamily: "'SF Mono', 'Fira Code', 'Cascadia Code', Consolas, monospace",
      fontSize: 14,
      lineHeight: 1.2,
      cursorBlink: true,
      allowProposedApi: true,
    });

    fitAddon = new FitAddon.FitAddon();
    term.loadAddon(fitAddon);
    term.open(termContainer);
    fitAddon.fit();

    term.onData(data => {
      if (ws && ws.readyState === WebSocket.OPEN) {
        ws.send(new TextEncoder().encode(data));
      }
    });

    term.onResize(({ cols, rows }) => {
      if (ws && ws.readyState === WebSocket.OPEN) {
        ws.send(JSON.stringify({ type: 'resize', cols, rows }));
      }
    });

    connect();
  }

  function connect() {
    if (disposed) return;
    setStatus('connecting');
    try {
      ws = new WebSocket(getWsUrl());
      ws.binaryType = 'arraybuffer';
    } catch (e) {
      setStatus('disconnected');
      scheduleReconnect();
      return;
    }

    ws.onopen = () => {
      setStatus('connected');
      if (term && fitAddon) {
        fitAddon.fit();
        ws.send(JSON.stringify({ type: 'resize', cols: term.cols, rows: term.rows }));
      }
    };

    ws.onmessage = (e) => {
      if (disposed) return;
      if (term) {
        if (e.data instanceof ArrayBuffer) {
          term.write(new Uint8Array(e.data));
        } else {
          term.write(e.data);
        }
      }
    };

    ws.onclose = () => {
      setStatus('disconnected');
      scheduleReconnect();
    };

    ws.onerror = () => {
      setStatus('disconnected');
    };
  }

  function scheduleReconnect() {
    if (disposed || reconnectTimer) return;
    reconnectTimer = setTimeout(() => {
      reconnectTimer = null;
      if (!disposed) connect();
    }, 3000);
  }

  function handleResize() {
    if (fitAddon && !disposed) {
      try { fitAddon.fit(); } catch {}
    }
  }

  window.addEventListener('resize', handleResize);

  reconnectBtn.addEventListener('click', () => {
    if (reconnectTimer) {
      clearTimeout(reconnectTimer);
      reconnectTimer = null;
    }
    if (ws) {
      ws.close();
      ws = null;
    }
    connect();
  });

  // Slight delay to ensure container is rendered
  setTimeout(initTerminal, 50);

  return function cleanup() {
    disposed = true;
    window.removeEventListener('resize', handleResize);
    if (reconnectTimer) {
      clearTimeout(reconnectTimer);
      reconnectTimer = null;
    }
    if (ws) {
      ws.close();
      ws = null;
    }
    if (term) {
      term.dispose();
      term = null;
    }
  };
}
