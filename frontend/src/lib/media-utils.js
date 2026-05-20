import { fetchWithRenewal, AuthRequiredError } from './auth.js';

export const MEDIA_FILE_ROUTES = [
  { appId: 'image', mediaType: 'image/png', extensions: ['png', 'jpg', 'jpeg', 'gif', 'webp', 'avif', 'bmp', 'svg'] },
  { appId: 'audio', mediaType: 'audio/mpeg', extensions: ['mp3', 'm4a', 'aac', 'ogg', 'oga', 'wav', 'flac', 'opus'] },
  { appId: 'video', mediaType: 'video/mp4', extensions: ['mp4', 'm4v', 'webm', 'mov', 'avi', 'mkv'] },
  { appId: 'pdf', mediaType: 'application/pdf', extensions: ['pdf'] },
  { appId: 'epub', mediaType: 'application/epub+zip', extensions: ['epub'] },
];

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
  const route = MEDIA_FILE_ROUTES.find((candidate) => candidate.extensions.includes(ext));
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

export function mediaStateKey(kind, source = {}) {
  return `choir-media:${kind}:${source.filePath || source.sourceUrl || source.title || 'untitled'}`;
}

export function loadMediaPosition(kind, source = {}) {
  try {
    const raw = window.localStorage.getItem(mediaStateKey(kind, source));
    if (!raw) return 0;
    const parsed = JSON.parse(raw);
    const value = Number(parsed.currentTime);
    return Number.isFinite(value) && value > 0 ? value : 0;
  } catch (_err) {
    return 0;
  }
}

export function saveMediaPosition(kind, source = {}, currentTime = 0, duration = 0) {
  if (!Number.isFinite(currentTime) || currentTime < 0) return;
  try {
    window.localStorage.setItem(mediaStateKey(kind, source), JSON.stringify({
      currentTime,
      duration: Number.isFinite(duration) ? duration : 0,
      updatedAt: new Date().toISOString(),
    }));
  } catch (_err) {
    // Local playback state is best-effort.
  }
}

const MEDIA_RECENT_LIMIT = 10;

export function mediaRecentStorageKey(kind) {
  return `choir-media-recent:${kind}`;
}

function mediaSourceIdentity(source = {}) {
  return source.filePath || source.sourceUrl || source.contentId || '';
}

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

export function loadRecentMedia(kind) {
  try {
    const raw = window.localStorage.getItem(mediaRecentStorageKey(kind));
    const parsed = raw ? JSON.parse(raw) : [];
    return Array.isArray(parsed) ? parsed.filter((entry) => entry?.identity).slice(0, MEDIA_RECENT_LIMIT) : [];
  } catch (_err) {
    return [];
  }
}

export function rememberRecentMedia(kind, source = {}) {
  const identity = mediaSourceIdentity(source);
  if (!kind || !identity) return false;
  try {
    const entry = {
      identity,
      kind,
      title: source.title || displayFileName(source) || appTitle(kind),
      fileName: displayFileName(source),
      filePath: source.filePath || '',
      sourceUrl: source.sourceUrl || '',
      mediaType: source.mediaType || '',
      contentId: source.contentId || '',
      openedAt: new Date().toISOString(),
    };
    const next = [
      entry,
      ...loadRecentMedia(kind).filter((candidate) => candidate.identity !== identity),
    ].slice(0, MEDIA_RECENT_LIMIT);
    window.localStorage.setItem(mediaRecentStorageKey(kind), JSON.stringify(next));
    return true;
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
