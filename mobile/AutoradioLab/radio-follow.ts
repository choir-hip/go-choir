// The sentence the radio is saying, from the facts native hands over, and the place of it on the page in front. Shared code on both platforms; native hands over only FACTS:
// the spoken words of the block on air, the audio's own place in that block (0 to 1) and the block's number among the programme's. Everything made from them is here:
// the sentences, the sentence the place falls in, the programme's progress, and the block and sentence of the page the reader draws that the words are in (the follow-along).
// A sentence is the one splitter's (reader-parse.ts `sentenceEnds`): the radio's sentence and the page's are cut by the same rule, so the words match.
import { sentenceEnds, sentenceTexts, type Block } from './reader-parse.ts';

/** What native knows (`radioFacts`): the spoken words of the block on air, the audio's place in it, the block's number and how many there are. */
export type SpokenFacts = { spoken: string; block: number; blocks: number; fraction: number };

/** The sentences of a block's spoken words, each trimmed, none empty. Words with no stop are one sentence; no words, none. */
export function sentencesOf(text: string): string[] {
  const out: string[] = [];
  let from = 0;
  for (const end of sentenceEnds(text)) { const s = text.slice(from, end).trim(); if (s !== '') out.push(s); from = end; }
  return out.length === 0 && text.trim() !== '' ? [text.trim()] : out;
}

/** The sentence a place in the block falls in, by the share of the block's characters each takes. One sentence or none: the first. A place past the end is the last. */
export function sentenceAt(sentences: string[], progress: number): number {
  const total = sentences.reduce((n, s) => n + [...s].length, 0);
  if (total === 0 || sentences.length < 2) return 0;
  let acc = 0;
  for (let i = 0; i < sentences.length; i++) {
    acc += [...sentences[i]].length;
    if (acc / total > progress) return i;
  }
  return sentences.length - 1;
}

/** A place in a block never reaches 1 (the audio's own cap, as native always had it): the end of a block is not the start of the next, so progress in the middle of a programme is never the next block's. */
const MAX_PLACE = 0.99;
export type SpokenNow = { sentences: string[]; sentenceIndex: number; progress: number };
/** The sentences of the block on air, the one being said, and how far through the programme the audio is. A block number past the end is the last block; no blocks is no progress. */
export function spokenNow(f: SpokenFacts): SpokenNow {
  const fin = (v: number): number => (Number.isFinite(v) ? Math.floor(v) : 0);
  const n = Math.max(0, fin(f.blocks)), idx = n === 0 ? 0 : Math.min(Math.max(fin(f.block), 0), n - 1);
  const bp = Number.isFinite(f.fraction) ? Math.min(MAX_PLACE, Math.max(0, f.fraction)) : 0;
  const sentences = n === 0 ? [] : sentencesOf(f.spoken);
  return { sentences, sentenceIndex: sentenceAt(sentences, bp), progress: n === 0 ? 0 : Math.min(1, (idx + bp) / n) };
}

/** Lower-cased, curly quotes and dashes straightened, runs of white space collapsed and trimmed: what spoken words and the page's text are compared through. */
export function normalised(s: string): string {
  let out = '', lastSpace = true;
  for (const ch of s) {
    let c = ch;
    if (c === '‘' || c === '’') c = "'"; else if (c === '“' || c === '”') c = '"'; else if (c === '–' || c === '—') c = '-';
    if (/\s/.test(c) || c === ' ') { if (!lastSpace) { out += ' '; lastSpace = true; } } else { out += c.toLowerCase(); lastSpace = false; }
  }
  return out.endsWith(' ') ? out.slice(0, -1) : out;
}

export type Follow = { address: string; blockId: string; sentence: number };
export const NO_FOLLOW: Follow = { address: '', blockId: '', sentence: -1 };

/** The block whose text contains the spoken words, and the sentence of it that contains their start; the first block that does. None when the words are too short to be sure of (under 6 characters) or in no block. */
export function locate(spoken: string, blocks: Block[]): { block: string; sentence: number } | undefined {
  const t = spoken.trim();
  if (t === '') return undefined;
  const key = normalised([...t].slice(0, 60).join(''));
  if ([...key].length < 6) return undefined;
  const head = [...key].slice(0, 24).join('');
  for (const b of blocks) {
    if (b.text === '' || !normalised(b.text).includes(key)) continue;
    const at = sentenceTexts(b).findIndex((s) => normalised(s).includes(head));
    return { block: b.id, sentence: Math.max(0, at) };
  }
  return undefined;
}

/** What the reader lights: the block and sentence of the page at `address` that the radio is saying, or none. `doc` is the page the reader holds for that address. */
export function followOf(address: string, spoken: string, doc: { blocks: Block[] } | undefined): Follow {
  if (address === '' || spoken === '' || !doc) return NO_FOLLOW;
  const hit = locate(spoken, doc.blocks);
  return hit ? { address, blockId: hit.block, sentence: hit.sentence } : NO_FOLLOW;
}

/** The sentence being said, by itself: nothing when the radio is not playing, has no page address or the sentence is past the list. */
export function saying(playing: boolean, address: string, now: SpokenNow): string {
  return playing && address !== '' && now.sentenceIndex >= 0 && now.sentenceIndex < now.sentences.length ? now.sentences[now.sentenceIndex] : '';
}

/** What native told, in one answer taken at one moment: the document the block is in, a key that changes when the block or its words do, and the words only when the key is new to us. */
export type SpokenSnapshot = SpokenFacts & { key: string; document: string };
export const NO_SPOKEN: SpokenSnapshot = { key: '', document: '', spoken: '', block: 0, blocks: 0, fraction: 0 };
const text = (v: unknown): string => (typeof v === 'string' ? v : '');
const number = (v: unknown): number => (typeof v === 'number' && Number.isFinite(v) ? v : 0);
/** The `radioFacts` answer as a snapshot. Native sends the words only when `have` (the key we sent) was not the block's key, so the words of the same key are the ones we held. */
export function readSpoken(o: Record<string, unknown>, held: SpokenSnapshot): SpokenSnapshot {
  const key = text(o.blockKey);
  if (key === '') return NO_SPOKEN;
  const words = typeof o.spoken === 'string' ? o.spoken : held.key === key ? held.spoken : '';
  return { key, document: text(o.document), spoken: words, block: number(o.block), blocks: number(o.blocks), fraction: number(o.blockFraction) };
}

/** Whether the document native is speaking is the entry on air: a story read from the paper is `pub:<route>`, any other entry is `rs:<entry id>`. Anything else is not this entry's, and nothing is lit. */
export function ownsAir(document: string, address: string, entryId: string): boolean {
  return document !== '' && (document === `rs:${entryId}` || document === address.split('@')[0]);
}
