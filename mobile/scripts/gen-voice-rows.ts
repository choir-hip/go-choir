// Makes mobile/AutoradioKit/Sources/AutoradioKit/Voice/VoiceRows.swift from the voice packs' own metadata, so no voice's name or blurb is typed by hand
// (owner, 2026-11-24: more voices in Voice and models; names and blurbs from the pack).
//   Kokoro 82M (FluidInference/kokoro-82m-coreml): the pack lists `voices/<id>.json`; the id is Kokoro's own code (a, b = American, British; f, m = female, male;
//     the part after the underscore is the voice's name). English voices only: af, am, bf, bm. A voice of another language is not listed.
//   Phonon 31M (FluidInference/pocket-tts-coreml, phonon/phonon-31m-en): the pack's `voices_catalog.jsonl` gives each voice's name, locale, gender and
//     description; the id the library takes is `<locale with _>_<name>` (FluidAudio's own PhononDemo lists them by `availableVoices()`). English only: en-us, en-gb.
// The snapshots are in scripts/voice-pack/ (the pack's files at a pinned commit, copied as they are). `bun scripts/gen-voice-rows.ts --refresh` fetches them again at the
// pinned commits and regenerates; `--check` fails when the committed Swift file differs from what the snapshots make. A model with no credit refuses to generate.
import { readFileSync, writeFileSync } from 'node:fs';

export type Pack = { model: string; stage: string; label: string; group: string; license: string; credit: string };
export type Row = { id: string; group: string; stage: string; voice: string; model: string; name: string; blurb: string; sample: string; license: string; credit: string; bytes: number };
type KokoroSnap = { repo: string; commit: string; files: { path: string; bytes: number }[] };
type PhononPin = { repo: string; commit: string; path: string };

/** The default voices: Kokoro's is the radio's (the owner's pick, first on the cards); Phonon's is the stage's own default (`PhononVoice`, a setting with no voice means it). Each is first of its model. */
const DEFAULT_KOKORO = 'af_heart', DEFAULT_PHONON = 'en_us_Easton';
const APACHE = 'Licensed under Apache-2.0 (https://www.apache.org/licenses/LICENSE-2.0).';
export const PACKS: Pack[] = [
  { model: 'kokoro-82m-ane', stage: 'kokoro', label: 'Kokoro', group: 'Kokoro 82M', license: 'Apache-2.0', credit: `Kokoro 82M voice by hexgrad, Core ML build by FluidInference. ${APACHE}` },
  { model: 'phonon-31m', stage: 'phonon', label: 'Phonon', group: 'Phonon 31M', license: 'Apache-2.0', credit: `Phonon 31M voice by Gradium, Core ML build by FluidInference. ${APACHE}` },
];

const cap = (s: string) => s.charAt(0).toUpperCase() + s.slice(1);
const firstSentence = (s: string) => { const m = s.match(/^.*?[.!?](?=\s|$)/); return (m ? m[0] : s).trim(); };

export function rows(kokoro: KokoroSnap, phononJsonl: string, packs: Pack[] = PACKS): Row[] {
  for (const p of packs) if (!p.credit.trim()) throw new Error(`${p.model} has no credit: a licence that asks for a notice is not offered without it`);
  const pack = (m: string) => packs.find((p) => p.model === m)!;
  const out: Row[] = [];
  const k = pack('kokoro-82m-ane');
  if (k) {
    const accent: Record<string, string> = { a: 'American', b: 'British' }, sex: Record<string, string> = { f: 'female', m: 'male' };
    const voices = kokoro.files.map((f) => ({ f, id: f.path.replace(/^voices\//, '').replace(/\.json$/, '') })).filter((v) => /^(af|am|bf|bm)_[a-z]+$/.test(v.id));
    voices.sort((a, b) => (a.id === DEFAULT_KOKORO ? -1 : b.id === DEFAULT_KOKORO ? 1 : a.id.localeCompare(b.id)));
    for (const v of voices) {
      const name = cap(v.id.slice(3));
      out.push({ group: k.group, id: `${k.stage}:${v.id}`, stage: k.stage, voice: v.id, model: k.model, name, blurb: `${accent[v.id[0]]}, ${sex[v.id[1]]} voice`, sample: `Hello, I'm ${name}. I'll read you the paper.`, license: k.license, credit: k.credit, bytes: v.f.bytes });
    }
  }
  const p = pack('phonon-31m');
  if (p) {
    const pv: Row[] = [];
    for (const line of phononJsonl.split('\n').filter((l) => l.trim())) {
      const r = JSON.parse(line) as { name: string; locale: string; description: string };
      if (r.locale !== 'en-us' && r.locale !== 'en-gb') continue;
      const voice = `${r.locale.replace('-', '_')}_${r.name}`;
      pv.push({ group: p.group, id: `${p.stage}:${voice}`, stage: p.stage, voice, model: p.model, name: `${r.name} (${p.label})`, blurb: firstSentence(r.description), sample: `Hello, I'm ${r.name}. This is Phonon, a small voice that runs on your phone.`, license: p.license, credit: p.credit, bytes: 0 });
    }
    pv.sort((a, b) => (a.voice === DEFAULT_PHONON ? -1 : b.voice === DEFAULT_PHONON ? 1 : 0));
    out.push(...pv);
  }
  return out;
}

const q = (s: string) => JSON.stringify(s);
export function render(kokoro: KokoroSnap, phononJsonl: string, phononPin: PhononPin): string {
  const lines = rows(kokoro, phononJsonl).map((r) => `        VoiceRow(id: ${q(r.id)}, stageID: ${q(r.stage)}, voice: ${q(r.voice)}, modelID: ${q(r.model)}, group: ${q(r.group)}, name: ${q(r.name)}, blurb: ${q(r.blurb)}, sample: ${q(r.sample)}, license: ${q(r.license)}, credit: ${q(r.credit)}, bytes: ${r.bytes}),`);
  return `import Foundation

// GENERATED by mobile/scripts/gen-voice-rows.ts from the voice packs' own metadata (scripts/voice-pack/). Do not edit: edit the generator or refresh the snapshots.
// Kokoro: ${kokoro.repo} at ${kokoro.commit}. Phonon: ${phononPin.repo} at ${phononPin.commit}, ${phononPin.path}.
// English voices only. A row carries the credit its licence asks for; a pack with no credit makes the generator refuse.

public struct VoiceRow: Equatable, Sendable {
    public var id: String, stageID: String, voice: String, modelID: String, group: String, name: String, blurb: String, sample: String, license: String, credit: String
    /// The voice file's own size where the pack lists it (Kokoro's \`voices/<id>.json\`); 0 where the pack ships all its voices in one fetch (Phonon).
    public var bytes: Int64
}

public enum VoiceRows {
    public static let kokoroCommit = ${q(kokoro.commit)}
    public static let phononCommit = ${q(phononPin.commit)}
    public static let all: [VoiceRow] = [
${lines.join('\n')}
    ]
}
`;
}

if (import.meta.main) {
  const dir = new URL('./voice-pack/', import.meta.url).pathname;
  const out = new URL('../AutoradioKit/Sources/AutoradioKit/Voice/VoiceRows.swift', import.meta.url).pathname;
  const arg = process.argv[2] ?? '';
  const read = (f: string) => readFileSync(dir + f, 'utf8');
  if (arg === '--refresh') {
    const kp = JSON.parse(read('kokoro-voices.json')) as KokoroSnap, pp = JSON.parse(read('phonon.json')) as PhononPin;
    const tree = (await (await fetch(`https://huggingface.co/api/models/${kp.repo}/tree/${kp.commit}/voices`)).json()) as { path: string; size: number }[];
    writeFileSync(dir + 'kokoro-voices.json', JSON.stringify({ repo: kp.repo, commit: kp.commit, files: tree.filter((t) => t.path.endsWith('.json')).map((t) => ({ path: t.path, bytes: t.size })).sort((a, b) => a.path.localeCompare(b.path)) }, null, 1));
    writeFileSync(dir + 'phonon-voices_catalog.jsonl', await (await fetch(`https://huggingface.co/${pp.repo}/resolve/${pp.commit}/${pp.path}`)).text());
  }
  const text = render(JSON.parse(read('kokoro-voices.json')), read('phonon-voices_catalog.jsonl'), JSON.parse(read('phonon.json')));
  if (arg === '--check') { if (readFileSync(out, 'utf8') !== text) { console.error('VoiceRows.swift is not what the generator makes'); process.exit(1); } }
  else writeFileSync(out, text);
}
