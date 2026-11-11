/**
 * The golden documents through the real web renderer
 * (internal/texturedoc/golden/README.md; Autoradio pass-1 acceptance: web and
 * native draw the same page format).
 *
 * Each golden file is a publication bundle. The spec serves it from the
 * public resolve route, opens /pub/texture/<slug>, lets the Texture reader
 * draw it (renderStructuredTextureDocHTML or, with no body_doc, the Markdown
 * renderer), and projects the DOM into the shared structural form: the ordered
 * blocks with kind, level, depth, list marker, whitespace-normalised text with
 * a [n] token for each citation, the styled spans, each citation's number,
 * title and quote, an embed's kind and label, a table's cells.
 *
 * The native side (AutoradioKit GoldenDocumentsTests) writes the same form
 * from TextureDocParser's ReaderDoc. scripts/golden-docs diffs the two.
 *
 * Ways it can fail, written before the code:
 *   1. the reader draws nothing for a document (a body the renderer refuses
 *      falls to a blank page): the spec fails for that document by name;
 *   2. the projection depends on a class or a style a redesign renames: it
 *      reads tags and data- attributes only;
 *   3. the page is read before the reader settles: the DOM is read after it
 *      has been unchanged for 400 ms;
 *   4. a document is silently skipped: the spec counts the golden files and
 *      writes one projection for each.
 *
 *   cd frontend && pnpm build && pnpm exec vite preview --port 4174 --strictPort &
 *   GOLDEN_OUT=/some/dir PLAYWRIGHT_BASE_URL=http://localhost:4174 \
 *     pnpm exec playwright test tests/golden-documents.spec.js
 * (scripts/golden-docs does all of it.)
 */
import fs from 'node:fs';
import nodePath from 'node:path';
import { fileURLToPath } from 'node:url';
import { expect, test } from '@playwright/test';

const BASE_URL = (process.env.PLAYWRIGHT_BASE_URL || 'http://localhost:4173').replace(/\/+$/, '');
const HERE = nodePath.dirname(fileURLToPath(import.meta.url));
const GOLDEN_DIR = nodePath.resolve(HERE, '..', '..', 'internal', 'texturedoc', 'golden');
const OUT_DIR = nodePath.join(process.env.GOLDEN_OUT || nodePath.resolve(HERE, '..', 'test-results', 'golden'), 'web');

const files = fs.readdirSync(GOLDEN_DIR).filter((f) => f.endsWith('.json') && f !== 'known-differences.json').sort();

test.use({ viewport: { width: 390, height: 844 }, storageState: { cookies: [], origins: [] } });

test('there are golden documents', () => {
  expect(files.length).toBeGreaterThanOrEqual(8);
});

for (const file of files) {
  const golden = JSON.parse(fs.readFileSync(nodePath.join(GOLDEN_DIR, file), 'utf8'));
  const name = golden.golden.name;
  test(`golden ${name}`, async ({ page }) => {
    await page.route('**/auth/session', (route) =>
      route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ authenticated: false }) }),
    );
    await page.route('**/api/platform/publications/resolve?*', (route) =>
      route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(golden) }),
    );
    await page.goto(`${BASE_URL}${golden.route.path}`);
    const reader = page.locator('[data-texture-published-reader]');
    await expect(reader).toBeVisible({ timeout: 30_000 });
    // Let the reader settle: its HTML unchanged for 400 ms.
    await expect.poll(async () => {
      const a = await reader.innerHTML();
      await page.waitForTimeout(400);
      return a === (await reader.innerHTML()) && a.trim().length > 0;
    }, { timeout: 20_000 }).toBe(true);

    const projection = await reader.evaluate((root, withChecks) => {
      const squash = (s) => String(s || '').replace(/[\s ]+/g, ' ').trim();
      const BLOCK_TAGS = new Set(['H1', 'H2', 'H3', 'H4', 'H5', 'H6', 'P', 'UL', 'OL', 'BLOCKQUOTE', 'PRE', 'HR', 'TABLE', 'FIGURE', 'DIV', 'SECTION']);
      const blocks = [];

      function citeOf(el) {
        const label = el.querySelector('.texture-source-ref-label')?.textContent || el.getAttribute('data-source-label') || '';
        const pop = el.querySelector('[data-texture-source-ref-popover]');
        const c = { n: Number(squash(label)) || squash(label), title: squash(pop?.querySelector('strong')?.textContent) };
        const q = pop?.querySelector('mark[data-citation-quote]') || pop?.querySelector('.texture-transclusion-quote');
        c.quote = squash(q?.textContent).replace(/^“|”$/g, '');
        if (withChecks) c.check = el.getAttribute('data-check') || '';
        return c;
      }

      // The inline content of nodes: text with a [n] token per citation, the
      // styled spans, the citations.
      function inline(nodes) {
        const out = { text: '', pieces: [], cites: [] };
        const walk = (node, style) => {
          if (node.nodeType === 3) {
            const t = node.textContent;
            if (!t) return;
            out.text += t;
            out.pieces.push({ style: [...style].sort().join('+'), text: t });
            return;
          }
          if (node.nodeType !== 1) return;
          if (node.matches('[data-texture-source-ref]')) {
            const c = citeOf(node);
            out.text += `[${c.n}]`;
            out.pieces.push({ style: '', text: `[${c.n}]`, cite: true });
            out.cites.push(c);
            return;
          }
          if (node.tagName === 'BR') { out.text += '\n'; out.pieces.push({ style: '', text: '\n' }); return; }
          const next = new Set(style);
          if (node.tagName === 'STRONG' || node.tagName === 'B') next.add('strong');
          if (node.tagName === 'EM' || node.tagName === 'I') next.add('emphasis');
          if (node.tagName === 'CODE') next.add('code');
          node.childNodes.forEach((child) => walk(child, next));
        };
        nodes.forEach((n) => walk(n, new Set()));
        const spans = [];
        for (const p of out.pieces) {
          if (!p.style || p.cite) { if (p.cite || squash(p.text)) spans.push(null); continue; }
          const prev = spans[spans.length - 1];
          if (prev && prev.style === p.style) prev.text += p.text; else spans.push({ style: p.style, text: p.text });
        }
        const styled = spans.filter(Boolean).map((s) => ({ style: s.style, text: squash(s.text) })).filter((s) => s.text);
        return { text: squash(out.text), spans: styled, cites: out.cites };
      }

      function push(kind, nodes, extra = {}) {
        const i = inline(nodes);
        const b = { kind, ...extra, text: i.text };
        if (i.spans.length) b.spans = i.spans;
        if (i.cites.length) b.cites = i.cites;
        blocks.push(b);
      }

      function list(el, listDepth, quoteDepth) {
        const ordered = el.tagName === 'OL';
        let n = Number(el.getAttribute('start')) || 1;
        for (const li of [...el.children].filter((c) => c.tagName === 'LI')) {
          const marker = ordered ? `${n}.` : 'bullet';
          n += 1;
          let run = [];
          let first = true;
          const flush = () => {
            if (run.length === 0) return;
            const i = inline(run);
            if (i.text || i.cites.length || first) push('list_item', run, { depth: listDepth + 1, marker: first ? marker : '' });
            first = false;
            run = [];
          };
          for (const child of li.childNodes) {
            if (child.nodeType === 1 && BLOCK_TAGS.has(child.tagName)) { flush(); if (first) { push('list_item', [], { depth: listDepth + 1, marker }); first = false; } block(child, listDepth + 1, quoteDepth); }
            else run.push(child);
          }
          flush();
          if (first) push('list_item', [], { depth: listDepth + 1, marker });
        }
      }

      function block(el, listDepth, quoteDepth) {
        const tag = el.tagName;
        // Page chrome the reader puts above the text (the AI disclosure, the
        // opinion label): native carries it in ReaderDoc fields, not blocks.
        if (el.matches('[data-texture-ai-disclosure],[data-texture-opinion-label],[data-texture-opinion-notice]')) return;
        if (/^H[1-6]$/.test(tag)) return push('heading', [...el.childNodes], { level: Number(tag[1]), ...(quoteDepth ? { depth: quoteDepth } : {}) });
        if (tag === 'P') return push(quoteDepth ? 'quote' : 'paragraph', [...el.childNodes], quoteDepth ? { depth: quoteDepth } : {});
        if (tag === 'UL' || tag === 'OL') return list(el, listDepth, quoteDepth);
        if (tag === 'BLOCKQUOTE') {
          const inlineRun = [];
          for (const child of el.childNodes) {
            if (child.nodeType === 1 && BLOCK_TAGS.has(child.tagName)) block(child, listDepth, quoteDepth + 1);
            else inlineRun.push(child);
          }
          if (squash(inlineRun.map((n) => n.textContent).join(''))) push('quote', inlineRun, { depth: quoteDepth + 1 });
          return;
        }
        if (tag === 'PRE') return void blocks.push({ kind: 'code', text: el.textContent.replace(/\n+$/, '') });
        if (tag === 'HR') return void blocks.push({ kind: 'rule', text: '' });
        if (tag === 'TABLE') {
          const rows = [...el.querySelectorAll('tr')].map((tr) => [...tr.children].map((c) => squash(c.textContent)));
          return void blocks.push({ kind: 'table', text: '', rows });
        }
        if (tag === 'FIGURE' && el.hasAttribute('data-texture-embed')) {
          const cap = el.querySelector('figcaption .texture-embed-caption') || el.querySelector('.texture-embed-file-title') || el.querySelector('figcaption');
          return void blocks.push({ kind: 'embed', embed: el.getAttribute('data-embed-kind') || '', text: squash(cap?.textContent) });
        }
        // A container the renderer wraps blocks in: read through it.
        [...el.children].forEach((c) => block(c, listDepth, quoteDepth));
      }

      [...root.children].forEach((c) => block(c, 0, 0));
      return blocks;
    }, !!golden.golden.checks);

    expect(projection.length, `${name}: the web reader drew no blocks`).toBeGreaterThan(0);
    fs.mkdirSync(OUT_DIR, { recursive: true });
    fs.writeFileSync(nodePath.join(OUT_DIR, `${name}.json`), JSON.stringify({ name, blocks: projection }, null, 1) + '\n');
  });
}
