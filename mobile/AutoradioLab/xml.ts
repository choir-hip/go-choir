// A small, tolerant XML reader for the podcast feed. Exact's runtime has no DOMParser and no XML parser (EXACT.md, "What Exact's shared layer can
// carry": "No XML parser"), so this reads the one shape we need: elements with attributes, text, CDATA, comments, processing instructions and a
// DOCTYPE (skipped, never expanded: no external entity is ever read). It reports events in document order, with names as written (`itunes:duration`).
//
// Ways this can fail, and what it does (a feed is written by strangers):
//  - text that is not XML at all (HTML, an empty body, a JSON error): no events; the caller says "not a feed".
//  - an unclosed element at the end, an end tag with no start, an end tag that skips open children: the document is read as far as it goes; a skipped
//    child is closed first; a stray end tag is ignored. (Apple's XMLParser stopped at the first error; a feed that is nearly well formed is better read.)
//  - an entity that is not defined (`&nbsp;` in a feed that never declared it): read as the HTML name when we know it, else left as written.
//  - a deliberately huge feed: the handler returns `false` from `start` or `end` to stop reading (the caller keeps the newest 300 episodes).

export type Attrs = Record<string, string>;

export type XMLHandler = {
  /** An element opens. Return false to stop reading. */
  start?(name: string, attrs: Attrs): boolean | void;
  /** Characters of the element that is open (entities read). */
  text?(s: string): void;
  /** A CDATA section's characters, as written. */
  cdata?(s: string): void;
  /** An element closes (also for `<x/>`). Return false to stop reading. */
  end?(name: string): boolean | void;
};

const NAMED: Record<string, string> = {
  amp: '&', lt: '<', gt: '>', quot: '"', apos: "'", nbsp: ' ', ndash: '–', mdash: '—', hellip: '…', rsquo: '’', lsquo: '‘', ldquo: '“', rdquo: '”',
  copy: '©', reg: '®', trade: '™', eacute: 'é', egrave: 'è', agrave: 'à', uuml: 'ü', ouml: 'ö', auml: 'ä', ntilde: 'ñ', bull: '•', middot: '·',
};

/** `&amp;`, `&#38;`, `&#x26;` and the names above are read; anything else stays as written. */
export function decodeEntities(s: string): string {
  if (s.indexOf('&') < 0) return s;
  let out = '';
  let i = 0;
  while (i < s.length) {
    const ch = s[i];
    if (ch === '&') {
      const semi = s.indexOf(';', i + 1);
      if (semi > i && semi - i <= 12) {
        const body = s.slice(i + 1, semi);
        let rep: string | undefined;
        if (body[0] === '#') {
          const hex = body[1] === 'x' || body[1] === 'X';
          const n = parseInt(body.slice(hex ? 2 : 1), hex ? 16 : 10);
          if (Number.isFinite(n) && n > 0 && n <= 0x10ffff && !(n >= 0xd800 && n <= 0xdfff)) rep = String.fromCodePoint(n);
        } else rep = NAMED[body];
        if (rep !== undefined) { out += rep; i = semi + 1; continue; }
      }
    }
    out += ch;
    i++;
  }
  return out;
}

function isSpace(c: string): boolean { return c === ' ' || c === '\n' || c === '\t' || c === '\r'; }

/** The attributes of a start tag's inside (`name a="1" b='2'`), after the name. A value with no quotes is read up to a space. */
function readAttrs(s: string, from: number): Attrs {
  const out: Attrs = {};
  let i = from;
  const n = s.length;
  while (i < n) {
    while (i < n && isSpace(s[i])) i++;
    if (i >= n) break;
    let j = i;
    while (j < n && s[j] !== '=' && !isSpace(s[j])) j++;
    const key = s.slice(i, j);
    i = j;
    while (i < n && isSpace(s[i])) i++;
    if (s[i] !== '=') { if (key) out[key] = ''; continue; }
    i++;
    while (i < n && isSpace(s[i])) i++;
    let value: string;
    if (s[i] === '"' || s[i] === "'") {
      const q = s[i];
      const e = s.indexOf(q, i + 1);
      if (e < 0) { value = s.slice(i + 1); i = n; } else { value = s.slice(i + 1, e); i = e + 1; }
    } else {
      let e = i;
      while (e < n && !isSpace(s[e])) e++;
      value = s.slice(i, e); i = e;
    }
    if (key) out[key] = decodeEntities(value);
  }
  return out;
}

/** Reads `xml` and reports events. Returns how many elements opened (0: not XML at all). */
export function readXML(xml: string, h: XMLHandler): number {
  const n = xml.length;
  const open: string[] = [];
  let opened = 0;
  let i = 0;
  const textTo = (to: number): void => { if (to > i && h.text && open.length > 0) h.text(decodeEntities(xml.slice(i, to))); };
  while (i < n) {
    const lt = xml.indexOf('<', i);
    if (lt < 0) { textTo(n); break; }
    textTo(lt);
    i = lt;
    if (xml.startsWith('<!--', i)) { const e = xml.indexOf('-->', i + 4); if (e < 0) break; i = e + 3; continue; }
    if (xml.startsWith('<![CDATA[', i)) {
      const e = xml.indexOf(']]>', i + 9);
      const body = xml.slice(i + 9, e < 0 ? n : e);
      if (h.cdata && open.length > 0) h.cdata(body);
      if (e < 0) break;
      i = e + 3; continue;
    }
    if (xml.startsWith('<?', i)) { const e = xml.indexOf('?>', i + 2); if (e < 0) break; i = e + 2; continue; }
    if (xml.startsWith('<!', i)) {
      // A DOCTYPE may hold a bracketed subset with `>` inside it.
      let depth = 0, j = i + 2;
      for (; j < n; j++) { const c = xml[j]; if (c === '[') depth++; else if (c === ']') depth--; else if (c === '>' && depth <= 0) break; }
      i = j + 1; continue;
    }
    if (xml[i + 1] === '/') {
      const e = xml.indexOf('>', i + 2);
      if (e < 0) break;
      const name = xml.slice(i + 2, e).trim();
      const at = open.lastIndexOf(name);
      i = e + 1;
      if (at < 0) continue;
      while (open.length > at) {
        const closing = open.pop() as string;
        if (h.end && h.end(closing) === false) return opened;
      }
      continue;
    }
    // A start tag. Find its end, skipping `>` inside a quoted attribute value.
    let j = i + 1;
    let quote = '';
    for (; j < n; j++) {
      const c = xml[j];
      if (quote) { if (c === quote) quote = ''; } else if (c === '"' || c === "'") quote = c; else if (c === '>') break;
    }
    if (j >= n) break;
    let inner = xml.slice(i + 1, j);
    i = j + 1;
    const selfClosing = inner.endsWith('/');
    if (selfClosing) inner = inner.slice(0, -1);
    let k = 0;
    while (k < inner.length && !isSpace(inner[k])) k++;
    const name = inner.slice(0, k);
    if (!name || !/^[A-Za-z_:]/.test(name)) { continue; }
    opened++;
    open.push(name);
    if (h.start && h.start(name, readAttrs(inner, k)) === false) return opened;
    if (selfClosing) { open.pop(); if (h.end && h.end(name) === false) return opened; }
  }
  while (open.length > 0) { const closing = open.pop() as string; if (h.end && h.end(closing) === false) return opened; }
  return opened;
}
