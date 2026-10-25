import { fetchWithRenewal, AuthRequiredError } from './auth.js';
import { currentDeviceId } from './live-events.js';
import { PACKAGE_MEDIA_ROUTES } from './apps/registry';

  // PDF and EPUB are opened by the Reader package (src/packages/reader), whose
  // manifest declares those extensions in `opens`; packages.ts consults
  // PACKAGE_MEDIA_ROUTES first, so mediaRouteForFileName still answers
  // { appId: 'pdf' } and { appId: 'epub' } exactly as this table did.
export const MEDIA_FILE_ROUTES = [
  { appId: 'audio', mediaType: 'audio/mpeg', extensions: ['mp3', 'm4a', 'aac', 'ogg', 'oga', 'wav', 'flac', 'opus'] },
  { appId: 'video', mediaType: 'video/mp4', extensions: ['mp4', 'm4v', 'webm', 'mov', 'avi', 'mkv'] },
  { appId: 'slides', mediaType: 'application/vnd.openxmlformats-officedocument.presentationml.presentation', extensions: ['pptx', 'html', 'htm'] },
  { appId: 'calendar', mediaType: 'text/calendar', extensions: ['ics'] },
];

// The shell's upload, offered to packages through app-sdk.ts (the package
// boundary refuses any other import): one file to top-level Files, by the same
// request Files itself sends, so an upload from a package renews the session,
// reports progress and answers the same way an upload from Files does.
export async function uploadMediaFile(file, options = {}) {
  const { onProgress, signal } = options || {};
  const url = '/api/files/' + encodeURIComponent(String(file?.name || ''));
  const headers = { 'X-Choir-Device': currentDeviceId() };
  if (file?.type) headers['Content-Type'] = file.type;
  const wantsProgress = typeof onProgress === 'function' && typeof XMLHttpRequest !== 'undefined';

  let res;
  try {
    if (wantsProgress) {
      res = await putFileWithProgress(file, url, headers, onProgress, signal);
    } else {
      res = await fetchWithRenewal(url, { method: 'PUT', headers, body: file, signal });
    }
  } catch (err) {
    if (err?.name === 'AuthRequiredError') return { ok: false, path: '', status: 401, error: 'Your session ended. Sign in again to save the file.' };
    if (err?.name === 'AbortError') return { ok: false, path: '', status: 0, error: 'The upload was cancelled.' };
    return { ok: false, path: '', status: 0, error: 'The file did not finish uploading. Check the connection and try again.' };
  }
  if (!res || !res.ok) {
    const status = Number(res?.status) || 0;
    let message = '';
    try {
      const body = await res.json();
      message = String(body?.error || '');
    } catch (_err) {
      message = '';
    }
    if (status === 401) return { ok: false, path: '', status, error: 'Your session ended. Sign in again to save the file.' };
    if (status === 0) return { ok: false, path: '', status, error: 'The file did not finish uploading. Check the connection and try again.' };
    return { ok: false, path: '', status, error: message || 'The library answered ' + status + ' while saving the file.' };
  }
  return { ok: true, path: String(file?.name || ''), status: Number(res.status) || 0, error: '' };
}

// The same PUT, sent by XMLHttpRequest when the caller wants progress: fetch
// cannot report upload progress. Resolves to a Response-shaped value.
function putFileWithProgress(file, url, headers, onProgress, signal) {
  return new Promise((resolve, reject) => {
    const request = new XMLHttpRequest();
    request.open('PUT', url);
    request.withCredentials = true;
    for (const [key, value] of Object.entries(headers)) request.setRequestHeader(key, value);
    request.upload.addEventListener('progress', (event) => {
      if (!event.lengthComputable) return;
      onProgress({ loaded: event.loaded, total: event.total, fraction: event.total ? event.loaded / event.total : 0 });
    });
    request.addEventListener('load', () => {
      resolve({
        ok: request.status >= 200 && request.status < 300,
        status: request.status,
        json: async () => {
          try {
            return JSON.parse(request.responseText || '{}');
          } catch (_err) {
            return {};
          }
        },
      });
    });
    request.addEventListener('error', () => {
      const err = new Error('Upload failed');
      err.name = 'NetworkError';
      reject(err);
    });
    request.addEventListener('abort', () => {
      const err = new Error('Upload cancelled');
      err.name = 'AbortError';
      reject(err);
    });
    if (signal) {
      if (signal.aborted) {
        request.abort();
        return;
      }
      signal.addEventListener('abort', () => request.abort());
    }
    request.send(file);
  });
}

export function appTitle(kind) {
  if (kind === 'pdf') return 'PDF';
  if (kind === 'epub') return 'EPUB';
  return `${String(kind || 'file').slice(0, 1).toUpperCase()}${String(kind || 'file').slice(1)}`;
}

export function apiFileURL(path, inline = true) {
  const encoded = String(path || '').split('/').map(encodeURIComponent).join('/');
  if (!encoded) return '';
  return `/api/files/${encoded}${inline ? '?disposition=inline' : ''}`;
}

export function mediaRouteForFileName(name) {
  const ext = String(name || '').toLowerCase().split('.').pop();
  if (!ext || ext === String(name || '').toLowerCase()) return null;
  const route = [...PACKAGE_MEDIA_ROUTES, ...MEDIA_FILE_ROUTES].find((candidate) => candidate.extensions.includes(ext));
  if (!route) return null;
  return {
    appId: route.appId,
    mediaType: ext === 'svg' ? 'image/svg+xml' : route.mediaType,
  };
}

export function resolveMediaSource(appContext = {}, item = null, fallbackKind = 'file') {
  const sourceUrl = item?.source_url || appContext.sourceUrl || '';
  const filePath = item?.file_path || appContext.filePath || appContext.sourcePath || '';
  const mediaType = item?.media_type || appContext.mediaType || '';
  const contentId = item?.content_id || appContext.contentId || appContext.content_id || '';
  const title =
    item?.title ||
    appContext.windowTitle ||
    appContext.title ||
    appContext.fileName ||
    appTitle(fallbackKind);
  return {
    sourceUrl,
    filePath,
    mediaType,
    contentId,
    title,
    displayUrl: filePath ? apiFileURL(filePath) : sourceUrl,
  };
}

export async function loadContextContentItem(appContext = {}, currentItem = null, fallbackLabel = 'Media') {
  const contentId = appContext.contentId || appContext.content_id || '';
  const sourceUrl = currentItem?.source_url || appContext.sourceUrl || '';
  const shouldImportSource = appContext.importUrl === true || appContext.forceImport === true;
  if (currentItem || (!contentId && (!sourceUrl || !shouldImportSource))) {
    return { item: currentItem, skipped: true };
  }

  try {
    const res = contentId
      ? await fetchWithRenewal(`/api/content/items/${encodeURIComponent(contentId)}`)
      : await fetchWithRenewal('/api/content/import-url', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({
            url: sourceUrl,
            query: appContext.windowTitle || appContext.title || sourceUrl,
          }),
        });
    if (!res.ok) {
      if (res.status === 401) return { authRequired: true };
      const body = await res.json().catch(() => ({}));
      return { error: body.error || `${fallbackLabel} load failed (${res.status})` };
    }
    return { item: await res.json() };
  } catch (err) {
    if (err instanceof AuthRequiredError) return { authRequired: true };
    return { error: `${fallbackLabel} load failed` };
  }
}

export function youtubeEmbedURL(raw) {
  try {
    const url = new URL(raw);
    if (url.hostname === 'youtu.be') {
      const videoId = url.pathname.startsWith('/') ? url.pathname.slice(1) : url.pathname;
      return videoId ? `https://www.youtube.com/embed/${encodeURIComponent(videoId)}` : '';
    }
    const id = url.searchParams.get('v');
    if (id) return `https://www.youtube.com/embed/${encodeURIComponent(id)}`;
  } catch (err) {
    return '';
  }
  return '';
}

export function clampNumber(value, min, max) {
  return Math.min(max, Math.max(min, value));
}

export function formatTime(seconds) {
  if (!Number.isFinite(seconds) || seconds <= 0) return '0:00';
  const total = Math.floor(seconds);
  const minutes = Math.floor(total / 60);
  const remainder = String(total % 60).padStart(2, '0');
  return `${minutes}:${remainder}`;
}

export function mediaSourceIdentity(source = {}) {
  return source.filePath || source.sourceUrl || source.contentId || source.title || '';
}

export async function loadMediaProgress(kind, source = {}) {
  const identity = mediaSourceIdentity(source);
  if (!kind || !identity) return { currentTime: 0, duration: 0, playbackRate: 1 };
  let res;
  try {
    res = await fetchWithRenewal(`/api/media/progress?kind=${encodeURIComponent(kind)}&identity=${encodeURIComponent(identity)}`);
  } catch (err) {
    if (err instanceof AuthRequiredError) return { currentTime: 0, duration: 0, playbackRate: 1 };
    throw err;
  }
  if (!res.ok) {
    return { currentTime: 0, duration: 0, playbackRate: 1 };
  }
  const body = await res.json();
  return {
    currentTime: Number(body.current_time) || 0,
    duration: Number(body.duration) || 0,
    playbackRate: Number(body.playback_rate) || 1,
    updatedByDevice: body.updated_by_device || '',
  };
}

export async function loadMediaPosition(kind, source = {}) {
  const progress = await loadMediaProgress(kind, source);
  return Number.isFinite(progress.currentTime) && progress.currentTime > 0 ? progress.currentTime : 0;
}

// Players report progress on every timeupdate (about four a second while
// playing), and every progress PUT is a tape event on the computer. Send at
// most one PUT per source every MEDIA_PROGRESS_INTERVAL_MS: the first at once,
// then a trailing PUT of the latest position. A pause or the end
// ({ immediate: true }), hiding the page and leaving it send at once, with
// keepalive so the browser finishes the request after unload
// (docs/problems/stock-ui-writes-make-a-computer-checkpoint-ineligible-2026-10-24.md).
const MEDIA_PROGRESS_INTERVAL_MS = 5000;
const pendingMediaProgress = new Map();

function sendMediaProgress(body) {
  const payload = JSON.stringify(body);
  fetchWithRenewal('/api/media/progress', {
    method: 'PUT',
    headers: {
      'Content-Type': 'application/json',
      'X-Choir-Device': currentDeviceId(),
    },
    keepalive: payload.length < 60_000,
    body: payload,
  }).catch(() => {});
}

function flushMediaProgress(key) {
  const entry = pendingMediaProgress.get(key);
  if (!entry) return;
  if (entry.timer) clearTimeout(entry.timer);
  entry.timer = null;
  const body = entry.body;
  entry.body = null;
  if (!body) return;
  entry.lastSentAt = Date.now();
  sendMediaProgress(body);
}

function flushAllMediaProgress() {
  for (const key of pendingMediaProgress.keys()) flushMediaProgress(key);
}

if (typeof window !== 'undefined') {
  window.addEventListener('pagehide', flushAllMediaProgress);
}
if (typeof document !== 'undefined') {
  document.addEventListener('visibilitychange', () => {
    if (document.visibilityState === 'hidden') flushAllMediaProgress();
  });
}

export function saveMediaPosition(kind, source = {}, currentTime = 0, duration = 0, options = {}) {
  if (!Number.isFinite(currentTime) || currentTime < 0) return;
  const identity = mediaSourceIdentity(source);
  if (!kind || !identity) return;
  const key = `${kind}\u0000${identity}`;
  const entry = pendingMediaProgress.get(key) || { body: null, timer: null, lastSentAt: 0 };
  pendingMediaProgress.set(key, entry);
  entry.body = {
    kind,
    identity,
    current_time: currentTime,
    duration: Number.isFinite(duration) ? duration : 0,
    playback_rate: Number(source.playbackRate) || 1,
    updated_by_device: currentDeviceId(),
  };
  if (options.immediate === true) {
    flushMediaProgress(key);
    return;
  }
  if (entry.timer) return;
  const wait = entry.lastSentAt + MEDIA_PROGRESS_INTERVAL_MS - Date.now();
  if (wait <= 0) {
    flushMediaProgress(key);
    return;
  }
  entry.timer = setTimeout(() => flushMediaProgress(key), wait);
}

const MEDIA_RECENT_LIMIT = 10;

function displayFileName(source = {}) {
  const raw = source.filePath || source.sourceUrl || source.title || '';
  try {
    const url = new URL(raw);
    const pathname = decodeURIComponent(url.pathname || '');
    return pathname.split('/').filter(Boolean).pop() || url.hostname || source.title || raw;
  } catch (_err) {
    return String(raw).split('/').filter(Boolean).pop() || source.title || raw;
  }
}

export async function loadRecentMedia(kind) {
  const suffix = kind ? `?kind=${encodeURIComponent(kind)}&limit=${MEDIA_RECENT_LIMIT}` : `?limit=${MEDIA_RECENT_LIMIT}`;
  let res;
  try {
    res = await fetchWithRenewal(`/api/media/recents${suffix}`);
  } catch (err) {
    if (err instanceof AuthRequiredError) return [];
    throw err;
  }
  if (!res.ok) {
    return [];
  }
  const body = await res.json();
  return Array.isArray(body.items) ? body.items.map((entry) => ({
    identity: entry.identity,
    kind: entry.kind,
    title: entry.title,
    fileName: entry.file_name,
    filePath: entry.file_path,
    sourceUrl: entry.source_url,
    mediaType: entry.media_type,
    contentId: entry.content_id,
    openedAt: entry.opened_at,
  })).filter((entry) => entry.identity).slice(0, MEDIA_RECENT_LIMIT) : [];
}

export async function rememberRecentMedia(kind, source = {}) {
  const identity = mediaSourceIdentity(source);
  if (!kind || !identity) return false;
  const entry = {
    identity,
    kind,
    title: source.title || displayFileName(source) || appTitle(kind),
    file_name: displayFileName(source),
    file_path: source.filePath || '',
    source_url: source.sourceUrl || '',
    media_type: source.mediaType || '',
    content_id: source.contentId || '',
  };
  try {
    const res = await fetchWithRenewal('/api/media/recents', {
      method: 'PUT',
      headers: {
        'Content-Type': 'application/json',
        'X-Choir-Device': currentDeviceId(),
      },
      body: JSON.stringify({
        kind,
        identity,
        title: entry.title,
        file_name: entry.file_name,
        file_path: entry.file_path,
        source_url: entry.source_url,
        media_type: entry.media_type,
        content_id: entry.content_id,
      }),
    });
    return res.ok;
  } catch (_err) {
    return false;
  }
}

export function recentMediaAppContext(entry = {}) {
  return {
    windowTitle: entry.title || entry.fileName || appTitle(entry.kind),
    title: entry.title || entry.fileName || '',
    fileName: entry.fileName || entry.title || '',
    filePath: entry.filePath || '',
    sourcePath: entry.filePath || '',
    sourceUrl: entry.sourceUrl || '',
    mediaType: entry.mediaType || '',
    contentId: entry.contentId || '',
  };
}
