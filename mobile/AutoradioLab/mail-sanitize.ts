// The allowlist sanitiser behind "Show original", ported from the Swift MailSanitizer.swift (written for the app review of 2026-11-11 and kept
// rule for rule). The server sends a message's HTML as it arrived (maild's `html_body` is never cleaned), so this is the only thing between a
// stranger's markup and the page. Nothing in the output is copied from the input except text and the values of a few attributes, and those are
// re-escaped. The iframe sandbox (`sandbox=""`, mail.contract) and the CSP meta are the second layer: a miss still runs and loads nothing.
//
// WAYS THIS CAN FAIL (each is a row of the hostile-input table in mail.test.ts):
//  1. A scheme the checker does not see: unquoted `href=javascript:`, entity-encoded (`jav&#x61;script:`, `&colon;`, `&Tab;`), whitespace or
//     control characters inside the scheme, upper case, `vbscript:`, `data:`. The value is entity-decoded once, stripped of every space and
//     control character, and must then START with `http://`, `https://`, `mailto:` or `#`; anything else loses the attribute. Fail-closed.
//  2. Event handlers (`onclick`, `ONLOAD`, `on\u0000click`): attributes are allowlisted by name, so none survives.
//  3. A tag that loads or runs (`base`, `meta http-equiv=refresh`, `link`, `script`, `iframe`, `object`, `embed`, `svg onload`, `math`, `form`,
//     `input`, `button formaction`, `img srcset`): only allowlisted tag names are written; script, style, svg, math, object, template, head are
//     dropped WITH their content.
//  4. CSS that fetches or runs (`url(`, `expression(`, `@import`, `behavior`, `-moz-binding`, backslash escapes, comments): a `style` attribute
//     containing any of them is dropped whole; `<style>` elements are dropped.
//  5. Markup hidden in odd syntax: comments and conditional comments, `<!DOCTYPE`, `<![CDATA[`, `<?xml`, NUL bytes, a tag with no closing `>`, a
//     `<script>` with no end tag, nested `<svg><svg>`.
//  6. Images: only `data:image/png|jpeg|gif|webp;base64,...` survive as `src`; remote images lose the `src` (the `alt` stays). Nothing is fetched.
//  7. Output markup breaking out of the document: text has `>` and `<` escaped, attribute values have `& " < >` escaped.
//  8. An unclosed `<head>` swallowing the message: an opening `<body>` ends the head skip.

export const CSP = "default-src 'none'; style-src 'unsafe-inline'; img-src data:";

const ALLOWED_TAGS = new Set([
  'a', 'abbr', 'address', 'article', 'aside', 'b', 'big', 'blockquote', 'br', 'caption', 'center', 'cite', 'code', 'col', 'colgroup',
  'dd', 'del', 'dfn', 'div', 'dl', 'dt', 'em', 'figcaption', 'figure', 'font', 'footer', 'h1', 'h2', 'h3', 'h4', 'h5', 'h6', 'header',
  'hr', 'i', 'img', 'ins', 'kbd', 'li', 'main', 'mark', 'nav', 'ol', 'p', 'pre', 'q', 's', 'samp', 'section', 'small', 'span',
  'strike', 'strong', 'sub', 'sup', 'table', 'tbody', 'td', 'tfoot', 'th', 'thead', 'tr', 'tt', 'u', 'ul', 'var', 'wbr',
]);
const VOID_TAGS = new Set(['br', 'hr', 'img', 'col', 'wbr']);
/** Their content is text up to the closing tag, never markup: skipped to `</name`. */
const RAW_TEXT = new Set(['script', 'style', 'iframe', 'noscript', 'noembed', 'noframes', 'xmp', 'textarea', 'title', 'plaintext']);
/** Dropped with everything inside (tracked by nesting depth). */
const DROP_WITH_CONTENT = new Set(['svg', 'math', 'object', 'applet', 'template', 'head', 'frameset', 'video', 'audio', 'canvas', 'map', 'select']);
const GLOBAL_ATTRS = new Set(['title', 'lang', 'dir', 'align', 'valign', 'width', 'height', 'colspan', 'rowspan', 'bgcolor', 'color', 'size', 'border', 'cellpadding', 'cellspacing', 'alt']);

type Tag = { name: string; closing: boolean; attrs: [string, string][] };
type Scalars = string[];

// ---- scanning (on an array of code points, so an astral character is one element)

const isLetter = (c: string): boolean => /^[A-Za-z]$/.test(c);
const isAlnum = (c: string): boolean => /^[A-Za-z0-9]$/.test(c);
const isSpace = (c: string): boolean => c === ' ' || c === '\t' || c === '\n' || c === '\r' || c === '\f';
const lower = (c: string): string => (c >= 'A' && c <= 'Z' ? String.fromCharCode(c.charCodeAt(0) + 32) : c);

function has(s: Scalars, i: number, lit: string): boolean {
  if (i + lit.length > s.length) return false;
  for (let k = 0; k < lit.length; k++) if (lower(s[i + k]) !== lit[k]) return false;
  return true;
}
function find(s: Scalars, lit: string, from: number): number | null {
  if (from > s.length) return null;
  for (let i = from; i + lit.length <= s.length; i++) if (has(s, i, lit)) return i;
  return null;
}
/** The index of `</name` (followed by a space, '/', '>' or the end) at or after `from`. */
function findEndTag(s: Scalars, name: string, from: number): number | null {
  let i = from;
  for (;;) {
    const k = find(s, '</' + name, i);
    if (k === null) return null;
    const after = k + 2 + name.length;
    if (after >= s.length || isSpace(s[after]) || s[after] === '/' || s[after] === '>') return k;
    i = k + 1;
  }
}

/** Parse the tag at `start` (s[start] is "<"). Null when it has no closing '>'. */
function parseTag(s: Scalars, start: number): [Tag, number] | null {
  const n = s.length;
  let i = start + 1;
  let closing = false;
  if (s[i] === '/') { closing = true; i += 1; }
  let name = '';
  while (i < n && !isSpace(s[i]) && s[i] !== '/' && s[i] !== '>') { name += lower(s[i]); i += 1; }
  const attrs: [string, string][] = [];
  for (;;) {
    while (i < n && (isSpace(s[i]) || s[i] === '/')) i += 1;
    if (i >= n) return null;
    if (s[i] === '>') { i += 1; break; }
    let an = '';
    while (i < n && !isSpace(s[i]) && s[i] !== '=' && s[i] !== '>' && s[i] !== '/') { an += lower(s[i]); i += 1; }
    if (an === '') { i += 1; continue; }                          // a stray '=': step over it
    while (i < n && isSpace(s[i])) i += 1;
    let value = '';
    if (i < n && s[i] === '=') {
      i += 1;
      while (i < n && isSpace(s[i])) i += 1;
      if (i >= n) return null;
      if (s[i] === '"' || s[i] === "'") {
        const q = s[i]; i += 1;
        while (i < n && s[i] !== q) { value += s[i]; i += 1; }
        if (i >= n) return null;
        i += 1;                                                   // the closing quote
      } else {
        while (i < n && !isSpace(s[i]) && s[i] !== '>') { value += s[i]; i += 1; }
      }
    }
    attrs.push([an, value]);
  }
  return [{ name, closing, attrs }, i];
}

// ---- entities (decoded once, as a browser does in an attribute value)

const NAMED: Record<string, number> = {
  amp: 0x26, lt: 0x3c, gt: 0x3e, quot: 0x22, apos: 0x27, nbsp: 0xa0, tab: 0x09, newline: 0x0a, colon: 0x3a, lpar: 0x28, rpar: 0x29, sol: 0x2f, bsol: 0x5c,
  period: 0x2e, comma: 0x2c, semi: 0x3b, excl: 0x21, num: 0x23, percnt: 0x25, equals: 0x3d, quest: 0x3f, commat: 0x40, lsqb: 0x5b, rsqb: 0x5d, lbrack: 0x5b,
  rbrack: 0x5d, lbrace: 0x7b, rbrace: 0x7d, lcub: 0x7b, rcub: 0x7d, lowbar: 0x5f, plus: 0x2b, ast: 0x2a, dollar: 0x24,
};

function digit(c: string, hex: boolean): number | null {
  const v = c.charCodeAt(0);
  if (c.length === 1 && v >= 0x30 && v <= 0x39) return v - 0x30;
  if (hex && c.length === 1 && v >= 0x61 && v <= 0x66) return v - 0x61 + 10;
  if (hex && c.length === 1 && v >= 0x41 && v <= 0x46) return v - 0x41 + 10;
  return null;
}

/** Decode character references once. A reference not understood is left as written, so it can only make a value fail the scheme check, never pass it. */
export function decodeAttr(str: string): string {
  const u = Array.from(str);
  let out = '';
  let i = 0;
  while (i < u.length) {
    if (u[i] !== '&') { out += u[i]; i += 1; continue; }
    let j = i + 1;
    if (j < u.length && u[j] === '#') {
      j += 1;
      let hex = false;
      if (j < u.length && (u[j] === 'x' || u[j] === 'X')) { hex = true; j += 1; }
      const start = j;
      let value = 0;
      while (j < u.length && j - start < 8) {
        const d = digit(u[j], hex);
        if (d === null) break;
        value = (value * (hex ? 16 : 10) + d) % 0x100000000;
        j += 1;
      }
      if (j === start) { out += u[i]; i += 1; continue; }
      if (j < u.length && u[j] === ';') j += 1;
      out += value === 0 || value > 0x10ffff || (value >= 0xd800 && value <= 0xdfff) ? '�' : String.fromCodePoint(value);
      i = j;
      continue;
    }
    const nameStart = j;
    while (j < u.length && j - nameStart < 12 && isAlnum(u[j])) j += 1;
    const name = u.slice(nameStart, j).join('').toLowerCase();
    if (j < u.length && u[j] === ';' && NAMED[name] !== undefined) { out += String.fromCodePoint(NAMED[name]); i = j + 1; continue; }
    if (['amp', 'lt', 'gt', 'quot', 'nbsp'].includes(name)) { out += String.fromCodePoint(NAMED[name]); i = j; continue; }
    out += u[i]; i += 1;
  }
  return out;
}

// ---- values

/** Characters a browser ignores or that hide a scheme: all of them are removed before the scheme is looked at. */
function ignorable(v: number): boolean {
  return v <= 0x20 || (v >= 0x7f && v <= 0xa0) || (v >= 0x200b && v <= 0x200f) || v === 0x2028 || v === 0x2029 || v === 0xfeff || v === 0xad;
}
function compact(s: string): string {
  let out = '';
  for (const ch of s) if (!ignorable(ch.codePointAt(0)!)) out += ch;
  return out;
}
function stripControls(s: string): string {
  let out = '';
  for (const ch of s) { const v = ch.codePointAt(0)!; if (v >= 0x20 && v !== 0x7f) out += ch; }
  return out;
}

/** http, https, mailto and in-page anchors only, judged after decoding entities and removing hidden characters. */
function safeLink(raw: string): string | null {
  const v = compact(decodeAttr(raw));
  const l = v.toLowerCase();
  return l.startsWith('http://') || l.startsWith('https://') || l.startsWith('mailto:') || l.startsWith('#') ? v : null;
}
function safeImage(raw: string): string | null {
  const v = compact(decodeAttr(raw));
  return /^data:image\/(png|jpe?g|gif|webp);base64,[A-Za-z0-9+/=]*$/i.test(v) ? v : null;
}
function safeStyle(raw: string): string | null {
  const v = stripControls(decodeAttr(raw));
  const l = compact(v).toLowerCase();
  for (const bad of ['url(', 'expression', '@import', 'behavior', 'binding', 'javascript:', 'vbscript:', 'image-set', 'image(', '\\', '/*', '<', '>', 'src(']) if (l.includes(bad)) return null;
  return v;
}
function escapeAttr(s: string): string {
  let out = '';
  for (const ch of s) {
    out += ch === '&' ? '&amp;' : ch === '"' ? '&quot;' : ch === '<' ? '&lt;' : ch === '>' ? '&gt;' : ch;
  }
  return out;
}

const SHOWN_TARGET = 90;

/** The opening tag as cleaned, and a link's target as the sender wrote it and the checker allowed it ('' when it allowed none): returned, not kept in a variable (the target is the sender's text, an account's). */
function open(tag: Tag): { html: string; target: string } {
  let target = '';
  const parts = ['<' + tag.name];
  const seen = new Set<string>();
  for (const [name, rawValue] of tag.attrs) {
    if (seen.has(name)) continue;
    let value: string | null;
    if (name === 'href' && tag.name === 'a') { const t = safeLink(rawValue); if (t !== null && target === '') target = t; value = null; }   // inert: a tap goes nowhere, the target is written after the link
    else if (name === 'src' && tag.name === 'img') value = safeImage(rawValue);
    else if (name === 'style') value = safeStyle(rawValue);
    else if (GLOBAL_ATTRS.has(name)) value = stripControls(decodeAttr(rawValue));
    else value = null;
    if (value === null) continue;
    seen.add(name);
    parts.push(`${name}="${escapeAttr(value)}"`);
  }
  if (tag.name === 'a') parts.push('rel="noopener noreferrer nofollow"');
  return { html: parts.join(' ') + '>', target };
}

/** The cleaned fragment: only allowlisted markup, no script, no remote load. */
export function clean(input: string): string {
  const s: Scalars = Array.from(input).filter((c) => c !== '\u0000');
  const n = s.length;
  let out = '';
  let i = 0;
  const links: string[] = [];                                      // the targets of the links now open, written after each one closes
  let skipName: string | null = null;                              // inside a dropWithContent element
  let skipDepth = 0;
  while (i < n) {
    const c = s[i];
    if (c !== '<') {
      if (skipName === null) out += c === '>' ? '&gt;' : c;
      i += 1;
      continue;
    }
    if (has(s, i, '<!--')) {                                       // a comment (conditional comments are comments too)
      const e = find(s, '-->', i + 4);
      i = e === null ? n : e + 3;
      continue;
    }
    if (i + 1 < n && (s[i + 1] === '!' || s[i + 1] === '?')) {     // doctype, CDATA, processing instruction
      let e = -1;
      for (let k = i; k < n; k++) if (s[k] === '>') { e = k; break; }
      i = e < 0 ? n : e + 1;
      continue;
    }
    const startsTag = i + 1 < n && (isLetter(s[i + 1]) || (s[i + 1] === '/' && i + 2 < n && isLetter(s[i + 2])));
    if (!startsTag) {
      if (skipName === null) out += '&lt;';
      i += 1;
      continue;
    }
    const parsed = parseTag(s, i);
    if (parsed === null) break;                                    // no closing '>': the rest is not a tag, drop it
    const [tag, next] = parsed;
    i = next;
    if (skipName !== null) {
      if (skipName === 'head' && !tag.closing && tag.name === 'body') { skipName = null; skipDepth = 0; continue; }   // an unclosed <head>
      if (tag.name === skipName) {
        if (tag.closing) { skipDepth -= 1; if (skipDepth <= 0) { skipName = null; skipDepth = 0; } } else skipDepth += 1;
      }
      continue;
    }
    if (tag.closing) {
      if (ALLOWED_TAGS.has(tag.name) && !VOID_TAGS.has(tag.name)) out += `</${tag.name}>`;
      if (tag.name === 'a' && links.length > 0) { const t = links.pop()!; if (t !== '') out += ` <small>(${escapeAttr(t.length > SHOWN_TARGET ? t.slice(0, SHOWN_TARGET) + '…' : t)})</small>`; }
      continue;
    }
    if (RAW_TEXT.has(tag.name)) {                                  // skip to the matching end tag (parsed, and ignored, next)
      const e = findEndTag(s, tag.name, i);
      i = e === null ? n : e;
      continue;
    }
    if (DROP_WITH_CONTENT.has(tag.name)) { skipName = tag.name; skipDepth = 1; continue; }
    if (ALLOWED_TAGS.has(tag.name)) { const o = open(tag); out += o.html; if (tag.name === 'a') links.push(o.target); }
    // any other tag (html, body, form, input, base, meta, link, unknowns): the tag goes, its text stays
  }
  return out;
}

/** The whole document "Show original" displays: the cleaned fragment under a CSP that refuses every load but inline style and data: images. */
/** The most HTML `clean` is given: it builds a code-point array and appends per character, and a page's turn is 100 ms. Over it the owner is told the message is
 * too large to show in full (never a blank page); the readable text of the message is on the page itself. */
export const CLEAN_MAX = 120_000;
export const TOO_LARGE_NOTE = 'This message is too large to show in full here. Its text is on the message page.';

export function sanitized(html: string): string {
  const t = html.length > CLEAN_MAX ? `<p>${TOO_LARGE_NOTE}</p>` : clean(html);
  return `<!doctype html><html><head><meta charset="utf-8"><meta http-equiv="Content-Security-Policy" content="${CSP}">\n` +
    '<meta name="viewport" content="width=device-width, initial-scale=1"><style>:root{color-scheme:light dark}body{font:16px -apple-system,sans-serif;margin:12px;overflow-wrap:anywhere}img{max-width:100%;height:auto}table{max-width:100%}</style></head>' +
    `<body>${t}</body></html>`;
}
