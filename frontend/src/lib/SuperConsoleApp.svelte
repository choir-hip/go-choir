<!--
  SuperConsoleApp — singleton zot repair console rendered inside a floating window.

  Initializes ghostty-web WASM (once at module level), creates a Terminal instance
  with FitAddon, connects to /api/super-console/ws WebSocket, and renders in the
  parent container provided by FloatingWindow.

  Features:
    - Theme-native terminal colors
    - Cursor blink enabled
    - 10000 line scrollback
    - Copy/paste via keyboard shortcuts (Cmd+C/V on macOS, Ctrl+Shift+C/V on Linux)
    - Responsive fit within window using FitAddon + ResizeObserver
    - Independent PTY sessions per window

  Props:
    windowId — unique window identifier for session management

  Data attributes for test targeting:
    data-super-console          — root container div
    data-super-console-canvas   — the canvas element (set by ghostty-web)
    data-super-console-error    — error message container
-->
<script>
  import { onMount, onDestroy } from 'svelte';
  import { withDesktopSelector } from './desktop-selector.js';
  import {
    createTerminalSession,
    updateTerminalSession,
    getTerminalSession,
    destroyTerminalSession,
    terminalSessions,
  } from './stores/terminal.js';

  export let windowId = '';
  export let authenticated = false;

  // Reactive access to this session's error state
  $: session = $terminalSessions[windowId] || {};
  $: errorMessage = session.error;

  // ---- DOM refs ----
  let terminalContainer;

  // ---- ResizeObserver for responsive fit ----
  let resizeObserver = null;

  // ---- WASM init promise (module-level, initialized once) ----
  let wasmInitPromise = null;

  /**
   * Initialize ghostty-web WASM exactly once.
   * The init() function loads the ghostty-vt.wasm file.
   * We cache the promise so concurrent terminal windows don't re-init.
   */
  async function ensureWasmInit() {
    if (!wasmInitPromise) {
      const ghosttyWeb = await import('ghostty-web');
      wasmInitPromise = ghosttyWeb.init();
    }
    return wasmInitPromise;
  }

  /**
   * Detect macOS platform for keyboard shortcut handling.
   */
  function isMac() {
    return navigator.platform.toUpperCase().indexOf('MAC') >= 0 ||
           navigator.userAgent.toUpperCase().indexOf('MAC') >= 0;
  }

  function themeColor(name, fallback = 'CanvasText') {
    const value = getComputedStyle(document.documentElement).getPropertyValue(name).trim();
    return value || fallback;
  }

  function parseHexColor(value) {
    const normalized = value.trim();
    const match = normalized.match(/^#([0-9a-f]{3}|[0-9a-f]{6})$/i);
    if (!match) return null;
    const hex = match[1].length === 3
      ? match[1].split('').map((part) => part + part).join('')
      : match[1];
    return {
      r: parseInt(hex.slice(0, 2), 16) / 255,
      g: parseInt(hex.slice(2, 4), 16) / 255,
      b: parseInt(hex.slice(4, 6), 16) / 255,
    };
  }

  function linearize(channel) {
    return channel <= 0.03928 ? channel / 12.92 : ((channel + 0.055) / 1.055) ** 2.4;
  }

  function relativeLuminance(color) {
    return 0.2126 * linearize(color.r) + 0.7152 * linearize(color.g) + 0.0722 * linearize(color.b);
  }

  function contrastRatio(foreground, background) {
    const fg = parseHexColor(foreground);
    const bg = parseHexColor(background);
    if (!fg || !bg) return 21;
    const l1 = relativeLuminance(fg);
    const l2 = relativeLuminance(bg);
    const light = Math.max(l1, l2);
    const dark = Math.min(l1, l2);
    return (light + 0.05) / (dark + 0.05);
  }

  function currentTerminalTheme() {
    const foreground = themeColor('--choir-text-primary');
    const mediaBackground = themeColor('--choir-surface-media', 'Canvas');
    const pageBackground = themeColor('--choir-bg', themeColor('--choir-surface-app', 'Canvas'));
    const background = contrastRatio(foreground, mediaBackground) >= 4.5
      ? mediaBackground
      : pageBackground;

    if (terminalContainer) {
      terminalContainer.style.setProperty('--super-console-bg', background);
    }

    return {
      background,
      foreground,
      cursor: themeColor('--choir-text-accent'),
      cursorAccent: background,
      selectionBackground: themeColor('--choir-state-selected'),
      selectionForeground: foreground,
      black: background,
      red: themeColor('--choir-status-danger'),
      green: themeColor('--choir-status-success'),
      yellow: themeColor('--choir-status-warning'),
      blue: themeColor('--choir-chart-1'),
      magenta: themeColor('--choir-chart-4'),
      cyan: themeColor('--choir-chart-2'),
      white: themeColor('--choir-text-muted'),
      brightBlack: themeColor('--choir-text-subtle'),
      brightRed: themeColor('--choir-status-danger'),
      brightGreen: themeColor('--choir-status-success'),
      brightYellow: themeColor('--choir-status-warning'),
      brightBlue: themeColor('--choir-chart-1'),
      brightMagenta: themeColor('--choir-chart-4'),
      brightCyan: themeColor('--choir-chart-2'),
      brightWhite: foreground,
    };
  }

  /**
   * Initialize the terminal: create session, init WASM, create Terminal,
   * connect WebSocket, attach FitAddon + ResizeObserver.
   */
  async function initTerminal() {
    if (!terminalContainer) return;

    // Create session record
    createTerminalSession(windowId);

    try {
      // Initialize WASM (once globally)
      await ensureWasmInit();

      // Dynamic import for Terminal and FitAddon
      const ghosttyWeb = await import('ghostty-web');
      const { Terminal, FitAddon } = ghosttyWeb;

      // Create Terminal instance using the active desktop theme.
      const term = new Terminal({
        fontSize: 14,
        fontFamily: "'Menlo', 'Monaco', 'Courier New', monospace",
        cursorBlink: true,
        cursorStyle: 'block',
        scrollback: 10000,
        convertEol: false,
        theme: currentTerminalTheme(),
      });

      // Create FitAddon
      const fitAddon = new FitAddon();
      term.loadAddon(fitAddon);

      // Open terminal in container
      term.open(terminalContainer);

      // Add data-test attribute to the canvas element
      const canvas = terminalContainer.querySelector('canvas');
      if (canvas) {
        canvas.setAttribute('data-super-console-canvas', '');
      }

      // Fit to container
      fitAddon.fit();

      // Connect WebSocket to PTY backend
      const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
      const wsUrl = withDesktopSelector(`${protocol}//${window.location.host}/api/super-console/ws`);
      const ws = new WebSocket(wsUrl);

      // Protocol uses text-based JSON messages (not binary).
      // Do NOT set ws.binaryType = 'arraybuffer'.

      ws.onopen = () => {
        // Send initial resize so backend PTY knows our size
        ws.send(JSON.stringify({
          type: 'resize',
          cols: term.cols,
          rows: term.rows,
        }));
      };

      // PTY output -> terminal write
      // Backend sends JSON: {type: "output", data: "..."} or {type: "error", data: "..."}
      ws.onmessage = (event) => {
        try {
          const msg = JSON.parse(event.data);
          if (msg.type === 'output' && typeof msg.data === 'string') {
            term.write(msg.data);
          } else if (msg.type === 'error') {
            updateTerminalSession(windowId, {
              error: msg.data || 'Unknown server error',
            });
          }
        } catch (_e) {
          // Fallback: if message is not JSON, write raw text
          term.write(event.data);
        }
      };

      ws.onerror = () => {
        updateTerminalSession(windowId, {
          error: 'WebSocket connection error',
        });
      };

      ws.onclose = (event) => {
        if (event.code !== 1000) {
          updateTerminalSession(windowId, {
            error: `WebSocket closed (code ${event.code})`,
          });
        }
      };

      // Terminal input -> WebSocket send (JSON protocol)
      term.onData((data) => {
        if (ws.readyState === WebSocket.OPEN) {
          ws.send(JSON.stringify({
            type: 'input',
            data: data,
          }));
        }
      });

      // On resize, inform the backend PTY
      term.onResize(({ cols, rows }) => {
        if (ws.readyState === WebSocket.OPEN) {
          ws.send(JSON.stringify({
            type: 'resize',
            cols,
            rows,
          }));
        }
      });

      // Enable automatic fitting on container resize
      fitAddon.observeResize();

      // Store references in session
      updateTerminalSession(windowId, {
        term,
        fitAddon,
        ws,
        initialized: true,
        error: null,
      });

    } catch (err) {
      console.error('[SuperConsoleApp] Failed to initialize:', err);
      updateTerminalSession(windowId, {
        error: `Initialization failed: ${err.message}`,
      });
    }
  }

  /**
   * Clean up terminal session on component destroy.
   */
  function cleanup() {
    // Disconnect ResizeObserver
    if (resizeObserver) {
      resizeObserver.disconnect();
      resizeObserver = null;
    }
    // Destroy session (disposes terminal, closes WebSocket)
    destroyTerminalSession(windowId);
  }

  onMount(() => {
    if (!authenticated) return;
    initTerminal();
  });

  onDestroy(() => {
    if (!authenticated) return;
    cleanup();
  });
</script>

<div
  class="terminal-wrapper"
  bind:this={terminalContainer}
  data-super-console
>
  {#if !authenticated}
    <div class="terminal-preview" data-super-console-preview>
      <p class="terminal-kicker">Super Console preview</p>
      <h2>zot repair requires sign-in</h2>
      <p>
        This window opens in logged-out review so every app is visible. A real zot session can inspect or mutate private computer state, so connecting asks for auth.
      </p>
      <pre>$ choir status
public-preview: ready
$ open apps --preview
files email texture settings podcast media super-console</pre>
    </div>
  {/if}

  <!-- ghostty-web renders its canvas here via term.open() when authenticated -->

  <!-- Error display overlay -->
  {#if errorMessage}
    <div class="terminal-error" data-super-console-error>
      <div class="terminal-error-content">
        <span class="terminal-error-icon">⚠</span>
        <span class="terminal-error-text">{errorMessage}</span>
      </div>
    </div>
  {/if}
</div>

<style>
  .terminal-wrapper {
    width: 100%;
    height: 100%;
    background: var(--super-console-bg, var(--choir-bg));
    overflow: hidden;
    position: relative;
  }

  .terminal-preview {
    display: grid;
    align-content: center;
    gap: 0.75rem;
    min-height: 100%;
    padding: clamp(1rem, 3vw, 2rem);
    color: var(--choir-text-primary);
    background:
      radial-gradient(circle at 20% 10%, color-mix(in srgb, var(--choir-accent) 14%, transparent), transparent 34%),
      var(--choir-surface-app);
  }

  .terminal-kicker,
  .terminal-preview h2,
  .terminal-preview p {
    margin: 0;
  }

  .terminal-kicker {
    color: var(--choir-accent);
    font-size: 0.72rem;
    font-weight: 850;
    text-transform: uppercase;
  }

  .terminal-preview pre {
    margin: 0;
    overflow: auto;
    border-radius: var(--choir-radius-control, 20px);
    background: color-mix(in srgb, var(--choir-bg) 82%, transparent);
    box-shadow: var(--choir-control-shadow);
    padding: 1rem;
    color: var(--choir-text-muted);
  }

  /* Ensure ghostty-web canvas fills the container */
  .terminal-wrapper :global(canvas) {
    display: block;
  }

  /* Error state overlay */
  .terminal-error {
    position: absolute;
    top: 0;
    left: 0;
    right: 0;
    bottom: 0;
    display: flex;
    align-items: center;
    justify-content: center;
    background: var(--choir-state-selected);
    color: var(--choir-status-danger);
    font-family: monospace;
    font-size: 0.85rem;
    padding: 1rem;
    text-align: center;
    z-index: 10;
  }

  .terminal-error-content {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 0.5rem;
    max-width: 320px;
  }

  .terminal-error-icon {
    font-size: 1.5rem;
  }

  .terminal-error-text {
    word-break: break-word;
  }
</style>
