// The design rule (owner, 2026-11-15): "i want no pills. no pills. no outlined bubbles. pills promote bad design, bad typography." Controls read as
// controls through type, position and one affordance, not a container. The rule is written at the top of ui.contract; these tests keep it.
// Ported from Swift DesignRulesTests (every case but R1, the theme radius clamp, which moved to themes.test.ts with the themes).
//  R2 a contract draws a capsule: `th.radiusPill`, or a border-radius above 12, outside ui.contract (every contract is scanned).
//  R3 a contract draws an outline (`border-style="solid"`, or a `border-width` with a colour) outside ui.contract. A hairline above a row is a rule.
//  R5 a circle or a half-height rounding: a box whose border-radius is half its height or more outside ui.contract; across lines in every contract.
//  R6 a radio drawn as a glyph (● or ○).
//  R7 a filled primary block (`primary=true`) outside the flow pages: every other action is an accent text action (Reader's Add, Reply, Sign in).
//  R4 ui.contract itself draws no capsule or outline.
import { describe, expect, test } from 'bun:test';
import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import { allElements, code, contracts, lab, numberAfter, radii, sizes, trim } from './contract-rules.ts';

const isRowRule = (l: string): boolean => l.includes('border-top-width=1 border-right-width=0 border-bottom-width=0');

describe('design rules', () => {
  test('R2 no contract draws a capsule', () => {
    const bad: string[] = []; let seen = 0;
    for (const f of contracts().filter((c) => c.name !== 'ui.contract')) f.lines.forEach((raw, n) => {
      const l = code(raw); seen++;
      if (l.includes('th.radiusPill')) bad.push(`${f.name}:${n + 1} uses th.radiusPill`);
      for (const v of radii(l)) if (v > 12) bad.push(`${f.name}:${n + 1} a radius above 12 (a capsule or a bubble)`);
    });
    expect(bad).toEqual([]); expect(seen).toBeGreaterThan(500);
  });

  test('R3 no contract outlines anything', () => {
    const bad: string[] = [];
    for (const f of contracts().filter((c) => c.name !== 'ui.contract')) f.lines.forEach((raw, n) => {
      const l = code(raw);
      if (!(l.includes('border-style="solid"') || (l.includes('border-width=') && l.includes('border-color=')))) return;
      if (isRowRule(l)) return;
      bad.push(`${f.name}:${n + 1} draws an outline: ${trim(l).slice(0, 110)}`);
    });
    expect(bad).toEqual([]);
  });

  test('R5, R6 no circles or round-ended bars', () => {
    const bad: string[] = [];
    for (const f of contracts().filter((c) => c.name !== 'ui.contract')) f.lines.forEach((raw, n) => {
      const l = code(raw);
      if (l.includes('●') || l.includes('○')) bad.push(`${f.name}:${n + 1} a radio glyph`);
      const r = radii(l)[0]; const h = numberAfter(' height=', l);
      if (r === undefined || r <= 0 || h === undefined) return;
      if (!(r * 2 < h)) bad.push(`${f.name}:${n + 1} a circle or a round-ended bar (radius ${r}, height ${Math.trunc(h)})`);
    });
    expect(bad).toEqual([]);
  });

  // R5 across lines, in every contract including ui.contract: radius and size are read together wherever on the element they are written.
  test('R5 no element is a capsule across lines', () => {
    const bad: string[] = []; let elements = 0;
    for (const els of allElements()) for (const e of els) {
      elements++;
      const rs = radii(e.text).filter((r) => r > 0);
      if (rs.length === 0) continue;
      const r = Math.max(...rs);
      const dims = [...sizes('height', e.text), ...sizes('width', e.text), ...sizes('min-height', e.text)];
      if (dims.length === 0) continue;
      const d = Math.min(...dims);
      if (!(r * 2 < d)) bad.push(`${e.file}:${e.line} radius ${r} on a box ${Math.trunc(d)} pt across (half or more of it)`);
    }
    expect(bad).toEqual([]); expect(elements).toBeGreaterThan(2000);
  });

  // R7: a flat accent block is for a page whose whole job is one action in a sequence.
  const flowPages = new Set(['onboarding.contract', 'voice.contract', 'ui.contract']);
  test('R7 filled blocks are only on the flow pages', () => {
    const bad: string[] = [];
    for (const f of contracts().filter((c) => !flowPages.has(c.name))) f.lines.forEach((raw, n) => {
      if (code(raw).includes('primary=true')) bad.push(`${f.name}:${n + 1} a filled block outside the flow pages (use an accent text action)`);
    });
    expect(bad).toEqual([]);
  });

  // One accent use per list row (ui.contract). A general check would need the element tree with its branches (an if and an else both carry an accent), so
  // the row that broke it is pinned: the public paper's rows on the radio carry at most one accent text (the Play word; Add is muted).
  test('the radio public rows carry one accent use', () => {
    let rows = 0; const bad: string[] = [];
    for (const els of allElements()) els.forEach((r, k) => {
      if (r.file !== 'radio.contract' || !r.text.includes('testId=`radio-public-row-')) return;
      rows++;
      let n = 0;
      for (let j = k + 1; j < els.length && els[j].indent > r.indent; j++) if (els[j].text.startsWith('text ') && /color=th\.accent(?![A-Za-z])/.test(els[j].text)) n++;
      if (n > 1) bad.push(`radio.contract:${r.line} a public row has ${n} accent texts`);
    });
    expect(bad).toEqual([]); expect(rows).toBe(1);
  });

  // R4: the capsule button face is gone from ui.contract itself.
  test('R4 the shared faces draw no capsule or outline', () => {
    const ui = readFileSync(join(lab, 'ui.contract'), 'utf8').split('\n').map(code).join('\n');
    expect(ui.includes('radiusPill')).toBe(false); expect(ui.includes('border-style')).toBe(false); expect(ui.includes('border-width')).toBe(false);
    expect(ui.includes('component ChoiceWord') && ui.includes('component ChipFace')).toBe(true);
  });
});
