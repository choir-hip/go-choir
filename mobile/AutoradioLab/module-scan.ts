// TEST SUPPORT (account-world.test.ts M3): finds every top-level binding of a module, so the test can say whether it is provably immutable or must be listed with the reason it holds no account data. Nothing in the app imports it.
//
// It is a small lexer, not a regex over lines: comments, strings, regular expressions and template literals are masked (same length, so offsets agree), brackets are counted, and a top-level statement is a declaration that starts at
// depth 0 in column 0. So an initialiser on the next line, a call (`const x = factory()`), an IIFE, a `let a, b`, a destructuring pattern and a class's static fields are all seen. Exact's toolchain does not give a bun test a parser
// (its TypeScript is the native 7.x: no JavaScript API; checked 2026-11-21), so this is the parser. Its limits are in the problem record: it does not follow a binding through an alias (`const m = TABLE; m.set(...)`), it
// sees only the module's own top level (not a binding made inside a function and stored on an exported object), and it trusts the masking of a regular expression by the previous token.
export type Binding = { name: string; kind: 'let' | 'var' | 'const' | 'static' | 'statement'; init: string };

const REGEX_AFTER = new Set(['', '(', ',', '=', ':', '[', '!', '&', '|', '?', '{', '}', ';', '+', '-', '*', '%', '<', '>', '~', '^']);

/** The source with comments blanked and the insides of strings, templates and regular expressions replaced by `_` (newlines kept). `${ }` code inside a template is kept. */
export function mask(src: string): string {
  const out: string[] = []; const n = src.length; let i = 0; let prev = '';           // prev: the last significant character
  const stack: ('tpl' | 'brace')[] = [];                                             // inside `${ ... }` of a template: the brace that closes it
  const blank = (c: string): string => (c === '\n' ? '\n' : '_');
  const tblank = (c: string): string => (c === '\n' ? '\n' : '\u0001');       // inside a template: a line may start in it, and that is not a statement
  const readString = (q: string): void => { out.push(q); i++; while (i < n && src[i] !== q) { if (src[i] === '\\') { out.push('_'); i++; } out.push(blank(src[i])); i++; } out.push(q); i++; prev = q; };
  const readTemplate = (resume = false): void => {
    if (!resume) { out.push('`'); i++; }
    while (i < n && src[i] !== '`') {
      if (src[i] === '\\') { out.push('\u0001'); i++; out.push(tblank(src[i] ?? '_')); i++; continue; }
      if (src[i] === '$' && src[i + 1] === '{') { out.push('${'); i += 2; stack.push('tpl'); return; }
      out.push(tblank(src[i])); i++;
    }
    out.push('`'); i++; prev = '`';
  };
  while (i < n) {
    const c = src[i];
    if (c === '/' && src[i + 1] === '/') { while (i < n && src[i] !== '\n') { out.push(' '); i++; } continue; }
    if (c === '/' && src[i + 1] === '*') { while (i < n && !(src[i] === '*' && src[i + 1] === '/')) { out.push(blank(src[i]) === '\n' ? '\n' : ' '); i++; } out.push('  '); i += 2; continue; }
    if (c === '"' || c === "'") { readString(c); continue; }
    if (c === '`') { readTemplate(); continue; }
    if (c === '/' ) {
      const word = !REGEX_AFTER.has(prev) && /(?:^|[^\w$.])(return|typeof|case|in|of|delete|void|throw|new|else|do|yield|await)$/.test(out.slice(-24).join('').trimEnd());
      if (REGEX_AFTER.has(prev) || word) {                                           // a regular expression literal
        out.push('/'); i++; let cls = false;
        while (i < n && (src[i] !== '/' || cls) && src[i] !== '\n') { if (src[i] === '\\') { out.push('_'); i++; } else if (src[i] === '[') cls = true; else if (src[i] === ']') cls = false; out.push('_'); i++; }
        out.push('/'); i++; while (i < n && /[a-z]/.test(src[i])) { out.push(src[i]); i++; } prev = '/'; continue;
      }
    }
    if (c === '{') stack.push('brace');
    if (c === '}') { const top = stack.pop(); if (top === 'tpl') { out.push('}'); i++; readTemplate(true); continue; } }
    out.push(c); if (!/\s/.test(c)) prev = c; i++;
  }
  return out.join('');
}

const DECL = /^(?:export\s+)?(?:default\s+)?(?:declare\s+)?(?:abstract\s+)?(?:async\s+)?(let|var|const|class|function\*?|enum|type|interface|import|namespace)\b/;

type Piece = { start: number; text: string };
/** The top-level statements: a statement starts at column 0 at bracket depth 0 on a line that is not blank or a closing bracket, and runs to the next one. */
function statements(m: string): { pieces: Piece[]; balanced: boolean } {
  const pieces: Piece[] = []; let depth = 0; let atLineStart = true; let startedAt = -1;
  for (let i = 0; i < m.length; i++) {
    const c = m[i];
    if (atLineStart && depth === 0 && !/[\s})\]\u0001]/.test(c)) { if (startedAt >= 0) pieces.push({ start: startedAt, text: m.slice(startedAt, i) }); startedAt = i; }
    atLineStart = c === '\n';
    if (c === '(' || c === '[' || c === '{') depth++; else if (c === ')' || c === ']' || c === '}') depth--;
  }
  if (startedAt >= 0) pieces.push({ start: startedAt, text: m.slice(startedAt) });
  return { pieces, balanced: depth === 0 };
}

/** Split `s` at depth-0 commas. */
function splitTop(s: string): string[] {
  const out: string[] = []; let depth = 0; let at = 0;
  for (let i = 0; i < s.length; i++) {
    const c = s[i];
    if ('([{'.includes(c)) depth++; else if (')]}'.includes(c)) depth--;
    else if (c === ',' && depth === 0 && /^\s*(?:[A-Za-z_$][\w$]*\s*(?:[=:,!]|$)|[{[])/.test(s.slice(i + 1))) { out.push(s.slice(at, i)); at = i + 1; }      // a comma inside a generic (`Record<string, unknown>`) does not begin a declarator
  }
  out.push(s.slice(at)); return out;
}
const idents = (pattern: string): string[] => [...pattern.matchAll(/[A-Za-z_$][\w$]*/g)].map((x) => x[0]).filter((x) => !['const', 'let', 'var'].includes(x));

/** Every top-level binding of the module: `let`/`var`/`const` (every declarator, destructured names too), `static` fields of a top-level class, and top-level statements that are not declarations (a call, an assignment). */
export function bindings(src: string): { found: Binding[]; balanced: boolean } {
  const m = mask(src); const { pieces, balanced } = statements(m); const found: Binding[] = [];
  for (const p of pieces) {
    const t = p.text.replace(/\s+$/, '');
    const d = DECL.exec(t);
    if (!d) {
      if (/^export\s*(\{|\*|type\b)/.test(t) || /^export\s+default\b/.test(t) && false) continue;
      if (t.trim() !== '' && !/^(['"`]use strict|\/)/.test(t)) found.push({ name: t.slice(0, 40).replace(/\s+/g, ' '), kind: 'statement', init: t });
      continue;
    }
    const kw = d[1];
    if (kw === 'let' || kw === 'var' || kw === 'const') {
      let body = t.slice(d[0].length); const semi = topSemi(body); if (semi >= 0) body = body.slice(0, semi);
      for (const decl of splitTop(body)) {
        const eq = topEquals(decl);
        const pattern = (eq < 0 ? decl : decl.slice(0, eq)).trim();
        const init = eq < 0 ? '' : decl.slice(eq + 1).trim();
        const names = /^[A-Za-z_$][\w$]*(\s*[:!][^=]*)?$/.test(pattern) ? [/^[A-Za-z_$][\w$]*/.exec(pattern)![0]] : idents(pattern.replace(/(?<=[}\]])\s*:.*$/, '').replace(/[\w$]+\s*:\s*([\w$]+)/g, '$1'));
        for (const name of names) found.push({ name, kind: kw as Binding['kind'], init });
      }
    } else if (kw === 'class') {
      const body = t.slice(t.indexOf('{') + 1);
      let depth = 0; let at = 0;
      for (let i = 0; i <= body.length; i++) {
        const c = body[i];
        if (c === '{' || c === '(' || c === '[') depth++; else if (c === '}' || c === ')' || c === ']') depth--;
        if (depth < 0) break;
        if (depth === 0 && (c === ';' || c === '\n' || i === body.length)) {
          const member = body.slice(at, i).trim(); at = i + 1;
          const s = /^(?:(?:public|private|protected|readonly|declare)\s+)*static\s+(?:(?:readonly)\s+)?(#?[A-Za-z_$][\w$]*)\s*(?![\w$]*\s*\()([=:;!?]|$)/.exec(member);
          if (s && !/^static\s+(?:async\s+)?(?:get|set)?\s*[\w$#]+\s*\(/.test(member)) found.push({ name: `${/class\s+([\w$]+)/.exec(t)?.[1] ?? '?'}.${s[1]}`, kind: 'static', init: member });
        }
      }
    }
  }
  return { found, balanced };
}
function topSemi(s: string): number { let d = 0; for (let i = 0; i < s.length; i++) { const c = s[i]; if ('([{'.includes(c)) d++; else if (')]}'.includes(c)) d--; else if (c === ';' && d === 0) return i; } return -1; }
function topEquals(s: string): number { let d = 0; for (let i = 0; i < s.length; i++) { const c = s[i]; if ('([{'.includes(c)) d++; else if (')]}'.includes(c)) d--; else if (c === '=' && d === 0 && s[i + 1] !== '>' && s[i + 1] !== '=' && !'=!<>'.includes(s[i - 1] ?? '')) return i; } return -1; }

/** Is the initialiser PROVABLY immutable by its text alone: a primitive, a regular expression, a function or arrow, a symbol, a frozen value, a template with nothing in it? Anything that builds an object, an array, a Map, a Set, a class instance
 * or a promise, or calls something, is not: it is listed (with its reason), whatever it looks like. */
export function provablyImmutable(init: string): boolean {
  const s = init.replace(/\s+as\s+const\s*$/, '').trim();
  if (/^(?:-?\d[\d_.]*n?|0x[\da-f]+|true|false|null|undefined)$/i.test(s)) return true;
  if (/^(['"])_*\1$/.test(s) || /^`[_\u0001]*`$/.test(s)) return true;                                  // a string, a template with no expression
  if (/^\/_*\/[a-z]*$/.test(s)) return true;                                                      // a regular expression literal
  if ((/^(?:async\s+)?(?:<[^>]*>)?\(/.test(s) || /^(?:async\s+)?[\w$]+\s*=>/.test(s)) && hasTopArrow(s) || /^(?:async\s+)?function\b/.test(s)) return true;      // an arrow or a function expression: the binding holds code (an IIFE has its arrow inside parentheses, not at the top)
  if (/^scoped\(/.test(s)) return true;                                                           // the world's own state (api.ts `scoped`): a new world starts with a fresh one
  if (/^Symbol\(/.test(s) || /^Object\.freeze\(/.test(s)) return true;
  if (/^new RegExp\(/.test(s) || /^new TextEncoder\(\)$|^new TextDecoder\(\)$/.test(s)) return true;      // a regular expression, an encoder: no state worth the name
  if (/^[\d\s_.*+\-/()e]+$/.test(s)) return true;                                                    // arithmetic of numbers
  return false;
}

/** A literal table: an array, object, Set or Map written out with its contents. (An empty one is state, not a table.) Safe only while nothing changes it, in its own file or in any file that imports it (`mutations`). */
export function isTable(init: string): boolean {
  const s = init.replace(/\s+as\s+const\s*$/, '').trim();
  if (/^(\[\s*\]|\{\s*\})$/.test(s)) return false;
  return /^(\[|\{|new (?:Set|Map)(?:<[^(]*>)?\(\[)/.test(s) && !/^new (?:Set|Map)(?:<[^(]*>)?\(\[\s*\]\)$/.test(s);
}

/** The places in `src` (a module that imports `name` from `from`, or declares it) where `name` is changed: a mutating call, an assignment to it or to a member of it, `delete`, `Object.assign(name, ...)`. `alias`: the namespace under which another module's export is reached (`X.name`). */
export function mutations(src: string, name: string, alias = ''): string[] {
  const m = mask(src); const out: string[] = [];
  const ref = `(?<![\\w$.])(?:${alias ? alias + '\\.' : ''})${name}`.replace('(?:)', '');
  const re = [
    new RegExp(`${ref}(?:\\.[\\w$]+|\\[[^\\]]*\\])*\\.(?:set|add|push|pop|shift|unshift|splice|delete|clear|sort|reverse|fill|copyWithin)\\(`, 'g'),
    new RegExp(`${ref}(?:\\.[\\w$]+|\\[[^\\]]*\\])*\\s*(?:=|\\+=|-=|\\*=|\\|\\|=|\\?\\?=|\\+\\+|--)(?![=>])`, 'g'),
    new RegExp(`delete\\s+${ref}\\b`, 'g'),
    new RegExp(`Object\\.(?:assign|defineProperty|setPrototypeOf)\\(\\s*${ref}\\b`, 'g'),
  ];
  for (const r of re) for (const x of m.matchAll(r)) {
    const at = x.index ?? 0; const line = m.lastIndexOf('\n', at) + 1; const head = m.slice(line, at);
    if (/^(?:export\s+)?(?:const|let|var)\s+$/.test(head) || /^\s*(?:export\s+)?(?:const|let|var)\s+[\w$]+\s*[:=]/.test(m.slice(line, at + x[0].length)) && new RegExp(`^(?:export\\s+)?(?:const|let|var)\\s+${name}\\b`).test(m.slice(line))) continue;      // its own declaration
    out.push(x[0].slice(0, 60));
  }
  return out;
}

/** Is there a `=>` at bracket depth 0 (an arrow function that IS the initialiser, not one passed to a call or called at once)? */
function hasTopArrow(s: string): boolean { let d = 0; for (let i = 0; i < s.length; i++) { const c = s[i]; if ('([{'.includes(c)) d++; else if (')]}'.includes(c)) d--; else if (c === '=' && s[i + 1] === '>' && d === 0) return true; } return false; }
