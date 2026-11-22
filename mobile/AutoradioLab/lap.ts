// The instrument for a turn that overran. Exact refuses every clock in shared code (no `Date.now()`, no `performance.now()`: docs/reference.md:2774), so a stretch cannot be timed here; and its own
// overrun message names only the mutation and its total ("`radioWrite` took 152.6 ms"), not which stretch. A `lap(label)` at each place where the turn is ended writes one line to the shared console; the
// platform log stamps every console line with its own time, and the proof (scripts/proof-real.sh, "laps over 50 ms") prints each gap between two lap lines over 50 ms with the labels at both ends.
// On the phone: the same lines are in the device log. It costs one console line per yield point (a few per story, one per 64 blocks in a long one).
/** Ends the turn (Exact gives one about 100 ms of CPU; Bun has no scheduler) and marks where the next stretch begins. */
export async function lapYield(label: string): Promise<void> {
  const sc = (globalThis as { scheduler?: { yield?: () => Promise<void> } }).scheduler;
  if (sc?.yield) { console.log(`lap: ${label}`); await sc.yield(); }
}
/** A mark with no yield (a stretch's end that is not a turn's end). */
export const lap = (label: string): void => console.log(`lap: ${label}`);
