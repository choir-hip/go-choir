// The run sheet reducer under bun test, held to the same fixtures as the Go reducer (mobile/fixtures/runsheet/*.json).
// Failure list (ported from RunSheetTests.swift, one for one):
//  every shared fixture agrees (sheet and results) · move after on air dropped (01) · rewrite of heard dropped (02) · aired from the desk refused (03)
//  aired backwards refused (04) · place with a vanished after lands next (05) · queued lands after the listener's last placement (06)
//  now cuts the entry on air (07) · duplicate op_id is a no-op (08) · clear at a stale version dropped (09) · one version per applied op (10)
//  unknown op and kind are decode errors · unknown entry fields ignored · applied_ops bounded to 512, input not changed · validId.
import { describe, expect, test } from 'bun:test';
import { readdirSync, readFileSync } from 'node:fs';
import { applyAll, apply, decodeEntry, decodeOp, decodeSheet, validId, entryById, Bounds, DecodeError, emptySheet, type Op } from './runsheet-model.ts';

const dir = new URL('../fixtures/runsheet/', import.meta.url).pathname;
const files = readdirSync(dir).filter((f) => f.endsWith('.json')).sort();

function load(name: string) {
  const root = JSON.parse(readFileSync(dir + name, 'utf8'));
  let ops: Op[] = [], failed = false;
  try { ops = (root.ops as unknown[]).map(decodeOp); } catch (e) { if (!(e instanceof DecodeError)) throw e; failed = true; }
  return { root, ops, failed, start: decodeSheet(root.start), want: decodeSheet(root.want), results: root.results as unknown[] };
}
const outcome = (prefix: string) => {
  const f = files.find((x) => x.startsWith(prefix)) as string;
  const fx = load(f);
  return { fx, ...applyAll(fx.start, fx.ops) };
};

describe('shared fixtures', () => {
  test('there are at least twenty', () => expect(files.length).toBeGreaterThanOrEqual(20));
  for (const f of files) {
    test(f, () => {
      const fx = load(f);
      if (fx.root.decode_error) { expect(fx.failed).toBe(true); return; }
      expect(fx.failed).toBe(false);
      const { sheet, results } = applyAll(fx.start, fx.ops);
      expect(sheet).toEqual(fx.want);
      expect(results).toEqual(fx.results as never);
    });
  }
});

describe('the failure list', () => {
  test('move after it went on air is dropped', () => { const { sheet, results } = outcome('01_'); expect(results.map((r) => r.dropped)).toEqual([undefined, 'not_ahead', 'not_ahead']); expect(sheet.on_air).toBe('b'); });
  test('rewrite of heard is dropped', () => expect(outcome('02_').results[0].dropped).toBe('not_ahead'));
  test('aired from the desk is refused', () => { const { sheet, results } = outcome('03_'); expect(results.map((r) => r.dropped)).toEqual(['not_phone', 'not_phone']); expect(sheet.version).toBe(2); });
  test('aired backwards is refused', () => expect(outcome('04_').results.slice(0, 3).map((r) => r.dropped)).toEqual(['bad_transition', 'bad_transition', 'bad_transition']));
  test('place with a vanished after lands next', () => expect(outcome('05_').sheet.entries.map((e) => e.id)).toEqual(['h', 'a', 'z', 'x', 'b', 'y', 'c']));
  test('queued lands after the listener\'s last placement', () => expect(outcome('06_').sheet.entries.map((e) => e.id)).toEqual(['a', 'd1', 'l1', 'q', 'd2']));
  test('now cuts the entry on air', () => { const { sheet } = outcome('07_'); expect(entryById(sheet, 'a')?.state).toBe('skipped'); expect(entryById(sheet, 'a')?.ended?.outcome).toBe('cut'); expect(sheet.on_air).toBe('x'); });
  test('a duplicate op_id is a no-op', () => { const { sheet, results } = outcome('08_'); expect(results.map((r) => r.dropped)).toEqual([undefined, 'duplicate', 'duplicate']); expect(sheet.entries.map((e) => e.id)).toEqual(['a', 'x']); });
  test('clear at a stale version is dropped', () => expect(outcome('09_').results[0].dropped).toBe('stale_version'));
  test('the version increments once per applied op', () => { const { fx, sheet, results } = outcome('10_'); expect(sheet.version).toBe(fx.start.version + results.filter((r) => r.applied).length); });
  test('an unknown op and an unknown kind are decode errors', () => {
    expect(() => decodeOp({ op_id: 'o1', by: 'phone', op: 'teleport', base_version: 0 })).toThrow(DecodeError);
    expect(() => decodeEntry({ id: 'x', kind: 'jingle', placed_by: 'desk', state: 'ahead', title: '', est_seconds: 1 })).toThrow(DecodeError);
  });
  test('unknown entry fields are ignored', () => expect(decodeEntry({ id: 'x', kind: 'page', placed_by: 'listener', state: 'ahead', title: 't', est_seconds: 1, wibble: { a: [1] } }).id).toBe('x'));
  test('applied_ops are bounded and the input is not changed', () => {
    let s = emptySheet();
    const before = structuredClone(s);
    apply(s, { op_id: 'z', by: 'listener', op: 'clear', base_version: 0 });
    expect(s).toEqual(before);
    const N = Bounds.maxAppliedOps;
    for (let i = 0; i < N + 10; i++) { const r = apply(s, { op_id: `o${i}`, by: 'listener', op: 'clear', base_version: s.version }); expect(r.result.applied).toBe(true); s = r.sheet; }
    expect(s.applied_ops.length).toBe(N);
    expect(s.version).toBe(N + 10);
    expect(apply(s, { op_id: `o${N + 9}`, by: 'listener', op: 'clear', base_version: s.version }).result.dropped).toBe('duplicate');
  });
  test('validId', () => {
    for (const ok of ['a', 'p-1a2b3c4d', 'd-turn_7.2-3', 'a'.repeat(64)]) expect(validId(ok)).toBe(true);
    for (const bad of ['', '-a', '.a', 'a b', 'a/b', 'a'.repeat(65), 'é']) expect(validId(bad)).toBe(false);
  });
});
