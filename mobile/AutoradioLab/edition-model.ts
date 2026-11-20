
// Hoisted: a regex literal with a \p{...} class is compiled at each evaluation by Hermes (the device's engine), and these ran once per character;
// on the simulator that took paperFront 390 ms against a 100 ms turn. Compile once, here.
const LETTER_OR_DIGIT = /[\p{L}\p{N}]/u;
const CONTROL = /\p{Cc}/u;
// Edition setup: the reader picks the topics of their edition on a screen of chips, or says them. Ported from EditionSetup.swift (the logic;
// the screen is onboarding's). Pure data: the selection, the topic tree, how a spoken sentence becomes edits, what is said back, and how finishing
// always ends. Saving is edition-picks.ts's; nothing here needs the network (E6).
//
// WAYS THIS CAN FAIL (each has a test in edition.test.ts):
//  E1 The same topic twice: by tap, by typing, by speech, in another case, with a plural, or as a synonym ("tech"): one entry.
//  E2 Speech names a topic that is not a suggestion: it is added as the reader's own topic, as said, not dropped.
//  E3 A very long typed topic: cut at a word boundary at `MAX_TOPIC_LENGTH`; empty or punctuation-only input adds nothing.
//  E4 Nothing selected: valid. It is the general edition, and the edition says so. There is no minimum and no maximum count.
//  E5 A question or a long sentence spoken at the screen is not a topic: no edit, and a short hint.
//  E6 Saving needs no network: the selection is a local file written after every change; a failed write loses nothing in memory.
//  E7 Coming back to the step shows the saved selection; a corrupt or empty file starts empty and never throws.
//  E8 "Not X" or "no X" said aloud is not an exclusion list: it only takes X off the selection if it is there.
//  T1 A subtopic under several parents is one topic: picking it anywhere lights it everywhere; the saved list holds it once.
//  T2 Every subtopic has a parent, no label is a category and a subtopic at once (no cycles), and speech finds the one "AI".
//  T3 Picking a category does not pick its subtopics, and picking a subtopic does not pick its category; a category with only some
//     subtopics picked is "partial". Picking changes no row, only expanding does.
//  E9 Chips of the reader's own topics stay on screen after they are deselected until the step is left, so a mis-tap is undone.
//  C1 to C4 finishing always ends: the edition, a failure, or the deadline; the reader's topics are untouched.

export const MAX_TOPIC_LENGTH = 60;

/** The house sections (id, label): `internal/sectionlist/house.json`. */
export const HOUSE: { id: string; label: string }[] = [
  { id: 'world', label: 'World' }, { id: 'us', label: 'U.S.' }, { id: 'politics', label: 'Politics' }, { id: 'business', label: 'Business' }, { id: 'tech_ai', label: 'Technology' },
  { id: 'science', label: 'Science' }, { id: 'health', label: 'Health' }, { id: 'climate', label: 'Climate' }, { id: 'culture', label: 'Culture' }, { id: 'sports', label: 'Sports' },
];

/** The graph: each house section with the subtopics its own `question` (house.json) covers. A subtopic may sit under several sections; it is
 *  still ONE topic (one label, one selection state). */
export const TREE: { label: string; children: string[] }[] = [
  { label: 'World', children: ['Conflict', 'Elections', 'Trade', 'Security'] },
  { label: 'U.S.', children: ['Courts', 'Elections'] },
  { label: 'Politics', children: ['Elections', 'Conflict', 'Public health'] },
  { label: 'Business', children: ['Markets', 'Banks', 'Trade', 'Jobs', 'Prices', 'AI', 'Energy'] },
  { label: 'Technology', children: ['AI', 'Software', 'Security'] },
  { label: 'Science', children: ['Space', 'Physics', 'Biology'] },
  { label: 'Health', children: ['Medicine', 'Public health'] },
  { label: 'Climate', children: ['Energy', 'Extreme weather', 'Nature'] },
  { label: 'Culture', children: ['Film', 'Music', 'Books', 'Food', 'Religion', 'Media', 'Ideas'] },
  { label: 'Sports', children: [] },
];

/** Letters and digits only, lower case: "U.S." and "us" are one topic, "Public  Health" and "public health" too. */
export const key = (s: string): string => [...s.toLowerCase()].filter((c) => LETTER_OR_DIGIT.test(c)).join('');

/** The label a house section id carries; an unknown id is made readable ("tech_ai" style ids become words). */
export function labelForSectionID(id: string): string {
  const h = HOUSE.find((x) => x.id === id);
  if (h) return h.label;
  const words = id.replace(/_/g, ' ').replace(/-/g, ' ').trim();
  return words === '' ? '' : words.charAt(0).toUpperCase() + words.slice(1);
}

/** The top level: the tree, then any live section the house list does not have (no subtopics). */
export function categories(liveSections: string[] = []): { label: string; children: string[] }[] {
  const out = [...TREE];
  for (const id of liveSections) {
    const l = labelForSectionID(id);
    if (l !== '' && !out.some((c) => key(c.label) === key(l)) && !out.some((c) => c.children.some((k) => key(k) === key(l)))) out.push({ label: l, children: [] });
  }
  return out;
}
/** Every subtopic once, in first-seen order. */
export function finer(): string[] {
  const seen: string[] = [];
  for (const c of TREE) for (const k of c.children) if (!seen.includes(k)) seen.push(k);
  return seen;
}
/** The categories a subtopic sits under (one or more). */
export const parentsOf = (child: string): string[] => TREE.filter((c) => c.children.some((k) => key(k) === key(child))).map((c) => c.label);

/** Other ways of saying a suggestion (keys are `key()` of the phrase). */
const ALIASES: Record<string, string> = {
  tech: 'Technology', technology: 'Technology', artificialintelligence: 'AI', machinelearning: 'AI', economy: 'Business', economics: 'Business', usnews: 'U.S.', america: 'U.S.',
  unitedstates: 'U.S.', war: 'Conflict', wars: 'Conflict', election: 'Elections', movies: 'Film', movie: 'Film', films: 'Film', cybersecurity: 'Security', weather: 'Extreme weather',
  environment: 'Climate', sport: 'Sports', 'world news': 'World', worldnews: 'World', arts: 'Culture', art: 'Culture',
};

/** The chips to offer: the house sections, any live section not among them, then the finer topics. No duplicates. */
export function suggestions(liveSections: string[] = []): string[] {
  const out: string[] = [];
  const add = (l: string) => { if (l !== '' && !out.some((o) => key(o) === key(l))) out.push(l); };
  categories(liveSections).forEach((c) => add(c.label));
  finer().forEach(add);
  return out;
}

const TRIM = new Set([...' .,;:!?"\'()-']);
const trimChars = (s: string): string => { let a = 0, b = s.length; while (a < b && TRIM.has(s[a])) a++; while (b > a && TRIM.has(s[b - 1])) b--; return s.slice(a, b); };

/** Typed or spoken words as one tidy topic, or undefined when there is nothing in them (E3). */
export function tidy(raw: string): string | undefined {
  let s = [...raw].filter((c) => !CONTROL.test(c) || /\s/.test(c)).join('').split(/\s+/).filter((x) => x !== '').join(' ');
  s = trimChars(s);
  if (!s.endsWith('.') && raw.toLowerCase().includes('u.s.') && s.toLowerCase() === 'u.s') s = 'U.S.';
  if (![...s].some((c) => LETTER_OR_DIGIT.test(c))) return undefined;
  const cs = [...s];
  if (cs.length > MAX_TOPIC_LENGTH) {
    const cut = cs.slice(0, MAX_TOPIC_LENGTH).join('');
    const sp = cut.lastIndexOf(' ');
    s = sp >= 0 && [...cut.slice(0, sp)].length >= MAX_TOPIC_LENGTH / 2 ? cut.slice(0, sp) : cut;
    s = trimChars(s);
  }
  if (s === '') return undefined;
  return s.charAt(0).toUpperCase() + s.slice(1);
}

/** The topic a phrase names: a suggestion when it matches one (any case, plural, synonym), else the reader's own words (E1, E2). */
export function resolve(raw: string, among: string[]): string | undefined {
  const t = tidy(raw);
  if (t === undefined) return undefined;
  const k = key(t);
  let s = among.find((x) => key(x) === k);
  if (s) return s;
  if (k.endsWith('s')) { s = among.find((x) => key(x) === k.slice(0, -1)); if (s) return s; }
  s = among.find((x) => key(x) + 's' === k);
  if (s) return s;
  const a = ALIASES[k] ?? ALIASES[t.toLowerCase()];
  if (a) { s = among.find((x) => key(x) === key(a)); if (s) return s; }
  return t;
}

// ---- the selection ----

/** What the reader has chosen: any number of topics, in the order chosen. Zero is the general edition (E4). */
export class Selection {
  topics: string[] = [];
  constructor(topics: string[] = []) { for (const t of topics) if (!this.contains(t)) this.topics.push(t); }
  contains(t: string): boolean { return this.topics.some((x) => key(x) === key(t)); }
  /** Add by label or words. Returns the topic when it was newly added, undefined when it was empty or already there. */
  add(raw: string, among: string[]): string | undefined {
    const t = resolve(raw, among);
    if (t === undefined || this.contains(t)) return undefined;
    this.topics.push(t); return t;
  }
  /** Take a topic off. Returns it when it was there. */
  remove(raw: string, among: string[]): string | undefined {
    const t = resolve(raw, among);
    if (t === undefined) return undefined;
    const i = this.topics.findIndex((x) => key(x) === key(t));
    return i < 0 ? undefined : this.topics.splice(i, 1)[0];
  }
  /** A tap: selects it, or deselects it when it was selected. */
  toggle(label: string, among: string[]): void { if (this.contains(label)) this.remove(label, among); else this.add(label, among); }
  /** Typed words: commas and semicolons separate topics ("space, chess"); a topic may itself contain "and" ("Arts and crafts"). */
  addTyped(text: string, among: string[]): string[] {
    const added: string[] = [];
    for (const part of text.split(/[,;\n]/)) { const t = this.add(part, among); if (t !== undefined) added.push(t); }
    return added;
  }
  /** Speech applied to the selection (the same path the chips use, so a spoken topic and a tapped one are one thing). */
  apply(spoken: string, among: string[]): SpokenOutcome {
    const e = parseSpoken(spoken);
    const o: SpokenOutcome = { added: [], removed: [], finish: false, reply: '' };
    if (e.finish) { o.finish = true; o.reply = 'Okay.'; return o; }
    for (const a of e.adds) { const t = this.add(a, among); if (t !== undefined) o.added.push(t); }
    for (const r of e.removes) { const t = this.remove(r, among); if (t !== undefined) o.removed.push(t); }
    const parts: string[] = [];
    if (o.added.length) parts.push('Added ' + joined(o.added) + '.');
    if (o.removed.length) parts.push('Took off ' + joined(o.removed) + '.');
    if (parts.length === 0) parts.push(e.adds.length === 0 && e.removes.length === 0 ? 'Tell me a topic to add, like space or chess.' : 'You have that already.');
    o.reply = parts.join(' ');
    return o;
  }
}

// ---- speech ----

export type SpokenEdit = { adds: string[]; removes: string[]; finish: boolean };
export type SpokenOutcome = { added: string[]; removed: string[]; finish: boolean; reply: string };
export const isEmptyEdit = (e: SpokenEdit): boolean => e.adds.length === 0 && e.removes.length === 0 && !e.finish;

const FILLERS = ['please', 'can you', 'could you', 'would you', 'um', 'uh', 'okay', 'ok', 'so', 'and', 'also', 'yeah', 'yes', 'well', 'hey', 'now', 'then', 'the', 'a', 'an', 'my'];
const ADD_VERBS = ['i would like to hear about', "i'd like to hear about", 'i want to hear about', 'i want to know about', 'tell me about', 'give me', "i'm interested in",
  'i am interested in', 'i care about', "i'd like", 'i would like', 'i want', 'i like', 'i love', 'add', 'select', 'include', 'pick', 'choose', 'follow', 'put in', 'more', 'mostly', 'mainly', 'just', 'some'];
const REMOVE_VERBS = ['take off', 'take out', 'remove', 'drop', 'delete', 'deselect', 'unselect', 'unpick', 'untick'];
const FINISH = new Set(['continue', 'done', "i'm done", 'i am done', 'all done', "that's all", 'that is all', "that's it", 'that is it', 'next', 'go on', 'finished', "i'm finished", "that's enough", 'that is enough', "that's everything"]);
const QUESTION_LEAD = ['what', 'how', 'why', 'who', 'does', 'do', 'is', 'are', 'can', 'will', 'could', 'should', 'which', 'where', 'when'];
const NOTHING = new Set(['everything', 'anything', 'whatever', 'stuff', 'something', 'it', 'that', 'this', 'more', 'some', 'all', 'no', 'nope', 'nah', 'none', 'nothing', 'the', 'a', 'an', 'too', 'as well', 'please', 'thanks', 'thank you']);
const NEGATIVE_LEAD = ['not ', 'no ', 'without ', 'never '];
const MAX_ITEM_WORDS = 6;

function norm(s: string): string {
  let t = s.toLowerCase().replace(/’/g, "'");
  t = t.replace(/u\.s\./g, 'us');            // keep "u.s." intact as "us"
  const kept = [...t].map((c) => (LETTER_OR_DIGIT.test(c) || " '?,;&".includes(c) ? c : ' ')).join('');
  return kept.split(' ').filter((x) => x !== '').join(' ');
}
const trimSet = (s: string, set: string): string => { let a = 0, b = s.length; while (a < b && set.includes(s[a])) a++; while (b > a && set.includes(s[b - 1])) b--; return s.slice(a, b); };
/** The first prefix of `list` that starts `s` as a whole word, and what is left. */
function strip(s: string, list: string[]): [string, boolean] {
  for (const l of [...list].sort((a, b) => b.length - a.length)) {
    if (s === l || s.startsWith(l + ' ') || s.startsWith(l + ',')) return [trimSet(s.slice(l.length), ' ,'), true];
  }
  return [s, false];
}
function stripFillers(s: string): string { let p = s, again = true; while (again) [p, again] = strip(p, FILLERS); return p; }
function splitItems(s: string): string[] {
  let t = s;
  for (const sep of [' as well as ', ' and also ', ' plus ', ' & ', ';', ' and ']) t = t.split(sep).join(',');
  return t.split(',').map((x) => x.trim()).filter((x) => x !== '');
}

/** "add space and chess" -> adds [space, chess]. "drop chess" -> removes. "space" alone adds. A question adds nothing (E5). */
export function parseSpoken(words: string): SpokenEdit {
  const out: SpokenEdit = { adds: [], removes: [], finish: false };
  const n0 = norm(words);
  let n = stripFillers(n0);
  if (n === '') return out;
  if (FINISH.has(n.replace(/\?/g, ''))) { out.finish = true; return out; }
  const toks = n.split(' ');
  const [rest, removing] = strip(n, REMOVE_VERBS);
  const [afterAdd, adding] = removing ? [rest, false] : strip(n, ADD_VERBS);
  if (!removing && !adding && (words.includes('?') || (toks.length >= 2 && QUESTION_LEAD.includes(toks[0])))) return out;
  n = removing ? rest : adding ? afterAdd : n;
  n = stripFillers(n);
  for (let item of splitItems(n)) {
    item = stripFillers(item);
    let [a] = strip(item, ADD_VERBS); item = stripFillers(a);
    const [r, rem] = strip(item, REMOVE_VERBS);
    let isRemove = removing || rem;
    if (rem) item = r;
    for (const neg of NEGATIVE_LEAD) if (item.startsWith(neg)) { item = item.slice(neg.length); isRemove = true; break; }
    for (const tail of [' too', ' as well', ' please']) if (item.endsWith(tail)) item = item.slice(0, -tail.length);
    [a] = strip(item, ADD_VERBS); item = trimSet(a, ' ?');
    if (item === '' || NOTHING.has(item)) continue;
    if (item.split(' ').filter((x) => x !== '').length > MAX_ITEM_WORDS) continue;
    if (isRemove) { if (!out.removes.includes(item)) out.removes.push(item); } else if (!out.adds.includes(item)) out.adds.push(item);
  }
  return out;
}

// ---- words said and shown ----

export function joined(xs: string[]): string {
  switch (xs.length) {
    case 0: return '';
    case 1: return xs[0];
    case 2: return xs[0] + ' and ' + xs[1];
    default: return xs.slice(0, -1).join(', ') + ' and ' + xs[xs.length - 1];
  }
}
/** The two or three plain sentences that explain this step, said once at the top of the screen. */
export const EXPLAINER = 'Your edition is the paper arranged around what you care about. Tap a topic to pick it, open one to see what is inside, or tell me. You can change your edition at any time.';
export const VOICE_HINT = 'Say, for example, add space and chess.';
export const EMPTY_LINE = 'Nothing picked yet. You will get the general edition, and you can add topics whenever you like.';
export const SPOKEN_OPEN = 'Tap the topics you want, or tell me. Say, add space and chess.';
export const selectionLine = (topics: string[]): string => (topics.length === 0 ? EMPTY_LINE : (topics.length === 1 ? 'Your edition follows: ' : `Your edition follows ${topics.length} topics: `) + joined(topics) + '.');

// ---- the tree on screen ----

/** One line of the tree on screen. `kind`: category | sub | yoursHead | yours. */
export type TopicRow = { key: string; label: string; kind: string; selected: boolean; partial: boolean; expandable: boolean; expanded: boolean; note: string };

/** The rows to show. Collapsed: one per category. A subtopic appears under every parent that is expanded, with the same selection state
 *  everywhere. The reader's own topics come last under "Yours". Picking never changes which rows exist; only `expanded` does. */
export function treeRows(o: { liveSections?: string[]; expanded: Set<string>; selection: Selection; own: string[] }): TopicRow[] {
  const out: TopicRow[] = [];
  for (const c of categories(o.liveSections ?? [])) {
    const picked = c.children.filter((k) => o.selection.contains(k)).length;
    const isSel = o.selection.contains(c.label);
    const open = o.expanded.has(key(c.label)) && c.children.length > 0;
    out.push({ key: 'c/' + c.label, label: c.label, kind: 'category', selected: isSel, partial: !isSel && picked > 0, expandable: c.children.length > 0, expanded: open, note: picked > 0 ? `${picked} picked` : '' });
    if (open) for (const k of c.children) {
      const others = parentsOf(k).filter((p) => key(p) !== key(c.label));
      out.push({ key: `s/${c.label}/${k}`, label: k, kind: 'sub', selected: o.selection.contains(k), partial: false, expandable: false, expanded: false, note: others.length === 0 ? '' : 'also in ' + joined(others) });
    }
  }
  if (o.own.length) {
    out.push({ key: 'yours', label: 'Yours', kind: 'yoursHead', selected: false, partial: false, expandable: false, expanded: false, note: '' });
    for (const x of o.own) out.push({ key: 'y/' + x, label: x, kind: 'yours', selected: o.selection.contains(x), partial: false, expandable: false, expanded: false, note: '' });
  }
  return out;
}

// ---- the edition, written from the choices ----

export type Edition = { title: string; intro: string; writingLines: string[]; sections: { heading: string; lines: string[] }[]; failFirst?: boolean };
export const editionLines = (e: Edition): string[] => e.sections.flatMap((s) => [s.heading, ...s.lines]);
/** The edition on this phone, from the choices alone: deterministic, no network (E6). The computer's desk is not asked. */
export function localEdition(topics: string[]): Edition {
  return { title: 'My edition', intro: 'This is your edition. Change anything by asking.', writingLines: ['Putting your edition together'], sections: [{ heading: 'What I follow', lines: topics.length === 0 ? ['The whole paper, until I say otherwise.'] : topics }] };
}
export type Writer = (topics: string[], progress: (line: string) => void, signal: AbortSignal) => Promise<Edition>;
export const localWriter: Writer = async (topics, progress) => { progress('Putting your edition together'); return localEdition(topics); };

export type Outcome = { kind: 'written'; edition: Edition } | { kind: 'failed'; line: string } | { kind: 'timedOut' };
export const DEADLINE_SECONDS = 20;
export const FAILED_LINE = "I couldn't write your edition just now. Your topics are saved.";
export const TIMED_OUT_LINE = 'Writing your edition is taking too long. Your topics are saved.';

/** Finishing always ends: the edition, a failure, or the deadline (the writer's signal is aborted). */
export async function complete(topics: string[], writer: Writer, progress: (line: string) => void = () => undefined, deadlineSeconds = DEADLINE_SECONDS): Promise<Outcome> {
  const ac = new AbortController();
  let timer: ReturnType<typeof setTimeout> | undefined;
  const timeout = new Promise<Outcome>((res) => { timer = setTimeout(() => res({ kind: 'timedOut' }), Math.max(deadlineSeconds, 0) * 1000); });
  const run = writer(topics, progress, ac.signal).then<Outcome, Outcome>((edition) => ({ kind: 'written', edition }), (e) => (ac.signal.aborted ? { kind: 'timedOut' } : { kind: 'failed', line: FAILED_LINE }));
  const first = await Promise.race([run, timeout]);
  if (timer) clearTimeout(timer);
  ac.abort();
  return first;
}
/** The general edition: what "Continue anyway" writes. It needs nothing from the network and cannot fail. */
export const generalEdition = (): Edition => localEdition([]);
