// Library (new) data layer: the published package catalog, one release, and
// this computer's installs. Every request goes through the shell's renewing
// fetch; nothing here hard-codes a computer id.
import { AuthRequiredError, fetchWithRenewal, notifyPackagesChanged } from '../../lib/app-sdk';

export class CatalogError extends Error {
  constructor(message, status = 0, reason = '') {
    super(message);
    this.name = 'CatalogError';
    this.status = status;
    this.reason = reason;
  }
}

const UNREACHABLE = 'The catalog did not answer. Check this computer\'s connection, then try again.';
const SERVER_BROKE = 'The catalog could not complete the request. Try again in a moment.';
const REFUSED = 'This computer refused the request. Nothing changed here.';

// plainReason turns a machine reason into what the owner can act on. The raw
// server message is never shown.
export function plainReason(reason, status = 0) {
  switch (String(reason || '')) {
    case 'patch_does_not_apply':
      return 'The release was built on a different base, so its patch does not apply to this computer. Nothing changed on this computer. Install a release built on this computer\'s base, or wait for the publisher to publish one.';
    case 'package_name_taken':
      return 'This computer already has its own app by that name, which an install never overwrites. Nothing changed on this computer.';
    case 'package_already_present':
      return 'This computer already holds exactly this release. Nothing changed.';
    case 'package_not_installed':
      return 'This release is not what this computer holds under that name, so there is nothing of it to uninstall. Nothing changed on this computer.';
    case 'package_content_invalid':
    case 'patch_secret_refused':
    case 'patch_digest_mismatch':
    case 'release_digest_mismatch':
    case 'published_record_digest_mismatch':
      return 'The release did not verify on this computer, so it was not installed. Nothing changed on this computer.';
    case 'patch_unavailable':
      return 'The Library does not hold this release\'s files, so it cannot be installed. Nothing changed on this computer.';
    case 'desks_unavailable':
      return 'This computer\'s desks did not take the install. Nothing changed on this computer. Try again in a moment.';
    case 'package_build_failed':
      return 'This computer could not build the release, so it was not installed. Nothing changed on this computer.';
    case 'install_unavailable':
      return 'This computer cannot install apps right now. Nothing changed on this computer. Try again in a moment.';
    case 'release_not_published':
      return 'That release is not available to install on this computer (it is no longer published). Nothing changed on this computer.';
    case 'release_id_invalid':
    case 'request_invalid':
      return 'This window sent an install the computer could not read. Nothing changed on this computer. Reload the app and try again.';
    case 'capsule_fixture':
      return 'This window cannot install anything. Nothing changed on this computer.';
    default:
      if (status >= 500) return SERVER_BROKE;
      return REFUSED;
  }
}

async function requestJSON(path, options) {
  let res;
  try {
    res = await fetchWithRenewal(path, { credentials: 'include', ...(options || {}) });
  } catch (err) {
    if (err && err.name === 'AuthRequiredError') throw err;
    throw new CatalogError(UNREACHABLE, 0, 'unreachable');
  }
  let body = null;
  try {
    body = await res.json();
  } catch (_err) {
    body = null;
  }
  if (!res.ok) {
    const reason = body && typeof body.reason === 'string' ? body.reason : '';
    throw new CatalogError(plainReason(reason, res.status), res.status, reason);
  }
  return body && typeof body === 'object' ? body : {};
}

// loadPackages lists the packages this computer may see; q searches.
export async function loadPackages(q = '') {
  const needle = String(q || '').trim();
  const path = needle ? '/api/catalog/packages?q=' + encodeURIComponent(needle) : '/api/catalog/packages';
  return requestJSON(path, { headers: { accept: 'application/json' } });
}

// loadPackageReleases reads one package track's history, newest first.
export async function loadPackageReleases(name, track = '', opts = {}) {
  const params = new URLSearchParams();
  if (track) params.set('track', track);
  if (opts.limit) params.set('limit', String(opts.limit));
  if (opts.beforeSeq) params.set('before_seq', String(opts.beforeSeq));
  const query = params.toString();
  const path = '/api/catalog/packages/' + encodeURIComponent(name) + '/releases' + (query ? '?' + query : '');
  return requestJSON(path, { headers: { accept: 'application/json' } });
}

// loadRelease reads one release, its record and its listing.
export async function loadRelease(releaseId) {
  return requestJSON('/api/catalog/releases/' + encodeURIComponent(releaseId), { headers: { accept: 'application/json' } });
}

// loadInstalls reads what is installed on this computer.
export async function loadInstalls(computerId) {
  const path = '/api/computers/' + encodeURIComponent(computerId) + '/self-development/adoptions';
  return requestJSON(path, { headers: { accept: 'application/json' } });
}

// installRelease asks this computer to install a release (action "install")
// or remove one it installed ("uninstall"). The platform reads the release
// and its files from the Library itself; the record, when sent, must be the
// one the catalog returned. 201 is a new request, 200 one already under way
// or done.
export async function installRelease(computerId, releaseId, record, action = 'install') {
  const path = '/api/computers/' + encodeURIComponent(computerId) + '/self-development/adoptions';
  let res;
  try {
    res = await fetchWithRenewal(path, {
      method: 'POST',
      credentials: 'include',
      headers: { 'content-type': 'application/json', accept: 'application/json' },
      body: JSON.stringify(record ? { release_id: releaseId, record, action } : { release_id: releaseId, action }),
    });
  } catch (err) {
    if (err && err.name === 'AuthRequiredError') throw err;
    throw new CatalogError(UNREACHABLE, 0, 'unreachable');
  }
  let body = null;
  try {
    body = await res.json();
  } catch (_err) {
    body = null;
  }
  if (res.status === 200 || res.status === 201) notifyPackagesChanged();
  if (res.status !== 200 && res.status !== 201) {
    const reason = body && typeof body.reason === 'string' ? body.reason : '';
    throw new CatalogError(plainReason(reason, res.status), res.status, reason);
  }
  return { status: res.status, body: body && typeof body === 'object' ? body : {} };
}

// ---- reading a release ----

export function packageOf(release) {
  const record = (release && release.record) || {};
  const listing = (release && release.listing) || {};
  const name = (record.package && record.package.name) || listing.package || '';
  return String(name || '');
}

export function versionOf(release) {
  const listing = (release && release.listing) || {};
  const record = (release && release.record) || {};
  const version = (record.package && record.package.manifest_version) || '';
  return String(version || '');
}

export function seqOf(release) {
  const listing = (release && release.listing) || {};
  const seq = Number(listing.seq);
  return Number.isFinite(seq) ? seq : 0;
}

export function releaseLabel(release) {
  const name = packageOf(release) || 'release';
  const seq = seqOf(release);
  return seq > 0 ? name + ' ' + seq : name;
}

export function publisherOf(release) {
  const record = (release && release.record) || {};
  const publisher = record.publisher || {};
  return String(publisher.computer_id || '');
}

export function ownerOf(release) {
  const record = (release && release.record) || {};
  const publisher = record.publisher || {};
  return String(publisher.owner_id || '');
}

export function releaseIdOf(release) {
  return String((release && release.release_id) || ((release && release.listing && release.listing.record_id) || ''));
}

// recent release id: the id as published, shortened for a line of text.
export function shortId(value, keep = 8) {
  const text = String(value || '');
  if (!text) return '';
  if (text.startsWith('computer-') && text.length > 9 + keep) return 'computer-' + text.slice(9, 9 + keep) + '…';
  if (text.length <= keep + 1) return text;
  return text.slice(0, keep) + '…';
}

// publisherLabel names this computer as "this computer", as the owner asked.
export function publisherLabel(computerId, thisComputerId) {
  const id = String(computerId || '');
  if (!id) return 'an unknown computer';
  if (thisComputerId && id === String(thisComputerId)) return 'this computer';
  return shortId(id);
}

const MONTHS = ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec'];

// formatWhen renders a published time in plain words, in UTC (the catalog's
// own clock) so a screen reads the same for every owner.
export function formatWhen(iso) {
  const raw = String(iso || '');
  if (!raw) return '';
  const at = new Date(raw);
  if (Number.isNaN(at.getTime())) return '';
  const month = MONTHS[at.getUTCMonth()] || '';
  const day = at.getUTCDate();
  const hh = String(at.getUTCHours()).padStart(2, '0');
  const mm = String(at.getUTCMinutes()).padStart(2, '0');
  return month + ' ' + day + ', ' + hh + ':' + mm + ' UTC';
}

export function formatBytes(bytes) {
  const n = Number(bytes);
  if (!Number.isFinite(n) || n <= 0) return '';
  if (n < 1024) return n + ' B';
  if (n < 1024 * 1024) return (n / 1024).toFixed(n < 10 * 1024 ? 1 : 0) + ' KB';
  return (n / (1024 * 1024)).toFixed(1) + ' MB';
}

// mediaOf is always an array: a release with none is the empty state.
export function mediaOf(release) {
  const media = release && release.media;
  if (!Array.isArray(media)) return [];
  return media.filter((item) => item && typeof item === 'object' && item.url);
}

// isPhoneShot: a shot taller than it is wide is a phone shot.
export function isPhoneShot(item) {
  const width = Number(item && item.width);
  const height = Number(item && item.height);
  if (!Number.isFinite(width) || !Number.isFinite(height) || width <= 0 || height <= 0) return false;
  return width < height;
}

export function isVideo(item) {
  return Boolean(item && (item.kind === 'video' || String(item.media_type || '').startsWith('video/')));
}

// kindLabel tells the two screenshot shapes apart in words.
export function kindLabel(item) {
  if (isVideo(item)) return 'Video';
  return isPhoneShot(item) ? 'Phone shot' : 'Desktop shot';
}

export function changedPathsOf(release) {
  const record = (release && release.record) || {};
  const source = record.source || {};
  return Array.isArray(source.changed_paths) ? source.changed_paths.map(String) : [];
}

export function precommitmentsOf(release) {
  const record = (release && release.record) || {};
  return Array.isArray(record.precommitments) ? record.precommitments.filter(Boolean) : [];
}

export function scopesOf(release) {
  const listing = (release && release.listing) || {};
  return Array.isArray(listing.scopes) ? listing.scopes.map(String) : [];
}

export function sharedWithOf(release) {
  const listing = (release && release.listing) || {};
  const shared = listing.shared_with || {};
  const owners = Array.isArray(shared.owners) ? shared.owners.map(String) : [];
  const computers = Array.isArray(shared.computers) ? shared.computers.map(String) : [];
  return { owners, computers };
}

// sortedByNewest keeps the history newest first even if a server page repeats.
export function sortedByNewest(releases) {
  const list = Array.isArray(releases) ? releases.slice() : [];
  list.sort((a, b) => {
    const seqA = seqOf(a);
    const seqB = seqOf(b);
    if (seqA !== seqB) return seqB - seqA;
    return String((b && b.published_at) || '').localeCompare(String((a && a.published_at) || ''));
  });
  return list;
}

// installState names where one install record stands: 'done' when it is in
// force (a runtime install is the package's current install, an uninstall its
// latest record), 'replaced' when a later install or uninstall of the package
// superseded it, and, for a source install from before runtime install,
// 'failed' when its operation failed, was rejected or was rolled back,
// 'approve' while the change waits for the owner's approval, and 'working'
// otherwise.
export function installState(item) {
  if (!item) return '';
  if (item.patch_matches_release === true) return 'done';
  if (item.state === 'replaced') return 'replaced';
  // A source install from before runtime install: what it did is in this
  // computer's source; it is neither running nor failed.
  if (item.state === 'source') return 'source';
  const state = String((item.operation && item.operation.state) || '');
  if (state === 'failed' || state === 'rejected' || state === 'rolled_back') return 'failed';
  if (state === 'applied') return 'failed';
  if (state === 'awaiting_approval') return 'approve';
  return 'working';
}

// adoptionsOf is the installed list, always an array.
export function adoptionsOf(body) {
  const list = body && body.adoptions;
  return Array.isArray(list) ? list.filter(Boolean) : [];
}
