// Lane G: the front page takes a design from Texture's /api/texture/style and
// maps closed names to fixed CSS. Run: node --test frontend/tests/style-design.test.mjs
import test from 'node:test';
import assert from 'node:assert/strict';
import { DEFAULT_DESIGN, designToCssVars, styleAttribute, fetchStyleDesign } from '../src/packages/autopaper/style-design.js';

const answer = (design) => ({ ok: true, json: async () => ({ source: 'document', design }) });

test('defaults reproduce the current page', () => {
  const vars = designToCssVars(DEFAULT_DESIGN);
  assert.equal(vars['--ap-headline-weight'], '700');
  assert.equal(vars['--ap-dek-style'], 'normal');
  assert.equal(vars['--ap-nameplate-rule'], '3px double');
  assert.match(vars['--ap-serif'], /^'Iowan Old Style'/);
});

test('an answer from Texture becomes fixed CSS', async () => {
  const d = await fetchStyleDesign(async () => answer({ serif: 'modern', headline_weight: '800', dek_style: 'italic', nameplate_rule: 'none' }));
  const vars = designToCssVars(d);
  assert.equal(vars['--ap-headline-weight'], '800');
  assert.equal(vars['--ap-dek-style'], 'italic');
  assert.equal(vars['--ap-nameplate-rule'], '0 none');
});

test('hostile or unknown values in an answer never reach a variable', async () => {
  const d = await fetchStyleDesign(async () => answer({ serif: 'red; x', headline_weight: '700; } body { display:none', dek_style: 'url(//evil)', nameplate_rule: {} , extra: 'x' }));
  assert.deepEqual(d, DEFAULT_DESIGN);
  for (const v of Object.values(designToCssVars(d))) assert.doesNotMatch(v, /[;{}]|url\(|evil/);
  assert.doesNotMatch(styleAttribute(d), /evil|display/);
});

test('failures keep the default: network error, 401, bad json, no design', async () => {
  for (const f of [
    async () => { throw new Error('offline'); },
    async () => ({ ok: false, status: 401, json: async () => ({}) }),
    async () => ({ ok: true, json: async () => { throw new Error('bad json'); } }),
    async () => ({ ok: true, json: async () => ({ source: 'default' }) }),
  ]) {
    assert.deepEqual(await fetchStyleDesign(f), DEFAULT_DESIGN);
  }
});

test('a personal answer overrides the house design field by field', async () => {
  const house = { ...DEFAULT_DESIGN, serif: 'modern' };
  const d = await fetchStyleDesign(async () => answer({ headline_weight: '600' }), house);
  assert.equal(d.serif, 'modern');
  assert.equal(d.headlineWeight, '600');
});
