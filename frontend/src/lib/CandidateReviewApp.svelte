<script>
  import { createEventDispatcher, onMount } from 'svelte';
  import { AuthRequiredError, fetchWithRenewal } from './auth.js';

  export let appContext = {};
  export let authenticated = false;

  const dispatch = createEventDispatcher();

  let intakeID = String(appContext?.intakeID || appContext?.intake_id || '').trim();
  let adoptionID = String(appContext?.adoptionID || appContext?.adoption_id || '').trim();
  let loading = false;
  let error = '';
  let surface = null;
  let activationDecision = null;

  $: canFetch = !!intakeID && !!adoptionID && !loading;
  $: boundaryRows = surface ? [
    ['Package publication', surface.package_publication],
    ['Deployed promotion', surface.deployed_promotion],
    ['Deployed route mutation', surface.deployed_route_mutation],
    ['Promotion-level acceptance', surface.promotion_level],
    ['Run acceptance record', surface.run_acceptance_record],
    ['Auth/session', surface.auth_session],
    ['Staging', surface.staging],
    ['VM lifecycle', surface.vm_lifecycle],
    ['AppChangePackage mutation', surface.app_change_package_mutation],
    ['AppAdoption mutation', surface.app_adoption_mutation],
  ] : [];

  function encodePathPart(value) {
    return encodeURIComponent(String(value || '').trim());
  }

  function clearResult() {
    surface = null;
    activationDecision = null;
    error = '';
  }

  async function fetchReviewSurface() {
    if (!intakeID || !adoptionID) {
      surface = null;
      error = 'Enter both intake and adoption IDs to load a review surface.';
      return;
    }
    if (!authenticated) {
      dispatch('authrequired', { kind: 'candidate_review', appId: 'candidate-review', appName: 'Candidate Review' });
      return;
    }
    loading = true;
    error = '';
    surface = null;
    const path = `/api/candidate-package-intakes/${encodePathPart(intakeID)}/adoption-review/${encodePathPart(adoptionID)}/promotion-switch/review-surface`;
    try {
      const res = await fetchWithRenewal(path, { method: 'GET' });
      const body = await res.json().catch(() => ({}));
      if (!res.ok) {
        throw new Error(body?.error || `Review surface unavailable (${res.status})`);
      }
      surface = body;
    } catch (err) {
      if (err instanceof AuthRequiredError) {
        dispatch('authexpired');
        return;
      }
      error = err?.message || 'Review surface unavailable';
    } finally {
      loading = false;
    }
  }

  function prepareActivationDecision() {
    if (!surface) return;
    const boundary = surface.activation_decision_boundary || {};
    activationDecision = {
      state: boundary.state || 'owner_decision_preparable',
      action: boundary.prepared_action || 'prepare_activation_decision',
      noMutation: boundary.no_mutation !== false,
      nextBoundary: boundary.next_boundary || 'app_adoption_promotion_requires_separate_product_activation_contract',
      usesAcceptanceID: boundary.uses_local_acceptance_id || surface.local_acceptance_id || surface.acceptance_evidence?.acceptance_id || '',
      requiredContracts: boundary.required_contracts || [],
      blockedRoutes: boundary.blocked_routes || [],
    };
  }

  onMount(() => {
    if (intakeID && adoptionID && authenticated) {
      void fetchReviewSurface();
    }
  });
</script>

<section class="candidate-review-app" data-candidate-review-app>
  <header class="hero">
    <div>
      <p class="eyebrow">Non-deployed adoption review</p>
      <h1>Candidate Review</h1>
      <p class="lede">Review accepted local source-lineage evidence without publishing packages, mutating deployed routes, touching auth/session, or claiming run acceptance.</p>
    </div>
    <div class="status-card" data-candidate-review-mode>
      <span>Mode</span>
      <strong>Read-only</strong>
      <small>Product-visible, non-deployed</small>
    </div>
  </header>

  <form class="review-form" data-candidate-review-form on:submit|preventDefault={fetchReviewSurface}>
    <label>
      <span>Intake ID</span>
      <input
        data-candidate-review-intake
        value={intakeID}
        on:input={(event) => { intakeID = event.currentTarget.value; clearResult(); }}
        placeholder="intake-..."
        autocomplete="off"
      />
    </label>
    <label>
      <span>Adoption ID</span>
      <input
        data-candidate-review-adoption
        value={adoptionID}
        on:input={(event) => { adoptionID = event.currentTarget.value; clearResult(); }}
        placeholder="adoption-..."
        autocomplete="off"
      />
    </label>
    <button data-candidate-review-load type="submit" disabled={!canFetch}>{loading ? 'Loading...' : 'Load review surface'}</button>
  </form>

  {#if error}
    <div class="review-error" role="alert" data-candidate-review-error>{error}</div>
  {/if}

  {#if surface}
    <article class="surface-card" data-candidate-review-surface>
      <div class="surface-heading">
        <div>
          <p class="eyebrow">{surface.artifact_kind || 'review surface'}</p>
          <h2>{surface.state || 'reviewable'}</h2>
        </div>
        <div class="pill-row">
          <span class="pill" data-candidate-review-deployment>{surface.deployment_state || 'non_deployed'}</span>
          <span class="pill">{surface.review_scope || 'non-deployed-candidate-package-source-lineage'}</span>
        </div>
      </div>

      <dl class="provenance-grid">
        <div><dt>Package</dt><dd>{surface.package_id || surface.candidate_package_id || 'unknown'}</dd></div>
        <div><dt>App</dt><dd>{surface.app_id || 'unknown'}</dd></div>
        <div><dt>Target computer</dt><dd>{surface.target_computer_id || 'unknown'}</dd></div>
        <div><dt>Candidate source</dt><dd>{surface.candidate_source_ref || 'unknown'}</dd></div>
        <div><dt>Acceptance</dt><dd>{surface.local_acceptance_id || surface.acceptance_evidence?.acceptance_id || 'unknown'}</dd></div>
        <div><dt>Acceptance level</dt><dd>{surface.local_acceptance_level || surface.acceptance_evidence?.acceptance_level || 'unknown'}</dd></div>
      </dl>

      <section class="actions-panel" data-candidate-review-actions>
        <h3>Available actions</h3>
        <ul>
          {#each surface.allowed_actions || [] as action}
            <li>{action}</li>
          {/each}
        </ul>
      </section>

      <section class="activation-panel" data-candidate-review-activation-boundary>
        <h3>Owner activation decision</h3>
        <p>This prepares the owner decision boundary from accepted local evidence. It does not activate, publish, promote, create run acceptance, or call AppAdoption mutation routes.</p>
        <button
          class="primary-action"
          data-candidate-review-prepare-activation
          type="button"
          on:click={prepareActivationDecision}
        >
          Prepare activation decision
        </button>
        {#if activationDecision}
          <div class="decision-summary" data-candidate-review-activation-summary>
            <strong>{activationDecision.state}</strong>
            <p>Acceptance: {activationDecision.usesAcceptanceID || 'local acceptance evidence referenced by review surface'}</p>
            <p>Next boundary: {activationDecision.nextBoundary}</p>
            <p>{activationDecision.noMutation ? 'No mutation was performed.' : 'Mutation boundary is not allowed here.'}</p>
            <div class="decision-lists">
              <div>
                <h4>Required contracts</h4>
                <ul>
                  {#each activationDecision.requiredContracts as contract}
                    <li>{contract}</li>
                  {/each}
                </ul>
              </div>
              <div>
                <h4>Blocked routes</h4>
                <ul>
                  {#each activationDecision.blockedRoutes as route}
                    <li>{route}</li>
                  {/each}
                </ul>
              </div>
            </div>
          </div>
        {/if}
      </section>

      <section class="boundary-panel" data-candidate-review-boundaries>
        <h3>Blocked boundaries</h3>
        <div class="boundary-grid">
          {#each boundaryRows as [label, state]}
            <div class="boundary-row">
              <span>{label}</span>
              <strong>{state || 'blocked'}</strong>
            </div>
          {/each}
        </div>
      </section>

      <section class="evidence-panel" data-candidate-review-evidence>
        <h3>Accepted local evidence</h3>
        <p>{surface.acceptance_evidence?.acceptance_id || surface.local_acceptance_id || 'Acceptance evidence is referenced by the review surface.'}</p>
        <p>{surface.acceptance_evidence?.residual_risks?.join(' ') || 'No deployed acceptance is claimed by this surface.'}</p>
      </section>
    </article>
  {:else if !error}
    <div class="empty-state" data-candidate-review-empty>
      Enter an intake/adoption pair from an accepted local source-lineage review to inspect its non-deployed review surface.
    </div>
  {/if}
</section>

<style>
  .candidate-review-app {
    display: grid;
    gap: 1rem;
    padding: 1rem;
    min-height: 100%;
    color: var(--choir-fg);
  }

  .hero,
  .surface-card,
  .review-form,
  .empty-state,
  .review-error {
    border: 1px solid color-mix(in srgb, var(--choir-fg) 12%, transparent);
    border-radius: 18px;
    background: color-mix(in srgb, var(--choir-panel, #111827) 92%, transparent);
    box-shadow: 0 18px 40px rgba(0, 0, 0, 0.18);
  }

  .hero {
    display: flex;
    justify-content: space-between;
    gap: 1rem;
    padding: 1.1rem;
  }

  .hero h1,
  .surface-heading h2 {
    margin: 0;
  }

  .eyebrow {
    margin: 0 0 0.35rem;
    color: var(--choir-muted);
    font-size: 0.72rem;
    font-weight: 760;
    letter-spacing: 0.08em;
    text-transform: uppercase;
  }

  .lede {
    max-width: 58rem;
    margin: 0.45rem 0 0;
    color: var(--choir-muted);
    line-height: 1.45;
  }

  .status-card {
    min-width: 12rem;
    padding: 0.8rem;
    border-radius: 14px;
    background: color-mix(in srgb, var(--choir-accent, #60a5fa) 14%, transparent);
  }

  .status-card span,
  .status-card small {
    display: block;
    color: var(--choir-muted);
  }

  .status-card strong {
    display: block;
    margin: 0.25rem 0;
    font-size: 1.1rem;
  }

  .review-form {
    display: grid;
    grid-template-columns: minmax(0, 1fr) minmax(0, 1fr) auto;
    gap: 0.75rem;
    align-items: end;
    padding: 1rem;
  }

  .review-form label {
    display: grid;
    gap: 0.35rem;
    color: var(--choir-muted);
    font-size: 0.82rem;
    font-weight: 700;
  }

  .review-form input {
    width: 100%;
    box-sizing: border-box;
    border: 1px solid color-mix(in srgb, var(--choir-fg) 18%, transparent);
    border-radius: 10px;
    padding: 0.65rem 0.75rem;
    background: color-mix(in srgb, var(--choir-bg, #020617) 78%, transparent);
    color: var(--choir-fg);
  }

  .review-form button {
    border: 0;
    border-radius: 999px;
    padding: 0.7rem 1rem;
    background: var(--choir-accent, #60a5fa);
    color: #04111f;
    font-weight: 800;
    cursor: pointer;
  }

  .review-form button:disabled {
    opacity: 0.55;
    cursor: not-allowed;
  }

  .review-error {
    padding: 0.9rem 1rem;
    color: var(--choir-danger, #f87171);
  }

  .empty-state {
    padding: 1rem;
    color: var(--choir-muted);
  }

  .surface-card {
    display: grid;
    gap: 1rem;
    padding: 1rem;
  }

  .surface-heading {
    display: flex;
    justify-content: space-between;
    gap: 1rem;
  }

  .pill-row {
    display: flex;
    flex-wrap: wrap;
    gap: 0.4rem;
    justify-content: flex-end;
  }

  .pill {
    display: inline-flex;
    align-items: center;
    border-radius: 999px;
    padding: 0.35rem 0.6rem;
    background: color-mix(in srgb, var(--choir-accent, #60a5fa) 14%, transparent);
    color: var(--choir-fg);
    font-size: 0.78rem;
    font-weight: 760;
  }

  .provenance-grid,
  .boundary-grid {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 0.65rem;
  }

  .provenance-grid div,
  .boundary-row,
  .actions-panel,
  .activation-panel,
  .boundary-panel,
  .evidence-panel,
  .decision-summary {
    border: 1px solid color-mix(in srgb, var(--choir-fg) 10%, transparent);
    border-radius: 14px;
    padding: 0.8rem;
    background: color-mix(in srgb, var(--choir-bg, #020617) 42%, transparent);
  }

  dt,
  .boundary-row span {
    color: var(--choir-muted);
    font-size: 0.76rem;
    font-weight: 760;
    text-transform: uppercase;
    letter-spacing: 0.05em;
  }

  dd {
    margin: 0.25rem 0 0;
    overflow-wrap: anywhere;
    font-weight: 720;
  }

  .activation-panel {
    display: grid;
    gap: 0.75rem;
  }

  .activation-panel p,
  .decision-summary p {
    margin: 0;
    color: var(--choir-muted);
    overflow-wrap: anywhere;
  }

  .primary-action {
    width: fit-content;
    border: 0;
    border-radius: 999px;
    padding: 0.65rem 0.9rem;
    background: var(--choir-accent, #60a5fa);
    color: #04111f;
    font-weight: 800;
    cursor: pointer;
  }

  .decision-summary {
    display: grid;
    gap: 0.6rem;
  }

  .decision-summary strong {
    overflow-wrap: anywhere;
  }

  .decision-lists {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 0.75rem;
  }

  .decision-lists h4 {
    margin: 0 0 0.35rem;
    color: var(--choir-muted);
    font-size: 0.78rem;
    text-transform: uppercase;
    letter-spacing: 0.05em;
  }

  .decision-lists ul {
    margin: 0;
    padding-left: 1.1rem;
    color: var(--choir-muted);
    overflow-wrap: anywhere;
  }

  .actions-panel h3,
  .activation-panel h3,
  .boundary-panel h3,
  .evidence-panel h3 {
    margin: 0 0 0.6rem;
  }

  .actions-panel ul {
    display: flex;
    flex-wrap: wrap;
    gap: 0.4rem;
    margin: 0;
    padding: 0;
    list-style: none;
  }

  .actions-panel li {
    border-radius: 999px;
    padding: 0.35rem 0.55rem;
    background: color-mix(in srgb, var(--choir-success, #34d399) 16%, transparent);
    font-weight: 760;
  }

  .boundary-row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 0.75rem;
  }

  .boundary-row strong {
    color: var(--choir-muted);
  }

  .evidence-panel p {
    margin: 0.35rem 0 0;
    color: var(--choir-muted);
    overflow-wrap: anywhere;
  }

  @media (max-width: 760px) {
    .hero,
    .surface-heading {
      display: grid;
    }

    .review-form,
    .provenance-grid,
    .boundary-grid,
    .decision-lists {
      grid-template-columns: 1fr;
    }
  }
</style>
