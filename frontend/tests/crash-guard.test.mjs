// node --test tests/crash-guard.test.mjs
//
// FAILURE LIST (written before the code; owner, 2026-11-16: "No crash loops").
// The page cannot see its own death, so a marker is written before an app body
// mounts and cleared only when the window has proven itself.
//
//  F1  A window pending when the page died is reported dead at the next load,
//      with a count of 1, and is no longer pending (it is held, not mounted).
//  F2  A window proven before the page died is not reported dead.
//  F3  Closing a window (settle) clears its pending marker and its death count.
//  F4  A second death of the same window reports a count of 2 (reopen marks it
//      pending again; the count survives the hold).
//  F5  Proving a window resets its death count.
//  F6  pagehide with a window still pending leaves the pending marker AND the
//      boot marker (a user reloading a frozen page is the crash case).
//  F7  pagehide with nothing pending clears the boot marker (a clean leave
//      before the desktop was stable is not a crash).
//  F8  A boot marker left by the previous load is reported as an interrupted
//      boot; once the desktop was stable it is not.
//  F9  Another tab's markers are neither reported nor removed.
//  F10 Storage that is missing, throws, or holds garbage never throws, and
//      reports nothing dead: the desktop behaves as today.
//  F11 Markers older than a day are dropped, not held forever.
//  F12 A pending marker for a window the server no longer has is pruned.
//  F13 The quiet period counts only visible time, restarts when a window is
//      unmounted, proves each window once, and a late death (after mount but
//      before the quiet period ends) is still reported at the next load.
//  F15 The step an app reported last survives into the held record, the
//      last-step lookup and a per-device history capped at 10 (appId, step,
//      user agent, time, count). A step for an unknown window is ignored.
//  F14 hold-all is the page-level answer: interrupted boot with no window
//      identified; with a window identified only that window is held.
import test from 'node:test';
import assert from 'node:assert/strict';
import {
  createCrashGuard,
  createProofClock,
  QUIET_MS,
  HOLD_CLOSE_FIRST_AFTER,
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
const guardOn = (storage, extra = {}) =>
  createCrashGuard({ storage, key: 'k', tabId: 'tab-a', now: () => 1000, ...extra });

test('F1 a pending window is dead at the next load', () => {
  const s = memory();
  const g1 = guardOn(s);
  g1.beginBoot();
  g1.markPending(win('w1'));
  const g2 = guardOn(s);
  const boot = g2.beginBoot();
  const dead = g2.takeDeaths(['w1', 'w2']);
  assert.deepEqual(dead, { w1: 1 });
  assert.equal(boot.interrupted, true);
  assert.deepEqual(guardOn(s).takeDeaths(['w1']), {}, 'no longer pending once held');
});

test('F2 a proven window is not dead', () => {
  const s = memory();
  const g1 = guardOn(s);
  g1.markPending(win('w1'));
  g1.settle('w1', { proven: true });
  assert.deepEqual(guardOn(s).takeDeaths(['w1']), {});
});

test('F3 closing clears pending and the count', () => {
  const s = memory();
  const g = guardOn(s);
  g.markPending(win('w1'));
  guardOn(s).takeDeaths(['w1']);
  g.markPending(win('w1'));
  g.settle('w1');
  assert.deepEqual(guardOn(s).takeDeaths(['w1']), {});
  g.markPending(win('w1'));
  assert.deepEqual(guardOn(s).takeDeaths(['w1']), { w1: 1 });
});

test('F4 a second death counts two', () => {
  const s = memory();
  guardOn(s).markPending(win('w1'));
  assert.deepEqual(guardOn(s).takeDeaths(['w1']), { w1: 1 });
  guardOn(s).markPending(win('w1')); // reopened
  assert.deepEqual(guardOn(s).takeDeaths(['w1']), { w1: 2 });
  assert.ok(2 >= HOLD_CLOSE_FIRST_AFTER);
});

test('F5 proving resets the count', () => {
  const s = memory();
  guardOn(s).markPending(win('w1'));
  guardOn(s).takeDeaths(['w1']);
  const g = guardOn(s);
  g.markPending(win('w1'));
  g.settle('w1', { proven: true });
  g.markPending(win('w1'));
  assert.deepEqual(guardOn(s).takeDeaths(['w1']), { w1: 1 });
});

test('F6 pagehide with a pending window keeps both markers', () => {
  const s = memory();
  const g = guardOn(s);
  g.beginBoot();
  g.markPending(win('w1'));
  g.leaveCleanly();
  const g2 = guardOn(s);
  assert.equal(g2.beginBoot().interrupted, true);
  assert.deepEqual(g2.takeDeaths(['w1']), { w1: 1 });
});

test('F7 pagehide with nothing pending clears the boot marker', () => {
  const s = memory();
  const g = guardOn(s);
  g.beginBoot();
  g.leaveCleanly();
  assert.equal(guardOn(s).beginBoot().interrupted, false);
});

test('F8 boot marker: interrupted until stable', () => {
  const s = memory();
  guardOn(s).beginBoot();
  const g = guardOn(s);
  assert.equal(g.beginBoot().interrupted, true);
  g.bootStable();
  assert.equal(guardOn(s).beginBoot().interrupted, false);
});

test('F9 another tab is neither reported nor removed', () => {
  const s = memory();
  const other = createCrashGuard({ storage: s, key: 'k', tabId: 'tab-b', now: () => 1000 });
  other.beginBoot();
  other.markPending(win('w1'));
  const mine = guardOn(s);
  assert.equal(mine.beginBoot().interrupted, false);
  assert.deepEqual(mine.takeDeaths(['w1']), {});
  assert.deepEqual(
    createCrashGuard({ storage: s, key: 'k', tabId: 'tab-b', now: () => 1000 }).takeDeaths(['w1']),
    { w1: 1 },
  );
});

test('F10 broken storage never throws and reports nothing', () => {
  const boom = { getItem() { throw new Error('x'); }, setItem() { throw new Error('x'); }, removeItem() { throw new Error('x'); } };
  for (const storage of [null, undefined, boom]) {
    const g = guardOn(storage);
    assert.deepEqual(g.beginBoot(), { interrupted: false });
    g.markPending(win('w1'));
    g.settle('w1');
    g.bootStable();
    g.leaveCleanly();
    assert.deepEqual(g.takeDeaths(['w1']), {});
  }
  const s = memory();
  s.setItem('k', '{not json');
  assert.deepEqual(guardOn(s).takeDeaths(['w1']), {});
  s.setItem('k', '[1,2]');
  assert.deepEqual(guardOn(s).takeDeaths(['w1']), {});
});

test('F11 old markers are dropped', () => {
  const s = memory();
  createCrashGuard({ storage: s, key: 'k', tabId: 'tab-a', now: () => 1000 }).markPending(win('w1'));
  const later = createCrashGuard({ storage: s, key: 'k', tabId: 'tab-a', now: () => 1000 + 25 * 3600 * 1000 });
  assert.deepEqual(later.takeDeaths(['w1']), {});
});

test('F12 a marker for a window the server no longer has is pruned', () => {
  const s = memory();
  guardOn(s).markPending(win('gone'));
  assert.deepEqual(guardOn(s).takeDeaths(['w1']), {});
  assert.deepEqual(guardOn(s).takeDeaths(['gone']), {});
});

test('F13 the quiet period counts visible time only', () => {
  const s = memory();
  const g = guardOn(s);
  g.markPending(win('w1'));
  g.markPending(win('w2'));
  const clock = createProofClock({ guard: g });
  assert.deepEqual(clock.advance(['w1', 'w2'], QUIET_MS - 1000, true), []);
  assert.deepEqual(clock.advance(['w1', 'w2'], 5000, false), [], 'hidden page does not count');
  assert.deepEqual(clock.advance(['w1'], 500, true), [], 'w2 unmounted: its clock restarts');
  assert.deepEqual(clock.advance(['w1'], 500, true), ['w1']);
  assert.deepEqual(clock.advance(['w1'], 500, true), [], 'proved once');
  // w2 never proved: a death now is still reported.
  assert.deepEqual(guardOn(s).takeDeaths(['w1', 'w2']), { w2: 1 });
});

test('F14 interrupted boot is the backstop only when no window is identified', () => {
  const s = memory();
  guardOn(s).beginBoot();
  const g = guardOn(s);
  const boot = g.beginBoot();
  const dead = g.takeDeaths(['w1', 'w2']);
  assert.equal(boot.interrupted && Object.keys(dead).length === 0, true);
  const s2 = memory();
  const a = guardOn(s2);
  a.beginBoot();
  a.markPending(win('w1'));
  const b = guardOn(s2);
  const boot2 = b.beginBoot();
  const dead2 = b.takeDeaths(['w1', 'w2']);
  assert.equal(boot2.interrupted && Object.keys(dead2).length === 0, false);
});

test('F15 the last step survives into the held record and the history', () => {
  const s = memory();
  const mk = () => guardOn(s, { userAgent: 'UA/1' });
  const g = mk();
  g.markPending(win('w1'));
  g.step('w1', 'fetching the file');
  g.step('w1', 'importing the PDF library');
  g.step('nope', 'ignored');
  assert.deepEqual(mk().takeDeaths(['w1']), { w1: 1 });
  const g2 = mk();
  assert.equal(g2.lastStep('w1'), 'importing the PDF library');
  const [h] = g2.history();
  assert.equal(h.appId, 'reader');
  assert.equal(h.step, 'importing the PDF library');
  assert.equal(h.userAgent, 'UA/1');
  assert.equal(h.crashes, 1);
  assert.equal(h.at, 1000);
  g2.settle('w1');
  assert.equal(g2.lastStep('w1'), '');
  assert.equal(mk().history().length, 1, 'history outlives the window');
  for (let i = 0; i < 12; i++) {
    const x = mk();
    x.markPending(win('h' + i));
    mk().takeDeaths(['h' + i]);
  }
  assert.equal(mk().history().length, 10);
});
