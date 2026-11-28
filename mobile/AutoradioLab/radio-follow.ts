// The sentence the radio is saying, and its place on the page in front, from the place native reports in the deck's state (`DeckState.place`: docs/design/sentence-follow-in-shared-code-2026-11-28.md).
// Native reports WHERE it is in the text shared code handed it (an entry id, a digest of that text, the unit's range, the seconds). Everything made from that is here, on both platforms.
// Every function is pure: every value it reads is an argument (the deck's state, the on-air entry, the text handed to the deck, the page's block ids and texts). No module state, no cache, no clock.
// A sentence is lit only when the ids and the digest agree; otherwise nothing is, which is the safe direction (one beat of nothing at a change of entry).
import { NO_PLACE, type DeckPlace, type DeckState, type DeckWhy } from './deck.ts';
import { readState } from './deck-native.ts';
import { MAX_SENTENCE_CHARS, sentenceEnds } from './reader-parse.ts';

export type Place = DeckPlace;

/** The on-air entry as the contract hands it: its id and the address of the page it reads (`airAddressOf`), and what shared code last handed the deck under that id (`handedId`, `handedText`: the item's `paragraphs`). */
export type Air = { id: string; address: string; handedId: string; handedText: string };
export type Follow = { address: string; blockId: string; sentence: number };
export const NO_FOLLOW: Follow = { address: '', blockId: '', sentence: -1 };

const str = (v: unknown): string => (typeof v === 'string' ? v : '');
const obj = (v: unknown): Record<string, unknown> => (typeof v === 'object' && v !== null && !Array.isArray(v) ? (v as Record<string, unknown>) : {});
const readAir = (v: unknown): Air => { const o = obj(v); return { id: str(o.id), address: str(o.address), handedId: str(o.handedId), handedText: str(o.handedText) }; };

/** The length in UTF-16 code units, a colon, and the 32-bit FNV-1a of those code units as eight hex digits. The iPhone (`DeckDigest`) and Android (`DeckDigest`) make the same string; each pins the same test values. */
export function digestOf(text: string): string {
  let h = 0x811c9dc5;
  for (let i = 0; i < text.length; i++) h = Math.imul(h ^ text.charCodeAt(i), 0x01000193);
  return `${text.length}:${(h >>> 0).toString(16).padStart(8, '0')}`;
}

/** The sentences of a unit's words, each trimmed, none empty, cut by the reader's own rule so the radio's sentence and the page's are the same. Words with no stop are one sentence; no words, none. */
export function sentencesOf(text: string): string[] {
  const out: string[] = [];
  let from = 0;
  for (const end of sentenceEnds(text)) { const s = text.slice(from, end).trim(); if (s !== '') out.push(s); from = end; }
  return out.length === 0 && text.trim() !== '' ? [text.trim()] : out;
}

/** A place in a unit never reaches 1: the end of a unit is not the start of the next, so the last sentence stays lit until the unit changes. */
const MAX_PLACE = 0.99;
/** The sentence a place in the unit falls in, by the share of the unit's characters each takes. One sentence or none: the first. A place past the end is the last; NaN and negative are the start. */
export function sentenceAt(sentences: string[], fraction: number): number {
  const f = Number.isNaN(fraction) ? 0 : Math.min(MAX_PLACE, Math.max(0, fraction));
  const total = sentences.reduce((n, s) => n + [...s].length, 0);
  if (total === 0 || sentences.length < 2) return 0;
  let acc = 0;
  for (let i = 0; i < sentences.length; i++) { acc += [...sentences[i]].length; if (acc / total > f) return i; }
  return sentences.length - 1;
}

/** Media seconds over media seconds, 0 when the unit's length is not known (0, or a negative number: ExoPlayer's unknown duration), never past 0.99 of the unit (see MAX_PLACE). */
export function fractionOf(p: Place): number {
  const s = Number(p.seconds), t = Number(p.total);
  return t > 0 && Number.isFinite(s) && Number.isFinite(t) ? Math.min(MAX_PLACE, Math.max(0, s / t)) : 0;
}

/** The words of the unit being spoken, sliced from the text shared code handed over: only when that text's digest is the place's, and the range lies inside it. Anything else is "". It is never searched for. */
export function unitOf(p: Place, text: string): string {
  if (p.digest === '' || p.digest !== digestOf(text)) return '';
  if (!Number.isInteger(p.start) || !Number.isInteger(p.length) || p.start < 0 || p.length <= 0 || p.start + p.length > text.length) return '';
  return text.slice(p.start, p.start + p.length);
}

/** True only when the deck is on the entry on air, the text handed was that entry's, and the deck is in one of `whys`: the three ids (the place's, the deck's item, the sheet's on-air entry) and the handed id are equal and not empty. */
export function owns(ds: DeckState, air: Air, whys: readonly DeckWhy[]): boolean {
  const e = ds.place.entry;
  return e !== '' && e === ds.item && e === air.id && e === air.handedId && whys.includes(ds.why);
}

/** Lower-cased, curly quotes and dashes straightened, runs of white space collapsed and trimmed: what the spoken words and the page's text are compared through. */
export function normalised(s: string): string {
  let out = '', lastSpace = true;
  for (const ch of s) {
    let c = ch;
    if (c === '‘' || c === '’') c = "'"; else if (c === '“' || c === '”') c = '"'; else if (c === '–' || c === '—') c = '-';
    if (/\s/.test(c)) { if (!lastSpace) { out += ' '; lastSpace = true; } } else { out += c.toLowerCase(); lastSpace = false; }
  }
  return out.endsWith(' ') ? out.slice(0, -1) : out;
}

/** The sentences of a page block as the reader numbers them (every end of `sentenceEnds` is one, none dropped; a block over the reader's size is one). */
function pageSentences(text: string): string[] {
  if (text === '' || text.length > MAX_SENTENCE_CHARS) return [text];
  const out: string[] = []; let from = 0;
  for (const end of sentenceEnds(text)) { out.push(text.slice(from, end)); from = end; }
  return out;
}

/** The block of the page whose text contains the sentence, and the page's sentence that contains its head; the first block that does. None when the sentence is under 6 characters (too short to be sure of) or in no block. */
export function locate(sentence: string, blocks: { id: string; text: string }[]): { block: string; sentence: number } | undefined {
  const t = sentence.trim();
  if (t === '') return undefined;
  const key = normalised([...t].slice(0, 60).join(''));
  if ([...key].length < 6) return undefined;
  const head = [...key].slice(0, 24).join('');
  for (const b of blocks) {
    if (b.text === '' || !normalised(b.text).includes(key)) continue;
    return { block: b.id, sentence: Math.max(0, pageSentences(b.text).findIndex((s) => normalised(s).includes(head))) };
  }
  return undefined;
}

/** The sentence being said now, from the unit's words and the place in it; "" when it cannot be told. */
function sayingOf(p: Place, text: string): string {
  const unit = unitOf(p, text);
  const sentences = sentencesOf(unit);
  return sentences[sentenceAt(sentences, fractionOf(p))] ?? '';
}

/** What the reader lights: the block and sentence of the page at `address` that the radio is saying, or none. `ids` and `texts` are the page's blocks (ids one a line; texts joined by U+001F). Lit only while the deck plays the entry on air, which reads that page. */
export function followOf(rawDeck: unknown, rawAir: unknown, address: string, ids: string, texts: string): Follow {
  const ds = readState(rawDeck), air = readAir(rawAir);
  if (address === '' || address !== air.address || !owns(ds, air, ['playing'])) return NO_FOLLOW;
  const say = sayingOf(ds.place, air.handedText);
  const idList = ids.split('\n'), textList = texts.split('\u001F');
  if (say === '' || idList.length !== textList.length) return NO_FOLLOW;
  const hit = locate(say, idList.map((id, i) => ({ id, text: textList[i] })));
  return hit ? { address, blockId: hit.block, sentence: hit.sentence } : NO_FOLLOW;
}

/** The Autoradio page's sentence line and the entry's progress. Shown while the entry on air is the deck's and the deck is playing or paused (the place stands); the station's own words (no address) show too. Progress is the unit's start plus its share, over the entry's length. */
export function nowOf(rawDeck: unknown, rawAir: unknown): { sentence: string; progress: number } {
  const ds = readState(rawDeck), air = readAir(rawAir), p = ds.place;
  if (!owns(ds, air, ['playing', 'paused_listener', 'paused_system']) || unitOf(p, air.handedText) === '') return { sentence: '', progress: 0 };
  const share = Number.isFinite(p.seconds) && p.total > 0 ? Math.min(1, Math.max(0, p.seconds / p.total)) : 0;
  return { sentence: sayingOf(p, air.handedText), progress: Math.min(1, (p.start + share * p.length) / air.handedText.length) };
}

export { NO_PLACE };
