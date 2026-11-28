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
// A review-forcing pin, not an analyser (exact-facts.test.ts H3): every line that reads the clock is listed there, and no resource declaration names a name that carries it.
const CLOCK = /performanceNow\s*\(|\bepochAtZero\b/;

/** The contracts of the lab and of its fixture app `ago/`. */
export const withFixtures = (): Contract[] => [...contracts(), ...readdirSync(join(lab, 'ago')).filter((f) => f.endsWith('.contract')).sort().map((f) => ({ name: `ago/${f}`, lines: readFileSync(join(lab, 'ago', f), 'utf8').split('\n') }))];

/** Every line of every non-test contract (the lab and its subfolders) that reads the clock, as `file: trimmed line`, comments stripped. */
export function clockLines(files: Contract[]): string[] {
  const out: string[] = [];
  for (const f of files) if (!f.name.endsWith('.test.contract')) for (const raw of f.lines) { const c = trim(code(raw)); if (CLOCK.test(c)) out.push(`${f.name}: ${c}`); }
  return out.sort();
}

/** Each `resource` declaration: from its keyword to the next line that is not indented more than it, string literals and comments stripped, as `file:line` and text. */
export function resourceDecls(files: Contract[]): { at: string; text: string }[] {
  const out: { at: string; text: string }[] = [];
  for (const f of files) {
    const ls = f.lines.map((raw) => code(raw));
    ls.forEach((l, i) => {
      if (!/^\s*resource\s/.test(l)) return;
      const ind = l.length - l.trimStart().length; let text = l;
      for (let j = i + 1; j < ls.length && trim(ls[j]) !== '' && ls[j].length - ls[j].trimStart().length > ind; j++) text += ' ' + ls[j];
      out.push({ at: `${f.name}:${i + 1}`, text: text.replace(/"(?:[^"\\]|\\.)*"|`[^`]*`/g, '""') });
    });
  }
  return out;
}

/** The resource declarations that name one of `names` (a whole word, not a field of something else). */
export function resourcesNaming(files: Contract[], names: string[]): string[] {
  const bad: string[] = [];
  for (const d of resourceDecls(files)) for (const n of names) if (new RegExp(`(?<![.\\w])${n}(?!\\w)`).test(d.text)) bad.push(`${d.at} names ${n}`);
  return bad;
}
