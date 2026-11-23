// The rule these tests hold (docs/reference/exact/answers-let-go.md): a fetch or a native call made after an `await` belongs to the answer that began it, and an answer is LET GO when the next ask of the
// same resource comes (Exact js/src/fetch.js: "the answer was let go before this fetch"; host/web-js/ts-fetch.js `letGo`). The radio pulse is asked every second, so it only READS and says what work is due;
// the work (pump, starts, the desk's ask, posts) is in mutations that stay until done (`radioHold`, `radioWrite`). A pulse that fetched after an await lost the desk's ask, the deck's held list and the radio's start.
import { describe, expect, test } from 'bun:test';
import { readdirSync, readFileSync } from 'node:fs';

const src = (f: string): string => readFileSync(new URL(f, import.meta.url), 'utf8');
const bodyOf = (text: string, head: string): string => {
  const i = text.indexOf(head); if (i < 0) throw new Error('no ' + head);
  const open = text.indexOf('{\n', i), close = text.indexOf('\n}\n', open);
  return text.slice(open, close);
};

describe('the radio pulse reads and says; it does not work', () => {
  const pulse = bodyOf(src('radio-bridge.ts'), 'export async function radioPulse(');
  test('its body makes no fetch and does no pump, start, post or ask', () => {
    for (const bad of ['fetchFront', 'openRadioOf', '.pump(', 'handle(', 'getJSON', 'authed', 'fetch(', 'flush(', 'checkAhead(', 'startEntry', 'keepHeld'])
      expect(pulse.includes(bad), `radioPulse calls ${bad}`).toBe(false);
  });
  test('the only native calls after its awaits are the deck read and the one radioSet push', () => {
    const calls = [...pulse.matchAll(/await ([A-Za-z_.]+)\(/g)].map((m) => m[1]);
    expect(calls.sort()).toEqual(['getRadio', 'firstRunOver', 'later', 'r.rt.peek'].sort());
  });
  test('the work is in mutations the contract sends and the pulse only names', () => {
    const contract = src('members-radio.contract');
    expect(contract).toContain('send rwm = radioWrite(');
    expect(contract).toContain('send rhm = radioHold(');
    expect(contract).toContain('task writeKick when rpulse.write');
    expect(contract).toContain('task holdKick when rpulse.hold');
  });
});

/** A `void` in front of a call starts work nothing waits for: what it fetches after an await is never run once its answer is gone. Each one that is allowed says why. */
const ALLOWED: Record<string, string> = {
  'account.ts::void p.then((ok) => { if (timedOut && ok) onLate?.(); });': 'a late native answer after the deadline: bookkeeping that the next launch repeats (reconcile)',
  'account.ts::void (async () => { if ((await keyState()).state': 'the late continuation of a sign-out (the marker stays, the next launch finishes it)',
  'account.ts::void (async () => { const n = await keyState();': 'the late continuation of a wipe (the marker stays, the next launch finishes it)',
  'notebook-run.ts::void (async () => {': 'one native push whose first call is made inside the answer; a miss is pushed again by the next read',
  'radio-runtime.ts::if (how === \'restart\') { const it = this.known.get(air.id); if (it) void this.deck.play(it); }': 'one native call started inside the pump (a mutation); the journal says if it did not play',
  'radio-runtime.ts::if (!this.inPump) void this.pump();': 'joined by the caller\'s own `await rt.pump()` (the chain), so the answer waits for it',
  'reader.ts::void Promise.resolve().then(() => send(': 'one native call in the same turn as its answer',
  'runsheet-store.ts::void this.save();': 'a storage write (storage is not a fetch)',
};
describe('nothing is started and forgotten', () => {
  test('every `void` call in the shared code is one of the reviewed ones', () => {
    const seen: string[] = [];
    for (const f of readdirSync(new URL('.', import.meta.url)).filter((n) => n.endsWith('.ts') && !n.endsWith('.test.ts') && !n.endsWith('.d.ts'))) {
      for (const line of src(f).split('\n')) {
        const t = line.trim();
        if (t.startsWith('//') || t.startsWith('*')) continue;
        const at = t.search(/\bvoid\s+(\(async|[A-Za-z_.]+\(|Promise)/);
        if (at < 0) continue;
        seen.push(`${f}::${t.slice(0, 200)}`);
      }
    }
    const unknown = seen.filter((s) => !Object.keys(ALLOWED).some((k) => { const [f, snip] = k.split('::'); return s.startsWith(f + '::') && s.includes(snip); }));
    expect(unknown, 'a new `void` call: await it in an answer, or move it to a mutation, or review it into ALLOWED with the reason').toEqual([]);
  });
});


describe('an ask of the desk starts only from a mutation\'s own body (the radio ran dry when a chained ask hung)', () => {
  test('the writer\'s `ask` body starts no other ask', () => {
    const w = src('radio-writer.ts');
    const body = w.slice(w.indexOf('private ask('), w.indexOf('private async post('));
    expect(body.length).toBeGreaterThan(500);
    expect(body.includes('this.checkAhead('), 'ask() chains checkAhead() from its own tail').toBe(false);
    expect(body.includes('this.ask('), 'ask() calls itself').toBe(false);
  });
  test('checkAhead is called from radioWrite and nowhere else in the bridge', () => {
    const b = src('radio-bridge.ts');
    const at = [...b.matchAll(/checkAhead\(/g)].map((m) => m.index as number);
    expect(at.length).toBe(1);
    const write = b.slice(b.indexOf('export async function radioWrite('));
    expect(write.indexOf('checkAhead(')).toBeGreaterThan(-1);
    expect(write.indexOf('checkAhead(')).toBeLessThan(write.indexOf('\n}\n'));
  });
  test('the pulse\'s log line says what is in flight and how many are ahead, so a dry deck shows in the log', () => {
    const pulse = bodyOf(src('radio-bridge.ts'), 'export async function radioPulse(');
    expect(pulse).toContain('inFlight=${r.writer.inFlight}'); expect(pulse).toContain('ahead=${');
  });
});
