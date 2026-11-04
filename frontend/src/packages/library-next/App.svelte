<script>
  // Library (new): the published package releases this computer may see, what
  // each release is and who made it, and installing one as the owner's act.
  // The catalog, the install route and the install list are the platform's; this
  // package reads them through app-sdk's renewing fetch and never hard-codes the
  // computer it is on.
  import { onMount, createEventDispatcher } from 'svelte';
  import { contextMenu, fetchWithRenewal } from '../../lib/app-sdk';
  import Icon from './Icon.svelte';
  import {
    adoptionsOf,
    changedPathsOf,
    formatWhen,
    installRelease,
    installState,
    isPhoneShot,
    isVideo,
    kindLabel,
    loadInstalls,
    loadPackageReleases,
    loadPackages,
    loadRelease,
    mediaOf,
    ownerOf,
    packageOf,
    plainReason,
    precommitmentsOf,
    publisherLabel,
    publisherOf,
    releaseIdOf,
    releaseLabel,
    scopesOf,
    seqOf,
    sharedWithOf,
    shortId,
    sortedByNewest,
    versionOf,
  } from './catalog';
  import './library.css';

  const dispatch = createEventDispatcher();

  export let authenticated = false;
  export let appContext = {};
  export let windowId = '';

  // view: { kind: 'list' } | { kind: 'package', name, track } | { kind: 'release', id }
  let view = { kind: 'list' };
  let back = null;

  let computerId = '';
  let identityError = '';

  let packages = [];
  let packageCount = 0;
  let reader = '';
  let listLoading = false;
  let listError = '';
  let query = '';
  let heads = {};
  let searchTimer = null;
  let generation = 0;

  let history = { package: '', track: '', count: 0, releases: [] };
  let historyLoading = false;
  let historyError = '';
  let trackTabs = [];

  let release = null;
  let releaseLoading = false;
  let releaseError = '';

  let installs = [];
  let installNames = {};
  let installsError = '';

  let missing = {};
  let viewer = { open: false, index: 0, items: [] };
  let confirm = { open: false, release: null };
  let outcome = null;
  let refusedSheet = null;
  let pathsSheet = null;
  let notice = '';
  let noticeTimer = null;
  let actionBusy = false;

  // No platform-owner signal reaches packages on this base
  // (frontend/src/lib/apps/README.md documents it). The Base image view is
  // reserved for that owner and stays hidden until the shell exports one; it
  // makes no request and has no control that acts.
  let showBaseImage = false;

  let packagesInstalledFirst = false;
  $: packageName = view.kind === 'package' ? view.name : view.kind === 'release' && release ? packageOf(release) : '';
  $: packageTrack = view.kind === 'package' ? view.track : view.kind === 'release' && release ? trackOf(release) : '';
  $: releaseMedia = release ? mediaOf(release) : [];
  $: packagesInstalledFirst = installs.length > 0;

  onMount(() => {
    void boot();
    window.addEventListener('keydown', onKeydown);
    return () => {
      window.removeEventListener('keydown', onKeydown);
      if (searchTimer) clearTimeout(searchTimer);
      if (noticeTimer) clearTimeout(noticeTimer);
    };
  });

  async function boot() {
    await readIdentity();
    await Promise.all([refreshList(), refreshInstalls()]);
  }

  async function readIdentity() {
    try {
      const res = await fetchWithRenewal('/api/shell/bootstrap', {
        credentials: 'include',
        headers: { accept: 'application/json' },
      });
      if (!res.ok) throw new Error('bootstrap');
      const body = await res.json();
      computerId = String((body && body.computer_id) || '');
    } catch (_err) {
      computerId = '';
    }
    identityError = computerId
      ? ''
      : 'This window could not read which computer it is on, so it cannot install anything. Reload the app to try again.';
  }

  // ---- the list of packages ----

  async function refreshList() {
    const gen = ++generation;
    listLoading = true;
    listError = '';
    try {
      const body = await loadPackages(query);
      if (gen !== generation) return;
      const list = Array.isArray(body.packages) ? body.packages : [];
      packages = list;
      packageCount = Number.isFinite(Number(body.count)) ? Number(body.count) : list.length;
      reader = String(body.reader || '');
      listLoading = false;
      void loadHeads(list, gen);
    } catch (err) {
      if (gen !== generation) return;
      listLoading = false;
      packages = [];
      packageCount = 0;
      listError = messageOf(err);
      if (err && err.name === 'AuthRequiredError') reportAuth('catalog_read');
    }
  }

  // loadHeads reads each visible package's newest release so a row can say in
  // plain words who published it and when (never a track and a head hash).
  async function loadHeads(list, gen) {
    const wanted = list.slice(0, 16).filter((pkg) => pkg && pkg.head_release_id);
    const next = {};
    await Promise.all(
      wanted.map(async (pkg) => {
        const key = headKey(pkg);
        if (heads[key]) {
          next[key] = heads[key];
          return;
        }
        try {
          next[key] = await loadRelease(pkg.head_release_id);
        } catch (_err) {
          // The row keeps its release count when the newest release cannot be read.
        }
      }),
    );
    if (gen !== generation) return;
    heads = { ...heads, ...next };
  }

  function onSearchInput(event) {
    query = event.currentTarget.value;
    if (searchTimer) clearTimeout(searchTimer);
    searchTimer = setTimeout(() => {
      searchTimer = null;
      void refreshList();
    }, 220);
  }

  function clearSearch() {
    query = '';
    if (searchTimer) {
      clearTimeout(searchTimer);
      searchTimer = null;
    }
    void refreshList();
  }

  // ---- installs ----

  async function refreshInstalls() {
    if (!computerId) return;
    installsError = '';
    try {
      const body = await loadInstalls(computerId);
      installs = adoptionsOf(body);
      void nameInstalls(installs);
    } catch (err) {
      installs = [];
      installsError = messageOf(err);
      if (err && err.name === 'AuthRequiredError') reportAuth('install_read');
    }
  }

  // nameInstalls resolves each installed release so the list names it by
  // package and release instead of by its hash.
  async function nameInstalls(list) {
    const ids = [...new Set(list.map((item) => String(item.release_id || '')).filter(Boolean))];
    const next = { ...installNames };
    await Promise.all(
      ids.map(async (id) => {
        if (next[id]) return;
        try {
          next[id] = await loadRelease(id);
        } catch (_err) {
          // The row falls back to its release id.
        }
      }),
    );
    installNames = next;
  }

  function installOf(releaseId) {
    const id = String(releaseId || '');
    return installs.find((item) => String(item.release_id || '') === id) || null;
  }

  function installedTone(releaseId) {
    const item = installOf(releaseId);
    if (!item) return '';
    const state = installState(item);
    if (item.action === 'uninstall' || state === 'replaced' || state === 'source') return state === 'done' || state === 'replaced' || state === 'source' ? '' : 'warn';
    return state === 'done' ? 'ok' : 'warn';
  }

  function installedLabel(releaseId) {
    const item = installOf(releaseId);
    if (!item) return '';
    const state = installState(item);
    if (item.action === 'uninstall') {
      if (state === 'done') return '';
      return state === 'failed' ? 'Uninstall failed' : 'Uninstalling';
    }
    if (state === 'done') return 'Installed';
    if (state === 'replaced' || state === 'source') return '';
    if (state === 'failed') return 'Install failed';
    if (state === 'approve') return 'Waiting for your approval';
    return 'Installing';
  }

  // installTag is the tag an install row shows.
  function installTag(item) {
    const state = installState(item);
    const uninstall = item.action === 'uninstall';
    if (state === 'done') return { tone: 'ok', text: uninstall ? 'Uninstalled' : 'Installed' };
    if (state === 'replaced') return { tone: '', text: 'Replaced' };
    if (state === 'source') return { tone: '', text: 'Earlier source install' };
    if (state === 'failed') return { tone: 'warn', text: uninstall ? 'Uninstall failed' : 'Install failed' };
    if (state === 'approve') return { tone: 'warn', text: 'Waiting for your approval' };
    return { tone: 'warn', text: uninstall ? 'Uninstalling' : 'Installing' };
  }

  async function doUninstall(item) {
    if (!item || actionBusy || !computerId) return;
    actionBusy = true;
    const rel = installNames[String(item.release_id || '')] || { release_id: item.release_id, record: null, listing: {} };
    try {
      const result = await installRelease(computerId, String(item.release_id || ''), null, 'uninstall');
      const removed = installState(result.body) === 'done';
      outcome = {
        tone: 'ok',
        title: removed ? 'Uninstalled' : result.status === 201 ? 'Uninstall started' : 'Uninstall already under way',
        text: removed
          ? installTitle(item) + ' is uninstalled and has left the launcher.'
          : 'This computer is removing ' + installTitle(item) + '.',
        facts: [{ label: 'Install id', value: shortId(result.body.adoption_id, 24) || 'not recorded' }],
        release: rel,
      };
      await refreshInstalls();
    } catch (err) {
      if (err && err.name === 'AuthRequiredError') reportAuth('install');
      refusedSheet = { title: 'The uninstall was refused', text: messageOf(err), release: rel };
    } finally {
      actionBusy = false;
    }
  }

  function installTitle(item) {
    const known = installNames[String(item.release_id || '')];
    if (known) return releaseLabel(known);
    return 'release ' + shortId(item.release_id, 10);
  }

  function installLine(item) {
    const bits = [];
    const when = formatWhen(item.requested_at);
    const verb = item.action === 'uninstall' ? 'uninstall asked ' : 'install asked ';
    bits.push(when ? verb + when : 'time not recorded');
    bits.push('from ' + publisherLabel(item.publisher_computer_id, computerId));
    // A runtime install has no base: it is not in this computer's source.
    if (item.mode !== 'runtime') bits.push(item.same_base === true ? 'the same base' : 'a different base');
    const state = item.operation && item.operation.state ? String(item.operation.state) : '';
    if (state) bits.push('operation ' + state);
    return bits.join(' · ');
  }

  // ---- package history ----

  function tracksFor(name) {
    return packages.filter((pkg) => pkg && pkg.package === name);
  }

  function openPackage(name, track) {
    back = view;
    view = { kind: 'package', name, track: String(track || '') };
    void loadHistory(name, String(track || ''));
  }

  async function loadHistory(name, track) {
    historyLoading = true;
    historyError = '';
    trackTabs = tracksFor(name);
    try {
      const body = await loadPackageReleases(name, track, { limit: 6 });
      const list = sortedByNewest(body.releases);
      history = {
        package: String(body.package || name),
        track: String(body.track || track),
        count: Number.isFinite(Number(body.count)) ? Number(body.count) : list.length,
        releases: list,
      };
    } catch (err) {
      history = { package: name, track, count: 0, releases: [] };
      historyError = messageOf(err);
      if (err && err.name === 'AuthRequiredError') reportAuth('history_read');
    }
    historyLoading = false;
  }

  async function loadOlder() {
    const seqs = history.releases.map(seqOf).filter((n) => n > 0);
    const before = seqs.length ? Math.min(...seqs) : 0;
    if (!before) return;
    historyLoading = true;
    try {
      const body = await loadPackageReleases(history.package, history.track, { limit: 6, beforeSeq: before });
      const more = sortedByNewest(body.releases).filter((rel) => seqOf(rel) < before);
      history = {
        ...history,
        releases: sortedByNewest([...history.releases, ...more]),
        count: Number.isFinite(Number(body.count)) ? Number(body.count) : history.count,
      };
    } catch (err) {
      historyError = messageOf(err);
    }
    historyLoading = false;
  }

  // ---- one release ----

  function openRelease(id) {
    back = view;
    view = { kind: 'release', id: String(id || '') };
    void loadOne(String(id || ''));
  }

  async function loadOne(id) {
    releaseLoading = true;
    releaseError = '';
    release = null;
    try {
      release = await loadRelease(id);
    } catch (err) {
      releaseError = messageOf(err);
      if (err && err.name === 'AuthRequiredError') reportAuth('release_read');
    }
    releaseLoading = false;
  }

  function goList() {
    back = null;
    view = { kind: 'list' };
  }

  function goBack() {
    const previous = back;
    back = null;
    if (!previous) {
      view = { kind: 'list' };
      return;
    }
    view = previous;
    if (previous.kind === 'package') void loadHistory(previous.name, previous.track);
    else if (previous.kind === 'release') void loadOne(previous.id);
  }

  function backToPackage() {
    if (packageName) openPackage(packageName, packageTrack);
  }

  // ---- installing ----

  function askInstall(rel) {
    if (!rel) return;
    outcome = null;
    refusedSheet = null;
    confirm = { open: true, release: rel };
  }

  function cancelInstall() {
    confirm = { open: false, release: null };
  }

  function confirmTitle(rel) {
    const name = packageOf(rel) || 'this package';
    const seq = seqOf(rel);
    return seq > 0 ? 'Install ' + name + ' ' + seq + '?' : 'Install ' + name + '?';
  }

  async function doInstall() {
    const rel = confirm.release;
    if (!rel || actionBusy) return;
    if (!computerId) {
      confirm = { open: false, release: null };
      refusedSheet = {
        title: 'The install could not start',
        text: identityError || 'This window could not read which computer it is on, so it cannot install anything. Reload the app to try again.',
        release: rel,
      };
      return;
    }
    actionBusy = true;
    const id = releaseIdOf(rel);
    try {
      const result = await installRelease(computerId, id, rel.record);
      confirm = { open: false, release: null };
      showOutcome(rel, result);
      await refreshInstalls();
    } catch (err) {
      confirm = { open: false, release: null };
      if (err && err.name === 'AuthRequiredError') reportAuth('install');
      showRefusal(rel, err);
    } finally {
      actionBusy = false;
    }
  }

  function showOutcome(rel, result) {
    const body = (result && result.body) || {};
    const state = installState(body);
    const facts = [
      { label: 'Install id', value: shortId(body.adoption_id, 24) || 'not recorded' },
      { label: 'Release', value: releaseLabel(rel) },
    ];
    const op = body.operation && body.operation.state ? String(body.operation.state) : '';
    if (op) facts.push({ label: 'Operation', value: op });
    if (state === 'done') {
      outcome = {
        tone: 'ok',
        title: 'Installed',
        text: releaseLabel(rel) + ' is installed on this computer. Open it from the launcher.',
        facts,
        release: rel,
      };
      return;
    }
    if (state === 'failed') {
      outcome = {
        tone: 'warn',
        title: 'The last install did not finish',
        text: 'The install of ' + releaseLabel(rel) + ' ended without changing this computer (operation ' + (op || 'closed') + '). Press Install again to retry.',
        facts,
        release: rel,
      };
      return;
    }
    outcome = {
      tone: 'ok',
      title: result && result.status === 201 ? 'Install started' : 'Install already under way',
      text: 'This computer is installing ' + releaseLabel(rel) + '.',
      facts,
      release: rel,
    };
  }

  function showRefusal(rel, err) {
    const text = messageOf(err);
    refusedSheet = { title: 'The install was refused', text, release: rel };
  }

  // ---- actions a release row offers ----

  async function copyText(value, what) {
    const text = String(value || '');
    if (!text) {
      showNotice('There is no ' + what + ' to copy for this release.');
      return;
    }
    try {
      if (typeof navigator !== 'undefined' && navigator.clipboard && navigator.clipboard.writeText) {
        await navigator.clipboard.writeText(text);
        showNotice('Copied the ' + what + '.');
        return;
      }
    } catch (_err) {
      // fall through to the visible value
    }
    showNotice('This window could not copy the ' + what + '. The value is ' + text + '.');
  }

  function releaseMenu(rel) {
    return [
      { id: 'open', label: 'Open detail', onSelect: () => openRelease(releaseIdOf(rel)) },
      { id: 'install', label: 'Install', onSelect: () => askInstall(rel) },
      { separator: true },
      { id: 'copy-release', label: 'Copy release id', onSelect: () => copyText(releaseIdOf(rel), 'release id') },
      {
        id: 'copy-publisher',
        label: 'Copy publisher computer id',
        onSelect: () => copyText(publisherOf(rel), 'publisher computer id'),
      },
      { id: 'paths', label: 'Show changed paths', onSelect: () => openPaths(rel) },
    ];
  }

  function packageMenu(pkg) {
    const head = heads[headKey(pkg)];
    const items = [
      { id: 'open', label: 'Open detail', onSelect: () => openPackage(pkg.package, pkg.track) },
    ];
    if (head) {
      items.push({ id: 'install', label: 'Install newest release', onSelect: () => askInstall(head) });
      items.push({ separator: true });
      items.push({ id: 'copy-release', label: 'Copy release id', onSelect: () => copyText(releaseIdOf(head), 'release id') });
      items.push({
        id: 'copy-publisher',
        label: 'Copy publisher computer id',
        onSelect: () => copyText(publisherOf(head), 'publisher computer id'),
      });
      items.push({ id: 'paths', label: 'Show changed paths', onSelect: () => openPaths(head) });
    }
    return items;
  }

  function openPaths(rel) {
    const paths = changedPathsOf(rel);
    pathsSheet = {
      title: 'Changed paths — ' + releaseLabel(rel),
      paths,
      builtOn: String((((rel && rel.record) || {}).source || {}).built_on || ''),
    };
  }

  function showNotice(text) {
    notice = String(text || '');
    if (noticeTimer) clearTimeout(noticeTimer);
    noticeTimer = setTimeout(() => {
      noticeTimer = null;
      notice = '';
    }, 6000);
  }

  // ---- gallery ----

  function openViewer(items, index) {
    if (!items || !items.length) return;
    viewer = { open: true, index, items };
  }

  function viewerNext() {
    if (!viewer.open || !viewer.items.length) return;
    viewer = { ...viewer, index: (viewer.index + 1) % viewer.items.length };
  }

  function viewerPrev() {
    if (!viewer.open || !viewer.items.length) return;
    viewer = { ...viewer, index: (viewer.index - 1 + viewer.items.length) % viewer.items.length };
  }

  function closeViewer() {
    viewer = { ...viewer, open: false };
  }

  function markMissing(item) {
    const key = String((item && (item.sha256 || item.url)) || '');
    if (!key) return;
    missing = { ...missing, [key]: true };
  }

  function isMissing(item) {
    return Boolean(missing[String((item && (item.sha256 || item.url)) || '')]);
  }

  function currentMedia() {
    return viewer.items[viewer.index] || null;
  }

  // ---- keyboard ----

  function onKeydown(event) {
    if (event.key === 'Escape') {
      if (viewer.open) {
        closeViewer();
        event.preventDefault();
        return;
      }
      if (confirm.open || outcome || refusedSheet || pathsSheet) {
        confirm = { open: false, release: null };
        outcome = null;
        refusedSheet = null;
        pathsSheet = null;
        event.preventDefault();
      }
      return;
    }
    if (!viewer.open) return;
    if (event.key === 'ArrowRight') {
      viewerNext();
      event.preventDefault();
    } else if (event.key === 'ArrowLeft') {
      viewerPrev();
      event.preventDefault();
    }
  }

  function reportAuth(kind) {
    dispatch('authrequired', { kind, appId: 'library-next', appName: 'Library (new)' });
  }

  // ---- plain words ----

  function messageOf(err) {
    if (err && err.name === 'AuthRequiredError') {
      return 'This window lost its sign-in. Sign in again, then try once more.';
    }
    const text = err && err.message ? String(err.message) : '';
    return text || plainReason('', 0);
  }

  function headKey(pkg) {
    return String(pkg.package) + '\n' + String(pkg.track);
  }

  function trackOf(rel) {
    return String((rel && rel.listing && rel.listing.track) || '');
  }

  function trackLabel(track) {
    return 'track ' + shortId(track, 8);
  }

  function parentLine(rel) {
    const parent = ((rel && rel.record) || {}).parent || {};
    if (!parent.kind) return 'Not recorded';
    const parts = [String(parent.kind)];
    if (parent.commit) parts.push('commit ' + shortId(parent.commit, 10));
    if (parent.release_id) parts.push('release ' + shortId(parent.release_id, 10));
    if (!parent.commit && !parent.release_id && parent.tree) parts.push('tree ' + shortId(parent.tree, 10));
    return parts.join(' · ');
  }

  function scopeLine(rel) {
    const scopes = scopesOf(rel);
    const shared = sharedWithOf(rel);
    const state = String((rel && rel.listing && rel.listing.state) || '');
    const bits = [];
    bits.push(scopes.length ? 'visible to ' + scopes.join(', ') : 'no scopes listed');
    if (state) bits.push('state ' + state);
    if (shared.owners.length) bits.push(shared.owners.length + ' owner(s) named');
    if (shared.computers.length) bits.push(shared.computers.length + ' computer(s) named');
    return bits.join(' · ');
  }

  function publisherLine(rel) {
    const bits = ['published by ' + publisherLabel(publisherOf(rel), computerId)];
    const when = formatWhen(rel && rel.published_at);
    if (when) bits.push(when);
    const track = trackOf(rel);
    if (track) bits.push('track ' + shortId(track, 10));
    const seq = seqOf(rel);
    if (seq > 0) bits.push('seq ' + seq);
    return bits.join(' · ');
  }

  function rowLine(pkg, head) {
    if (!head) {
      const count = Number(pkg.releases) || 0;
      return 'newest ' + pkg.package + ' ' + pkg.head_seq + ' · ' + count + (count === 1 ? ' release' : ' releases');
    }
    const seq = seqOf(head) || Number(pkg.head_seq) || 0;
    const bits = [pkg.package + ' ' + seq, 'published by ' + publisherLabel(publisherOf(head), computerId)];
    const when = formatWhen(head.published_at);
    if (when) bits.push(when);
    return bits.join(' · ');
  }

  function historyLine(rel) {
    const bits = ['published by ' + publisherLabel(publisherOf(rel), computerId)];
    const when = formatWhen(rel && rel.published_at);
    if (when) bits.push(when);
    const track = trackOf(rel);
    if (track) bits.push('track ' + shortId(track, 8));
    return bits.join(' · ');
  }

  function releaseCountLabel(count) {
    return count + (count === 1 ? ' release' : ' releases');
  }

  function mediaSummary(media) {
    if (!media.length) return 'none attached';
    const desktop = media.filter((item) => !isVideo(item) && !isPhoneShot(item)).length;
    const phone = media.filter((item) => !isVideo(item) && isPhoneShot(item)).length;
    const video = media.filter(isVideo).length;
    const bits = [];
    if (desktop) bits.push(desktop + ' desktop');
    if (phone) bits.push(phone + ' phone');
    if (video) bits.push(video + ' video');
    return media.length + (media.length === 1 ? ' shot' : ' shots') + ' · ' + bits.join(', ');
  }

  function mediaLabel(rel) {
    const media = mediaOf(rel);
    return media.length ? media.length + (media.length === 1 ? ' shot' : ' shots') : 'No screenshots';
  }

  function shortKind(item) {
    if (isVideo(item)) return 'Video';
    return isPhoneShot(item) ? 'Phone' : 'Desktop';
  }

  function permille(value) {
    const n = Number(value);
    if (!Number.isFinite(n)) return 'p not recorded';
    return 'p ' + (n / 1000).toFixed(2);
  }

  function headMedia(pkg) {
    const list = pkg && Array.isArray(pkg.head_media) ? pkg.head_media : [];
    return list.filter((item) => item && item.url);
  }

  function shotsLabel(count) {
    if (!count) return 'No screenshots';
    return count + (count === 1 ? ' shot' : ' shots');
  }

  function headThumb(pkg) {
    const list = headMedia(pkg);
    const shot = list.find((item) => !isVideo(item)) || list[0] || null;
    return shot;
  }
</script>

<div class="lib-shell" data-library-app data-view={view.kind}>
  <div class="lib-bar">
    {#if view.kind !== 'list'}
      <button class="lib-btn lib-btn-quiet" data-back aria-label="Back" title="Back" on:click={goBack}>
        <Icon name="left" size={15} />
      </button>
    {/if}
    <div class="lib-crumbs">
      {#if view.kind === 'list'}
        <span class="lib-crumb-here">Library</span>
      {:else}
        <button class="lib-link" data-crumb="library" on:click={goList}>Library</button>
        <span class="lib-crumb-sep" aria-hidden="true">/</span>
        {#if view.kind === 'package'}
          <span class="lib-crumb-here">{view.name}</span>
        {:else}
          <button class="lib-link" data-crumb="package" on:click={backToPackage}>{packageName || 'Package'}</button>
          <span class="lib-crumb-sep" aria-hidden="true">/</span>
          <span class="lib-crumb-here">{release ? releaseLabel(release) : 'Release'}</span>
        {/if}
      {/if}
    </div>

    {#if view.kind === 'list'}
      <label class="lib-search">
        <span class="lib-search-icon"><Icon name="search" size={15} /></span>
        <input
          type="search"
          data-search-input
          value={query}
          placeholder="Search packages"
          aria-label="Search packages"
          autocomplete="off"
          spellcheck="false"
          on:input={onSearchInput}
        />
        {#if query}
          <button class="lib-clear" data-clear-search-inline aria-label="Clear search" title="Clear search" on:click={clearSearch}>
            <Icon name="close" size={14} />
          </button>
        {/if}
      </label>
    {/if}

    {#if showBaseImage}
      <button class="lib-tab" data-base-image on:click={() => (view = { kind: 'base' })}>Base image</button>
    {/if}
  </div>

  <div class="lib-body">
    {#if identityError && view.kind === 'list'}
      <p class="lib-note lib-identity" data-identity>{identityError}</p>
    {/if}

    {#if view.kind === 'list'}
      <section class="lib-pane" data-list-view>
        <div class="lib-list-head">
          <span class="lib-list-title">Packages</span>
          <span class="lib-list-count" data-list-count>{releaseCountLabel(packageCount)} from this catalog{reader ? ' · reader ' + shortId(reader, 8) : ''}</span>
        </div>

        {#if listError}
          <div class="lib-error" data-error role="alert">
            <span class="lib-error-icon"><Icon name="warning" size={26} /></span>
            <p class="lib-error-title">The catalog did not answer</p>
            <p class="lib-error-text">{listError}</p>
            <button class="lib-btn lib-btn-quiet" data-try-again on:click={refreshList}>
              <Icon name="refresh" size={15} /> Try again
            </button>
          </div>
        {:else if listLoading && !packages.length}
          <div class="lib-loading" data-loading role="status">
            <span>Loading the catalog…</span>
          </div>
        {:else if !packages.length && query}
          <div class="lib-empty" data-empty-search role="status">
            <p class="lib-empty-title">No packages match “{query}”</p>
            <p class="lib-empty-text">
              Nothing in the catalog has that name. Check the spelling, or search for an app such as files-next or
              slides-next.
            </p>
            <button class="lib-btn lib-btn-quiet" data-clear-search on:click={clearSearch}>
              <Icon name="close" size={15} /> Clear search
            </button>
          </div>
        {:else if !packages.length}
          <div class="lib-empty" data-empty role="status">
            <p class="lib-empty-title">No releases are published yet</p>
            <p class="lib-empty-text">When a computer publishes a package release, it appears here for every owner.</p>
          </div>
        {:else}
          <ul class="lib-packages">
            {#each packages as pkg (headKey(pkg))}
              {@const thumb = headThumb(pkg)}
              <li>
                <button
                  class="lib-row"
                  data-package-row
                  data-package={pkg.package}
                  data-track={pkg.track}
                  use:contextMenu={() => packageMenu(pkg)}
                  on:click={() => openPackage(pkg.package, pkg.track)}
                >
                  <span class="lib-thumb">
                    {#if thumb && !isMissing(thumb)}
                      <img src={thumb.url} alt="" loading="lazy" on:error={() => markMissing(thumb)} />
                    {:else if thumb}
                      <span class="lib-strip-none">Missing</span>
                    {:else}
                      <span class="lib-strip-none"><Icon name="image" size={16} /></span>
                    {/if}
                  </span>
                  <span class="lib-row-main">
                    <span class="lib-row-title">
                      {pkg.package}
                      {#if installedLabel(pkg.head_release_id)}
                        <span class="lib-tag" class:lib-tag-ok={installedTone(pkg.head_release_id) === 'ok'} class:lib-tag-warn={installedTone(pkg.head_release_id) === 'warn'}>
                          {installedLabel(pkg.head_release_id)}
                        </span>
                      {/if}
                    </span>
                    <span class="lib-row-meta">{rowLine(pkg, heads[headKey(pkg)])}</span>
                    <span class="lib-row-sub">{trackLabel(pkg.track)}</span>
                  </span>
                  <span class="lib-row-side">{shotsLabel(headMedia(pkg).length)}</span>
                </button>
              </li>
            {/each}
          </ul>
        {/if}

        <section class="lib-installed" class:lib-installed-first={packagesInstalledFirst} data-installed-section>
          <div class="lib-subhead">
            <span>Installed on this computer</span>
            <span data-installed-count>{installs.length} install{installs.length === 1 ? '' : 's'}</span>
          </div>
          {#if installsError}
            <p class="lib-note">{installsError}</p>
          {:else if !installs.length}
            <p class="lib-note" data-installed-empty>
              Nothing from the catalog is installed yet. Installing is your choice: open a release and press Install.
            </p>
          {:else}
            <ul class="lib-installs" data-installed-list>
              {#each installs as item (item.adoption_id || item.release_id)}
                <li class="lib-row lib-install-row" data-installed-row data-release-id={item.release_id}>
                  <span class="lib-row-main">
                    <span class="lib-row-title">{installTitle(item)}</span>
                    <span class="lib-row-meta">{installLine(item)}</span>
                  </span>
                  <span class="lib-row-side">
                    <span class="lib-tag" class:lib-tag-ok={installTag(item).tone === 'ok'} class:lib-tag-warn={installTag(item).tone === 'warn'} data-install-state>
                      {installTag(item).text}
                    </span>
                    {#if item.action !== 'uninstall' && installState(item) === 'done'}
                      <button class="lib-btn lib-btn-quiet" data-uninstall-button disabled={actionBusy || !computerId} on:click={() => doUninstall(item)}>
                        Uninstall
                      </button>
                    {/if}
                  </span>
                </li>
              {/each}
            </ul>
          {/if}
        </section>
      </section>
    {:else if view.kind === 'package'}
      <section class="lib-pane" data-package-view>
        <div class="lib-head">
          <div class="lib-head-main">
            <h2 class="lib-head-title">{view.name}</h2>
            <span class="lib-head-meta">
              {releaseCountLabel(history.count)} on this track · newest first
            </span>
          </div>
          <div class="lib-tabs" role="tablist" aria-label="Tracks">
            {#each trackTabs as tab (tab.track)}
              <button
                class="lib-tab"
                class:lib-on={tab.track === view.track}
                role="tab"
                aria-selected={tab.track === view.track}
                data-track-tab={tab.track}
                on:click={() => openPackage(view.name, tab.track)}
              >
                {trackLabel(tab.track)} · {tab.releases}
              </button>
            {/each}
          </div>
        </div>

        {#if historyError}
          <div class="lib-error" data-error role="alert">
            <span class="lib-error-icon"><Icon name="warning" size={26} /></span>
            <p class="lib-error-title">This history did not load</p>
            <p class="lib-error-text">{historyError}</p>
            <button class="lib-btn lib-btn-quiet" data-try-again on:click={() => loadHistory(view.name, view.track)}>
              <Icon name="refresh" size={15} /> Try again
            </button>
          </div>
        {:else if historyLoading && !history.releases.length}
          <div class="lib-loading" data-loading role="status"><span>Loading the release history…</span></div>
        {:else if !history.releases.length}
          <div class="lib-empty" data-empty role="status">
            <p class="lib-empty-title">No releases on this track yet</p>
            <p class="lib-empty-text">This package has no published release on this track.</p>
          </div>
        {:else}
          <div class="lib-subhead"><span>Release history</span><span>Newest first</span></div>
          <ul class="lib-releases">
            {#each history.releases as rel (releaseIdOf(rel))}
              <li>
                <button
                  class="lib-release-row"
                  data-release-row
                  data-release-id={releaseIdOf(rel)}
                  data-seq={seqOf(rel)}
                  use:contextMenu={() => releaseMenu(rel)}
                  on:click={() => openRelease(releaseIdOf(rel))}
                >
                  <span class="lib-seq">{seqOf(rel)}</span>
                  <span class="lib-row-main">
                    <span class="lib-row-title">
                      {packageOf(rel)} {seqOf(rel)}
                      {#if installedLabel(releaseIdOf(rel))}
                        <span class="lib-tag" class:lib-tag-ok={installedTone(releaseIdOf(rel)) === 'ok'} class:lib-tag-warn={installedTone(releaseIdOf(rel)) === 'warn'}>
                          {installedLabel(releaseIdOf(rel))}
                        </span>
                      {/if}
                    </span>
                    <span class="lib-row-meta">{historyLine(rel)}</span>
                  </span>
                  <span class="lib-row-side">{mediaLabel(rel)}</span>
                </button>
              </li>
            {/each}
          </ul>
          {#if history.releases.length < history.count}
            <button class="lib-btn lib-btn-quiet" data-older on:click={loadOlder}>
              <Icon name="download" size={15} /> Show older releases
            </button>
          {/if}
        {/if}
      </section>
    {:else if view.kind === 'release'}
      {#if releaseLoading && !release}
        <div class="lib-loading" data-loading role="status"><span>Loading the release…</span></div>
      {:else if releaseError}
        <div class="lib-error" data-error role="alert">
          <span class="lib-error-icon"><Icon name="warning" size={26} /></span>
          <p class="lib-error-title">This release did not load</p>
          <p class="lib-error-text">{releaseError}</p>
          <button class="lib-btn lib-btn-quiet" data-try-again on:click={() => loadOne(view.id)}>
            <Icon name="refresh" size={15} /> Try again
          </button>
        </div>
      {:else if release}
        <section class="lib-pane" data-release-view>
          <div class="lib-head">
            <div class="lib-head-main">
              <h2 class="lib-head-title" data-release-title>{releaseLabel(release)}</h2>
              <span class="lib-head-meta">{publisherLine(release)}</span>
            </div>
            <div class="lib-head-actions">
              {#if installedLabel(releaseIdOf(release))}
                <span class="lib-tag" class:lib-tag-ok={installedTone(releaseIdOf(release)) === 'ok'} class:lib-tag-warn={installedTone(releaseIdOf(release)) === 'warn'}>
                  {installedLabel(releaseIdOf(release))}
                </span>
              {/if}
              <button class="lib-btn" data-install-button on:click={() => askInstall(release)}>
                <Icon name="download" size={15} /> Install
              </button>
            </div>
          </div>

          <div class="lib-gallery" data-gallery>
            <div class="lib-gallery-head">
              <span>Screenshots</span>
              <span>{mediaSummary(releaseMedia)}</span>
            </div>
            {#if releaseMedia.length}
              <ul class="lib-media-strip">
                {#each releaseMedia as item, index (item.sha256 || item.url)}
                  <li>
                    <button
                      class="lib-strip-item"
                      class:lib-phone={isPhoneShot(item)}
                      data-strip-item
                      data-media-sha={item.sha256}
                      aria-label={'Open ' + kindLabel(item) + ' ' + (item.name || '')}
                      on:click={() => openViewer(releaseMedia, index)}
                    >
                      {#if isMissing(item)}
                        <span class="lib-strip-none">Missing</span>
                      {:else if isVideo(item)}
                        <span class="lib-strip-none"><Icon name="play" size={16} /> Video</span>
                      {:else}
                        <img
                          src={item.url}
                          alt={kindLabel(item) + ' of ' + releaseLabel(release)}
                          loading="lazy"
                          on:error={() => markMissing(item)}
                        />
                      {/if}
                      <span class="lib-strip-badge">{shortKind(item)}</span>
                    </button>
                  </li>
                {/each}
              </ul>
            {:else}
              <div class="lib-noshots" data-noshots>
                <span class="lib-noshots-title">No screenshots yet</span>
                <span class="lib-noshots-text">
                  The publisher attached no screenshot or video to this release. Its record and changes are below.
                </span>
              </div>
            {/if}
          </div>

          <div class="lib-grid" data-facts>
            <div class="lib-kv">
              <span class="lib-kv-label">Published</span>
              <span class="lib-kv-value">{formatWhen(release.published_at) || 'Not recorded'}</span>
            </div>
            <div class="lib-kv">
              <span class="lib-kv-label">Publisher</span>
              <span class="lib-kv-value">{publisherLabel(publisherOf(release), computerId)}</span>
            </div>
            <div class="lib-kv">
              <span class="lib-kv-label">Track and seq</span>
              <span class="lib-kv-value lib-code">{shortId(trackOf(release), 12) || 'no track'} · seq {seqOf(release)}</span>
            </div>
            <div class="lib-kv">
              <span class="lib-kv-label">Publisher owner</span>
              <span class="lib-kv-value lib-code">{shortId(ownerOf(release), 12) || 'not recorded'}</span>
            </div>
            <div class="lib-kv">
              <span class="lib-kv-label">Manifest version</span>
              <span class="lib-kv-value lib-code">{versionOf(release) || 'not recorded'}</span>
            </div>
            <div class="lib-kv">
              <span class="lib-kv-label">Built on</span>
              <span class="lib-kv-value lib-code">{shortId(String((((release.record || {}).source) || {}).built_on || ''), 12) || 'not recorded'}</span>
            </div>
            <div class="lib-kv">
              <span class="lib-kv-label">Parent</span>
              <span class="lib-kv-value">{parentLine(release)}</span>
            </div>
            <div class="lib-kv">
              <span class="lib-kv-label">Made by operation</span>
              <span class="lib-kv-value lib-code">{shortId(String((((release.record || {}).operation) || {}).operation_id || ''), 18) || 'not recorded'}</span>
            </div>
            <div class="lib-kv lib-kv-wide">
              <span class="lib-kv-label">Who may see it</span>
              <span class="lib-kv-value">{scopeLine(release)}</span>
            </div>
            <div class="lib-kv lib-kv-wide">
              <span class="lib-kv-label">What it changes</span>
              {#if changedPathsOf(release).length}
                <ul class="lib-paths lib-scroll">
                  {#each changedPathsOf(release) as path}
                    <li>{path}</li>
                  {/each}
                </ul>
              {:else}
                <span class="lib-kv-value">No changed paths recorded.</span>
              {/if}
            </div>
          </div>

          <div class="lib-subhead">
            <span>Precommitments</span>
            <span>{precommitmentsOf(release).length} recorded</span>
          </div>
          {#if precommitmentsOf(release).length}
            <ul class="lib-precs">
              {#each precommitmentsOf(release) as precommit, index (precommit.record_id || index)}
                <li class="lib-prec">
                  <span class="lib-prec-q">{precommit.question || 'No question recorded'}</span>
                  <span class="lib-tag">{permille(precommit.p_yes_permille)}</span>
                  <span class="lib-tag lib-tag-muted">
                    {precommit.settled ? 'Settled: ' + precommit.settled : 'Not settled yet'}
                  </span>
                </li>
              {/each}
            </ul>
          {:else}
            <p class="lib-note">No precommitments recorded for this release.</p>
          {/if}
        </section>
      {/if}
    {:else if view.kind === 'base'}
      <section class="lib-pane" data-base-view>
        <div class="lib-head">
          <div class="lib-head-main">
            <h2 class="lib-head-title">Base image</h2>
            <span class="lib-head-meta">Platform owner only</span>
          </div>
        </div>
        <div class="lib-grid">
          <div class="lib-kv lib-kv-wide">
            <span class="lib-kv-label">Current contents</span>
            <span class="lib-kv-value">coming: base image configuration</span>
          </div>
          <div class="lib-kv lib-kv-wide">
            <span class="lib-kv-label">Add to base image</span>
            <span class="lib-kv-value">coming: base image configuration</span>
          </div>
          <div class="lib-kv lib-kv-wide">
            <span class="lib-kv-label">Replace built-in app</span>
            <span class="lib-kv-value">coming: base image configuration</span>
          </div>
        </div>
      </section>
    {/if}
  </div>

  <p class="lib-notice" data-notice>{notice}</p>

  {#if confirm.open && confirm.release}
    <div class="lib-modal" data-confirm role="dialog" aria-modal="true" aria-label={confirmTitle(confirm.release)}>
      <div class="lib-veil" role="presentation" on:click={cancelInstall}></div>
      <div class="lib-dialog">
        <div class="lib-dialog-head">
          <h2 class="lib-dialog-title" data-confirm-title>{confirmTitle(confirm.release)}</h2>
          <button class="lib-x" data-confirm-close aria-label="Close" on:click={cancelInstall}>
            <Icon name="close" size={16} />
          </button>
        </div>
        <div class="lib-dialog-body">
          <p class="lib-note">
            {publisherLabel(publisherOf(confirm.release), computerId)} published this release
            {formatWhen(confirm.release.published_at) ? ' on ' + formatWhen(confirm.release.published_at) : ''}.
          </p>
          <div class="lib-facts">
            <div class="lib-fact">
              <span class="lib-kv-label-inline">Publisher</span>
              <span class="lib-code">{ownerOf(confirm.release) ? shortId(ownerOf(confirm.release), 12) : 'unknown'}</span>
            </div>
            <div class="lib-fact">
              <span class="lib-kv-label-inline">Publisher computer</span>
              <span class="lib-code">{publisherLabel(publisherOf(confirm.release), computerId)}</span>
            </div>
            <div class="lib-fact">
              <span class="lib-kv-label-inline">Release id</span>
              <span class="lib-code">{shortId(releaseIdOf(confirm.release), 16)}</span>
            </div>
            <div class="lib-fact">
              <span class="lib-kv-label-inline">Track and seq</span>
              <span>{shortId(trackOf(confirm.release), 10)} · seq {seqOf(confirm.release)}</span>
            </div>
          </div>
          <div>
            <span class="lib-kv-label">Paths this install changes</span>
            {#if changedPathsOf(confirm.release).length}
              <ul class="lib-paths lib-scroll">
                {#each changedPathsOf(confirm.release) as path}
                  <li>{path}</li>
                {/each}
              </ul>
            {:else}
              <p class="lib-note">This release records no changed paths.</p>
            {/if}
          </div>
          <p class="lib-note">Installing is your choice. Nothing else on this computer changes.</p>
          {#if identityError}
            <div class="lib-banner lib-banner-warn">
              <span class="lib-banner-title">Install is unavailable here</span>
              <p class="lib-note">{identityError}</p>
            </div>
          {/if}
        </div>
        <div class="lib-dialog-foot">
          <button class="lib-btn lib-btn-quiet" data-confirm-cancel on:click={cancelInstall}>Cancel</button>
          <button class="lib-btn" data-confirm-install disabled={actionBusy || !computerId} on:click={doInstall}>
            {actionBusy ? 'Installing…' : 'Install'}
          </button>
        </div>
      </div>
    </div>
  {/if}

  {#if outcome}
    <div class="lib-modal" data-outcome data-tone={outcome.tone} role="dialog" aria-modal="true" aria-label={outcome.title}>
      <div class="lib-veil" role="presentation" on:click={() => (outcome = null)}></div>
      <div class="lib-dialog">
        <div class="lib-dialog-head">
          <h2 class="lib-dialog-title" data-outcome-title>{outcome.title}</h2>
          <button class="lib-x" data-outcome-close aria-label="Close" on:click={() => (outcome = null)}>
            <Icon name="close" size={16} />
          </button>
        </div>
        <div class="lib-dialog-body">
          <div class="lib-banner" class:lib-banner-warn={outcome.tone === 'warn'}>
            <span class="lib-banner-title">{outcome.title}</span>
            <p class="lib-note">{outcome.text}</p>
          </div>
          <div class="lib-facts">
            {#each outcome.facts as fact}
              <div class="lib-fact">
                <span class="lib-kv-label-inline">{fact.label}</span>
                <span class="lib-code">{fact.value}</span>
              </div>
            {/each}
          </div>
        </div>
        <div class="lib-dialog-foot">
          <button class="lib-btn lib-btn-quiet" data-outcome-done on:click={() => (outcome = null)}>Done</button>
        </div>
      </div>
    </div>
  {/if}

  {#if refusedSheet}
    <div class="lib-modal" data-refused role="dialog" aria-modal="true" aria-label={refusedSheet.title}>
      <div class="lib-veil" role="presentation" on:click={() => (refusedSheet = null)}></div>
      <div class="lib-dialog">
        <div class="lib-dialog-head">
          <h2 class="lib-dialog-title" data-refused-title>{refusedSheet.title}</h2>
          <button class="lib-x" data-refused-close aria-label="Close" on:click={() => (refusedSheet = null)}>
            <Icon name="close" size={16} />
          </button>
        </div>
        <div class="lib-dialog-body">
          <div class="lib-banner lib-banner-bad">
            <span class="lib-banner-title">{refusedSheet.title}</span>
            <p class="lib-note" data-refused-text>{refusedSheet.text}</p>
          </div>
          <div class="lib-facts">
            <div class="lib-fact">
              <span class="lib-kv-label-inline">Release</span>
              <span class="lib-code">{releaseLabel(refusedSheet.release)}</span>
            </div>
            <div class="lib-fact">
              <span class="lib-kv-label-inline">Publisher</span>
              <span class="lib-code">{publisherLabel(publisherOf(refusedSheet.release), computerId)}</span>
            </div>
          </div>
          <p class="lib-note">Nothing was installed, and nothing else on this computer changed.</p>
        </div>
        <div class="lib-dialog-foot">
          <button class="lib-btn lib-btn-quiet" data-refused-done on:click={() => (refusedSheet = null)}>Close</button>
        </div>
      </div>
    </div>
  {/if}

  {#if pathsSheet}
    <div class="lib-modal" data-paths role="dialog" aria-modal="true" aria-label={pathsSheet.title}>
      <div class="lib-veil" role="presentation" on:click={() => (pathsSheet = null)}></div>
      <div class="lib-dialog">
        <div class="lib-dialog-head">
          <h2 class="lib-dialog-title">{pathsSheet.title}</h2>
          <button class="lib-x" data-paths-close aria-label="Close" on:click={() => (pathsSheet = null)}>
            <Icon name="close" size={16} />
          </button>
        </div>
        <div class="lib-dialog-body">
          {#if pathsSheet.paths.length}
            <ul class="lib-paths">
              {#each pathsSheet.paths as path}
                <li>{path}</li>
              {/each}
            </ul>
          {:else}
            <p class="lib-note">This release records no changed paths.</p>
          {/if}
          {#if pathsSheet.builtOn}
            <p class="lib-note">Built on commit {shortId(pathsSheet.builtOn, 12)}.</p>
          {/if}
        </div>
        <div class="lib-dialog-foot">
          <button class="lib-btn lib-btn-quiet" data-paths-done on:click={() => (pathsSheet = null)}>Close</button>
        </div>
      </div>
    </div>
  {/if}

  {#if viewer.open}
    {@const media = currentMedia()}
    <div class="lib-modal" data-viewer role="dialog" aria-modal="true" aria-label="Screenshot viewer">
      <div class="lib-veil" role="presentation" on:click={closeViewer}></div>
      <div class="lib-viewer">
        <div class="lib-viewer-head">
          <span class="lib-viewer-caption">
            {media ? shortKind(media) : 'Screenshot'}
            <span class="lib-viewer-name">{media ? media.name || '' : ''}</span>
          </span>
          <button class="lib-x" data-viewer-close aria-label="Close the larger view" on:click={closeViewer}>
            <Icon name="close" size={16} />
          </button>
        </div>
        <div class="lib-viewer-stage">
          {#if !media}
            <span class="lib-viewer-missing">This release has no screenshot to show.</span>
          {:else if isMissing(media)}
            <span class="lib-viewer-missing">
              This screenshot is missing from the catalog, so it cannot be shown. Try again later.
            </span>
          {:else if isVideo(media)}
            <video src={media.url} controls preload="metadata"></video>
          {:else}
            <img
              src={media.url}
              alt={kindLabel(media) + (release ? ' of ' + releaseLabel(release) : '')}
              on:error={() => markMissing(media)}
            />
          {/if}
        </div>
        <div class="lib-viewer-foot">
          <span class="lib-note">
            {viewer.items.length ? viewer.index + 1 + ' of ' + viewer.items.length : 'No shots'}
            {#if media}· {media.width && media.height ? media.width + ' × ' + media.height : 'video'}{/if}
          </span>
          <div class="lib-viewer-nav">
            <button class="lib-btn lib-btn-quiet" data-viewer-prev aria-label="Previous screenshot" disabled={viewer.items.length < 2} on:click={viewerPrev}>
              <Icon name="left" size={15} /> Previous
            </button>
            <button class="lib-btn lib-btn-quiet" data-viewer-next aria-label="Next screenshot" disabled={viewer.items.length < 2} on:click={viewerNext}>
              Next <Icon name="right" size={15} />
            </button>
          </div>
        </div>
      </div>
    </div>
  {/if}
</div>

<style>
  .lib-head-actions {
    display: flex;
    flex: 0 0 auto;
    flex-wrap: wrap;
    align-items: center;
    gap: 0.75rem;
  }

  .lib-installed {
    display: flex;
    flex-direction: column;
    gap: 0.5rem;
    min-width: 0;
  }

  /* When something is installed, the shelf shows it above the catalog. */
  .lib-installed-first {
    order: -1;
  }

  .lib-install-row {
    cursor: default;
  }

  .lib-identity {
    margin: 0 0 0.6rem;
  }
</style>
