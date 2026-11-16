// Crash guard for the web desktop: a crash never resumes work; a clean load may.
//
// The operating contract's restart rule, at the page layer. A page cannot see
// its own death, so before an app body mounts the window is written to
// `pending`; it is removed only when the window has stayed mounted through a
// quiet period (see createProofClock) or is closed. A reload or pagehide while
// a window is pending does NOT clear it (a user reloading a frozen page is the
// crash case). At the next load a window still pending is restored held, not
// mounted. Per device (browser storage), per page load (a nonce), per
// signed-in user (the key). Storage is injected so this runs under node; any
// storage failure makes every call a no-op and the desktop behaves as before.
//
// Record: docs/problems/desktop-restore-reopens-the-window-that-crashed-the-page-crash-loop-2026-11-16.md

export const QUIET_MS = 8000;
// A window whose app announced foreground work and never said it finished is
// proven at this much visible time after the last step: a hang is not a death.
export const BUSY_MAX_MS = 60000;
export const HOLD_CLOSE_FIRST_AFTER = 2;
// A marker has no age limit (a user back after two days must not re-enter the
// loop): it ends by proof, by close, or when the server no longer has the
// window. Storage is bounded by count, oldest first.
export const PENDING_CAP = 50;
export const RECORD_CAP = 200;
const BOOT_CAP = 20;
export const HISTORY_LIMIT = 10;
const STEP_LIMIT = 200;
export const HEARTBEAT_KEY = 'choir.crashGuard.beat';
export const HEARTBEAT_MS = 2000;
export const HEARTBEAT_STALE_MS = 6000;
export const LOCK_PREFIX = 'choir-desktop-tab-';

function emptyState() {
  return { v: 2, pending: {}, crashes: {}, steps: {}, history: [], boots: {}, planned: {} };
}

const obj = (v) => (v && typeof v === 'object' && !Array.isArray(v) ? { ...v } : {});

/**
 * pageId: a nonce new on every page load. A page is "alive" when its lock or
 * heartbeat (named by pageId) says so; every other page's markers are deaths.
 * Pending entries are keyed by window and page, so one page's proof or close
 * never deletes another page's marker.
 */
export function createCrashGuard({ storage, key, pageId, now = () => Date.now(), userAgent = '' } = {}) {
  function read() {
    try {
      if (!storage) return null;
      const raw = storage.getItem(key);
      if (!raw) return emptyState();
      const parsed = JSON.parse(raw);
      if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) return emptyState();
      const state = {
        v: 2,
        pending: obj(parsed.pending),
        crashes: obj(parsed.crashes),
        steps: obj(parsed.steps),
        history: Array.isArray(parsed.history) ? parsed.history.filter((h) => h && typeof h === 'object').slice(-HISTORY_LIMIT) : [],
        boots: obj(parsed.boots),
        planned: obj(parsed.planned),
      };
      for (const [id, entry] of Object.entries(state.pending)) {
        if (!entry || typeof entry !== 'object' || !entry.windowId) delete state.pending[id];
      }
      return state;
    } catch (_err) {
      return null;
    }
  }

  function write(state) {
    try {
      if (!storage) return;
      const empty = !state.history.length && ['pending', 'crashes', 'steps', 'boots', 'planned'].every((k) => !Object.keys(state[k]).length);
      if (empty) storage.removeItem(key);
      else storage.setItem(key, JSON.stringify(state));
    } catch (_err) {
      // Storage blocked or full: the guard is inert.
    }
  }

  // Oldest first for timestamped maps; insertion order for the rest.
  function capMap(map, cap, ageOf) {
    const ids = Object.keys(map);
    if (ids.length <= cap) return;
    if (ageOf) ids.sort((a, b) => (Number(ageOf(map[a])) || 0) - (Number(ageOf(map[b])) || 0));
    for (const id of ids.slice(0, ids.length - cap)) delete map[id];
  }

  function mutate(fn) {
    const state = read();
    if (!state) return undefined;
    const out = fn(state);
    capMap(state.pending, PENDING_CAP, (e) => e.since);
    capMap(state.boots, BOOT_CAP, (since) => since);
    capMap(state.planned, BOOT_CAP, (at) => at);
    capMap(state.crashes, RECORD_CAP);
    capMap(state.steps, RECORD_CAP);
    write(state);
    return out;
  }

  const entryKey = (windowId) => `${windowId}|${pageId}`;
  // Dead: any page but this one that is not demonstrably alive right now.
  const isDead = (page, alive) => page !== pageId && !alive.has(page);

  return {
    /** Call once per page load. interrupted: a previous page of this device never became stable (planned reloads are clean). */
    beginBoot(alive = new Set()) {
      return mutate((state) => {
        let interrupted = false;
        for (const page of Object.keys(state.boots)) {
          if (!isDead(page, alive)) continue;
          if (!state.planned[page]) interrupted = true;
          delete state.boots[page];
        }
        state.boots[pageId] = now();
        return { interrupted };
      }) || { interrupted: false };
    },
    /** The desktop has been stable for the quiet period. */
    bootStable() {
      mutate((state) => {
        delete state.boots[pageId];
      });
    },
    /** Synchronously, before an app body mounts. */
    markPending(win) {
      mutate((state) => {
        state.pending[entryKey(win.windowId)] = {
          windowId: win.windowId,
          appId: win.appId || '',
          title: win.title || '',
          since: now(),
          page: pageId,
        };
      });
    },
    /** Proven, or closed: forget this page's marker and the window's death count and step. */
    settle(windowId) {
      mutate((state) => {
        delete state.pending[entryKey(windowId)];
        delete state.crashes[windowId];
        delete state.steps[windowId];
      });
    },
    /**
     * An app body says where it is, synchronously, so a death names the step
     * it was in. opts.win ({windowId, appId, title}) lets a foreground step on
     * an already proven window arm it again; a background step never does.
     */
    step(windowId, text, opts = {}) {
      mutate((state) => {
        let entry = state.pending[entryKey(windowId)];
        if (!entry) {
          if (opts.background || !opts.win) return;
          entry = state.pending[entryKey(windowId)] = {
            windowId,
            appId: opts.win.appId || '',
            title: opts.win.title || '',
            since: now(),
            page: pageId,
          };
        }
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
     * At load: which of the restored windows have a marker from a dead page.
     * One count per window per call. The dead pages' markers go (the window is
     * held, not mounted); a dead page that announced a planned reload is clean.
     * Markers for windows the server no longer has are pruned; markers of
     * pages that are alive, and this page's own, are left alone.
     */
    takeDeaths(restoredIds = [], alive = new Set()) {
      return mutate((state) => {
        const restored = new Set(restoredIds);
        const dead = {};
        const deadPages = new Set();
        for (const [k, entry] of Object.entries(state.pending)) {
          if (!isDead(entry.page, alive)) continue;
          deadPages.add(entry.page);
          delete state.pending[k];
          if (!restored.has(entry.windowId) || state.planned[entry.page]) continue;
          if (!dead[entry.windowId]) {
            state.crashes[entry.windowId] = (Number(state.crashes[entry.windowId]) || 0) + 1;
            dead[entry.windowId] = state.crashes[entry.windowId];
            state.steps[entry.windowId] = entry.step || '';
            state.history.push({
              windowId: entry.windowId,
              appId: entry.appId || '',
              title: entry.title || '',
              step: entry.step || '',
              userAgent,
              at: now(),
              crashes: state.crashes[entry.windowId],
            });
            state.history = state.history.slice(-HISTORY_LIMIT);
          }
        }
        for (const page of Object.keys(state.planned)) {
          if (isDead(page, alive)) delete state.planned[page];
        }
        return dead;
      }) || {};
    },
    /** A planned reload (an update, an account switch): this page's markers are clean at the next load. */
    markPlanned() {
      mutate((state) => {
        state.planned[pageId] = now();
      });
    },
    /** pagehide: only a page with nothing pending left cleanly. Never clears a pending window. */
    leaveCleanly() {
      mutate((state) => {
        const anyPending = Object.values(state.pending).some((e) => e.page === pageId);
        if (!anyPending) delete state.boots[pageId];
      });
    },
  };
}

/**
 * The quiet period: a window is proven after QUIET_MS of the page being alive
 * and visible with the window's body mounted, counted from the later of its
 * mount and its latest foreground step (noteStep). A foreground step without
 * done means work is in flight: proof then waits for done, or BUSY_MAX_MS.
 * Background steps (a text search pass) do not count. Hidden time does not
 * count and an unmounted window starts over. advance(mountedIds, dtMs,
 * visible) returns the ids proven by this step.
 */
export function createProofClock({ guard, quietMs = QUIET_MS, busyMaxMs = BUSY_MAX_MS } = {}) {
  const elapsed = new Map();
  const busy = new Set();
  const proven = new Set();
  return {
    noteStep(id, { background = false, done = false } = {}) {
      if (background) return;
      proven.delete(id);
      elapsed.set(id, 0);
      if (done) busy.delete(id);
      else busy.add(id);
    },
    advance(mountedIds, dtMs, visible) {
      const mounted = new Set(mountedIds);
      for (const id of [...elapsed.keys()]) if (!mounted.has(id)) { elapsed.delete(id); busy.delete(id); }
      for (const id of [...proven]) if (!mounted.has(id)) proven.delete(id);
      if (!visible) return [];
      const newlyProven = [];
      for (const id of mounted) {
        if (proven.has(id)) continue;
        const total = (elapsed.get(id) || 0) + dtMs;
        if (total >= (busy.has(id) ? busyMaxMs : quietMs)) {
          elapsed.delete(id);
          busy.delete(id);
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
export function startPresence({ locks, storage, pageId, now = () => Date.now(), setTimer = setInterval, clearTimer = clearInterval } = {}) {
  try {
    if (locks && typeof locks.request === 'function') {
      // Held forever: the browser releases it when the page goes.
      void locks.request(LOCK_PREFIX + pageId, () => new Promise(() => {})).catch(() => {});
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
      beats[pageId] = t;
      storage.setItem(HEARTBEAT_KEY, JSON.stringify(beats));
    } catch (_err) {
      // inert
    }
  };
  beat();
  const timer = setTimer(beat, HEARTBEAT_MS);
  return () => clearTimer(timer);
}
