<script>
  // "Updates available - Restart to update" (docs/design/update-now-2026-11-09.md
  // item 10). A persistent notice, not a toast: it shows while vmctl has a
  // pending update for this computer (GET /api/computers/{id}/update, read at
  // load, on live-channel reconnect and every 5 minutes), and a click confirms
  // and asks for the owner update (POST .../lifecycle/update). The outcome:
  //   refreshed (201)         "Restarting..." until the live channel reconnects
  //                           and the signal clears; reload if the frontend
  //                           build changed
  //   current (200)           "Already up to date", the chip clears
  //   deferred/skipped (409)  the refusal in plain words; the chip stays
  //   timeout / 502 / 504     the proxy gave up waiting, the act did not stop:
  //                           treated as "Restarting..."
  import { onDestroy, onMount } from 'svelte';
  import { fetchWithRenewal } from './auth.js';

  export let computerId = '';
  export let liveStatus = '';

  const POLL_MS = 5 * 60 * 1000;
  const RESTART_POLL_MS = 4000;
  const RESTART_GIVE_UP_MS = 4 * 60 * 1000;
  const CONFIRM_POLL_MS = 3000;
  const CONFIRM_GIVE_UP_MS = 90 * 1000;

  let available = false;
  let target = '';
  let phase = 'idle'; // idle | confirm | sending | confirming | restarting | current
  let message = '';
  let timer = null;
  let restartTimer = null;
  let restartStarted = 0;
  let sawDisconnect = false;
  let lastLive = '';
  let mountedScript = '';
  let destroyed = false;

  $: path = `/api/computers/${encodeURIComponent(computerId)}`;
  $: if (liveStatus !== lastLive) onLiveStatus(liveStatus);

  function moduleScript(html) {
    const m = /<script[^>]+type="module"[^>]+src="([^"]+)"/.exec(html || '');
    return m ? m[1] : '';
  }

  function onLiveStatus(next) {
    const prev = lastLive;
    lastLive = next;
    if (phase === 'restarting' && next !== 'connected') sawDisconnect = true;
    if (next === 'connected' && prev !== 'connected') {
      if (phase === 'restarting' && sawDisconnect) void finishRestartIfCleared();
      else void readSignal();
    }
  }

  async function readSignal() {
    if (!computerId || destroyed) return null;
    try {
      const res = await fetchWithRenewal(`${path}/update`, { headers: { Accept: 'application/json' } });
      if (!res.ok) return null;
      const body = await res.json();
      const signal = body?.update_available || null;
      if (phase !== 'restarting' && phase !== 'confirming' && phase !== 'current') {
        available = Boolean(signal);
        target = signal?.target || '';
      }
      return signal;
    } catch {
      return null;
    }
  }

  async function frontendBuildChanged() {
    try {
      const res = await fetch('/', { cache: 'no-store', headers: { Accept: 'text/html' } });
      if (!res.ok) return false;
      const now = moduleScript(await res.text());
      return Boolean(mountedScript && now && now !== mountedScript);
    } catch {
      return false;
    }
  }

  async function finishRestartIfCleared() {
    const signal = await readSignal();
    if (signal || destroyed) return false;
    stopRestartPoll();
    phase = 'idle';
    available = false;
    message = '';
    if (await frontendBuildChanged()) window.location.reload();
    return true;
  }

  function stopRestartPoll() {
    if (restartTimer) clearInterval(restartTimer);
    restartTimer = null;
  }

  function beginRestarting() {
    phase = 'restarting';
    message = '';
    restartStarted = Date.now();
    sawDisconnect = false;
    stopRestartPoll();
    restartTimer = setInterval(async () => {
      if (Date.now() - restartStarted > RESTART_GIVE_UP_MS) {
        stopRestartPoll();
        phase = 'idle';
        await readSignal();
        return;
      }
      // A signal that cleared while the guest was away is also the answer.
      if (sawDisconnect || Date.now() - restartStarted > 8000) await finishRestartIfCleared();
    }, RESTART_POLL_MS);
  }

  // The computer's realization epoch and state, from the lifecycle status route.
  async function readStatus() {
    try {
      const res = await fetchWithRenewal(`${path}/lifecycle/status`, { headers: { Accept: 'application/json' } });
      if (!res.ok) return { reachable: false };
      const body = await res.json();
      return { reachable: true, epoch: Number(body?.realization_epoch), state: String(body?.state || '') };
    } catch {
      return { reachable: false };
    }
  }

  // The proxy gave up (502/504/network) but the act does not stop. Say
  // "Restarting..." only once something shows it began: the epoch moved, or the
  // computer is no longer reachable and active. Otherwise give a plain message.
  function confirmRestartStarted(epochBefore) {
    phase = 'confirming';
    message = '';
    const started = Date.now();
    const tick = async () => {
      if (destroyed || phase !== 'confirming') return;
      const status = await readStatus();
      const moved = status.reachable && Number.isFinite(epochBefore) && Number.isFinite(status.epoch) && status.epoch > epochBefore;
      const away = !status.reachable || (status.state && status.state !== 'active');
      if (moved || away) {
        beginRestarting();
        return;
      }
      if (Date.now() - started > CONFIRM_GIVE_UP_MS) {
        phase = 'idle';
        message = 'The restart could not be confirmed. Try again.';
        return;
      }
      setTimeout(tick, CONFIRM_POLL_MS);
    };
    setTimeout(tick, CONFIRM_POLL_MS);
  }

  const REFUSALS = {
    ring_waits: 'Waiting for its turn in the rollout.',
    held: 'This computer is on hold, so it cannot restart for an update.',
    divergent: 'This computer runs its own version, so it is not updated automatically.',
    not_active: 'The computer is not running. It will start on the new version.',
    route_refused: 'The computer\'s version route is not ready. Try again shortly.',
    no_release: 'There is no release to update to yet.',
    live_capsule: 'An engineering sandbox is running; it can be paused for the restart once sandbox pause lands.',
    selfdev_active: 'A self-development change is being applied. Try again in a moment.',
    selfdev_awaiting_decision: 'A self-development change is waiting for your decision.',
    snapshot_in_flight: 'A snapshot is being taken. Try again in a moment.',
    hibernate_fenced: 'The computer is going to sleep. Try again after it wakes.',
  };

  function refusalText(body) {
    const reason = String(body?.reason || '');
    if (REFUSALS[reason]) return REFUSALS[reason];
    if (reason === 'refresh_in_flight') return '';
    return 'The computer is busy right now. It will update at its next quiet moment.';
  }

  async function restart() {
    phase = 'sending';
    message = '';
    const epochBefore = (await readStatus()).epoch;
    let res;
    try {
      res = await fetchWithRenewal(`${path}/lifecycle/update`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ idempotency_key: crypto.randomUUID() }),
      });
    } catch {
      // The proxy's wait ended or the link dropped; the act does not stop.
      confirmRestartStarted(epochBefore);
      return;
    }
    let body = null;
    try { body = await res.json(); } catch { body = null; }
    if (res.status === 201) {
      beginRestarting();
    } else if (res.status === 200 && body?.outcome === 'current') {
      phase = 'current';
      message = 'Already up to date';
      setTimeout(() => {
        if (destroyed) return;
        phase = 'idle';
        available = false;
        message = '';
      }, 2500);
    } else if (res.status === 409) {
      const text = refusalText(body);
      if (text === '') {
        beginRestarting(); // a second click while one is in flight
      } else {
        phase = 'idle';
        message = text;
      }
    } else if (res.status === 502 || res.status === 504) {
      confirmRestartStarted(epochBefore);
    } else if (res.status === 500 && body?.outcome === 'failed') {
      phase = 'idle';
      message = 'The restart failed. Try again.';
    } else {
      phase = 'idle';
      message = 'The restart could not be started. Try again.';
    }
  }

  onMount(() => {
    mountedScript = moduleScript(document.documentElement.outerHTML) || document.querySelector('script[type="module"][src]')?.getAttribute('src') || '';
    void readSignal();
    timer = setInterval(() => void readSignal(), POLL_MS);
  });

  onDestroy(() => {
    destroyed = true;
    if (timer) clearInterval(timer);
    stopRestartPoll();
  });
</script>

{#if available || phase === 'restarting' || phase === 'confirming' || phase === 'current'}
  <div class="update-chip" data-update-chip data-update-state={phase === 'idle' ? 'available' : phase} role="status" aria-live="polite">
    {#if phase === 'restarting'}
      <span class="label" data-update-label>Restarting...</span>
    {:else if phase === 'confirming'}
      <span class="label" data-update-label>Checking...</span>
    {:else if phase === 'current'}
      <span class="label" data-update-label>{message}</span>
    {:else if phase === 'confirm'}
      <span class="label" data-update-label>Running work resumes after the restart.</span>
      <button type="button" class="act primary" data-update-confirm on:click={restart}>Restart</button>
      <button type="button" class="act" data-update-cancel on:click={() => (phase = 'idle')}>Not now</button>
    {:else}
      <button type="button" class="chip" data-update-open disabled={phase === 'sending'} title={target ? `Release ${target.slice(0, 8)}` : ''} on:click={() => { message = ''; phase = 'confirm'; }}>
        <span class="dot" aria-hidden="true"></span>
        <span data-update-label>Updates available - Restart to update</span>
      </button>
      {#if message}
        <span class="note" data-update-message>{message}</span>
      {/if}
    {/if}
  </div>
{/if}

<style>
  .update-chip {
    position: fixed;
    top: 12px;
    right: 16px;
    z-index: 9000;
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 8px;
    max-width: calc(100vw - 32px);
    padding: 6px 10px;
    border: 1px solid var(--choir-border-strong, currentColor);
    border-radius: 999px;
    background: var(--choir-panel, var(--choir-bg));
    color: var(--choir-text-accent, inherit);
    font-size: 13px;
    box-shadow: 0 6px 24px color-mix(in srgb, var(--choir-shadow-color, #000) 30%, transparent);
  }
  .chip,
  .act {
    display: inline-flex;
    align-items: center;
    gap: 8px;
    border: 0;
    background: transparent;
    color: inherit;
    font: inherit;
    cursor: pointer;
    padding: 2px 4px;
  }
  .chip:disabled {
    opacity: 0.6;
    cursor: default;
  }
  .act {
    border: 1px solid var(--choir-border-strong, currentColor);
    border-radius: 999px;
    padding: 2px 10px;
  }
  .act.primary {
    font-weight: 600;
  }
  .dot {
    width: 8px;
    height: 8px;
    border-radius: 50%;
    background: var(--choir-status-warning, currentColor);
  }
  .note {
    flex-basis: 100%;
    opacity: 0.85;
  }
  @media (max-width: 520px) {
    .update-chip {
      left: 16px;
      right: 16px;
      border-radius: 14px;
    }
  }
</style>
