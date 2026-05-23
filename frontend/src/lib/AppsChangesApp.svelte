<script>
  import { createEventDispatcher, onDestroy, onMount } from 'svelte';
  import { AuthRequiredError, fetchWithRenewal } from './auth.js';
  import { addLiveEventListener, liveEventKind } from './live-events.js';
  import ChangePreviewFrame from './ChangePreviewFrame.svelte';

  export let appContext = {};

  const dispatch = createEventDispatcher();
  const TARGET_COMPUTER_ID = 'primary';

  let packages = [];
  let adoptions = [];
  let runAcceptances = [];
  let reviewEvidence = {};
  let loading = true;
  let error = '';
  let actionError = '';
  let actionStatus = '';
  let acting = '';
  let selectedPackageId = appContext?.packageId || '';
  let previewCandidateId = '';
  let previewStates = {};
  let removeLiveListener = () => {};

  $: changes = packages.map(packageToChange);
  $: if (!selectedPackageId && changes.length > 0) {
    selectedPackageId = changes[0].id;
  }
  $: selectedChange = changes.find((change) => change.id === selectedPackageId) || changes[0] || null;
  $: selectedPackage = selectedChange?.pkg || null;
  $: selectedAdoption = selectedPackage
    ? adoptions.find((adoption) => adoption.package_id === selectedPackage.package_id) || null
    : null;
  $: selectedPreviewId = previewCandidateId || selectedAdoption?.target_candidate_id || '';
  $: selectedPreviewState = selectedPreviewId ? previewStates[selectedPreviewId] || 'empty' : 'empty';
  $: selectedRemoval = removalProfile(selectedAdoption);
  $: selectedAcceptance = (selectedAdoption?.trace_id ? latestAcceptanceForTrace(selectedAdoption.trace_id) : null)
    || latestReviewAcceptance(selectedChange)
    || latestAcceptanceForPackage(selectedChange);
  $: reviewableCount = changes.filter((change) => change.humanProof.state === 'human_reviewable').length;
  $: evidencePendingCount = changes.filter((change) => change.humanProof.state !== 'human_reviewable').length;
  $: installedAdoptions = adoptions.filter((adoption) => adoption.status === 'adopted');
  $: reviewAdoptions = adoptions.filter((adoption) => adoption.status !== 'adopted');

  function parseRecordJSON(value) {
    if (!value) return {};
    if (typeof value === 'string') {
      try {
        return JSON.parse(value);
      } catch {
        return {};
      }
    }
    if (typeof value === 'object') return value;
    return {};
  }

  function safeID(value) {
    return String(value || '')
      .toLowerCase()
      .replace(/[^a-z0-9]+/g, '-')
      .replace(/^-+|-+$/g, '')
      .slice(0, 64) || 'change';
  }

  function shortRef(value) {
    if (!value) return 'pending';
    const text = String(value);
    return text.length > 16 ? text.slice(0, 16) : text;
  }

  function newRunID(prefix, change) {
    if (globalThis.crypto?.randomUUID) {
      return `${prefix}-${safeID(change?.id)}-${globalThis.crypto.randomUUID()}`;
    }
    return `${prefix}-${safeID(change?.id)}-${Date.now()}`;
  }

  function compact(values) {
    const seen = new Set();
    const out = [];
    for (const value of values || []) {
      const text = String(value || '').trim();
      if (!text || seen.has(text)) continue;
      seen.add(text);
      out.push(text);
    }
    return out;
  }

  function credibleHumanBenchmarkRef(value) {
    const text = String(value || '').trim().toLowerCase();
    if (!text) return false;
    const blockedTerms = [
      'blocked',
      'failed',
      'failure',
      'error',
      'unavailable',
      'not available',
      'pending',
      'not run',
      'not captured',
      'cannot run',
      'could not',
    ];
    if (blockedTerms.some((term) => text.includes(term))) return false;
    const receiptOnlyTerms = [
      'npm --prefix frontend run build',
      'npm --prefix frontend ci',
      'npm ci',
      'npm install',
      'pnpm build',
      'go build',
      'vite build',
      'build proof',
      'build receipt',
      'build passed',
      'build pass',
      'frontend production build',
      'chunk-size warning',
      'npm audit',
    ];
    if (receiptOnlyTerms.some((term) => text.includes(term))) return false;
    if (!/[0-9]/.test(text)) return false;
    return ['benchmark', 'latency', 'duration', 'tokens', 'fps', 'memory', 'cpu', 'resource', 'wall time', 'p95', 'median']
      .some((term) => text.includes(term));
  }

  function hasCredibleHumanBenchmarkRefs(refs) {
    return (refs || []).some(credibleHumanBenchmarkRef);
  }

  function normalizeHumanProof(proof = {}) {
    const normalized = {
      state: proof.state || 'evidence_pending',
      summary: proof.summary || '',
      recommendation: proof.recommendation || '',
      narrative_refs: compact(proof.narrative_refs || []),
      screenshot_refs: compact(proof.screenshot_refs || []),
      video_refs: compact(proof.video_refs || []),
      benchmark_refs: compact(proof.benchmark_refs || []),
      artifact_refs: compact(proof.artifact_refs || []),
      missing: compact(proof.missing || []),
    };
    if (!normalized.state) normalized.state = 'evidence_pending';
    return normalized;
  }

  function collectHumanProofValue(proof, value, key = '') {
    if (!value) return;
    if (Array.isArray(value)) {
      for (const item of value) collectHumanProofValue(proof, item, key);
      return;
    }
    if (typeof value === 'object') {
      for (const [nextKey, nextValue] of Object.entries(value)) {
        const lower = String(nextKey || '').toLowerCase();
        if (['summary', 'human_summary', 'narrative_summary'].includes(lower) && !proof.summary) {
          proof.summary = String(nextValue || '').trim();
        }
        if (lower === 'recommendation' && !proof.recommendation) {
          proof.recommendation = String(nextValue || '').trim();
        }
        collectHumanProofValue(proof, nextValue, lower);
      }
      return;
    }
    collectHumanProofString(proof, key, String(value));
  }

  function collectHumanProofString(proof, key, raw) {
    const text = String(raw || '').trim();
    if (!text) return;
    const lowerKey = String(key || '').toLowerCase();
    const lowerText = text.toLowerCase();
    if (lowerKey.includes('vtext') || lowerKey.includes('narrative_ref') || lowerText.startsWith('vtext:')) {
      proof.narrative_refs.push(text);
    } else if (
      lowerKey.includes('screenshot') ||
      lowerKey.includes('image') ||
      lowerText.endsWith('.png') ||
      lowerText.endsWith('.jpg') ||
      lowerText.endsWith('.jpeg')
    ) {
      proof.screenshot_refs.push(text);
    } else if (lowerKey.includes('video') || lowerText.endsWith('.webm') || lowerText.endsWith('.mp4')) {
      proof.video_refs.push(text);
    } else if (lowerKey.includes('benchmark') || lowerText.includes('benchmark')) {
      proof.benchmark_refs.push(text);
    } else if (lowerKey.includes('artifact') || lowerKey.includes('evidence') || lowerKey.includes('acceptance')) {
      proof.artifact_refs.push(text);
    }
  }

  function humanProofForPackage(pkg, evidence = null) {
    if (evidence?.human_proof) {
      return normalizeHumanProof(evidence.human_proof);
    }
    const proof = {
      state: 'evidence_pending',
      summary: '',
      recommendation: '',
      narrative_refs: [],
      screenshot_refs: [],
      video_refs: [],
      benchmark_refs: [],
      artifact_refs: [],
      missing: [],
    };
    collectHumanProofValue(proof, parseRecordJSON(pkg?.provenance_refs_json), '');
    proof.narrative_refs = compact(proof.narrative_refs);
    proof.screenshot_refs = compact(proof.screenshot_refs);
    proof.video_refs = compact(proof.video_refs);
    proof.benchmark_refs = compact(proof.benchmark_refs);
    proof.artifact_refs = compact(proof.artifact_refs);
    proof.missing = [];
    const hasNarrative = proof.narrative_refs.length > 0;
    const hasHumanMedia = proof.screenshot_refs.length > 0 || proof.video_refs.length > 0 || hasCredibleHumanBenchmarkRefs(proof.benchmark_refs);
    if (hasNarrative && hasHumanMedia) {
      proof.state = 'human_reviewable';
    } else {
      proof.state = proof.artifact_refs.length > 0 ? 'machine_receipt_only' : 'evidence_pending';
      if (!hasNarrative) proof.missing.push('causal VText narrative');
      if (!hasHumanMedia) proof.missing.push('screenshot, video, or benchmark evidence');
    }
    return proof;
  }

  function titleForPackage(pkg, proof) {
    const manifest = parseRecordJSON(pkg?.manifest_json);
    return (
      manifest.title ||
      manifest.name ||
      proof?.summary?.split('\n')[0]?.slice(0, 80) ||
      pkg?.app_id ||
      'Untitled change'
    );
  }

  function packageToChange(pkg) {
    const evidence = reviewEvidence[pkg.package_id] || null;
    const proof = humanProofForPackage(pkg, evidence);
    const manifest = parseRecordJSON(pkg.manifest_json);
    return {
      id: pkg.package_id,
      pkg,
      title: titleForPackage(pkg, proof),
      family: manifest.family || manifest.category || pkg.app_id || 'Change',
      summary: proof.summary || manifest.summary || 'Published source-level change. Human proof has not been attached yet.',
      recommendation: proof.recommendation || manifest.recommendation || '',
      humanProof: proof,
      reviewAcceptances: Array.isArray(evidence?.acceptances) ? evidence.acceptances : [],
    };
  }

  function latestAdoptionForPackage(packageId) {
    if (!packageId) return null;
    return adoptions.find((adoption) => adoption.package_id === packageId) || null;
  }

  function latestAcceptanceForTrace(traceId) {
    if (!traceId) return null;
    return runAcceptances.find((acceptance) => acceptance.trajectory_id === traceId) || null;
  }

  function latestReviewAcceptance(change) {
    return change?.reviewAcceptances?.find((acceptance) => acceptance.state === 'accepted') || change?.reviewAcceptances?.[0] || null;
  }

  function latestAcceptanceForPackage(change) {
    if (!change?.id) return null;
    return runAcceptances.find((acceptance) => JSON.stringify(acceptance).includes(change.id)) || null;
  }

  function acceptanceEvidenceCount(acceptance) {
    if (!acceptance) return 0;
    if (Array.isArray(acceptance.evidence_refs)) return acceptance.evidence_refs.length;
    return Number(acceptance.evidence_ref_count || 0);
  }

  function acceptanceRollbackCount(acceptance) {
    if (!acceptance) return 0;
    if (Array.isArray(acceptance.rollback_refs)) return acceptance.rollback_refs.length;
    return Number(acceptance.rollback_ref_count || 0);
  }

  function statusLabel(change) {
    const adoption = latestAdoptionForPackage(change?.id);
    if (adoption?.status === 'adopted') return 'installed';
    if (adoption?.status === 'verified') return 'build verified';
    if (adoption?.status === 'blocked') return 'blocked';
    if (adoption?.status) return adoption.status.replaceAll('_', ' ');
    if (change?.humanProof?.state === 'human_reviewable') return 'ready to review';
    if (change?.humanProof?.state === 'machine_receipt_only') return 'machine receipts only';
    return 'evidence pending';
  }

  function proofLabel(state) {
    if (state === 'human_reviewable') return 'Human review ready';
    if (state === 'machine_receipt_only') return 'Machine receipts only';
    return 'Needs human proof';
  }

  function hasRollbackProfile(adoption) {
    const profile = parseRecordJSON(adoption?.rollback_profile_json);
    return !!profile.previous_active_source_ref;
  }

  function rollbackProfileLabel(adoption) {
    if (!adoption) return 'pending';
    return hasRollbackProfile(adoption) ? 'recorded' : 'pending';
  }

  function removalProfile(adoption) {
    if (!adoption) {
      return {
        mode: 'Not tried',
        rollback: 'Try and verify this change before recovery actions are available.',
        uninstall: 'Unavailable until a recipient adoption exists.',
        disable: 'Unavailable until a recipient adoption exists.',
      };
    }
    if (adoption.status === 'rolled_back') {
      return {
        mode: 'Rolled back',
        rollback: 'This adoption has already been rolled back to the recorded source ref.',
        uninstall: 'Not needed after rollback.',
        disable: 'Not applicable after rollback.',
      };
    }
    if (hasRollbackProfile(adoption)) {
      return {
        mode: 'Rollback-only',
        rollback: 'Available: restore the previous active source ref and route profile.',
        uninstall: 'Unavailable: this package has no verified inverse source patch.',
        disable: 'Unavailable: this package has no declared feature flag or capability toggle.',
      };
    }
    return {
      mode: 'Recovery pending',
      rollback: 'Pending: verify the recipient build to record rollback refs.',
      uninstall: 'Unavailable: source-level inverse removal has not been verified.',
      disable: 'Unavailable: no feature flag or capability toggle is declared.',
    };
  }

  function actionKey(id, action) {
    return `${id}:${action}`;
  }

  function canTry(change) {
    if (!change) return false;
    return change.humanProof.state === 'human_reviewable' && !latestAdoptionForPackage(change.id);
  }

  function canVerify(adoption) {
    return adoption && ['adoption_proposed', 'candidate_applied', 'blocked'].includes(adoption.status) && selectedPreviewState === 'ready';
  }

  function canInstall(adoption, change) {
    return adoption && change?.humanProof?.state === 'human_reviewable' && ['verified', 'owner_approved'].includes(adoption.status);
  }

  function canRollback(adoption) {
    return adoption && ['verified', 'adopted', 'blocked'].includes(adoption.status) && hasRollbackProfile(adoption);
  }

  async function fetchJSON(path, options = {}) {
    const res = await fetchWithRenewal(path, options);
    const body = await res.json().catch(() => ({}));
    if (!res.ok) throw new Error(body?.error || `${path} failed (${res.status})`);
    return body;
  }

  function acceptanceIDsForPackage(pkg) {
    const refs = [];
    collectHumanProofValue({ narrative_refs: [], screenshot_refs: [], video_refs: [], benchmark_refs: [], artifact_refs: refs }, parseRecordJSON(pkg?.provenance_refs_json), '');
    collectHumanProofValue({ narrative_refs: [], screenshot_refs: [], video_refs: [], benchmark_refs: [], artifact_refs: refs }, parseRecordJSON(pkg?.manifest_json), '');
    for (const acceptance of runAcceptances) {
      const body = JSON.stringify(acceptance);
      if (body.includes(pkg.package_id)) refs.push(acceptance.acceptance_id);
    }
    return compact(refs.filter((ref) => String(ref).startsWith('runacc-')));
  }

  async function loadRunAcceptances() {
    try {
      const body = await fetchJSON('/api/run-acceptances?limit=100', { method: 'GET' });
      runAcceptances = Array.isArray(body?.acceptances) ? body.acceptances : [];
    } catch (err) {
      if (err instanceof AuthRequiredError) {
        dispatch('authexpired');
        return;
      }
      runAcceptances = [];
    }
  }

  async function loadPackageReviewEvidence(nextPackages) {
    const nextEvidence = {};
    await Promise.all((nextPackages || []).map(async (pkg) => {
      const ids = acceptanceIDsForPackage(pkg);
      const query = ids.length ? `?${ids.map((id) => `acceptance_id=${encodeURIComponent(id)}`).join('&')}` : '';
      try {
        const body = await fetchJSON(`/api/app-change-packages/${encodeURIComponent(pkg.package_id)}/review-evidence${query}`, {
          method: 'GET',
        });
        nextEvidence[pkg.package_id] = body;
      } catch {
        nextEvidence[pkg.package_id] = { human_proof: humanProofForPackage(pkg), acceptances: [] };
      }
    }));
    reviewEvidence = nextEvidence;
  }

  function mergePreservedAdoptions(nextAdoptions, preservedAdoptions = []) {
    const merged = Array.isArray(nextAdoptions) ? [...nextAdoptions] : [];
    for (const adoption of preservedAdoptions) {
      if (!adoption?.adoption_id) continue;
      if (!merged.some((item) => item.adoption_id === adoption.adoption_id)) merged.unshift(adoption);
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
      const nextPackages = Array.isArray(packageBody?.packages) ? packageBody.packages : [];
      packages = nextPackages;
      const nextAdoptions = Array.isArray(adoptionBody?.adoptions) ? adoptionBody.adoptions : [];
      adoptions = mergePreservedAdoptions(nextAdoptions, preservedAdoptions);
      await loadRunAcceptances();
      await loadPackageReviewEvidence(nextPackages);
      if (selectedPackageId && !nextPackages.some((pkg) => pkg.package_id === selectedPackageId)) {
        selectedPackageId = nextPackages[0]?.package_id || '';
      }
    } catch (err) {
      if (err instanceof AuthRequiredError) {
        dispatch('authexpired');
        return;
      }
      error = err.message || 'Apps & Changes is unavailable';
      packages = [];
      adoptions = [];
      runAcceptances = [];
      reviewEvidence = {};
    } finally {
      loading = false;
    }
  }

  function selectChange(change) {
    selectedPackageId = change.id;
    const adoption = latestAdoptionForPackage(change.id);
    previewCandidateId = adoption?.target_candidate_id || '';
    actionError = '';
  }

  function openExistingVText(change) {
    const provenance = parseRecordJSON(change?.pkg?.provenance_refs_json);
    const docId = provenance.vtext_doc_id || provenance.vtextDocId || '';
    if (!docId) {
      actionError = 'This change does not include a VText narrative yet.';
      return;
    }
    dispatch('openvtext', {
      docId,
      revisionId: provenance.vtext_revision_id || provenance.vtextRevisionId || '',
      title: `${change.title} narrative`,
    });
  }

  async function tryChange(change) {
    if (!canTry(change)) return;
    const pkg = change.pkg;
    actionError = '';
    actionStatus = `Preparing a candidate preview for ${change.title}`;
    acting = actionKey(change.id, 'try');
    try {
      await fetchJSON('/api/app-change-packages/pull', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          package_id: pkg.package_id,
          source_owner_id: pkg.owner_id,
          source_desktop_id: pkg.source_computer_id,
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
          package_id: pkg.package_id,
          target_candidate_id: targetCandidateId,
          candidate_source_ref: `refs/computers/${TARGET_COMPUTER_ID}/candidates/${targetCandidateId}`,
          foreground_tail_merge_result: 'pending-recipient-review',
          merge_strategy: 'rebase',
          trace_id: `apps-changes-${safeID(change.title)}`,
        }),
      });
      adoptions = [adoption, ...adoptions.filter((item) => item.adoption_id !== adoption.adoption_id)];
      previewCandidateId = adoption.target_candidate_id || targetCandidateId;
      actionStatus = `Candidate preview is starting from ${shortRef(lineage.active_source_ref)}`;
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
    actionStatus = `${action === 'verify' ? 'Verifying recipient build' : action === 'promote' ? 'Installing change' : 'Rolling back change'}`;
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
      await refreshCatalog([next]);
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

  function openTraceForEvidence() {
    const trajectoryId = selectedAdoption?.trace_id || (selectedAcceptance?.trace_visible !== false ? selectedAcceptance?.trajectory_id : '');
    if (!trajectoryId) return;
    dispatch('opentrace', {
      trajectoryId,
      acceptanceId: selectedAcceptance?.acceptance_id || '',
      title: `${selectedChange.title} Trace`,
      toastMessage: `Opened Trace for ${selectedChange.title}`,
    });
  }

  function handlePreviewState(event) {
    const detail = event.detail || {};
    if (!detail.candidateDesktopId) return;
    previewStates = { ...previewStates, [detail.candidateDesktopId]: detail.state || 'loading' };
    if (detail.state === 'blocked') {
      actionStatus = '';
      actionError = detail.message || 'Candidate preview is blocked';
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
        kind === 'app_adoption.rolled_back' ||
        kind === 'run_acceptance.synthesized' ||
        kind === 'run_acceptance.accepted'
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
      <h2>Review changes before they enter this computer</h2>
      <p>Try source-level changes in a candidate, then install only after human proof and recipient build verification.</p>
    </div>
    <div class="hero-meter" data-apps-changes-count>
      <strong>{reviewableCount}</strong>
      <span>ready to review</span>
    </div>
  </header>

  {#if error}
    <div class="state-banner error" data-apps-changes-error role="alert">{error}</div>
  {:else if actionError}
    <div class="state-banner error" data-apps-changes-action-error role="alert">{actionError}</div>
  {:else if actionStatus}
    <div class="state-banner" data-apps-changes-action-status>{actionStatus}</div>
  {/if}

  {#if loading}
    <div class="empty-state" data-apps-changes-loading>Loading changes...</div>
  {:else if changes.length === 0}
    <div class="empty-state" data-apps-changes-empty>
      <strong>No reviewable changes have been published to this computer yet.</strong>
      <span>Choir agents must publish AppChangePackages with VText narrative plus screenshot, video, or benchmark evidence before they appear as useful review items.</span>
    </div>
  {:else}
    <div class="store-layout">
      <aside class="change-catalog" data-change-catalog>
        <div class="section-heading">
          <strong>Changes</strong>
          <span>{reviewableCount} ready, {evidencePendingCount} pending</span>
        </div>
        <div class="change-list">
          {#each changes as change (change.id)}
            <button
              class:active={selectedChange?.id === change.id}
              class="change-card"
              data-change-card
              data-change-id={change.id}
              data-human-proof-state={change.humanProof.state}
              on:click={() => selectChange(change)}
            >
              <span class="change-family">{change.family}</span>
              <strong>{change.title}</strong>
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
              <button class="ledger-row" on:click={() => (selectedPackageId = adoption.package_id)}>
                <strong>{adoption.app_id || adoption.package_id}</strong>
                <span>{shortRef(adoption.runtime_artifact_digest)} / {shortRef(adoption.ui_artifact_digest)}</span>
              </button>
            {/each}
          {/if}
        </section>
      </aside>

      <main class="change-detail" data-change-detail>
        {#if selectedChange}
          <section class="detail-card">
            <div class="detail-top">
              <div>
                <p class="eyebrow">{selectedChange.family}</p>
                <h3>{selectedChange.title}</h3>
                <p>{selectedChange.summary}</p>
              </div>
              <span class="status-pill" data-change-status>{statusLabel(selectedChange)}</span>
            </div>

            <section
              class:ready={selectedChange.humanProof.state === 'human_reviewable'}
              class="human-proof-panel"
              data-human-proof-panel
              data-human-proof-state={selectedChange.humanProof.state}
            >
              <div class="section-heading">
                <strong>{proofLabel(selectedChange.humanProof.state)}</strong>
                <span>{selectedChange.humanProof.recommendation || selectedChange.recommendation || 'No recommendation yet'}</span>
              </div>
              {#if selectedChange.humanProof.state !== 'human_reviewable'}
                <p data-human-proof-missing>
                  Missing: {selectedChange.humanProof.missing.join(', ') || 'human proof'}
                </p>
              {/if}
              <div class="evidence-strip" data-change-evidence>
                {#if selectedChange.humanProof.narrative_refs.length > 0}
                  <span>{selectedChange.humanProof.narrative_refs.length} narrative</span>
                {/if}
                {#if selectedChange.humanProof.screenshot_refs.length > 0}
                  <span>{selectedChange.humanProof.screenshot_refs.length} screenshots</span>
                {/if}
                {#if selectedChange.humanProof.video_refs.length > 0}
                  <span>{selectedChange.humanProof.video_refs.length} videos</span>
                {/if}
                {#if selectedChange.humanProof.benchmark_refs.length > 0}
                  <span>{selectedChange.humanProof.benchmark_refs.length} benchmarks</span>
                {/if}
                {#if selectedChange.humanProof.artifact_refs.length > 0}
                  <span>{selectedChange.humanProof.artifact_refs.length} machine refs</span>
                {/if}
              </div>
              <div class="report-actions" data-change-report-actions>
                <button class="report-action" data-change-open-vtext-report on:click={() => openExistingVText(selectedChange)}>
                  Open VText narrative
                </button>
              </div>
            </section>

            <div class="change-actions" data-change-actions>
              <button
                class="primary-action"
                data-change-try
                on:click={() => tryChange(selectedChange)}
                disabled={!canTry(selectedChange) || !!acting}
              >
                {acting === actionKey(selectedChange.id, 'try') ? 'Preparing...' : selectedAdoption ? 'Candidate prepared' : 'Try in candidate'}
              </button>
              <button
                class="secondary-action"
                data-change-verify
                on:click={() => runAdoptionAction(selectedAdoption, 'verify')}
                disabled={!canVerify(selectedAdoption) || !!acting}
              >
                {selectedAdoption && acting === actionKey(selectedAdoption.adoption_id, 'verify') ? 'Verifying...' : 'Verify build'}
              </button>
              <button
                class="install-action"
                data-change-install
                on:click={() => runAdoptionAction(selectedAdoption, 'promote')}
                disabled={!canInstall(selectedAdoption, selectedChange) || !!acting}
              >
                {selectedAdoption && acting === actionKey(selectedAdoption.adoption_id, 'promote') ? 'Installing...' : 'Install'}
              </button>
              <button
                class="danger-action"
                data-change-rollback
                on:click={() => runAdoptionAction(selectedAdoption, 'rollback')}
                disabled={!canRollback(selectedAdoption) || !!acting}
              >
                {selectedAdoption && acting === actionKey(selectedAdoption.adoption_id, 'rollback') ? 'Rolling back...' : 'Rollback'}
              </button>
            </div>

            <div class="candidate-summary" data-change-candidate-summary>
              <div>
                <span>candidate</span>
                <strong>{selectedAdoption?.target_candidate_id || 'not tried'}</strong>
              </div>
              <div>
                <span>preview</span>
                <strong>{selectedPreviewId ? selectedPreviewState : 'not opened'}</strong>
              </div>
              <div>
                <span>runtime</span>
                <strong>{shortRef(selectedAdoption?.runtime_artifact_digest)}</strong>
              </div>
              <div>
                <span>rollback</span>
                <strong>{rollbackProfileLabel(selectedAdoption)}</strong>
              </div>
            </div>

            <section
              class="trace-review-panel"
              data-change-trace-review
              data-change-trace-ready={selectedAcceptance ? 'accepted' : selectedAdoption?.trace_id ? 'trace-only' : 'none'}
              data-change-trace-id={selectedAdoption?.trace_id || selectedAcceptance?.trajectory_id || ''}
              data-change-acceptance-id={selectedAcceptance?.acceptance_id || ''}
            >
              <div class="section-heading">
                <strong>Trace and acceptance</strong>
                <span>{selectedAcceptance?.acceptance_level || (selectedAdoption?.trace_id ? 'trace linked' : 'not tried')}</span>
              </div>
              <div class="trace-review-grid">
                <div>
                  <span>trajectory</span>
                  <strong>{selectedAdoption?.trace_id ? shortRef(selectedAdoption.trace_id) : selectedAcceptance?.trajectory_id ? shortRef(selectedAcceptance.trajectory_id) : 'not created'}</strong>
                </div>
                <div>
                  <span>acceptance</span>
                  <strong>{selectedAcceptance?.acceptance_level || 'not synthesized'}</strong>
                </div>
                <div>
                  <span>state</span>
                  <strong>{selectedAcceptance?.state || selectedAdoption?.status || 'available'}</strong>
                </div>
                <div>
                  <span>evidence</span>
                  <strong>{selectedAcceptance ? `${acceptanceEvidenceCount(selectedAcceptance)} refs / ${acceptanceRollbackCount(selectedAcceptance)} rollback` : 'pending'}</strong>
                </div>
              </div>
              <p class="trace-review-note" data-change-acceptance-summary>
                {#if selectedAcceptance}
                  {selectedAcceptance.supports_human_review === false ? 'Machine acceptance exists, but this is not enough for human review.' : selectedAcceptance.target_mission_id || selectedAcceptance.acceptance_id}
                {:else if selectedAdoption?.trace_id}
                  Trace is linked, but no run acceptance record is available yet.
                {:else}
                  Try this change before Trace and run-acceptance evidence can be opened.
                {/if}
              </p>
              <div class="trace-review-actions">
                <button
                  class="report-action"
                  data-change-open-trace
                  on:click={openTraceForEvidence}
                  disabled={!selectedAdoption?.trace_id && !(selectedAcceptance?.trajectory_id && selectedAcceptance?.trace_visible !== false)}
                >
                  Open Trace evidence
                </button>
              </div>
            </section>

            <section class="removal-panel" data-change-removal-model data-removal-mode={selectedRemoval.mode}>
              <div class="section-heading">
                <strong>Removal and recovery</strong>
                <span>{selectedRemoval.mode}</span>
              </div>
              <div class="removal-status-grid">
                <div>
                  <span>rollback</span>
                  <strong>{selectedRemoval.rollback}</strong>
                </div>
                <div>
                  <span>uninstall</span>
                  <strong>{selectedRemoval.uninstall}</strong>
                </div>
                <div>
                  <span>disable</span>
                  <strong>{selectedRemoval.disable}</strong>
                </div>
              </div>
              <div class="removal-actions">
                <button class="unavailable-action" data-change-uninstall disabled>Uninstall unavailable</button>
                <button class="unavailable-action" data-change-disable disabled>Disable unavailable</button>
              </div>
            </section>

            <details class="technical-details" data-change-technical-details>
              <summary>Technical refs</summary>
              <dl>
                <div><dt>Package</dt><dd>{selectedPackage.package_id}</dd></div>
                <div><dt>Source owner</dt><dd>{selectedPackage.owner_id}</dd></div>
                <div><dt>Source computer</dt><dd>{selectedPackage.source_computer_id}</dd></div>
                <div><dt>Manifest hash</dt><dd>{selectedPackage.package_manifest_sha256 || 'missing'}</dd></div>
                <div><dt>Adoption</dt><dd>{selectedAdoption?.adoption_id || 'not created'}</dd></div>
                <div><dt>Candidate ref</dt><dd>{selectedAdoption?.candidate_source_ref || 'not created'}</dd></div>
              </dl>
            </details>
          </section>

          <section class="preview-card" data-change-preview>
            <div class="preview-heading">
              <div>
                <strong>Candidate preview</strong>
                <span>Try opens a candidate computer. A blocked route cannot be verified or installed.</span>
              </div>
              <span>{selectedPreviewId ? selectedPreviewState : 'empty'}</span>
            </div>
            <ChangePreviewFrame
              candidateDesktopId={selectedPreviewId}
              title={`${selectedChange.title} candidate preview`}
              on:previewstate={handlePreviewState}
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
                    <span>{adoption.status} / {adoption.target_computer_id}</span>
                  </div>
                  {#if adoption.error}
                    <p>{adoption.error}</p>
                  {/if}
                </article>
              {/each}
            {/if}
          </section>
        {/if}
      </main>
    </div>
  {/if}
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

  .state-banner,
  .empty-state {
    margin: 12px 16px 0;
    padding: 12px;
    border: 1px solid rgba(34, 211, 238, 0.22);
    border-radius: 8px;
    background: rgba(8, 47, 73, 0.46);
    color: #dff7ff;
  }

  .state-banner.error {
    border-color: rgba(248, 113, 113, 0.35);
    background: rgba(69, 10, 10, 0.54);
    color: #fecaca;
  }

  .empty-state {
    display: grid;
    gap: 8px;
    place-content: center;
    min-height: 260px;
    text-align: center;
  }

  .empty-state span {
    max-width: 620px;
    color: #9fb1c9;
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

  .human-proof-panel {
    display: grid;
    gap: 10px;
    margin-top: 14px;
    padding: 12px;
    border: 1px solid rgba(251, 191, 36, 0.22);
    border-radius: 8px;
    background: rgba(69, 26, 3, 0.16);
  }

  .human-proof-panel.ready {
    border-color: rgba(34, 211, 238, 0.24);
    background: rgba(8, 47, 73, 0.2);
  }

  .evidence-strip {
    display: flex;
    flex-wrap: wrap;
    gap: 8px;
  }

  .evidence-strip span {
    padding: 5px 8px;
    border: 1px solid rgba(125, 211, 252, 0.18);
    border-radius: 999px;
    background: rgba(8, 47, 73, 0.36);
    color: #bae6fd;
    font-size: 0.78rem;
  }

  .report-actions,
  .change-actions,
  .trace-review-actions,
  .removal-actions {
    display: flex;
    flex-wrap: wrap;
    gap: 8px;
  }

  button {
    font: inherit;
  }

  .primary-action,
  .secondary-action,
  .install-action,
  .danger-action,
  .report-action {
    min-height: 40px;
    padding: 0 13px;
    border: 1px solid rgba(96, 165, 250, 0.28);
    border-radius: 8px;
    color: #e0f2fe;
    background: rgba(30, 64, 175, 0.34);
    cursor: pointer;
  }

  .report-action {
    border-color: rgba(34, 211, 238, 0.3);
    background: rgba(8, 47, 73, 0.52);
  }

  .install-action {
    border-color: rgba(74, 222, 128, 0.3);
    background: rgba(22, 101, 52, 0.34);
  }

  .danger-action {
    border-color: rgba(251, 113, 133, 0.3);
    background: rgba(136, 19, 55, 0.26);
  }

  .unavailable-action {
    min-height: 36px;
    padding: 0 12px;
    border: 1px solid rgba(148, 163, 184, 0.2);
    border-radius: 8px;
    color: #cbd5e1;
    background: rgba(15, 23, 42, 0.62);
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

  .candidate-summary div,
  .trace-review-grid div,
  .removal-status-grid div {
    min-width: 0;
    padding: 10px;
    border: 1px solid rgba(148, 163, 184, 0.14);
    border-radius: 8px;
    background: rgba(2, 6, 23, 0.46);
  }

  .candidate-summary span,
  .candidate-summary strong,
  .trace-review-grid span,
  .trace-review-grid strong,
  .removal-status-grid span,
  .removal-status-grid strong {
    display: block;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .removal-panel,
  .trace-review-panel {
    display: grid;
    gap: 10px;
    margin-top: 14px;
    padding: 12px;
    border-radius: 8px;
  }

  .removal-panel {
    border: 1px solid rgba(251, 191, 36, 0.18);
    background: rgba(69, 26, 3, 0.18);
  }

  .trace-review-panel {
    border: 1px solid rgba(34, 211, 238, 0.2);
    background: rgba(8, 47, 73, 0.2);
  }

  .trace-review-grid {
    display: grid;
    grid-template-columns: repeat(4, minmax(0, 1fr));
    gap: 8px;
  }

  .trace-review-grid span,
  .removal-status-grid span {
    color: #67e8f9;
    font-size: 0.74rem;
    font-weight: 800;
    letter-spacing: 0.08em;
    text-transform: uppercase;
  }

  .trace-review-grid strong,
  .removal-status-grid strong {
    margin-top: 5px;
    color: #f8fafc;
    font-size: 0.84rem;
  }

  .trace-review-note {
    color: #bfdbfe;
    font-size: 0.86rem;
    line-height: 1.4;
  }

  .removal-status-grid {
    display: grid;
    grid-template-columns: repeat(3, minmax(0, 1fr));
    gap: 8px;
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
    .apps-changes {
      overflow: auto;
    }

    .store-hero {
      padding: 14px;
    }

    .store-layout {
      display: flex;
      flex: 1 0 auto;
      flex-direction: column;
      padding: 12px;
      overflow: visible;
    }

    .change-catalog,
    .change-detail {
      position: relative;
      width: 100%;
      max-width: 100%;
      overflow: visible;
    }

    .change-detail {
      grid-template-rows: auto minmax(300px, 58vh) auto;
    }

    .candidate-summary,
    .trace-review-grid {
      grid-template-columns: repeat(2, minmax(0, 1fr));
    }

    .removal-status-grid {
      grid-template-columns: 1fr;
    }

    .hero-meter {
      display: none;
    }
  }
</style>
