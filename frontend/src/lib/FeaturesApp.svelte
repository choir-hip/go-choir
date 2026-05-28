<script>
  import { createEventDispatcher, onDestroy, onMount } from 'svelte';
  import { AuthRequiredError, fetchWithRenewal } from './auth.js';
  import { addLiveEventListener, liveEventKind } from './live-events.js';

  export let appContext = {};
  export let currentUser = null;

  const dispatch = createEventDispatcher();
  const TARGET_COMPUTER_ID = 'primary';

  let packages = [];
  let adoptions = [];
  let loading = true;
  let error = '';
  let actionError = '';
  let actionStatus = '';
  let selectedPackageId = appContext?.packageId || '';
  let acting = '';
  let removeLiveListener = () => {};

  $: features = packages.map(packageToFeature);
  $: if (!selectedPackageId && features.length > 0) selectedPackageId = features[0].id;
  $: selectedFeature = features.find((feature) => feature.id === selectedPackageId) || features[0] || null;
  $: selectedAdoption = selectedFeature ? latestAdoptionForPackage(selectedFeature.id) : null;
  $: activeCount = adoptions.filter((adoption) => adoption.status === 'adopted').length;
  $: readyCount = adoptions.filter((adoption) => ['verified', 'owner_approved', 'rolled_back'].includes(adoption.status)).length;
  $: availableCount = Math.max(0, features.length - adoptions.length);
  $: ownerEmail = currentUser?.email || 'your signup email';

  function parseRecordJSON(value, fallback = {}) {
    if (!value) return fallback;
    if (typeof value === 'object') return value;
    try {
      return JSON.parse(value);
    } catch {
      return fallback;
    }
  }

  function text(value, fallback = '') {
    const out = String(value || '').trim();
    return out || fallback;
  }

  function safeID(value) {
    return String(value || 'feature')
      .toLowerCase()
      .replace(/[^a-z0-9]+/g, '-')
      .replace(/^-+|-+$/g, '')
      .slice(0, 64) || 'feature';
  }

  function shortRef(value) {
    const out = String(value || '').trim();
    if (!out) return 'pending';
    return out.length > 14 ? out.slice(0, 14) : out;
  }

  function newRunID(prefix, feature) {
    if (globalThis.crypto?.randomUUID) {
      return `${prefix}-${safeID(feature?.id)}-${globalThis.crypto.randomUUID()}`;
    }
    return `${prefix}-${safeID(feature?.id)}-${Date.now()}`;
  }

  function compact(values) {
    const seen = new Set();
    const out = [];
    for (const value of values || []) {
      const item = String(value || '').trim();
      if (!item || seen.has(item)) continue;
      seen.add(item);
      out.push(item);
    }
    return out;
  }

  function collectRefs(value, key = '', refs = { video: [], screenshot: [], narrative: [], benchmark: [], artifact: [] }) {
    if (!value) return refs;
    if (Array.isArray(value)) {
      for (const item of value) collectRefs(item, key, refs);
      return refs;
    }
    if (typeof value === 'object') {
      for (const [nextKey, nextValue] of Object.entries(value)) {
        collectRefs(nextValue, nextKey, refs);
      }
      return refs;
    }
    const raw = String(value || '').trim();
    if (!raw) return refs;
    const lowerKey = String(key || '').toLowerCase();
    const lowerRaw = raw.toLowerCase();
    if (lowerKey.includes('video') || /\.(mp4|webm|mov)(\?|#|$)/.test(lowerRaw)) refs.video.push(raw);
    else if (lowerKey.includes('screenshot') || /\.(png|jpg|jpeg|webp)(\?|#|$)/.test(lowerRaw)) refs.screenshot.push(raw);
    else if (lowerKey.includes('benchmark') || lowerRaw.includes('benchmark')) refs.benchmark.push(raw);
    else if (lowerKey.includes('narrative') || lowerKey.includes('vtext')) refs.narrative.push(raw);
    else refs.artifact.push(raw);
    return refs;
  }

  function refsFromPackage(pkg) {
    const refs = collectRefs(parseRecordJSON(pkg.provenance_refs_json, {}));
    return {
      video: compact(refs.video),
      screenshot: compact(refs.screenshot),
      narrative: compact(refs.narrative),
      benchmark: compact(refs.benchmark),
      artifact: compact(refs.artifact),
    };
  }

  function packageToFeature(pkg) {
    const manifest = parseRecordJSON(pkg.manifest_json, {});
    const refs = refsFromPackage(pkg);
    const title = text(manifest.title || manifest.name || manifest.app_name || pkg.app_id || pkg.package_id, 'Untitled feature');
    const summary = text(
      manifest.summary || manifest.description || manifest.human_summary,
      'A source-level feature that can be rebuilt for this computer.'
    );
    return {
      id: pkg.package_id,
      appId: pkg.app_id || '',
      pkg,
      title,
      summary,
      videoRefs: refs.video,
      screenshotRefs: refs.screenshot,
      narrativeRefs: refs.narrative,
      benchmarkRefs: refs.benchmark,
      artifactRefs: refs.artifact,
      hasDemo: refs.video.length > 0 || refs.screenshot.length > 0,
    };
  }

  function latestAdoptionForPackage(packageID) {
    const matches = adoptions.filter((adoption) => adoption.package_id === packageID);
    if (matches.length === 0) return null;
    return matches.sort((a, b) => new Date(b.updated_at || b.created_at || 0) - new Date(a.updated_at || a.created_at || 0))[0];
  }

  function featureState(feature) {
    const adoption = latestAdoptionForPackage(feature?.id);
    if (!adoption) return 'available';
    if (['adoption_proposed', 'candidate_applied', 'verifying', 'built'].includes(adoption.status)) return 'importing';
    if (['verified', 'owner_approved'].includes(adoption.status)) return 'ready';
    if (adoption.status === 'adopted') return 'active';
    if (adoption.status === 'rolled_back') return 'rolled back';
    if (adoption.status === 'blocked') return 'blocked';
    return String(adoption.status || 'pending').replaceAll('_', ' ');
  }

  function hasRollbackRef(adoption) {
    const rollback = parseRecordJSON(adoption?.rollback_profile_json, {});
    return !!rollback.previous_active_source_ref;
  }

  function canImport(feature) {
    return feature && !latestAdoptionForPackage(feature.id);
  }

  function canActivate(adoption) {
    return adoption && ['verified', 'owner_approved'].includes(adoption.status) && hasRollbackRef(adoption);
  }

  function canRollback(adoption) {
    return adoption && adoption.status === 'adopted' && hasRollbackRef(adoption);
  }

  function canRollForward(adoption) {
    return adoption && adoption.status === 'rolled_back' && adoption.runtime_artifact_digest && adoption.ui_artifact_digest;
  }

  async function fetchJSON(url, options = {}) {
    const res = await fetchWithRenewal(url, {
      ...options,
      headers: {
        ...(options.body ? { 'Content-Type': 'application/json' } : {}),
        ...(options.headers || {}),
      },
    });
    const body = await res.json().catch(() => ({}));
    if (!res.ok) {
      throw new Error(body?.error || `Request failed (${res.status})`);
    }
    return body;
  }

  async function refreshFeatures(preserved = []) {
    loading = true;
    error = '';
    try {
      const [packageBody, adoptionBody] = await Promise.all([
        fetchJSON('/api/app-change-packages?limit=100', { method: 'GET' }),
        fetchJSON('/api/adoptions?limit=100', { method: 'GET' }),
      ]);
      packages = Array.isArray(packageBody?.packages) ? packageBody.packages : [];
      const nextAdoptions = Array.isArray(adoptionBody?.adoptions) ? adoptionBody.adoptions : [];
      adoptions = mergeAdoptions(nextAdoptions, preserved);
    } catch (err) {
      if (err instanceof AuthRequiredError) {
        dispatch('authexpired');
        return;
      }
      error = err.message || 'Features are unavailable';
      packages = [];
      adoptions = [];
    } finally {
      loading = false;
    }
  }

  function mergeAdoptions(nextAdoptions, preserved) {
    const merged = [...nextAdoptions];
    for (const adoption of preserved || []) {
      if (adoption?.adoption_id && !merged.some((item) => item.adoption_id === adoption.adoption_id)) {
        merged.unshift(adoption);
      }
    }
    return merged;
  }

  async function importFeature(feature) {
    if (!feature?.id) return;
    actionError = '';
    acting = `import:${feature.id}`;
    actionStatus = `Importing ${feature.title}. Choir will email ${ownerEmail} when it is ready or blocked.`;
    try {
      const lineage = await fetchJSON(`/api/computers/${encodeURIComponent(TARGET_COMPUTER_ID)}/source-lineage`, { method: 'GET' });
      const adoptionID = newRunID('feature', feature);
      const adoption = await fetchJSON(`/api/computers/${encodeURIComponent(TARGET_COMPUTER_ID)}/adoptions`, {
        method: 'POST',
        body: JSON.stringify({
          adoption_id: adoptionID,
          package_id: feature.id,
          target_candidate_id: `${TARGET_COMPUTER_ID}-feature-${safeID(feature.title)}-${Date.now()}`,
          target_active_source_ref_at_candidate_start: lineage.active_source_ref,
          trace_id: `features-${safeID(feature.title)}`,
        }),
      });
      adoptions = [adoption, ...adoptions.filter((item) => item.adoption_id !== adoption.adoption_id)];
      let verified;
      try {
        verified = await fetchJSON(`/api/adoptions/${encodeURIComponent(adoption.adoption_id)}/verify`, {
          method: 'POST',
          body: JSON.stringify({
            target_active_source_ref_at_cutover: adoption.target_active_source_ref_at_candidate_start,
            foreground_tail_merge_result: adoption.foreground_tail_merge_result || 'no-conflict',
            merge_strategy: adoption.merge_strategy || 'rebase',
          }),
        });
      } catch (verifyErr) {
        await refreshFeatures([adoption]);
        const blocked = latestAdoptionForPackage(feature.id) || { ...adoption, status: 'blocked', error: verifyErr.message };
        adoptions = [blocked, ...adoptions.filter((item) => item.adoption_id !== blocked.adoption_id)];
        actionStatus = `${feature.title} is blocked. We emailed ${ownerEmail} with a concise status link.`;
        await notifyCompletion(feature, blocked);
        throw verifyErr;
      }
      adoptions = [verified, ...adoptions.filter((item) => item.adoption_id !== verified.adoption_id)];
      actionStatus = verified.status === 'verified'
        ? `${feature.title} is ready. We emailed ${ownerEmail}; open Desk to activate or leave it for later.`
        : `${feature.title} finished with status ${featureState(feature)}. We emailed ${ownerEmail}.`;
      await notifyCompletion(feature, verified);
      await refreshFeatures([verified]);
    } catch (err) {
      if (err instanceof AuthRequiredError) {
        dispatch('authexpired');
        return;
      }
      actionError = err.message || 'Import failed';
    } finally {
      acting = '';
    }
  }

  async function runFeatureAction(adoption, action) {
    if (!adoption?.adoption_id) return;
    const feature = features.find((item) => item.id === adoption.package_id);
    actionError = '';
    acting = `${action}:${adoption.adoption_id}`;
    const label = action === 'promote' ? 'Activating' : action === 'rollback' ? 'Rolling back' : 'Rolling forward';
    actionStatus = `${label} ${feature?.title || adoption.app_id || adoption.package_id}`;
    try {
      const next = await fetchJSON(`/api/adoptions/${encodeURIComponent(adoption.adoption_id)}/${action}`, {
        method: 'POST',
        body: action === 'rollback' ? undefined : '{}',
      });
      adoptions = [next, ...adoptions.filter((item) => item.adoption_id !== next.adoption_id)];
      actionStatus = action === 'promote'
        ? 'Activated with rollback available from Desk.'
        : action === 'rollback'
          ? 'Rolled back. Desk can roll forward again.'
          : 'Rolled forward to the verified feature.';
      await refreshFeatures([next]);
    } catch (err) {
      if (err instanceof AuthRequiredError) {
        dispatch('authexpired');
        return;
      }
      actionError = err.message || 'Feature action failed';
    } finally {
      acting = '';
    }
  }

  async function notifyCompletion(feature, adoption) {
    try {
      await fetchWithRenewal('/api/notifications/completion-email', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          to_email: currentUser?.email || '',
          title: feature?.title || adoption?.app_id || adoption?.package_id || 'Feature import',
          status: adoption?.status || 'ready',
          feature_id: feature?.id || adoption?.package_id || '',
          link: '/?app=features',
        }),
      });
    } catch (_err) {
      actionStatus = `${actionStatus} Email notification is queued in product copy but the mail handoff did not complete.`;
    }
  }

  function watchDemo(feature) {
    const url = feature?.videoRefs?.[0] || feature?.screenshotRefs?.[0] || '';
    if (url && /^https?:\/\//.test(url)) {
      window.open(url, '_blank', 'noopener,noreferrer');
      return;
    }
    actionStatus = feature?.hasDemo
      ? 'Demo evidence is available in details.'
      : 'No short demo video has been attached yet.';
  }

  function openTrace(feature, adoption) {
    const traceID = adoption?.trace_id || feature?.pkg?.trace_id || '';
    if (!traceID) {
      actionStatus = 'No trace has been attached yet.';
      return;
    }
    dispatch('opentrace', { traceId: traceID });
  }

  function handleLiveEvent(message) {
    const kind = liveEventKind(message);
    if (
      kind === 'app_change_package.published' ||
      kind === 'app_adoption.proposed' ||
      kind === 'app_adoption.verification_started' ||
      kind === 'app_adoption.verified' ||
      kind === 'app_adoption.blocked' ||
      kind === 'app_adoption.promoted' ||
      kind === 'app_adoption.rolled_back'
    ) {
      void refreshFeatures();
    }
  }

  onMount(() => {
    void refreshFeatures();
    removeLiveListener = addLiveEventListener(handleLiveEvent);
  });

  onDestroy(() => {
    removeLiveListener();
  });
</script>

<section class="features-app" data-features-app>
  <header class="features-header">
    <div>
      <p class="eyebrow">Features</p>
      <h2>Import proven work into your computer</h2>
      <p class="subcopy">Watch the short proof, import once, then let Choir rebuild and verify it in the background. We will email {ownerEmail} when it is ready.</p>
    </div>
    <div class="feature-counts" aria-label="Feature counts">
      <span><strong>{features.length}</strong> catalog</span>
      <span><strong>{readyCount}</strong> ready</span>
      <span><strong>{activeCount}</strong> active</span>
    </div>
  </header>

  {#if error}
    <div class="state-banner error" data-features-error role="alert">{error}</div>
  {/if}
  {#if actionError}
    <div class="state-banner error" data-features-action-error role="alert">{actionError}</div>
  {/if}
  {#if actionStatus}
    <div class="state-banner" data-features-action-status>{actionStatus}</div>
  {/if}

  {#if loading}
    <div class="empty-state" data-features-loading>Loading features...</div>
  {:else if features.length === 0}
    <div class="empty-state" data-features-empty>
      <strong>No features yet</strong>
      <span>Published work appears here after it includes demo evidence and source-level rebuild data.</span>
    </div>
  {:else}
    <div class="features-layout">
      <aside class="catalog" aria-label="Feature catalog">
        <div class="catalog-meta">
          <span>{availableCount} available</span>
          <span>video first</span>
        </div>
        {#each features as feature}
          <button
            class:selected={feature.id === selectedPackageId}
            class="feature-row"
            data-feature-row
            data-feature-id={feature.id}
            on:click={() => (selectedPackageId = feature.id)}
          >
            <span class:has-demo={feature.hasDemo} class="demo-dot"></span>
            <span>
              <strong>{feature.title}</strong>
              <small>{featureState(feature)}</small>
            </span>
          </button>
        {/each}
      </aside>

      {#if selectedFeature}
        <article class="feature-detail" data-feature-detail>
          <div class="demo-panel" data-feature-demo>
            {#if selectedFeature.videoRefs.length > 0}
              <div class="demo-video">
                <span class="play-mark">▶</span>
                <span>Short demo video</span>
              </div>
            {:else if selectedFeature.screenshotRefs.length > 0}
              <div class="demo-video screenshot">
                <span class="play-mark">▣</span>
                <span>Screenshot proof</span>
              </div>
            {:else}
              <div class="demo-video missing">
                <span class="play-mark">?</span>
                <span>No demo video yet</span>
              </div>
            {/if}
          </div>

          <div class="detail-main">
            <div class="detail-title">
              <div>
                <p class="eyebrow">Catalog proof</p>
                <h3>{selectedFeature.title}</h3>
              </div>
              <span class="state-pill" data-feature-state>{featureState(selectedFeature)}</span>
            </div>
            <p class="summary">{selectedFeature.summary}</p>
            <p class="email-copy" data-feature-email-copy>Imports finish in the background. Choir will email {ownerEmail} when ready or blocked.</p>

            <div class="primary-actions" data-feature-actions>
              <button class="secondary-action" data-feature-watch-demo on:click={() => watchDemo(selectedFeature)}>
                Watch demo
              </button>
              {#if canImport(selectedFeature)}
                <button
                  class="primary-action"
                  data-feature-import
                  on:click={() => importFeature(selectedFeature)}
                  disabled={!!acting}
                >
                  {acting === `import:${selectedFeature.id}` ? 'Importing...' : 'Import'}
                </button>
              {/if}
              {#if canActivate(selectedAdoption)}
                <button
                  class="primary-action"
                  data-feature-activate
                  on:click={() => runFeatureAction(selectedAdoption, 'promote')}
                  disabled={!!acting}
                >
                  {acting === `promote:${selectedAdoption.adoption_id}` ? 'Activating...' : 'Activate'}
                </button>
              {/if}
              {#if canRollback(selectedAdoption)}
                <button
                  class="danger-action"
                  data-feature-rollback
                  on:click={() => runFeatureAction(selectedAdoption, 'rollback')}
                  disabled={!!acting}
                >
                  {acting === `rollback:${selectedAdoption.adoption_id}` ? 'Rolling back...' : 'Roll back'}
                </button>
              {/if}
              {#if canRollForward(selectedAdoption)}
                <button
                  class="primary-action"
                  data-feature-roll-forward
                  on:click={() => runFeatureAction(selectedAdoption, 'roll-forward')}
                  disabled={!!acting}
                >
                  {acting === `roll-forward:${selectedAdoption.adoption_id}` ? 'Rolling forward...' : 'Roll forward'}
                </button>
              {/if}
              <button class="secondary-action" data-feature-later on:click={() => (actionStatus = 'Saved for later.')}>Later</button>
            </div>

            {#if selectedAdoption?.error}
              <div class="state-banner error" data-feature-blocker>{selectedAdoption.error}</div>
            {/if}

            <details class="technical-details" data-feature-evidence-details>
              <summary>View details</summary>
              <dl>
                <div><dt>Source package</dt><dd>{selectedFeature.id}</dd></div>
                <div><dt>Import record</dt><dd>{selectedAdoption?.adoption_id || 'not imported'}</dd></div>
                <div><dt>Build ref</dt><dd>{shortRef(selectedAdoption?.candidate_source_ref || selectedFeature.pkg.candidate_source_ref)}</dd></div>
                <div><dt>Runtime digest</dt><dd>{shortRef(selectedAdoption?.runtime_artifact_digest || selectedFeature.pkg.source_runtime_artifact_digest)}</dd></div>
                <div><dt>UI digest</dt><dd>{shortRef(selectedAdoption?.ui_artifact_digest || selectedFeature.pkg.source_ui_artifact_digest)}</dd></div>
                <div><dt>Rollback</dt><dd>{hasRollbackRef(selectedAdoption) ? 'recorded' : 'pending'}</dd></div>
              </dl>
              <div class="evidence-refs">
                {#each selectedFeature.videoRefs as ref}
                  <span>video: {ref}</span>
                {/each}
                {#each selectedFeature.screenshotRefs as ref}
                  <span>screenshot: {ref}</span>
                {/each}
                {#each selectedFeature.benchmarkRefs as ref}
                  <span>benchmark: {ref}</span>
                {/each}
              </div>
              <button class="secondary-action compact" on:click={() => openTrace(selectedFeature, selectedAdoption)}>Open Trace</button>
            </details>
          </div>
        </article>
      {/if}
    </div>
  {/if}
</section>

<style>
  .features-app {
    height: 100%;
    min-height: 0;
    overflow: auto;
    padding: clamp(1rem, 2vw, 1.4rem);
    color: #e5eefc;
    background:
      radial-gradient(circle at 16% 4%, rgba(59, 130, 246, 0.16), transparent 28%),
      linear-gradient(135deg, #060b12 0%, #0a121f 48%, #10151d 100%);
  }

  .features-header {
    display: flex;
    justify-content: space-between;
    gap: 1rem;
    align-items: flex-start;
    margin-bottom: 0.9rem;
  }

  .eyebrow,
  h2,
  h3,
  p {
    margin: 0;
  }

  .eyebrow {
    color: #93c5fd;
    font-size: 0.72rem;
    font-weight: 850;
    letter-spacing: 0.14em;
    text-transform: uppercase;
  }

  h2 {
    margin-top: 0.2rem;
    font-size: clamp(1.35rem, 3vw, 2.2rem);
    letter-spacing: 0;
  }

  h3 {
    margin-top: 0.2rem;
    font-size: clamp(1.15rem, 2.2vw, 1.7rem);
    letter-spacing: 0;
  }

  .subcopy,
  .summary,
  .email-copy {
    max-width: 52rem;
    color: #a9b7cb;
    line-height: 1.5;
  }

  .email-copy {
    color: #bfdbfe;
    font-size: 0.9rem;
  }

  .feature-counts {
    display: flex;
    flex-wrap: wrap;
    justify-content: flex-end;
    gap: 0.45rem;
    min-width: 14rem;
  }

  .feature-counts span,
  .state-pill {
    border: 1px solid rgba(147, 197, 253, 0.22);
    border-radius: 999px;
    background: rgba(15, 23, 42, 0.72);
    color: #cfe1ff;
    padding: 0.35rem 0.55rem;
    font-size: 0.78rem;
    font-weight: 800;
    white-space: nowrap;
  }

  .state-banner {
    border: 1px solid rgba(96, 165, 250, 0.26);
    border-radius: 10px;
    background: rgba(37, 99, 235, 0.14);
    color: #dbeafe;
    padding: 0.7rem 0.85rem;
    margin-bottom: 0.75rem;
  }

  .state-banner.error {
    border-color: rgba(248, 113, 113, 0.35);
    background: rgba(127, 29, 29, 0.38);
    color: #fecaca;
  }

  .empty-state {
    display: grid;
    gap: 0.3rem;
    border: 1px solid rgba(148, 163, 184, 0.15);
    border-radius: 12px;
    background: rgba(15, 23, 42, 0.52);
    padding: 1rem;
    color: #a8b3c7;
  }

  .features-layout {
    display: grid;
    grid-template-columns: minmax(14rem, 0.38fr) minmax(0, 1fr);
    gap: 0.9rem;
    min-height: 0;
  }

  .catalog,
  .feature-detail {
    border: 1px solid rgba(148, 163, 184, 0.14);
    border-radius: 12px;
    background: rgba(8, 13, 24, 0.72);
  }

  .catalog {
    display: grid;
    align-content: start;
    gap: 0.35rem;
    padding: 0.65rem;
  }

  .catalog-meta {
    display: flex;
    justify-content: space-between;
    gap: 0.5rem;
    color: #93a4ba;
    font-size: 0.72rem;
    font-weight: 760;
    padding: 0.15rem 0.25rem 0.45rem;
  }

  .feature-row {
    display: grid;
    grid-template-columns: 0.75rem minmax(0, 1fr);
    gap: 0.65rem;
    align-items: center;
    width: 100%;
    border: 1px solid transparent;
    border-radius: 10px;
    background: transparent;
    color: #e5eefc;
    cursor: pointer;
    padding: 0.7rem;
    text-align: left;
  }

  .feature-row:hover,
  .feature-row.selected {
    border-color: rgba(96, 165, 250, 0.38);
    background: rgba(30, 64, 175, 0.24);
  }

  .demo-dot {
    width: 0.55rem;
    height: 0.55rem;
    border-radius: 50%;
    background: #64748b;
  }

  .demo-dot.has-demo {
    background: #22c55e;
    box-shadow: 0 0 0 0.24rem rgba(34, 197, 94, 0.12);
  }

  .feature-row span:last-child {
    display: grid;
    gap: 0.16rem;
    min-width: 0;
  }

  .feature-row strong,
  .feature-row small {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .feature-row small {
    color: #93a4ba;
    font-size: 0.76rem;
  }

  .feature-detail {
    display: grid;
    grid-template-columns: minmax(16rem, 0.52fr) minmax(0, 1fr);
    min-height: 31rem;
    overflow: hidden;
  }

  .demo-panel {
    min-height: 100%;
    background: #020617;
    padding: clamp(0.8rem, 2vw, 1rem);
  }

  .demo-video {
    display: grid;
    place-items: center;
    gap: 0.7rem;
    height: 100%;
    min-height: 18rem;
    border: 1px solid rgba(96, 165, 250, 0.3);
    border-radius: 10px;
    background:
      linear-gradient(135deg, rgba(37, 99, 235, 0.18), transparent),
      repeating-linear-gradient(90deg, rgba(148, 163, 184, 0.06) 0 1px, transparent 1px 34px),
      #07111f;
    color: #dbeafe;
    text-align: center;
    font-weight: 850;
  }

  .demo-video.missing {
    border-color: rgba(148, 163, 184, 0.18);
    color: #94a3b8;
  }

  .play-mark {
    display: grid;
    place-items: center;
    width: 4.2rem;
    height: 4.2rem;
    border: 1px solid rgba(147, 197, 253, 0.4);
    border-radius: 50%;
    background: rgba(15, 23, 42, 0.72);
    font-size: 1.45rem;
  }

  .detail-main {
    display: grid;
    align-content: start;
    gap: 0.9rem;
    padding: clamp(1rem, 2vw, 1.25rem);
    min-width: 0;
  }

  .detail-title {
    display: flex;
    justify-content: space-between;
    gap: 1rem;
    align-items: flex-start;
  }

  .primary-actions {
    display: flex;
    flex-wrap: wrap;
    gap: 0.55rem;
  }

  button {
    font: inherit;
  }

  .primary-action,
  .secondary-action,
  .danger-action {
    min-height: 2.35rem;
    border-radius: 9px;
    cursor: pointer;
    padding: 0.55rem 0.8rem;
    font-size: 0.84rem;
    font-weight: 850;
  }

  .primary-action {
    border: 1px solid rgba(96, 165, 250, 0.55);
    background: rgba(37, 99, 235, 0.62);
    color: white;
  }

  .secondary-action {
    border: 1px solid rgba(148, 163, 184, 0.22);
    background: rgba(15, 23, 42, 0.7);
    color: #e5eefc;
  }

  .secondary-action.compact {
    width: fit-content;
  }

  .danger-action {
    border: 1px solid rgba(248, 113, 113, 0.42);
    background: rgba(127, 29, 29, 0.36);
    color: #fecaca;
  }

  .primary-action:disabled,
  .secondary-action:disabled,
  .danger-action:disabled {
    opacity: 0.55;
    cursor: not-allowed;
  }

  .technical-details {
    border-top: 1px solid rgba(148, 163, 184, 0.14);
    padding-top: 0.85rem;
  }

  .technical-details summary {
    color: #bfdbfe;
    cursor: pointer;
    font-weight: 850;
  }

  dl {
    display: grid;
    gap: 0.42rem;
    margin: 0.8rem 0 0;
  }

  dl div {
    display: grid;
    grid-template-columns: minmax(7rem, 0.28fr) minmax(0, 1fr);
    gap: 0.7rem;
  }

  dt {
    color: #94a3b8;
    font-size: 0.76rem;
  }

  dd {
    margin: 0;
    color: #e2e8f0;
    font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
    font-size: 0.78rem;
    overflow-wrap: anywhere;
  }

  .evidence-refs {
    display: grid;
    gap: 0.24rem;
    margin: 0.8rem 0;
    color: #a8b3c7;
    font-size: 0.78rem;
    overflow-wrap: anywhere;
  }

  @media (max-width: 780px) {
    .features-header {
      display: grid;
    }

    .feature-counts {
      justify-content: flex-start;
      min-width: 0;
    }

    .features-layout,
    .feature-detail {
      grid-template-columns: 1fr;
    }

    .catalog {
      max-height: 17rem;
      overflow: auto;
    }

    .demo-video {
      min-height: 14rem;
    }

    .detail-title {
      display: grid;
    }

    dl div {
      grid-template-columns: 1fr;
      gap: 0.12rem;
    }
  }
</style>
