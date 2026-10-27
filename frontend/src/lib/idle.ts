// One owner of "the desktop is idle". After IDLE_MS without input the root
// gets `data-idle` (app.css pauses every CSS animation under it) and
// subscribers (ChoirField) freeze; input or a return to visibility clears it.
// Reason: infinite compositor animations keep the GPU redrawing the page even
// when nothing changes (docs/problems/idle-tab-cpu-after-the-field-fix-2026-10-27.md).

export const IDLE_MS = 120_000;
const EVENTS = ['pointermove', 'pointerdown', 'keydown', 'wheel', 'focus'] as const;

type Listener = (idle: boolean) => void;
const listeners = new Set<Listener>();
let idle = false;
let timer: ReturnType<typeof setTimeout> | null = null;
let installed = false;

// Test hook: localStorage `choir.idleMs` shortens the window.
function idleMs(): number {
  try {
    const v = Number(localStorage.getItem('choir.idleMs'));
    if (v > 0) return v;
  } catch {
    /* storage unavailable */
  }
  return IDLE_MS;
}

function setIdle(next: boolean) {
  if (idle === next) return;
  idle = next;
  document.documentElement.toggleAttribute('data-idle', next);
  for (const fn of listeners) fn(next);
}

function poke() {
  if (timer) clearTimeout(timer);
  setIdle(false);
  timer = setTimeout(() => setIdle(true), idleMs());
}

function onVisibility() {
  if (document.visibilityState === 'visible') poke();
}

export function isIdle(): boolean {
  return idle;
}

/** Idempotent; starts the idle timer once for the page. */
export function installIdleWatcher() {
  if (installed || typeof window === 'undefined') return;
  installed = true;
  for (const ev of EVENTS) window.addEventListener(ev, poke, { passive: true });
  document.addEventListener('visibilitychange', onVisibility);
  poke();
}

/** Subscribes and calls fn with the current state at once, so a subscriber
 * that arrives while the page is idle freezes too (review 347 M1). */
export function onIdleChange(fn: Listener): () => void {
  installIdleWatcher();
  listeners.add(fn);
  fn(idle);
  return () => listeners.delete(fn);
}
