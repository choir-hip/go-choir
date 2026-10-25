// Embed blocks in the Texture web renderer (station TE,
// docs/design/texture-evidence-embeds-te-2026-10-25.md section 5).
// Ways the renderer could fail, written before the code:
//   - the image loads from anywhere but the owner's asset route, or a
//     malformed digest becomes a URL;
//   - the box is not reserved from the viewport, so the page jumps;
//   - the caption, route or alt reach the page unescaped, or the figure
//     carries an inline handler (script in the document origin);
//   - a blank capture is not badged, and all-blank evidence is not called out;
//   - an unresolvable capture shows a broken image instead of a clear state;
//   - an unknown kind renders nothing (it must be a plain card);
//   - the editor's serializer builds an embed from the DOM (a forged
//     attribute) or drops one on an owner save;
//   - the editor's projection differs from the server's.
import { test, expect } from '@playwright/test';
import {
  allBlankOperations,
  embedProjectionText,
  evidenceIdentityText,
  markEmbedUnresolvable,
  renderEmbedBlockHTML,
  serializeEmbedElement,
} from '../src/lib/texture-embeds.ts';

const DIGEST = '0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef';

function evidence(overrides = {}, target = {}) {
  return {
    type: 'embed',
    attrs: {
      id: 'embed-1',
      kind: 'capsule_evidence',
      target: { digest: DIGEST, operation_id: 'selfdev-op-1', computer_id: 'computer-1', route: '/reader', width: 1280, height: 800, blank: false, ...target },
      caption: 'Screenshot of /reader from candidate selfdev-op-1 (1280x800)',
      alt: 'Screenshot of /reader',
      ...overrides,
    },
  };
}

test('capsule evidence renders from the owner asset route in a reserved box', () => {
  const html = renderEmbedBlockHTML(evidence());
  expect(html).toContain(`<img src="/api/assets/sha256/${DIGEST}"`);
  expect(html).toContain('width="1280"');
  expect(html).toContain('height="800"');
  expect(html).toContain('contenteditable="false"');
  expect(html).toContain('data-texture-embed-id="embed-1"');
  expect(html).toContain('<code>/reader</code>');
  expect(html).toContain('selfdev-op-1');
  expect(html).not.toMatch(/\son[a-z]+=/i);
  expect(html).not.toContain('<script');
  expect(html).not.toContain('style=');
  expect(html).not.toContain('texture-embed-badge');
});

test('text reaches the page escaped and a malformed digest never becomes a URL', () => {
  const hostile = renderEmbedBlockHTML(evidence({ caption: '<img src=x onerror=alert(1)>', alt: '"><script>alert(1)</script>' }, { route: '/"><b>x</b>' }));
  expect(hostile).not.toContain('<script>');
  expect(hostile).not.toContain('<b>x</b>');
  expect(hostile).not.toMatch(/<img src=x/);
  expect(hostile).toContain('&lt;img src=x onerror=alert(1)&gt;');
  const bad = renderEmbedBlockHTML(evidence({}, { digest: '../../api/secret' }));
  expect(bad).not.toContain('<img');
  expect(bad).toContain('data-embed-state="unresolvable"');
  expect(bad).not.toContain('../../api/secret');
});

test('blank captures are badged and all-blank evidence is called out', () => {
  const blank = evidence({ id: 'embed-2' }, { blank: true });
  const html = renderEmbedBlockHTML(blank);
  expect(html).toContain('texture-embed-badge');
  expect(html).toContain('blank');
  expect(html).not.toContain('Every capture of this candidate is blank');
  const ops = allBlankOperations([blank, evidence({ id: 'embed-3' }, { blank: true })]);
  expect(ops.has('selfdev-op-1')).toBe(true);
  expect(renderEmbedBlockHTML(blank, { allBlank: ops })).toContain('Every capture of this candidate is blank');
  expect(allBlankOperations([blank, evidence()]).has('selfdev-op-1')).toBe(false);
});

test('an unresolvable image switches its figure to a clear state', () => {
  const figure = { dataset: {}, querySelector: (sel) => (sel === '[data-texture-embed-unresolvable]' ? notice : null) };
  const notice = { hidden: true };
  const img = {
    hidden: false,
    matches: (sel) => sel === 'img[data-texture-embed-media]',
    closest: (sel) => (sel === '[data-texture-embed]' ? figure : null),
  };
  expect(markEmbedUnresolvable(img)).toBe(true);
  expect(figure.dataset.embedState).toBe('unresolvable');
  expect(img.hidden).toBe(true);
  expect(notice.hidden).toBe(false);
  expect(markEmbedUnresolvable({ matches: () => false })).toBe(false);
});

test('an unknown kind renders as a plain card and image renders its alt', () => {
  const unknown = renderEmbedBlockHTML({ type: 'embed', attrs: { id: 'e9', kind: 'video', target: { digest: DIGEST } } });
  expect(unknown).toContain('texture-embed-unknown');
  expect(unknown).toContain('video');
  expect(unknown).not.toContain('<img');
  expect(unknown).not.toContain('<video');
  const image = renderEmbedBlockHTML({ type: 'embed', attrs: { id: 'e8', kind: 'image', target: { digest: DIGEST }, alt: 'A diagram' } });
  expect(image).toContain(`<img src="/api/assets/sha256/${DIGEST}"`);
  expect(image).toContain('alt="A diagram"');
});

test('the serializer keeps an embed only by its id from the loaded revision', () => {
  const original = evidence();
  const existingDoc = { schema: 'choir.texture_doc.v1', doc: { type: 'doc', attrs: { id: 'd' }, content: [original] } };
  const element = (id) => ({ getAttribute: (name) => (name === 'data-texture-embed-id' ? id : name === 'data-embed-digest' ? 'f'.repeat(64) : '') });
  const kept = serializeEmbedElement(element('embed-1'), existingDoc);
  expect(kept).toEqual(original);
  expect(kept).not.toBe(original);
  expect(serializeEmbedElement(element('embed-forged'), existingDoc)).toBeNull();
  expect(serializeEmbedElement(element('embed-1'), null)).toBeNull();
});

test('the projection matches the server and the identity line reads the live listing', () => {
  expect(embedProjectionText(evidence())).toBe('[evidence: Screenshot of /reader from candidate selfdev-op-1 (1280x800)]');
  expect(embedProjectionText({ type: 'embed', attrs: { kind: 'image', alt: 'A diagram' } })).toBe('[image: A diagram]');
  expect(embedProjectionText({ type: 'embed', attrs: { kind: 'image', alt: 'A', caption: 'Cap' } })).toBe('[image: Cap]');
  expect(embedProjectionText({ type: 'embed', attrs: { kind: 'video' } })).toBe('[embed]');
  const line = evidenceIdentityText('selfdev-op-1', { status: 200, listing: { bundle_digest: 'b'.repeat(64), frontend_sha256: 'f'.repeat(64), capture_receipt: 'capsule-exec:sha256:' + 'r'.repeat(64) } });
  expect(line).toContain('selfdev-op-1');
  expect(line).toContain('bundle bbbbbbbbbbbb');
  expect(line).toContain('build ffffffffffff');
  expect(evidenceIdentityText('selfdev-op-1', { status: 410 })).toContain('no longer retained');
  expect(evidenceIdentityText('selfdev-op-1', { status: 404 })).toContain('no longer retained');
});
