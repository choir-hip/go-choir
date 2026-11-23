// Keeping the deck's held items in the sheet's order: what to do so the deck holds the next items, and doing it through the deck port. The deck is the
// evidence of what played (docs/reference/exact/deck.md); the verdicts are posted by the run sheet store (runsheet-store.ts), not from here. (The first design
// of this file built posted ops from the deck's journal; it was never wired and is deleted, with its poster.)
import { DECK_HELD_MAX, type DeckItem, type DeckPort } from './deck.ts';

// ---- keeping the deck's held items in the sheet's order

export type HeldPlan = { keep: number; enqueue: { at: number; item: DeckItem }[] };

/** What to do so the deck holds `wanted` (the next items, in the sheet's order) when it holds the ids `held`: keep what matches from the front,
 *  and hold the rest again in order from the first difference. At most DECK_HELD_MAX, each id once. */
export function heldPlan(held: string[], wanted: DeckItem[], max = DECK_HELD_MAX, sent?: Map<string, string>): HeldPlan {
  const want: DeckItem[] = [];
  for (const w of wanted) if (want.length < max && !want.some((x) => x.id === w.id)) want.push(w);
  let keep = 0;
  // `sent`: what shared code last handed the deck under each id. An id held with other words than the sheet has now (the desk rewrote a transition while the deck held it) is not kept: it would play stale.
  while (keep < held.length && keep < want.length && held[keep] === want[keep].id && (sent === undefined || sent.get(want[keep].id) === undefined || sent.get(want[keep].id) === JSON.stringify(want[keep]))) keep++;
  return { keep, enqueue: want.slice(keep).map((item, i) => ({ at: keep + i, item })) };
}

/** Make the deck hold `wanted`. The first `enqueue` already drops what follows its place, so a trim is only for a shorter list. */
export async function syncHeld(port: DeckPort, wanted: DeckItem[], sent?: Map<string, string>): Promise<void> {
  const s = await port.state();
  const plan = heldPlan(s.held, wanted, undefined, sent);
  if (plan.enqueue.length === 0) {
    if (s.held.length > plan.keep) await port.act('trim', plan.keep);
    return;
  }
  for (const e of plan.enqueue) { await port.enqueue(e.item, e.at); sent?.set(e.item.id, JSON.stringify(e.item)); }
}
