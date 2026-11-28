// A control whose native op the platform does not answer is not offered (owner's conservative default, 2026-11-28). The failure list, written before the change:
//  K1 no native name is added (the interface has no `nativeAnswers`); the fact is the table in native-answers.ts: Android has a row, any other platform (ios, macos, web, "") answers everything, so nothing changes there.
//  K2 the Android row equals Kotlin's `when` cases in `AutoradioModule.kt` exactly, and none of the four unanswered ops is in it; every iPhone interface name has a Swift handler (so "all" is true there).
//  K3 shared code decides "answered" in ONE place (`opAnswered`, from the table by `hpg.platform`): no platform name, no `noNative`, no `if android` in the derives.
//  K4 the speed row of the Autoradio page, the bar's microphone button, its Cancel, its Send, the placeholder and the line above the field, the mail page's "Summarise this thread" and the Texture list's "Make your edition"
//     are each drawn only when their op is answered (or, for Send, when the text is an address, which shared code opens).
//  K5 a typed question that nothing answers is never cleared and never sent, and signed out it does not open the sign-in sheet for it; an address still opens in place.
//  K6 an answered op leaves the contracts as they were: the branch that sends is the one the iPhone ran (S2/S3 of bar-url.test.ts still hold).
import { describe, expect, test } from 'bun:test';
import { readdirSync, readFileSync } from 'node:fs';
import { ANDROID_ANSWERS, nativeOps } from './native-answers.ts';

const read = (p: string): string => readFileSync(new URL(p, import.meta.url), 'utf8');
const spine = read('./spine.contract');
const bar = read('./bar.contract');
const radio = read('./radio.contract');
const mail = read('./mail.contract');
const textures = read('./textures.contract');
const app = read('./app.contract');
const kt = read('../exact-package/android/AutoradioModule.kt');
const iface = JSON.parse(read('../exact-package/exact.json'));

/** The ops of a Kotlin `later` `when` block: every string label before a `->`, in any layout (several to a branch, a branch over several lines, comments). Fails loudly when it cannot find the block, and cross-checks the count against a looser scan. */
export const parseWhenCases = (src: string): string[] => {
  const start = src.indexOf('override fun later(');
  if (start < 0) throw new Error('ktCases: no `override fun later(` in the Kotlin source');
  const w = src.indexOf('when (name)', start);
  if (w < 0) throw new Error('ktCases: no `when (name)` after `later(`');
  const open = src.indexOf('{', w);
  if (open < 0) throw new Error('ktCases: the `when (name)` block has no `{`');
  let nest = 0, end = -1;
  for (let k = open; k < src.length; k++) {                       // the block's own braces; string literals here hold no braces
    if (src[k] === '{') nest++;
    else if (src[k] === '}' && --nest === 0) { end = k; break; }
  }
  if (end < 0) throw new Error('ktCases: the `when (name)` block is not closed');
  const body = src.slice(open + 1, end).replace(/\/\*[\s\S]*?\*\//g, ' ').replace(/\/\/[^\n]*/g, ' ');
  // Tokenise the block, keeping only its own branches (brace and paren depth 0): a string at depth 0 is a candidate label, and a `->` at depth 0 makes the candidates since the last branch its labels.
  const out: string[] = [];
  let depth = 0, cands: string[] = [];
  for (let k = 0; k < body.length; k++) {
    const c = body[k];
    if (c === '"') {
      let e = k + 1;
      while (e < body.length && body[e] !== '"') e += body[e] === '\\' ? 2 : 1;
      if (depth === 0) cands.push(body.slice(k + 1, e));
      k = e;
    } else if (c === '{' || c === '(') depth++;
    else if (c === '}' || c === ')') depth--;
    else if (c === '-' && body[k + 1] === '>' && depth === 0) {
      for (const n of cands) { if (!/^[A-Za-z0-9_]+$/.test(n)) throw new Error(`ktCases: label "${n}" is not a plain op name`); out.push(n); }
      cands = [];
      k++;
    } else if (depth === 0 && c === '\n' && cands.length && !/,\s*$/.test(body.slice(0, k))) cands = [];      // a string not followed by `->` or a comma is not a label (e.g. the `else` branch's body)
  }
  // An independent, looser count: line by line, every string literal before a `->` on a line that opens at brace depth 0, plus strings on the wrapped label lines above it.
  let loose = 0, d = 0;
  const lines = body.split('\n');
  for (let n = 0; n < lines.length; n++) {
    const line = lines[n];
    if (d === 0 && line.includes('->')) {
      let labels = [...line.slice(0, line.indexOf('->')).matchAll(/"[^"]*"/g)].length;
      for (let b = n - 1; b >= 0 && /,\s*$/.test(lines[b]) && lines[b].includes('"'); b--) labels += [...lines[b].matchAll(/"[^"]*"/g)].length;
      loose += labels;
    }
    for (const ch of line.replace(/"[^"]*"/g, '')) { if (ch === '{' || ch === '(') d++; else if (ch === '}' || ch === ')') d--; }
  }
  if (out.length !== loose) throw new Error(`ktCases: parsed ${out.length} case labels but a looser scan finds ${loose}: the parser missed or invented one`);
  if (out.length === 0) throw new Error('ktCases: no case labels found');
  return out.sort();
};
const ktCases = (): string[] => parseWhenCases(kt);

describe('a control is offered only when its op is answered', () => {
  test('K1 no native name; one table; a platform with no row answers everything', () => {
    expect(iface.names.some((x: { source?: string }) => x.source === 'nativeAnswers')).toBe(false);
    const a = nativeOps('android');
    expect(a.all).toBe(false);
    for (const never of ['radioSpeed', 'choirAsk', 'choirHandsFreeToggle', 'choirTalkCancel']) expect(a.ops).not.toContain(never);
    expect(spine).toContain('fn opAnswered(all: bool, ops: string[], name: string): bool = all or ops.indexOf(name) >= 0');
    expect(spine).toContain('resource nans = nativeOps(hpg.platform) as shape NativeOps else empty(known=false, all=false, ops=[])');
    expect(read('./app.ts')).toContain("nativeOps: ([platform]) => nativeOps(String(platform ?? '')),");
  });
  test('K2 the Android row is the Kotlin when cases; the iPhone answers every interface name', () => {
    const cases = ktCases();
    expect(cases.length).toBeGreaterThan(8);
    expect([...ANDROID_ANSWERS].sort()).toEqual(cases);
    const declared = new Set(iface.names.map((x: { source?: string; send?: string }) => x.source ?? x.send));
    const own = ['nativeModelFacts', 'nativeModelEnsure', 'nativeModelRemove', 'nativeModelProof'];
    for (const n of cases) expect(declared.has(n) || own.includes(n), n).toBe(true);
    const swift = readdirSync(new URL('../exact-package/apple/', import.meta.url)).filter((f) => f.endsWith('.swift')).map((f) => read('../exact-package/apple/' + f)).join('\n');
    for (const n of declared) expect(swift.includes(`handle("${n}")`) || swift.includes(`case "${n}"`) || swift.includes(`for name in ["${n}"]`), `Swift answers ${String(n)}`).toBe(true);
  });
  test('K3 one rule, from the one table, no platform test beside a control', () => {
    for (const [d, op] of [['canSpeed', 'radioSpeed'], ['canAsk', 'choirAsk'], ['canHandsFree', 'choirHandsFreeToggle'], ['canTalkCancel', 'choirTalkCancel']]) {
      expect(spine).toContain(`derive ${d} = nans.known and opAnswered(nans.all, nans.ops, "${op}")`);
    }
    const lines = spine.split('\n').filter((l) => /derive (canSpeed|canAsk|canHandsFree|canTalkCancel|sendOffered)\b/.test(l)).join('\n');
    expect(lines).not.toMatch(/noNative|platform|hc\.mode|hpg|android/);
    for (const c of [bar, radio, mail, textures]) expect(c).not.toMatch(/platform ==|"android"/);
    for (const n of ['canSpeed', 'canAsk', 'canHandsFree', 'canTalkCancel', 'sendOffered']) expect(app).toContain(`    ${n}\n`);
  });
  test('K4 each control is drawn only when its op is answered', () => {
    expect(radio).toContain('if rp.onAir and canSpeed');
    expect(bar).toContain('if typing and sendOffered');
    expect(bar).toContain('if typing == false and canHandsFree');
    expect(bar).toContain('if canTalkCancel\n                button press=micCancel');
    
    expect(bar).toContain('if lineMode == "aim" and canAsk');
    expect(bar).toContain('if lineMode == "aim" and opsKnown and canAsk == false');
    expect(bar).toContain('This phone does not answer questions yet. A web address opens in the app.');
    expect(mail).toContain('if canAsk\n            button press=askMail("Summarise this thread")');
    expect(textures).toContain('if tlist.status == "ready" and tlist.hasEdition == false and q == "" and canAsk');
  });
  test('K5 a question nothing answers stays in the field and is not sent', () => {
    const i = spine.indexOf('  action sendAsk'); const j = spine.indexOf('\n  //', i);
    const send = spine.slice(i, j);
    expect(send).toContain('if au.signedIn == false and (canAsk or askUrl != "")\n      gate = "signin"');
    expect(send).toContain('if au.signedIn and askUrl != ""\n      send qm = queueAct("openWeb", askUrl)\n      ask = ""');
    expect(send).toContain('if au.signedIn and askUrl == "" and canAsk\n      send acct = choirAsk(text=ask, aim=pg.top)\n      ask = ""');
    expect(send.match(/ask = ""/g)?.length).toBe(2);
    expect(spine).toContain('derive sendOffered = canAsk or askUrl != ""');
  });
  test('K6 the bar and mail still send the question with the same arguments', () => {
    expect(spine).toContain('send acct = choirAsk(text=ask, aim=pg.top)');
    expect(read('./members-mail.contract')).toContain('send acct = choirAsk(text=q, aim=pg.top)');
    expect(bar).toContain('button testId="bar-send" press=sendAsk retainFocus=true');
    expect(bar).toContain('button testId="bar-mic" press=micTap');
  });
  test('K7 the platform not yet known offers nothing native; an unknown future platform answers all', () => {
    expect(nativeOps('')).toEqual({ known: false, all: false, ops: [] });
    expect(nativeOps('ios')).toEqual({ known: true, all: true, ops: [] });
    expect(nativeOps('web')).toEqual({ known: true, all: true, ops: [] });
    expect(nativeOps('visionos')).toEqual({ known: true, all: true, ops: [] });
    const a = nativeOps('android');
    expect(a.known).toBe(true);
    expect(a.all).toBe(false);
    expect(a.ops).toEqual([...ANDROID_ANSWERS]);
    expect(spine).toContain('derive canSpeed = nans.known and opAnswered(nans.all, nans.ops, "radioSpeed")');
    expect(spine).toContain('else empty(known=false, all=false, ops=[])');
    expect(spine).toContain('derive opsKnown = nans.known');
    expect(bar).toContain('if lineMode == "aim" and opsKnown and canAsk == false');
    expect(bar).toContain('derive aim = canAsk ? "Ask about this page…" : (opsKnown ? "Open a web address…" : "")');
  });
  test('K8 while the platform is unknown a question is kept and nothing is sent; an address still opens', () => {
    const i = spine.indexOf('  action sendAsk'); const j = spine.indexOf('\n  //', i);
    const send = spine.slice(i, j);
    expect(send).not.toContain('choirAsk(text=ask, aim=pg.top)\n      ask = ""\n    if');   // the only clearing send is behind canAsk (K5)
    expect(send).toContain('if au.signedIn and askUrl == "" and canAsk');
    expect(spine).toContain('derive sendOffered = canAsk or askUrl != ""');       // canAsk is false while unknown
  });
  test('parser self-check: a multi-line branch, a comment, a digit in a name', () => {
    const sample = `class M {
      override fun later(name: String) {
        when (name) {
          // "commented" -> nothing
          "a1", "b_2" -> reply(1)
          "c3",
          "d4" -> {
            reply("x")
          }
          "e5" -> foo(
            "arg"
          )
          else -> reply(JSONObject().put("error", "NotSupportedError"))
        }
      }
    }`;
    expect(parseWhenCases(sample)).toEqual(['a1', 'b_2', 'c3', 'd4', 'e5']);
    expect(() => parseWhenCases('no function here')).toThrow(/no `override fun later\(`/);
    expect(() => parseWhenCases('override fun later(x) { }')).toThrow(/no `when \(name\)`/);
  });
});
