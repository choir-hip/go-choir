// The one Files listing: how a folder is read, ordered and drawn (an icon, a
// size). The Files app and the file picker modal (FilePicker.svelte) both use
// it, so a folder lists the same everywhere and the picker is not a second file
// browser (owner, 2026-11-09: the "upload from files" chooser was "a command
// line file tree"; it should be the Files listing in a modal).
import { fetchWithRenewal } from './auth.js';
import { mediaRouteForFileName } from './media-utils.js';

export type FileEntry = { name: string; type: 'file' | 'directory' | string; size?: number; modified?: string };
export type ListResult =
  | { ok: true; entries: FileEntry[] }
  | { ok: false; status: number; error: string; authRequired?: boolean };

/** The extensions of images a wallpaper can be. */
export const IMAGE_EXTENSIONS = ['png', 'jpg', 'jpeg', 'webp', 'gif'];

export const filesURL = (segments: string[]): string =>
  segments.length ? '/api/files/' + segments.map(encodeURIComponent).join('/') : '/api/files';

/** Directories first, then files, each alphabetical. */
export function sortEntries<T extends FileEntry>(entries: T[]): T[] {
  return [...(entries || [])].sort((a, b) => {
    if (a.type === 'directory' && b.type !== 'directory') return -1;
    if (a.type !== 'directory' && b.type === 'directory') return 1;
    return String(a.name).localeCompare(String(b.name));
  });
}

/** Read one folder (segments from the Files root) as the Files app does. */
export async function listFolder(segments: string[]): Promise<ListResult> {
  try {
    const res = await fetchWithRenewal(filesURL(segments));
    if (!res.ok) {
      const body = await res.json().catch(() => ({}));
      let error = body?.error || `Failed to load directory (${res.status})`;
      if (res.status === 403) error = 'Access denied: you do not have permission to view this directory.';
      else if (res.status === 404) error = 'Directory not found.';
      return { ok: false, status: res.status, error, authRequired: res.status === 401 };
    }
    const data = await res.json();
    return { ok: true, entries: sortEntries(Array.isArray(data) ? data : []) };
  } catch (err) {
    if ((err as Error)?.name === 'AuthRequiredError') return { ok: false, status: 401, error: 'Sign in again.', authRequired: true };
    return { ok: false, status: 0, error: 'Failed to load directory. Please try again.' };
  }
}

export function isTextureShortcutName(name: unknown): boolean {
  return typeof name === 'string' && name.toLowerCase().endsWith('.texture');
}

export function isImageName(name: unknown): boolean {
  const text = String(name || '');
  const ext = text.toLowerCase().split('.').pop() || '';
  return text.includes('.') && IMAGE_EXTENSIONS.includes(ext);
}

export function fileIconFor(entry: FileEntry): string {
  if (entry.type === 'directory') return '📁';
  if (isTextureShortcutName(entry.name)) return '📝';
  const mediaRoute = mediaRouteForFileName(entry.name);
  if (mediaRoute?.appId === 'image') return '🖼️';
  if (mediaRoute?.appId === 'audio') return '🎧';
  if (mediaRoute?.appId === 'video') return '🎬';
  if (mediaRoute?.mediaType === 'application/pdf') return '📄';
  if (mediaRoute?.mediaType === 'application/epub+zip') return '📚';
  if (mediaRoute?.appId === 'slides') return '🖥️';
  if (mediaRoute?.appId === 'calendar') return '📅';
  return '📄';
}

export function formatFileSize(bytes: number): string {
  if (!bytes) return '0 B';
  const units = ['B', 'KB', 'MB', 'GB'];
  const i = Math.min(units.length - 1, Math.floor(Math.log(bytes) / Math.log(1024)));
  const val = (bytes / Math.pow(1024, i)).toFixed(i === 0 ? 0 : 1);
  return `${val} ${units[i]}`;
}
