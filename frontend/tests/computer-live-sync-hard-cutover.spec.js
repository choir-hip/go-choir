import { expect, test } from '@playwright/test';
import fs from 'node:fs/promises';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const frontendRoot = path.resolve(__dirname, '..');

const syncedStateFiles = [
  'src/App.svelte',
  'src/lib/AppsChangesApp.svelte',
  'src/lib/AudioApp.svelte',
  'src/lib/ChangePreviewFrame.svelte',
  'src/lib/ComputeMonitorApp.svelte',
  'src/lib/Desktop.svelte',
  'src/lib/EpubApp.svelte',
  'src/lib/FileBrowser.svelte',
  'src/lib/ImageApp.svelte',
  'src/lib/PdfApp.svelte',
  'src/lib/PodcastApp.svelte',
  'src/lib/SettingsApp.svelte',
  'src/lib/VTextEditor.svelte',
  'src/lib/VideoApp.svelte',
  'src/lib/media-utils.js',
  'src/lib/preferences.js',
  'src/lib/theme.js',
];

const coveredRefreshFiles = syncedStateFiles.filter((file) => file !== 'src/lib/theme.js');

async function readRelative(file) {
  return fs.readFile(path.join(frontendRoot, file), 'utf8');
}

test('synced computer state does not use browser storage compatibility', async () => {
  for (const file of syncedStateFiles) {
    const source = await readRelative(file);
    expect(source, `${file} should not call localStorage/sessionStorage for synced state`).not.toMatch(
      /\b(?:localStorage|sessionStorage)\s*\.(?:getItem|setItem|removeItem|clear)\b/,
    );
  }
});

test('covered product apps do not expose manual refresh or reload sync controls', async () => {
  for (const file of coveredRefreshFiles) {
    const source = await readRelative(file);
    expect(source, `${file} should not expose manual data Refresh/Reload controls`).not.toMatch(
      /(?:>\s*(?:Refresh|Refreshing|Reload)\s*<|aria-label="(?:Refresh|Reload)(?:\s[^"]*)?"|title="(?:Refresh|Reload)(?:\s[^"]*)?")/,
    );
  }
});

test('desktop live state cannot seize the visible foreground window stack', async () => {
  const source = await readRelative('src/lib/Desktop.svelte');

  expect(source).toContain('function handleRemoteDesktopStateUpdate(message = {})');
  expect(source).toContain("document.visibilityState === 'hidden'");
  expect(source).toContain('mergeRemoteDesktopSharedState();');
  expect(source).toContain('observeRemoteDriverSession');
  expect(source).toContain('handleRemoteDesktopStateUpdate(message);');
  expect(source).not.toMatch(
    /message\.kind === 'desktop\.state\.updated'[\s\S]{0,120}void loadDesktopState\(\);/,
  );
  expect(source).not.toContain("message.kind === 'desktop.state.updated'");
});

test('server-applied desktop state does not echo-save from store subscriptions', async () => {
  const source = await readRelative('src/lib/Desktop.svelte');

  expect(source).toContain('applyingPersistedDesktopState = true');
  expect(source).toContain('applyPersistedDesktopState(() =>');
  expect(source).toContain('stateLoaded && !applyingPersistedDesktopState');
});

test('desktop saves are gated by the local driver lease', async () => {
  const desktopSource = await readRelative('src/lib/Desktop.svelte');
  const apiSource = await readRelative('src/lib/desktop.js');
  const liveSource = await readRelative('src/lib/live-events.js');

  expect(liveSource).toContain('currentSessionId');
  expect(liveSource).toContain('renewDriverLease');
  expect(liveSource).toContain('observeRemoteDriverSession');
  expect(apiSource).toContain('X-Choir-Session');
  expect(apiSource).toContain('X-Choir-Viewport');
  expect(apiSource).toContain('driver: isDrivingSession()');
  expect(desktopSource).toContain('if (!isDrivingSession()) return;');
});
