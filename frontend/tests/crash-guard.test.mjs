// node --test tests/crash-guard.test.mjs
//
// FAILURE LIST (written before the code; owner, 2026-11-16: "No crash loops").
// The page cannot see its own death, so a marker is written before an app body
// mounts and cleared only when the window has proven itself. A page is
// identified by a nonce that is new on every load (pageId), never by anything
// a lost sessionStorage could change.
//
//  F1  A window pending when the page died is reported dead at the next load,
//      with a count of 1, and is no longer pending (it is held, not mounted).
//  F2  A window proven before the page died is not reported dead.
//  F3  Closing a window (settle) clears its pending marker and its death count.
//  F4  A second death of the same window reports a count of 2 (reopen marks it
//      pending again; the count survives the hold).
//  F5  Proving a window resets its death count.
//  F6  (round 4) A page that leaves alive (pagehide, hidden) takes its own
//      markers and boot marker with it; a hung or killed page runs nothing, so
//      its markers stay. Except a foreground step in flight (no done).
//  F7  Leaving clears the boot marker; rejoin (shown again, unstable) restores it.
//  F8  A boot marker left by the previous load is reported as an interrupted
//      boot; once the desktop was stable it is not.
//  F9  A marker from another page is left alone while that page is
//      demonstrably alive, and is a death when it is not (a new tab, a lost
//      sessionStorage, a duplicated tab cannot walk back into the loop).
//  F10 Storage that is missing, throws, or holds garbage never throws, and
//      reports nothing dead: the desktop behaves as today.
//  F11 Markers never expire by age; storage is bounded by count only.
//  F12 A marker for a window the server no longer has is pruned.
//  F13 The quiet period counts only visible time, restarts when a window is
//      unmounted, proves each window once.
//  F14 Boot interrupted with no window identified is the page-level backstop.
//  F15 The step an app reported last survives into the held record, the
//      last-step lookup and a per-device history capped at 10.
//  F16 Liveness: Web Locks only; without them, or on error, nobody is alive.
//  F17 (review B2) This page's own markers are never dead for itself, so the
//      guard can be armed and windows marked before the first restore runs.
//  F18 (review B4) Markers are per (window, page): another page's proof or
//      close does not delete this page's marker nor the reverse; two dead
//      pages with a marker for one window are one death, not two.
//  F19 (review B3) Proof needs the quiet period after the mount AND after the
//      latest foreground step; background steps do not count; a window that
//      announced work and never finished is proven only at the busy ceiling;
//      a foreground step on a proven window arms it again.
//  F22 (round 2) beginBoot reports previousDead: an interrupted boot OR any
//      dead page's pending marker so an automatic
//      open at load (a route, a replay: a fresh window id, unmatchable) can be
//      held when the last boot of this device died. An alive page's markers
//      and this page's own do not count.
//  F21 (review nb) holds are capped by count.
//  F23 (round 3) A hold is durable per-device state: held, then any number of
//      reloads (no markers, no liveness), still held.
//  F24 Reopen does not end a hold: Reopen then die is held again with count 2;
//      Reopen then reload before proof is held with the count up.
//  F25 Reopen then proof ends the hold and resets the count: the next load mounts.
//  F26 Close ends the hold; the next load does not have it.
//  F27 A hold for a window the server no longer has is pruned.
//  F28 An automatic open (fresh window id) dies: held by app+route key, durably;
//      holdAuto records a hold when the previous boot died; proof or Close by key ends it.
//  F29 A hold survives a page leaving alive.
//  F30 A liveness read that never resolves is bounded (withTimeout) and answers 'nobody alive'.
//  F32 (round 3) The body may mount iff crashHoldFor(win, held, reopened) is 0:
//      held by window id or by automatic-open key, unless Reopened in this page.
//      No window object carries the hold, so no path can skip it.
//  F33 Suspension (unmark) drops only the marker, never a hold; only proof,
//      Close or a prune against a SUCCESSFULLY fetched list end a hold.
//  F34 A failed fetch (takeDeaths(null)) holds every dead window by id and
//      prunes nothing; holds() against a real list prunes the absent ones.
//  F35 (round 4) leave then rejoin: markers are written again without
//      overwriting an entry still there (its step and in-flight flag are kept).
//  F36 A dead automatic open holds its key AND the window id it created; a
//      restored window with that id is held, and the id is pruned like any other.
import test from 'node:test';
import assert from 'node:assert/strict';
import {
  createCrashGuard,
  createProofClock,
  QUIET_MS,
  BUSY_MAX_MS,
  HOLD_CLOSE_FIRST_AFTER,
  PENDING_CAP,
  withTimeout,
  crashHoldFor,
  RECORD_CAP,
  readLiveTabs,
} from '../src/lib/crash-guard.js';

function memory() {
  const m = new Map();
  return {
    getItem: (k) => (m.has(k) ? m.get(k) : null),
    setItem: (k, v) => void m.set(k, String(v)),
    removeItem: (k) => void m.delete(k),
    raw: m,
  };
}
const win = (id, appId = 'reader') => ({ windowId: id, appId, title: `T ${id}` });
let clockNow = 1000;
const pageOn = (storage, pageId, extra = {}) =>
  createCrashGuard({ storage, key: 'k', pageId, now: () => clockNow, ...extra });
const none = new Set();

test('F1 a pending window is dead at the next load', () => {
  const s = memory();
  const g1 = pageOn(s, 'p1');
  g1.beginBoot(none);
  g1.markPending(win('w1'));
  const g2 = pageOn(s, 'p2');
  const boot = g2.beginBoot(none);
  assert.deepEqual(g2.takeDeaths(['w1', 'w2'], none), { w1: 1 });
  assert.equal(boot.interrupted, true);
  assert.deepEqual(pageOn(s, 'p3').takeDeaths(['w1'], none), {}, 'no longer pending once held');
});

test('F2 a proven window is not dead', () => {
  const s = memory();
  const g1 = pageOn(s, 'p1');
  g1.markPending(win('w1'));
  g1.settle('w1');
  assert.deepEqual(pageOn(s, 'p2').takeDeaths(['w1'], none), {});
});

test('F3 closing clears pending and the count', () => {
  const s = memory();
  pageOn(s, 'p1').markPending(win('w1'));
  pageOn(s, 'p2').takeDeaths(['w1'], none);
  const g = pageOn(s, 'p3');
  g.markPending(win('w1'));
  g.settle('w1');
  assert.deepEqual(pageOn(s, 'p4').takeDeaths(['w1'], none), {});
  pageOn(s, 'p4').markPending(win('w1'));
  assert.deepEqual(pageOn(s, 'p5').takeDeaths(['w1'], none), { w1: 1 });
});

test('F4 a second death counts two', () => {
  const s = memory();
  pageOn(s, 'p1').markPending(win('w1'));
  assert.deepEqual(pageOn(s, 'p2').takeDeaths(['w1'], none), { w1: 1 });
  pageOn(s, 'p3').markPending(win('w1')); // reopened
  assert.deepEqual(pageOn(s, 'p4').takeDeaths(['w1'], none), { w1: 2 });
  assert.ok(2 >= HOLD_CLOSE_FIRST_AFTER);
});

test('F5 proving resets the count', () => {
  const s = memory();
  pageOn(s, 'p1').markPending(win('w1'));
  pageOn(s, 'p2').takeDeaths(['w1'], none);
  const g = pageOn(s, 'p3');
  g.markPending(win('w1'));
  g.settle('w1');
  g.markPending(win('w1'));
  assert.deepEqual(pageOn(s, 'p4').takeDeaths(['w1'], none), { w1: 1 });
});

test('F6 a page that leaves alive takes its markers with it, except work in flight', () => {
  const s = memory();
  const g = pageOn(s, 'p1');
  g.beginBoot(none);
  g.markPending(win('w1'));
  g.markPending(win('w2'));
  g.step('w2', 'fetching the PDF file', { win: win('w2') }); // foreground, no done: in flight
  g.leave();
  const g2 = pageOn(s, 'p2');
  assert.equal(g2.beginBoot(none).interrupted, true, 'work in flight keeps the boot marker too');
  assert.deepEqual(g2.takeDeaths(['w1', 'w2'], none), { w2: 1 }, 'only the slow open stays a death');
});

test('F7 leaving with nothing in flight clears the boot marker; rejoin restores it', () => {
  const s = memory();
  const g = pageOn(s, 'p1');
  g.beginBoot(none);
  g.leave();
  assert.equal(pageOn(s, 'p2').beginBoot(none).interrupted, false);
  const s2 = memory();
  const h = pageOn(s2, 'p1');
  h.beginBoot(none);
  h.leave();
  h.rejoin(false); // shown again before the desktop was stable
  assert.equal(pageOn(s2, 'p2').beginBoot(none).interrupted, true);
  const s3 = memory();
  const k = pageOn(s3, 'p1');
  k.beginBoot(none);
  k.bootStable();
  k.leave();
  k.rejoin(true);
  assert.equal(pageOn(s3, 'p2').beginBoot(none).interrupted, false);
});

test('F35 leave then rejoin: this page marks its windows again, keeping an entry that is still there', () => {
  const s = memory();
  const g = pageOn(s, 'p1');
  g.markPending(win('w1'));
  g.step('w1', 'x', {});
  g.leave(); // busy: stays
  g.markPending(win('w1')); // rejoin re-marks; the existing entry (with its step) is kept
  g.markPending(win('w2'));
  assert.deepEqual(pageOn(s, 'p2').takeDeaths(['w1', 'w2'], none), { w1: 1, w2: 1 });
  assert.equal(pageOn(s, 'p3').lastStep('w1'), 'x');
});

test('F8 boot marker: interrupted until stable', () => {
  const s = memory();
  pageOn(s, 'p1').beginBoot(none);
  const g = pageOn(s, 'p2');
  assert.equal(g.beginBoot(none).interrupted, true);
  g.bootStable();
  assert.equal(pageOn(s, 'p3').beginBoot(none).interrupted, false);
});

test('F9 another page: left alone while alive, a death when not alive', () => {
  const s = memory();
  const other = pageOn(s, 'pb');
  other.beginBoot(none);
  other.markPending(win('w1'));
  const live = pageOn(s, 'pa');
  assert.equal(live.beginBoot(new Set(['pb'])).interrupted, false);
  assert.deepEqual(live.takeDeaths(['w1'], new Set(['pb'])), {});
  // pb still holds its marker; a fresh page that finds pb dead holds the window.
  const fresh = pageOn(s, 'pc');
  assert.equal(fresh.beginBoot(none).interrupted, true);
  assert.deepEqual(fresh.takeDeaths(['w1'], none), { w1: 1 });
});

test('F10 broken storage never throws and reports nothing', () => {
  const boom = { getItem() { throw new Error('x'); }, setItem() { throw new Error('x'); }, removeItem() { throw new Error('x'); } };
  for (const storage of [null, undefined, boom]) {
    const g = pageOn(storage, 'p1');
    assert.deepEqual(g.beginBoot(none), { interrupted: false, previousDead: false });
    g.markPending(win('w1'));
    g.step('w1', 'x', {});
    g.settle('w1');
    g.bootStable();
    g.leave();
    assert.deepEqual(g.takeDeaths(['w1'], none), {});
  }
  const s = memory();
  s.setItem('k', '{not json');
  assert.deepEqual(pageOn(s, 'p1').takeDeaths(['w1'], none), {});
  s.setItem('k', '[1,2]');
  assert.deepEqual(pageOn(s, 'p1').takeDeaths(['w1'], none), {});
});

test('F11 markers never expire by age; the count is capped', () => {
  const s = memory();
  clockNow = 1000;
  pageOn(s, 'p1').markPending(win('w1'));
  clockNow = 1000 + 90 * 24 * 3600 * 1000;
  assert.deepEqual(pageOn(s, 'p2').takeDeaths(['w1'], none), { w1: 1 });
  const s2 = memory();
  clockNow = 1000;
  for (let i = 0; i < PENDING_CAP + 5; i++) {
    clockNow = 1000 + i;
    pageOn(s2, 'p1').markPending(win('c' + i));
  }
  const ids = Array.from({ length: PENDING_CAP + 5 }, (_, i) => 'c' + i);
  assert.equal(Object.keys(pageOn(s2, 'p2').takeDeaths(ids, none)).length, PENDING_CAP);
  clockNow = 1000;
});

test('F12 a marker for a window the server no longer has is pruned', () => {
  const s = memory();
  pageOn(s, 'p1').markPending(win('gone'));
  assert.deepEqual(pageOn(s, 'p2').takeDeaths(['w1'], none), {});
  assert.deepEqual(pageOn(s, 'p3').takeDeaths(['gone'], none), {});
});

test('F13 the quiet period counts visible time only', () => {
  const s = memory();
  const g = pageOn(s, 'p1');
  g.markPending(win('w1'));
  g.markPending(win('w2'));
  const clock = createProofClock({ guard: g });
  assert.deepEqual(clock.advance(['w1', 'w2'], QUIET_MS - 1000, true), []);
  assert.deepEqual(clock.advance(['w1', 'w2'], 5000, false), [], 'hidden page does not count');
  assert.deepEqual(clock.advance(['w1'], 500, true), [], 'w2 unmounted: its clock restarts');
  assert.deepEqual(clock.advance(['w1'], 500, true), ['w1']);
  assert.deepEqual(clock.advance(['w1'], 500, true), [], 'proved once');
  assert.deepEqual(pageOn(s, 'p2').takeDeaths(['w1', 'w2'], none), { w2: 1 });
});

test('F14 interrupted boot is the backstop only when no window is identified', () => {
  const s = memory();
  pageOn(s, 'p1').beginBoot(none);
  const g = pageOn(s, 'p2');
  const boot = g.beginBoot(none);
  const dead = g.takeDeaths(['w1', 'w2'], none);
  assert.equal(boot.interrupted && Object.keys(dead).length === 0, true);
  const s2 = memory();
  const a = pageOn(s2, 'p1');
  a.beginBoot(none);
  a.markPending(win('w1'));
  const b = pageOn(s2, 'p2');
  const boot2 = b.beginBoot(none);
  const dead2 = b.takeDeaths(['w1', 'w2'], none);
  assert.equal(boot2.interrupted && Object.keys(dead2).length === 0, false);
});

test('F15 the last step survives into the held record and the history', () => {
  const s = memory();
  const mk = (p) => pageOn(s, p, { userAgent: 'UA/1' });
  const g = mk('p1');
  g.markPending(win('w1'));
  g.step('w1', 'fetching the file', {});
  g.step('w1', 'importing the PDF library', {});
  g.step('nope', 'ignored', {});
  assert.deepEqual(mk('p2').takeDeaths(['w1'], none), { w1: 1 });
  const g2 = mk('p3');
  assert.equal(g2.lastStep('w1'), 'importing the PDF library');
  const [h] = g2.history();
  assert.equal(h.appId, 'reader');
  assert.equal(h.step, 'importing the PDF library');
  assert.equal(h.userAgent, 'UA/1');
  assert.equal(h.crashes, 1);
  g2.settle('w1');
  assert.equal(g2.lastStep('w1'), '');
  assert.equal(mk('p4').history().length, 1, 'history outlives the window');
  for (let i = 0; i < 12; i++) {
    mk('q' + i).markPending(win('h' + i));
    mk('r' + i).takeDeaths(['h' + i], none);
  }
  assert.equal(mk('z').history().length, 10);
});

test('F16 liveness from locks; without them nobody is alive', async () => {
  const locks = (names) => ({ query: async () => ({ held: names.map((name) => ({ name })) }) });
  const alive = await readLiveTabs({ locks: locks(['choir-desktop-tab-b', 'other-lock']) });
  assert.deepEqual([...alive], ['b']);
  assert.equal((await readLiveTabs({ locks: null })).size, 0);
  assert.equal((await readLiveTabs({ locks: { query: async () => { throw new Error('x'); } } })).size, 0);
  assert.equal((await readLiveTabs({})).size, 0);
});

test('F17 the page that marks first is not dead to itself', () => {
  const s = memory();
  const g = pageOn(s, 'p1');
  g.markPending(win('w1')); // armed and marked before the first restore ran
  assert.deepEqual(g.takeDeaths(['w1'], none), {});
  assert.equal(g.beginBoot(none).interrupted, false);
  assert.deepEqual(pageOn(s, 'p2').takeDeaths(['w1'], none), { w1: 1 }, 'but dead to the next load');
});

test('F18 markers are per window and page; one death counts once', () => {
  const s = memory();
  const a = pageOn(s, 'pa');
  const b = pageOn(s, 'pb');
  a.markPending(win('w1'));
  b.markPending(win('w1'));
  b.settle('w1'); // pb proved it; pa is still opening it
  const next = pageOn(s, 'pc');
  assert.deepEqual(next.takeDeaths(['w1'], new Set(['pb'])), { w1: 1 }, "pa's marker survives pb's proof");
  const s2 = memory();
  pageOn(s2, 'pa').markPending(win('w1'));
  pageOn(s2, 'pb').markPending(win('w1'));
  assert.deepEqual(pageOn(s2, 'pc').takeDeaths(['w1'], none), { w1: 1 }, 'two dead pages, one death');
  const s3 = memory();
  const x = pageOn(s3, 'px');
  x.markPending(win('w1'));
  pageOn(s3, 'py').settle('w1'); // another live page closing it
  assert.deepEqual(pageOn(s3, 'pz').takeDeaths(['w1'], none), { w1: 1 }, "py's close does not delete px's marker");
});

test('F19 proof needs quiet after mount and after the latest foreground step', () => {
  const s = memory();
  const g = pageOn(s, 'p1');
  g.markPending(win('w1'));
  const clock = createProofClock({ guard: g });
  assert.deepEqual(clock.advance(['w1'], 6000, true), []);
  clock.noteStep('w1', {}); // foreground step at 6 s, work in flight
  assert.deepEqual(clock.advance(['w1'], 6000, true), [], '12 s after mount, 6 s after the step: not proven');
  assert.deepEqual(pageOn(s, 'p2').takeDeaths(['w1'], none), { w1: 1 }, 'a death at 12 s is still held');

  const s2 = memory();
  const g2 = pageOn(s2, 'p1');
  g2.markPending(win('w1'));
  const c2 = createProofClock({ guard: g2 });
  c2.noteStep('w1', {});
  assert.deepEqual(c2.advance(['w1'], QUIET_MS + 1000, true), [], 'announced work, never finished: not at the quiet period');
  c2.noteStep('w1', { done: true });
  assert.deepEqual(c2.advance(['w1'], QUIET_MS, true), ['w1'], 'done: quiet period after it');

  const s3 = memory();
  const g3 = pageOn(s3, 'p1');
  g3.markPending(win('w1'));
  const c3 = createProofClock({ guard: g3 });
  c3.noteStep('w1', {});
  assert.deepEqual(c3.advance(['w1'], BUSY_MAX_MS, true), ['w1'], 'a hang is proven at the ceiling');

  const s4 = memory();
  const g4 = pageOn(s4, 'p1');
  g4.markPending(win('w1'));
  const c4 = createProofClock({ guard: g4 });
  c4.advance(['w1'], 7000, true);
  c4.noteStep('w1', { background: true });
  assert.deepEqual(c4.advance(['w1'], 1000, true), ['w1'], 'a background step does not restart proof');

  const s5 = memory();
  const g5 = pageOn(s5, 'p1');
  g5.markPending(win('w1'));
  const c5 = createProofClock({ guard: g5 });
  c5.advance(['w1'], QUIET_MS, true);
  g5.step('w1', 'opening another document', { win: win('w1') });
  c5.noteStep('w1', {});
  assert.deepEqual(pageOn(s5, 'p2').takeDeaths(['w1'], none), { w1: 1 }, 'a foreground step on a proven window arms it again');
});

test('F21 holds are capped by count', () => {
  const s = memory();
  for (let i = 0; i < RECORD_CAP + 10; i++) {
    pageOn(s, 'a' + i).markPending(win('w' + i));
    pageOn(s, 'b' + i).takeDeaths(['w' + i], none);
  }
  const state = JSON.parse(s.getItem('k'));
  assert.ok(Object.keys(state.held).length <= RECORD_CAP);
});

test('F22 previousDead: interrupted boot or any dead pending marker', () => {
  const s = memory();
  assert.equal(pageOn(s, 'p0').beginBoot(none).previousDead, false, 'a clean device');
  // died with a boot marker only
  const s1 = memory();
  pageOn(s1, 'p1').beginBoot(none);
  assert.equal(pageOn(s1, 'p2').beginBoot(none).previousDead, true);
  // boot stable, but a window was still pending (an automatic open, fresh id)
  const s2 = memory();
  const a = pageOn(s2, 'p1');
  a.beginBoot(none);
  a.bootStable();
  a.markPending(win('route-window-17'));
  assert.equal(pageOn(s2, 'p2').beginBoot(none).previousDead, true);
  // the same page, or a page that is alive, is not the previous boot
  const s3 = memory();
  const b = pageOn(s3, 'pb');
  b.beginBoot(none);
  b.markPending(win('w1'));
  assert.equal(pageOn(s3, 'pa').beginBoot(new Set(['pb'])).previousDead, false);
  const s3b = memory();
  const own = pageOn(s3b, 'solo');
  own.markPending(win('w1'));
  assert.equal(own.beginBoot(none).previousDead, false, 'own markers are not dead to itself');
});

const dieHeld = (s) => {
  // page 1 dies pending; page 2 judges it (held, count 1) and stays unmounted
  pageOn(s, 'p1').markPending(win('w1'));
  const p2 = pageOn(s, 'p2');
  p2.beginBoot(none);
  assert.deepEqual(p2.takeDeaths(['w1'], none), { w1: 1 });
  return p2;
};

test('F23 held, then reload after reload: still held', () => {
  const s = memory();
  dieHeld(s);
  for (const p of ['p3', 'p4', 'p5']) {
    const g = pageOn(s, p);
    g.beginBoot(none);
    g.takeDeaths(['w1'], none);
    assert.deepEqual(g.holds(['w1']), { w1: 1 }, p);
  }
  // a second tab, with its own page and no boot history
  assert.deepEqual(pageOn(s, 'tab2').holds(['w1']), { w1: 1 });
});

test('F24 Reopen does not end the hold: die or reload before proof is held, count up', () => {
  const s = memory();
  dieHeld(s);
  pageOn(s, 'p3').markPending(win('w1')); // Reopen mounted it, then the page died
  const p4 = pageOn(s, 'p4');
  assert.deepEqual(p4.takeDeaths(['w1'], none), { w1: 2 });
  assert.deepEqual(p4.holds(['w1']), { w1: 2 });
  pageOn(s, 'p5').markPending(win('w1')); // Reopen then an immediate reload
  const p6 = pageOn(s, 'p6');
  p6.takeDeaths(['w1'], none);
  assert.deepEqual(p6.holds(['w1']), { w1: 3 });
});

test('F25 Reopen then proof ends the hold and resets the count', () => {
  const s = memory();
  dieHeld(s);
  const p3 = pageOn(s, 'p3');
  p3.markPending(win('w1'));
  assert.deepEqual(p3.holds(['w1']), { w1: 1 }, 'Reopen alone leaves it held on disk');
  p3.settle('w1'); // the quiet period passed
  assert.deepEqual(pageOn(s, 'p4').holds(['w1']), {});
  pageOn(s, 'p5').markPending(win('w1'));
  const p6 = pageOn(s, 'p6');
  assert.deepEqual(p6.takeDeaths(['w1'], none), { w1: 1 }, 'count restarted');
});

test('F26 Close ends the hold', () => {
  const s = memory();
  dieHeld(s).settle('w1');
  assert.deepEqual(pageOn(s, 'p3').holds(['w1']), {});
});

test('F27 a hold for a window the server no longer has is pruned', () => {
  const s = memory();
  dieHeld(s);
  assert.deepEqual(pageOn(s, 'p3').holds(['other']), {});
  assert.deepEqual(pageOn(s, 'p4').holds(['w1']), {}, 'gone for good');
});

test('F28 an automatic open is held by key, durably', () => {
  const s = memory();
  const key = 'texture|/pub/texture/a-story';
  const a = pageOn(s, 'p1');
  a.beginBoot(none);
  a.markPending(win('auto-17', 'texture'), { autoKey: key });
  const b = pageOn(s, 'p2');
  const boot = b.beginBoot(none);
  assert.equal(boot.previousDead, true);
  assert.equal(b.autoHeld(key), 1, 'the death of a fresh-id window is matched by key');
  assert.equal(pageOn(s, 'p3').autoHeld(key), 1, 'and survives the next load');
  // holdAuto records a hold when the previous boot died but nothing matched
  const c = pageOn(s, 'p4');
  assert.equal(c.holdAuto('reader|{"x":1}', { appId: 'reader' }), 1);
  assert.equal(pageOn(s, 'p5').autoHeld('reader|{"x":1}'), 1);
  // Close, by key (the window never mounted, so no marker of its own)
  pageOn(s, 'p6').settle('auto-99', { autoKey: key });
  assert.equal(pageOn(s, 'p7').autoHeld(key), 0);
  // Reopen then proof: the pending entry knows its key
  const d = pageOn(s, 'p8');
  d.markPending(win('auto-5', 'reader'), { autoKey: 'reader|{"x":1}' });
  d.settle('auto-5');
  assert.equal(pageOn(s, 'p9').autoHeld('reader|{"x":1}'), 0);
});

test('F29 a hold survives a page leaving alive', () => {
  const s = memory();
  const p2 = dieHeld(s);
  p2.leave();
  const p3 = pageOn(s, 'p3');
  p3.beginBoot(none);
  p3.takeDeaths(['w1'], none);
  assert.deepEqual(p3.holds(['w1']), { w1: 1 });
});

test('F30 a liveness read that never resolves is bounded', async () => {
  const never = new Promise(() => {});
  const fired = [];
  const out = await withTimeout(never, 2000, new Set(), {
    setTimer: (fn, ms) => { fired.push(ms); fn(); return 1; },
    clearTimer: () => {},
  });
  assert.deepEqual([...out], []);
  assert.deepEqual(fired, [2000]);
  const rejected = await withTimeout(Promise.reject(new Error('x')), 10, 'fb');
  assert.equal(rejected, 'fb');
  assert.equal(await withTimeout(Promise.resolve('ok'), 1000, 'fb'), 'ok');
});

test('F32 the body may mount iff the derived hold is 0', () => {
  const held = { 'w:w1': 2, 'a:texture|[]': 1 };
  assert.equal(crashHoldFor({ windowId: 'w1' }, held, new Set()), 2);
  assert.equal(crashHoldFor({ windowId: 'w2' }, held, new Set()), 0);
  assert.equal(crashHoldFor({ windowId: 'fresh', crashAutoKey: 'texture|[]' }, held, new Set()), 1);
  assert.equal(crashHoldFor({ windowId: 'fresh', crashAutoKey: 'other' }, held, new Set()), 0);
  assert.equal(crashHoldFor({ windowId: 'w1' }, held, new Set(['w1'])), 0, 'Reopened in this page');
  assert.equal(crashHoldFor({ windowId: 'fresh', crashAutoKey: 'texture|[]' }, held, new Set(['fresh'])), 0);
  assert.equal(crashHoldFor(null, held, new Set()), 0);
  assert.equal(crashHoldFor({ windowId: 'w1' }), 0, 'no held map: mount');
});

test('F33 suspension keeps the hold; readHeld mirrors the map', () => {
  const s = memory();
  dieHeld(s);
  const p3 = pageOn(s, 'p3');
  p3.markPending(win('w1')); // Reopened
  p3.unmark('w1'); // then suspended: the body is gone, the hold is not
  assert.deepEqual(p3.readHeld(), { 'w:w1': 1 });
  assert.deepEqual(pageOn(s, 'p4').holds(['w1']), { w1: 1 });
  p3.settle('w1');
  assert.deepEqual(pageOn(s, 'p5').readHeld(), {});
});

test('F34 a failed fetch holds by id and prunes nothing; a real list prunes', () => {
  const s = memory();
  pageOn(s, 'p1').markPending(win('w1'));
  pageOn(s, 'p1').markPending(win('w2'));
  const p2 = pageOn(s, 'p2');
  p2.takeDeaths(null, none); // the state fetch failed: server list unknown
  assert.deepEqual(p2.readHeld(), { 'w:w1': 1, 'w:w2': 1 });
  assert.deepEqual(p2.holds([]), {}, 'a successful but empty list prunes (the server has none)');
  assert.deepEqual(p2.readHeld(), {});
});

test('F36 a dead automatic open holds its key and the window id it created', () => {
  const s = memory();
  const key = 'texture|[]';
  const a = pageOn(s, 'p1');
  a.markPending(win('auto-1', 'texture'), { autoKey: key });
  const b = pageOn(s, 'p2');
  b.beginBoot(none);
  assert.deepEqual(b.readHeld(), { [`a:${key}`]: 1, 'w:auto-1': 1 });
  assert.deepEqual(b.holds(['auto-1']), { 'auto-1': 1 }, 'the server restored that window: held by id');
  assert.deepEqual(pageOn(s, 'p3').holds(['other']), {}, 'not restored: the id is pruned');
  assert.equal(pageOn(s, 'p4').autoHeld(key), 1, 'the key stays');
  // holdAuto records the id as well
  const c = pageOn(s, 'p5');
  assert.equal(c.holdAuto('reader|[]', { windowId: 'r1' }), 1);
  assert.deepEqual(c.readHeld()['w:r1'], 1);
});
