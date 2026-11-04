<script>
  // Library: an app store. Every app listed here has screenshots and can be
  // installed on this computer. A card is an app and the person who made it
  // (shown by their account email); an app opens as one page with its
  // screenshots, what it promised and a plain Install button. Nothing here
  // shows an id: the catalog, the install route and the install list are the
  // platform's, read through app-sdk's renewing fetch.
  import { onMount, createEventDispatcher } from 'svelte';
  import { fetchWithRenewal } from '../../lib/app-sdk';
  import Icon from './Icon.svelte';
  import {
    adoptionsOf,
    descriptionOf,
    formatDay,
    inFlightFor,
    installRelease,
    installedFor,
    isVideo,
    loadInstalls,
    loadPackageReleases,
    loadPackages,
    loadRelease,
    mediaOf,
    outcomeLabel,
    plainReason,
    precommitmentsOf,
    predictionLabel,
    releaseIdOf,
    seqOf,
    sortedByNewest,
    versionLabel,
  } from './catalog';
  import './library.css';

  const dispatch = createEventDispatcher();

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

  let page = { entry: null, head: null, versions: [], loading: false, error: '' };
  let pageGeneration = 0;

  let installs = [];
  let busy = false;
  let confirm = null; // the release the owner is asked about
  let message = null; // { tone: 'ok' | 'warn' | 'bad', text }
  let missing = {};
  let viewer = { open: false, index: 0 };
  let touchX = null;

  $: media = page.head ? mediaOf(page.head) : [];
  $: shots = media.filter((item) => !isVideo(item));
  $: installed = view.kind === 'app' ? installedFor(installs, view.name) : null;
  $: working = view.kind === 'app' ? inFlightFor(installs, view.name) : null;
  $: headId = page.head ? releaseIdOf(page.head) : '';
  $: upToDate = Boolean(installed && headId && String(installed.release_id) === headId);

  onMount(() => {
    void boot();
    window.addEventListener('keydown', onKeydown);
    return () => {
      window.removeEventListener('keydown', onKeydown);
      if (searchTimer) clearTimeout(searchTimer);
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
      installs = adoptionsOf(await loadInstalls(computerId));
    } catch (err) {
      installs = [];
      if (err && err.name === 'AuthRequiredError') reportAuth('install_read');
    }
  }

  function cardKey(pkg) {
    return String(pkg.package) + '\n' + String(pkg.track);
  }

  function thumbOf(pkg) {
    const list = Array.isArray(pkg && pkg.head_media) ? pkg.head_media : [];
    return list.find((item) => item && item.url && !isVideo(item)) || null;
  }

  function cardInstalled(pkg) {
    return Boolean(installedFor(installs, pkg.package));
  }

  function publisherName(pkg) {
    return pkg && pkg.publisher_email ? pkg.publisher_email : 'an unnamed publisher';
  }

  // ---- one app ----

  function openApp(pkg) {
    message = null;
    view = { kind: 'app', name: String(pkg.package), track: String(pkg.track || '') };
    page = { entry: pkg, head: null, versions: [], loading: true, error: '' };
    void loadApp(pkg);
  }

  async function loadApp(pkg) {
    const gen = ++pageGeneration;
    try {
      const [head, history] = await Promise.all([
        loadRelease(pkg.head_release_id),
        loadPackageReleases(pkg.package, pkg.track, { limit: 20 }),
      ]);
      if (gen !== pageGeneration) return;
      page = { entry: pkg, head, versions: sortedByNewest(history.releases), loading: false, error: '' };
    } catch (err) {
      if (gen !== pageGeneration) return;
      page = { entry: pkg, head: null, versions: [], loading: false, error: messageOf(err) };
      if (err && err.name === 'AuthRequiredError') reportAuth('release_read');
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
      const done = installedFor(installs, view.kind === 'app' ? view.name : '');
      message = done
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

  async function doUninstall() {
    if (!installed || busy || !computerId) return;
    busy = true;
    try {
      await installRelease(computerId, String(installed.release_id || ''), null, 'uninstall');
      await refreshInstalls();
      message = installedFor(installs, view.name)
        ? { tone: 'ok', text: 'Removing this app from your computer.' }
        : { tone: 'ok', text: 'Uninstalled. It has left the launcher.' };
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
    dispatch('authrequired', { kind, appId: 'library-next', appName: 'Library' });
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

  function publisherOfRelease(rel, fallback) {
    return (rel && rel.publisher_email) || (fallback && fallback.publisher_email) || 'an unnamed publisher';
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
          <ul class="lib-cards">
            {#each packages as pkg (cardKey(pkg))}
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
                      {#if cardInstalled(pkg)}<span class="lib-tag lib-tag-ok" data-card-installed>Installed</span>{/if}
                    </span>
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
              {:else if installed && upToDate}
                <span class="lib-tag lib-tag-ok" data-installed>Installed</span>
                <button class="lib-btn lib-btn-quiet" data-uninstall-button disabled={busy || !computerId} on:click={doUninstall}>Uninstall</button>
              {:else if installed}
                <button class="lib-btn lib-btn-primary" data-install-button disabled={busy || !computerId} on:click={() => askInstall(page.head)}>
                  <Icon name="download" size={16} /> Update
                </button>
                <button class="lib-btn lib-btn-quiet" data-uninstall-button disabled={busy || !computerId} on:click={doUninstall}>Uninstall</button>
              {:else}
                <button class="lib-btn lib-btn-primary" data-install-button disabled={busy || !computerId} on:click={() => askInstall(page.head)}>
                  <Icon name="download" size={16} /> Install
                </button>
              {/if}
            </div>
          </header>

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
                    {#if !(installed && String(installed.release_id) === releaseIdOf(rel))}
                      <button class="lib-btn lib-btn-quiet lib-btn-small" data-install-version disabled={busy || !computerId} on:click={() => askInstall(rel)}>
                        Install this version
                      </button>
                    {/if}
                  </li>
                {/each}
              </ul>
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
