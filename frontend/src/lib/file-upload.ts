// Upload helpers for every app (owner, 2026-11-03: "all apps that depend on
// files should support file upload!"). They send the request Files sends,
// PUT /api/files/<folder>/<name> through uploadMediaFile, so the session
// renews, the device header is set and the file lands where Files shows it.
//
//   pickAndUploadFiles({ accept, folder, multiple }) -> stored paths
//   uploadFiles(files, { folder })                   -> stored paths
//   use:dropZone={{ folder, accept, onUploaded, onError }}
//
// A failed upload rejects with an UploadError that carries the paths that
// were stored before or after it (`paths`) and each failure (`failures`).
import { uploadMediaFile } from './media-utils.js';

export type UploadFailure = { name: string; status: number; error: string };

export class UploadError extends Error {
  paths: string[];
  failures: UploadFailure[];
  /** The session ended: the caller should ask the shell for sign-in. */
  authRequired: boolean;

  constructor(paths: string[], failures: UploadFailure[]) {
    const first = failures[0];
    super(failures.length === 1 ? `${first.name}: ${first.error}` : `${failures.length} files did not upload (${first.name}: ${first.error})`);
    this.name = 'UploadError';
    this.paths = paths;
    this.failures = failures;
    this.authRequired = failures.some((f) => f.status === 401);
  }
}

export type UploadOptions = {
  /** Folder under the files root, e.g. "Decks/2026"; '' or omitted for the top level. */
  folder?: string;
  onProgress?: (progress: { file: File; index: number; loaded: number; total: number; fraction: number }) => void;
  signal?: AbortSignal;
};

/** Normalizes accept: ['pptx', '.pdf', 'text/html'] -> '.pptx,.pdf,text/html'. */
export function acceptAttribute(accept?: string | string[]): string {
  if (!accept) return '';
  const list = Array.isArray(accept) ? accept : String(accept).split(',');
  return list
    .map((a) => String(a).trim())
    .filter(Boolean)
    .map((a) => (a.includes('/') || a.startsWith('.') ? a : `.${a}`))
    .map((a) => (a.includes('/') ? a : a.toLowerCase()))
    .join(',');
}

/** True when `file` matches an accept list (extensions, exact or wildcard media types). */
export function fileMatchesAccept(file: { name?: string; type?: string }, accept?: string | string[]): boolean {
  const parts = acceptAttribute(accept).split(',').filter(Boolean);
  if (!parts.length) return true;
  const name = String(file?.name || '').toLowerCase();
  const type = String(file?.type || '').toLowerCase();
  return parts.some((a) => {
    if (a.startsWith('.')) return name.endsWith(a);
    if (a.endsWith('/*')) return type.startsWith(a.slice(0, -1));
    return type === a.toLowerCase();
  });
}

/** Uploads each file in turn into `folder`; resolves to the stored paths. */
export async function uploadFiles(files: Iterable<File> | ArrayLike<File>, options: UploadOptions = {}): Promise<string[]> {
  const list = Array.from(files || []);
  const paths: string[] = [];
  const failures: UploadFailure[] = [];
  for (let index = 0; index < list.length; index += 1) {
    const file = list[index];
    const name = String(file?.name || '');
    if (!name || name.includes('/') || name.includes('\\')) {
      failures.push({ name: name || '(unnamed)', status: 0, error: 'File names cannot contain / or \\' });
      continue;
    }
    const onProgress = options.onProgress
      ? (p: { loaded: number; total: number; fraction: number }) => options.onProgress?.({ file, index, ...p })
      : undefined;
    const answer = await uploadMediaFile(file, { folder: options.folder || '', onProgress, signal: options.signal });
    if (answer.ok) paths.push(answer.path);
    else failures.push({ name, status: answer.status, error: answer.error });
    if (answer.status === 0 && options.signal?.aborted) break;
  }
  if (failures.length) throw new UploadError(paths, failures);
  return paths;
}

/**
 * Opens the system file picker. Resolves to the chosen files, or [] when the
 * picker is dismissed. Call it from a user gesture (a click or a menu item).
 */
export function pickFiles(options: { accept?: string | string[]; multiple?: boolean } = {}): Promise<File[]> {
  return new Promise((resolve) => {
    const input = document.createElement('input');
    input.type = 'file';
    input.multiple = options.multiple !== false;
    const accept = acceptAttribute(options.accept);
    if (accept) input.accept = accept;
    input.setAttribute('data-choir-file-picker', '');
    input.style.position = 'fixed';
    input.style.left = '-10000px';
    input.style.opacity = '0';
    let settled = false;
    const finish = (files: File[]) => {
      if (settled) return;
      settled = true;
      input.remove();
      resolve(files);
    };
    input.addEventListener('change', () => finish(Array.from(input.files || [])));
    input.addEventListener('cancel', () => finish([]));
    document.body.appendChild(input);
    input.click();
  });
}

/** Picks files and uploads them into `folder`; resolves to the stored paths ([] if dismissed). */
export async function pickAndUploadFiles(
  options: { accept?: string | string[]; folder?: string; multiple?: boolean } & Omit<UploadOptions, 'folder'> = {},
): Promise<string[]> {
  const files = await pickFiles({ accept: options.accept, multiple: options.multiple });
  if (!files.length) return [];
  return uploadFiles(files, options);
}

export type DropZoneOptions = {
  folder?: string;
  accept?: string | string[];
  /** Called with the stored paths after the dropped files upload. */
  onUploaded?: (paths: string[]) => void;
  /** Called with the UploadError (or a refusal) when an upload fails. */
  onError?: (error: Error) => void;
  /** Called with files the accept list skipped. */
  onRejected?: (files: File[]) => void;
  /** Return false to refuse a drop (e.g. signed out); the zone then does nothing. */
  enabled?: boolean | (() => boolean);
  multiple?: boolean;
};

function hasFiles(event: DragEvent) {
  return Array.from(event.dataTransfer?.types || []).includes('Files');
}

/**
 * Svelte action: drop files on `node` to upload them. While files hover over
 * it the node carries data-drop-active (style it to show the target).
 */
export function dropZone(node: HTMLElement, options: DropZoneOptions = {}) {
  let opts = options;
  let depth = 0;
  const enabled = () => (typeof opts.enabled === 'function' ? opts.enabled() : opts.enabled !== false);
  const setActive = (on: boolean) => {
    if (on) node.setAttribute('data-drop-active', '');
    else node.removeAttribute('data-drop-active');
  };

  function onDragEnter(event: DragEvent) {
    if (!hasFiles(event) || !enabled()) return;
    event.preventDefault();
    depth += 1;
    setActive(true);
  }

  function onDragOver(event: DragEvent) {
    if (!hasFiles(event) || !enabled()) return;
    event.preventDefault();
    try {
      if (event.dataTransfer) event.dataTransfer.dropEffect = 'copy';
    } catch (_err) {
      // read-only dataTransfer in some browsers
    }
  }

  function onDragLeave(event: DragEvent) {
    if (!hasFiles(event)) return;
    depth = Math.max(0, depth - 1);
    if (depth === 0) setActive(false);
  }

  async function onDrop(event: DragEvent) {
    if (!hasFiles(event) || !enabled()) return;
    event.preventDefault();
    event.stopPropagation();
    depth = 0;
    setActive(false);
    let files = Array.from(event.dataTransfer?.files || []);
    if (opts.multiple === false) files = files.slice(0, 1);
    const accepted = files.filter((f) => fileMatchesAccept(f, opts.accept));
    const rejected = files.filter((f) => !accepted.includes(f));
    if (rejected.length) opts.onRejected?.(rejected);
    if (!accepted.length) return;
    try {
      const paths = await uploadFiles(accepted, { folder: opts.folder || '' });
      opts.onUploaded?.(paths);
    } catch (err) {
      if (opts.onError) opts.onError(err as Error);
      else console.error('[dropZone] upload failed', err);
    }
  }

  node.addEventListener('dragenter', onDragEnter);
  node.addEventListener('dragover', onDragOver);
  node.addEventListener('dragleave', onDragLeave);
  node.addEventListener('drop', onDrop);
  return {
    update(next: DropZoneOptions) {
      opts = next || {};
    },
    destroy() {
      node.removeEventListener('dragenter', onDragEnter);
      node.removeEventListener('dragover', onDragOver);
      node.removeEventListener('dragleave', onDragLeave);
      node.removeEventListener('drop', onDrop);
    },
  };
}

/** The same as dropZone, for callers outside Svelte markup. */
export const createDropZone = dropZone;
