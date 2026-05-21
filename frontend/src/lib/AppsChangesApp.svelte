<script>
  import { createEventDispatcher, onDestroy, onMount } from 'svelte';
  import { AuthRequiredError, fetchWithRenewal } from './auth.js';
  import { addLiveEventListener, liveEventKind } from './live-events.js';
  import ChangePreviewFrame from './ChangePreviewFrame.svelte';

  export let appContext = {};

  const dispatch = createEventDispatcher();

  const TARGET_COMPUTER_ID = 'primary';
  const SEED_CHANGES = [
    {
      id: 'chiron-shelf',
      name: 'Chiron Shelf Observability',
      family: 'Shell',
      packageId: '28433c19-5d02-416f-9368-de56390e1927',
      sourceOwnerId: '80e6da5b-9394-4ebd-8aee-a531927221c7',
      sourceComputerId: 'primary',
      status: 'reviewable',
      summary: 'Streams tool calls and interim agent progress through the Shelf without blocking Desk controls.',
      proof: 'First end-to-end payload for Apps & Changes install, rollback, and review evidence.',
      evidence: ['Wave 1 trace clip', 'Shelf interaction screenshots', 'Run acceptance refs'],
    },
    {
      id: 'motion-language',
      name: 'Process Animation Language',
      family: 'Motion',
      packageId: '98b98c73-eef0-4a88-a6f5-b7dfe695be09',
      sourceOwnerId: '80e6da5b-9394-4ebd-8aee-a531927221c7',
      sourceComputerId: 'primary',
      status: 'benchmark-needed',
      summary: 'Adds expressive loading and process motion patterns for agent work without hiding state.',
      proof: 'Requires Playwright video evidence before promotion recommendation.',
      evidence: ['Wave 1 source desktop screenshot', 'Trace-derived video clip'],
    },
    {
      id: 'liquid-material',
      name: 'Choir Liquid Material Engine',
      family: 'Visual System',
      packageId: '1dad3dfc-7f83-4b22-bfb5-7f1714159f66',
      sourceOwnerId: 'e1842324-90e5-4dfa-b9f1-64db95a46744',
      sourceComputerId: 'primary',
      status: 'benchmark-needed',
      summary: 'Explores bounded liquid glass materials with explicit privacy and resource constraints.',
      proof: 'Needs real GPU/resource benchmark before any platform recommendation.',
      evidence: ['Wave 2 source desktop screenshot', 'Liquid resource benchmark pending'],
    },
    {
      id: 'python-code-mode',
      name: 'Python Code Mode',
      family: 'Code Execution',
      packageId: 'f31edbc8-1b43-44f5-82a1-834dce4833ca',
      sourceOwnerId: 'e1842324-90e5-4dfa-b9f1-64db95a46744',
      sourceComputerId: 'primary',
      status: 'benchmark-needed',
      summary: 'Tests replacing bash-style coding loops with a Python-oriented mode for code work.',
      proof: 'Needs token and latency A/B against the current bash tool path.',
      evidence: ['Wave 2 source desktop screenshot', 'Python mode benchmark pending'],
    },
  ];

  let selectedChangeId = appContext?.changeId || SEED_CHANGES[0].id;
  let packages = [];
  let adoptions = [];
  let loading = true;
  let error = '';
  let actionError = '';
  let actionStatus = '';
  let acting = '';
  let previewCandidateId = '';
  let removeLiveListener = () => {};

  $: selectedChange = SEED_CHANGES.find((change) => change.id === selectedChangeId) || SEED_CHANGES[0];
  $: selectedPackage = selectedChange?.packageId
    ? packages.find((pkg) => pkg.package_id === selectedChange.packageId) || null
    : null;
  $: selectedAdoption = selectedChange?.packageId
    ? adoptions.find((adoption) => adoption.package_id === selectedChange.packageId) || null
    : null;
  $: selectedPreviewId = previewCandidateId || selectedAdoption?.target_candidate_id || '';
  $: installedAdoptions = adoptions.filter((adoption) => adoption.status === 'adopted');
  $: reviewAdoptions = adoptions.filter((adoption) => adoption.status !== 'adopted');

  function packageForChange(change) {
    if (!change?.packageId) return null;
    return packages.find((pkg) => pkg.package_id === change.packageId) || null;
  }

  function latestAdoptionForPackage(packageId) {
    if (!packageId) return null;
    return adoptions.find((adoption) => adoption.package_id === packageId) || null;
  }

  function shortRef(value) {
    if (!value) return 'pending';
    const text = String(value);
    return text.length > 14 ? text.slice(0, 14) : text;
  }

  function statusLabel(change) {
    const adoption = latestAdoptionForPackage(change?.packageId);
    if (adoption?.status === 'adopted') return 'installed';
    if (adoption?.status === 'verified') return 'verified';
    if (adoption?.status === 'blocked') return 'blocked';
    if (adoption?.status) return adoption.status.replaceAll('_', ' ');
    if (packageForChange(change)) return 'pulled';
    return change?.status || 'available';
  }

  function canVerify(adoption) {
    return adoption && ['adoption_proposed', 'candidate_applied', 'blocked'].includes(adoption.status);
  }

  function canInstall(adoption) {
    return adoption && ['verified', 'owner_approved'].includes(adoption.status);
  }

  function canRollback(adoption) {
    return adoption && ['verified', 'adopted', 'blocked'].includes(adoption.status);
  }

  function actionKey(id, action) {
    return `${id}:${action}`;
  }

  function safeID(value) {
    return String(value || '')
      .toLowerCase()
      .replace(/[^a-z0-9]+/g, '-')
      .replace(/^-+|-+$/g, '')
      .slice(0, 48) || 'change';
  }

  function newRunID(prefix, change) {
    if (globalThis.crypto?.randomUUID) {
      return `${prefix}-${safeID(change?.id)}-${globalThis.crypto.randomUUID()}`;
    }
    return `${prefix}-${safeID(change?.id)}-${Date.now()}`;
  }

  async function fetchJSON(path, options = {}) {
    const res = await fetchWithRenewal(path, options);
    const body = await res.json().catch(() => ({}));
    if (!res.ok) {
      throw new Error(body?.error || `${path} failed (${res.status})`);
    }
    return body;
  }

  function mergePreservedAdoptions(nextAdoptions, preservedAdoptions = []) {
    const merged = Array.isArray(nextAdoptions) ? [...nextAdoptions] : [];
    for (const adoption of preservedAdoptions) {
      if (!adoption?.adoption_id) continue;
      if (!merged.some((item) => item.adoption_id === adoption.adoption_id)) {
        merged.unshift(adoption);
      }
    }
    return merged;
  }

  async function refreshCatalog(preservedAdoptions = []) {
    loading = true;
    error = '';
    try {
      const [packageBody, adoptionBody] = await Promise.all([
        fetchJSON('/api/app-change-packages?limit=100', { method: 'GET' }),
        fetchJSON('/api/adoptions?limit=100', { method: 'GET' }),
      ]);
      packages = Array.isArray(packageBody?.packages) ? packageBody.packages : [];
      const nextAdoptions = Array.isArray(adoptionBody?.adoptions) ? adoptionBody.adoptions : [];
      adoptions = mergePreservedAdoptions(nextAdoptions, preservedAdoptions);
    } catch (err) {
      if (err instanceof AuthRequiredError) {
        dispatch('authexpired');
        return;
      }
      error = err.message || 'Apps & Changes is unavailable';
      packages = [];
      adoptions = [];
    } finally {
      loading = false;
    }
  }

  function selectChange(change) {
    selectedChangeId = change.id;
    const adoption = latestAdoptionForPackage(change.packageId);
    previewCandidateId = adoption?.target_candidate_id || '';
    actionError = '';
  }

  async function tryChange(change) {
    if (!change?.packageId) return;
    const existing = latestAdoptionForPackage(change.packageId);
    if (existing) {
      selectedChangeId = change.id;
      previewCandidateId = existing.target_candidate_id || '';
      return;
    }
    actionError = '';
    actionStatus = `Preparing a candidate preview for ${change.name}`;
    acting = actionKey(change.id, 'try');
    try {
      await fetchJSON('/api/app-change-packages/pull', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          package_id: change.packageId,
          source_owner_id: change.sourceOwnerId,
          source_desktop_id: change.sourceComputerId,
          target_desktop_id: TARGET_COMPUTER_ID,
        }),
      });
      const lineage = await fetchJSON(`/api/computers/${encodeURIComponent(TARGET_COMPUTER_ID)}/source-lineage`, {
        method: 'GET',
      });
      const targetCandidateId = newRunID('candidate', change);
      const adoptionID = newRunID('adoption', change);
      const adoption = await fetchJSON(`/api/computers/${encodeURIComponent(TARGET_COMPUTER_ID)}/adoptions`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          adoption_id: adoptionID,
          package_id: change.packageId,
          target_candidate_id: targetCandidateId,
          candidate_source_ref: `refs/computers/${TARGET_COMPUTER_ID}/candidates/${targetCandidateId}`,
          foreground_tail_merge_result: 'pending-recipient-review',
          merge_strategy: 'rebase',
          trace_id: `apps-changes-${safeID(change.id)}`,
        }),
      });
      adoptions = [adoption, ...adoptions.filter((item) => item.adoption_id !== adoption.adoption_id)];
      previewCandidateId = adoption.target_candidate_id || targetCandidateId;
      actionStatus = `Candidate preview is ready from ${shortRef(lineage.active_source_ref)}`;
      await refreshCatalog([adoption]);
    } catch (err) {
      if (err instanceof AuthRequiredError) {
        dispatch('authexpired');
        return;
      }
      actionError = err.message || 'Could not prepare candidate preview';
    } finally {
      acting = '';
    }
  }

  async function runAdoptionAction(adoption, action) {
    if (!adoption?.adoption_id) return;
    actionError = '';
    actionStatus = `${action === 'verify' ? 'Verifying recipient build' : action === 'promote' ? 'Installing change' : 'Rolling back change'} for ${adoption.app_id || 'change'}`;
    acting = actionKey(adoption.adoption_id, action);
    try {
      const payload = action === 'verify'
        ? {
            target_active_source_ref_at_cutover: adoption.target_active_source_ref_at_candidate_start,
            foreground_tail_merge_result: adoption.foreground_tail_merge_result || 'no-conflict',
            merge_strategy: adoption.merge_strategy || 'rebase',
            merge_conflicts: [],
          }
        : {};
      const next = await fetchJSON(`/api/adoptions/${encodeURIComponent(adoption.adoption_id)}/${action}`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload),
      });
      adoptions = [next, ...adoptions.filter((item) => item.adoption_id !== next.adoption_id)];
      previewCandidateId = next.target_candidate_id || previewCandidateId;
      actionStatus = action === 'promote'
        ? 'Installed into the active computer with rollback evidence'
        : action === 'rollback'
          ? 'Rolled back to the previous active source ref'
          : 'Recipient build verified';
      await refreshCatalog();
    } catch (err) {
      if (err instanceof AuthRequiredError) {
        dispatch('authexpired');
        return;
      }
      actionError = err.message || `Could not ${action} adoption`;
      await refreshCatalog();
    } finally {
      acting = '';
    }
  }

  onMount(() => {
    void refreshCatalog();
    removeLiveListener = addLiveEventListener((message) => {
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
        void refreshCatalog();
      }
    });
  });

  onDestroy(() => {
    removeLiveListener();
  });
</script>

<section class="apps-changes" data-apps-changes-app>
  <header class="store-hero">
    <div>
      <p class="eyebrow">Apps & Changes</p>
      <h2>Pull useful changes into this computer</h2>
      <p>Review source-level changes, try them in a candidate, then install only after recipient build verification.</p>
    </div>
    <div class="hero-meter" data-apps-changes-count>
      <strong>{SEED_CHANGES.length}</strong>
      <span>reviewable changes</span>
    </div>
  </header>

  {#if error}
    <div class="state-banner error" data-apps-changes-error role="alert">{error}</div>
  {:else if actionError}
    <div class="state-banner error" data-apps-changes-action-error role="alert">{actionError}</div>
  {:else if actionStatus}
    <div class="state-banner" data-apps-changes-action-status>{actionStatus}</div>
  {/if}

  <div class="store-layout">
    <aside class="change-catalog" data-change-catalog>
      <div class="section-heading">
        <strong>Experiment changes</strong>
        <span>{loading ? 'syncing' : `${packages.length} pulled`}</span>
      </div>
      <div class="change-list">
        {#each SEED_CHANGES as change}
          <button
            class:active={selectedChangeId === change.id}
            class="change-card"
            data-change-card
            data-change-id={change.id}
            on:click={() => selectChange(change)}
          >
            <span class="change-family">{change.family}</span>
            <strong>{change.name}</strong>
            <span>{change.summary}</span>
            <em>{statusLabel(change)}</em>
          </button>
        {/each}
      </div>

      <section class="installed-ledger" data-installed-ledger>
        <div class="section-heading">
          <strong>Installed</strong>
          <span>{installedAdoptions.length}</span>
        </div>
        {#if installedAdoptions.length === 0}
          <p>No installed changes yet.</p>
        {:else}
          {#each installedAdoptions as adoption}
            <button class="ledger-row" on:click={() => selectChange(SEED_CHANGES.find((change) => change.packageId === adoption.package_id) || selectedChange)}>
              <strong>{adoption.app_id || adoption.package_id}</strong>
              <span>{shortRef(adoption.runtime_artifact_digest)} · {shortRef(adoption.ui_artifact_digest)}</span>
            </button>
          {/each}
        {/if}
      </section>
    </aside>

    <main class="change-detail" data-change-detail>
      <section class="detail-card">
        <div class="detail-top">
          <div>
            <p class="eyebrow">{selectedChange.family}</p>
            <h3>{selectedChange.name}</h3>
            <p>{selectedChange.summary}</p>
          </div>
          <span class="status-pill" data-change-status>{statusLabel(selectedChange)}</span>
        </div>

        <p class="proof-text">{selectedChange.proof}</p>

        <div class="evidence-strip" data-change-evidence>
          {#each selectedChange.evidence as item}
            <span>{item}</span>
          {/each}
          <span>VText report pending</span>
        </div>

        <div class="change-actions" data-change-actions>
          <button
            class="primary-action"
            data-change-try
            on:click={() => tryChange(selectedChange)}
            disabled={!!acting || !!selectedAdoption}
          >
            {acting === actionKey(selectedChange.id, 'try') ? 'Preparing…' : selectedAdoption ? 'Candidate prepared' : 'Try in candidate'}
          </button>
          <button
            class="secondary-action"
            data-change-verify
            on:click={() => runAdoptionAction(selectedAdoption, 'verify')}
            disabled={!canVerify(selectedAdoption) || !!acting}
          >
            {selectedAdoption && acting === actionKey(selectedAdoption.adoption_id, 'verify') ? 'Verifying…' : 'Verify build'}
          </button>
          <button
            class="install-action"
            data-change-install
            on:click={() => runAdoptionAction(selectedAdoption, 'promote')}
            disabled={!canInstall(selectedAdoption) || !!acting}
          >
            {selectedAdoption && acting === actionKey(selectedAdoption.adoption_id, 'promote') ? 'Installing…' : 'Install'}
          </button>
          <button
            class="danger-action"
            data-change-rollback
            on:click={() => runAdoptionAction(selectedAdoption, 'rollback')}
            disabled={!canRollback(selectedAdoption) || !!acting}
          >
            {selectedAdoption && acting === actionKey(selectedAdoption.adoption_id, 'rollback') ? 'Rolling back…' : 'Rollback'}
          </button>
        </div>

        <div class="candidate-summary" data-change-candidate-summary>
          <div>
            <span>candidate</span>
            <strong>{selectedAdoption?.target_candidate_id || 'not tried'}</strong>
          </div>
          <div>
            <span>runtime</span>
            <strong>{shortRef(selectedAdoption?.runtime_artifact_digest)}</strong>
          </div>
          <div>
            <span>UI</span>
            <strong>{shortRef(selectedAdoption?.ui_artifact_digest)}</strong>
          </div>
          <div>
            <span>rollback</span>
            <strong>{selectedAdoption?.rollback_profile_json ? 'recorded' : 'pending'}</strong>
          </div>
        </div>

        <details class="technical-details" data-change-technical-details>
          <summary>Technical refs</summary>
          <dl>
            <div><dt>Package</dt><dd>{selectedChange.packageId}</dd></div>
            <div><dt>Source owner</dt><dd>{selectedChange.sourceOwnerId}</dd></div>
            <div><dt>Manifest hash</dt><dd>{selectedPackage?.package_manifest_sha256 || 'not pulled'}</dd></div>
            <div><dt>Adoption</dt><dd>{selectedAdoption?.adoption_id || 'not created'}</dd></div>
            <div><dt>Candidate ref</dt><dd>{selectedAdoption?.candidate_source_ref || 'not created'}</dd></div>
          </dl>
        </details>
      </section>

      <section class="preview-card" data-change-preview>
        <div class="preview-heading">
          <div>
            <strong>Candidate preview</strong>
            <span>Try opens a candidate computer; Install is the active-state transition.</span>
          </div>
          <span>{selectedPreviewId ? 'candidate' : 'empty'}</span>
        </div>
        <ChangePreviewFrame
          candidateDesktopId={selectedPreviewId}
          title={`${selectedChange.name} candidate preview`}
        />
      </section>

      <section class="review-ledger" data-review-ledger>
        <div class="section-heading">
          <strong>Review queue</strong>
          <span>{reviewAdoptions.length}</span>
        </div>
        {#if reviewAdoptions.length === 0}
          <p>No candidate reviews yet.</p>
        {:else}
          {#each reviewAdoptions as adoption}
            <article class="review-row" data-review-adoption-id={adoption.adoption_id}>
              <div>
                <strong>{adoption.app_id || adoption.package_id}</strong>
                <span>{adoption.status} · {adoption.target_computer_id}</span>
              </div>
              {#if adoption.error}
                <p>{adoption.error}</p>
              {/if}
            </article>
          {/each}
        {/if}
      </section>
    </main>
  </div>
</section>

<style>
  .apps-changes {
    display: flex;
    flex-direction: column;
    height: 100%;
    min-height: 0;
    overflow: hidden;
    background:
      radial-gradient(circle at 18% 0%, rgba(34, 211, 238, 0.12), transparent 34%),
      linear-gradient(135deg, #07111e 0%, #0b1020 55%, #0a0d17 100%);
    color: #e5f0ff;
  }

  .store-hero {
    display: flex;
    justify-content: space-between;
    gap: 18px;
    padding: 18px 20px;
    border-bottom: 1px solid rgba(148, 163, 184, 0.18);
  }

  .eyebrow {
    margin: 0 0 6px;
    color: #67e8f9;
    font-size: 0.74rem;
    font-weight: 800;
    letter-spacing: 0.12em;
    text-transform: uppercase;
  }

  h2,
  h3,
  p {
    margin: 0;
  }

  h2 {
    font-size: clamp(1.55rem, 2.2vw, 2.25rem);
    line-height: 1.08;
  }

  h3 {
    font-size: 1.35rem;
  }

  .store-hero p,
  .detail-card p,
  .preview-heading span,
  .section-heading span,
  .change-card span,
  .change-card em,
  .installed-ledger p,
  .review-ledger p,
  .review-row span,
  .candidate-summary span {
    color: #9fb1c9;
  }

  .hero-meter,
  .status-pill {
    align-self: flex-start;
    padding: 10px 12px;
    border: 1px solid rgba(96, 165, 250, 0.28);
    border-radius: 8px;
    background: rgba(15, 23, 42, 0.76);
  }

  .hero-meter strong {
    display: block;
    font-size: 1.35rem;
  }

  .hero-meter span,
  .status-pill {
    color: #bae6fd;
    font-size: 0.78rem;
    font-weight: 800;
    text-transform: uppercase;
  }

  .state-banner {
    margin: 12px 16px 0;
    padding: 10px 12px;
    border: 1px solid rgba(34, 211, 238, 0.22);
    border-radius: 8px;
    background: rgba(8, 47, 73, 0.58);
    color: #dff7ff;
  }

  .state-banner.error {
    border-color: rgba(248, 113, 113, 0.35);
    background: rgba(69, 10, 10, 0.54);
    color: #fecaca;
  }

  .store-layout {
    display: grid;
    grid-template-columns: minmax(260px, 340px) minmax(0, 1fr);
    gap: 16px;
    min-height: 0;
    padding: 16px;
    overflow: hidden;
  }

  .change-catalog,
  .change-detail {
    min-height: 0;
    overflow: auto;
  }

  .change-catalog,
  .detail-card,
  .preview-card,
  .review-ledger {
    border: 1px solid rgba(148, 163, 184, 0.18);
    border-radius: 8px;
    background: rgba(15, 23, 42, 0.72);
    box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.04);
  }

  .change-catalog {
    padding: 12px;
  }

  .section-heading,
  .detail-top,
  .preview-heading,
  .candidate-summary {
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    gap: 12px;
  }

  .change-list {
    display: grid;
    gap: 10px;
    margin-top: 12px;
  }

  .change-card,
  .ledger-row {
    width: 100%;
    border: 1px solid rgba(96, 165, 250, 0.16);
    border-radius: 8px;
    background: rgba(2, 6, 23, 0.52);
    color: #dbeafe;
    text-align: left;
    cursor: pointer;
  }

  .change-card {
    display: grid;
    gap: 7px;
    padding: 12px;
  }

  .change-card:hover,
  .change-card.active,
  .ledger-row:hover {
    border-color: rgba(34, 211, 238, 0.46);
    background: rgba(14, 116, 144, 0.18);
  }

  .change-family {
    color: #67e8f9;
    font-size: 0.72rem;
    font-weight: 800;
    letter-spacing: 0.08em;
    text-transform: uppercase;
  }

  .change-card em {
    justify-self: flex-start;
    padding: 4px 8px;
    border: 1px solid rgba(148, 163, 184, 0.18);
    border-radius: 999px;
    font-style: normal;
    font-size: 0.76rem;
  }

  .installed-ledger,
  .review-ledger {
    margin-top: 14px;
    padding: 12px;
  }

  .ledger-row {
    display: grid;
    gap: 4px;
    margin-top: 10px;
    padding: 10px;
  }

  .change-detail {
    display: grid;
    grid-template-rows: auto minmax(360px, 1fr) auto;
    gap: 14px;
  }

  .detail-card,
  .preview-card {
    padding: 14px;
  }

  .proof-text {
    margin-top: 12px;
  }

  .evidence-strip {
    display: flex;
    flex-wrap: wrap;
    gap: 8px;
    margin-top: 12px;
  }

  .evidence-strip span {
    padding: 5px 8px;
    border: 1px solid rgba(125, 211, 252, 0.18);
    border-radius: 999px;
    background: rgba(8, 47, 73, 0.36);
    color: #bae6fd;
    font-size: 0.78rem;
  }

  .change-actions {
    display: flex;
    flex-wrap: wrap;
    gap: 8px;
    margin-top: 14px;
  }

  button {
    font: inherit;
  }

  .primary-action,
  .secondary-action,
  .install-action,
  .danger-action {
    min-height: 40px;
    padding: 0 13px;
    border: 1px solid rgba(96, 165, 250, 0.28);
    border-radius: 8px;
    color: #e0f2fe;
    background: rgba(30, 64, 175, 0.34);
    cursor: pointer;
  }

  .install-action {
    border-color: rgba(74, 222, 128, 0.3);
    background: rgba(22, 101, 52, 0.34);
  }

  .danger-action {
    border-color: rgba(251, 113, 133, 0.3);
    background: rgba(136, 19, 55, 0.26);
  }

  button:disabled {
    cursor: not-allowed;
    opacity: 0.48;
  }

  .candidate-summary {
    display: grid;
    grid-template-columns: repeat(4, minmax(0, 1fr));
    margin-top: 14px;
  }

  .candidate-summary div {
    min-width: 0;
    padding: 10px;
    border: 1px solid rgba(148, 163, 184, 0.14);
    border-radius: 8px;
    background: rgba(2, 6, 23, 0.46);
  }

  .candidate-summary span,
  .candidate-summary strong {
    display: block;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .technical-details {
    margin-top: 12px;
    border-top: 1px solid rgba(148, 163, 184, 0.14);
    padding-top: 10px;
  }

  .technical-details summary {
    cursor: pointer;
    color: #c7d2fe;
    font-weight: 800;
  }

  dl {
    display: grid;
    gap: 8px;
    margin: 10px 0 0;
  }

  dl div {
    display: grid;
    grid-template-columns: 120px minmax(0, 1fr);
    gap: 8px;
  }

  dt {
    color: #94a3b8;
  }

  dd {
    min-width: 0;
    margin: 0;
    overflow-wrap: anywhere;
    font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
    color: #dbeafe;
  }

  .preview-card {
    display: grid;
    grid-template-rows: auto minmax(0, 1fr);
    gap: 10px;
    min-height: 0;
  }

  .review-row {
    display: grid;
    gap: 8px;
    margin-top: 10px;
    padding: 10px;
    border: 1px solid rgba(148, 163, 184, 0.14);
    border-radius: 8px;
    background: rgba(2, 6, 23, 0.46);
  }

  @media (max-width: 760px) {
    .store-hero {
      padding: 14px;
    }

    .store-layout {
      grid-template-columns: 1fr;
      padding: 12px;
      overflow: auto;
    }

    .change-catalog,
    .change-detail {
      overflow: visible;
    }

    .change-detail {
      grid-template-rows: auto minmax(300px, 58vh) auto;
    }

    .candidate-summary {
      grid-template-columns: repeat(2, minmax(0, 1fr));
    }

    .hero-meter {
      display: none;
    }
  }
</style>
