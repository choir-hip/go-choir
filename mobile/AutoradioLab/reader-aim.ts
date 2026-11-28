// What the bar's prompt is aimed at: a document, or one block of it. It was Swift (`ReaderAim`, with the ops `readerAim` and `readerAimSet`) although nothing native ever read it: the
// reader sets it (a long press on a block, the page menu), the reader and the bar show it. It is shared code now, so Android has it too.
//
// The memory is one aim, kept under the account generation (api.ts): a sign-out moves the generation, so the old account's aim reads as nothing and is never the next account's.
// It is dropped, not shown, when what it points at is gone:
//  - the page is no longer open (closed, or replaced in the stack): the aim goes for good and does not come back when the address is opened again;
//  - the document in FRONT is that page, it has loaded, and the block is not among its blocks: the aim goes for good (an edit, a restore, a revision removed it).
// A document that is still loading, or failed, or is another page, cannot be judged, so a block aim stays until it can.
// Every value `aimView` reads is an argument (the open pages, the document's address, status and block ids) except the held aim itself.
import { accountGeneration } from './api.ts';

export type Aim = { address: string; blockId: string; label: string };
const NONE = (): Aim => ({ address: '', blockId: '', label: '' });
/** The longest label kept: a block's first words, never a document. */
export const LABEL_MAX = 200;

let held: { gen: number; aim: Aim } = { gen: -1, aim: NONE() };
const str = (v: unknown): string => (typeof v === 'string' ? v : '');
const lines = (v: unknown): string[] => (typeof v === 'string' && v !== '' ? v.split('\n') : []);

/** Forget the aim (tests). */
export function forgetAim(): void { held = { gen: -1, aim: NONE() }; }
/** Aim the prompt at `address` (a whole document when `blockId` is empty). An empty or non-text address clears it. Returns the aim now held. */
export function setAim(address: unknown, blockId: unknown, label: unknown): Aim {
  const a = str(address);
  held = { gen: accountGeneration(), aim: a === '' ? NONE() : { address: a, blockId: str(blockId), label: [...str(label)].slice(0, LABEL_MAX).join('') } };
  return { ...held.aim };
}
export function clearAim(): Aim { held = { gen: accountGeneration(), aim: NONE() }; return NONE(); }

/** The aim now. `open`: the open pages' addresses, one a line; `docAddress`, `docStatus` and `blocks` (ids, one a line): the document in front. */
export function aimView(open: unknown, docAddress: unknown, docStatus: unknown, blocks: unknown): Aim {
  if (held.gen !== accountGeneration()) { held = { gen: accountGeneration(), aim: NONE() }; return NONE(); }
  const a = held.aim;
  if (a.address === '') return NONE();
  if (!lines(open).includes(a.address)) { held = { gen: held.gen, aim: NONE() }; return NONE(); }
  if (a.blockId !== '' && str(docAddress) === a.address && str(docStatus) === 'ready' && !lines(blocks).includes(a.blockId)) { held = { gen: held.gen, aim: NONE() }; return NONE(); }
  return { ...a };
}
