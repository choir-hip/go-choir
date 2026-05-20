import { fetchWithRenewal, AuthRequiredError } from './auth.js';
import { currentDeviceId } from './live-events.js';

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

export function saveMediaPosition(kind, source = {}, currentTime = 0, duration = 0) {
  if (!Number.isFinite(currentTime) || currentTime < 0) return;
  const identity = mediaSourceIdentity(source);
  if (!kind || !identity) return;
  fetchWithRenewal('/api/media/progress', {
    method: 'PUT',
    headers: {
      'Content-Type': 'application/json',
      'X-Choir-Device': currentDeviceId(),
    },
    body: JSON.stringify({
      kind,
      identity,
      current_time: currentTime,
      duration: Number.isFinite(duration) ? duration : 0,
      playback_rate: Number(source.playbackRate) || 1,
      updated_by_device: currentDeviceId(),
    }),
  }).catch(() => {});
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
