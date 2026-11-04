#!/usr/bin/env node
// choir-shot: the screenshot helper an engineer runs in its capsule.
//
// It is a tool the engineer runs, not a platform stage: nothing in Freeze,
// verify, judge or acceptance calls it, waits for it or refuses for its absence
// (docs/problems/selfdev-screens-are-not-files-2026-11-03.md: "no platform
// capture"). It exists because an engineer wrote its own shots.mjs, re-emitted
// it up to 14 times, and ran the whole set after every edit for 60 to 275 s a
// run (docs/evidence/selfdev-round-cycle-time-2026-11-03.md).
//
// What it does: builds the frontend once (cached by a hash of the sources, in
// /tmp), serves it, opens the store Chromium ($CHOIR_BROWSER) with the pinned
// Playwright from $CHOIR_FRONTEND_DEPS, and takes the named shots, waiting for
// network idle, fonts and each shot's selector (never a fixed sleep). PNGs go
// to /var/lib/artifact/screens/, which Freeze saves as files Texture embeds.
//
//   node /workspace/platform/scripts/choir-shot.mjs --shots /tmp/shots.mjs
//   node ... --shots /tmp/shots.mjs --only slides-home            # one shot
//   node ... --shots /tmp/shots.mjs --only a,b --preset mobile    # two, phone
//   node ... --shots /tmp/shots.mjs --size 1000x700               # custom size
//   node ... --shots /tmp/shots.mjs --list
//   node ... --shots /tmp/shots.mjs --theme futuristic-noir,london-salmon   # presets
//   node ... --shots /tmp/shots.mjs --contrast '.window'                    # WCAG
//   node ... --shots /tmp/shots.mjs --open image                            # a package app
//
// --open <app> (or a shot's `open` field) opens a package app in the shell
// before the shot, the way a person does: it clicks the app switcher
// ([data-desk-menu-button]), then the app's launcher entry
// ([data-desk-sheet-app][data-desk-app-id=<id>], matched by id, else by the
// title shown on the entry), and waits for the app window
// ([data-window][data-window-app-id=<id>]). A step that does not happen fails
// the shot and names what was missing (no switcher button, no such launcher
// entry with the ids it does have, no window). The shot's selector is waited
// for after the open, and `act` runs after it. A route-stubbed placeholder is
// not the app; this opens the real one.
//
// A contrast failure names its element: the nearest stable selector (an id or a
// data-* attribute on the node or its closest ancestor, then the class path
// below it), a text snippet, and `scope`: "package-window" inside the opened
// app's window content ([data-window-content]), "shell-chrome" outside it (title bar, tray, desk), or
// "unscoped" when nothing was opened. Fix a failure only in its own scope.
//
// --theme a,b (or a shot's `theme` field, a preset id or a list) sets the shell
// theme without the Settings window: it seeds localStorage 'choir.theme.boot.v2'
// ({"schema_version":2,"id":"<preset>"}, what App.svelte reads at first paint)
// through context.addInitScript, and the helper's own server answers
// GET /api/preferences/theme with that preset before it proxies. Each shot is
// taken once per preset, as <name>.<preset>.png (<name>.<preset>-<view>.png for
// a preset or size). Preset ids come from frontend/src/lib/theme.ts; an unknown
// one fails before anything is built.
//
// --contrast <selector> (or a shot's `contrast` field) measures, after the shot
// is ready, the WCAG contrast ratio of every visible text element under the
// selector against its real background: it walks up to the first ancestor whose
// background-color is opaque and has no background-image, compositing the
// translucent layers above it. At a background-image, a backdrop-filter, a
// color it cannot read or the root with no opaque layer it reads the pixels
// under the element's box from a screenshot taken with all text made
// transparent (the lowest ratio over the sampled pixels counts), decoded with
// pngjs from the locked dependency tree; without pngjs the node is reported
// "unmeasured: image background", never measured against black. It writes
// <shot file stem>.contrast.json (min ratio, failing nodes with selector path,
// text and ratio, and the method per node) to contrastDir(out), that is
// /var/lib/artifact/contrast beside the screens directory and not in it:
// choir.Freeze refuses any file in screens/ that is not a plain .png name
// (internal/capsule/screens.go screenNames). It prints one line per shot.
// Text under 24 px (18.66 px bold) must reach 4.5, larger text 3. Ancestor
// opacity below 1 is not modelled.
//
// The shots file is an ES module whose default export is a list of
//   { name, path = '/', selector, routes(context, base), before(page, base),
//     act(page), fullPage = false }
// name: letters, digits, '.', '-', '_'. routes mocks the backend
// (context.route) for signed-in views. before runs before the page is opened
// (seed storage with context.addInitScript through page.context()); act runs
// after it is ready (click, type), and the helper waits for idle again.
// Files: <name>.png for the desktop preset, <name>-<preset>.png otherwise,
// <name>-<W>x<H>.png for --size. Presets: desktop 1280x800, tablet 820x1180,
// mobile 390x844. Run single shots while iterating; run the full set once
// before choir.Freeze (a change no one looks at needs no shot).
// Each shot starts from a clean desktop (resetDesktop, before the page opens);
// one shot is one page load plus its single act, so there is no between-step reset.
//
// Real content: when /computer/state.db exists (an implementation capsule's
// private copy of the computer's store), the helper starts an autoputer itself
// over /computer (RUNTIME_STORE_PATH=/computer/state AUTOPUTER_FILES_ROOT=
// /computer/files AUTOPUTER_PORT=8082 CHOIR_GUEST_TRUST_LOOPBACK=1) as a child
// of this process, so it lives exactly as long as the shots, and kills it at
// exit. The binary is /var/lib/artifact/release/bin/autoputer, else
// /tmp/autoputer, else built once with go build from /workspace/platform. The
// served frontend proxies /api/* to it with X-Authenticated-User: owner and
// answers GET /auth/session as that owner; a shot's routes() (context.route)
// still wins for any URL it handles. --no-computer keeps the mocked backend.
//
// The package catalog: /api/catalog/* is the platform proxy's (it reads corpusd
// for the owner it authenticated), and no capsule reaches it, so the autoputer
// here answers it 404 (docs/problems/capsule-library-catalog-404-2026-11-03.md).
// The helper's own server answers GET /api/catalog/* from
// scripts/fixtures/capsule-catalog.json, written from corpusd's handler by
// internal/platform/capsule_catalog_fixture_test.go, with the header
// x-choir-fixture: capsule-catalog. A shot whose page read it carries a visible
// "catalog fixture" badge and logs a line saying so. A listing act (PATCH)
// answers 409 capsule_fixture, any other write 405 as at the proxy; history
// paging (limit, before_seq) is not modelled. A shot's routes() still wins for any
// catalog URL it handles.
//
// Install: POST/GET /api/computers/<id>/self-development/adoptions[/<aid>] is the
// owner's act on the real computer, which the autoputer here refuses. The helper
// answers it from scripts/fixtures/capsule-install.json, written from the real
// adoption handler by internal/agentcore/capsule_install_fixture_test.go, with the
// same x-choir-fixture header and badge. An install of a catalog fixture release
// answers the real 201 view (a repeat 200), and the installed list holds what this
// shot installed; the state is per shot, emptied with the desktop reset. The
// earliest release of the fixture's refusal_package answers the real refusal (409
// patch_does_not_apply, a base mismatch); a release the catalog fixture lacks
// answers 404 release_not_published.
//
// Run it as a capsule job (choir.Start), not a choir.Exec: a first build and a
// browser start outlast 30 s. It does not touch /workspace/platform (the build runs in a private copy under /tmp).
import { spawn, spawnSync } from 'node:child_process';
import { createHash } from 'node:crypto';
import fs from 'node:fs';
import http from 'node:http';
import net from 'node:net';
import { createRequire } from 'node:module';
import path from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';

export const PRESETS = {
  desktop: { width: 1280, height: 800 },
  tablet: { width: 820, height: 1180 },
  mobile: { width: 390, height: 844, isMobile: true, hasTouch: true, deviceScaleFactor: 2 },
};
const NAME_RE = /^[A-Za-z0-9._-]+$/;
export const THEME_BOOT_CACHE_KEY = 'choir.theme.boot.v2';
export const THEME_SCHEMA_VERSION = 2;
export const THEME_PREFERENCE_PATH = '/api/preferences/theme';
const STEP_MS = 15000;
export const COMPUTER_DIR = '/computer';
export const AUTOPUTER_PORT = 8082;
export const AUTOPUTER_USER = 'owner';
const AUTOPUTER_BINARIES = ['/var/lib/artifact/release/bin/autoputer', '/tmp/autoputer'];

// themeIdsFrom reads the preset ids from the shell's theme.ts source (the
// two-space `id: 'x',` lines of the preset objects), so the list is the shell's own.
export function themeIdsFrom(source) {
  return [...source.matchAll(/^ {2}id: '([a-z0-9-]+)',$/gm)].map((m) => m[1]);
}

function checkThemes(list, known, what) {
  for (const t of list) if (!/^[a-z0-9-]+$/.test(t)) throw new Error(`${what} ${t}: a preset id is lowercase letters, digits and '-'`);
  if (!known) return;
  for (const t of list) if (!known.includes(t)) throw new Error(`${what} ${t}: unknown theme preset (have ${known.join(', ')})`);
}

// USAGE is what --help prints: the flags, and where things go.
export const USAGE = `usage: node scripts/choir-shot.mjs --shots FILE [options]

  --shots FILE      ES module whose default export is a list of shots (required)
  --only a,b        take only these shots
  --preset p,q      desktop (default), tablet, mobile
  --size WxH        custom viewport, such as 1000x700 (exclusive with --preset)
  --theme a,b       shell theme preset ids, one shot per preset
  --contrast SEL    WCAG contrast of every visible text element under SEL
  --open APP        open a package app in the shell before the shot
  --out DIR         PNG directory (default /var/lib/artifact/screens)
  --list            print the file names the run would write and exit
  --fail-fast       stop at the first failed shot (the default for a run without --only)
  --keep-going      take every shot even after one fails (the default with --only)
  --no-computer     mock the backend instead of starting the autoputer over /computer
  --help            print this usage

The autoputer's stdout and stderr go to autoputerLogPath(out) (default
/var/lib/artifact/shot-logs/autoputer.log), not to this output; its path is
printed when the autoputer or a shot fails. Run it as a capsule job (choir.Start).
The header of scripts/choir-shot.mjs documents the shots file and the options.
`;

// autoputerLogPath is where the autoputer's output goes: beside the screens
// directory, since Freeze refuses any file in screens/ that is not a plain .png.
export function autoputerLogPath(out) {
  return path.join(path.dirname(path.resolve(out)), 'shot-logs', 'autoputer.log');
}

export function parseArgs(argv, knownThemes = null) {
  if (argv.includes('--help') || argv.includes('-h')) return { help: true };
  const o = { shots: '', only: [], presets: [], themes: [], contrast: '', open: '', size: '', out: '/var/lib/artifact/screens', list: false, computer: true, failFast: undefined };
  for (let i = 0; i < argv.length; i++) {
    const a = argv[i];
    const val = () => {
      if (i + 1 >= argv.length) throw new Error(`${a} needs a value`);
      return argv[++i];
    };
    if (a === '--shots') o.shots = val();
    else if (a === '--only') o.only = val().split(',').filter(Boolean);
    else if (a === '--preset') o.presets = val().split(',').filter(Boolean);
    else if (a === '--theme') o.themes = val().split(',').filter(Boolean);
    else if (a === '--contrast') o.contrast = val();
    else if (a === '--open') o.open = val();
    else if (a === '--size') o.size = val();
    else if (a === '--out') o.out = val();
    else if (a === '--list') o.list = true;
    else if (a === '--fail-fast') o.failFast = true;
    else if (a === '--keep-going') o.failFast = false;
    else if (a === '--no-computer') o.computer = false;
    else throw new Error(`unknown flag ${a}`);
  }
  if (!o.shots) throw new Error('--shots FILE is required (an ES module whose default export is a list of shots)');
  if (a0(argv, '--fail-fast') && a0(argv, '--keep-going')) throw new Error('--fail-fast and --keep-going are exclusive');
  if (o.size && o.presets.length) throw new Error('--size and --preset are exclusive');
  if (o.size && !/^\d{2,5}x\d{2,5}$/.test(o.size)) throw new Error(`--size ${o.size}: want WIDTHxHEIGHT, such as 1000x700`);
  for (const p of o.presets) if (!PRESETS[p]) throw new Error(`--preset ${p}: want one of ${Object.keys(PRESETS).join(', ')}`);
  if (a0(argv, '--theme') && o.themes.length === 0) throw new Error('--theme needs at least one preset id');
  if (new Set(o.themes).size !== o.themes.length) throw new Error(`--theme ${o.themes.join(',')}: a preset is listed twice`);
  checkThemes(o.themes, knownThemes, '--theme');
  if (a0(argv, '--open')) checkApp(o.open, '--open');
  return o;
}

const APP_RE = /^[A-Za-z0-9][A-Za-z0-9 ._-]*$/;
function checkApp(app, what) {
  if (typeof app !== 'string' || !APP_RE.test(app)) throw new Error(`${what} ${JSON.stringify(app)}: want an app id or title such as image or Slides`);
}

function a0(argv, flag) {
  return argv.includes(flag);
}

// themesFor is the presets one shot is taken under: its own `theme` field, else
// --theme, else none (the shell's default, no seeding).
export function themesFor(shot, o, known = null) {
  let t = shot.theme;
  if (t === undefined || t === null || t === '') return o.themes.length ? o.themes : [null];
  t = Array.isArray(t) ? t : [t];
  if (t.length === 0 || t.some((x) => typeof x !== 'string' || !x)) throw new Error(`shot ${shot.name}: theme must be a preset id or a list of them`);
  checkThemes(t, known, `shot ${shot.name}: theme`);
  return t;
}

// plan expands the selected shots and presets into the files to write.
export function plan(shots, o, knownThemes = null) {
  if (!Array.isArray(shots) || shots.length === 0) throw new Error('the shots file must default-export a non-empty list');
  const seen = new Set();
  for (const s of shots) {
    if (!s || !NAME_RE.test(s.name || '')) throw new Error(`shot name ${JSON.stringify(s && s.name)}: letters, digits, '.', '-' and '_' only`);
    if (seen.has(s.name)) throw new Error(`duplicate shot name ${s.name}`);
    seen.add(s.name);
  }
  for (const n of o.only) if (!seen.has(n)) throw new Error(`--only ${n}: no such shot (have ${[...seen].join(', ')})`);
  const picked = o.only.length ? shots.filter((s) => o.only.includes(s.name)) : shots;
  let views;
  if (o.size) {
    const [width, height] = o.size.split('x').map(Number);
    views = [{ label: o.size, viewport: { width, height } }];
  } else {
    const names = o.presets.length ? o.presets : ['desktop'];
    views = names.map((n) => ({ label: n, preset: PRESETS[n] }));
  }
  const out = [];
  for (const shot of picked)
    for (const theme of themesFor(shot, o, knownThemes))
      for (const v of views) {
        const { width, height, ...device } = v.preset || { ...v.viewport };
        const stem = `${shot.name}${theme ? '.' + theme : ''}${v.label === 'desktop' ? '' : '-' + v.label}`;
        const contrast = shot.contrast || o.contrast || '';
        const open = shot.open || o.open || '';
        if (open) checkApp(open, `shot ${shot.name}: open`);
        out.push({ shot, theme, contrast, open, viewport: { width, height }, device, file: `${stem}.png`, contrastFile: contrast ? `${stem}.contrast.json` : '' });
      }
  return out;
}

// openApp opens a package app the way a person does: the app switcher button,
// the launcher entry, the app window. page is a Playwright page (or anything
// with locator(sel) giving click, waitFor and evaluateAll). It returns the
// app id the window carries; every missing step throws a message naming it.
export async function openApp(page, app, ms = STEP_MS) {
  const waitFor = async (sel, state, what) => {
    try { await page.locator(sel).first().waitFor({ state, timeout: ms }); } catch { throw new Error(`--open ${app}: ${what} (${sel}) did not become ${state} within ${ms / 1000} s`); }
  };
  // The desktop refuses a launch ("Desktop is still connecting" toast, no
  // window) until its live channel is open and its state is loaded; wait for
  // that rather than hiding the toast.
  await waitFor('[data-desktop][data-desktop-ready="true"]', 'visible', 'the desktop connecting (live channel and state)');
  await waitFor('[data-desk-menu-button]', 'visible', 'the app switcher button');
  await page.locator('[data-desk-menu-button]').first().click();
  await waitFor('[data-desk-sheet]', 'visible', 'the launcher sheet after clicking the switcher');
  const entries = await page.locator('[data-desk-sheet-app]').evaluateAll((els) => els.map((e) => ({
    id: e.getAttribute('data-desk-app-id') || '',
    title: ((e.querySelector('strong') || {}).textContent || '').trim(),
  })));
  const want = app.toLowerCase();
  const hit = entries.find((e) => e.id.toLowerCase() === want) || entries.find((e) => e.title.toLowerCase() === want);
  if (!hit) throw new Error(`--open ${app}: no launcher entry with that id or title; the launcher menu has ${entries.map((e) => e.id + (e.title && e.title.toLowerCase() !== e.id ? ` (${e.title})` : '')).join(', ') || 'no apps'}. An app with launcher.menu false is not in it`);
  await page.locator(`[data-desk-sheet-app][data-desk-app-id="${hit.id}"]`).first().click();
  await waitFor(`[data-window][data-window-app-id="${hit.id}"]`, 'visible', `the ${hit.id} app window (it may have asked to sign in: a private app logged out opens no window)`);
  return hit.id;
}

// windowProblem judges what the page reports about an app's windows: each
// {visible, minimized, w, h}, plus whether a taskbar chip names the app. It
// returns '' when one window is visible, not minimized and has a size, else the
// failure message (the app and what was found).
export function windowProblem(app, wins, chip) {
  if (wins.some((w) => w.visible && !w.minimized && w.w > 0 && w.h > 0)) return '';
  const found = wins.length === 0 ? 'no window element' : `${wins.length} window(s), none visible and sized (${wins.map((w) => (w.minimized ? 'minimized' : !w.visible ? 'hidden' : `${w.w}x${w.h}`)).join(', ')})`;
  return `no window for ${app} on screen at capture; ${found}; taskbar chip ${chip ? 'present' : 'absent'}. If the act closes the window on purpose, set expect_window: false`;
}

// checkAppWindow fails a shot that names an app but shows none of its windows.
export async function checkAppWindow(page, app) {
  const r = await page.evaluate((id) => {
    const wins = [...document.querySelectorAll(`[data-window][data-window-app-id="${id}"]`)].map((el) => {
      const b = el.getBoundingClientRect(), cs = getComputedStyle(el);
      return { visible: cs.display !== 'none' && cs.visibility !== 'hidden' && Number(cs.opacity) > 0, minimized: el.getAttribute('data-window-mode') === 'minimized', w: Math.round(b.width), h: Math.round(b.height) };
    });
    const chip = !!document.querySelector('[data-window-tray-item]'); // a tray chip carries a window id, not an app id
    return { wins, chip };
  }, app);
  const msg = windowProblem(app, r.wins, r.chip);
  if (msg) throw new Error(msg);
}

// SETTLE_MS bounds the wait for a window to stop animating before the shot.
const SETTLE_MS = 3000;

// settleProblem says why the app's windows are not settled, or '' when one is:
// opacity 1 and every animation on it and its subtree finished. Pure, so the
// self-test can feed it states.
export function settleProblem(app, wins) {
  if (wins.some((w) => w.opacity >= 1 && w.running === 0 && w.ancestorRunning === 0)) return '';
  const w = wins.find((x) => x.visible) || wins[0];
  if (!w) return '';
  return `window for ${app} did not settle within ${SETTLE_MS / 1000} s (opacity ${w.opacity}, ${w.running} running animation(s) on it, ${w.ancestorRunning} on its ancestors)`;
}

// waitSettled waits, bounded, until the app's window has opacity 1 and no
// running CSS animation or transition on it, its subtree or its ancestors, and
// the fonts are ready; a shot caught mid-open-animation is a failed shot.
export async function waitSettled(page, app) {
  const probe = (id) => page.evaluate((appId) => {
    // finite only: a spinner that loops forever never settles and is not an open animation
    const live = (a) => (a.playState === 'running' || a.playState === 'pending') && Number.isFinite(a.effect?.getComputedTiming().endTime);
    return [...document.querySelectorAll(`[data-window][data-window-app-id="${appId}"]`)].filter((el) => el.getAttribute('data-window-mode') !== 'minimized').map((el) => {
      const cs = getComputedStyle(el);
      let ancestorRunning = 0;
      for (let p = el.parentElement; p; p = p.parentElement) ancestorRunning += p.getAnimations().filter(live).length;
      return { visible: cs.display !== 'none' && cs.visibility !== 'hidden', opacity: Number(cs.opacity), running: el.getAnimations({ subtree: true }).filter(live).length, ancestorRunning };
    });
  }, id);
  const deadline = Date.now() + SETTLE_MS;
  let wins = await probe(app);
  while (wins.length && settleProblem(app, wins) && Date.now() < deadline) {
    await page.waitForTimeout(100);
    wins = await probe(app);
  }
  const msg = settleProblem(app, wins);
  if (msg) throw new Error(msg);
  await Promise.race([page.evaluate(() => document.fonts.ready), page.waitForTimeout(SETTLE_MS)]);
}

// themeSeedScript is the init script that stores the boot cache before any page
// script runs, in the shape App.svelte's loadThemeBootCache reads: a JSON
// string of {schema_version, id} (normalizeThemeConfig fills in the rest).
export function themeSeedScript(preset) {
  const value = JSON.stringify({ schema_version: THEME_SCHEMA_VERSION, id: preset });
  return `try { window.localStorage.setItem(${JSON.stringify(THEME_BOOT_CACHE_KEY)}, ${JSON.stringify(value)}); } catch (e) {}`;
}

// themeAnswer is the helper server's answer to GET /api/preferences/theme while
// a preset is set: the body, or null when the request is not that one.
export function themeAnswer(preset, method, pathname) {
  if (!preset || method !== 'GET' || pathname !== THEME_PREFERENCE_PATH) return null;
  return { theme: { schema_version: THEME_SCHEMA_VERSION, id: preset } };
}

// ---- contrast ----

export function parseColor(s) {
  s = String(s || '').trim();
  let m = s.match(/^rgba?\(\s*([\d.]+)[,\s]+([\d.]+)[,\s]+([\d.]+)(?:\s*[,/]\s*([\d.]+)(%?))?\s*\)$/);
  if (m) return { r: +m[1], g: +m[2], b: +m[3], a: m[4] === undefined ? 1 : m[5] ? +m[4] / 100 : +m[4] };
  m = s.match(/^color\(srgb\s+([\d.]+)\s+([\d.]+)\s+([\d.]+)(?:\s*\/\s*([\d.]+)(%?))?\s*\)$/);
  if (m) return { r: +m[1] * 255, g: +m[2] * 255, b: +m[3] * 255, a: m[4] === undefined ? 1 : m[5] ? +m[4] / 100 : +m[4] };
  if (s === 'transparent') return { r: 0, g: 0, b: 0, a: 0 };
  return null;
}

// over composites the top color (with alpha) onto an opaque or translucent bottom.
export function over(top, bottom) {
  const a = top.a + bottom.a * (1 - top.a);
  if (a === 0) return { r: 0, g: 0, b: 0, a: 0 };
  const mix = (t, b) => (t * top.a + b * bottom.a * (1 - top.a)) / a;
  return { r: mix(top.r, bottom.r), g: mix(top.g, bottom.g), b: mix(top.b, bottom.b), a };
}

// resolveBackground walks layers (element first, root last; each {color, image},
// color null when unreadable) to the first opaque layer with no image and
// composites the translucent ones above it. {kind:'pixel', reason} means the
// walk cannot know: an image, an unreadable color or no opaque layer.
export function resolveBackground(layers) {
  const above = [];
  for (const l of layers) {
    if (l.image) return { kind: 'pixel', reason: 'image background' };
    if (l.color === null || l.color === undefined) return { kind: 'pixel', reason: 'unreadable background color' };
    if (l.color.a === 0) continue;
    if (l.color.a >= 1) {
      let c = { ...l.color, a: 1 };
      for (let i = above.length - 1; i >= 0; i--) c = over(above[i], c);
      return { kind: 'opaque', color: { r: c.r, g: c.g, b: c.b, a: 1 } };
    }
    above.push(l.color);
  }
  return { kind: 'pixel', reason: above.length ? 'no opaque layer under translucent ones' : 'no opaque layer' };
}

export function luminance(c) {
  const f = (v) => { v /= 255; return v <= 0.03928 ? v / 12.92 : Math.pow((v + 0.055) / 1.055, 2.4); };
  return 0.2126 * f(c.r) + 0.7152 * f(c.g) + 0.0722 * f(c.b);
}

export function contrastRatio(fg, bg) {
  const a = luminance(fg), b = luminance(bg);
  return (Math.max(a, b) + 0.05) / (Math.min(a, b) + 0.05);
}

export function requiredRatio(fontSizePx, fontWeight) {
  return fontSizePx >= 24 || (fontSizePx >= 18.66 && Number(fontWeight) >= 700) ? 3 : 4.5;
}

// samplePixels returns up to ~max opaque colors on a grid over the css-pixel
// rect of a decoded PNG {width, height, data} taken at one pixel per css pixel.
export function samplePixels(png, rect, max = 200) {
  const x0 = Math.max(0, Math.floor(rect.x)), y0 = Math.max(0, Math.floor(rect.y));
  const x1 = Math.min(png.width, Math.ceil(rect.x + rect.width)), y1 = Math.min(png.height, Math.ceil(rect.y + rect.height));
  const w = x1 - x0, h = y1 - y0;
  if (w <= 0 || h <= 0) return [];
  const step = Math.max(1, Math.ceil(Math.sqrt((w * h) / max)));
  const out = [];
  for (let y = y0; y < y1; y += step)
    for (let x = x0; x < x1; x += step) {
      const i = (y * png.width + x) * 4;
      out.push({ r: png.data[i], g: png.data[i + 1], b: png.data[i + 2], a: 1 });
    }
  return out;
}

// measureNode scores one collected node {fg, layers, rect, fontSize, fontWeight}.
export function measureNode(node, png) {
  const fg = parseColor(node.color);
  if (!fg) return { ratio: null, method: 'unmeasured: unreadable text color' };
  const req = requiredRatio(node.fontSize, node.fontWeight);
  const layers = node.layers.map((l) => ({ color: parseColor(l.bg), image: l.image }));
  const bg = resolveBackground(layers);
  if (bg.kind === 'opaque') {
    const text = fg.a < 1 ? over(fg, bg.color) : fg;
    return { ratio: contrastRatio(text, bg.color), required: req, method: 'opaque-ancestor' };
  }
  if (!png) return { ratio: null, required: req, method: 'unmeasured: image background' };
  const px = samplePixels(png, node.rect);
  if (px.length === 0) return { ratio: null, required: req, method: 'unmeasured: box outside the screenshot' };
  let min = Infinity;
  for (const p of px) min = Math.min(min, contrastRatio(fg.a < 1 ? over(fg, p) : fg, p));
  return { ratio: min, required: req, method: `pixels (${bg.reason}; lowest of ${px.length} samples)` };
}

// contrastDir is where the reports go: beside the screens directory, since
// Freeze refuses a non-PNG file inside it.
export function contrastDir(out) {
  return path.join(path.dirname(path.resolve(out)), 'contrast');
}

// summarizeContrast builds the JSON report and the one-line summary.
export function summarizeContrast(meta, nodes, png) {
  const results = nodes.map((n) => ({ n, r: measureNode(n, png) }));
  const measured = results.filter((x) => x.r.ratio !== null);
  const unmeasured = results.filter((x) => x.r.ratio === null);
  const failing = measured.filter((x) => x.r.ratio < x.r.required).map((x) => ({ selector: x.n.path, text: x.n.text, scope: x.n.scope || 'unscoped', ratio: +x.r.ratio.toFixed(2), required: x.r.required, method: x.r.method }));
  const min = measured.length ? +Math.min(...measured.map((x) => x.r.ratio)).toFixed(2) : null;
  const methods = {};
  for (const x of results) { const k = x.r.method.split(' (')[0]; methods[k] = (methods[k] || 0) + 1; }
  const report = { ...meta, nodes: results.length, measured: measured.length, unmeasured: unmeasured.length, min_ratio: min, failing, unmeasured_nodes: unmeasured.slice(0, 20).map((x) => ({ selector: x.n.path, text: x.n.text, scope: x.n.scope || 'unscoped', method: x.r.method })), methods };
  const byScope = {};
  for (const f of failing) byScope[f.scope] = (byScope[f.scope] || 0) + 1;
  const scopes = Object.keys(byScope).some((k) => k !== 'unscoped') ? ` (${Object.entries(byScope).map(([k, v]) => `${v} ${k}`).join(', ')})` : '';
  const line = `contrast ${meta.file}: min ${min === null ? 'n/a' : min} over ${measured.length}/${results.length} text nodes, ${failing.length} failing${scopes}, ${unmeasured.length} unmeasured`;
  const detail = failing.slice(0, 10).map((f) => `  ${f.scope} ${f.ratio} < ${f.required} ${f.selector} "${f.text}"`);
  return { report, line, detail };
}

// collectTextNodes runs in the page: every visible element under selector with
// its own text, its color, box and the background layers from itself to the root.
export function collectTextNodes({ selector, limit, appId = '' }) {
  const roots = [...document.querySelectorAll(selector)];
  const seen = new Set();
  const out = [];
  // pathOf: the nearest stable anchor (an id, else a data-* attribute whose
  // value is not per-window state) on the node or an ancestor, then the class
  // path below it; with no anchor, the last five steps.
  const volatile = new Set(['data-window-id', 'data-window-mode', 'data-window-active', 'data-overview-preview-state', 'data-window-tray-item-active', 'data-svelte-h']);
  const pathOf = (el) => {
    const parts = [];
    for (let e = el; e && e.nodeType === 1 && e !== document.documentElement; e = e.parentElement) {
      let p = e.tagName.toLowerCase();
      if (e.id) { parts.unshift(`${p}#${e.id}`); return parts.join(' > '); }
      const attr = [...(e.attributes || [])].find((a) => a.name.startsWith('data-') && !volatile.has(a.name));
      if (attr) { parts.unshift(attr.value && attr.value.length <= 30 ? `${p}[${attr.name}="${attr.value}"]` : `${p}[${attr.name}]`); return parts.join(' > '); }
      const cls = [...e.classList].slice(0, 2).join('.');
      if (cls) p += '.' + cls;
      parts.unshift(p);
    }
    return parts.slice(-5).join(' > ');
  };
  const windowSel = appId ? `[data-window][data-window-app-id="${appId}"]` : '';
  const scopeOf = (el) => {
    if (!appId) return 'unscoped';
    const content = el.closest('[data-window-content]'); // the package's own surface; the window's title bar and controls are chrome
    return content && content.closest(windowSel) ? 'package-window' : 'shell-chrome';
  };
  for (const root of roots) {
    const els = [root, ...root.querySelectorAll('*')];
    for (const el of els) {
      if (seen.has(el) || out.length >= limit) continue;
      seen.add(el);
      const text = [...el.childNodes].filter((c) => c.nodeType === 3).map((c) => c.textContent).join(' ').replace(/\s+/g, ' ').trim();
      if (!text) continue;
      const cs = getComputedStyle(el);
      const r = el.getBoundingClientRect();
      if (cs.display === 'none' || cs.visibility === 'hidden' || r.width < 1 || r.height < 1) continue;
      const layers = [];
      for (let e = el; e; e = e.parentElement) {
        const s = getComputedStyle(e);
        layers.push({ bg: s.backgroundColor, image: s.backgroundImage !== 'none' || (s.backdropFilter && s.backdropFilter !== 'none') || false });
      }
      out.push({ path: pathOf(el), scope: scopeOf(el), text: text.slice(0, 60), color: cs.color, fontSize: parseFloat(cs.fontSize), fontWeight: cs.fontWeight, rect: { x: r.x + window.scrollX, y: r.y + window.scrollY, width: r.width, height: r.height }, layers });
    }
  }
  return out;
}

// pickAutoputerBinary returns the first existing binary in the fixed order, or
// '' when none exists and the caller must build one.
export function pickAutoputerBinary(exists = fs.existsSync) {
  return AUTOPUTER_BINARIES.find((b) => exists(b)) || '';
}

// CATALOG_FIXTURE_PATH is the catalog fixture beside this script.
export const CATALOG_FIXTURE_PATH = path.join(path.dirname(fileURLToPath(import.meta.url)), 'fixtures', 'capsule-catalog.json');
export const CATALOG_FIXTURE_HEADER = 'x-choir-fixture';
export const CATALOG_FIXTURE_BADGE = 'Catalog fixture, not live';

// loadCatalogFixture reads the fixture; null when it is missing or not one.
export function loadCatalogFixture(file = CATALOG_FIXTURE_PATH) {
  try {
    const fx = JSON.parse(fs.readFileSync(file, 'utf8'));
    return fx && fx.fixture === 'capsule-catalog' && fx.packages && Array.isArray(fx.packages.packages) && Array.isArray(fx.history) && Array.isArray(fx.releases) ? fx : null;
  } catch {
    return null;
  }
}

// INSTALL_FIXTURE_PATH is the install fixture beside this script.
export const INSTALL_FIXTURE_PATH = path.join(path.dirname(fileURLToPath(import.meta.url)), 'fixtures', 'capsule-install.json');

// loadInstallFixture reads the install fixture; null when it is missing or not one.
export function loadInstallFixture(file = INSTALL_FIXTURE_PATH) {
  try {
    const fx = JSON.parse(fs.readFileSync(file, 'utf8'));
    const ok = (a) => a && Number.isInteger(a.status) && a.body && typeof a.body === 'object';
    return fx && fx.fixture === 'capsule-install' && typeof fx.refusal_package === 'string' && ok(fx.created) && ok(fx.repeat) && ok(fx.list) && ok(fx.refusal)
      && Array.isArray(fx.list.body.adoptions) ? fx : null;
  } catch {
    return null;
  }
}

const SHA256_RE = /^[0-9a-f]{64}$/;
const PACKAGE_NAME_RE = /^[a-z][a-z0-9-]{1,31}$/; // objectgraph.ValidPackageName's grammar (not its reserved names)

// catalogAnswer is the fixture's answer to a /api/catalog request, as the proxy
// relays corpusd's: { status, body }, or null for a path that is not the
// catalog's (or no fixture). search is the URL's query string.
export function catalogAnswer(fx, method, pathname, search) {
  if (!fx || !(pathname === '/api/catalog' || pathname.startsWith('/api/catalog/'))) return null;
  const parts = pathname.replace(/^\/api\/catalog\/?/, '').split('/').filter(Boolean);
  if (method === 'PATCH' && parts.length === 3 && parts[0] === 'records' && parts[2] === 'listing') {
    return { status: 409, body: { error: 'the catalog in a capsule is a fixture; a listing act is the owner\'s on the real computer', reason: 'capsule_fixture' } };
  }
  if (method !== 'GET') return { status: 405, body: { error: 'method not allowed' } };
  const q = new URLSearchParams(search || '');
  const reader = fx.packages.reader;
  if (parts.length === 1 && parts[0] === 'packages') {
    const needle = (q.get('q') || '').trim().toLowerCase();
    const packages = fx.packages.packages.filter((p) => !needle || p.package.includes(needle));
    return { status: 200, body: { ...fx.packages, packages, count: packages.length } };
  }
  if (parts.length === 3 && parts[0] === 'packages' && parts[2] === 'releases' && PACKAGE_NAME_RE.test(parts[1])) {
    const track = (q.get('track') || '').trim() || reader;
    const found = fx.history.find((h) => h.package === parts[1] && h.track === track);
    return { status: 200, body: found || { package: parts[1], track, releases: [], count: 0 } };
  }
  if (parts.length === 2 && parts[0] === 'releases' && SHA256_RE.test(parts[1].toLowerCase())) {
    const found = fx.releases.find((r) => r.release_id === parts[1].toLowerCase());
    return found ? { status: 200, body: found } : { status: 404, body: { error: 'package release not published' } };
  }
  return { status: 404, body: { error: 'not found' } };
}

const INSTALL_PATH_RE = /^\/api\/computers\/([^/]+)\/self-development\/adoptions(?:\/([^/]+))?\/?$/;

// isInstallPath: is this URL path an adoptions route (the install fixture's)?
export function isInstallPath(pathname) { return INSTALL_PATH_RE.test(pathname); }

// refusalRelease is the catalog fixture release the install fixture refuses: the
// earliest published of refusal_package.
function refusalRelease(ifx, cfx) {
  const of = cfx.releases.filter((r) => r.record && r.record.package && r.record.package.name === ifx.refusal_package);
  of.sort((a, b) => String(a.published_at).localeCompare(String(b.published_at)));
  return of.length ? of[0].release_id : '';
}

// installAnswer is the install fixture's answer to an adoptions request, as the
// proxy relays the computer's: { status, body }, or null for a path that is not
// the adoptions route (or no fixture). bodyText is the request body. state is
// the shot's { adoptions: [] }, appended to by an install and emptied per shot.
export function installAnswer(ifx, cfx, state, method, pathname, bodyText) {
  const m = INSTALL_PATH_RE.exec(pathname);
  if (!ifx || !cfx || !m) return null;
  const computerId = m[1]; // pathname arrives decoded once, as the proxy's router sees it
  const aid = m[2] || '';
  const clone = (v) => JSON.parse(JSON.stringify(v));
  if (method === 'GET' && !aid) return { status: ifx.list.status, body: { ...clone(ifx.list.body), adoptions: clone(state.adoptions.filter((a) => a.computer_id === computerId)) } };
  if (method === 'GET') {
    const found = state.adoptions.find((a) => a.adoption_id === aid && a.computer_id === computerId);
    return found ? { status: ifx.repeat.status, body: clone(found) } : { status: 404, body: { error: 'adoption not found' } };
  }
  if (method !== 'POST' || aid) return { status: 405, body: { error: 'method not allowed' } };
  let req;
  try { req = JSON.parse(bodyText || ''); } catch { req = null; }
  if (!req || typeof req !== 'object' || Array.isArray(req)) return { status: 400, body: { error: 'invalid adoption request', reason: 'request_invalid' } };
  const releaseId = String(req.release_id || '').trim().toLowerCase();
  if (!SHA256_RE.test(releaseId)) return { status: 400, body: { error: 'release_id must be a sha256', reason: 'release_id_invalid' } };
  const adoptionId = 'adoption-' + createHash('sha256').update(`${computerId}\0${releaseId}`).digest('hex').slice(0, 24);
  const existing = state.adoptions.find((a) => a.adoption_id === adoptionId);
  if (existing) return { status: ifx.repeat.status, body: clone(existing) };
  const rel = cfx.releases.find((r) => r.release_id === releaseId);
  if (!rel) return { status: 404, body: { error: `the host has no published release ${releaseId}`, reason: 'release_not_published' } };
  if (releaseId === refusalRelease(ifx, cfx)) return { status: ifx.refusal.status, body: clone(ifx.refusal.body) };
  const n = state.adoptions.length + 1;
  const rec = rel.record || {};
  const src = rec.source || {};
  const view = clone(ifx.created.body);
  Object.assign(view, {
    adoption_id: adoptionId, computer_id: computerId, release_id: releaseId,
    publisher_computer_id: (rec.publisher || {}).computer_id || '', publisher_operation_id: (rec.operation || {}).operation_id || '',
    patch_sha256: (src.tree_patch || {}).sha256 || '', release_base_commit: src.built_on || '', release_tree_hash: src.tree || '',
    adopter_base_commit: src.built_on || '', adopter_tree_hash: src.tree || '', same_base: true,
    staged_dir: `/var/lib/choir/updater/adoptions/${releaseId}`,
    trajectory_id: `traj-adopt-${n}`, submission_id: `submission-${n}`, doc_id: `doc-adopt-${n}`,
  });
  state.adoptions.unshift(view); // newest first, as the store lists
  return { status: ifx.created.status, body: clone(view) };
}

// fixtureBadge runs in the page: a fixed label on any shot that read the
// fixture, so the screen says what it shows. It sits at the top-left edge:
// never over the taskbar or prompt bar at the bottom of a phone-size shot, and
// never over a window's minimize, maximize and close at its top right (a judge
// rejected every desktop shot of claude Library r2 for that, 2026-11-04). New
// windows open clear of the desktop icons there (getNewWindowGeometry).
export function fixtureBadge(text) {
  const el = document.createElement('div');
  el.dataset.choirShotFixture = 'capsule-catalog';
  el.textContent = text;
  Object.assign(el.style, {
    position: 'fixed', top: '2px', left: '8px', maxWidth: '40vw', zIndex: '2147483647', pointerEvents: 'none',
    font: '600 12px/1.3 system-ui, sans-serif', color: '#111', background: '#ffd84d', border: '1px solid #111',
    borderRadius: '4px', padding: '3px 7px',
  });
  document.body.appendChild(el);
}

// isProxied is the rule for what the served frontend sends to the autoputer:
// every /api/ URL. /auth/session is answered locally; other /auth URLs 404.
export function isProxied(pathname) {
  return pathname === '/api' || pathname.startsWith('/api/');
}

export function autoputerEnv(base = process.env) {
  return { ...base, RUNTIME_STORE_PATH: `${COMPUTER_DIR}/state`, AUTOPUTER_FILES_ROOT: `${COMPUTER_DIR}/files`, AUTOPUTER_PORT: String(AUTOPUTER_PORT), CHOIR_GUEST_TRUST_LOOPBACK: '1' };
}

function portOpen(port) {
  return new Promise((resolve) => {
    const sock = net.connect(port, '127.0.0.1');
    sock.once('connect', () => { sock.destroy(); resolve(true); });
    sock.once('error', () => { sock.destroy(); resolve(false); });
  });
}

function waitForPort(port, child, deadlineMs) {
  const end = Date.now() + deadlineMs;
  return new Promise((resolve, reject) => {
    const tryOnce = () => {
      if (child.exitCode !== null) return reject(new Error(`the autoputer exited (code ${child.exitCode}) before port ${port} accepted connections`));
      const sock = net.connect(port, '127.0.0.1');
      sock.once('connect', () => { sock.destroy(); resolve(); });
      sock.once('error', () => {
        sock.destroy();
        if (Date.now() > end) reject(new Error(`the autoputer did not accept connections on 127.0.0.1:${port} within ${deadlineMs / 1000} s`));
        else setTimeout(tryOnce, 200);
      });
    };
    tryOnce();
  });
}

// startAutoputer starts the autoputer over /computer as a child of this
// process and returns it once its port accepts connections.
async function startAutoputer(root, holder) {
  if (await portOpen(AUTOPUTER_PORT)) throw new Error(`127.0.0.1:${AUTOPUTER_PORT} already accepts connections (an autoputer started by hand?); stop it, since the shots would read its content`);
  let bin = pickAutoputerBinary();
  if (!bin) {
    bin = AUTOPUTER_BINARIES[1];
    console.log('choir-shot: no autoputer binary found; the first run builds it (go build -o /tmp/autoputer ./cmd/autoputer)');
    const r = spawnSync('go', ['build', '-o', bin, './cmd/autoputer'], { cwd: root, stdio: 'inherit' });
    if (r.status !== 0) throw new Error(`go build of the autoputer failed (exit ${r.status}); pass --no-computer to mock the backend instead`);
  }
  fs.mkdirSync(path.dirname(holder.log), { recursive: true });
  const logFd = fs.openSync(holder.log, 'w');
  const child = spawn(bin, [], { env: autoputerEnv(), stdio: ['ignore', logFd, logFd] });
  fs.closeSync(logFd);
  holder.child = child;
  child.on('error', () => {});
  try {
    await waitForPort(AUTOPUTER_PORT, child, 30000);
  } catch (e) {
    child.kill();
    throw e;
  }
  console.log(`choir-shot: autoputer (${bin}) is up on 127.0.0.1:${AUTOPUTER_PORT}`);
  return child;
}

export function sourceHash(frontend) {
  const h = createHash('sha256');
  const walk = (p) => {
    for (const e of fs.readdirSync(p, { withFileTypes: true }).sort((a, b) => (a.name < b.name ? -1 : 1))) {
      if (['node_modules', 'dist', 'tests', '.vite'].includes(e.name)) continue;
      const f = path.join(p, e.name);
      if (e.isDirectory()) walk(f);
      else h.update(f).update(fs.readFileSync(f));
    }
  };
  walk(frontend);
  return h.digest('hex').slice(0, 16);
}

// build runs vite once per source hash, in a private copy of the frontend under
// /tmp whose node_modules is a link to $CHOIR_FRONTEND_DEPS/node_modules. It
// never reads or writes /workspace/platform/frontend/node_modules: a directory
// an earlier run or the engineer left there (a stray .vite cache, a half
// install) hid the locked dependencies from the build ("could not resolve
// @chenglou/pretext", problem 18), and a link made there could freeze into the
// candidate. Vite follows the link (resolve.preserveSymlinks is off), so a
// dependency resolves through the read-only Nix tree and its real path.
function build(frontend, deps) {
  const cacheRoot = '/tmp/choir-shot';
  const key = path.join(cacheRoot, sourceHash(frontend));
  const dist = path.join(key, 'dist');
  if (fs.existsSync(path.join(key, 'done'))) {
    console.log(`choir-shot: build cached (${dist})`);
    return dist;
  }
  const depsModules = path.join(deps, 'node_modules');
  const missing = missingDependencies(frontend, depsModules);
  if (missing.length) {
    throw new Error(`the locked dependency tree ${depsModules} lacks ${missing.join(', ')}: package.json changed since the tree was built, so the shots need an npm ci round with network "ecosystem_proxy" (a build job does that); nothing here can install them`);
  }
  const work = path.join(key, 'work');
  fs.rmSync(key, { recursive: true, force: true });
  fs.mkdirSync(work, { recursive: true });
  for (const e of fs.readdirSync(frontend)) {
    if (['node_modules', 'dist', 'tests', '.vite'].includes(e)) continue;
    fs.cpSync(path.join(frontend, e), path.join(work, e), { recursive: true });
  }
  fs.symlinkSync(depsModules, path.join(work, 'node_modules'));
  const t = Date.now();
  const r = spawnSync(path.join(work, 'node_modules', '.bin', 'vite'), ['build', '--outDir', dist, '--emptyOutDir'], { cwd: work, stdio: 'inherit' });
  fs.rmSync(work, { recursive: true, force: true });
  if (r.status !== 0) throw new Error(`vite build failed (exit ${r.status}); the full npm run build prints the same error`);
  console.log(`choir-shot: built in ${((Date.now() - t) / 1000).toFixed(1)} s`);
  fs.writeFileSync(path.join(key, 'done'), ''); // a build killed midway has no marker and is redone
  for (const d of fs.readdirSync(cacheRoot)) if (path.join(cacheRoot, d) !== key) fs.rmSync(path.join(cacheRoot, d), { recursive: true, force: true });
  return dist;
}

// missingDependencies names each package.json dependency and dev that the
// locked tree does not hold, so a stale tree fails here with its cause.
export function missingDependencies(frontend, depsModules) {
  const pkg = JSON.parse(fs.readFileSync(path.join(frontend, 'package.json'), 'utf8'));
  return Object.keys({ ...pkg.dependencies, ...pkg.devDependencies }).filter((n) => !fs.existsSync(path.join(depsModules, n, 'package.json')));
}

const TYPES = { '.html': 'text/html', '.js': 'text/javascript', '.mjs': 'text/javascript', '.css': 'text/css', '.json': 'application/json', '.svg': 'image/svg+xml', '.png': 'image/png', '.woff2': 'font/woff2', '.ico': 'image/x-icon', '.webmanifest': 'application/manifest+json' };

// resetDesktop empties the owner's persisted desktop (PUT /api/desktop/state with
// no windows, as the driver; the shell's own save path, the store converges every
// session on it) so a shot starts from a clean desktop. The shell restores saved
// windows on load, and the autoputer keeps them across shots, so without this
// shot N shows the windows of shots 1..N-1. `put` is injectable for the unit test.
export const DESKTOP_STATE_PATH = '/api/desktop/state';
export async function resetDesktop(base, put = fetch) {
  const res = await put(base + DESKTOP_STATE_PATH, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json', 'X-Choir-Driver': 'true' },
    body: JSON.stringify({ windows: [], active_window_id: '', driver: true }),
  });
  if (!res.ok) throw new Error(`desktop reset failed (${res.status})`);
}

function proxy(req, res) {
  const headers = { ...req.headers, host: `127.0.0.1:${AUTOPUTER_PORT}`, 'x-authenticated-user': AUTOPUTER_USER };
  const up = http.request({ host: '127.0.0.1', port: AUTOPUTER_PORT, method: req.method, path: req.url, headers }, (r) => {
    res.writeHead(r.statusCode, r.headers);
    r.pipe(res);
  });
  up.on('error', (e) => { if (!res.headersSent) res.writeHead(502, { 'content-type': 'text/plain' }); res.end(`autoputer: ${e.message}`); });
  res.on('close', () => up.destroy());
  req.pipe(up);
}

// tunnel carries a WebSocket upgrade under /api (the live stream) to the
// autoputer as the owner; anything else is refused.
function tunnel(req, socket, head) {
  const u = new URL(req.url, 'http://x').pathname;
  if (!isProxied(u)) { socket.destroy(); return; }
  const up = net.connect(AUTOPUTER_PORT, '127.0.0.1', () => {
    const lines = [`${req.method} ${req.url} HTTP/1.1`];
    const h = { ...req.headers, host: `127.0.0.1:${AUTOPUTER_PORT}`, 'x-authenticated-user': AUTOPUTER_USER };
    for (const [k, v] of Object.entries(h)) lines.push(`${k}: ${v}`);
    up.write(lines.join('\r\n') + '\r\n\r\n');
    if (head && head.length) up.write(head);
    up.pipe(socket);
    socket.pipe(up);
  });
  up.on('error', () => socket.destroy());
  socket.on('error', () => up.destroy());
  socket.on('close', () => up.destroy());
}

function serve(dist, computer = false, themeState = { preset: null }, catalog = null, install = null, installState = { adoptions: [] }) {
  const server = http.createServer((req, res) => {
    const url = new URL(req.url, 'http://x');
    const u = decodeURIComponent(url.pathname);
    const themed = themeAnswer(themeState.preset, req.method, u);
    if (themed) { res.writeHead(200, { 'content-type': 'application/json' }).end(JSON.stringify(themed)); return; }
    const listed = catalogAnswer(catalog, req.method, u, url.search);
    if (listed) {
      themeState.fixtureServed = true; // the badge rests on this, not on the page's response event
      res.writeHead(listed.status, { 'content-type': 'application/json', 'cache-control': 'no-store', [CATALOG_FIXTURE_HEADER]: 'capsule-catalog' }).end(JSON.stringify(listed.body));
      return;
    }
    if (install && catalog && isInstallPath(u)) {
      const chunks = [];
      req.on('data', (c) => chunks.push(c));
      req.on('end', () => {
        const a = installAnswer(install, catalog, installState, req.method, u, Buffer.concat(chunks).toString('utf8'));
        themeState.fixtureServed = true;
        res.writeHead(a.status, { 'content-type': 'application/json', 'cache-control': 'no-store', [CATALOG_FIXTURE_HEADER]: 'capsule-install' }).end(JSON.stringify(a.body));
      });
      return;
    }
    if (computer && isProxied(u)) return proxy(req, res);
    if (computer && u === '/auth/session') {
      res.writeHead(200, { 'content-type': 'application/json' }).end(JSON.stringify({ authenticated: true, user: { id: AUTOPUTER_USER, username: AUTOPUTER_USER } }));
      return;
    }
    if (computer && u.startsWith('/auth/')) { res.writeHead(404).end('not found'); return; }
    let f = path.join(dist, path.normalize(u));
    if (!f.startsWith(dist)) { res.writeHead(403).end(); return; }
    if (!fs.existsSync(f) || fs.statSync(f).isDirectory()) {
      if (path.extname(u)) { res.writeHead(404).end('not found'); return; } // a missing asset or an unmocked /api call
      f = path.join(dist, 'index.html'); // the SPA shell
    }
    res.writeHead(200, { 'content-type': TYPES[path.extname(f)] || 'application/octet-stream' });
    fs.createReadStream(f).pipe(res);
  });
  if (computer) server.on('upgrade', (req, socket, head) => tunnel(req, socket, head));
  return new Promise((resolve) => server.listen(0, '127.0.0.1', () => resolve({ server, base: `http://127.0.0.1:${server.address().port}` })));
}

// selectorTimeoutMessage: a shot's selector that never appears fails the shot
// (so --fail-fast counts it) naming the selector and what the page shows, so
// the engineer fixes the selector instead of retrying a 15 s wait.
export async function selectorTimeoutMessage(page, selector, err, ms = STEP_MS) {
  let text = '';
  try { text = String(await page.evaluate(() => (document.body && document.body.innerText) || '')).replace(/\s+/g, ' ').trim().slice(0, 200); } catch { /* page gone */ }
  const why = /Timeout/i.test(String(err && err.message)) ? `never appeared within ${ms / 1000} s` : String(err && err.message).split('\n')[0];
  return `selector ${JSON.stringify(selector)} ${why}: check the selector matches the app's DOM; the page shows: ${text || '(no visible text)'}`;
}

async function settle(page, selector) {
  await page.waitForLoadState('networkidle', { timeout: STEP_MS });
  if (selector) {
    try { await page.waitForSelector(selector, { state: 'visible', timeout: STEP_MS }); } catch (e) {
      throw new Error(await selectorTimeoutMessage(page, selector, e));
    }
  }
  await Promise.race([page.evaluate(() => document.fonts && document.fonts.ready), new Promise((r) => setTimeout(r, STEP_MS))]);
}

// loadPng decodes a PNG with pngjs from the locked tree (or beside this
// script); null when pngjs is absent.
function loadPngDecoder(req) {
  for (const r of [req, createRequire(import.meta.url)]) {
    try { const { PNG } = r('pngjs'); return (buf) => PNG.sync.read(buf); } catch { /* try the next */ }
  }
  return null;
}

// measureContrast scores the text under selector on a ready page. Text is made
// transparent for one screenshot so pixel sampling sees only the background.
async function measureContrast(page, it, decode, out, appId) {
  const nodes = await page.evaluate(collectTextNodes, { selector: it.contrast, limit: 400, appId });
  let png = null;
  if (decode && nodes.length) {
    const style = await page.addStyleTag({ content: '*, *::before, *::after { color: transparent !important; -webkit-text-fill-color: transparent !important; text-shadow: none !important; caret-color: transparent !important; }' });
    try {
      png = decode(await page.screenshot({ fullPage: true, scale: 'css' }));
    } finally {
      await style.evaluate((el) => el.remove());
    }
  }
  const { report, line, detail } = summarizeContrast({ shot: it.shot.name, preset: it.theme, file: it.file, selector: it.contrast, opened: appId || null, viewport: it.viewport, wcag: 'AA' }, nodes, png);
  const dir = contrastDir(out);
  fs.mkdirSync(dir, { recursive: true });
  fs.writeFileSync(path.join(dir, it.contrastFile), JSON.stringify(report, null, 2) + '\n');
  console.log(`choir-shot: ${line}`);
  for (const d of detail) console.log(`choir-shot: ${d}`);
}

// verdictLine is the one line a run ends with, so a failed run never reads as
// done when its output is cut or piped: "choir-shot: OK n/n shots" or
// "choir-shot: FAILED k/n: <first error>".
export function verdictLine(total, failures, skipped = 0) {
  if (failures.length === 0) return `choir-shot: OK ${total}/${total} shots`;
  const tail = skipped ? ` (${skipped} not run: fail-fast)` : '';
  return `choir-shot: FAILED ${failures.length}/${total}: ${failures[0].file}: ${failures[0].error}${tail}`;
}

// failFastFor: a run of the whole set stops at its first failure (a 12-shot
// run is about 5 minutes, and the later shots of a broken page or backend fail
// the same way); a run with --only names the shots it wants and is short, so
// it takes them all. --fail-fast and --keep-going override either way.
export function failFastFor(o) {
  return o.failFast ?? o.only.length === 0;
}

// runShots takes each item in turn, printing its OK or FAILED line the moment
// it finishes (stdout, so a job's tail shows progress), and under failFast
// stops at the first failure. runOne throws to fail a shot.
export async function runShots(items, runOne, { failFast, log = console.log }) {
  const failures = [];
  let skipped = 0;
  for (let i = 0; i < items.length; i++) {
    const it = items[i];
    const t = Date.now();
    try {
      await runOne(it);
      log(`choir-shot: shot ${i + 1}/${items.length} OK ${it.file} ${((Date.now() - t) / 1000).toFixed(1)} s`);
    } catch (e) {
      const msg = String(e.message || e).split('\n')[0];
      failures.push({ file: it.file, error: msg });
      log(`choir-shot: shot ${i + 1}/${items.length} FAILED ${it.file}: ${msg}`);
      if (failFast) {
        skipped = items.length - i - 1;
        if (skipped) log(`choir-shot: --fail-fast: stopped after ${it.file}; ${skipped} shot${skipped === 1 ? '' : 's'} not run (--keep-going takes them all)`);
        break;
      }
    }
  }
  return { failures, skipped };
}

async function main() {
  const root = process.env.CHOIR_PLATFORM_ROOT || path.resolve(path.dirname(new URL(import.meta.url).pathname), '..');
  const frontend = path.join(root, 'frontend');
  let known = null;
  try { known = themeIdsFrom(fs.readFileSync(path.join(frontend, 'src', 'lib', 'theme.ts'), 'utf8')); } catch { /* no source to check against */ }
  if (known && known.length === 0) known = null;
  const o = parseArgs(process.argv.slice(2), known);
  if (o.help) {
    console.log(USAGE);
    return 0;
  }
  const mod = await import(pathToFileURL(path.resolve(o.shots)).href);
  const items = plan(mod.default, o, known);
  if (o.list) {
    for (const it of items) console.log(it.file);
    return 0;
  }
  const deps = process.env.CHOIR_FRONTEND_DEPS;
  if (!deps) throw new Error('CHOIR_FRONTEND_DEPS is not set (a capsule job has it)');
  const exe = process.env.CHOIR_BROWSER;
  if (!exe) throw new Error('CHOIR_BROWSER is not set (a capsule job has the store Chromium)');
  const req = createRequire(path.join(deps, 'package.json'));
  let pw;
  try { pw = req('playwright'); } catch { pw = req('@playwright/test'); } // npm hoists playwright; either exports chromium
  const { chromium } = pw;
  const dist = build(frontend, deps);
  const useComputer = o.computer && fs.existsSync(path.join(COMPUTER_DIR, 'state.db'));
  const holder = { child: null, log: autoputerLogPath(o.out) }; // handlers first: a signal during the build or the wait must not orphan the child
  const stopAutoputer = () => { if (holder.child && holder.child.exitCode === null) holder.child.kill(); };
  process.on('exit', stopAutoputer);
  for (const sig of ['SIGINT', 'SIGTERM']) process.on(sig, () => { stopAutoputer(); process.exit(130); });
  if (useComputer) {
    try { await startAutoputer(root, holder); } catch (e) {
      console.error(`choir-shot: the autoputer's output is in ${holder.log}`);
      throw e;
    }
  }
  const themeState = { preset: null, fixtureServed: false };
  const catalog = loadCatalogFixture();
  if (!catalog) console.warn(`choir-shot: no catalog fixture at ${CATALOG_FIXTURE_PATH}; /api/catalog/* is unanswered in a capsule`);
  const install = loadInstallFixture();
  if (!install) console.warn(`choir-shot: no install fixture at ${INSTALL_FIXTURE_PATH}; install (self-development/adoptions) is refused in a capsule`);
  const installState = { adoptions: [] };
  const { server, base } = await serve(dist, useComputer, themeState, catalog, install, installState);
  const decode = items.some((it) => it.contrast) ? loadPngDecoder(req) : null;
  fs.mkdirSync(o.out, { recursive: true });
  const browser = await chromium.launch({ executablePath: exe, args: ['--no-sandbox', '--disable-dev-shm-usage'] });
  let failures = [];
  let skipped = 0;
  // A PNG from an earlier run is never left for a shot that fails or is not run (--fail-fast): Freeze would admit it.
  for (const it of items) { try { fs.rmSync(path.join(o.out, it.file), { force: true }); } catch { /* none */ } }
  try {
    ({ failures, skipped } = await runShots(items, async (it) => {
      const context = await browser.newContext({ viewport: it.viewport, ...it.device });
      try {
        const s = it.shot;
        themeState.preset = it.theme;
        themeState.fixtureServed = false;
        installState.adoptions.length = 0; // the installed list is per shot, emptied with the desktop
        if (useComputer) await resetDesktop(base);
        if (it.theme) await context.addInitScript(themeSeedScript(it.theme));
        if (s.routes) await s.routes(context, base);
        const page = await context.newPage();
        let readFixture = false;
        page.on('response', (r) => { if (r.headers()[CATALOG_FIXTURE_HEADER]) readFixture = true; });
        if (s.before) await s.before(page, base);
        await page.goto(base + (s.path || '/'), { waitUntil: 'load', timeout: STEP_MS });
        await settle(page, it.open ? null : s.selector);
        let appId = '';
        if (it.open) {
          appId = await openApp(page, it.open);
          await settle(page, s.selector);
        }
        if (s.act) {
          await s.act(page);
          await settle(page, s.selector);
        }
        if (appId && s.expect_window !== false) { await waitSettled(page, appId); await checkAppWindow(page, appId); } // before the screenshot: a failed shot leaves no PNG for Freeze to admit
        if (readFixture || themeState.fixtureServed) await page.evaluate(fixtureBadge, CATALOG_FIXTURE_BADGE);
        await page.screenshot({ path: path.join(o.out, it.file), fullPage: !!s.fullPage });
        if (readFixture || themeState.fixtureServed) {
          await page.evaluate(() => document.querySelectorAll('[data-choir-shot-fixture]').forEach((el) => el.remove()));
          console.log(`choir-shot: ${it.file} shows a fixture (${CATALOG_FIXTURE_PATH}, ${INSTALL_FIXTURE_PATH}), not the live catalog or computer`);
        }
        if (it.contrast) await measureContrast(page, it, decode, o.out, appId);
      } finally {
        await context.close();
      }
    }, { failFast: failFastFor(o) }));
  } finally {
    await browser.close();
    server.close();
    stopAutoputer();
  }
  const have = fs.readdirSync(o.out).filter((f) => f.endsWith('.png')).length;
  if (have > 12) console.warn(`choir-shot: ${have} PNGs in ${o.out}; Freeze takes at most 12`);
  const failed = failures.map((f) => f.file);
  if (failed.length && useComputer) console.error(`choir-shot: the autoputer's output is in ${holder.log}`);
  console.log(JSON.stringify({ shots: items.length - failed.length - skipped, failed, skipped, out: o.out }));
  const verdict = verdictLine(items.length, failures, skipped);
  console.log(verdict); // stdout, after the JSON summary: the last line of the run
  return failed.length ? 1 : 0;
}

if (import.meta.url === pathToFileURL(process.argv[1] || '').href) {
  main().then((c) => process.exit(c), (e) => { console.error(`choir-shot: ${e.message}`); console.error(`choir-shot: FAILED 0/0: ${String(e.message).split('\n')[0]}`); process.exit(2); });
}
