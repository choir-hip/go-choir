// The sandboxed frame for app UIs from other publishers (MA6.1; modular
// decision 7; docs/design/modular-architecture-plan-2026-11-04.md section 12).
//
// A package whose release is not on the owner's own track (the computer says
// so: GET /api/packages/installed `isolation`, anything but "shell" is a
// frame) never runs in the shell's realm. Its verified module bytes go into an
// iframe with an opaque origin:
//
//   sandbox="allow-scripts allow-forms"   no allow-same-origin, so the frame
//       cannot read the shell's DOM, storage, cookies or session; no
//       allow-top-navigation, allow-popups or allow-modals;
//   a meta CSP in the srcdoc: default-src 'none', connect-src only the hosts
//       of the package's ui.net.egress grants, no form targets, no frames;
//   one MessagePort, handed over once at load: the shell never reads window
//       messages, so another frame cannot speak for this one.
//
// The SDK in the frame is stubs (frame-runtime.js). Every call that crosses is
// checked here against the package's ceiling: the name must be one the bridge
// offers (SANDBOX_BRIDGED) and one the computer's build extracted from the
// package's source (`uses`); calls act on this window and this package only.
// fetchWithRenewal is answered by the shell's own fetch with the session,
// which never crosses: only routes under /api/app-packages/<package>/ (the
// package's own kinds; the X-Choir-App identity is set here, so the frame
// cannot claim another app's), and only the response body comes back.
//
// Failure modes, written before the code:
//  F1 the frame reads the shell's DOM, cookies, storage or session;
//  F2 the frame calls a route outside its ceiling, or names another app;
//  F3 the frame calls an SDK name outside the bridge or outside its uses;
//  F4 the frame acts on another window (setAppContext, windowAction);
//  F5 a module that needs names the frame does not offer half-mounts
//     (refused before the frame is made, naming them);
//  F6 the frame never answers (a hang, a throw at mount): failed, not loading;
//  F7 the frame navigates itself away (an exfiltration path the CSP cannot
//     close): the shell stops the app at the second load;
//  F8 a destroyed frame's late reply or subscription reaches the shell.
import { getSetting, onSetting, setSetting } from '../services/settings';
import { getTheme, onThemeChange } from '../services/theme';
import { themeCSSVariables } from '../theme';
import { SDK_VERSION } from '../sdk-version';
import { BUILD_INFO } from '../build-info.js';
import { fetchWithRenewal } from '../auth.js';
import { copyText, setAppContext, toast, windowAction } from '../shell-services';
import { focusWindow, restoreWindow } from '../stores/desktop.js';
import { notifyPackagesChanged } from './packages';
import { APP_HEADER } from '../app-identity.js';
import FRAME_RUNTIME from './frame-runtime.js?raw';

/** SDK names the frame runtime answers itself: no authority, nothing crosses. */
export const SANDBOX_LOCAL = [
  'AuthRequiredError', 'SDK_VERSION', 'buildInfo', 'getLaunchFile', 'getTheme', 'launchFileFromAppContext',
  'launchFileStore', 'onDocumentEvent', 'onSetting', 'onThemeChange', 'onWindowEvent', 'repeatEvery', 'sdkSatisfies',
] as const;

const MAX_TEXT = 4096;
const MAX_BODY = 8 * 1024 * 1024;
const MOUNT_MS = 20_000;
const REQUEST_HEADERS = new Set(['accept', 'content-type']);
const METHODS = new Set(['GET', 'HEAD', 'POST', 'PUT', 'PATCH', 'DELETE']);
const EGRESS_HOST = /^[a-z0-9][a-z0-9.-]{0,252}(:[0-9]{1,5})?$/;

type Ceiling = { package: string; windowId: string; uses: Set<string>; egress: string[] };
type Bridged = (c: Ceiling, args: any[]) => unknown;

class Refused extends Error {
  constructor(message: string) {
    super(message);
    this.name = 'SandboxRefused';
  }
}

const text = (v: unknown, max = MAX_TEXT) => String(v ?? '').slice(0, max);

/** The JSON-safe copy of a value that crosses into the frame (props, contexts). */
function plain(v: unknown): unknown {
  if (v === undefined) return undefined;
  try {
    return JSON.parse(JSON.stringify(v));
  } catch {
    return null;
  }
}

/** The package's own route, and only it: /api/app-packages/<name>[/...]. */
export function sandboxRouteAllowed(pkg: string, input: string, origin: string): string {
  let url: URL;
  try {
    url = new URL(String(input), origin);
  } catch {
    throw new Refused('not a URL');
  }
  if (url.origin !== origin) throw new Refused(`${url.origin} is not this computer; a framed app reaches other hosts only by an egress grant, from the frame`);
  const raw = url.pathname;
  // Encoded separators and dots would let a path name another package to the server.
  if (/%(2f|5c|2e|00)/i.test(raw) || raw.includes('//') || raw.includes('\\')) throw new Refused(`${raw} is not a plain path`);
  const own = `/api/app-packages/${pkg}`;
  if (raw !== own && !raw.startsWith(`${own}/`)) throw new Refused(`${raw} is outside ${pkg}'s routes (${own}/...)`);
  return `${raw}${url.search}`;
}

async function proxyFetch(c: Ceiling, [input, init]: any[]): Promise<unknown> {
  const path = sandboxRouteAllowed(c.package, text(input, 2048), location.origin);
  const method = String(init?.method || 'GET').toUpperCase();
  if (!METHODS.has(method)) throw new Refused(`method ${method} is not offered`);
  const headers = new Headers();
  for (const [k, v] of Object.entries(init?.headers && typeof init.headers === 'object' ? init.headers : {})) {
    if (REQUEST_HEADERS.has(k.toLowerCase())) headers.set(k, text(v, 256));
  }
  // The identity is the shell's to set: the frame cannot claim another app.
  headers.set(APP_HEADER, c.package);
  let body: BodyInit | undefined;
  const b = init?.body;
  if (b != null && method !== 'GET' && method !== 'HEAD') {
    if (typeof b === 'string') body = b.slice(0, MAX_BODY);
    else if (b instanceof ArrayBuffer && b.byteLength <= MAX_BODY) body = b;
    else throw new Refused('the body must be text or bytes under 8 MiB');
  }
  const res = await fetchWithRenewal(path, { method, headers, body, credentials: 'same-origin', redirect: 'error', cache: 'no-store' });
  const bytes = method === 'HEAD' ? new ArrayBuffer(0) : await res.arrayBuffer();
  if (bytes.byteLength > MAX_BODY) throw new Refused('the response is over 8 MiB');
  const out: Record<string, string> = {};
  const type = res.headers.get('content-type');
  if (type) out['content-type'] = type;
  return { status: res.status, statusText: res.statusText, headers: out, body: bytes };
}

/** The names the bridge answers, each bound to this window and this package. */
export const SANDBOX_BRIDGED: Record<string, Bridged> = {
  fetchWithRenewal: proxyFetch,
  toast: (_c, [message, options]) => toast(text(message, 300), { kind: options?.kind === 'error' ? 'error' : 'info' }),
  copyText: (_c, [value, label]) => copyText(text(value, 1 << 20), text(label || 'to the clipboard', 80)),
  // F4: the frame's own window, whatever id it names.
  setAppContext: (c, [change]) => setAppContext({
    windowId: c.windowId,
    appContext: change?.appContext && typeof change.appContext === 'object' ? (plain(change.appContext) as Record<string, unknown>) : undefined,
    title: change?.title === undefined ? undefined : text(change.title, 200),
  }),
  windowAction: (c, [, action]) => windowAction(c.windowId, action),
  focus: (c) => focusWindow(c.windowId),
  restore: (c) => restoreWindow(c.windowId),
  getSetting: (c, [key]) => getSetting(text(key, 128), c.package),
  setSetting: (c, [key, value]) => setSetting(text(key, 128), plain(value), c.package),
  notifyPackagesChanged: () => notifyPackagesChanged(),
};

/** Every SDK name a framed package may import. */
export const SANDBOX_SDK: ReadonlySet<string> = new Set([...SANDBOX_LOCAL, ...Object.keys(SANDBOX_BRIDGED)]);

/** The names a framed module uses that the frame does not offer (F5). */
export function sandboxMissing(uses: readonly string[]): string[] {
  return uses.filter((n) => !SANDBOX_SDK.has(n)).sort();
}

/** The hosts of the package's ui.net.egress grants, for the frame's connect-src. */
export function egressHosts(manifest: any): string[] {
  const caps = Array.isArray(manifest?.capabilities) ? manifest.capabilities : [];
  const hosts = caps
    .filter((c: any) => c && c.grant === 'ui.net.egress' && Array.isArray(c.args))
    .flatMap((c: any) => c.args.map((a: unknown) => String(a).toLowerCase()))
    .filter((h: string) => EGRESS_HOST.test(h));
  return [...new Set<string>(hosts)].sort();
}

/** The frame's document: the CSP, base styles and the runtime, nothing of the shell. */
export function sandboxDocument(egress: string[] = []): string {
  const connect = egress.length ? egress.map((h) => `https://${h}`).join(' ') : "'none'";
  const csp = [
    "default-src 'none'",
    "script-src 'unsafe-inline' blob:",
    "style-src 'unsafe-inline'",
    'img-src data: blob:',
    'media-src data: blob:',
    'font-src data:',
    `connect-src ${connect}`,
    "form-action 'none'",
    "base-uri 'none'",
    "frame-src 'none'",
    "worker-src 'none'",
    "object-src 'none'",
    "manifest-src 'none'",
  ].join('; ');
  return `<!doctype html><html><head><meta charset="utf-8">`
    + `<meta http-equiv="Content-Security-Policy" content="${csp}">`
    + `<meta name="referrer" content="no-referrer">`
    + `<style>html,body{margin:0;height:100%;background:transparent;color:var(--choir-fg,inherit)}#choir-app-root{display:flex;flex-direction:column;min-height:100%;height:100%}</style>`
    + `<script>${FRAME_RUNTIME}</script></head><body><div id="choir-app-root"></div></body></html>`;
}

export type SandboxOptions = {
  pkg: string;
  release: string;
  windowId: string;
  manifest: any;
  uses: readonly string[];
  code: string;
  props: Record<string, unknown>;
  launchContext?: { subscribe(run: (v: unknown) => void): () => void } | null;
  onState(state: 'mounted' | 'failed', message?: string): void;
};

export type SandboxHandle = { frame: HTMLIFrameElement; update(props: Record<string, unknown>): void; destroy(): void };

/** The props a framed app receives: its window's, never the signed-in person's identity. */
export function sandboxProps(props: Record<string, unknown>): Record<string, unknown> {
  return {
    windowId: props.windowId,
    appContext: plain(props.appContext),
    authenticated: Boolean(props.authenticated),
    currentTheme: plain(props.currentTheme),
  };
}

function themeMessage() {
  const theme = getTheme();
  const font = typeof document !== 'undefined' ? getComputedStyle(document.body).fontFamily : '';
  const scheme = typeof document !== 'undefined' ? getComputedStyle(document.documentElement).colorScheme : '';
  return { theme: plain(theme), vars: themeCSSVariables(theme), font, scheme };
}

/** Mounts a verified module into a fresh sandboxed frame inside `container`. */
export function mountSandboxed(container: HTMLElement, opts: SandboxOptions): SandboxHandle {
  const ceiling: Ceiling = { package: opts.pkg, windowId: opts.windowId, uses: new Set(opts.uses), egress: egressHosts(opts.manifest) };
  const bridged = Object.keys(SANDBOX_BRIDGED).filter((n) => ceiling.uses.has(n));
  const frame = document.createElement('iframe');
  frame.setAttribute('sandbox', 'allow-scripts allow-forms');
  frame.setAttribute('referrerpolicy', 'no-referrer');
  frame.setAttribute('allow', '');
  frame.setAttribute('title', String(opts.manifest?.display?.title || opts.pkg));
  frame.setAttribute('data-package-sandbox', opts.pkg);
  frame.className = 'package-sandbox';
  frame.srcdoc = sandboxDocument(ceiling.egress);

  const channel = new MessageChannel();
  const port = channel.port1;
  const disposers: (() => void)[] = [];
  const subs = new Map<number, () => void>();
  let loads = 0;
  let initSent = false;
  let done = false;
  let settled = false;
  const settle = (state: 'mounted' | 'failed', message?: string) => {
    if (settled || done) return;
    settled = true;
    clearTimeout(timer);
    opts.onState(state, message);
  };
  const timer = setTimeout(() => settle('failed', `${opts.pkg} did not start in its frame within ${MOUNT_MS / 1000}s`), MOUNT_MS);
  const send = (msg: unknown, transfer: Transferable[] = []) => {
    if (!done) port.postMessage(msg, transfer);
  };

  port.onmessage = async (e: MessageEvent) => {
    if (done) return;
    const msg = e.data || {};
    switch (msg.t) {
      case 'ready':
        if (initSent) return;
        initSent = true;
        send({
          t: 'init',
          code: opts.code,
          ctx: {
            package: opts.pkg,
            release: opts.release,
            windowId: opts.windowId,
            props: sandboxProps(opts.props),
            launch: launchValue,
            theme: themeMessage(),
            sdkVersion: SDK_VERSION,
            build: { ...BUILD_INFO },
            bridged,
          },
        });
        return;
      case 'mounted':
        settle('mounted');
        return;
      case 'failed':
        settle('failed', `${opts.pkg} failed in its frame: ${text(msg.message, 500)}`);
        return;
      case 'activate':
        focusWindow(opts.windowId);
        return;
      case 'call': {
        const id = Number(msg.id);
        const name = String(msg.name || '');
        const fn = Object.prototype.hasOwnProperty.call(SANDBOX_BRIDGED, name) ? SANDBOX_BRIDGED[name] : null;
        try {
          // F3: a name outside the bridge, or outside what the build saw this package use.
          if (!fn) throw new Refused(`${name} is not offered to a framed app`);
          if (!ceiling.uses.has(name)) throw new Refused(`${opts.pkg} does not use ${name}`);
          const value = await fn(ceiling, Array.isArray(msg.args) ? msg.args : []);
          const transfer = value && typeof value === 'object' && (value as any).body instanceof ArrayBuffer ? [(value as any).body] : [];
          send({ t: 'reply', id, ok: true, value: value === undefined ? undefined : value }, transfer);
        } catch (err: any) {
          send({ t: 'reply', id, ok: false, error: { name: String(err?.name || 'Error'), message: text(err?.message || err, 500) } });
        }
        return;
      }
      case 'sub': {
        const sub = Number(msg.sub);
        if (msg.name !== 'onSetting' || !ceiling.uses.has('onSetting') || subs.has(sub) || subs.size >= 64) return;
        const key = text(Array.isArray(msg.args) ? msg.args[0] : '', 128);
        subs.set(sub, onSetting(key, (value) => send({ t: 'event', sub, value: plain(value) }), ceiling.package));
        return;
      }
      case 'unsub': {
        const sub = Number(msg.sub);
        subs.get(sub)?.();
        subs.delete(sub);
        return;
      }
      default:
    }
  };

  let launchValue: unknown = null;
  if (opts.launchContext && typeof opts.launchContext.subscribe === 'function') {
    let first = true;
    disposers.push(opts.launchContext.subscribe((v) => {
      launchValue = plain(v) ?? null;
      if (!first) send({ t: 'launch', value: launchValue });
      first = false;
    }));
  }
  {
    let first = true;
    disposers.push(onThemeChange(() => {
      if (!first) send({ t: 'theme', ...themeMessage() });
      first = false;
    }));
  }

  frame.addEventListener('load', () => {
    loads += 1;
    if (loads === 1) {
      // The port goes over once; '*' because the frame's origin is opaque.
      frame.contentWindow?.postMessage({ t: 'choir-frame-port' }, '*', [channel.port2]);
      return;
    }
    // F7: a framed app that navigated away is no longer the verified module.
    handle.destroy();
    opts.onState('failed', `${opts.pkg} navigated its frame away and was stopped`);
  });
  container.appendChild(frame);

  const handle: SandboxHandle = {
    frame,
    update(props) {
      send({ t: 'props', props: sandboxProps(props) });
    },
    destroy() {
      if (done) return;
      send({ t: 'destroy' });
      done = true;
      clearTimeout(timer);
      for (const d of disposers.splice(0)) d();
      for (const d of subs.values()) d();
      subs.clear();
      port.onmessage = null;
      port.close();
      frame.remove();
    },
  };
  return handle;
}
