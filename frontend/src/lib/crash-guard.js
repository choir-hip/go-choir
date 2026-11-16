// Crash guard for the web desktop: a crash never resumes work; a clean load may.
//
// The operating contract's restart rule, at the page layer. A page cannot see
// its own death, so before an app body mounts the window is written to
// `pending`; it is removed only when the window has stayed mounted through a
// quiet period (see createProofClock) or is closed. A reload or pagehide while
// a window is pending does NOT clear it (a user reloading a frozen page is the
// crash case). At the next load a window still pending is restored held, not
// mounted. Per device (browser storage), per tab (sessionStorage tab id), per
// signed-in user (the key). Storage is injected so this runs under node; any
// storage failure makes every call a no-op and the desktop behaves as before.
//
// Record: docs/problems/desktop-restore-reopens-the-window-that-crashed-the-page-crash-loop-2026-11-16.md

export const QUIET_MS = 8000;
export const HOLD_CLOSE_FIRST_AFTER = 2;
// A marker has no age limit (a user back after two days must not re-enter the
// loop): it ends by proof, by close, or when the server no longer has the
// window. Storage is bounded by count, oldest first.
export const PENDING_CAP = 50;
const BOOT_CAP = 20;
export const HEARTBEAT_KEY = 'choir.crashGuard.beat';
export const HEARTBEAT_MS = 2000;
export const HEARTBEAT_STALE_MS = 6000;
export const LOCK_PREFIX = 'choir-desktop-tab-';
export const HISTORY_LIMIT = 10;
const STEP_LIMIT = 200;

function emptyState() {
  return { v: 1, pending: {}, crashes: {}, steps: {}, history: [], boots: {} };
}

export function createCrashGuard({ storage, key, tabId, now = () => Date.now(), userAgent = '' } = {}) {
  function read() {
    try {
      if (!storage) return null;
      const raw = storage.getItem(key);
      if (!raw) return emptyState();
      const parsed = JSON.parse(raw);
      if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) return emptyState();
      const state = {
        v: 1,
        pending: parsed.pending && typeof parsed.pending === 'object' ? { ...parsed.pending } : {},
        crashes: parsed.crashes && typeof parsed.crashes === 'object' ? { ...parsed.crashes } : {},
        steps: parsed.steps && typeof parsed.steps === 'object' ? { ...parsed.steps } : {},
        history: Array.isArray(parsed.history) ? parsed.history.filter((h) => h && typeof h === 'object').slice(-HISTORY_LIMIT) : [],
        boots: parsed.boots && typeof parsed.boots === 'object' && !Array.isArray(parsed.boots) ? { ...parsed.boots } : {},
      };
      for (const [id, entry] of Object.entries(state.pending)) {
        if (!entry || typeof entry !== 'object') delete state.pending[id];
      }
      return state;
    } catch (_err) {
      return null;
    }
  }

  function write(state) {
    try {
      if (!storage) return;
      const empty = !Object.keys(state.boots).length && !state.history.length && !Object.keys(state.pending).length && !Object.keys(state.crashes).length;
      if (empty) storage.removeItem(key);
      else storage.setItem(key, JSON.stringify(state));
    } catch (_err) {
      // Storage blocked or full: the guard is inert.
    }
  }

  function mutate(fn) {
    const state = read();
    if (!state) return undefined;
    const out = fn(state);
    capOldest(state.pending, (e) => e.since, PENDING_CAP);
    capOldest(state.boots, (since) => since, BOOT_CAP);
    write(state);
    return out;
  }

  function capOldest(map, ageOf, cap) {
    const ids = Object.keys(map);
    if (ids.length <= cap) return;
    ids.sort((a, b) => (Number(ageOf(map[a])) || 0) - (Number(ageOf(map[b])) || 0));
    for (const id of ids.slice(0, ids.length - cap)) delete map[id];
  }

  const mine = (entry) => entry && entry.tab === tabId;
  // A marker is a death unless its tab is another tab demonstrably alive now.
  const isDead = (tab, alive) => tab === tabId || !alive.has(tab);

  return {
    /** Call once per page load, before anything mounts. interrupted: the last load of this tab never became stable. */
    beginBoot(alive = new Set()) {
      return mutate((state) => {
        let interrupted = false;
        for (const tab of Object.keys(state.boots)) {
          if (isDead(tab, alive)) {
            interrupted = true;
            delete state.boots[tab];
          }
        }
        state.boots[tabId] = now();
        return { interrupted };
      }) || { interrupted: false };
    },
    /** The desktop has been stable for the quiet period. */
    bootStable() {
      mutate((state) => {
        delete state.boots[tabId];
      });
    },
    /** Synchronously, before an app body mounts. */
    markPending(win) {
      mutate((state) => {
        state.pending[win.windowId] = {
          windowId: win.windowId,
          appId: win.appId || '',
          title: win.title || '',
          since: now(),
          tab: tabId,
        };
      });
    },
    /** Proven, or closed: forget the marker and the death count. */
    settle(windowId) {
      mutate((state) => {
        delete state.pending[windowId];
        delete state.crashes[windowId];
        delete state.steps[windowId];
      });
    },
    /**
     * An app body says where it is, synchronously, so a death names the step
     * it was in. Reaches an app through the SDK (lib/window-step.ts).
     */
    step(windowId, text) {
      mutate((state) => {
        const entry = state.pending[windowId];
        if (!entry) return;
        entry.step = String(text || '').slice(0, STEP_LIMIT);
        entry.stepAt = now();
      });
    },
    /** The step a held window was in when the page died ('' when unknown). */
    lastStep(windowId) {
      const state = read();
      return state ? String(state.steps[windowId] || '') : '';
    },
    /** The last HISTORY_LIMIT deaths on this device, oldest first, for a later report. */
    history() {
      const state = read();
      return state ? state.history : [];
    },
    /**
     * At load: which of the restored windows were pending when this tab last
     * died. Their death count goes up, their markers go (they are held, not
     * mounted). Returns {windowId: deaths}. Markers for windows the server no
     * longer has are pruned. Other tabs' markers are left alone.
     */
    takeDeaths(restoredIds = [], alive = new Set()) {
      return mutate((state) => {
        const restored = new Set(restoredIds);
        const dead = {};
        for (const [id, entry] of Object.entries(state.pending)) {
          if (!isDead(entry.tab, alive)) continue;
          delete state.pending[id];
          if (!restored.has(id)) continue;
          state.crashes[id] = (Number(state.crashes[id]) || 0) + 1;
          dead[id] = state.crashes[id];
          state.steps[id] = entry.step || '';
          state.history.push({
            windowId: id,
            appId: entry.appId || '',
            title: entry.title || '',
            step: entry.step || '',
            userAgent,
            at: now(),
            crashes: state.crashes[id],
          });
          state.history = state.history.slice(-HISTORY_LIMIT);
        }
        return dead;
      }) || {};
    },
    /** Death count of a window (0 when none). */
    deaths(windowId) {
      const state = read();
      return state ? Number(state.crashes[windowId]) || 0 : 0;
    },
    /** pagehide: only a page with nothing pending left cleanly. Never clears a pending window. */
    leaveCleanly() {
      mutate((state) => {
        const anyPending = Object.values(state.pending).some(mine);
        if (!anyPending) delete state.boots[tabId];
      });
    },
  };
}

/**
 * The quiet period: a window is proven after QUIET_MS of the page being alive
 * and visible with the window's body mounted. Hidden time does not count and
 * an unmounted window starts over. advance(mountedIds, dtMs, visible) returns
 * the ids proven by this step.
 */
export function createProofClock({ guard, quietMs = QUIET_MS } = {}) {
  const elapsed = new Map();
  const proven = new Set();
  return {
    advance(mountedIds, dtMs, visible) {
      const mounted = new Set(mountedIds);
      for (const id of [...elapsed.keys()]) if (!mounted.has(id)) elapsed.delete(id);
      for (const id of [...proven]) if (!mounted.has(id)) proven.delete(id);
      if (!visible) return [];
      const newlyProven = [];
      for (const id of mounted) {
        if (proven.has(id)) continue;
        const total = (elapsed.get(id) || 0) + dtMs;
        if (total >= quietMs) {
          elapsed.delete(id);
          proven.add(id);
          guard.settle(id);
          newlyProven.push(id);
        } else {
          elapsed.set(id, total);
        }
      }
      return newlyProven;
    },
  };
}

/**
 * Which tabs are demonstrably alive right now (async). Web Locks: each tab
 * holds a lock named LOCK_PREFIX+id for its lifetime, released when the page
 * dies. Without them: a heartbeat per tab in storage, fresh within
 * HEARTBEAT_STALE_MS. Anything broken answers "nobody", which holds windows.
 */
export async function readLiveTabs({ locks, storage, now = () => Date.now() } = {}) {
  const alive = new Set();
  try {
    if (locks && typeof locks.query === 'function') {
      const snapshot = await locks.query();
      for (const lock of snapshot?.held || []) {
        const name = String(lock?.name || '');
        if (name.startsWith(LOCK_PREFIX)) alive.add(name.slice(LOCK_PREFIX.length));
      }
      return alive;
    }
  } catch (_err) {
    return new Set();
  }
  try {
    if (!storage) return alive;
    const beats = JSON.parse(storage.getItem(HEARTBEAT_KEY) || '{}');
    for (const [tab, at] of Object.entries(beats || {})) {
      if (now() - Number(at) < HEARTBEAT_STALE_MS) alive.add(tab);
    }
  } catch (_err) {
    return new Set();
  }
  return alive;
}

/** Announce this tab as alive until the page dies. Returns a stop function. */
export function startPresence({ locks, storage, tabId, now = () => Date.now(), setTimer = setInterval, clearTimer = clearInterval } = {}) {
  try {
    if (locks && typeof locks.request === 'function') {
      // Held forever: the browser releases it when the page goes.
      void locks.request(LOCK_PREFIX + tabId, () => new Promise(() => {})).catch(() => {});
      return () => {};
    }
  } catch (_err) {
    // fall through to the heartbeat
  }
  const beat = () => {
    try {
      if (!storage) return;
      const beats = JSON.parse(storage.getItem(HEARTBEAT_KEY) || '{}') || {};
      const t = now();
      for (const [tab, at] of Object.entries(beats)) if (t - Number(at) > HEARTBEAT_STALE_MS * 10) delete beats[tab];
      beats[tabId] = t;
      storage.setItem(HEARTBEAT_KEY, JSON.stringify(beats));
    } catch (_err) {
      // inert
    }
  };
  beat();
  const timer = setTimer(beat, HEARTBEAT_MS);
  return () => clearTimer(timer);
}
