import { withDesktopSelector } from './desktop-selector.js';

export const SURFACE_HANDOFF_STORAGE_KEY = 'choir.surface.asset-graph';

/**
 * Fingerprint the Vite asset graph named by an HTML document.
 * Host platform shell and guest computer surface are the same SPA source
 * compiled into different hashed /assets/* files. Mixing them 404s lazy
 * app chunks and shows AppHost "Reload app".
 */
export function assetGraphFromHTML(html) {
  if (typeof html !== 'string' || html.length === 0) return '';
  const urls = new Set();
  const re = /(?:src|href)="(\/assets\/[^"]+)"/g;
  let match;
  while ((match = re.exec(html))) {
    urls.add(match[1]);
  }
  return [...urls].sort().join('\n');
}

export function currentDocumentAssetGraph() {
  if (typeof document === 'undefined') return '';
  const urls = new Set();
  const nodes = document.querySelectorAll('script[src*="/assets/"], link[href*="/assets/"]');
  for (const el of nodes) {
    const raw = el.getAttribute('src') || el.getAttribute('href') || '';
    try {
      const path = new URL(raw, window.location.origin).pathname;
      if (path.startsWith('/assets/')) urls.add(path);
    } catch (_err) {
      // Ignore malformed asset URLs; the computer HTML fetch is authoritative.
    }
  }
  return [...urls].sort().join('\n');
}

export function shouldReplaceDocument(currentGraph, computerGraph, alreadyHandedOffGraph) {
  if (!computerGraph || currentGraph === computerGraph) return false;
  if (alreadyHandedOffGraph === computerGraph) return false;
  return true;
}

function readHandedOffGraph() {
  try {
    return sessionStorage.getItem(SURFACE_HANDOFF_STORAGE_KEY) || '';
  } catch (_err) {
    return '';
  }
}

function writeHandedOffGraph(graph) {
  try {
    sessionStorage.setItem(SURFACE_HANDOFF_STORAGE_KEY, graph);
  } catch (_err) {
    // sessionStorage may be unavailable; loop prevention is best-effort.
  }
}

function isHTMLContentType(value) {
  const type = String(value || '').toLowerCase();
  return type.includes('text/html') || type.includes('application/xhtml');
}

/**
 * If the executing document is the host platform shell (or a previous guest
 * build) and the authenticated computer now serves a different asset graph,
 * perform one C15/I25 document handoff.
 *
 * @returns {Promise<boolean>} true when navigation was started
 */
export async function handoffToComputerSurfaceIfStale() {
  if (typeof window === 'undefined' || typeof document === 'undefined') return false;
  let res;
  try {
    res = await fetch(withDesktopSelector('/'), {
      method: 'GET',
      credentials: 'include',
      cache: 'no-store',
      headers: { Accept: 'text/html' },
    });
  } catch (_err) {
    return false;
  }
  if (!res.ok || !isHTMLContentType(res.headers.get('content-type'))) {
    return false;
  }
  const computerGraph = assetGraphFromHTML(await res.text());
  const currentGraph = currentDocumentAssetGraph();
  if (!shouldReplaceDocument(currentGraph, computerGraph, readHandedOffGraph())) {
    if (computerGraph) writeHandedOffGraph(computerGraph);
    return false;
  }
  writeHandedOffGraph(computerGraph);
  window.location.replace(window.location.href);
  return true;
}

export function isDynamicImportError(err) {
  if (!err) return false;
  const msg = err instanceof Error ? err.message : String(err);
  return (
    /failed to fetch dynamically imported module/i.test(msg) ||
    /error loading dynamically imported module/i.test(msg) ||
    /unable to preload/i.test(msg) ||
    /failed to load module script/i.test(msg) ||
    /importing a module script failed/i.test(msg) ||
    /error resolving module specifier/i.test(msg)
  );
}
