import fs from 'node:fs';
import net from 'node:net';
import { spawn } from 'node:child_process';
import { chromium, expect, test } from '@playwright/test';

test.setTimeout(120_000);
test.skip(
  process.env.GO_CHOIR_RUN_OBSCURA_CDP !== '1',
  'set GO_CHOIR_RUN_OBSCURA_CDP=1 to verify the Obscura CDP screenshot substrate'
);

function delay(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

function pickPort() {
  return new Promise((resolve, reject) => {
    const server = net.createServer();
    server.on('error', reject);
    server.listen(0, '127.0.0.1', () => {
      const address = server.address();
      server.close(() => resolve(address.port));
    });
  });
}

async function waitForVersion(port, proc, logs) {
  for (let attempt = 0; attempt < 80; attempt += 1) {
    if (proc.exitCode !== null) {
      throw new Error(`obscura serve exited early (${proc.exitCode}): ${logs()}`);
    }
    try {
      const response = await fetch(`http://127.0.0.1:${port}/json/version`);
      if (response.ok) {
        return await response.json();
      }
    } catch {
      // Keep polling until the CDP endpoint is ready.
    }
    await delay(250);
  }
  throw new Error(`timed out waiting for obscura CDP endpoint: ${logs()}`);
}

async function stopProcess(proc) {
  if (proc.exitCode !== null) return;
  proc.kill('SIGINT');
  await Promise.race([
    new Promise((resolve) => proc.once('exit', resolve)),
    delay(3000).then(() => {
      if (proc.exitCode === null) {
        proc.kill('SIGKILL');
      }
    }),
  ]);
}

test('Obscura CDP serve can render a real screenshot substrate', async () => {
  const bin =
    process.env.CHOIR_OBSCURA_BIN ||
    process.env.OBSCURA_BIN ||
    '/Users/wiz/obscura/target/release/obscura';
  test.skip(!fs.existsSync(bin), `Obscura binary not found at ${bin}`);

  const port = await pickPort();
  const proc = spawn(bin, ['serve', '--port', String(port), '--workers', '1'], {
    stdio: ['ignore', 'pipe', 'pipe'],
  });
  const output = [];
  proc.stdout.on('data', (chunk) => output.push(chunk.toString()));
  proc.stderr.on('data', (chunk) => output.push(chunk.toString()));
  const logs = () => output.join('').slice(-2000);

  let browser;
  try {
    const version = await waitForVersion(port, proc, logs);
    expect(version.webSocketDebuggerUrl).toContain(`127.0.0.1:${port}`);

    browser = await chromium.connectOverCDP(version.webSocketDebuggerUrl);
    const page = await browser.newPage();
    await page.goto('https://example.com', { waitUntil: 'load', timeout: 20_000 });

    await expect(page).toHaveTitle('Example Domain');
    const screenshot = await page.screenshot({ fullPage: true });
    expect(screenshot.length).toBeGreaterThan(1000);
    expect(screenshot.subarray(0, 8).toString('hex')).toBe('89504e470d0a1a0a');
  } finally {
    if (browser) {
      await browser.close();
    }
    await stopProcess(proc);
  }
});
