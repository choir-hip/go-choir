// Library data layer: the published package catalog, one release, and
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
    case 'package_build_failed':
      return 'This computer could not build the release, so it was not installed. Nothing changed on this computer.';
    case 'install_unavailable':
      return 'This computer cannot install apps right now. Nothing changed on this computer. Try again in a moment.';
    case 'release_not_published':
      return 'That release is not available to install on this computer (it is no longer published). Nothing changed on this computer.';
    case 'release_id_invalid':
    case 'request_invalid':
      return 'This window sent an install the computer could not read. Nothing changed on this computer. Reload the app and try again.';
    case 'publisher_address_unavailable':
      return 'The Library could not look up who made these apps because the address service did not answer. Try again in a moment.';
    case 'publisher_address_missing':
      return 'An app in the Library has a maker with no Choir address, which should never happen. Nothing changed on this computer.';
    case 'install_review_owed':
      return 'Management is reviewing this app; it installs when management accepts. Nothing changed on this computer yet.';
    case 'install_review_required':
    case 'install_review_unavailable':
      return 'This app asks for more than the basic permissions, so it installs only after a review, and the review could not run on this computer yet. Nothing changed on this computer.';
    case 'install_review_rejected':
      return 'The review of this app found a problem with what it asks for, so it was not installed. Nothing changed on this computer.';
    case 'install_review_refused':
      return 'This app asks for a permission only Choir\'s own apps may hold, so it cannot be installed. Nothing changed on this computer.';
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

// loadReputation reads a maker's record on this computer (MA7.6): sentences,
// each with the n it rests on, never a score. publisher is the release's
// track (its maker's account); it is sent, never shown.
export async function loadReputation(publisher) {
  const path = '/api/packages/reputation?publisher=' + encodeURIComponent(String(publisher || ''));
  return requestJSON(path, { headers: { accept: 'application/json' } });
}

// reputationSentences keeps the well-formed sentences of a reputation answer.
export function reputationSentences(body) {
  const list = body && Array.isArray(body.sentences) ? body.sentences : [];
  return list
    .filter((s) => s && typeof s.text === 'string' && s.text && Number.isInteger(Number(s.n)) && Number(s.n) >= 0)
    .map((s) => ({ text: s.text, n: Number(s.n) }));
}

// loadInstalls reads what is installed on this computer.
export async function loadInstalls(computerId) {
  const path = '/api/computers/' + encodeURIComponent(computerId) + '/packages/installs';
  return requestJSON(path, { headers: { accept: 'application/json' } });
}

// installSetAside asks this computer to install a package the migration set
// aside, drafted from the computer's own kept source. The owner's act; the
// install review runs when the app needs one.
export async function installSetAside(computerId, pair, pkg) {
  const path = '/api/computers/' + encodeURIComponent(computerId) + '/self-development/package-migration-install';
  let res;
  try {
    res = await fetchWithRenewal(path, {
      method: 'POST',
      credentials: 'include',
      headers: { 'content-type': 'application/json', accept: 'application/json' },
      body: JSON.stringify({ pair, package: pkg }),
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
  if (res.status !== 201) {
    const reason = body && typeof body.reason === 'string' ? body.reason : '';
    const detail = body && typeof body.error === 'string' ? body.error : '';
    throw new CatalogError(detail || plainReason(reason, res.status), res.status, reason);
  }
  notifyPackagesChanged();
  return body && typeof body === 'object' ? body : {};
}

// installRelease asks this computer to install a release (action "install")
// or remove one it installed ("uninstall"). The platform reads the release
// and its files from the Library itself; the record, when sent, must be the
// one the catalog returned. 201 is a new request, 200 one already under way
// or done.
export async function installRelease(computerId, releaseId, record, action = 'install') {
  const path = '/api/computers/' + encodeURIComponent(computerId) + '/packages/installs';
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
  // 202: the release widens the ceiling and waits on management's review.
  if (res.status !== 200 && res.status !== 201 && res.status !== 202) {
    const reason = body && typeof body.reason === 'string' ? body.reason : '';
    throw new CatalogError(plainReason(reason, res.status), res.status, reason);
  }
  return { status: res.status, body: body && typeof body === 'object' ? body : {} };
}

// ---- updates (F1) ----

// UPDATE_POLICY_KEY is the owner's update setting: a user preference.
export const UPDATE_POLICY_KEY = 'packages.update_policy';

// syncUpdates asks this computer to compare its installs with their tracks:
// it answers each installed app's update (available, needs approval, updated
// automatically, or what the platform did) and the owner's policy. An app on
// automatic updates here once its newest release has been listed a day.
export async function syncUpdates(computerId) {
  const path = '/api/computers/' + encodeURIComponent(computerId) + '/packages/updates';
  let res;
  try {
    res = await fetchWithRenewal(path, {
      method: 'POST',
      credentials: 'include',
      headers: { 'content-type': 'application/json', accept: 'application/json' },
      body: '{}',
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
  if (!res.ok) {
    const reason = body && typeof body.reason === 'string' ? body.reason : '';
    throw new CatalogError(plainReason(reason, res.status), res.status, reason);
  }
  const updates = Array.isArray(body && body.updates) ? body.updates.filter((u) => u && typeof u.package === 'string') : [];
  if (updates.some((u) => u.state === 'updated' || u.state === 'updated_by_platform' || u.state === 'removed_by_platform')) notifyPackagesChanged();
  return { updates, policy: normalizePolicy(body && body.policy) };
}

// normalizePolicy reads a policy; anything unreadable is manual.
export function normalizePolicy(raw) {
  const mode = (v) => (v === 'automatic' ? 'automatic' : 'manual');
  const apps = {};
  if (raw && raw.apps && typeof raw.apps === 'object') {
    for (const [name, value] of Object.entries(raw.apps)) apps[name] = mode(value);
  }
  return { default: mode(raw && raw.default), apps };
}

// policyFor is one app's update mode.
export function policyFor(policy, name) {
  const p = normalizePolicy(policy);
  return Object.prototype.hasOwnProperty.call(p.apps, name) ? p.apps[name] : p.default;
}

// savePolicy writes the owner's update setting.
export async function savePolicy(policy) {
  const value = normalizePolicy(policy);
  const body = await requestJSON('/api/preferences/' + UPDATE_POLICY_KEY, {
    method: 'PUT',
    headers: { 'content-type': 'application/json', accept: 'application/json' },
    body: JSON.stringify({ value }),
  });
  return normalizePolicy(body && body.value);
}

// updateLine says an installed app's update in plain words, or ''.
export function updateLine(update) {
  if (!update) return '';
  const change = update.change || {};
  const widening = [];
  if (Array.isArray(change.added_grants) && change.added_grants.length) widening.push('asks for new permissions: ' + change.added_grants.join(', '));
  if (Array.isArray(change.added_uses) && change.added_uses.length) widening.push('uses new parts of Choir: ' + change.added_uses.join(', '));
  if (change.uses_unknown) widening.push('what the installed version uses was not recorded');
  switch (update.state) {
    case 'available':
      return 'Update available' + (update.seq ? ' (version ' + update.seq + ')' : '') + '.';
    case 'needs_approval':
      return 'Update available, needs approval' + (widening.length ? ': it ' + widening.join('; ') : '') + '.';
    case 'updated':
      return 'Updated automatically' + (update.seq ? ' to version ' + update.seq : '') + '.';
    case 'removed_by_platform':
      return 'Removed by the platform: ' + String(update.detail || 'no reason given');
    case 'updated_by_platform':
      return 'Updated by the platform: ' + String(update.detail || 'no reason given');
    default:
      return '';
  }
}

// platformLine is the platform's last act on a package, from the install
// records (newest first): "Removed by the platform: <reason>" or "Updated by
// the platform: <reason>", or ''.
export function platformLine(installs, name) {
  const list = Array.isArray(installs) ? installs : [];
  const latest = list.find((item) => item && item.package === name);
  if (!latest || latest.trigger !== 'platform') return '';
  const reason = String(latest.platform_reason || 'no reason given');
  return latest.action === 'uninstall' ? 'Removed by the platform: ' + reason : 'Updated by the platform: ' + reason;
}

// platformRemoved is every package whose latest record is the platform's
// uninstall: it is no longer installed, and the owner is told why.
export function platformRemoved(installs) {
  const list = Array.isArray(installs) ? installs : [];
  const seen = new Set();
  const out = [];
  for (const item of list) {
    if (!item || seen.has(item.package)) continue;
    seen.add(item.package);
    if (item.trigger === 'platform' && item.action === 'uninstall') out.push(item);
  }
  return out;
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
// superseded it (a source install from before runtime install is history and
// arrives as replaced), and otherwise 'failed' when its operation failed, was
// rejected or was rolled back, 'approve' while the change waits for the
// owner's approval, and 'working'.
export function installState(item) {
  if (!item) return '';
  if (item.patch_matches_release === true) return 'done';
  if (item.state === 'replaced') return 'replaced';
  // An install waiting on management's review is working; one whose review
  // closed without an install (rejected, timed out, interrupted by a restart,
  // superseded by a newer install) or whose approved run failed is failed.
  if (item.state === 'pending_review') return 'working';
  if (['review_rejected', 'review_timed_out', 'review_interrupted', 'superseded', 'install_failed'].includes(item.state)) return 'failed';
  const state = String((item.operation && item.operation.state) || '');
  if (state === 'failed' || state === 'rejected' || state === 'rolled_back') return 'failed';
  if (state === 'applied') return 'failed';
  if (state === 'awaiting_approval') return 'approve';
  return 'working';
}

// installListOf is the installed list in a response body, always an array.
export function installListOf(body) {
  const list = body && body.installs;
  return Array.isArray(list) ? list.filter(Boolean) : [];
}

// setAsideOf reads the packages the forced-modularity migration set aside on
// this computer (GET installs' set_aside): apps a past change made that are
// not installed, each with why.
export function setAsideOf(body) {
  const list = body && body.set_aside;
  return Array.isArray(list) ? list.filter((item) => item && item.pair && item.package) : [];
}

// ---- the app store's plain words ----

// formatDay is a date in plain words ("Nov 4"; the year only when it is not
// this one), in UTC, the catalog's own clock.
export function formatDay(iso, now = new Date()) {
  const raw = String(iso || '');
  if (!raw) return '';
  const at = new Date(raw);
  if (Number.isNaN(at.getTime())) return '';
  const text = (MONTHS[at.getUTCMonth()] || '') + ' ' + at.getUTCDate();
  return at.getUTCFullYear() === now.getUTCFullYear() ? text : text + ', ' + at.getUTCFullYear();
}

// versionLabel names a release for a person: "Version 3, Nov 4".
export function versionLabel(release, now = new Date()) {
  const seq = seqOf(release);
  const day = formatDay(release && release.published_at, now);
  const name = seq > 0 ? 'Version ' + seq : 'Version';
  return day ? name + ', ' + day : name;
}

// descriptionOf is the manifest's one-line description the host derived at
// publish (not part of the record), or ''.
export function descriptionOf(release) {
  return String((release && release.description) || '').trim();
}

// predictionLabel and outcomeLabel say a precommitment in words, never as a
// raw number or id: "Predicted 88% likely", "Came true".
export function predictionLabel(precommit) {
  const n = Number(precommit && precommit.p_yes_permille);
  if (!Number.isFinite(n)) return 'No prediction recorded';
  return 'Predicted ' + Math.round(n / 10) + '% likely';
}

export function outcomeLabel(precommit) {
  const settled = String((precommit && precommit.settled) || '').trim().toLowerCase();
  if (!settled) return { tone: '', text: 'Not settled yet' };
  if (settled === 'yes') return { tone: 'ok', text: 'Came true' };
  if (settled === 'no') return { tone: 'warn', text: 'Did not come true' };
  return { tone: '', text: 'Settled' };
}

// installsOf is every install in force on this computer: a finished install
// that is not an uninstall, one per package.
export function installsOf(installs) {
  const list = Array.isArray(installs) ? installs : [];
  return list.filter((item) => item && item.action !== 'uninstall' && installState(item) === 'done');
}

// installedFor is the install in force for one maker's app: the package's
// install whose release is on that maker's track. tracks maps an installed
// release id to the track it was published on ('' when it is not listed any
// more). Another maker's app of the same name is not this one.
export function installedFor(installs, name, track, tracks) {
  return installsOf(installs).find((item) => item.package === name && (tracks || {})[String(item.release_id)] === track) || null;
}

// installedElsewhere is an install in force of the same name that is not this
// maker's: installing this one replaces it.
export function installedElsewhere(installs, name, track, tracks) {
  return installsOf(installs).find((item) => item.package === name && (tracks || {})[String(item.release_id)] !== track) || null;
}

// inFlightFor finds an install or uninstall of the package still working.
export function inFlightFor(installs, name) {
  const list = Array.isArray(installs) ? installs : [];
  return list.find((item) => item && item.package === name && installState(item) === 'working') || null;
}

// ---- what an app is (L1) ----
//
// The host derives a release's facts from the manifest it verified at publish
// (kind, tier, level, slots, facets, ceiling) and the catalog serves them
// beside the record. They are shown in words; a raw grant name appears only
// for a grant these words do not know yet.

const KINDS = { app: 'App', pack: 'Pack', provider: 'Provider' };

const TIERS = {
  T0: 'Data only: Choir checks it and applies it, and it runs no code.',
  T1: 'Runs its own code, which reaches the rest of Choir only through the app SDK.',
  T2: "Draws part of Choir's own screen, which only Choir's reviewed apps may do.",
};

const LEVELS = {
  default: 'Basic permissions: it installs once you confirm.',
  extended: 'Asks for more than the basic permissions: it installs only after a review on this computer.',
  privileged: "Asks for permissions only Choir's own apps may hold.",
};

const FACETS = {
  agent: 'Agent',
  commands: 'Commands',
  keybindings: 'Shortcuts',
  kinds: 'Data types',
  links: 'Links',
  menus: 'Menu items',
  opens: 'Opens files',
  settings: 'Settings',
  themes: 'Themes',
};

const SLOTS = {
  'contextMenu.renderer': 'context menu',
  'desktop.icons': 'desktop icons',
  launcher: 'launcher',
  'notifications.host': 'notifications',
  overview: 'overview',
  prompt: 'prompt bar',
};

const GRANTS = {
  'ui.captures.read': () => 'Read your captures in its window',
  'ui.read.kind': (a) => 'Read your ' + a + ' records in its window',
  'ui.net.egress': (a) => 'Reach ' + a + ' on the internet from its window',
  'ui.shell': () => "Draw part of Choir's own screen",
  'desk.act': () => 'Act on your computer through its agent',
  'desk.act.cancel': () => "Cancel its agent's own acts",
  'desk.captures.read': () => 'Let its agent read your captures',
  'desk.intake.read': () => 'Let its agent read what you send it',
  'desk.request': (a) => 'Ask other agents for help: ' + a,
  'desk.texture.search': () => 'Let its agent search your documents',
  'desk.mail.read': (a) => 'Let its agent read the mail you point it to (' + a + ')',
  'desk.net.egress': (a) => 'Let its agent reach ' + a + ' on the internet',
  'desk.read.kind': (a) => 'Let its agent read your ' + a + ' records',
};

function strings(list) {
  return Array.isArray(list) ? list.filter((v) => typeof v === 'string' && v) : [];
}

function normalizeFacts(raw) {
  const f = raw && typeof raw === 'object' ? raw : {};
  const ceiling = Array.isArray(f.ceiling)
    ? f.ceiling.filter((c) => c && typeof c.grant === 'string' && c.grant).map((c) => ({ grant: c.grant, args: strings(c.args), level: String(c.level || '') }))
    : [];
  return {
    kind: KINDS[f.kind] ? f.kind : '',
    tier: TIERS[f.tier] ? f.tier : '',
    level: LEVELS[f.level] ? f.level : '',
    slots: strings(f.slots),
    facets: strings(f.facets),
    ceiling,
    recorded: f.recorded === true,
  };
}

// factsOf is a release's facts; entryFactsOf a shelf card's (its head's).
export function factsOf(release) {
  return normalizeFacts(release && release.facts);
}

export function entryFactsOf(entry) {
  return normalizeFacts(entry && entry.head_facts);
}

export function kindName(kind) {
  return KINDS[kind] || '';
}

export function tierLine(tier) {
  return TIERS[tier] || '';
}

export function levelLine(level) {
  return LEVELS[level] || '';
}

export function facetName(facet) {
  return FACETS[facet] || facet;
}

export function slotName(slot) {
  return SLOTS[slot] || slot;
}

// ceilingLines says each grant of the ceiling in words, with its level.
export function ceilingLines(facts) {
  return (facts && Array.isArray(facts.ceiling) ? facts.ceiling : []).map((c) => {
    const say = GRANTS[c.grant];
    const args = (c.args || []).join(', ');
    return { text: say ? say(args) : c.grant + (args ? ' (' + args + ')' : ''), level: c.level, grant: c.grant };
  });
}

// leased: a desk's grants are lent under the lease policy; a window's are
// checked by the SDK bridge on every call.
export function hasDeskGrant(facts) {
  return Boolean(facts && Array.isArray(facts.ceiling) && facts.ceiling.some((c) => String(c.grant).startsWith('desk.')));
}

// shelfFacets are the filters the shelf offers: each kind and each slot
// present among the cards, in a stable order.
export function shelfFacets(entries) {
  const kinds = new Set();
  const slots = new Set();
  for (const entry of Array.isArray(entries) ? entries : []) {
    const f = entryFactsOf(entry);
    if (f.kind) kinds.add(f.kind);
    for (const s of f.slots) slots.add(s);
  }
  const out = [];
  for (const kind of Object.keys(KINDS)) if (kinds.has(kind)) out.push({ type: 'kind', value: kind, label: KINDS[kind] + 's' });
  for (const slot of [...slots].sort()) out.push({ type: 'slot', value: slot, label: 'Provides the ' + slotName(slot) });
  return out;
}

export function matchesFacet(entry, filter) {
  if (!filter) return true;
  const f = entryFactsOf(entry);
  if (filter.type === 'kind') return f.kind === filter.value;
  if (filter.type === 'slot') return f.slots.includes(filter.value);
  return true;
}

// displacesOf says what installing a release would stop being used, one line
// per slot it provides: another installed app that provides the slot now, or
// Choir's built-in one. A window app displaces nothing here (another maker's
// app of the same name is said apart, installedElsewhere).
export function displacesOf(facts, installs, entries, tracks, name, track) {
  const out = [];
  for (const slot of (facts && facts.slots) || []) {
    const current = (Array.isArray(entries) ? entries : []).find(
      (entry) =>
        !(entry.package === name && entry.track === track) &&
        entryFactsOf(entry).slots.includes(slot) &&
        installedFor(installs, entry.package, entry.track, tracks),
    );
    out.push(current ? 'the ' + slotName(slot) + ' from ' + current.package : "Choir's built-in " + slotName(slot));
  }
  return out;
}

// usesOf is what an installed release uses of Choir, as this computer's build
// extracted it, or null when the install did not record it.
export function usesOf(install) {
  if (!install || install.uses_recorded !== true) return null;
  return strings(install.uses).slice().sort();
}
