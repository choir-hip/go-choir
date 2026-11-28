// Shared reading of the .contract files for the two rule tests (contract-design-rules.test.ts, contract-typography-rules.test.ts). The rules lint contract
// text; nothing about them is native. They were Swift tests (DesignRulesTests, TypographyRulesTests) that no CI job ran, so a violation landed unseen
// (docs/problems/contract-rule-tests-are-swift-and-ci-does-not-run-them-2026-11-20.md).
import { readdirSync, readFileSync } from 'node:fs';
import { join } from 'node:path';

// The parser reads each line as text, not as Exact, and has blind spots a reader should know: a `//` inside a template literal (a backtick string) is taken as a
// comment when the quote count before it is even; a radius written as an expression (`(th.radiusCard * 1.7)`) is not a number and is not checked; a quoted face
// with spaces ("Helvetica Neue") is dropped from faces() because only role tokens and single-word hyphenated names count. Keep contracts clear of these.
export const lab = import.meta.dir;

export type Contract = { name: string; lines: string[] };
export function contracts(): Contract[] {
  return readdirSync(lab).filter((f) => f.endsWith('.contract')).sort().map((name) => ({ name, lines: readFileSync(join(lab, name), 'utf8').split('\n') }));
}

/** A line without its comment. A // inside a string (a URL) is not a comment. */
export function code(line: string): string {
  const i = line.indexOf('//');
  if (i < 0) return line;
  const before = line.slice(0, i);
  return (before.split('"').length - 1) % 2 === 0 ? before : line;
}

export const trim = (s: string): string => s.replace(/^[ \t]+|[ \t]+$/g, '');

export type El = { file: string; line: number; indent: number; text: string };

/** Every contract's elements: a line plus the attribute-only lines (a line that starts `name=`) that continue it. */
export function allElements(): El[][] {
  const attr = /^[a-z-]+=/;
  return contracts().map((f) => {
    const els: El[] = [];
    f.lines.forEach((raw, n) => {
      const c = code(raw); const t = trim(c);
      if (t === '') return;
      const indent = c.length - c.trimStart().length;
      if (attr.test(t) && els.length > 0) els[els.length - 1].text += ' ' + t;
      else els.push({ file: f.name, line: n + 1, indent, text: t });
    });
    return els;
  });
}

/** The numbers written as `border-radius=14`, `border-top-left-radius=20` on a line (an expression like `(th.radiusCard * 1.7)` is not a number). */
export function radii(l: string): number[] {
  return [...l.matchAll(/border(?:-[a-z]+)*-radius=([0-9]+)(?![0-9.])/g)].map((m) => Number(m[1]));
}

export function sizes(key: string, t: string): number[] {
  return [...t.matchAll(new RegExp('(?:^| )' + key + '=([0-9.]+)(?![0-9.%])', 'g'))].map((m) => Number(m[1]));
}

export function numberAfter(key: string, l: string): number | undefined {
  const i = l.indexOf(key);
  if (i < 0) return undefined;
  const m = /^[0-9.]*/.exec(l.slice(i + key.length))![0];
  return m === '' ? undefined : Number(m);
}

/** The value of `key=` in an element: a string, a template, a balanced expression or a bare word. */
export function attr(key: string, s: string): string | undefined {
  const m = new RegExp('(?<![A-Za-z-])' + key.replace(/[.*+?^${}()|[\]\\-]/g, '\\$&') + '=').exec(s);
  if (!m) return undefined;
  const chars = [...s.slice(m.index + m[0].length)];
  let i = 0, depth = 0; let quote: string | null = null;
  const q0 = chars[0];
  if (q0 === '"' || q0 === '`') { i = 1; while (i < chars.length && chars[i] !== q0) i++; return chars.slice(0, Math.min(i + 1, chars.length)).join(''); }
  while (i < chars.length) {
    const c = chars[i];
    if (quote) { if (c === quote) quote = null; }
    else if (c === '"' || c === '`') quote = c;
    else if (c === '(' || c === '[') depth++;
    else if (c === ')' || c === ']') depth--;
    else if (c === ' ' && depth === 0) break;
    i++;
  }
  return chars.slice(0, i).join('');
}

export const roleFaces = new Set(['th.fontHead', 'th.fontText', 'th.fontLabel', 'th.fontAction', 'th.fontMono']);
export function faces(s: string): string[] {
  const v = attr('font-family', s);
  if (v === undefined) return [];
  return (v.match(/th\.font[A-Za-z]+|t\.font[A-Za-z]+|"[^"]*"/g) ?? []).filter((f) => !f.startsWith('"') || (f.length > 3 && !f.includes(' ') && f.includes('-')));
}
export function ancestors(els: El[], k: number): El[] {
  let cur = els[k].indent; const out: El[] = [];
  for (let j = k - 1; j >= 0; j--) if (els[j].indent < cur) { cur = els[j].indent; out.push(els[j]); }
  return out;
}
export const isText = (e: El): boolean => e.text.startsWith('text ');
export function literal(e: El): string | undefined { return /^text\s+"([^"]*)"/.exec(e.text)?.[1]; }
/** A caps label: uppercase through text-transform, or a string that is already upper case. */
export function isCaps(e: El): boolean {
  return attr('text-transform', e.text) === '"uppercase"' || /"[A-Z][A-Z .·&']{3,}"/.test(e.text);
}

// ---- the clock and the resources (2026-11-28: a date passed after `with` was a new number every frame, so a source was asked every frame) ----
// A resource is asked again whenever any of its arguments, its `with` or its `when` changes. So no resource may name the clock: `performanceNow()` or `epochAtZero`, or a name
// that carries them, a `derive`, a `fn`, a `state` initialised from them or assigned them in an action (transitively). A declaration runs over the lines indented under its head.
const CLOCK = /performanceNow\s*\(\s*\)|\bepochAtZero\b/;
const mentions = (text: string, name: string): boolean => new RegExp(`(?<![.\\w])${name}(?!\\w)`).test(text);

/** Every resource declaration, whole (its head and the lines indented under it), that names the clock directly or through a name that carries it. A name is read in its own
 *  component or members block (the `members` blocks of every file are one scope: they are one root), and a top-level `fn` anywhere; a `fn`'s parameters shadow. */
export function clockInResources(files: Contract[]): string[] {
  type Block = { file: string; line: number; head: string; text: string; body: string[]; scope: string };
  const blocks: Block[] = [];
  for (const f of files) {
    const ls = f.lines.map((raw) => code(raw));
    let top = '';
    for (let i = 0; i < ls.length; i++) {
      if (trim(ls[i]) === '') continue;
      const ind = ls[i].length - ls[i].trimStart().length, body: string[] = []; let j = i + 1;
      for (; j < ls.length; j++) { if (trim(ls[j]) === '') continue; if (ls[j].length - ls[j].trimStart().length <= ind) break; body.push(trim(ls[j])); }
      const head = trim(ls[i]);
      if (ind === 0) top = /^members\s/.test(head) ? head : /^fn\s/.test(head) ? 'global' : `${f.name}:${head}`;
      blocks.push({ file: f.name, line: i + 1, head, text: [head, ...body].join(' '), body, scope: ind === 0 && /^fn\s/.test(head) ? 'global' : top });
    }
  }
  const tainted = new Set<string>();
  const key = (scope: string, n: string): string => `${scope}\u0000${n}`;
  const carries = (text: string, scope: string, locals: Set<string> = new Set(), shadow: Set<string> = new Set()): boolean =>
    CLOCK.test(text) || [...locals].some((n) => mentions(text, n)) || [...tainted].some((k) => { const [sc, n] = k.split('\u0000'); return (sc === scope || sc === 'global') && !shadow.has(n) && mentions(text, n); });
  for (let changed = true; changed;) {
    changed = false;
    const add = (scope: string, n: string): void => { if (!tainted.has(key(scope, n))) { tainted.add(key(scope, n)); changed = true; } };
    for (const b of blocks) {
      const d = /^(derive|fn|state)\s+(\w+)/.exec(b.head);
      if (d) {
        const params = new Set<string>(); const pm = /^fn\s+\w+\(([^)]*)\)/.exec(b.head);
        if (pm) for (const p of pm[1].split(',')) { const n = /^\s*(\w+)/.exec(p); if (n) params.add(n[1]); }
        const rest = b.text.slice(b.text.indexOf(d[2]) + d[2].length).replace(pm ? /^\([^)]*\)/ : /^$/, '');
        if (carries(rest, b.scope, new Set(), params)) add(b.scope, d[2]);
      }
      if (/^action\s+\w+/.test(b.head)) {
        const locals = new Set<string>(), mine = new Set<string>();
        for (const l of b.body) {
          const m = /^let\s+(\w+)\s*=\s*(.*)$/.exec(l);
          if (m) { mine.add(m[1]); if (carries(m[2], b.scope, locals)) locals.add(m[1]); continue; }
          const a = /^(\w+)\s*=(?!=)\s*(.*)$/.exec(l);
          if (a && !mine.has(a[1]) && carries(a[2], b.scope, locals)) add(b.scope, a[1]);
        }
      }
    }
  }
  const bad: string[] = [];
  for (const b of blocks) if (/^resource\s/.test(b.head) && carries(b.text, b.scope)) bad.push(`${b.file}:${b.line} a resource names the clock, or a value made from it: ${b.head.slice(0, 90)}`);
  return bad;
}
