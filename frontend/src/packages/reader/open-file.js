// Opening a PDF or EPUB from the user's disk, into the Reader's real path.
//
// The only way a file reaches the Reader's open path is a source the shell can
// fetch: a path under top-level Files (appContext.filePath) or a URL. So an
// uploaded file is written to top-level Files through the same route Files
// uses (PUT /api/files/<name>) and then opened by that path; nothing is
// rendered from an in-memory blob, and a reload of the window, the recents
// list and the media progress record all refer to the same durable file.
//
// This module owns the parts that must agree between the window and the page:
// the accept list, the name check, the byte signature of each format, and what
// the window makes of the shell's upload answer.
import { uploadMediaFile } from '../../lib/app-sdk';

export const READER_UPLOAD_ACCEPT = '.pdf,.epub,application/pdf,application/epub+zip';
export const READER_UPLOAD_ACCEPT_TYPES = ['application/pdf', 'application/epub+zip'];
export const READER_UPLOAD_FORMATS = [
  { format: 'pdf', mediaType: 'application/pdf', label: 'PDF' },
  { format: 'epub', mediaType: 'application/epub+zip', label: 'EPUB' },
];

const MAX_UPLOAD_BYTES = 512 * 1024 * 1024;

export function uploadFormatFor(name) {
  const ext = String(name || '').toLowerCase().split('.').pop();
  return READER_UPLOAD_FORMATS.find((entry) => entry.format === ext) || null;
}

// A name the Files route accepts as one path segment in top-level Files. It
// must not escape the root, carry a separator, be a dot name, or hold control
// bytes; the extension decides the format the Reader will parse it as.
export function validateUploadName(rawName) {
  const name = String(rawName || '').trim();
  if (!name) return { ok: false, error: 'The file has no name.' };
  if (name.length > 255) return { ok: false, error: 'That file name is too long.' };
  if (/[\u0000-\u001f\u007f]/.test(name)) return { ok: false, error: 'That file name has control characters the library cannot store.' };
  if (name === '.' || name === '..' || name.startsWith('.')) return { ok: false, error: 'That file name cannot start with a dot.' };
  if (name.includes('/') || name.includes('\\')) return { ok: false, error: 'File names cannot contain / or \\\\' };
  const spec = uploadFormatFor(name);
  if (!spec) return { ok: false, error: 'The Reader opens PDF and EPUB files. Choose one of those.' };
  return { ok: true, name, format: spec.format, mediaType: spec.mediaType };
}

// The first bytes of a file, as the format it claims to be. A browser usually
// sets file.type; when it does not (or a file is mislabelled), the signature is
// what the Reader will actually parse, and the Reader must agree with Files.
export function sniffUploadSignature(head) {
  const bytes = String(head || '');
  if (bytes.indexOf('%PDF') === 0) return 'pdf';
  // EPUB is a ZIP: "PK" then an end-of-central-directory or local header mark.
  if (bytes.indexOf('PK') === 0) return 'epub';
  return '';
}

export async function sniffUploadFormat(file) {
  const declared = uploadFormatFor(file && file.name);
  if (!file || typeof file.slice !== 'function') return { format: declared ? declared.format : '', error: '' };
  let head = '';
  try {
    const bytes = new Uint8Array(await file.slice(0, 8).arrayBuffer());
    head = String.fromCharCode.apply(null, Array.from(bytes));
  } catch (_err) {
    return { format: declared ? declared.format : '', error: '' };
  }
  const sniffed = sniffUploadSignature(head);
  if (sniffed) return { format: sniffed, error: '' };
  if (declared) {
    return {
      format: declared.format,
      error: declared.format === 'pdf'
        ? 'That file is not a PDF (its contents do not start with %PDF).'
        : 'That file is not an EPUB (it is not a ZIP archive).',
    };
  }
  return { format: '', error: '' };
}

// Check a dropped or chosen file and read what it is: the name decides what
// the Reader will parse, the signature decides whether it really is that.
export async function inspectUploadCandidate(file) {
  const named = validateUploadName(file && file.name);
  if (!named.ok) return { ok: false, error: named.error };
  const bytes = Number(file && file.size) || 0;
  if (!bytes) return { ok: false, error: named.name + ' is empty.' };
  if (bytes > MAX_UPLOAD_BYTES) return { ok: false, error: named.name + ' is larger than 512 MB.' };
  const sniffed = await sniffUploadFormat(file);
  if (sniffed.error) return { ok: false, error: sniffed.error };
  if (sniffed.format !== named.format) {
    return {
      ok: false,
      error: named.name + ' looks like an ' + String(sniffed.format).toUpperCase() + ', but its name says ' + String(named.format).toUpperCase() + '.',
    };
  }
  return { ok: true, name: named.name, format: named.format, mediaType: named.mediaType, file };
}

export function topLevelFileURL(name) {
  return '/api/files/' + encodeURIComponent(String(name || ''));
}

// Upload one file to top-level Files through the shell's upload (app-sdk's
// uploadMediaFile: the PUT /api/files/<name> request Files sends itself), then
// name the file path the Reader will open it by. The answer keeps the shape
// the Reader window already renders: ok, kind, error, and on success the name,
// format, mediaType, filePath and size.
//
// options: onProgress (upload progress) and signal (abort). The upload itself
// is app-sdk's uploadMediaFile; an upload made through it is the shell's PUT
// /api/files/<name>, which renews the session and classifies a 401 exactly as
// Files does, so a lost session comes back here as kind 'auth'.
export async function uploadReaderFile(file, options) {
  const opts = options || {};
  const onProgress = typeof opts.onProgress === 'function' ? opts.onProgress : undefined;
  const answer = await uploadMediaFile(file, {
    ...(onProgress ? { onProgress } : {}),
    ...(opts.signal ? { signal: opts.signal } : {}),
  });
  if (!answer.ok) {
    return {
      ok: false,
      kind: uploadFailureKind(answer),
      status: answer.status,
      error: answer.error || 'The file could not be saved to your files.',
    };
  }
  const named = validateUploadName(String(answer.path || file?.name || ''));
  const name = named.ok ? named.name : String(answer.path || file?.name || '');
  const spec = named.ok ? named : (uploadFormatFor(name) || { format: '', mediaType: '' });
  return {
    ok: true,
    name,
    format: spec.format,
    mediaType: spec.mediaType,
    filePath: answer.path,
    status: answer.status,
    size: Number(file?.size) || 0,
  };
}

// A failed upload as the Reader says it: an ended session, a cancelled
// request, an unreachable library, or a server answer worth showing verbatim.
function uploadFailureKind(answer) {
  const status = Number(answer?.status) || 0;
  if (status === 401) return 'auth';
  if (status === 0 && /cancel/i.test(String(answer?.error || ''))) return 'cancelled';
  if (status === 0) return 'network';
  return 'server';
}
