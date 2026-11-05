// Static `uses` extraction and the lock sections, slice U1 (yellow: proof
// tooling). The extractor reads imports of the SDK from package source; the
// build fails when the grants those imply exceed the manifest's. The sections
// (pipelines, services, contributes, ctx, grants) are locked and diffed.
import { test, expect } from '@playwright/test';
import { spawnSync } from 'node:child_process';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { extractUses, sdkImportsOf, usesProblems, readSections } from '../scripts/package-uses.mjs';
import { compareSurfaces, extractSurface } from '../scripts/sdk-surface.mjs';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');

test('sdkImportsOf reads named, multi-line, aliased and type imports, and flags a namespace import', () => {
  expect([...sdkImportsOf("import { a, b as c,\n  d } from '../../lib/app-sdk';\nimport type { T } from '../../lib/app-sdk';\nimport { z } from './local';")].sort()).toEqual(['a', 'b', 'd']);
  expect([...sdkImportsOf("import * as sdk from '../../lib/app-sdk';")]).toEqual(['*']);
  expect([...sdkImportsOf("import { x } from 'svelte';")]).toEqual([]);
});

test('every in-tree package has its uses extracted, and none exceeds its grants', () => {
  const sections = readSections(root);
  for (const name of fs.readdirSync(path.join(root, 'src/packages'))) {
    const dir = path.join(root, 'src/packages', name);
    const uses = extractUses(dir, sections);
    expect(uses.sdk.length, name).toBeGreaterThan(0);
    expect(usesProblems(uses, JSON.parse(fs.readFileSync(path.join(dir, 'choir-package.json'), 'utf8'))), name).toEqual([]);
  }
});

test('a grant-implying use without its grant fails the build gate; with it, passes', () => {
  const run = (declare) => {
    const dir = fs.mkdtempSync(path.join(fs.realpathSync(process.env.TMPDIR || '/tmp'), 'choir-uses-'));
    fs.cpSync(root, dir, { recursive: true, filter: (src) => !/node_modules|dist|test-results/.test(src) });
    fs.symlinkSync(path.join(root, 'node_modules'), path.join(dir, 'node_modules'));
    const app = path.join(dir, 'src/packages/image/App.svelte');
    fs.writeFileSync(app, fs.readFileSync(app, 'utf8').replace('<script', "<script context=\"module\">\n  import { submitPrompt } from '../../lib/app-sdk';\n  export const sendIt = submitPrompt;\n</script>\n<script"));
    if (declare) {
      const p = path.join(dir, 'src/packages/image/choir-package.json');
      const m = JSON.parse(fs.readFileSync(p, 'utf8'));
      m.capabilities = [{ grant: 'agent.prompt' }];
      fs.writeFileSync(p, JSON.stringify(m, null, 2));
    }
    const r = spawnSync(process.execPath, ['scripts/check-packages.mjs'], { cwd: dir, encoding: 'utf8' });
    fs.rmSync(dir, { recursive: true, force: true });
    return { status: r.status, out: r.stdout + r.stderr };
  };
  const without = run(false);
  expect(without.status).not.toBe(0);
  expect(without.out).toMatch(/uses imply grant "agent\.prompt"/);
  const withGrant = run(true);
  expect(withGrant.status, withGrant.out).toBe(0);
});

test('the lock diff names a section entry that left, moved or joined', () => {
  const surface = extractSurface({ frontendRoot: root });
  const clone = (o) => JSON.parse(JSON.stringify(o));
  const gone = clone(surface); delete gone.sections.pipelines['files.openWith'];
  expect(compareSurfaces(surface, gone, { frontendRoot: root })).toMatchObject({ required: 'major', removed: ['section:pipelines.files.openWith.payload'] });
  const moved = clone(surface); moved.sections.services.theme.abi = 'choir.service/theme/v2';
  expect(compareSurfaces(surface, moved, { frontendRoot: root }).required).toBe('major');
  const joined = clone(surface); joined.sections.ctx.push('zzz');
  expect(compareSurfaces(surface, joined, { frontendRoot: root })).toMatchObject({ required: 'minor', added: ['section:ctx.zzz'] });
  const ctxGone = clone(surface); ctxGone.sections.ctx = ctxGone.sections.ctx.filter((k) => k !== 'sdk');
  expect(compareSurfaces(surface, ctxGone, { frontendRoot: root }).required).toBe('major');
});
