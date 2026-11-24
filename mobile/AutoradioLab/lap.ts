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
/** What a lap line may say of a page: its KIND, never its address. An address can carry a document id, a mail id, a file name or a search query, and the unified log is on in every build. Only these words are ever
 * written; anything else is `other`. */
const KINDS: Record<string, string> = {
  radio: 'radio', notebook: 'notebook', paper: 'paper', pub: 'reader', texture: 'reader', textures: 'textures', file: 'files', mail: 'mail', mailbox: 'mail', activity: 'activity', settings: 'settings',
  lab: 'lab', imported: 'imported', doc: 'imported', conversation: 'conversation', podcasts: 'podcasts', podcast: 'podcasts', onboarding: 'onboarding', launcher: 'launcher', takes: 'takes', pages: 'pages',
};
export function lapKind(address: string): string {
  const i = address.indexOf(':');
  const scheme = i < 0 ? address : address.slice(0, i);
  return Object.prototype.hasOwnProperty.call(KINDS, scheme) ? KINDS[scheme] : 'other';
}
/** A request's route pattern: the first three path segments (`/api/email/messages`), the rest and any query as `/*`. An id or a file name is always past the third segment of the routes the app calls. */
export function lapRoute(path: string): string {
  const parts = path.split('?')[0].split('/').filter((p) => p !== '');
  const known = parts.slice(0, 3).map((p) => (/^[a-z][a-z0-9_-]{0,31}$/.test(p) ? p : '*'));
  return '/' + known.join('/') + (parts.length > 3 ? '/*' : '');
}
/** A mark on the way to a page's first draw (docs/reference/exact/testing-and-debugging.md, "Reading the laps of a first open"): `lap: open <stage> <kind>`. The kind is the page's kind (reader, mail, files, paper...), never
 * its address. Read by `mobile/scripts/laps.sh`. */
export const lapOpen = (stage: string, address: string): void => lap(`open ${stage} ${lapKind(address)}`);
/** A request's mark: `lap: net <what> <route pattern>`; the route pattern, never the path's ids or a query. */
export const lapNet = (what: string, path: string): void => lap(`net ${what} ${lapRoute(path)}`);
