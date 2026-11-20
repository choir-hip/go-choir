// The typographic system (owner, 2026-11-16: "We need to have more typography and use typography to signify what's the difference between static
// text and a button and a headline. Typography is too same same, sans serif."). The roles are in mobile/docs/typography.md; the contracts name a role's
// face, never a face: th.fontHead, th.fontText, th.fontLabel, th.fontAction, th.fontMono. Ported from Swift TypographyRulesTests (every case but T8,
// which tests Theme and ThemeStore and stays in Swift until the themes port).
//  T1 a face outside the role set.  T2 a `text` with no role face on itself or an ancestor.  T3 a bare-number font size.
//  T4 an Action that is not semibold or is uppercase, tracked or italic; a static Label that is not regular unless it is the caps label.
//  T5 a word inside a button that is not an Action, or an Action outside any control.  T6 uppercase or tracking outside the caps Label; italic outside serif.
//  T7 a figure set without tabular figures.  T9 a tappable Headline with no chevron or Action word.
import { describe, expect, test } from 'bun:test';
import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import { allElements, ancestors, attr, faces, isCaps, isText, lab, literal, roleFaces } from './contract-rules.ts';

describe('typography rules', () => {
  test('T1 every face is a role face', () => {
    const bad: string[] = []; let seen = 0;
    for (const els of allElements()) for (const e of els) {
      if (!e.text.includes('font-family=')) continue;
      seen++;
      const fs = faces(e.text);
      if (fs.length === 0) bad.push(`${e.file}:${e.line} font-family names no role face`);
      for (const f of fs) if (!roleFaces.has(f)) bad.push(`${e.file}:${e.line} face ${f} is not a role face`);
    }
    expect(bad).toEqual([]); expect(seen).toBeGreaterThan(300);
  });

  test('T2 every text has a role face', () => {
    const bad: string[] = []; let texts = 0;
    for (const els of allElements()) els.forEach((e, k) => {
      if (!isText(e)) return;
      texts++;
      if (e.text.includes('font-family=')) return;
      if (!ancestors(els, k).some((a) => a.text.includes('font-family='))) bad.push(`${e.file}:${e.line} a text with no role face on it or above it: ${e.text.slice(0, 80)}`);
    });
    expect(bad).toEqual([]); expect(texts).toBeGreaterThan(400);
  });

  test('T3 text is sized by style or rem, never bare points', () => {
    const bad: string[] = [];
    for (const els of allElements()) for (const e of els) {
      if (!isText(e)) continue;
      const v = attr('font-size', e.text);
      if (v === undefined) continue;
      const ok = v.startsWith('"-exact-') || v.startsWith('dyn(') || v.startsWith('remv(') || (v.startsWith('(') && v.includes('-exact-'));
      if (!ok) bad.push(`${e.file}:${e.line} font-size=${v} is a fixed size (use a text style or dyn())`);
    }
    expect(bad).toEqual([]);
  });

  test('T4, T6 Action, Label and special treatments', () => {
    const bad: string[] = []; let actions = 0;
    for (const els of allElements()) for (const e of els) {
      if (!isText(e)) continue;
      const fs = faces(e.text); const w = attr('font-weight', e.text);
      const caps = isCaps(e); const tracked = e.text.includes('letter-spacing='); const italic = e.text.includes('font-style=');
      if (fs.length > 1) { // a ternary of two roles: the breadcrumb and the status line are the only ones
        if (!['files.contract', 'books.contract', 'settings.contract'].includes(e.file)) bad.push(`${e.file}:${e.line} a text that is two roles`);
        continue;
      }
      const f = fs[0]; if (f === undefined) continue;
      const at = `${e.file}:${e.line}`;
      if (f === 'th.fontAction') {
        actions++;
        if (w !== '600') bad.push(`${at} an Action is semibold (600), not ${w ?? 'regular'}`);
        if (caps || tracked || italic) bad.push(`${at} an Action is never uppercase, tracked or italic`);
      } else if (f === 'th.fontLabel') {
        if (caps) { if (w !== '600') bad.push(`${at} the caps label is 600`); if (!tracked) bad.push(`${at} the caps label is tracked`); }
        else { if (!(w === undefined || w === '400')) bad.push(`${at} a static Label is regular, not ${w}`); if (tracked) bad.push(`${at} only the caps label is tracked`); }
        if (italic) bad.push(`${at} italic is a serif treatment`);
      } else if (f === 'th.fontHead') {
        if (attr('text-transform', e.text) !== undefined) bad.push(`${at} a headline is never transformed to capitals (the masthead is written in capitals)`);
      } else if (f === 'th.fontText') {
        if (caps || tracked) bad.push(`${at} reading text is never uppercase or tracked`);
      }
    }
    expect(bad).toEqual([]); expect(actions).toBeGreaterThan(40);
  });

  // T5: a word in a button is an Action; an Action is in a control or on a control's face.
  const faceComponents = new Set(['Btn', 'ChoiceWord', 'SegmentFace', 'ChipFace', 'ChoiceChip', 'Chip', 'LineRow', 'ListRow', 'Runs', 'ActionWord', 'SettingsBtn', 'Choice', 'BarGlyph', 'PodAction', 'ActionButton']);
  const titleWords = new Set<string>([]);
  const verbs = new Set(['Play', 'Queue', 'Cancel', 'Close', 'Sign in', 'Not now', 'Try again', 'Delete', 'Share', 'Reload', 'Skip for now', 'Edit my edition', 'Open episode', 'Sections', 'Source']);
  test('T5 actions are controls and controls are actions', () => {
    const bad: string[] = [];
    for (const els of allElements()) els.forEach((e, k) => {
      if (!isText(e)) return;
      const anc = ancestors(els, k);
      const inButton = anc.some((a) => a.text.startsWith('button'));
      const fs = faces(e.text);
      if (fs.length === 1 && fs[0] === 'th.fontAction') {
        const pressable = e.text.includes('press=');
        const c = anc.find((a) => a.text.startsWith('component '));
        const comp = c ? c.text.slice(10).split(' ')[0] ?? '' : '';
        if (!(inButton || pressable || faceComponents.has(comp))) bad.push(`${e.file}:${e.line} an Action outside a control (component ${comp}): static text is never an Action`);
      } else {
        const lit = literal(e);
        if (inButton && lit !== undefined && lit.length <= 28 && /\p{L}/u.test(lit) && !isCaps(e) && !e.text.includes('role="heading"')) {
          const f = fs[0] ?? '';
          if (f !== 'th.fontLabel' && f !== 'th.fontHead') return;
          if (!(titleWords.has(lit) || (f === 'th.fontLabel' && !verbs.has(lit)))) bad.push(`${e.file}:${e.line} the word "${lit}" is in a button but is not an Action`);
        }
      }
    });
    expect(bad).toEqual([]);
  });

  // T9 (owner, 2026-11-16): type tells a button from a headline. Pure navigation is an Action; a tappable title of an object stays a Headline, and its
  // button carries an action affordance: a chevron, or an Action word beside it. The serif alone never signals a tap.
  test('T9 a tappable headline carries an action affordance', () => {
    const bad: string[] = []; let headlineButtons = 0;
    for (const els of allElements()) els.forEach((b, k) => {
      if (!b.text.startsWith('button ')) return;
      const inner: typeof els = []; let j = k + 1;
      while (j < els.length && els[j].indent > b.indent) { inner.push(els[j]); j++; }
      const heads = inner.filter((x) => isText(x) && faces(x.text).includes('th.fontHead') && !x.text.includes('role="heading"'));
      if (heads.length === 0) return;
      headlineButtons++;
      const affordance = inner.some((x) => x.text.includes('forward-chevron') || x.text.includes('chevron=true') || x.text.startsWith('text "›"') || x.text.startsWith('SelectMark(')
        || (isText(x) && faces(x.text).length === 1 && faces(x.text)[0] === 'th.fontAction'));
      // Where the serif is the whole content and a chevron would be noise, named with the reason: the paper's story headlines (the front page is the list;
      // Play and Queue are Actions beside them), a block of a book (reading text; a tap places the voice), a file tile (its icon and card), a topic row and
      // the two demo toggles (their line under the title says "Tap to ...").
      const exempt = ['paper-story-', 'doc-block-', 'files-row-', 'ob-topic-', 'settings-dev-details'].some((p) => b.text.includes('testId=`' + p) || b.text.includes('testId="' + p));
      const objectCard = inner.some((x) => x.text.startsWith('Card(') || x.text.startsWith('CardFill(') || x.text.startsWith('PaperCard('));
      if (!(affordance || objectCard || exempt)) bad.push(`${b.file}:${b.line} a button whose title is a Headline has no chevron or Action word: the serif alone does not say it is tappable`);
    });
    expect(bad).toEqual([]); expect(headlineButtons).toBeGreaterThan(3);
  });

  test('key pages are glyphs with labels and the placeholder is a label', () => {
    let chip = 0, labels = 0, field = false; const bad: string[] = [];
    for (const els of allElements()) els.forEach((e, k) => {
      if (e.file === 'launcher.contract' && ancestors(els, k).some((a) => a.text.startsWith('component KeyGlyph'))) {
        if (isText(e)) {
          labels++;
          if (!e.text.startsWith('text name ')) bad.push(`the caption is the page's own name: ${e.text}`);
          if (!e.text.includes('aria-hidden=true')) bad.push('the caption is hidden from VoiceOver: the button\'s label already says the name');
          if (e.text.includes('line-clamp')) bad.push('a tray name is never truncated');
        }
        if (e.text.startsWith('button')) { chip++; if (!e.text.includes('aria-label=`Open ${name}`')) bad.push('each glyph names its page for accessibility'); }
      }
      if (e.text.includes('testId="bar-field"')) {
        field = true;
        const fam = attr('font-family', e.text) ?? '';
        if (!(fam.includes('th.fontLabel') && fam.includes('th.fontText'))) bad.push(`the ask field shows its placeholder as a Label and typed words as Reading text: ${fam}`);
      }
      if (e.text.includes('content-close-word')) bad.push('the content row has the X and no Close word');
    });
    expect(bad).toEqual([]); expect(chip).toBe(1); expect(labels).toBe(1); expect(field).toBe(true);
  });

  // T7
  test('T7 figures are tabular', () => {
    const ids = ['queue-elapsed', 'queue-remaining', 'radio-speed', 'radio-elapsed', 'radio-remaining', 'radio-onair-sentence-number', 'doc-position'];
    const bad: string[] = []; let found = 0;
    for (const els of allElements()) for (const e of els) {
      if (!isText(e)) continue;
      for (const id of ids) if (e.text.includes(`"${id}"`)) {
        found++;
        if (!e.text.includes('font-variant-numeric="tabular-nums"')) bad.push(`${e.file}:${e.line} ${id} is a figure and takes tabular figures`);
      }
    }
    expect(bad).toEqual([]); expect(found).toBeGreaterThanOrEqual(4);
  });

  // the role table is in the doc and names every token
  test('the doc names every role', () => {
    const s = readFileSync(join(lab, '..', 'docs', 'typography.md'), 'utf8');
    for (const t of [...roleFaces].sort()) expect(s.includes(t)).toBe(true);
    for (const r of ['Headline', 'Reading text', 'Spoken line', 'Label', 'Action', 'Figure']) expect(s.includes(r)).toBe(true);
  });
});
