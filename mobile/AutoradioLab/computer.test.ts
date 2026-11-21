// The Computer page under bun test: the Swift AccountsTests (C1 to C4) and the plain-words and signed-out lines, on a fake of the routes.
import { describe, expect, test } from 'bun:test';
import { Computer, POLLS, plain, snapshot, settingsComputer, computerAct, forgetComputer, NoComputer, type Call } from './computer.ts';
import { HTTPError, SignedOutError } from './api.ts';

type Shot = { state?: string; target?: string; epoch?: number };
function fake(shots: Shot[], o: { update?: unknown; recovery?: unknown } = {}): { call: Call; log: string[] } {
  const log: string[] = [];
  let i = 0;
  const call: Call = async (method, path, body) => {
    log.push(`${method} ${path}${body ? ' ' + JSON.stringify(body) : ''}`);
    if (path === '/api/compute/status') { const s = shots[Math.min(i, shots.length - 1)]; i += 1; return { computers: [{ computer_id: 'c', kind: 'interactive', current: true, state: s.state ?? 'running' }], _s: s }; }
    const s = shots[Math.min(Math.max(i - 1, 0), shots.length - 1)];
    if (path.endsWith('/update') && method === 'GET') return s.target ? { update_available: { target: s.target } } : {};
    if (path.endsWith('/lifecycle/status')) return { state: s.state ?? 'running', realization_epoch: s.epoch ?? 0 };
    if (path.endsWith('/lifecycle/update')) { if (o.update instanceof Error) throw o.update; return o.update ?? {}; }
    if (path === '/api/compute/recovery') return o.recovery ?? {};
    throw new Error('unexpected ' + path);
  };
  return { call, log };
}

describe('computer', () => {
  test('C3 a restart ends when the signal clears, and only then', async () => {
    const f = fake([{ target: 't2', epoch: 1 }, { target: 't2', state: 'restarting', epoch: 1 }, { target: '', epoch: 2 }]);
    const c = new Computer(f.call);
    await c.refresh(); expect(c.v.updateAvailable).toBe(true);
    await c.restartToUpdate(); expect(c.v.phase).toBe('updating');
    await c.poll(); expect(c.v.phase).toBe('updating');                       // still restarting: not yet "current"
    await c.poll();
    expect(c.v.phase).toBe('current'); expect(c.v.updateAvailable).toBe(false); expect(c.v.epoch).toBe(2);
  });
  test('C3 a gateway timeout or a dropped connection is not a failure', async () => {
    for (const e of [new HTTPError(504, 'x'), new HTTPError(502, 'x'), new Error('network')]) {
      const c = new Computer(fake([{ target: 't2' }], { update: e }).call);
      await c.refresh(); await c.restartToUpdate();
      expect(c.v.phase).toBe('updating');
    }
  });
  test('C3 the poll gives up after its count and says so, never "updated"', async () => {
    const c = new Computer(fake([{ target: 't2' }]).call);
    await c.refresh(); await c.restartToUpdate();
    for (let i = 0; i < POLLS; i++) await c.poll();
    expect(c.v.phase).toBe('idle'); expect(c.v.message).toContain('Still restarting'); expect(c.v.updateAvailable).toBe(true);
    const before = c.view(); await c.poll(); expect(c.view()).toEqual(before);
  });
  test('C2 a busy computer is told it will update when idle, never called updated', async () => {
    const c = new Computer(fake([{ target: 't2' }], { update: new HTTPError(409, 'capsule_live') }).call);
    await c.refresh(); await c.restartToUpdate();
    expect(c.v.phase).toBe('paused'); expect(c.v.message).toContain('when it is idle'); expect(c.v.message).toContain('capsule_live'); expect(c.v.updateAvailable).toBe(true);
    await c.poll(); expect(c.v.phase).toBe('paused');                           // nothing retries by itself
  });
  test('already current, and wake', async () => {
    const f = fake([{}], { update: { outcome: 'current' } });
    const c = new Computer(f.call);
    await c.restartToUpdate(); expect(c.v.phase).toBe('current'); expect(c.v.message).toBe('Already up to date.');
    await c.wake();
    expect(f.log.filter((l) => l.startsWith('POST /api/compute/recovery'))).toEqual(['POST /api/compute/recovery {"action":"wake_current_computer"}']);
    expect(c.v.phase).toBe('idle'); expect(c.v.message).toBe('Your computer was asked to wake.');
  });
  test('C1 a second tap while one is in flight is ignored', async () => {
    const f = fake([{ target: 't2' }]);
    const c = new Computer(f.call);
    await c.refresh(); await c.restartToUpdate();
    const posts = f.log.filter((l) => l.includes('lifecycle/update')).length;
    await c.restartToUpdate(); await c.wake();
    expect(f.log.filter((l) => l.includes('lifecycle/update')).length).toBe(posts); expect(c.v.phase).toBe('updating');
    expect(f.log.some((l) => l.includes('recovery'))).toBe(false);
  });
  test('the update request names its computer and carries an idempotency key that changes per tap', async () => {
    const f = fake([{ target: 't2', epoch: 3 }], { update: new HTTPError(409, '') });
    const c = new Computer(f.call);
    await c.refresh(); await c.restartToUpdate(); c.v.phase = 'idle'; await c.restartToUpdate();
    const keys = f.log.filter((l) => l.includes('lifecycle/update')).map((l) => l.split(' ').slice(2).join(' '));
    expect(keys.length).toBe(2); expect(keys[0]).not.toBe(keys[1]); expect(keys[0]).toContain('"idempotency_key"');
  });
  test('C4 no computer yet, and C5 the plain words', async () => {
    await expect(snapshot(async () => ({ computers: [] }))).rejects.toBeInstanceOf(NoComputer);
    const c = new Computer(async () => ({ computers: [{ computer_id: '', kind: 'interactive' }] }));
    await c.refresh();
    expect(c.v.phase).toBe('failed'); expect(c.v.message).toContain('try again in a moment'); expect(c.v.checked).toBe(true);
    expect(plain(new SignedOutError())).toBe('Not signed in.');
    expect(plain(new HTTPError(401, ''))).toContain('no longer accepted');
    expect(plain(new HTTPError(429, ''))).toContain('busy');
    expect(plain(new Error('x'))).toBe('Could not reach your computer.');
  });
  test('a computer of another kind is not the interactive one; the current one wins', async () => {
    const s = await snapshot(async (_m, p) => (p === '/api/compute/status' ? { computers: [{ computer_id: 'w', kind: 'worker' }, { computer_id: 'a', kind: 'interactive' }, { computer_id: 'b', current: true }] } : {}));
    expect(s.id).toBe('b');
  });
  test('a failed refresh clears at the next good one', async () => {
    let bad = true;
    const f = fake([{}]);
    const c = new Computer(async (m, p, b) => { if (bad) throw new Error('x'); return f.call(m, p, b); });
    await c.refresh(); expect(c.v.phase).toBe('failed');
    bad = false; await c.refresh(); expect(c.v.phase).toBe('idle'); expect(c.v.message).toBe('');
  });
  test('C7 signed out sends nothing; the source asks once when signed in', async () => {
    const f = fake([{ target: 't9' }]);
    forgetComputer(new Computer(f.call));
    expect((await settingsComputer({ signedIn: false, tick: 0 })).checked).toBe(false); expect(f.log.length).toBe(0);
    expect((await computerAct('wake', false)).phase).toBe('idle'); expect(f.log.length).toBe(0);
    const v = await settingsComputer({ signedIn: true, tick: 0 });
    expect(v).toMatchObject({ id: 'c', target: 't9', updateAvailable: true, checked: true });
    const n = f.log.length; await settingsComputer({ signedIn: true, tick: 1 }); expect(f.log.length).toBe(n);
    forgetComputer(undefined);
  });
});
