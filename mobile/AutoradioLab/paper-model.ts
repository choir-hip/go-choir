
// Hoisted: a regex literal with a \p{...} class is compiled at each evaluation by Hermes (the device's engine), and these ran once per character;
// on the simulator that took paperFront 390 ms against a 100 ms turn. Compile once, here.
const LETTER_OR_DIGIT = /[\p{L}\p{N}]/u;
const UPPER = /\p{Lu}/u;
const MARKS = /\p{M}/gu;
// The paper's data, with no network and no layout: a story of the front list, the views (Front page, My edition,
// one per category) and My edition's matching. Ported from the Swift this replaced (Layout/PaperModel.swift,
// PaperViews.swift, EditionSetup.swift's EditionTopics); the failure list V1 to V10 is in paper.test.ts.
// Time arrives as an argument (epoch milliseconds); nothing here reads a clock.

export type Entity = { id: string; label: string };

export type Story = {
  route: string; title: string; dek: string; section: string; words: number; importance: number; sources: number;
  versions: number; updated: number | null; kind: string; author: string; entities: Entity[]; sections: string[];
};

export type LiveSection = { id: string; label: string };
export type Section = { id: string; label: string; count: number; current: boolean };

const isObj = (v: unknown): v is Record<string, unknown> => v !== null && typeof v === 'object' && !Array.isArray(v);
const str = (v: unknown): string => (typeof v === 'string' ? v : '');
const int = (v: unknown): number | null => {
  if (typeof v === 'number' && Number.isFinite(v)) return Math.trunc(v);
  if (typeof v === 'string' && /^-?\d+$/.test(v.trim())) return parseInt(v, 10);
  return null;
};

/** ISO 8601 with any number of fraction digits ("2026-11-18T12:19:13.769525189Z") to epoch milliseconds, or null. */
export function parseDate(s: unknown): number | null {
  if (typeof s !== 'string' || s === '') return null;
  const m = /^(\d{4})-(\d{2})-(\d{2})T(\d{2}):(\d{2}):(\d{2})(?:\.(\d+))?(Z|[+-]\d{2}:?\d{2})$/.exec(s.trim());
  if (!m) return null;
  const ms = m[7] ? parseInt((m[7] + '00').slice(0, 3), 10) : 0;
  let t = Date.UTC(+m[1], +m[2] - 1, +m[3], +m[4], +m[5], +m[6], ms);
  if (m[8] !== 'Z') {
    const sign = m[8][0] === '-' ? -1 : 1;
    const digits = m[8].slice(1).replace(':', '');
    t -= sign * (parseInt(digits.slice(0, 2), 10) * 60 + parseInt(digits.slice(2), 10)) * 60000;
  }
  return Number.isFinite(t) ? t : null;
}

/** The real dek is cut from sentences joined with a space before each stop; say it the way it is read. */
export function tidy(s: string): string {
  return s.replace(/\s+([.,;:!?])/g, '$1').replace(/([:;,])\./g, '$1').trim();
}

/** One story object of the front list. Unknown or missing keys are tolerated; no title or no route drops the story. */
export function parseStory(o: unknown): Story | null {
  if (!isObj(o)) return null;
  const title = str(o.title);
  const route = isObj(o.route) ? str(o.route.bare) : str(o.route);
  if (title === '' || route === '') return null;
  const entities: Entity[] = [];
  if (Array.isArray(o.entities)) {
    for (const e of o.entities) if (isObj(e) && typeof e.id === 'string' && str(e.label) !== '') entities.push({ id: e.id, label: str(e.label) });
  }
  return {
    route, title, dek: tidy(str(o.dek)), section: str(o.section), words: int(o.words) ?? 0, importance: int(o.importance) ?? 5,
    sources: int(o.source_count) ?? 0, versions: Math.max(1, int(o.version_count) ?? 1), updated: parseDate(o.updated_at),
    kind: str(o.kind), author: str(o.author), entities,
    sections: Array.isArray(o.sections) ? o.sections.filter((x): x is string => typeof x === 'string' && x !== '') : [],
  };
}

/** The stories of a front answer. Throws when the body is not the front list's object (a malformed body is an error, not an empty paper). */
export function parseList(body: unknown): Story[] {
  if (!isObj(body) || !Array.isArray(body.stories)) throw new Error('the front list is not an object with stories');
  const out: Story[] = [];
  for (const s of body.stories) { const p = parseStory(s); if (p) out.push(p); }
  return out;
}

export function isOpinion(s: Story): boolean { return s.kind.toLowerCase() === 'opinion' || s.section.toLowerCase() === 'opinion'; }

/** Importance, lifted by depth (sources) and decayed by age, in the manner of the web's ranking. */
export function weight(s: Story, now: number): number {
  const imp = Math.min(Math.max(s.importance, 1), 10) / 10;
  const depth = s.sources <= 2 ? 0.75 : 1 + 0.1 * Math.min(Math.log2(s.sources / 3), 3);
  let recency = 1;
  if (s.updated !== null) recency = Math.pow(0.5, Math.max(0, now - s.updated) / 1000 / (24 * 3600));
  const w = imp * depth * (0.6 + 0.4 * recency);
  return Number.isFinite(w) ? w : 0;
}

// ---- text helpers

export function slug(s: string): string {
  const t = Array.from(s.toLowerCase()).map((c) => (LETTER_OR_DIGIT.test(c) ? c : '-')).join('');
  return t.split('-').filter((x) => x !== '').join('-');
}

export function relative(d: number | null, now: number): string {
  if (d === null) return '';
  const s = Math.trunc((now - d) / 1000);
  if (s < 90) return 'just now';
  if (s < 3600) return `${Math.trunc(s / 60)} min ago`;
  if (s < 86400) return `${Math.trunc(s / 3600)} h ago`;
  return `${Math.trunc(s / 86400)} d ago`;
}

const DAYS = ['Sun', 'Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat'];
const MONTHS = ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec'];
/** "Wed 18 Nov", the reader's LOCAL date (the radio's masthead is local too; this was UTC and showed tomorrow's date in the evening in the Americas). */
export function dateLine(ms: number): string {
  const d = new Date(ms);
  return `${DAYS[d.getDay()]} ${d.getDate()} ${MONTHS[d.getMonth()]}`;
}

// ---- topics (the house list and the ways a reader says one)

export const HOUSE: { id: string; label: string }[] = [
  { id: 'world', label: 'World' }, { id: 'us', label: 'U.S.' }, { id: 'politics', label: 'Politics' }, { id: 'business', label: 'Business' },
  { id: 'tech_ai', label: 'Technology' }, { id: 'science', label: 'Science' }, { id: 'health', label: 'Health' }, { id: 'climate', label: 'Climate' },
  { id: 'culture', label: 'Culture' }, { id: 'sports', label: 'Sports' },
];

const FINER = ['Conflict', 'Elections', 'Trade', 'Security', 'Courts', 'Public health', 'Markets', 'Banks', 'Jobs', 'Prices', 'AI', 'Energy', 'Software',
  'Space', 'Physics', 'Biology', 'Medicine', 'Extreme weather', 'Nature', 'Film', 'Music', 'Books', 'Food', 'Religion', 'Media', 'Ideas'];

const ALIASES: Record<string, string> = {
  tech: 'Technology', technology: 'Technology', artificialintelligence: 'AI', machinelearning: 'AI', economy: 'Business', economics: 'Business',
  usnews: 'U.S.', america: 'U.S.', unitedstates: 'U.S.', war: 'Conflict', wars: 'Conflict', election: 'Elections', movies: 'Film', movie: 'Film',
  films: 'Film', cybersecurity: 'Security', weather: 'Extreme weather', environment: 'Climate', sport: 'Sports', 'world news': 'World', worldnews: 'World',
  arts: 'Culture', art: 'Culture',
};

/** Letters and digits only, lower case: "U.S." and "us" are one topic. */
const NOT_LETTER_OR_DIGIT = /[^\p{L}\p{N}]/gu;
const KEYS = new Map<string, string>();
/** Letters and digits of a name, lower case (a pure function asked again and again by sections and belongs: remembered, bounded). */
export function key(s: string): string {
  const hit = KEYS.get(s);
  if (hit !== undefined) return hit;
  const out = s.toLowerCase().replace(NOT_LETTER_OR_DIGIT, '');
  if (KEYS.size > 4000) KEYS.clear();
  KEYS.set(s, out);
  return out;
}

const MAX_TOPIC = 60;
const TRIM = ' .,;:!?"\'()-';
const trimSet = (s: string): string => {
  let a = 0, b = s.length;
  while (a < b && TRIM.includes(s[a])) a++;
  while (b > a && TRIM.includes(s[b - 1])) b--;
  return s.slice(a, b);
};

/** Typed or spoken words as one tidy topic, or null when there is nothing in them. */
export function topicTidy(raw: string): string | null {
  let s = raw.replace(/[\u0000-\u001f\u007f]/g, '').split(/\s+/).filter((x) => x !== '').join(' ');
  s = trimSet(s);
  if (!s.endsWith('.') && raw.toLowerCase().includes('u.s.') && s.toLowerCase() === 'u.s') s = 'U.S.';
  if (!LETTER_OR_DIGIT.test(s)) return null;
  const chars = Array.from(s);
  if (chars.length > MAX_TOPIC) {
    const cut = chars.slice(0, MAX_TOPIC).join('');
    const sp = cut.lastIndexOf(' ');
    s = sp >= MAX_TOPIC / 2 ? cut.slice(0, sp) : cut;
    s = trimSet(s);
  }
  if (s === '') return null;
  return s.charAt(0).toUpperCase() + s.slice(1);
}

function resolveTopic(raw: string, among: string[]): string | null {
  const t = topicTidy(raw);
  if (t === null) return null;
  const k = key(t);
  let hit = among.find((x) => key(x) === k);
  if (hit) return hit;
  if (k.endsWith('s')) { hit = among.find((x) => key(x) === k.slice(0, -1)); if (hit) return hit; }
  hit = among.find((x) => key(x) + 's' === k);
  if (hit) return hit;
  const a = ALIASES[k] ?? ALIASES[t.toLowerCase()];
  if (a) { hit = among.find((x) => key(x) === key(a)); if (hit) return hit; }
  return t;
}

/** The label a house section id carries; an unknown id is made readable ("tech_ai" style ids become words). */
export function labelForSectionId(id: string): string {
  const h = HOUSE.find((x) => x.id === id);
  if (h) return h.label;
  const words = id.replace(/_/g, ' ').replace(/-/g, ' ').trim();
  return words === '' ? '' : words.charAt(0).toUpperCase() + words.slice(1);
}

/** A section name a reader would say. */
export function display(raw: string): string {
  const t = raw.trim();
  if (t === '') return '';
  const h = HOUSE.find((x) => x.id === t);
  if (h) return h.label;
  if (t.includes(' ') || UPPER.test(t)) return t;
  return labelForSectionId(t);
}

// ---- the views

export const FRONT = 'front';
export const MINE = 'mine';
/** The Front page view shows the newest this many stories; My edition reads the whole list it was given. */
export const FRONT_LIMIT = 60;
/** What a front or category request asks for (the route's own cap). */
export const LIST_LIMIT = 100;

/** The live house sections of a front answer, in the server's order; none from an older server. */
export function parseLiveSections(body: unknown): LiveSection[] {
  const out: LiveSection[] = [];
  const seen = new Set<string>();
  if (!isObj(body) || !Array.isArray(body.sections)) return out;
  for (const e of body.sections) {
    if (!isObj(e) || typeof e.id !== 'string' || e.id === '' || seen.has(e.id)) continue;
    seen.add(e.id);
    const l = str(e.label);
    out.push({ id: e.id, label: l === '' ? display(e.id) : l });
  }
  return out;
}

/** Whether a story is in a category: the category is its primary section or one of its sections (compared by letters and digits). */
export function belongs(s: Story, sec: Section): boolean {
  const keys = new Set([key(sec.id), key(sec.label)].filter((k) => k !== ''));
  return [s.section, ...s.sections].some((x) => keys.has(key(x)));
}

/** The switcher's categories, with counts from the front list (never from the open view). */
export function sections(live: LiveSection[], stories: Story[]): Section[] {
  let out: Section[];
  if (live.length > 0) {
    out = live.map((l) => ({ id: l.id, label: l.label, count: 0, current: false }));
  } else {
    const raws: string[] = [];
    for (const s of stories) {
      if (isOpinion(s)) continue;
      const r = s.section.trim();
      if (r !== '' && !raws.some((x) => key(x) === key(r))) raws.push(r);
    }
    const order = HOUSE.map((h) => h.id);
    const idx = (a: string): number => { const i = order.findIndex((o) => key(o) === key(a)); return i < 0 ? Number.MAX_SAFE_INTEGER : i; };
    raws.sort((a, b) => (idx(a) !== idx(b) ? idx(a) - idx(b) : key(a) < key(b) ? -1 : key(a) > key(b) ? 1 : 0));
    out = raws.map((r) => ({ id: slug(r), label: display(r), count: 0, current: false }));
  }
  for (const o of out) o.count = stories.filter((s) => belongs(s, o)).length;
  return out;
}

/** The category a view id names: by id, else by letters and digits of id or label. Null for "front", "mine" and a name the list lacks. */
export function resolveView(view: string, secs: Section[]): Section | null {
  const e = secs.find((x) => x.id === view);
  if (e) return e;
  const k = key(view);
  return k === '' ? null : (secs.find((x) => key(x.id) === k || key(x.label) === k) ?? null);
}

export const isCategory = (view: string): boolean => view !== '' && view !== FRONT && view !== MINE;

// ---- My edition

const TERMS: Record<string, string[]> = {
  ai: ['artificial intelligence', 'machine learning', 'large language model', 'chatbot'], software: ['developers', 'open source'],
  security: ['cybersecurity', 'cyber security', 'cyberattack', 'data breach', 'hackers'], conflict: ['war', 'ceasefire', 'airstrike', 'fighting'],
  elections: ['election', 'voters', 'ballot', 'polls'], courts: ['court', 'judge', 'supreme court', 'lawsuit'],
  trade: ['tariffs', 'exports', 'imports', 'trade deal'], markets: ['stocks', 'stock market', 'wall street', 'investors', 'shares'],
  banks: ['bank', 'banking', 'central bank', 'lender'], jobs: ['employment', 'unemployment', 'layoffs', 'hiring'], prices: ['inflation', 'cost of living'],
  energy: ['oil', 'electricity', 'renewables', 'solar', 'nuclear'], space: ['nasa', 'spacex', 'rocket', 'satellite', 'astronaut', 'astronomy'],
  physics: ['quantum', 'particle'], biology: ['genome', 'dna', 'species'], medicine: ['drug', 'vaccine', 'clinical trial', 'hospital', 'cancer'],
  publichealth: ['epidemic', 'outbreak', 'vaccination', 'pandemic'], extremeweather: ['hurricane', 'flood', 'wildfire', 'heatwave', 'drought'],
  nature: ['wildlife', 'biodiversity', 'conservation'], film: ['movie', 'cinema', 'box office'], music: ['album', 'concert', 'singer'],
  books: ['novel', 'author', 'publisher', 'literature'], food: ['restaurant', 'recipe', 'chef'], religion: ['church', 'pope', 'mosque', 'synagogue'],
  media: ['newspaper', 'broadcaster', 'journalism', 'streaming'],
};
const STOP = new Set(['the', 'and', 'for', 'with', 'from', 'about', 'into', 'over', 'news', 'new']);

function fold(s: string): string {
  try { return s.normalize('NFD').replace(MARKS, ''); } catch { return s; }
}
/** Lower case, accents folded, letters and digits only, single spaces, a space at both ends so " word " finds a whole word. */
const PADDED = new Map<string, string>();
export function padded(s: string): string {
  const hit = PADDED.get(s);
  if (hit !== undefined) return hit;
  const mapped = Array.from(fold(s).toLowerCase()).map((c) => (LETTER_OR_DIGIT.test(c) ? c : ' ')).join('');
  const out = ' ' + mapped.split(' ').filter((x) => x !== '').join(' ') + ' ';
  // Pure, and the keyword lists are asked again for every story and every section: remember the answers (bounded).
  if (PADDED.size > 4000) PADDED.clear();
  PADDED.set(s, out);
  return out;
}

function has(hay: string, phrase: string): boolean {
  const p = padded(phrase).trim();
  if (Array.from(p).length < 2) return false;
  const forms = [p, p + 's', p + 'es'];
  if (p.endsWith('s') && p.length > 3) forms.push(p.slice(0, -1));
  return forms.some((f) => hay.includes(' ' + f + ' '));
}

type Pick = { label: string; section: Section | null; phrases: string[]; words: string[] };

function pick(raw: string, secs: Section[]): Pick | null {
  const label = topicTidy(raw);
  if (label === null) return null;
  const k = key(label);
  let catLabel: string | null = null;
  const house = HOUSE.find((h) => key(h.label) === k || key(h.id) === k);
  if (house) catLabel = house.label;
  else {
    const a = ALIASES[k] ?? ALIASES[label.toLowerCase()];
    if (a && HOUSE.some((h) => h.label === a)) catLabel = a;
  }
  if (catLabel !== null) {
    const cl = catLabel;
    const id = HOUSE.find((h) => h.label === cl)?.id ?? '';
    const sec = secs.find((x) => key(x.id) === key(id)) ?? secs.find((x) => key(x.label) === key(cl)) ?? null;
    return { label: cl, section: sec, phrases: sec === null ? [cl] : [], words: [] };
  }
  const main = padded(label).split(' ').filter((w) => w !== '' && Array.from(w).length >= 3 && !STOP.has(w));
  const extra = TERMS[k] ?? TERMS[key(resolveTopic(label, FINER) ?? label)] ?? [];
  const phrases = [label, ...extra].filter((p) => !STOP.has(padded(p).trim()));
  return { label, section: null, phrases, words: main.length >= 2 ? main : [] };
}

function matches(p: Pick, s: Story, hay: string): boolean {
  if (p.section && belongs(s, p.section)) return true;
  if (p.phrases.some((x) => has(hay, x))) return true;
  if (p.words.length >= 2 && p.words.every((w) => hay.includes(' ' + w + ' ') || hay.includes(' ' + w + 's '))) return true;
  return false;
}

export function fromLine(labels: string[]): string {
  const shown = labels.slice(0, 5).join(', ');
  return labels.length > 5 ? `${shown} and ${labels.length - 5} more` : shown;
}

export type Mine = { state: 'ready' | 'nopicks' | 'nomatch'; stories: Story[]; priority: Record<string, number>; from: string };

/** The edition for these picks over these stories (the front list, once per route, in the order given). */
export function buildMine(stories: Story[], picks: string[], secs: Section[]): Mine {
  const seenKeys = new Set<string>();
  const labels: string[] = [];
  const resolved: Pick[] = [];
  for (const r of picks) {
    const p = pick(r, secs);
    if (!p || seenKeys.has(key(p.label))) continue;
    seenKeys.add(key(p.label));
    labels.push(p.label); resolved.push(p);
  }
  if (resolved.length === 0) return { state: 'nopicks', stories: [], priority: {}, from: '' };
  const seen = new Set<string>();
  const out: Story[] = [];
  const priority: Record<string, number> = {};
  for (const s of stories) {
    if (seen.has(s.route)) continue;
    seen.add(s.route);
    const hay = padded([s.title, s.dek, ...s.entities.map((e) => e.label)].join(' . '));
    const n = resolved.filter((p) => matches(p, s, hay)).length;
    if (n > 0) { out.push(s); priority[s.route] = n; }
  }
  return { state: out.length === 0 ? 'nomatch' : 'ready', stories: out, priority, from: fromLine(labels) };
}
