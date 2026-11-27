// The native `RadioInfo` (interface/bar.json) carries only what shared code reads. A field nothing reads is native work for no one, so adding one back fails here.
// Each field names where it is read: five by the contracts (`ri.<field>`), and three through the `radioFacts` op, which native fills from them and radio-bridge.ts reads.
// The checks run on text with comments removed, and match whole words, so a comment or a longer name (`uri.title`, `ri.titleText`) does not count as a reader.
import { describe, expect, test } from 'bun:test';
import { readFileSync, readdirSync } from 'node:fs';

const here = new URL('.', import.meta.url).pathname;
const stripped = (t: string) => t.replace(/\/\*[\s\S]*?\*\//g, '').replace(/^\s*\/\/.*$/gm, '').replace(/([^:\w"'`])\/\/.*$/gm, '$1');
const read = (path: string) => stripped(readFileSync(path, 'utf8'));
const contracts = readdirSync(here).filter((f) => f.endsWith('.contract')).map((f) => read(`${here}${f}`)).join('\n');
const bridge = read(`${here}radio-bridge.ts`);
const barModule = read(`${here}../exact-package/apple/AutoradioModule+Bar.swift`);
/** The body of `radioFacts(peek:)`, where native fills the op from RadioInfo. */
const radioFactsFn = barModule.slice(barModule.indexOf('func radioFacts(peek'));

export const readers: Record<string, (f: string) => boolean> = {
  title: (f) => new RegExp(`\\bri\\.${f}\\b`).test(contracts), playing: (f) => new RegExp(`\\bri\\.${f}\\b`).test(contracts), level: (f) => new RegExp(`\\bri\\.${f}\\b`).test(contracts),
  speedValue: (f) => new RegExp(`\\bri\\.${f}\\b`).test(contracts), micLevel: (f) => new RegExp(`\\bri\\.${f}\\b`).test(contracts),
  sentences: (f) => new RegExp(`\\bo\\.${f}\\b`).test(bridge) && new RegExp(`\\binfo\\.${f}\\b`).test(radioFactsFn),
  sentenceIndex: (f) => new RegExp(`\\bo\\.${f}\\b`).test(bridge) && new RegExp(`\\binfo\\.${f}\\b`).test(radioFactsFn),
  progress: (f) => new RegExp(`\\bo\\.${f}\\b`).test(bridge) && new RegExp(`\\binfo\\.${f}\\b`).test(radioFactsFn),
};

/** The failures for a shape: a field with no named reader, or whose reader is gone. */
export function problems(shape: Record<string, string>): string[] {
  const out: string[] = [];
  for (const f of Object.keys(shape)) {
    if (!readers[f]) out.push(`RadioInfo.${f} has no reader: delete it from interface/bar.json and BarState.swift`);
    else if (!readers[f](f)) out.push(`RadioInfo.${f}: its reader is gone (a contract's ri.${f}, or radio-bridge.ts and radioFacts in AutoradioModule+Bar.swift)`);
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
  test('the interface default for radioView has exactly the shape\'s fields', () => {
    const view = iface.names.find((n: { source?: string }) => n.source === 'radioView');
    expect(Object.keys(view.substitute.default).sort()).toEqual([...fields].sort());
  });
});
