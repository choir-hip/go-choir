<!--
  DeletionCancelOffer — what a passkey sign-in of an account with a pending deletion shows instead of
  signing in (internal/auth/deletion_handlers.go): no session exists; the offer's one-use token
  cancels the deletion and signs the browser in. Nothing was removed inside the 72 hours.

  Data attributes: data-deletion-offer (root), data-deletion-offer-cancel, data-deletion-offer-leave,
  data-deletion-offer-error.
-->
<script lang="ts">
  import { createEventDispatcher } from 'svelte';
  import { cancelAccountDeletion, plainDeletionDate } from './auth.js';

  export let offer: { cancelToken: string; purgeAfter: string };

  const dispatch = createEventDispatcher();
  let working = false;
  let error = '';

  async function cancel() {
    error = '';
    working = true;
    try {
      await cancelAccountDeletion(offer.cancelToken);
      dispatch('cancelled');
    } catch (_err) {
      error = 'Choir could not cancel the deletion just now. Sign in again and try once more.';
      working = false;
    }
  }
</script>

<div class="offer" data-deletion-offer>
  <h2>This account is scheduled for deletion</h2>
  <p>
    Everything is removed for good on {plainDeletionDate(offer.purgeAfter) || 'the date in 72 hours'}. Nothing has been removed yet.
    Cancel the deletion to keep your account and sign in.
  </p>
  {#if error}<p class="error" role="alert" data-deletion-offer-error>{error}</p>{/if}
  <button type="button" class="primary" data-deletion-offer-cancel disabled={working} on:click={cancel}>Cancel deletion</button>
  <button type="button" class="secondary" data-deletion-offer-leave disabled={working} on:click={() => dispatch('leave')}>Leave it scheduled</button>
</div>

<style>
  .offer { display: flex; flex-direction: column; gap: 0.7rem; padding: 0.4rem 0; }
  h2 { margin: 0; font-size: 1.2rem; font-weight: 780; }
  p { margin: 0; font-size: 0.95rem; line-height: 1.45; }
  .primary, .secondary {
    min-height: 3rem; padding: 0.8rem 1rem; font-size: 1rem; font-weight: 820; cursor: pointer;
    border-radius: var(--choir-radius-control, 16px);
  }
  .primary { background: var(--choir-accent); color: var(--choir-text-on-accent, var(--choir-text-primary)); border: none; }
  .secondary { background: transparent; color: var(--choir-text-primary); border: 1px solid var(--choir-border-strong); }
  .error { padding: 0.7rem 0.8rem; border-radius: 14px; background: var(--choir-status-danger-soft); color: var(--choir-status-danger); font-size: 0.9rem; }
</style>
