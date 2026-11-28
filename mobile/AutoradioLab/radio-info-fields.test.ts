// The native `RadioInfo` (interface/bar.json) carries only what shared code reads. A field nothing reads is native work for no one, so adding one back fails here.
// Each field names where it is read: all three by the contracts (`ri.<field>`). The title and whether it plays are shared code's (radio-follow.ts `runOf`), so they are not here.
// The checks run on text with comments removed, and match whole words, so a comment or a longer name (`uri.title`, `ri.titleText`) does not count as a reader.
import { describe, expect, test } from 'bun:test';
import { readFileSync, readdirSync } from 'node:fs';

const here = new URL('.', import.meta.url).pathname;
const stripped = (t: string) => t.replace(/\/\*[\s\S]*?\*\//g, '').replace(/^\s*\/\/.*$/gm, '').replace(/([^:\w"'`])\/\/.*$/gm, '$1');
const read = (path: string) => stripped(readFileSync(path, 'utf8'));
const contracts = readdirSync(here).filter((f) => f.endsWith('.contract')).map((f) => read(`${here}${f}`)).join('\n');

export const readers: Record<string, (f: string) => boolean> = {
  level: (f) => new RegExp(`\\bri\\.${f}\\b`).test(contracts),
  speedValue: (f) => new RegExp(`\\bri\\.${f}\\b`).test(contracts), micLevel: (f) => new RegExp(`\\bri\\.${f}\\b`).test(contracts),
};

/** The failures for a shape: a field with no named reader, or whose reader is gone. */
export function problems(shape: Record<string, string>): string[] {
  const out: string[] = [];
  for (const f of Object.keys(shape)) {
    if (!readers[f]) out.push(`RadioInfo.${f} has no reader: delete it from interface/bar.json and BarState.swift`);
    else if (!readers[f](f)) out.push(`RadioInfo.${f}: its reader is gone (a contract's ri.${f})`);
  }
  return out;
}

const iface = JSON.parse(readFileSync(`${here}../exact-package/interface/bar.json`, 'utf8'));

describe('RadioInfo carries only fields something reads', () => {
  const fields = Object.keys(iface.shapes.RadioInfo);
  test('every field has a named reader, and each reader is found in the source', () => {
    expect(problems(iface.shapes.RadioInfo)).toEqual([]);
  });
  test('a field added to the shape is refused, and a reader that is only a comment or a longer name is not a reader', () => {
    expect(problems({ ...iface.shapes.RadioInfo, heard: 'string' })).toEqual(['RadioInfo.heard has no reader: delete it from interface/bar.json and BarState.swift']);
    expect(new RegExp('\\bri\\.title\\b').test(stripped('uri.title ri.titleText // ri.title\n'))).toBe(false);
  });
  test('the shape is the three fields the contracts read: no title, no playing, no sentence or progress (what plays is shared code: docs/design/sentence-follow-in-shared-code-2026-11-28.md)', () => {
    expect(fields).toEqual(['level', 'speedValue', 'micLevel']);
  });
  test('the interface default for radioView has exactly the shape\'s fields', () => {
    const view = iface.names.find((n: { source?: string }) => n.source === 'radioView');
    expect(Object.keys(view.substitute.default).sort()).toEqual([...fields].sort());
  });
});

describe('BarFacts carries only what the voice alone reports, and bar.ts reads every field', () => {
  const bar = read(`${here}bar.ts`);
  const fields = Object.keys(iface.shapes.BarFacts);
  test('the shape is the voice\'s facts: no title, no playing, no block numbers', () => {
    expect(fields).toEqual(['at', 'phase', 'transcript', 'message', 'handsFree', 'holding', 'revisedAt', 'errorAt', 'errorStage', 'errorMessage']);
  });
  test('every field is read by bar.ts (`f.<field>`), and the default for barFacts has exactly the shape\'s fields', () => {
    for (const f of fields) expect(new RegExp(`\\bf\\.${f}\\b`).test(bar), f).toBe(true);
    const view = iface.names.find((n: { source?: string }) => n.source === 'barFacts');
    expect(Object.keys(view.substitute.default).sort()).toEqual([...fields].sort());
  });
});
