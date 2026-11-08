<script>
  import { requireAuth as shellRequireAuth } from '../../lib/app-sdk';
  // Library: an app store. Every app listed here has screenshots and can be
  // installed on this computer. A card is an app and the person who made it
  // (shown by their account email); an app opens as one page with its
  // screenshots, what it promised and a plain Install button. Nothing here
  // shows an id: the catalog, the install route and the install list are the
  // platform's, read through app-sdk's renewing fetch.
  import { onMount } from 'svelte';
  import { fetchWithRenewal, onWindowEvent } from '../../lib/app-sdk';
  import Icon from './Icon.svelte';
  import {
    installListOf,
    installSetAside,
    setAsideOf,
    descriptionOf,
    formatDay,
    inFlightFor,
    installRelease,
    installedElsewhere,
    installedFor,
    installsOf,
    isVideo,
    loadInstalls,
    loadPackageReleases,
    loadPackages,
    loadRelease,
    loadReputation,
    mediaOf,
    outcomeLabel,
    plainReason,
    platformLine,
    platformRemoved,
    policyFor,
    savePolicy,
    syncUpdates,
    updateLine,
    precommitmentsOf,
    predictionLabel,
    releaseIdOf,
    reputationSentences,
    seqOf,
    sortedByNewest,
    versionLabel,
    ceilingLines,
    displacesOf,
    entryFactsOf,
    facetName,
    factsOf,
    hasDeskGrant,
    kindName,
    levelLine,
    matchesFacet,
    shelfFacets,
    slotName,
    tierLine,
    usesOf,
  } from './catalog';
  import './library.css';

  export let authenticated = false;
  export let appContext = {};
  export let windowId = '';

  // view: { kind: 'list' } | { kind: 'app', name, track }
  let view = { kind: 'list' };

  let computerId = '';
  let identityError = '';

  let packages = [];
  let listLoading = false;
  let listError = '';
  let query = '';
  let searchTimer = null;
  let generation = 0;

  let page = { entry: null, head: null, versions: [], next: 0, loading: false, error: '' };
  let pageGeneration = 0;

  let installs = [];
  // setAside: apps a past change made that the forced-modularity migration
  // did not install, each with why; the owner may install one.
  let setAside = [];
  // Where each installed release came from: release id -> { track, seq }.
  // An installed app belongs to one maker; another maker's app of the same
  // name is not it.
  let installInfo = {};
  let busy = false;
  let confirm = null; // the release the owner is asked about
  let confirmReplaces = false; // it replaces another maker's app of the name
  let message = null; // { tone: 'ok' | 'warn' | 'bad', text }
  let missing = {};
  let viewer = { open: false, index: 0 };
  let touchX = null;

  $: media = page.head ? mediaOf(page.head) : [];
  $: shots = media.filter((item) => !isVideo(item));
  $: tracks = Object.fromEntries(Object.entries(installInfo).map(([id, info]) => [id, info.track]));
  $: installed = view.kind === 'app' ? installedFor(installs, view.name, view.track, tracks) : null;
  $: elsewhere = view.kind === 'app' && !installed ? installedElsewhere(installs, view.name, view.track, tracks) : null;
  $: working = view.kind === 'app' ? inFlightFor(installs, view.name) : null;
  $: headId = page.head ? releaseIdOf(page.head) : '';
  $: installedSeq = installed ? (installInfo[String(installed.release_id)] || {}).seq || 0 : 0;
  $: upToDate = Boolean(installed && headId && String(installed.release_id) === headId);
  // Update is offered only to a newer version of the same maker's app.
  $: canUpdate = Boolean(installed && !upToDate && page.head && seqOf(page.head) > installedSeq);
  $: shelfInstalls = installsOf(installs);
  $: removedByPlatform = platformRemoved(installs);
  // Updates (F1): each installed app's update from the last sync, and the
  // owner's update setting (manual unless chosen otherwise).
  let updates = {};
  let policy = { default: 'manual', apps: {} };
  $: pageUpdate = installed ? updates[String(installed.package)] || null : null;
  // What the app is (L1): the host's facts about the head release, the shelf's
  // facet filter, and what installing it would stop being used.
  let facet = null; // { type: 'kind' | 'slot', value }
  $: facets = shelfFacets(packages);
  $: if (facet && !facets.some((f) => f.type === facet.type && f.value === facet.value)) facet = null;
  $: shelf = packages.filter((pkg) => matchesFacet(pkg, facet));
  $: facts = page.head ? factsOf(page.head) : null;
  $: displaces = facts && view.kind === 'app' ? displacesOf(facts, installs, packages, tracks, view.name, view.track) : [];
  // The install confirm says what the confirmed release would replace, from
  // that release's own facts (an older version may provide other slots).
  $: confirmDisplaces = confirm && view.kind === 'app' ? displacesOf(factsOf(confirm), installs, packages, tracks, view.name, view.track) : [];
  $: installedUses = installed ? usesOf(installed) : null;

  onMount(() => {
    void boot();
    const offKeydown = onWindowEvent('keydown', onKeydown);
    return () => {
      offKeydown();
      if (searchTimer) clearTimeout(searchTimer);
    };
  });

  async function boot() {
    await readIdentity();
    await refreshUpdates();
    await Promise.all([refreshList(), refreshInstalls()]);
  }

  // refreshUpdates compares this computer's installs with their tracks. An
  // app on automatic updates installs here; the platform's act lands here.
  async function refreshUpdates() {
    if (!computerId) return;
    try {
      const result = await syncUpdates(computerId);
      updates = Object.fromEntries(result.updates.map((u) => [u.package, u]));
      policy = result.policy;
    } catch (err) {
      if (err && err.name === 'AuthRequiredError') reportAuth('install_read');
    }
  }

  // askUpdate offers the update's release through the ordinary install
  // confirmation; the press is the owner's approval.
  function askUpdate(update) {
    const rel = update && update.view;
    if (!rel || !rel.record) return;
    askInstall(rel);
  }

  async function setPolicy(name, mode) {
    const next = { default: policy.default, apps: { ...policy.apps } };
    if (name) next.apps[name] = mode;
    else next.default = mode;
    try {
      policy = await savePolicy(next);
      await refreshUpdates();
      await refreshInstalls();
    } catch (err) {
      if (err && err.name === 'AuthRequiredError') reportAuth('install');
      message = { tone: 'bad', text: messageOf(err) + ' The update setting did not change.' };
    }
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
      : 'This window could not tell which computer it is on, so it cannot install anything. Reload the app to try again.';
  }

  // ---- the shelf ----

  async function refreshList() {
    const gen = ++generation;
    listLoading = true;
    listError = '';
    try {
      const body = await loadPackages(query);
      if (gen !== generation) return;
      packages = Array.isArray(body.packages) ? body.packages : [];
      listLoading = false;
    } catch (err) {
      if (gen !== generation) return;
      listLoading = false;
      packages = [];
      listError = messageOf(err);
      if (err && err.name === 'AuthRequiredError') reportAuth('catalog_read');
    }
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

  async function refreshInstalls() {
    if (!computerId) return;
    try {
      const body = await loadInstalls(computerId);
      installs = installListOf(body);
      setAside = setAsideOf(body);
      await resolveInstalled();
    } catch (err) {
      installs = [];
      setAside = [];
      if (err && err.name === 'AuthRequiredError') reportAuth('install_read');
    }
  }

  // resolveInstalled learns which maker's track each installed release is on.
  // A release that cannot be read (no longer listed) stays unresolved and
  // belongs to no card; the Installed section still lists it.
  async function resolveInstalled() {
    const next = { ...installInfo };
    await Promise.all(
      installsOf(installs).map(async (item) => {
        const id = String(item.release_id || '');
        if (!id || next[id]) return;
        try {
          const rel = await loadRelease(id);
          next[id] = { track: String((rel.listing && rel.listing.track) || ''), seq: seqOf(rel) };
        } catch (_err) {
          // unresolved
        }
      }),
    );
    installInfo = next;
  }

  function cardKey(pkg) {
    return String(pkg.package) + '\n' + String(pkg.track);
  }

  function thumbOf(pkg) {
    const list = Array.isArray(pkg && pkg.head_media) ? pkg.head_media : [];
    return list.find((item) => item && item.url && !isVideo(item)) || null;
  }

  function cardInstalled(pkg, list, byRelease) {
    return Boolean(installedFor(list, pkg.package, pkg.track, byRelease));
  }

  // The maker's public Choir address, as the server resolved it. The server
  // fails the read when it cannot, so there is no fallback text here.
  function publisherName(pkg) {
    return String((pkg && pkg.publisher_address) || '');
  }

  // ---- one app ----

  function openApp(pkg) {
    message = null;
    view = { kind: 'app', name: String(pkg.package), track: String(pkg.track || '') };
    page = { entry: pkg, head: null, versions: [], next: 0, loading: true, error: '' };
    void loadApp(pkg);
  }

  // The maker's record on this computer (MA7.6): sentences with their n.
  let reputation = { sentences: [], loaded: false, failed: false };

  function loadMakerRecord(pkg, gen) {
    reputation = { sentences: [], loaded: false, failed: false };
    const publisher = pkg && pkg.track;
    if (!publisher) return;
    loadReputation(publisher)
      .then((body) => {
        if (gen === pageGeneration) reputation = { sentences: reputationSentences(body), loaded: true, failed: false };
      })
      .catch(() => {
        if (gen === pageGeneration) reputation = { sentences: [], loaded: true, failed: true };
      });
  }

  async function loadApp(pkg) {
    const gen = ++pageGeneration;
    loadMakerRecord(pkg, gen);
    try {
      const [head, history] = await Promise.all([
        loadRelease(pkg.head_release_id),
        loadPackageReleases(pkg.package, pkg.track, { limit: 20 }),
      ]);
      if (gen !== pageGeneration) return;
      page = { entry: pkg, head, versions: sortedByNewest(history.releases), next: Number(history.next_before_seq) || 0, loading: false, error: '' };
    } catch (err) {
      if (gen !== pageGeneration) return;
      page = { entry: pkg, head: null, versions: [], next: 0, loading: false, error: messageOf(err) };
      if (err && err.name === 'AuthRequiredError') reportAuth('release_read');
    }
  }

  async function loadOlder() {
    if (!page.next || busy) return;
    const pkg = page.entry;
    try {
      const body = await loadPackageReleases(pkg.package, pkg.track, { limit: 20, beforeSeq: page.next });
      page = { ...page, versions: sortedByNewest([...page.versions, ...(body.releases || [])]), next: Number(body.next_before_seq) || 0 };
    } catch (err) {
      message = { tone: 'bad', text: messageOf(err) };
    }
  }

  function goList() {
    view = { kind: 'list' };
    message = null;
    viewer = { open: false, index: 0 };
    void refreshList();
  }

  // ---- install and uninstall ----

  function askInstall(rel) {
    if (!rel) return;
    message = null;
    confirmReplaces = Boolean(elsewhere);
    confirm = rel;
  }

  async function doInstall() {
    const rel = confirm;
    if (!rel || busy) return;
    if (!computerId) {
      confirm = null;
      message = { tone: 'bad', text: identityError };
      return;
    }
    busy = true;
    try {
      const result = await installRelease(computerId, releaseIdOf(rel), rel.record);
      confirm = null;
      await refreshInstalls();
      await refreshUpdates();
      const done = installedFor(installs, view.kind === 'app' ? view.name : '');
      message = result.status === 202
        ? { tone: 'ok', text: 'Management is reviewing this app; it installs when management accepts.' }
        : done
        ? { tone: 'ok', text: 'Installed. Open it from the launcher.' }
        : {
            tone: 'ok',
            text: result.status === 201 ? 'Installing. This takes a moment.' : 'This app is already being installed.',
          };
    } catch (err) {
      confirm = null;
      if (err && err.name === 'AuthRequiredError') reportAuth('install');
      message = { tone: 'bad', text: messageOf(err) + ' Nothing was installed.' };
    } finally {
      busy = false;
    }
  }

  async function doInstallSetAside(item) {
    if (!item || busy || !computerId) return;
    busy = true;
    try {
      await installSetAside(computerId, item.pair, item.package);
      await refreshInstalls();
      message = { tone: 'ok', text: 'Installed ' + item.package + '. Open it from the launcher.' };
    } catch (err) {
      if (err && err.name === 'AuthRequiredError') reportAuth('install');
      message = { tone: 'bad', text: messageOf(err) + ' Nothing was installed.' };
    } finally {
      busy = false;
    }
  }

  async function doUninstall(item) {
    if (!item || busy || !computerId) return;
    busy = true;
    try {
      await installRelease(computerId, String(item.release_id || ''), null, 'uninstall');
      await refreshInstalls();
      message = installsOf(installs).some((other) => String(other.release_id) === String(item.release_id))
        ? { tone: 'ok', text: 'Removing ' + item.package + ' from your computer.' }
        : { tone: 'ok', text: 'Uninstalled ' + item.package + '. It has left the launcher.' };
    } catch (err) {
      if (err && err.name === 'AuthRequiredError') reportAuth('install');
      message = { tone: 'bad', text: messageOf(err) + ' Nothing was removed.' };
    } finally {
      busy = false;
    }
  }

  // ---- screenshots ----

  function openViewer(index) {
    viewer = { open: true, index };
  }

  function closeViewer() {
    viewer = { ...viewer, open: false };
  }

  function step(delta) {
    if (!shots.length) return;
    viewer = { ...viewer, index: (viewer.index + delta + shots.length) % shots.length };
  }

  function onTouchStart(event) {
    touchX = event.changedTouches && event.changedTouches[0] ? event.changedTouches[0].clientX : null;
  }

  function onTouchEnd(event) {
    if (touchX === null) return;
    const x = event.changedTouches && event.changedTouches[0] ? event.changedTouches[0].clientX : touchX;
    const dx = x - touchX;
    touchX = null;
    if (Math.abs(dx) > 40) step(dx < 0 ? 1 : -1);
  }

  function markMissing(item) {
    missing = { ...missing, [String(item.url)]: true };
  }

  function onKeydown(event) {
    if (event.key === 'Escape') {
      if (viewer.open) closeViewer();
      else if (confirm) confirm = null;
      else return;
      event.preventDefault();
      return;
    }
    if (!viewer.open) return;
    if (event.key === 'ArrowRight') step(1);
    else if (event.key === 'ArrowLeft') step(-1);
    else return;
    event.preventDefault();
  }

  function reportAuth(kind) {
    shellRequireAuth({ kind, appId: 'library-next', appName: 'Library' });
  }

  function messageOf(err) {
    if (err && err.name === 'AuthRequiredError') return 'This window lost its sign-in. Sign in again, then try once more.';
    const text = err && err.message ? String(err.message) : '';
    return text || plainReason('', 0);
  }

  function updatedLine(rel) {
    const day = formatDay(rel && rel.published_at);
    return day ? 'Updated ' + day : '';
  }

  function cardDate(pkg) {
    const day = formatDay(pkg && pkg.head_published_at);
    return day ? 'Updated ' + day : '';
  }

  function publisherOfRelease(rel, entry) {
    return String((rel && rel.publisher_address) || (entry && entry.publisher_address) || '');
  }
</script>

<div class="lib-shell" data-library-app data-view={view.kind}>
  <div class="lib-bar">
    {#if view.kind === 'app'}
      <button class="lib-back" data-back aria-label="Back to the Library" on:click={goList}>
        <Icon name="left" size={18} />
      </button>
      <h1 class="lib-title" data-app-title>{view.name}</h1>
    {:else}
      <h1 class="lib-title lib-crumb-here">Library</h1>
      <label class="lib-search">
        <span class="lib-search-icon"><Icon name="search" size={15} /></span>
        <input
          type="search"
          data-search-input
          value={query}
          placeholder="Search apps"
          aria-label="Search apps"
          autocomplete="off"
          spellcheck="false"
          on:input={onSearchInput}
        />
        {#if query}
          <button class="lib-clear" data-clear-search-inline aria-label="Clear search" on:click={clearSearch}>
            <Icon name="close" size={14} />
          </button>
        {/if}
      </label>
    {/if}
  </div>

  <div class="lib-body">
    {#if message}
      <p class="lib-message" class:lib-message-bad={message.tone === 'bad'} data-message role="status">{message.text}</p>
    {/if}

    {#if view.kind === 'list'}
      {#if identityError}
        <p class="lib-note" data-identity>{identityError}</p>
      {/if}
      <section data-list-view>
        {#if listError}
          <div class="lib-state" data-error role="alert">
            <Icon name="warning" size={26} />
            <p class="lib-state-title">The Library did not answer</p>
            <p class="lib-note">{listError}</p>
            <button class="lib-btn lib-btn-quiet" data-try-again on:click={refreshList}>
              <Icon name="refresh" size={15} /> Try again
            </button>
          </div>
        {:else if listLoading && !packages.length}
          <div class="lib-state" data-loading role="status"><span>Loading apps…</span></div>
        {:else if !packages.length && query}
          <div class="lib-state" data-empty-search role="status">
            <p class="lib-state-title">No apps match “{query}”</p>
            <button class="lib-btn lib-btn-quiet" data-clear-search on:click={clearSearch}>
              <Icon name="close" size={15} /> Clear search
            </button>
          </div>
        {:else if !packages.length}
          <div class="lib-state" data-empty role="status">
            <p class="lib-state-title">No apps yet</p>
            <p class="lib-note">An app appears here once its maker publishes it with screenshots.</p>
          </div>
        {:else}
          {#if facets.length > 1 || facets.some((f) => f.type === 'slot')}
            <div class="lib-facets" data-facets role="group" aria-label="Show">
              <button class="lib-facet" class:lib-facet-on={!facet} data-facet-filter="all" aria-pressed={!facet} on:click={() => (facet = null)}>All</button>
              {#each facets as f (f.type + ':' + f.value)}
                {@const on = Boolean(facet && facet.type === f.type && facet.value === f.value)}
                <button class="lib-facet" class:lib-facet-on={on} data-facet-filter={f.type + ':' + f.value} aria-pressed={on} on:click={() => (facet = on ? null : { type: f.type, value: f.value })}>{f.label}</button>
              {/each}
            </div>
          {/if}
          <ul class="lib-cards">
            {#each shelf as pkg (cardKey(pkg))}
              {@const thumb = thumbOf(pkg)}
              <li>
                <button class="lib-card" data-package-row data-package={pkg.package} on:click={() => openApp(pkg)}>
                  <span class="lib-thumb">
                    {#if thumb && !missing[thumb.url]}
                      <img src={thumb.url} alt={'Screenshot of ' + pkg.package} loading="lazy" on:error={() => markMissing(thumb)} />
                    {:else}
                      <span class="lib-thumb-none"><Icon name="image" size={22} /></span>
                    {/if}
                  </span>
                  <span class="lib-card-body">
                    <span class="lib-card-name">
                      {pkg.package}
                      {#if cardInstalled(pkg, installs, tracks)}<span class="lib-tag lib-tag-ok" data-card-installed>Installed</span>{/if}
                    </span>
                    {#if kindName(entryFactsOf(pkg).kind)}
                      <span class="lib-card-kind" data-card-kind={entryFactsOf(pkg).kind}>{kindName(entryFactsOf(pkg).kind)}</span>
                    {/if}
                    {#if pkg.head_description}
                      <span class="lib-card-desc" data-card-desc>{pkg.head_description}</span>
                    {/if}
                    <span class="lib-card-by" data-card-publisher>{publisherName(pkg)}</span>
                    <span class="lib-card-date">{cardDate(pkg)}</span>
                  </span>
                </button>
              </li>
            {/each}
          </ul>
        {/if}
      </section>
      {#if shelfInstalls.length || removedByPlatform.length}
        <section class="lib-section" data-installed-section>
          <h3 class="lib-h3">Installed on this computer</h3>
          <label class="lib-note lib-update-default" data-update-default>
            Updates for every app:
            <select value={policy.default} disabled={busy} on:change={(e) => setPolicy('', e.currentTarget.value)}>
              <option value="manual">Ask me</option>
              <option value="automatic">Automatic, a day after release</option>
            </select>
          </label>
          <ul class="lib-version-list lib-installed-list">
            {#each shelfInstalls as item (item.install_id || item.release_id)}
              {@const update = updates[item.package]}
              <li class="lib-version" data-installed-row data-package={item.package}>
                <span>{item.package}</span>
                {#if platformLine(installs, item.package)}
                  <span class="lib-note" data-platform-line>{platformLine(installs, item.package)}</span>
                {/if}
                {#if updateLine(update)}
                  <span class="lib-note" data-update-line data-update-state={update.state}>{updateLine(update)}</span>
                {/if}
                {#if update && (update.state === 'available' || update.state === 'needs_approval') && update.view}
                  <button class="lib-btn lib-btn-primary lib-btn-small" data-update-button disabled={busy || !computerId} on:click={() => askUpdate(update)}>
                    {update.state === 'needs_approval' ? 'Review and update' : 'Update'}
                  </button>
                {/if}
                <select class="lib-update-policy" data-update-policy aria-label={'Updates for ' + item.package} value={policyFor(policy, item.package)} disabled={busy} on:change={(e) => setPolicy(item.package, e.currentTarget.value)}>
                  <option value="manual">Ask me</option>
                  <option value="automatic">Automatic</option>
                </select>
                <button class="lib-btn lib-btn-quiet lib-btn-small" data-uninstall-button disabled={busy || !computerId} on:click={() => doUninstall(item)}>
                  Uninstall
                </button>
              </li>
            {/each}
            {#each removedByPlatform as item (item.install_id)}
              <li class="lib-version" data-platform-removed-row data-package={item.package}>
                <span>{item.package}</span>
                <span class="lib-note" data-platform-line>{platformLine(installs, item.package)}</span>
              </li>
            {/each}
          </ul>
        </section>
      {/if}
      {#if setAside.length}
        <section class="lib-section" data-set-aside-section>
          <h3 class="lib-h3">Set aside</h3>
          <p class="lib-note">Apps an earlier change made on this computer. They were not installed when apps moved out of the platform; each says why.</p>
          <ul class="lib-version-list">
            {#each setAside as item (item.pair + ':' + item.package)}
              <li class="lib-version" data-set-aside-row data-package={item.package}>
                <span>{item.package}</span>
                <span class="lib-note" data-set-aside-reason>{item.reason}</span>
                {#if item.installable}
                  <button class="lib-btn lib-btn-primary lib-btn-small" data-set-aside-install disabled={busy || !computerId} on:click={() => doInstallSetAside(item)}>
                    Install
                  </button>
                {/if}
              </li>
            {/each}
          </ul>
        </section>
      {/if}
    {:else}
      <section class="lib-app" data-app-view>
        {#if page.loading}
          <div class="lib-state" data-loading role="status"><span>Loading…</span></div>
        {:else if page.error}
          <div class="lib-state" data-error role="alert">
            <Icon name="warning" size={26} />
            <p class="lib-state-title">This app did not load</p>
            <p class="lib-note">{page.error}</p>
            <button class="lib-btn lib-btn-quiet" data-try-again on:click={() => loadApp(page.entry)}>
              <Icon name="refresh" size={15} /> Try again
            </button>
          </div>
        {:else if page.head}
          <header class="lib-hero">
            <div class="lib-hero-text">
              <h2 class="lib-app-name" data-app-name>{view.name}</h2>
              <p class="lib-app-by" data-app-publisher>{publisherOfRelease(page.head, page.entry)}</p>
              {#if updatedLine(page.head)}<p class="lib-app-date">{updatedLine(page.head)}</p>{/if}
              {#if descriptionOf(page.head)}<p class="lib-app-desc" data-app-desc>{descriptionOf(page.head)}</p>{/if}
            </div>
            <div class="lib-hero-actions">
              {#if working}
                <button class="lib-btn lib-btn-primary" disabled data-working>Working…</button>
              {:else if installed}
                {#if canUpdate}
                  <button class="lib-btn lib-btn-primary" data-install-button disabled={busy || !computerId} on:click={() => askInstall(page.head)}>
                    <Icon name="download" size={16} /> Update
                  </button>
                {:else}
                  <span class="lib-tag lib-tag-ok" data-installed>Installed</span>
                {/if}
                <button class="lib-btn lib-btn-quiet" data-uninstall-button disabled={busy || !computerId} on:click={() => doUninstall(installed)}>Uninstall</button>
              {:else}
                <button class="lib-btn lib-btn-primary" data-install-button disabled={busy || !computerId} on:click={() => askInstall(page.head)}>
                  <Icon name="download" size={16} /> Install
                </button>
              {/if}
            </div>
          </header>

          {#if pageUpdate && updateLine(pageUpdate)}
            <p class="lib-note" data-app-update-line data-update-state={pageUpdate.state}>{updateLine(pageUpdate)}</p>
          {/if}

          {#if elsewhere && !working}
            <p class="lib-note" data-elsewhere>
              Another version of {view.name}, from a different maker, is installed on this computer. Installing this one replaces it.
            </p>
          {/if}

          <div class="lib-gallery" data-gallery>
            <ul class="lib-strip">
              {#each media as item, index (item.url)}
                <li class="lib-strip-item">
                  {#if isVideo(item)}
                    <video src={item.url} controls preload="metadata" playsinline aria-label={'Video of ' + view.name}></video>
                  {:else if missing[item.url]}
                    <span class="lib-thumb-none">Missing</span>
                  {:else}
                    <button class="lib-shot" data-shot aria-label={'Enlarge screenshot ' + (shots.indexOf(item) + 1) + ' of ' + shots.length} on:click={() => openViewer(shots.indexOf(item))}>
                      <img src={item.url} alt={'Screenshot of ' + view.name} loading="lazy" on:error={() => markMissing(item)} />
                    </button>
                  {/if}
                </li>
              {/each}
            </ul>
          </div>

          {#if facts}
            <section class="lib-section" data-app-facts>
              <h3 class="lib-h3">What it is</h3>
              <p class="lib-facts-kind">
                {#if kindName(facts.kind)}<span class="lib-tag" data-app-kind={facts.kind}>{kindName(facts.kind)}</span>{/if}
                <span class="lib-note" data-app-tier={facts.tier}>{tierLine(facts.tier)}</span>
              </p>
              {#if facts.facets.length}
                <ul class="lib-facet-list">
                  {#each facts.facets as name (name)}
                    <li class="lib-tag" data-facet={name}>{facetName(name)}</li>
                  {/each}
                </ul>
              {/if}
              {#if facts.slots.length}
                <p class="lib-note" data-slots>Provides the {facts.slots.map(slotName).join(', ')}.</p>
              {/if}
              {#if displaces.length && !installed}
                <p class="lib-note" data-displaces>Installing it and using it replaces {displaces.join(', and ')}. You can switch back.</p>
              {/if}
              {#if installedUses}
                <p class="lib-note" data-uses>
                  {installedUses.length ? 'Uses these parts of Choir: ' + installedUses.join(', ') + '.' : 'Uses no part of Choir beyond its own window.'}
                </p>
              {/if}

              <h3 class="lib-h3" data-ceiling-title>What it may ask for</h3>
              {#if levelLine(facts.level)}
                <p class="lib-note" data-level={facts.level}>{levelLine(facts.level)}</p>
              {/if}
              {#if !facts.recorded}
                <p class="lib-note" data-ceiling-unrecorded>This version was published before the Library recorded each permission it may ask for.</p>
              {:else}
                {#if facts.ceiling.length}
                  <ul class="lib-ceiling" data-ceiling>
                    {#each ceilingLines(facts) as line, index (index)}
                      <li class="lib-ceiling-line" data-ceiling-grant={line.grant} data-grant-level={line.level}>
                        <span>{line.text}</span>
                        {#if line.level === 'extended'}<span class="lib-tag lib-tag-warn">Needs review</span>{/if}
                      </li>
                    {/each}
                  </ul>
                  {#if hasDeskGrant(facts) && page.head.lease_policy}
                    <p class="lib-note" data-lease-policy>{page.head.lease_policy}</p>
                  {/if}
                {:else}
                  <p class="lib-note" data-ceiling-empty>It asks for no permissions.</p>
                {/if}
              {/if}
            </section>
          {/if}

          <section class="lib-section" data-track-record>
            <h3 class="lib-h3">What the maker promised</h3>
            {#if precommitmentsOf(page.head).length}
              <ul class="lib-precs">
                {#each precommitmentsOf(page.head) as precommit, index (index)}
                  {@const outcome = outcomeLabel(precommit)}
                  <li class="lib-prec" data-precommitment>
                    <span class="lib-prec-q">{precommit.question || 'No question recorded'}</span>
                    <span class="lib-prec-line">
                      <span class="lib-prec-p">{predictionLabel(precommit)}</span>
                      <span class="lib-tag" class:lib-tag-ok={outcome.tone === 'ok'} class:lib-tag-warn={outcome.tone === 'warn'}>{outcome.text}</span>
                    </span>
                  </li>
                {/each}
              </ul>
            {:else}
              <p class="lib-note">The maker made no promises about this version.</p>
            {/if}
            <h3 class="lib-h3" data-maker-record-title>The maker's record on this computer</h3>
            {#if reputation.sentences.length}
              <ul class="lib-rep" data-maker-record>
                {#each reputation.sentences as sentence, index (index)}
                  <li class="lib-rep-line" data-maker-sentence data-n={sentence.n}>
                    <span>{sentence.text}</span>
                    <span class="lib-rep-n" data-maker-n>n = {sentence.n}</span>
                  </li>
                {/each}
              </ul>
            {:else if reputation.failed}
              <p class="lib-note" data-maker-record-error>The maker's record could not be read right now.</p>
            {:else if !reputation.loaded}
              <p class="lib-note" data-maker-record-loading>Reading the maker's record…</p>
            {/if}
          </section>

          {#if page.versions.length > 1}
            <details class="lib-versions" data-versions>
              <summary>Version history ({page.versions.length})</summary>
              <ul class="lib-version-list">
                {#each page.versions as rel (releaseIdOf(rel))}
                  <li class="lib-version" data-version>
                    <span>
                      {versionLabel(rel)}
                      {#if installed && String(installed.release_id) === releaseIdOf(rel)}<span class="lib-tag lib-tag-ok">Installed</span>{/if}
                    </span>
                    {#if !installed || seqOf(rel) > installedSeq}
                      <button class="lib-btn lib-btn-quiet lib-btn-small" data-install-version disabled={busy || !computerId} on:click={() => askInstall(rel)}>
                        Install this version
                      </button>
                    {/if}
                  </li>
                {/each}
              </ul>
              {#if page.next}
                <button class="lib-btn lib-btn-quiet lib-btn-small" data-older on:click={loadOlder}>Show older versions</button>
              {/if}
            </details>
          {/if}
        {/if}
      </section>
    {/if}
  </div>

  {#if confirm}
    <div class="lib-modal" data-confirm role="dialog" aria-modal="true" aria-label={'Install ' + view.name}>
      <div class="lib-veil" role="presentation" on:click={() => (confirm = null)}></div>
      <div class="lib-dialog">
        <h2 class="lib-dialog-title" data-confirm-title>Install {view.name}?</h2>
        <p class="lib-note">
          Made by {publisherOfRelease(confirm, page.entry)}. It will be added to your launcher and run with your
          account's access, so install apps only from people you trust.
        </p>
        {#if confirm.facts && factsOf(confirm).level === 'extended'}
          <p class="lib-note" data-confirm-level>{levelLine('extended')}</p>
        {/if}
        {#if confirmDisplaces.length}
          <p class="lib-note" data-confirm-displaces>It will be used instead of {confirmDisplaces.join(', and ')}.</p>
        {/if}
        {#if confirmReplaces}
          <p class="lib-note" data-confirm-replaces>
            This replaces the {view.name} that is installed now, which is a different maker's app.
          </p>
        {/if}
        {#if identityError}<p class="lib-note">{identityError}</p>{/if}
        <div class="lib-dialog-foot">
          <button class="lib-btn lib-btn-quiet" data-confirm-cancel on:click={() => (confirm = null)}>Cancel</button>
          <button class="lib-btn lib-btn-primary" data-confirm-install disabled={busy || !computerId} on:click={doInstall}>
            {busy ? 'Installing…' : 'Install'}
          </button>
        </div>
      </div>
    </div>
  {/if}

  {#if viewer.open && shots[viewer.index]}
    {@const shot = shots[viewer.index]}
    <div class="lib-modal lib-modal-viewer" data-viewer role="dialog" aria-modal="true" aria-label="Screenshots">
      <div class="lib-veil" role="presentation" on:click={closeViewer}></div>
      <div class="lib-viewer" on:touchstart={onTouchStart} on:touchend={onTouchEnd}>
        <div class="lib-viewer-head">
          <span class="lib-note">{viewer.index + 1} of {shots.length}</span>
          <button class="lib-x" data-viewer-close aria-label="Close" on:click={closeViewer}><Icon name="close" size={18} /></button>
        </div>
        <div class="lib-viewer-stage">
          <img src={shot.url} alt={'Screenshot ' + (viewer.index + 1) + ' of ' + view.name} />
        </div>
        {#if shots.length > 1}
          <div class="lib-viewer-nav">
            <button class="lib-btn lib-btn-quiet" data-viewer-prev aria-label="Previous screenshot" on:click={() => step(-1)}><Icon name="left" size={16} /></button>
            <button class="lib-btn lib-btn-quiet" data-viewer-next aria-label="Next screenshot" on:click={() => step(1)}><Icon name="right" size={16} /></button>
          </div>
        {/if}
      </div>
    </div>
  {/if}
</div>
