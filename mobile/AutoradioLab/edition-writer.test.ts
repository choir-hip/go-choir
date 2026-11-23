// The edition writer under bun test: the Swift EditionSetupTests C1 to C4 and the general edition.
import { describe, expect, test } from 'bun:test';
import { localEdition, editionLines, complete, localWriter, generalEdition, FAILED_LINE, type Writer } from './edition-writer.ts';

describe('finishing always ends (C1 to C4)', () => {
  test('C1: success', async () => {
    const o = await complete(['Space'], localWriter);
    expect(o.kind).toBe('written');
    expect(o.kind === 'written' && editionLines(o.edition)).toEqual(['What I follow', 'Space']);
  });
  test('C2: failure says so plainly', async () => {
    const o = await complete(['Space'], async () => { throw new Error('no'); });
    expect(o).toEqual({ kind: 'failed', line: FAILED_LINE }); expect(FAILED_LINE).toContain('topics are saved');
  });
  test('C3: a writer that never returns times out at the deadline and is aborted', async () => {
    let aborted = false;
    const never: Writer = (_t, _p, signal) => new Promise((_res, rej) => { signal.addEventListener('abort', () => { aborted = true; rej(new Error('aborted')); }); });
    const t0 = Date.now();
    const o = await complete([], never, undefined, 0.2);
    expect(o).toEqual({ kind: 'timedOut' }); expect(Date.now() - t0).toBeLessThan(5000); expect(aborted).toBe(true);
  });
  test('"Continue anyway" is the general edition', () => expect(editionLines(generalEdition())).toEqual(['What I follow', 'The whole paper, until I say otherwise.']));
  test('the edition is written without a network from the choices', async () => {
    const e = await localWriter(['Space', 'Chess'], () => undefined, new AbortController().signal);
    expect(e.title).toBe('My edition'); expect(editionLines(e)).toEqual(['What I follow', 'Space', 'Chess']);
  });
});

describe('the local edition', () => { test('no topics is the whole paper', () => expect(localEdition([]).sections[0].lines).toEqual(['The whole paper, until I say otherwise.'])); });
