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
//   - the editor's projection differs from the server's;
//   - a file card (station L8) renders its title or path unescaped, is
//     clickable for a path outside Files (absolute, ".."), or has an inline
//     handler instead of the editor's delegated open;
//   - a video embed (station L8) puts anything but the 11-character id in the
//     frame URL, drops the sandbox or lazy loading, or autoplays; an audio
//     embed plays a non-https source, autoplays, or preloads the file.
import { test, expect } from '@playwright/test';
import {
  allBlankOperations,
  applyEntriesToAdd,
  applyEntryHTML,
  hydrateApplyEntries,
  embedProjectionText,
  evidenceIdentityText,
  evidenceListingPath,
  fileCardTarget,
  isAudioURL,
  isFilesPath,
  isVideoID,
  handleRevertClick,
  isRevertable,
  laterAppliesAfter,
  markEmbedUnresolvable,
  renderEmbedBlockHTML,
  revertConfirmText,
  revertPath,
  serializeEmbedElement,
} from '../src/packages/texture/texture-embeds.ts';

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
  const unknown = renderEmbedBlockHTML({ type: 'embed', attrs: { id: 'e9', kind: 'hologram', target: { digest: DIGEST } } });
  expect(unknown).toContain('texture-embed-unknown');
  expect(unknown).toContain('hologram');
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

test('a screen file embed names its file in the computer tree', () => {
  const html = renderEmbedBlockHTML(evidence({}, { route: undefined, file: 'Self-development/selfdev-op-1/screens/home.png' }));
  expect(html).toContain('<code data-texture-embed-file>Self-development/selfdev-op-1/screens/home.png</code>');
  expect(html).toContain(`src="/api/assets/sha256/${DIGEST}"`);
  expect(evidenceListingPath('computer-1', 'selfdev-op-1')).toBe('/api/computers/computer-1/self-development/operations/selfdev-op-1');
});

test('the projection matches the server and the identity line reads the live listing', () => {
  expect(embedProjectionText(evidence())).toBe('[evidence: Screenshot of /reader from candidate selfdev-op-1 (1280x800)]');
  expect(embedProjectionText({ type: 'embed', attrs: { kind: 'image', alt: 'A diagram' } })).toBe('[image: A diagram]');
  expect(embedProjectionText({ type: 'embed', attrs: { kind: 'image', alt: 'A', caption: 'Cap' } })).toBe('[image: Cap]');
  expect(embedProjectionText({ type: 'embed', attrs: { kind: 'hologram' } })).toBe('[embed]');
  const line = evidenceIdentityText('selfdev-op-1', { status: 200, listing: { bundle_digest: 'b'.repeat(64), state: 'awaiting_approval' } });
  expect(line).toContain('selfdev-op-1');
  expect(line).toContain('bundle bbbbbbbbbbbb');
  expect(line).toContain('awaiting_approval');
  expect(evidenceIdentityText('selfdev-op-1', { status: 410 })).toContain('no longer retained');
  expect(evidenceIdentityText('selfdev-op-1', { status: 404 })).toContain('no longer retained');
});

test('a Revert action warns when later applies are undone with the candidate', async () => {
  const ops = [
    { operation_id: 'a', state: 'applied', created_at: '2026-11-06T10:00:00Z' },
    { operation_id: 'b', state: 'applied', created_at: '2026-11-06T11:00:00.5Z' },
    { operation_id: 'c', state: 'failed', created_at: '2026-11-06T12:00:00Z' },
    { operation_id: 'd', state: 'applied', created_at: '2026-11-06T13:00:00Z' },
  ];
  expect(laterAppliesAfter(ops, 'a')).toBe(2);
  expect(laterAppliesAfter(ops, 'd')).toBe(0);
  expect(laterAppliesAfter(ops, 'missing')).toBe(0);
  expect(revertConfirmText(0)).not.toContain('also undoes');
  expect(revertConfirmText(1)).toContain('also undoes 1 later applied change,');
  expect(revertConfirmText(2)).toContain('2 later applied changes');
  expect(revertPath('computer-1', 'selfdev-op-1')).toBe('/api/computers/computer-1/self-development/operations/selfdev-op-1/revert');
  expect(isRevertable({ status: 200, listing: { state: 'applied' } })).toBe(true);
  expect(isRevertable({ status: 200, listing: { state: 'rolled_back' } })).toBe(false);
  expect(isRevertable({ status: 404 })).toBe(false);
  expect(isRevertable(null)).toBe(false);
});

test('an apply with no screenshot still gets an entry with a Revert action', async () => {
  const decision = 'c'.repeat(64);
  const ops = [
    { operation_id: 'op-a', state: 'applied', trajectory_id: 'traj-1', decision_actor: 'consensus:reversible-selfdev-v1', decision_event: decision, created_at: '2026-11-06T10:00:00Z' },
    { operation_id: 'op-b', state: 'applied', trajectory_id: 'traj-other', created_at: '2026-11-06T11:00:00Z' },
    { operation_id: 'op-c', state: 'failed', trajectory_id: 'traj-1', created_at: '2026-11-06T12:00:00Z' },
    { operation_id: 'op-d', state: 'applied', trajectory_id: 'traj-1', decision_actor: 'user-1', created_at: '2026-11-06T13:00:00Z' },
  ];
  const added = applyEntriesToAdd(ops, 'traj-1', new Set(['op-d']));
  expect(added.map((op) => op.operation_id)).toEqual(['op-a']);
  const html = applyEntryHTML(ops[0], 'computer-1');
  expect(html).toContain('on a consensus decision');
  expect(html).toContain('sha256:cccccccccccc');
  expect(html).toContain('data-texture-embed ');
  expect(html).toContain('data-texture-embed-revert');
  expect(html).toContain('data-embed-operation="op-a"');
  expect(html).not.toMatch(/\son[a-z]+=/i);
  expect(html).not.toContain('data-texture-embed-id');
  const inserted = [];
  const root = { querySelectorAll: () => [], insertAdjacentHTML: (where, markup) => inserted.push([where, markup]) };
  const fetchFn = async (url) => ({
    ok: true,
    json: async () => (url === '/api/shell/bootstrap' ? { computer_id: 'computer-1' } : { operations: ops }),
  });
  await hydrateApplyEntries(root, 'traj-1', fetchFn);
  expect(inserted).toHaveLength(2);
  expect(inserted[0][1]).toContain('op-a');
  await hydrateApplyEntries(root, '', fetchFn);
  expect(inserted).toHaveLength(2);
});

test('an operation the computer no longer carries is not counted, hosted or reverted', async () => {
  const ops = [
    { operation_id: 'a', state: 'applied', trajectory_id: 't', created_at: '2026-11-06T10:00:00Z' },
    { operation_id: 'b', state: 'applied', trajectory_id: 't', created_at: '2026-11-06T11:00:00Z' },
    { operation_id: 'c', state: 'applied', trajectory_id: 't', created_at: '2026-11-06T12:00:00Z' },
  ];
  expect(laterAppliesAfter(ops, 'a', ['a', 'c'])).toBe(1);
  expect(laterAppliesAfter(ops, 'a')).toBe(2);
  expect(applyEntriesToAdd(ops, 't', new Set(), ['a', 'c']).map((op) => op.operation_id)).toEqual(['a', 'c']);
  const attrs = { 'data-embed-operation': 'b', 'data-embed-computer': 'computer-1' };
  const button = { disabled: false, textContent: 'Revert', getAttribute: (k) => attrs[k], closest() { return this; } };
  const calls = [];
  const fetchFn = async (url, init) => {
    calls.push(init?.method || 'GET');
    return { ok: true, json: async () => ({ operations: ops, live_operation_ids: ['a', 'c'] }) };
  };
  expect(await handleRevertClick({ closest: () => button }, fetchFn, () => true)).toBe(true);
  expect(button.textContent).toBe('Already undone');
  expect(calls).toEqual(['GET']);
});

test('the Revert click confirms, then posts the owner revert once', async () => {
  const attrs = { 'data-embed-operation': 'a', 'data-embed-computer': 'computer-1' };
  const button = { disabled: false, textContent: 'Revert', getAttribute: (k) => attrs[k], closest() { return this; } };
  const target = { closest: (selector) => (selector === '[data-texture-embed-revert]' ? button : null) };
  const calls = [];
  const fetchFn = async (url, init) => {
    calls.push([url, init?.method || 'GET']);
    if (url.endsWith('/operations')) {
      return { ok: true, json: async () => ({ operations: [{ operation_id: 'a', state: 'applied', created_at: '2026-11-06T10:00:00Z' }, { operation_id: 'b', state: 'applied', created_at: '2026-11-06T11:00:00Z' }] }) };
    }
    return { ok: true, status: 201, json: async () => ({ state: 'rollback_pending' }) };
  };
  let asked = '';
  expect(await handleRevertClick({ closest: () => null }, fetchFn, () => true)).toBe(false);
  expect(await handleRevertClick(target, fetchFn, (text) => { asked = text; return false; })).toBe(true);
  expect(asked).toContain('also undoes 1 later applied change');
  expect(calls.filter(([, method]) => method === 'POST')).toHaveLength(0);
  expect(await handleRevertClick(target, fetchFn, () => true)).toBe(true);
  expect(calls.filter(([url, method]) => method === 'POST' && url.endsWith('/operations/a/revert'))).toHaveLength(1);
  expect(button.disabled).toBe(true);
  expect(button.textContent).toBe('Reverting started');
  expect(await handleRevertClick(target, fetchFn, () => true)).toBe(true);
  expect(calls.filter(([, method]) => method === 'POST')).toHaveLength(1);
});

test('a file card shows its title and path escaped and opens only a Files path', () => {
  const node = (path, title = 'Summary <report>') => ({ type: 'embed', attrs: { id: 'embed-f', kind: 'file', title, target: { path, mime: 'application/pdf' } } });
  const html = renderEmbedBlockHTML(node('Reports/summary.pdf'));
  expect(html).toContain('texture-embed-file');
  expect(html).toContain('Summary &lt;report&gt;');
  expect(html).toContain('data-embed-path="Reports/summary.pdf"');
  expect(html).toContain('PDF');
  expect(html).not.toMatch(/ on[a-z]+=/);
  expect(embedProjectionText(node('Reports/summary.pdf', 'Summary'))).toBe('[file: Summary]');
  for (const bad of ['/etc/passwd', '../x.pdf', 'Reports/../../x.pdf', 'Reports//x.pdf', '']) {
    expect(isFilesPath(bad)).toBe(false);
    expect(renderEmbedBlockHTML(node(bad))).not.toContain('data-texture-embed-file-open');
  }
  const card = { getAttribute: (k) => ({ 'data-embed-path': 'Reports/summary.pdf', 'data-embed-mime': 'application/pdf', 'data-embed-title': 'Summary' })[k] };
  expect(fileCardTarget({ closest: () => card })).toEqual({ path: 'Reports/summary.pdf', mime: 'application/pdf', title: 'Summary' });
  expect(fileCardTarget({ closest: () => null })).toBeNull();
  const forged = { getAttribute: () => '../../secret' };
  expect(fileCardTarget({ closest: () => forged })).toBeNull();
});

test('a video embed is a sandboxed lazy youtube-nocookie frame built from the id alone', () => {
  const node = (id, caption = 'The <talk>') => ({ type: 'embed', attrs: { id: 'embed-v', kind: 'video', alt: caption, caption, target: { video_id: id } } });
  const html = renderEmbedBlockHTML(node('dQw4w9WgXcQ'));
  expect(html).toContain('src="https://www.youtube-nocookie.com/embed/dQw4w9WgXcQ?rel=0"');
  expect(html).toContain('loading="lazy"');
  expect(html).toContain('sandbox="allow-scripts allow-same-origin allow-presentation"');
  expect(html).toContain('allow="fullscreen; picture-in-picture"');
  expect(html).toContain('The &lt;talk&gt;');
  expect(html).not.toMatch(/autoplay/i);
  expect(html).not.toMatch(/ on[a-z]+=/);
  expect(embedProjectionText(node('dQw4w9WgXcQ', 'Talk'))).toBe('[video: Talk]');
  for (const bad of ['', 'short', 'dQw4w9WgXc"', 'https://evil.example/x', 'dQw4w9WgXcQ"onload="x']) {
    expect(isVideoID(bad)).toBe(false);
    const out = renderEmbedBlockHTML(node(bad));
    expect(out).not.toContain('<iframe');
    expect(out).toContain('unresolvable');
  }
});

test('an audio embed is a native no-autoplay player for an https source only', () => {
  const node = (url, title = 'Episode <42>') => ({ type: 'embed', attrs: { id: 'embed-a', kind: 'audio', title, target: { url } } });
  const html = renderEmbedBlockHTML(node('https://example.org/ep/42.mp3?a=1&b="2"'));
  expect(html).toContain('<audio');
  expect(html).toContain(' controls');
  expect(html).toContain('preload="none"');
  expect(html).toContain('src="https://example.org/ep/42.mp3?a=1&amp;b=&quot;2&quot;"');
  expect(html).toContain('Episode &lt;42&gt;');
  expect(html).toContain('rel="noopener noreferrer"');
  expect(html).not.toMatch(/autoplay/i);
  expect(html).not.toMatch(/ on[a-z]+=/);
  expect(embedProjectionText(node('https://example.org/x.mp3', 'Ep'))).toBe('[audio: Ep]');
  for (const bad of ['', 'http://example.org/x.mp3', 'javascript:alert(1)', 'data:audio/mp3;base64,AAAA', 'https://u:p@example.org/x.mp3', 'https://example.org/a b.mp3', '//example.org/x.mp3']) {
    expect(isAudioURL(bad)).toBe(false);
    expect(renderEmbedBlockHTML(node(bad))).not.toContain('<audio');
  }
});
