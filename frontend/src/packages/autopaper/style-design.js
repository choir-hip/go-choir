// The front page's design, read at read time (lane G).
//
// Texture parses the reader's Style Guide document and answers
// /api/texture/style with closed names (internal/styleguide). This module
// never parses a document: it maps those names to fixed CSS strings and
// re-checks every name against its table, so nothing from a guide (or from a
// forged answer) reaches a style rule verbatim. A missing guide, a signed-out
// reader, a stopped computer or any failure leaves the compiled default,
// which is the current page. Personal values override the house design given
// here, for this reader only; the stored stories are never changed.

const SERIF = {
  classic: "'Iowan Old Style', 'Charter', 'Bitstream Charter', 'Sitka Text', Cambria, Georgia, 'Times New Roman', serif",
  modern: "'Charter', 'Bitstream Charter', 'Sitka Text', Cambria, Georgia, serif",
};
const WEIGHT = { 600: '600', 700: '700', 800: '800' };
const DEK = { normal: 'normal', italic: 'italic' };
const RULE = { double: '3px double', single: '2px solid', none: '0 none' };

// [design field, answer key, closed table, css variable]
const FIELDS = [
  ['serif', 'serif', SERIF, '--ap-serif'],
  ['headlineWeight', 'headline_weight', WEIGHT, '--ap-headline-weight'],
  ['dekStyle', 'dek_style', DEK, '--ap-dek-style'],
  ['nameplateRule', 'nameplate_rule', RULE, '--ap-nameplate-rule'],
];

export const DEFAULT_DESIGN = Object.freeze({
  serif: 'classic',
  headlineWeight: '700',
  dekStyle: 'normal',
  nameplateRule: 'double',
});

const has = (table, name) => typeof name === 'string' && Object.prototype.hasOwnProperty.call(table, name);

/** The CSS custom properties for a design: fixed strings from the tables. */
export function designToCssVars(design) {
  const out = {};
  for (const [field, , table, cssVar] of FIELDS) {
    out[cssVar] = table[has(table, design && design[field]) ? design[field] : DEFAULT_DESIGN[field]];
  }
  return out;
}

/** The style attribute value for the front page root. */
export function styleAttribute(design = DEFAULT_DESIGN) {
  return Object.entries(designToCssVars(design)).map(([k, v]) => `${k}: ${v}`).join('; ');
}

/**
 * Read the reader's own design from Texture, over `base` (the house design,
 * or the default). Never throws; a plain fetch, so a signed-out reader does
 * not start an auth renewal. `fetchFn` is injectable for the test.
 */
export async function fetchStyleDesign(fetchFn = (...a) => fetch(...a), base = DEFAULT_DESIGN) {
  const design = { ...DEFAULT_DESIGN, ...base };
  try {
    const res = await fetchFn('/api/texture/style', { method: 'GET', credentials: 'same-origin', headers: { Accept: 'application/json' } });
    if (!res.ok) return design;
    const body = await res.json();
    const answered = body && typeof body === 'object' ? body.design : null;
    if (!answered || typeof answered !== 'object') return design;
    for (const [field, key, table] of FIELDS) {
      if (has(table, answered[key])) design[field] = answered[key];
    }
  } catch {
    // keep what we have
  }
  return design;
}
