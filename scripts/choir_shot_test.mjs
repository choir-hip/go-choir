// Failure modes of scripts/choir-shot.mjs, then one real run: node scripts/choir_shot_test.mjs
// The real run needs CHOIR_FRONTEND_DEPS and CHOIR_BROWSER (a capsule job has both)
// and is skipped without them.
import assert from 'node:assert/strict';
import { spawnSync } from 'node:child_process';
import { createHash } from 'node:crypto';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import vm from 'node:vm';
import {
  autoputerEnv, isProxied, missingDependencies, pickAutoputerBinary, parseArgs, plan, themeIdsFrom, themeSeedScript, themeAnswer,
  openApp, collectTextNodes, parseColor, over, resolveBackground, contrastRatio, requiredRatio, samplePixels, measureNode, summarizeContrast, contrastDir, USAGE, autoputerLogPath, verdictLine, resetDesktop, runShots, failFastFor, selectorTimeoutMessage,
  loadCatalogFixture, catalogAnswer, loadInstallFixture, installAnswer, isInstallPath, CATALOG_FIXTURE_HEADER, CATALOG_FIXTURE_BADGE, fixtureBadge, windowProblem, checkAppWindow, settleProblem,
  loadPodcastFixture, podcastIndex, podcastAnswer, unservedHostNote, podcastRoute, PODCAST_FIXTURE_DIR,
  ffmpegShimPlan, videoFileFor, pickVideoItem, videoSizeProblem, VIDEO_MAX_BYTES, VIDEO_MAX_MS,
} from './choir-shot.mjs';

const throws = (f, re) => assert.throws(f, re);
throws(() => parseArgs([]), /--shots FILE is required/);
throws(() => parseArgs(['--shots', 'x', '--preset', 'watch']), /--preset watch/);
throws(() => parseArgs(['--shots', 'x', '--size', '10']), /WIDTHxHEIGHT/);
throws(() => parseArgs(['--shots', 'x', '--size', '800x600', '--preset', 'mobile']), /exclusive/);
throws(() => parseArgs(['--shots', 'x', '--bogus']), /unknown flag/);
throws(() => parseArgs(['--shots']), /needs a value/);
// --help prints the usage and needs no --shots.
assert.equal(parseArgs(['--help']).help, true);
assert.equal(parseArgs(['--shots', 'x', '-h']).help, true);
assert.match(USAGE, /--shots FILE/);
const helpRun = spawnSync('node', [new URL('./choir-shot.mjs', import.meta.url).pathname, '--help'], { encoding: 'utf8' });
assert.equal(helpRun.status, 0, helpRun.stderr);
assert.match(helpRun.stdout, /usage: node scripts\/choir-shot.mjs/);
// The autoputer's output goes beside the screens directory (Freeze refuses non-PNG files in it).
assert.equal(autoputerLogPath('/var/lib/artifact/screens'), '/var/lib/artifact/shot-logs/autoputer.log');

// The run ends with one verdict line.
assert.equal(verdictLine(3, []), 'choir-shot: OK 3/3 shots');
assert.equal(verdictLine(3, [{ file: 'b.png', error: 'selector never appeared' }, { file: 'c.png', error: 'x' }]), 'choir-shot: FAILED 2/3: b.png: selector never appeared');
{
  // A run that fails before any shot (no CHOIR_* env) exits non-zero and says FAILED.
  const f = fs.mkdtempSync(path.join(os.tmpdir(), 'shot-verdict-')) + '/s.mjs';
  fs.writeFileSync(f, "export default [{ name: 'a' }];");
  const env2 = { ...process.env }; delete env2.CHOIR_FRONTEND_DEPS; delete env2.CHOIR_BROWSER;
  const r = spawnSync('node', [new URL('./choir-shot.mjs', import.meta.url).pathname, '--shots', f], { encoding: 'utf8', env: env2 });
  assert.notEqual(r.status, 0);
  assert.match(r.stderr, /^choir-shot: FAILED 0\/0: .*CHOIR_FRONTEND_DEPS/m);
}

const shots = [{ name: 'home' }, { name: 'detail', path: '/d' }];
throws(() => plan([], parseArgs(['--shots', 'x'])), /non-empty/);
throws(() => plan([{ name: '../x' }], parseArgs(['--shots', 'x'])), /letters, digits/);
throws(() => plan([{ name: 'a' }, { name: 'a' }], parseArgs(['--shots', 'x'])), /duplicate/);
throws(() => plan(shots, parseArgs(['--shots', 'x', '--only', 'nope'])), /no such shot/);
assert.deepEqual(plan(shots, parseArgs(['--shots', 'x'])).map((i) => i.file), ['home.png', 'detail.png']);
assert.deepEqual(plan(shots, parseArgs(['--shots', 'x', '--only', 'home', '--preset', 'mobile,desktop'])).map((i) => i.file), ['home-mobile.png', 'home.png']);
assert.deepEqual(plan(shots, parseArgs(['--shots', 'x', '--only', 'detail', '--size', '1000x700'])).map((i) => [i.file, i.viewport.width]), [['detail-1000x700.png', 1000]]);
// A shot named for a phone is taken only at a portrait size: crouton's engineer named
// shots "...-mobile" with no --preset mobile and got 1280x800 phone shots three times.
throws(() => plan([{ name: '11-phone-agenda' }], parseArgs(['--shots', 'x'])), /--preset mobile/);
throws(() => plan([{ name: 'day-mobile' }], parseArgs(['--shots', 'x', '--size', '1000x700'])), /--preset mobile/);
assert.deepEqual(plan([{ name: 'home' }, { name: 'phone-day' }], parseArgs(['--shots', 'x', '--preset', 'desktop,mobile'])).map((i) => i.file), ['home.png', 'home-mobile.png', 'phone-day-mobile.png']);
assert.deepEqual(plan([{ name: 'phone-day' }], parseArgs(['--shots', 'x', '--size', '390x844'])).map((i) => i.file), ['phone-day-390x844.png']);
assert.deepEqual(plan([{ name: 'microphone' }], parseArgs(['--shots', 'x'])).map((i) => i.file), ['microphone.png']);
// A shot's own preset or size sizes it; crouton's engineer set preset: 'mobile' on its
// phone shots and choir-shot ignored the field.
assert.deepEqual(plan([{ name: 'home' }, { name: 'phone-day', preset: 'mobile' }], parseArgs(['--shots', 'x'])).map((i) => [i.file, i.viewport.width]), [['home.png', 1280], ['phone-day-mobile.png', 390]]);
assert.deepEqual(plan([{ name: 'a', size: '400x900' }], parseArgs(['--shots', 'x', '--preset', 'desktop'])).map((i) => i.file), ['a-400x900.png']);
assert.deepEqual(plan([{ name: 'a', preset: ['tablet', 'mobile'] }], parseArgs(['--shots', 'x'])).map((i) => i.file), ['a-tablet.png', 'a-mobile.png']);
throws(() => plan([{ name: 'a', preset: 'phone' }], parseArgs(['--shots', 'x'])), /shot a: preset phone/);
throws(() => plan([{ name: 'a', size: '390' }], parseArgs(['--shots', 'x'])), /shot a: size 390/);
throws(() => plan([{ name: 'a', size: '390x844', preset: 'mobile' }], parseArgs(['--shots', 'x'])), /shot a: size and preset are exclusive/);

// The autoputer binary order, the proxy rule, the recipe env and the opt-out.
assert.equal(pickAutoputerBinary((f) => true), '/var/lib/artifact/release/bin/autoputer');
assert.equal(pickAutoputerBinary((f) => f === '/tmp/autoputer'), '/tmp/autoputer');
assert.equal(pickAutoputerBinary(() => false), '');
assert.ok(isProxied('/api/shell/bootstrap') && isProxied('/api'));
assert.ok(!isProxied('/apixyz') && !isProxied('/auth/session') && !isProxied('/assets/a.js'));
const env = autoputerEnv({});
assert.deepEqual([env.RUNTIME_STORE_PATH, env.AUTOPUTER_FILES_ROOT, env.AUTOPUTER_PORT, env.CHOIR_GUEST_TRUST_LOOPBACK], ['/computer/state', '/computer/files', '8082', '1']);
assert.equal(parseArgs(['--shots', 'x']).computer, true);
assert.equal(parseArgs(['--shots', 'x', '--no-computer']).computer, false);

// Themes: the shell's own preset list, argument parsing, naming, the seed and the route answer.
const known = themeIdsFrom(fs.readFileSync(new URL('../frontend/src/lib/theme.ts', import.meta.url), 'utf8'));
assert.ok(known.includes('futuristic-noir') && known.includes('aero-chrome-heart') && known.length >= 7, known.join());
assert.deepEqual(parseArgs(['--shots', 'x', '--theme', 'london-salmon,liquid-metal', '--contrast', '.win'], known).themes, ['london-salmon', 'liquid-metal']);
assert.equal(parseArgs(['--shots', 'x', '--contrast', '.win']).contrast, '.win');
throws(() => parseArgs(['--shots', 'x', '--theme', 'neon'], known), /unknown theme preset/);
throws(() => parseArgs(['--shots', 'x', '--theme', 'liquid-metal,liquid-metal'], known), /twice/);
throws(() => parseArgs(['--shots', 'x', '--theme', ','], known), /at least one/);
throws(() => plan([{ name: 'a', theme: 'neon' }], parseArgs(['--shots', 'x']), known), /unknown theme preset/);
{
  const o = parseArgs(['--shots', 'x', '--theme', 'london-salmon,liquid-metal', '--contrast', '.w'], known);
  const items = plan([{ name: 'a' }, { name: 'b', theme: 'vector-heart', contrast: '.b' }], o, known);
  assert.deepEqual(items.map((i) => i.file), ['a.london-salmon.png', 'a.liquid-metal.png', 'b.vector-heart.png']);
  assert.deepEqual(items.map((i) => i.contrastFile), ['a.london-salmon.contrast.json', 'a.liquid-metal.contrast.json', 'b.vector-heart.contrast.json']);
  assert.equal(items[2].contrast, '.b');
  const sized = plan([{ name: 'a' }], parseArgs(['--shots', 'x', '--theme', 'liquid-metal', '--preset', 'mobile,desktop'], known), known);
  assert.deepEqual(sized.map((i) => i.file), ['a.liquid-metal-mobile.png', 'a.liquid-metal.png']);
  assert.deepEqual(plan([{ name: 'a' }], parseArgs(['--shots', 'x', '--theme', 'liquid-metal', '--size', '1000x700'], known), known).map((i) => i.file), ['a.liquid-metal-1000x700.png']);
  assert.deepEqual(plan([{ name: 'a' }], parseArgs(['--shots', 'x'])).map((i) => [i.file, i.theme, i.contrastFile]), [['a.png', null, '']]);
}
{
  // The seed script, run as the page would, stores exactly what App.svelte's loadThemeBootCache parses.
  const store = {};
  vm.runInNewContext(themeSeedScript('london-salmon'), { window: { localStorage: { setItem: (k, v) => { store[k] = v; } } } });
  assert.deepEqual(Object.keys(store), ['choir.theme.boot.v2']);
  assert.deepEqual(JSON.parse(store['choir.theme.boot.v2']), { schema_version: 2, id: 'london-salmon' });
  vm.runInNewContext(themeSeedScript('x'), { window: {} }); // a throwing storage must not break the page
}
assert.deepEqual(themeAnswer('liquid-metal', 'GET', '/api/preferences/theme'), { theme: { schema_version: 2, id: 'liquid-metal' } });
assert.equal(themeAnswer('liquid-metal', 'PUT', '/api/preferences/theme'), null);
assert.equal(themeAnswer('liquid-metal', 'GET', '/api/preferences/other'), null);
assert.equal(themeAnswer(null, 'GET', '/api/preferences/theme'), null);

// The catalog fixture (docs/problems/capsule-library-catalog-404-2026-11-03.md):
// /api/catalog/* is the proxy's, so a capsule answers it from the fixture the
// Go test writes from corpusd's handler, labelled, never from the autoputer.
{
  const fx = loadCatalogFixture();
  assert.equal(fx.fixture, 'capsule-catalog');
  assert.match(fx.note, /^Not the live catalog/);
  assert.equal(CATALOG_FIXTURE_HEADER, 'x-choir-fixture');
  // Not a catalog path, or no fixture: not ours (the autoputer or a route answers).
  assert.equal(catalogAnswer(fx, 'GET', '/api/files', ''), null);
  assert.equal(catalogAnswer(fx, 'GET', '/api/catalogue', ''), null);
  assert.equal(catalogAnswer(null, 'GET', '/api/catalog/packages', ''), null);
  // The list: the fixture's answer, signed in as choir-shot's owner.
  const list = catalogAnswer(fx, 'GET', '/api/catalog/packages', '');
  assert.equal(list.status, 200);
  assert.equal(list.body.reader, 'owner');
  assert.ok(list.body.packages.length >= 6);
  assert.deepEqual([...new Set(list.body.packages.map((p) => p.package))].sort(), ['files-next', 'library-next', 'slides-next']);
  assert.equal(list.body.count, list.body.packages.length);
  // ?q= filters by name, as corpusd does (substring, case-insensitive).
  assert.equal(catalogAnswer(fx, 'GET', '/api/catalog/packages', '?q=SLIDES').body.count, list.body.packages.filter((p) => p.package === 'slides-next').length);
  const none = catalogAnswer(fx, 'GET', '/api/catalog/packages', '?q=nothing-by-this-name');
  assert.deepEqual([none.status, none.body.count, none.body.packages], [200, 0, []]);
  // History: the reader's own track by default, another's by ?track=.
  const own = catalogAnswer(fx, 'GET', '/api/catalog/packages/slides-next/releases', '');
  assert.equal(own.status, 200);
  assert.equal(own.body.track, 'owner');
  assert.ok(own.body.releases.some((r) => r.listing.state === 'withdrawn'));
  const otherTrack = list.body.packages.find((p) => p.package === 'files-next').track;
  const theirs = catalogAnswer(fx, 'GET', '/api/catalog/packages/files-next/releases', `?track=${otherTrack}`);
  assert.equal(theirs.body.track, otherTrack);
  assert.ok(theirs.body.releases.length >= 1);
  // A package the fixture lacks: an empty history, as corpusd answers (200).
  const empty = catalogAnswer(fx, 'GET', '/api/catalog/packages/library/releases', '');
  assert.deepEqual([empty.status, empty.body.package, empty.body.track, empty.body.releases, empty.body.count], [200, 'library', 'owner', [], 0]);
  // One release: by id (any case); an unknown or malformed id is 404 as at the proxy.
  const head = list.body.packages[0].head_release_id;
  const one = catalogAnswer(fx, 'GET', `/api/catalog/releases/${head.toUpperCase()}`, '');
  assert.equal(one.status, 200);
  assert.equal(one.body.release_id, head);
  assert.equal(catalogAnswer(fx, 'GET', `/api/catalog/releases/${'0'.repeat(64)}`, '').status, 404);
  assert.equal(catalogAnswer(fx, 'GET', '/api/catalog/releases/not-an-id', '').status, 404);
  assert.equal(catalogAnswer(fx, 'GET', '/api/catalog/elsewhere', '').status, 404);
  // Every head the list names resolves, so a Library can open each one.
  for (const p of list.body.packages) assert.equal(catalogAnswer(fx, 'GET', `/api/catalog/releases/${p.head_release_id}`, '').status, 200);
  // Media (docs/problems/library-releases-have-no-screenshots-2026-11-04.md):
  // every item a release names answers its bytes and type from the fixture's
  // media files; a digest the release does not name, or another release's, is
  // 404; releases with and without media, and one video, exist.
  {
    const withMedia = fx.releases.filter((r) => r.media.length > 0);
    const without = fx.releases.filter((r) => r.media.length === 0);
    assert.ok(withMedia.length > 0 && without.length > 0);
    assert.ok(fx.releases.some((r) => r.media.some((m) => m.kind === 'video' && m.media_type === 'video/webm')));
    assert.ok(list.body.packages.some((p) => p.head_media.length > 0) && list.body.packages.some((p) => p.head_media.length === 0));
    for (const r of withMedia) {
      for (const m of r.media) {
        assert.equal(m.url, `/api/catalog/releases/${r.release_id}/media/${m.sha256}`);
        const a = catalogAnswer(fx, 'GET', m.url, '');
        assert.equal(a.status, 200);
        assert.equal(a.type, m.media_type);
        assert.ok(Buffer.isBuffer(a.body) && a.body.length === m.bytes);
        assert.equal(createHash('sha256').update(a.body).digest('hex'), m.sha256);
      }
    }
    const item = withMedia[0].media[0];
    assert.equal(catalogAnswer(fx, 'GET', `/api/catalog/releases/${without[0].release_id}/media/${item.sha256}`, '').status, 404);
    assert.equal(catalogAnswer(fx, 'GET', `/api/catalog/releases/${withMedia[0].release_id}/media/${'0'.repeat(64)}`, '').status, 404);
    assert.equal(catalogAnswer(fx, 'GET', `/api/catalog/releases/${withMedia[0].release_id}/media/../../x`, '').status, 404);
  }
  // A listing act is the owner's on the real computer: refused, by reason.
  const act = catalogAnswer(fx, 'PATCH', `/api/catalog/records/${head}/listing`, '');
  assert.deepEqual([act.status, act.body.reason], [409, 'capsule_fixture']);
  // Any other write is 405, as at the proxy.
  assert.equal(catalogAnswer(fx, 'POST', '/api/catalog/packages', '').status, 405);
  // The label a shot carries when its page read the fixture.
  assert.match(CATALOG_FIXTURE_BADGE, /fixture/i);
  const appended = [];
  const el = { dataset: {}, style: {}, set textContent(v) { this.text = v; } };
  vm.runInNewContext(`(${fixtureBadge})(${JSON.stringify(CATALOG_FIXTURE_BADGE)})`, {
    document: { createElement: () => el, body: { appendChild: (n) => appended.push(n) } },
  });
  assert.equal(appended.length, 1);
  assert.equal(el.text, CATALOG_FIXTURE_BADGE);
  assert.equal(el.dataset.choirShotFixture, 'capsule-catalog');
  // At the top-left edge: never over the taskbar or prompt bar at the bottom,
  // nor over a window's controls at its top right.
  assert.equal(el.style.top, '2px');
  assert.equal(el.style.left, '8px');
  assert.equal(el.style.bottom, undefined);
  assert.equal(el.style.right, undefined);
}

// The install fixture: adoptions are the owner's act on the real computer, so a
// capsule answers them from the fixture the Go test writes from the real handler.
{
  const cfx = loadCatalogFixture();
  const ifx = loadInstallFixture();
  assert.equal(ifx.fixture, 'capsule-install');
  assert.match(ifx.note, /^Not the live computer/);
  const route = '/api/computers/computer-x/self-development/adoptions';
  assert.equal(isInstallPath(route), true);
  assert.equal(isInstallPath(`${route}/adoption-1`), true);
  assert.equal(isInstallPath('/api/computers/c/self-development/approve'), false);
  assert.equal(installAnswer(ifx, cfx, { adoptions: [] }, 'GET', '/api/files', ''), null);
  assert.equal(installAnswer(null, cfx, { adoptions: [] }, 'GET', route, ''), null);
  const refusedId = cfx.releases.filter((r) => r.record.package.name === ifx.refusal_package).sort((a, b) => a.published_at.localeCompare(b.published_at))[0].release_id;
  const okId = cfx.releases.find((r) => r.release_id !== refusedId && r.record.package.name !== ifx.refusal_package).release_id;
  const state = { adoptions: [] };
  const post = (id) => installAnswer(ifx, cfx, state, 'POST', route, JSON.stringify({ release_id: id, record: {} }));
  // Nothing installed yet.
  assert.deepEqual(installAnswer(ifx, cfx, state, 'GET', route, '').body, { adoptions: [] });
  // An install answers the real 201 view, with this release's fields; a repeat 200 and the same adoption.
  const made = post(okId);
  assert.equal(made.status, 201);
  assert.deepEqual(Object.keys(made.body).sort(), Object.keys(ifx.created.body).sort());
  assert.equal(made.body.release_id, okId);
  assert.equal(made.body.computer_id, 'computer-x');
  assert.match(made.body.adoption_id, /^install-[0-9a-f]{24}$/);
  const again = post(okId.toUpperCase());
  assert.deepEqual([again.status, again.body.adoption_id], [200, made.body.adoption_id]);
  assert.equal(state.adoptions.length, 1);
  // The list holds it; the one-read route finds it.
  const listed = installAnswer(ifx, cfx, state, 'GET', route, '');
  assert.deepEqual([listed.status, listed.body.adoptions.map((a) => a.release_id)], [200, [okId]]);
  assert.equal(installAnswer(ifx, cfx, state, 'GET', `${route}/${made.body.adoption_id}`, '').body.release_id, okId);
  assert.equal(installAnswer(ifx, cfx, state, 'GET', `${route}/adoption-nope`, '').status, 404);
  // A second install lists newest first.
  const other = cfx.releases.find((r) => r.release_id !== refusedId && r.release_id !== okId).release_id;
  post(other);
  assert.deepEqual(installAnswer(ifx, cfx, state, 'GET', route, '').body.adoptions.map((a) => a.release_id), [other, okId]);
  // The refusal release answers the real refusal and installs nothing.
  const before = state.adoptions.length;
  const refused = post(refusedId);
  assert.deepEqual([refused.status, refused.body.reason, Object.keys(refused.body).sort()], [403, 'cross_owner_before_frame', ['error', 'reason']]);
  assert.equal(state.adoptions.length, before);
  // Unknown, malformed and wrong-method requests are refused in the handler's words.
  assert.deepEqual([post('0'.repeat(64)).status, post('0'.repeat(64)).body.reason], [404, 'release_not_published']);
  assert.deepEqual([post('nope').status, post('nope').body.reason], [400, 'release_id_invalid']);
  assert.deepEqual([installAnswer(ifx, cfx, state, 'POST', route, 'not json').status, installAnswer(ifx, cfx, state, 'POST', route, 'not json').body.reason], [400, 'request_invalid']);
  assert.equal(installAnswer(ifx, cfx, state, 'DELETE', route, '').status, 405);
  // Per-shot state: choir-shot empties it with the desktop reset (an independent state starts empty).
  state.adoptions.length = 0;
  assert.deepEqual(installAnswer(ifx, cfx, state, 'GET', route, '').body, { adoptions: [] });
  // The source resets it where it resets the desktop, and tags every install answer.
  const src = fs.readFileSync(new URL('./choir-shot.mjs', import.meta.url), 'utf8');
  assert.match(src, /installState\.adoptions\.length = 0;[^\n]*\n\s*if \(useComputer\) await resetDesktop\(base\)/);
  assert.match(src, /\[CATALOG_FIXTURE_HEADER\]: 'capsule-install'/);
}

// Contrast: colors, the background walk, ratios and pixel fallback.
assert.deepEqual(parseColor('rgba(10, 20, 30, 0.5)'), { r: 10, g: 20, b: 30, a: 0.5 });
assert.deepEqual(parseColor('rgb(10 20 30 / 50%)'), { r: 10, g: 20, b: 30, a: 0.5 });
assert.equal(parseColor('oklch(0.5 0.1 20)'), null);
const white = { r: 255, g: 255, b: 255, a: 1 }, black = { r: 0, g: 0, b: 0, a: 1 };
assert.ok(Math.abs(contrastRatio(black, white) - 21) < 1e-9);
assert.equal(requiredRatio(14, '400'), 4.5);
assert.equal(requiredRatio(24, '400'), 3);
assert.equal(requiredRatio(19, '700'), 3);
{
  const half = over({ r: 0, g: 0, b: 0, a: 0.5 }, white);
  assert.ok(Math.abs(half.r - 127.5) < 1e-9 && half.a === 1);
  // translucent black over translucent white over an opaque red
  const r = resolveBackground([
    { color: { r: 0, g: 0, b: 0, a: 0.5 }, image: false },
    { color: { r: 255, g: 255, b: 255, a: 0.5 }, image: false },
    { color: { r: 255, g: 0, b: 0, a: 1 }, image: false },
    { color: { r: 0, g: 0, b: 255, a: 1 }, image: false }, // below the first opaque layer: ignored
  ]);
  assert.equal(r.kind, 'opaque');
  assert.deepEqual([r.color.r, r.color.g, r.color.b].map(Math.round), [128, 64, 64]); // white at .5 over red, then black at .5
  assert.equal(resolveBackground([{ color: { r: 0, g: 0, b: 0, a: 0 }, image: false }, { color: white, image: false }]).color.r, 255); // transparent skipped
  assert.equal(resolveBackground([{ color: { r: 9, g: 9, b: 9, a: 0.3 }, image: false }, { color: white, image: true }]).kind, 'pixel'); // image above the opaque layer
  assert.equal(resolveBackground([{ color: white, image: true }]).reason, 'image background');
  assert.equal(resolveBackground([{ color: { r: 9, g: 9, b: 9, a: 0.3 }, image: false }, { color: { r: 0, g: 0, b: 0, a: 0 }, image: false }]).kind, 'pixel'); // reaches the root: never black
  assert.equal(resolveBackground([{ color: null, image: false }]).kind, 'pixel');
}
{
  const png = { width: 4, height: 2, data: new Uint8Array(4 * 2 * 4) };
  for (let i = 0; i < 8; i++) { png.data.set(i < 4 ? [255, 255, 255, 255] : [30, 30, 30, 255], i * 4); }
  assert.equal(samplePixels(png, { x: 0, y: 0, width: 4, height: 2 }, 100).length, 8);
  assert.equal(samplePixels(png, { x: 10, y: 0, width: 4, height: 2 }).length, 0);
  const node = (layers, rect = { x: 0, y: 0, width: 4, height: 2 }, color = 'rgb(255, 255, 255)') => ({ path: 'p', text: 't', color, fontSize: 14, fontWeight: '400', rect, layers });
  const grad = [{ bg: 'rgba(0, 0, 0, 0)', image: true }];
  const m = measureNode(node(grad), png);
  assert.ok(m.ratio < 1.5 && /^pixels/.test(m.method), JSON.stringify(m)); // white text over the white half: the lowest sample counts
  assert.equal(measureNode(node(grad), null).method, 'unmeasured: image background');
  assert.equal(measureNode(node(grad), null).ratio, null);
  const op = measureNode(node([{ bg: 'rgb(0, 0, 0)', image: false }]), null);
  assert.ok(op.ratio > 20 && op.method === 'opaque-ancestor');
  const { report, line } = summarizeContrast({ file: 'a.png' }, [node([{ bg: 'rgb(0, 0, 0)', image: false }]), node([{ bg: 'rgb(250, 250, 250)', image: false }])], null);
  assert.equal(report.failing.length, 1);
  assert.equal(report.min_ratio, 1.04);
  assert.match(line, /min 1\.04 over 2\/2 text nodes, 1 failing, 0 unmeasured/);
}

// --open: parsing, the per-shot field, and the steps over a fake page.
assert.equal(parseArgs(['--shots', 'x', '--open', 'image']).open, 'image');
throws(() => parseArgs(['--shots', 'x', '--open', '../x']), /--open/);
throws(() => plan([{ name: 'a', open: 'a"b' }], parseArgs(['--shots', 'x'])), /shot a: open/);
assert.deepEqual(plan([{ name: 'a' }, { name: 'b', open: 'slides' }], parseArgs(['--shots', 'x', '--open', 'image'])).map((i) => i.open), ['image', 'slides']);
assert.deepEqual(plan([{ name: 'a' }], parseArgs(['--shots', 'x'])).map((i) => i.open), ['']);
{
  // A fake page: the launcher lists entries; clicking one adds its window.
  const fakePage = ({ button = true, sheet = true, entries = [{ id: 'image', title: 'Image' }, { id: 'slides', title: 'Slides' }], window = true, ready = true } = {}) => {
    const clicks = [];
    const state = { sheetOpen: false, windows: new Set() };
    const page = {
      clicks,
      locator(sel) {
        const loc = {
          first: () => loc,
          async waitFor({ state: st }) {
            const ok = sel.startsWith('[data-desktop]') ? ready : sel === '[data-desk-menu-button]' ? button : sel === '[data-desk-sheet]' ? sheet && state.sheetOpen : state.windows.has(sel);
            if (!ok) throw new Error('timeout');
            assert.equal(st, 'visible');
          },
          async click() {
            clicks.push(sel);
            if (sel === '[data-desk-menu-button]') state.sheetOpen = true;
            const m = /data-desk-app-id="([^"]+)"/.exec(sel);
            if (m && window) state.windows.add(`[data-window][data-window-app-id="${m[1]}"]`);
          },
          async evaluateAll(fn) {
            assert.equal(sel, '[data-desk-sheet-app]');
            return fn(entries.map((e) => ({ getAttribute: () => e.id, querySelector: () => ({ textContent: ` ${e.title} ` }) })));
          },
        };
        return loc;
      },
    };
    return page;
  };
  let p = fakePage();
  assert.equal(await openApp(p, 'image', 5), 'image');
  assert.deepEqual(p.clicks, ['[data-desk-menu-button]', '[data-desk-sheet-app][data-desk-app-id="image"]']);
  assert.equal(await openApp(fakePage(), 'Slides', 5), 'slides'); // by title, any case
  await assert.rejects(openApp(fakePage({ ready: false }), 'image', 5), /desktop connecting.*did not become visible/);
  await assert.rejects(openApp(fakePage({ button: false }), 'image', 5), /app switcher button.*did not become visible/);
  await assert.rejects(openApp(fakePage({ sheet: false }), 'image', 5), /launcher sheet/);
  await assert.rejects(openApp(fakePage(), 'trace', 5), /no launcher entry.*has image, slides/);
  await assert.rejects(openApp(fakePage({ window: false }), 'image', 5), /image app window/);
}

// Contrast failure records: the stable selector, the snippet and the scope, over a fake DOM.
{
  const mk = (tag, { id = '', cls = [], attrs = {}, text = '', parent = null, kids = [] } = {}) => {
    const el = {
      nodeType: 1, tagName: tag.toUpperCase(), id, classList: cls, attributes: Object.entries(attrs).map(([name, value]) => ({ name, value })),
      parentElement: parent, children: kids,
      childNodes: text ? [{ nodeType: 3, textContent: text }] : [],
      closest(sel) { const m = /^\[data-window\]\[data-window-app-id="([^"]+)"\]$/.exec(sel); for (let e = el; e; e = e.parentElement) if (m ? e.attributes.some((a) => a.name === 'data-window-app-id' && a.value === m[1]) : sel === '[data-window-content]' && e.attributes.some((a) => a.name === 'data-window-content')) return e; return null; },
      getBoundingClientRect: () => ({ x: 0, y: 0, width: 10, height: 10 }),
      querySelectorAll() { const out = []; const walk = (e) => { for (const k of e.children) { out.push(k); walk(k); } }; walk(el); return out; },
    };
    for (const k of kids) k.parentElement = el;
    return el;
  };
  const badge = mk('span', { cls: ['badge', 'warn', 'x'], text: 'Draft' });
  const title = mk('span', { cls: ['titltexture'], text: 'Image' });
  const bar = mk('div', { cls: ['titlebar'], attrs: { 'data-window-titlebar': '' }, kids: [title] });
  const body = mk('div', { cls: ['window-content'], attrs: { 'data-window-content': '' }, kids: [mk('div', { cls: ['image-content'], attrs: { 'data-image-window': '' }, kids: [mk('div', { cls: ['row'], kids: [badge] })] })] });
  const win = mk('section', { cls: ['window'], attrs: { 'data-window': '', 'data-window-id': 'w-123', 'data-window-app-id': 'image' }, kids: [bar, body] });
  const root = mk('div', { id: 'app', kids: [win] });
  globalThis.document = { documentElement: mk('html'), querySelectorAll: () => [root] };
  globalThis.window = { scrollX: 0, scrollY: 0 };
  globalThis.getComputedStyle = () => ({ display: 'block', visibility: 'visible', color: 'rgb(0,0,0)', fontSize: '14px', fontWeight: '400', backgroundColor: 'rgb(255,255,255)', backgroundImage: 'none', backdropFilter: 'none' });
  try {
    const nodes = collectTextNodes({ selector: '#app', limit: 50, appId: 'image' });
    const byText = Object.fromEntries(nodes.map((n) => [n.text, n]));
    assert.equal(byText.Draft.scope, 'package-window');
    assert.equal(byText.Draft.path, 'div[data-image-window] > div.row > span.badge.warn');
    assert.equal(byText.Image.scope, 'shell-chrome'); // the window's own title bar is chrome, not the package
    assert.equal(byText.Image.path, 'div[data-window-titlebar] > span.titltexture');
    // The same nodes with another package opened are shell chrome.
    const other = collectTextNodes({ selector: '#app', limit: 50, appId: 'slides' });
    assert.ok(other.every((n) => n.scope === 'shell-chrome'));
    assert.ok(collectTextNodes({ selector: '#app', limit: 50 }).every((n) => n.scope === 'unscoped'));
    // The failing record carries selector, text, scope.
    const low = nodes.map((n) => ({ ...n, color: 'rgb(250,250,250)', layers: [{ bg: 'rgb(255,255,255)', image: false }] }));
    const { report, line, detail } = summarizeContrast({ file: 'a.png', opened: 'image' }, low, null);
    assert.deepEqual(Object.keys(report.failing[0]).sort(), ['method', 'ratio', 'required', 'scope', 'selector', 'text']);
    assert.equal(report.failing.find((f) => f.text === 'Draft').scope, 'package-window');
    assert.match(line, /2 failing \(1 shell-chrome, 1 package-window\)|2 failing \(1 package-window, 1 shell-chrome\)/);
    assert.ok(detail.some((d) => /package-window .* div\[data-image-window\] > div\.row > span\.badge/.test(d)), detail.join('\n'));
    assert.equal(report.opened, 'image');
  } finally {
    delete globalThis.document; delete globalThis.window; delete globalThis.getComputedStyle;
  }
}
assert.equal(contrastDir('/var/lib/artifact/screens'), '/var/lib/artifact/contrast');

// A locked tree that lacks a package.json dependency is named, not built against.
{
  const d = fs.mkdtempSync(path.join(os.tmpdir(), 'shot-deps-'));
  fs.mkdirSync(path.join(d, 'node_modules', 'have'), { recursive: true });
  fs.writeFileSync(path.join(d, 'node_modules', 'have', 'package.json'), '{}');
  fs.mkdirSync(path.join(d, 'fe'));
  fs.writeFileSync(path.join(d, 'fe', 'package.json'), JSON.stringify({ dependencies: { have: '1', gone: '1' }, devDependencies: { alsogone: '1' } }));
  assert.deepEqual(missingDependencies(path.join(d, 'fe'), path.join(d, 'node_modules')), ['gone', 'alsogone']);
}

if (process.env.CHOIR_FRONTEND_DEPS && process.env.CHOIR_BROWSER) {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'shot-test-'));
  const file = path.join(dir, 'shots.mjs');
  fs.writeFileSync(file, `export default [
    { name: 'shell', selector: 'body' },
    { name: 'broken', selector: '#does-not-exist-anywhere' },
  ];`);
  const out = path.join(dir, 'screens');
  const run = (...a) => spawnSync('node', [new URL('./choir-shot.mjs', import.meta.url).pathname, '--shots', file, '--out', out, ...a], { encoding: 'utf8' });
  // Problem 18: a stray frontend/node_modules (a .vite cache dir, a half install)
  // must not hide the locked dependencies from the build, and must be left alone.
  const fe = new URL('../frontend/', import.meta.url).pathname;
  const stray = path.join(fe, 'node_modules');
  const hadStray = fs.existsSync(stray);
  if (!hadStray) fs.mkdirSync(path.join(stray, '.vite'), { recursive: true });
  const one = run('--only', 'shell', '--preset', 'mobile');
  try {
    assert.equal(one.status, 0, one.stdout + one.stderr);
    assert.ok(fs.existsSync(path.join(stray, '.vite')) || hadStray, 'the helper changed the workspace node_modules');
  } finally {
    if (!hadStray) fs.rmSync(stray, { recursive: true, force: true });
  }
  assert.deepEqual(fs.readdirSync(out), ['shell-mobile.png']);
  assert.equal(fs.readFileSync(path.join(out, 'shell-mobile.png')).subarray(1, 4).toString(), 'PNG');
  // A shot whose selector never appears fails alone, names itself, and the others are still taken.
  const both = run();
  assert.equal(both.status, 1);
  assert.match(both.stdout, /^choir-shot: shot 2\/2 FAILED broken\.png: /m);
  assert.match(both.stdout, /^choir-shot: shot 1\/2 OK shell\.png /m);
  assert.match(both.stdout.trimEnd().split('\n').pop(), /^choir-shot: FAILED 1\/2: broken\.png: /);
  assert.match(one.stdout, /^choir-shot: OK 1\/1 shots$/m);
  assert.ok(fs.existsSync(path.join(out, 'shell.png')));
  console.log('choir_shot_test: real run ok');
} else {
  console.log('choir_shot_test: no CHOIR_FRONTEND_DEPS/CHOIR_BROWSER; skipped the real run');
}
console.log('choir_shot_test: ok');

// Per-shot lines and --fail-fast. A 12-shot run takes about 5 minutes; the first failure
// used to show only in the final line.
{
  assert.equal(parseArgs(['--shots', 'x']).failFast, undefined);
  assert.equal(parseArgs(['--shots', 'x', '--fail-fast']).failFast, true);
  assert.equal(parseArgs(['--shots', 'x', '--keep-going']).failFast, false);
  throws(() => parseArgs(['--shots', 'x', '--fail-fast', '--keep-going']), /exclusive/);
  // A full run (no --only) stops at the first failure by default; an --only run does not.
  assert.equal(failFastFor(parseArgs(['--shots', 'x'])), true);
  assert.equal(failFastFor(parseArgs(['--shots', 'x', '--only', 'a'])), false);
  assert.equal(failFastFor(parseArgs(['--shots', 'x', '--keep-going'])), false);
  assert.equal(failFastFor(parseArgs(['--shots', 'x', '--only', 'a', '--fail-fast'])), true);

  const items = ['a.png', 'b.png', 'c.png', 'd.png'].map((file) => ({ file }));
  const mk = (bad) => async (it) => { if (bad.includes(it.file)) throw new Error(`boom ${it.file}\nstack`); };
  const lines = [];
  const log = (l) => lines.push(l);
  let r = await runShots(items, mk(['b.png', 'd.png']), { failFast: false, log });
  assert.deepEqual(r.failures.map((f) => f.file), ['b.png', 'd.png']);
  assert.equal(r.skipped, 0);
  assert.match(lines[0], /^choir-shot: shot 1\/4 OK a\.png \d+\.\d s$/);
  assert.equal(lines[1], 'choir-shot: shot 2/4 FAILED b.png: boom b.png');
  assert.equal(lines.length, 4); // one line per shot, as each finishes
  lines.length = 0;
  r = await runShots(items, mk(['b.png', 'd.png']), { failFast: true, log });
  assert.deepEqual(r.failures.map((f) => f.file), ['b.png']);
  assert.equal(r.skipped, 2);
  assert.equal(lines.length, 3);
  assert.match(lines[2], /^choir-shot: --fail-fast: stopped after b\.png; 2 shots not run \(--keep-going takes them all\)$/);
  assert.equal(verdictLine(4, r.failures, r.skipped), 'choir-shot: FAILED 1/4: b.png: boom b.png (2 not run: fail-fast)');
  assert.equal(verdictLine(4, [], 0), 'choir-shot: OK 4/4 shots');
}

// Each shot starts from a clean desktop: the reset PUTs an empty window list as the driver.
{
  const calls = [];
  await resetDesktop('http://h:1', async (url, init) => { calls.push({ url, init }); return { ok: true, status: 200 }; });
  assert.equal(calls.length, 1);
  assert.equal(calls[0].url, 'http://h:1/api/desktop/state');
  assert.equal(calls[0].init.method, 'PUT');
  assert.deepEqual(JSON.parse(calls[0].init.body), { windows: [], active_window_id: '', driver: true });
  await assert.rejects(resetDesktop('http://h:1', async () => ({ ok: false, status: 500 })), /desktop reset failed \(500\)/);
}

// A shot that opens an app fails when no visible, sized, unminimized window of it is on screen.
assert.equal(windowProblem('library-next', [{ visible: true, minimized: false, w: 900, h: 600 }], true), '');
assert.equal(settleProblem('x', [{ visible: true, opacity: 1, running: 0, ancestorRunning: 0 }]), '');
assert.match(settleProblem('x', [{ visible: true, opacity: 0.1, running: 1, ancestorRunning: 0 }]), /did not settle within 3 s \(opacity 0.1, 1 running/);
assert.match(windowProblem('library-next', [], true), /no window for library-next on screen at capture; no window element; taskbar chip present/);
assert.match(windowProblem('library-next', [{ visible: false, minimized: true, w: 0, h: 0 }], true), /minimized/);
assert.match(windowProblem('library-next', [{ visible: true, minimized: false, w: 0, h: 40 }], false), /0x40.*chip absent/);
{
  const page = (r) => ({ evaluate: async () => r });
  await checkAppWindow(page({ wins: [{ visible: true, minimized: false, w: 5, h: 5 }], chip: true }), 'x');
  await assert.rejects(checkAppWindow(page({ wins: [], chip: true }), 'library-next'), /no window for library-next on screen at capture/);
}
console.log('choir-shot: no-window checks ok');

// A no-window failure (checkAppWindow throws) takes the per-shot FAILED line and counts for --fail-fast.
{
  const lines = [];
  const page = { evaluate: async () => ({ wins: [], chip: true }) };
  const r = await runShots([{ file: 'x.png' }, { file: 'y.png' }], async (it) => { if (it.file === 'x.png') await checkAppWindow(page, 'library-next'); }, { failFast: true, log: (l) => lines.push(l) });
  assert.equal(r.failures.length, 1);
  assert.equal(r.skipped, 1);
  assert.match(lines[0], /^choir-shot: shot 1\/2 FAILED x\.png: no window for library-next/);
}

// A selector that never appears fails the shot naming the selector and the page text, and counts for --fail-fast.
{
  const long = 'x'.repeat(300);
  const page = { evaluate: async () => `Hello   world ${long}` };
  const m = await selectorTimeoutMessage(page, '#nope', new Error('page.waitForSelector: Timeout 15000ms exceeded.'));
  assert.match(m, /^selector "#nope" never appeared within 15 s: check the selector matches the app's DOM; the page shows: Hello world x+$/);
  assert.ok(m.split('the page shows: ')[1].length <= 200);
  const lines = [];
  const r = await runShots([{ file: 'x.png' }, { file: 'y.png' }], async () => { throw new Error(m); }, { failFast: true, log: (l) => lines.push(l) });
  assert.equal(r.skipped, 1);
  assert.match(lines[0], /FAILED x\.png: selector "#nope" never appeared/);
  assert.match(await selectorTimeoutMessage({ evaluate: async () => '' }, '.a', new Error('Timeout')), /\(no visible text\)/);
}
console.log('choir-shot: selector timeout message ok');

// --video: one recorded shot, a webm beside the PNGs, ffmpeg found through a private browsers-path shim.
{
  assert.equal(parseArgs(['--shots', 'x']).video, false);
  assert.equal(parseArgs(['--shots', 'x', '--video']).video, true);
  assert.equal(parseArgs(['--shots', 'x', '--video', '--video-shot', 'a']).videoShot, 'a');
  throws(() => parseArgs(['--shots', 'x', '--video-shot', 'a']), /--video-shot needs --video/);
  assert.match(USAGE, /--video/);
  assert.equal(VIDEO_MAX_BYTES, 8 << 20);
  assert.equal(VIDEO_MAX_MS, 20000);
  assert.equal(videoFileFor('library-home.png'), 'library-home.webm');
  assert.equal(videoFileFor('a.london-salmon-mobile.png'), 'a.london-salmon-mobile.webm');
  throws(() => videoFileFor('../x.png'), /not a valid video name/);
  throws(() => videoFileFor(`${'a'.repeat(100)}.png`), /not a valid video name/);
  assert.deepEqual(ffmpegShimPlan('1011', '/nix/store/x/bin/ffmpeg', 'linux'), { dir: 'ffmpeg-1011', links: [{ name: 'ffmpeg-linux', target: '/nix/store/x/bin/ffmpeg' }] });
  assert.deepEqual(ffmpegShimPlan('1011', '/f', 'darwin').links.map((l) => l.name), ['ffmpeg-linux', 'ffmpeg-mac']);
  throws(() => ffmpegShimPlan('../1', '/f', 'linux'), /revision/);
  throws(() => ffmpegShimPlan('1011', 'relative/ffmpeg', 'linux'), /absolute/);
  const two = plan([{ name: 'a' }, { name: 'b' }], parseArgs(['--shots', 'x']));
  assert.equal(pickVideoItem(two, parseArgs(['--shots', 'x', '--video'])).item.file, 'a.png');
  assert.equal(pickVideoItem(two, parseArgs(['--shots', 'x', '--video', '--video-shot', 'b'])).item.file, 'b.png');
  throws(() => pickVideoItem(two, parseArgs(['--shots', 'x', '--video', '--video-shot', 'zz'])), /--video-shot zz: no such planned shot/);
  assert.equal(pickVideoItem(two, parseArgs(['--shots', 'x'])), null);
  const flagged = plan([{ name: 'a' }, { name: 'b', video: true }, { name: 'c', video: true }], parseArgs(['--shots', 'x']));
  assert.equal(pickVideoItem(flagged, parseArgs(['--shots', 'x'])), null); // a shot's video: true still needs --video
  const r = pickVideoItem(flagged, parseArgs(['--shots', 'x', '--video']));
  assert.equal(r.item.file, 'b.png');
  assert.match(r.warnings[0], /c\.png also sets video: true; only b\.png is recorded/);
  assert.equal(pickVideoItem(flagged, parseArgs(['--shots', 'x', '--video', '--video-shot', 'a'])).item.file, 'a.png');
  assert.equal(videoSizeProblem(8 << 20), '');
  assert.match(videoSizeProblem((8 << 20) + 1), /8388609 bytes.*8388608-byte cap/);
}
console.log('choir-shot: video helpers ok');

// Podcast fixture: artwork and audio hosts a capsule cannot reach are answered from files.
{
  const fx = loadPodcastFixture();
  assert.ok(fx && fx.shows.length >= 3, 'the podcast fixture loads');
  const idx = podcastIndex(fx);
  for (const show of fx.shows) {
    for (const u of [show.feed_url, ...show.artwork.map((a) => a.url), ...show.episodes]) assert.ok(new URL(u).hostname.endsWith('.invalid'), `${u}: a fixture host is reserved .invalid, never a real podcast host`);
    assert.ok(show.artwork.length > 0 && show.episodes.length > 0, show.slug);
    assert.ok(!/[\w.+-]+@[\w-]+\.\w/.test(fs.readFileSync(path.join(PODCAST_FIXTURE_DIR, show.feed_file), 'utf8')), `${show.slug}: no email address in the feed`);
    for (const a of show.artwork) {
      const r = podcastAnswer(idx, a.url);
      assert.equal(r.status, 200); assert.equal(r.headers['content-type'], 'image/jpeg');
      assert.ok(r.body.length > 1000 && r.body.length < 40000, 'small artwork');
    }
  }
  // failure mode 5: a query-string variant of an artwork URL still matches.
  const art = fx.shows[0].artwork[0].url;
  assert.equal(podcastAnswer(idx, art.replace(/\?.*$/, '') + '?x=1').status, 200);
  assert.equal(podcastAnswer(idx, 'https://example.com/none.jpg'), null);
  assert.equal(podcastAnswer(null, art), null);
  // failure mode 3: audio answers Range with 206, a suffix range, and 416 past the end.
  const ep = fx.shows[0].episodes[0];
  const full = podcastAnswer(idx, ep);
  assert.equal(full.status, 200); assert.equal(full.headers['accept-ranges'], 'bytes');
  const part = podcastAnswer(idx, ep, 'bytes=10-19');
  assert.equal(part.status, 206); assert.equal(part.body.length, 10);
  assert.equal(part.headers['content-range'], `bytes 10-19/${full.body.length}`);
  assert.equal(podcastAnswer(idx, ep, 'bytes=-5').body.length, 5);
  assert.equal(podcastAnswer(idx, ep, `bytes=${full.body.length + 5}-`).status, 416);
  // failure mode 4: an unmapped image or audio host is logged once; other resources are not.
  const seen = new Set();
  assert.match(unservedHostNote(seen, 'https://ssl-static.libsyn.com/p/a.jpg', 'image'), /ssl-static\.libsyn\.com is not reachable/);
  assert.equal(unservedHostNote(seen, 'https://ssl-static.libsyn.com/p/b.jpg', 'image'), '');
  assert.match(unservedHostNote(seen, 'https://traffic.libsyn.com/e.mp3', 'fetch'), /traffic\.libsyn\.com/);
  assert.equal(unservedHostNote(seen, 'https://api.example.com/x', 'fetch'), '');
  assert.equal(unservedHostNote(seen, 'data:image/png;base64,xx', 'image'), '');
  // failures 1, 2: the route fulfills a manifest URL, passes same-origin, logs an unmapped image once.
  const logs = [];
  const state = { served: false, seen: new Set() };
  const h = podcastRoute(fx, 'http://127.0.0.1:4000', state, PODCAST_FIXTURE_DIR, (m) => logs.push(m));
  const fake = (url, type = 'image', headers = {}) => { const o = { out: '' }; o.route = { request: () => ({ url: () => url, resourceType: () => type, headers: () => headers }), fulfill: async (r) => { o.out = 'fulfill'; o.r = r; }, fallback: async () => { o.out = 'fallback'; } }; return o; };
  let c = fake(art); await h(c.route); assert.equal(c.out, 'fulfill'); assert.equal(state.served, true);
  c = fake(ep, 'media', { range: 'bytes=0-9' }); await h(c.route); assert.equal(c.r.status, 206);
  c = fake('http://127.0.0.1:4000/app.js', 'script'); await h(c.route); assert.equal(c.out, 'fallback');
  c = fake('https://static.megaphone.fm/x.jpg'); await h(c.route); assert.equal(c.out, 'fallback');
  c = fake('https://static.megaphone.fm/y.jpg'); await h(c.route); assert.equal(logs.length, 1);
  // the helper wires the route before a shot's routes(), so a shot's own route wins.
  const src = fs.readFileSync(new URL('./choir-shot.mjs', import.meta.url), 'utf8');
  assert.ok(src.indexOf("context.route('**/*', podcastRoute") < src.indexOf('await s.routes(context, base)'));
  assert.match(USAGE, /scripts\/fixtures\/capsule-podcast/);
}
console.log('choir-shot: podcast fixture ok');
