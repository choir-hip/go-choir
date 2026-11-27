// The native `RadioInfo` (interface/bar.json) carries only what shared code reads. A field nothing reads is native work for no one, so adding one back fails here.
// Each field names where it is read: five by the contracts (`ri.<field>`), and three through the `radioFacts` op, which native fills from them and radio-bridge.ts reads.
import { describe, expect, test } from 'bun:test';
import { readFileSync, readdirSync } from 'node:fs';

const here = new URL('.', import.meta.url).pathname;
const iface = JSON.parse(readFileSync(`${here}../exact-package/interface/bar.json`, 'utf8'));
const read = (f: string) => readFileSync(`${here}${f}`, 'utf8');
const contracts = readdirSync(here).filter((f) => f.endsWith('.contract')).map((f) => read(f)).join('\n');
const bridge = read('radio-bridge.ts');
const swiftFacts = readFileSync(`${here}../exact-package/apple/AutoradioModule+Bar.swift`, 'utf8');

const readers: Record<string, (f: string) => boolean> = {
  title: (f) => contracts.includes(`ri.${f}`), playing: (f) => contracts.includes(`ri.${f}`), level: (f) => contracts.includes(`ri.${f}`),
  speedValue: (f) => contracts.includes(`ri.${f}`), micLevel: (f) => contracts.includes(`ri.${f}`),
  sentences: (f) => bridge.includes(`o.${f}`) && swiftFacts.includes(`info.${f}`), sentenceIndex: (f) => bridge.includes(`o.${f}`) && swiftFacts.includes(`info.${f}`),
  progress: (f) => bridge.includes(`o.${f}`) && swiftFacts.includes(`info.${f}`),
};

describe('RadioInfo carries only fields something reads', () => {
  const fields = Object.keys(iface.shapes.RadioInfo);
  test('every field has a named reader, and each reader is found in the source', () => {
    for (const f of fields) {
      expect(readers[f], `RadioInfo.${f} has no reader: delete it from interface/bar.json and BarState.swift`).toBeDefined();
      expect(readers[f](f), `RadioInfo.${f}: its reader is gone`).toBe(true);
    }
  });
  test('the interface default for radioView has exactly the shape\'s fields', () => {
    const view = iface.names.find((n: { source?: string }) => n.source === 'radioView');
    expect(Object.keys(view.substitute.default).sort()).toEqual([...fields].sort());
  });
});
