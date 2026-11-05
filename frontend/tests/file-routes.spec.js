// One file-route table, slice S3. The differential: Open with answers for
// every extension and media type the shell knows are equal to what they were
// before the static routes became one table (tests/fixtures/open-with-golden.json,
// written from the code before the change), and the structural gate reads the
// table as data.
//
// Failure modes:
//  R1 a route moved to the table changes an Open with answer (order, default,
//     media type, the list under a media type);
//  R2 a default-false row becomes a click route (or the reverse);
//  R3 the clash gate stops seeing the static extensions (it reads data now).
import { test, expect } from '@playwright/test';
import { spawnSync } from 'node:child_process';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { createServer } from 'vite';
import { openWithSnapshot } from './helpers/open-with-snapshot.mjs';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
let vite;
let mods;
test.beforeAll(async () => {
  vite = await createServer({ root, server: { middlewareMode: true, hmr: false, watch: null }, appType: 'custom', logLevel: 'silent' });
  mods = {
    openWith: await vite.ssrLoadModule('/src/lib/open-with.ts'),
    media: await vite.ssrLoadModule('/src/lib/media-utils.js'),
    registry: await vite.ssrLoadModule('/src/lib/apps/registry.ts'),
  };
});
test.afterAll(async () => { await vite?.close(); });

test('R1, R2: openersForFile and its siblings equal the golden before-answers', () => {
  const golden = JSON.parse(fs.readFileSync(path.join(root, 'tests/fixtures/open-with-golden.json'), 'utf8'));
  const now = JSON.parse(JSON.stringify(openWithSnapshot(mods)));
  expect(now).toEqual(golden);
});

test('R3: the clash gate reads the table, and refuses a package that claims a static extension', () => {
  const dir = fs.mkdtempSync(path.join(fs.realpathSync(process.env.TMPDIR || '/tmp'), 'choir-routes-'));
  fs.cpSync(root, dir, { recursive: true, filter: (src) => !/node_modules|dist|test-results/.test(src) });
  fs.symlinkSync(path.join(root, 'node_modules'), path.join(dir, 'node_modules'));
  const manifestPath = path.join(dir, 'src/packages/image/choir-package.json');
  const manifest = JSON.parse(fs.readFileSync(manifestPath, 'utf8'));
  manifest.opens.push({ media_type: 'audio/mpeg', extensions: ['mp3'] });
  fs.writeFileSync(manifestPath, JSON.stringify(manifest, null, 2));
  const r = spawnSync(process.execPath, ['scripts/check-packages.mjs'], { cwd: dir, encoding: 'utf8' });
  fs.rmSync(dir, { recursive: true, force: true });
  expect(r.status).not.toBe(0);
  expect(r.stdout + r.stderr).toMatch(/extension "mp3" is still routed by the static file-routes\.json/);
});
