import { expect, test } from '@playwright/test';
import fs from 'node:fs/promises';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const frontendRoot = path.resolve(__dirname, '..');

const syncedStateFiles = [
  'src/App.svelte',
  'src/lib/AudioApp.svelte',
  'src/lib/CandidateDesktopViewer.svelte',
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
