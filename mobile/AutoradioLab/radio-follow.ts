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

/** The pieces the reader numbers a text into: every end of `sentenceEnds` is one (none dropped), and a text over the reader's size is one piece. The page's blocks and the entry's paragraphs are cut by this one function. */
function piecesOf(text: string): { s: number; e: number }[] {
  if (text.length > MAX_SENTENCE_CHARS) return [{ s: 0, e: text.length }];
  const out: { s: number; e: number }[] = []; let from = 0;
  for (const end of sentenceEnds(text)) { out.push({ s: from, e: end }); from = end; }
  return out;
}

/** The trimmed, non-empty sentences of a text as [start, end) offsets, from `piecesOf`, so the radio's sentence and the page's are the same. */
function spansOf(text: string, base = 0): { s: number; e: number }[] {
  const out: { s: number; e: number }[] = [];
  for (const p of piecesOf(text)) {
    const raw = text.slice(p.s, p.e), lead = raw.length - raw.trimStart().length, trimmed = raw.trim();
    if (trimmed !== '') out.push({ s: base + p.s + lead, e: base + p.s + lead + trimmed.length });
  }
  return out;
}

/** The entry's sentences: each paragraph (a blank line ends one) is cut by `spansOf`, as each page block is. */
function entrySpans(text: string): { s: number; e: number }[] {
  const out: { s: number; e: number }[] = [];
  let from = 0;
  for (;;) {
    const cut = text.indexOf('\n\n', from), end = cut < 0 ? text.length : cut;
    out.push(...spansOf(text.slice(from, end), from));
    if (cut < 0) return out;
    from = cut + 2;
  }
}

/** The sentences of a unit's words, each trimmed, none empty. Words with no stop are one sentence; no words, none. */
export function sentencesOf(text: string): string[] { return spansOf(text).map((x) => text.slice(x.s, x.e)); }

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

const isSpace = (c: number): boolean => c === 32 || (c >= 9 && c <= 13) || c === 0xa0 || c === 0x1680 || (c >= 0x2000 && c <= 0x200a) || c === 0x2028 || c === 0x2029 || c === 0x202f || c === 0x205f || c === 0x3000 || c === 0xfeff;

/** Lower-cased, curly quotes and dashes straightened, runs of white space collapsed and trimmed: what two sentences are compared through. One pass over the code units (it is called for every sentence of the entry and the page). */
export function normalised(s: string): string {
  let out = '', lastSpace = true;
  for (let i = 0; i < s.length; i++) {
    const c = s.charCodeAt(i);
    if (isSpace(c)) { if (!lastSpace) { out += ' '; lastSpace = true; } continue; }
    lastSpace = false;
    if (c < 128) { out += c >= 65 && c <= 90 ? String.fromCharCode(c + 32) : s[i]; continue; }
    if (c === 0x2018 || c === 0x2019) out += "'"; else if (c === 0x201c || c === 0x201d) out += '"'; else if (c === 0x2013 || c === 0x2014) out += '-';
    else if (c >= 0xd800 && c <= 0xdbff && i + 1 < s.length) { out += s.slice(i, i + 2).toLowerCase(); i++; }
    else out += s[i].toLowerCase();
  }
  return out.endsWith(' ') ? out.slice(0, -1) : out;
}

/** The sentence being said, and where it begins in the entry's text: the unit's own range plus the sentence's offset in the unit. Nothing when the place is not valid for `text`. */
function sayingOf(p: Place, text: string): { text: string; at: number } | undefined {
  const unit = unitOf(p, text);
  const spans = spansOf(unit);
  if (spans.length === 0) return undefined;
  const k = sentenceAt(spans.map((x) => unit.slice(x.s, x.e)), fractionOf(p)), x = spans[k];
  return { text: unit.slice(x.s, x.e), at: p.start + x.s };
}

const MIN_SENTENCE = 6;

/**
 * The block and sentence of the page that the entry's sentence at offset `at` is, by this one rule: let K be the normalised entry sentence holding `at`, and k its number among the entry's sentences equal to K (1-based, in order).
 * Light the k-th sentence equal to K on the page, if and only if the page has exactly as many sentences equal to K as the entry has; otherwise light nothing. It cannot light another copy (same count, same ordinal), costs one pass
 * over each text, and gives up a sentence the page shows more or fewer times than the entry has, one under 6 characters, and one the page does not have.
 */
export function locateAt(entry: string, at: number, blocks: { id: string; text: string }[]): { block: string; sentence: number } | undefined {
  const spans = entrySpans(entry);
  const at0 = spans.findIndex((x) => at >= x.s && at < x.e);
  if (at0 < 0) return undefined;
  const key = normalised(entry.slice(spans[at0].s, spans[at0].e));
  if ([...key].length < MIN_SENTENCE) return undefined;
  let k = 0, total = 0;
  spans.forEach((x, j) => { if (normalised(entry.slice(x.s, x.e)) === key) { total++; if (j <= at0) k++; } });
  let seen = 0, hit: { block: string; sentence: number } | undefined;
  for (const b of blocks) {
    piecesOf(b.text).forEach((p, i) => {
      if (normalised(b.text.slice(p.s, p.e)) !== key) return;
      if (++seen === k) hit = { block: b.id, sentence: i };
    });
  }
  return seen === total ? hit : undefined;
}

/** What the reader lights: the block and sentence of the page at `address` that the radio is saying, or none. `ids` and `texts` are the page's blocks (ids one a line; texts joined by U+001F). Lit only while the deck plays the entry on air, which reads that page. */
export function followOf(rawDeck: unknown, rawAir: unknown, address: string, ids: string, texts: string): Follow {
  const ds = readState(rawDeck), air = readAir(rawAir);
  if (address === '' || address !== air.address || !owns(ds, air, ['playing'])) return NO_FOLLOW;
  const say = sayingOf(ds.place, air.handedText);
  const idList = ids.split('\n'), textList = texts.split('\u001F');
  if (!say || [...normalised(say.text)].length < MIN_SENTENCE || idList.length !== textList.length) return NO_FOLLOW;
  const hit = locateAt(air.handedText, say.at, idList.map((id, i) => ({ id, text: textList[i] })));
  return hit ? { address, blockId: hit.block, sentence: hit.sentence } : NO_FOLLOW;
}

/** The Autoradio page's sentence line and the entry's progress. Shown while the entry on air is the deck's and the deck is playing or paused (the place stands); the station's own words (no address) show too. Progress is the unit's start plus its share, over the entry's length. */
export function nowOf(rawDeck: unknown, rawAir: unknown): { sentence: string; progress: number } {
  const ds = readState(rawDeck), air = readAir(rawAir), p = ds.place;
  if (!owns(ds, air, ['playing', 'paused_listener', 'paused_system']) || unitOf(p, air.handedText) === '') return { sentence: '', progress: 0 };
  const share = Number.isFinite(p.seconds) && p.total > 0 ? Math.min(1, Math.max(0, p.seconds / p.total)) : 0;
  return { sentence: sayingOf(p, air.handedText)?.text ?? '', progress: Math.min(1, (p.start + share * p.length) / air.handedText.length) };
}

/** What the bar says is playing, from the run sheet and the deck (`barRun`): the title of the entry on air, and whether the deck is playing it, paused (by the listener or the system), or held for the audio session. Every value is an argument:
 *  `view` is what the sheet says of the entry on air (its title, whether it is audio), the deck's state and the on-air entry as the contract hands them. The deck speaks for the entry on air only when its item IS that entry's id: any other
 *  deck state (an entry that just ended, one the sheet has not yet put on air) is no verdict about it, and the bar shows nothing playing. Progress: audio is the seconds over its length; spoken words are the place's share of the text handed
 *  over (`nowOf`), 0 when the place is not that entry's. Nothing is kept. */
export function runOf(view: { onAir: boolean; title: string; audio: boolean }, rawDeck: unknown, rawAir: unknown): { ready: boolean; onAir: boolean; waiting: boolean; playing: boolean; paused: boolean; title: string; progress: number } {
  const ds = readState(rawDeck), air = readAir(rawAir);
  const on = view.onAir && air.id !== '' && ds.item === air.id;
  const playing = on && ds.why === 'playing', paused = on && (ds.why === 'paused_listener' || ds.why === 'paused_system'), waiting = on && ds.why === 'held_session';
  let progress = 0;
  if (on && view.audio) progress = ds.duration > 0 && Number.isFinite(ds.seconds) ? Math.min(1, Math.max(0, ds.seconds / ds.duration)) : 0;
  else if (on) progress = nowOf(rawDeck, rawAir).progress;
  return { ready: true, onAir: view.onAir, waiting, playing, paused, title: view.onAir ? view.title : '', progress };
}

export { NO_PLACE };
