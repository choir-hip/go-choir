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
  return { v: 3, pending: {}, held: {}, history: [], boots: {}, planned: {} };
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
        v: 3,
        pending: obj(parsed.pending),
        held: obj(parsed.held),
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
      const empty = !state.history.length && ['pending', 'held', 'boots', 'planned'].every((k) => !Object.keys(state[k]).length);
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
    capMap(state.held, RECORD_CAP, (h) => h.since);
    write(state);
    return out;
  }

  const entryKey = (windowId) => `${windowId}|${pageId}`;
  // Dead: any page but this one that is not demonstrably alive right now.
  const isDead = (page, alive) => page !== pageId && !alive.has(page);

  // A dead page's marker becomes a durable hold (state.held), counted once per
  // key per call. A planned reload excuses a marker only when no foreground
  // step was still in flight (a hung open is not laundered). Auto-open entries
  // are keyed by app and route/context (their window id is fresh every load);
  // others by window id, and only when the server still has the window.
  function absorbDead(state, alive, restored, onlyAuto) {
    const dead = {};
    const counted = new Set();
    for (const [k, entry] of Object.entries(state.pending)) {
      if (!isDead(entry.page, alive)) continue;
      if (onlyAuto && !entry.autoKey) continue;
      delete state.pending[k];
      if (state.planned[entry.page] && !entry.busy) continue;
      const holdKey = entry.autoKey ? `a:${entry.autoKey}` : restored && restored.has(entry.windowId) ? `w:${entry.windowId}` : '';
      if (!holdKey) continue;
      const h = state.held[holdKey] || { key: holdKey, windowId: entry.autoKey ? '' : entry.windowId, appId: entry.appId || '', title: entry.title || '', count: 0 };
      if (!counted.has(holdKey)) {
        counted.add(holdKey);
        h.count = (Number(h.count) || 0) + 1;
        h.since = now();
        state.history.push({
          windowId: entry.windowId,
          appId: entry.appId || '',
          title: entry.title || '',
          step: entry.step || '',
          userAgent,
          at: now(),
          crashes: h.count,
        });
        state.history = state.history.slice(-HISTORY_LIMIT);
      }
      h.step = entry.step || h.step || '';
      state.held[holdKey] = h;
      if (!entry.autoKey) dead[entry.windowId] = h.count;
    }
    return dead;
  }

  return {
    /**
     * Call once per page load. interrupted: a previous page of this device never
     * became stable (planned reloads are clean). previousDead: that, or any dead
     * page's pending marker. Dead automatic opens are held here (no restore needed).
     */
    beginBoot(alive = new Set()) {
      return mutate((state) => {
        let interrupted = false;
        for (const page of Object.keys(state.boots)) {
          if (!isDead(page, alive)) continue;
          if (!state.planned[page]) interrupted = true;
          delete state.boots[page];
        }
        const deadPending = Object.values(state.pending).some((e) => isDead(e.page, alive) && !(state.planned[e.page] && !e.busy));
        absorbDead(state, alive, null, true);
        state.boots[pageId] = now();
        return { interrupted, previousDead: interrupted || deadPending };
      }) || { interrupted: false, previousDead: false };
    },
    /** The desktop has been stable for the quiet period. */
    bootStable() {
      mutate((state) => {
        delete state.boots[pageId];
      });
    },
    /** Synchronously, before an app body mounts. opts.autoKey: an automatic open (route, replay). */
    markPending(win, opts = {}) {
      mutate((state) => {
        const entry = {
          windowId: win.windowId,
          appId: win.appId || '',
          title: win.title || '',
          since: now(),
          page: pageId,
        };
        if (opts.autoKey) entry.autoKey = String(opts.autoKey);
        state.pending[entryKey(win.windowId)] = entry;
      });
    },
    /**
     * Proven, or closed: forget this page's marker and the hold (a window's, and
     * the automatic open's by key). A Reopen does not call this; only proof or Close.
     */
    settle(windowId, opts = {}) {
      mutate((state) => {
        const entry = state.pending[entryKey(windowId)];
        const autoKey = opts.autoKey || entry?.autoKey || '';
        delete state.pending[entryKey(windowId)];
        delete state.held[`w:${windowId}`];
        if (autoKey) delete state.held[`a:${autoKey}`];
      });
    },
    /**
     * An app body says where it is, synchronously, so a death names the step
     * it was in. opts.win lets a foreground step on an already proven window
     * arm it again; a background step never does. A foreground step without
     * done marks work in flight (a planned reload does not excuse it).
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
          if (opts.autoKey) entry.autoKey = String(opts.autoKey);
        }
        entry.step = String(text || '').slice(0, STEP_LIMIT);
        entry.stepAt = now();
        if (!opts.background) entry.busy = !opts.done;
      });
    },
    /** The step a held window was in when the page died ('' when unknown). */
    lastStep(windowId, autoKey = '') {
      const state = read();
      if (!state) return '';
      return String(state.held[`w:${windowId}`]?.step || (autoKey ? state.held[`a:${autoKey}`]?.step : '') || '');
    },
    /** The last HISTORY_LIMIT deaths on this device, oldest first, for a later report. */
    history() {
      const state = read();
      return state ? state.history : [];
    },
    /**
     * At the first restore: dead pages' markers for restored windows become
     * holds (one count per window per call). Returns {windowId: count} for the
     * deaths absorbed now. Markers of windows the server no longer has are pruned;
     * markers of pages that are alive, and this page's own, are left alone.
     */
    takeDeaths(restoredIds = [], alive = new Set()) {
      return mutate((state) => {
        const dead = absorbDead(state, alive, new Set(restoredIds), false);
        for (const page of Object.keys(state.planned)) {
          if (isDead(page, alive)) delete state.planned[page];
        }
        return dead;
      }) || {};
    },
    /**
     * The durable holds for restored windows: {windowId: count}, every load,
     * with no dependence on boot markers or liveness. A hold for a window the
     * server no longer has is pruned.
     */
    holds(restoredIds = []) {
      return mutate((state) => {
        const restored = new Set(restoredIds);
        const out = {};
        for (const [k, h] of Object.entries(state.held)) {
          if (!k.startsWith('w:')) continue;
          if (restored.has(h.windowId)) out[h.windowId] = Number(h.count) || 1;
          else delete state.held[k];
        }
        return out;
      }) || {};
    },
    /** Count of an automatic open's hold (0 when none), keyed by app and route/context. */
    autoHeld(autoKey) {
      const state = read();
      return state ? Number(state.held[`a:${autoKey}`]?.count) || 0 : 0;
    },
    /** The previous boot died: hold this automatic open, durably. Returns its count. */
    holdAuto(autoKey, meta = {}) {
      return mutate((state) => {
        const k = `a:${autoKey}`;
        const h = state.held[k] || { key: k, windowId: '', appId: meta.appId || '', title: meta.title || '', count: 1, since: now() };
        state.held[k] = h;
        return Number(h.count) || 1;
      }) || 0;
    },
    /** A planned reload (an update, an account switch): this page's settled markers are clean at the next load. */
    markPlanned() {
      mutate((state) => {
        state.planned[pageId] = now();
      });
    },
    /** pagehide: only a page with nothing pending left cleanly. Never clears a pending window or a hold. */
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
      return alive; // locks are authoritative: a dead page's lock is gone at once, its heartbeat would linger
    }
  } catch (_err) {
    return new Set();
  }
  try {
    if (storage) {
      const beats = JSON.parse(storage.getItem(HEARTBEAT_KEY) || '{}');
      for (const [tab, at] of Object.entries(beats || {})) {
        if (now() - Number(at) < HEARTBEAT_STALE_MS) alive.add(tab);
      }
    }
  } catch (_err) {
    return new Set();
  }
  return alive;
}

/** Announce this page as alive until it dies: a lock, or a heartbeat where Web Locks are missing or refuse. Returns a stop function. */
export function startPresence({ locks, storage, pageId, now = () => Date.now(), setTimer = setInterval, clearTimer = clearInterval } = {}) {
  let timer = null;
  const startHeartbeat = () => {
    if (timer) return;
    beat();
    timer = setTimer(beat, HEARTBEAT_MS);
  };
  let useLocks = false;
  try {
    if (locks && typeof locks.request === 'function') {
      useLocks = true;
      // Held forever: the browser releases it when the page goes.
      void locks.request(LOCK_PREFIX + pageId, () => new Promise(() => {})).catch(startHeartbeat);
    }
  } catch (_err) {
    useLocks = false;
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
  if (!useLocks) startHeartbeat();
  return () => { if (timer) clearTimer(timer); };
}

export const LIVENESS_TIMEOUT_MS = 2000;

/** Answer fallback when promise has not settled in ms (a never-resolving locks.query() must not hold the boot). */
export function withTimeout(promise, ms, fallback, { setTimer = setTimeout, clearTimer = clearTimeout } = {}) {
  let timer;
  const timeout = new Promise((resolve) => { timer = setTimer(() => resolve(fallback), ms); });
  return Promise.race([Promise.resolve(promise).catch(() => fallback), timeout]).finally(() => clearTimer(timer));
}
