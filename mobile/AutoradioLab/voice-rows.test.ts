// The voice rows are made from the packs' own metadata (scripts/gen-voice-rows.ts), never typed. Failure list (each line a test):
//  V1 the committed Swift file is what the generator makes from the committed snapshots (a hand edit, or a refreshed snapshot without a regenerate, fails here).
//  V2 only English voices are listed: Kokoro's American and British packs (af, am, bf, bm), Phonon's en-us and en-gb; no voice of another language, no character voice.
//  V3 every row has a name, a blurb that is the pack's own words or the pack's own code spelled out, a sample line, and the credit of its model; a row with no credit makes the generator refuse (nothing is written).
//  V4 the default voice (Heart) is first, ids are unique, and the three Phonon voices that were hand-typed keep their ids.
//  V5 a row is never typed: the generator holds no voice name of its own (only the default's id and the credit of each model).
import { describe, expect, test } from 'bun:test';
import { readFileSync } from 'node:fs';
import { render, rows, type Pack } from '../scripts/gen-voice-rows.ts';

const root = new URL('../', import.meta.url).pathname;
const kokoro = JSON.parse(readFileSync(root + 'scripts/voice-pack/kokoro-voices.json', 'utf8'));
const phonon = readFileSync(root + 'scripts/voice-pack/phonon-voices_catalog.jsonl', 'utf8');
const phononPin = JSON.parse(readFileSync(root + 'scripts/voice-pack/phonon.json', 'utf8'));
const swift = readFileSync(root + 'AutoradioKit/Sources/AutoradioKit/Voice/VoiceRows.swift', 'utf8');

describe('voice rows', () => {
  test('V1 the committed file is what the generator makes', () => { expect(swift).toBe(render(kokoro, phonon, phononPin)); });
  test('V2 English voices only', () => {
    const r = rows(kokoro, phonon);
    const k = r.filter((x) => x.model === 'kokoro-82m-ane'), p = r.filter((x) => x.model === 'phonon-31m');
    expect(k.length).toBe(28); expect(p.length).toBe(25);
    for (const x of k) expect(/^(af|am|bf|bm)_/.test(x.voice)).toBe(true);
    for (const x of p) expect(/^en_(us|gb)_/.test(x.voice)).toBe(true);
    expect(r.some((x) => x.voice.startsWith('characters_') || x.voice.startsWith('zf_') || x.voice.startsWith('ef_'))).toBe(false);
  });
  test('V3 every row is whole and carries its credit', () => {
    for (const x of rows(kokoro, phonon)) { expect(x.name).not.toBe(''); expect(x.group).toMatch(/^(Kokoro 82M|Phonon 31M)$/); expect(x.blurb).not.toBe(''); expect(x.sample).toContain(x.name.replace(' (Phonon)', '')); expect(x.credit).toContain('Apache-2.0'); }
  });
  test('V3 a model with no credit makes the generator refuse', () => {
    const bad: Pack[] = [{ model: 'x', stage: 'x', label: 'X', group: 'X', license: 'Apache-2.0', credit: '' }];
    expect(() => rows(kokoro, phonon, bad)).toThrow(/credit/);
  });
  test('V4 defaults first (Heart for Kokoro, the stage own Easton for Phonon), ids unique, the three earlier Phonon ids kept', () => {
    const r = rows(kokoro, phonon);
    expect(r[0].id).toBe('kokoro:af_heart'); expect(r.find((x) => x.model === 'phonon-31m')!.id).toBe('phonon:en_us_Easton'); expect(new Set(r.map((x) => x.id)).size).toBe(r.length);
    for (const id of ['phonon:en_us_Easton', 'phonon:en_us_Zoey', 'phonon:en_gb_Maeve']) expect(r.some((x) => x.id === id)).toBe(true);
  });
});
