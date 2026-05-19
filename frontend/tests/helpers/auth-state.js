import fs from 'fs';
import path from 'path';
import { fileURLToPath } from 'url';
import { getSession, registerPasskey } from './auth.js';
import { setupVirtualAuthenticator, removeVirtualAuthenticator } from './webauthn.js';

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);
const FRONTEND_ROOT = path.resolve(__dirname, '../..');

const DEFAULT_BASE_URL =
  process.env.BASE_URL ||
  process.env.PLAYWRIGHT_BASE_URL ||
  process.env.GO_CHOIR_CONTENT_BASE_URL ||
  'http://localhost:4173';
const DEFAULT_DESKTOP_READY_TIMEOUT_MS = desktopReadyTimeout();

function defaultStatePath(baseURL) {
  const host = new URL(baseURL).hostname.replaceAll('.', '-');
  return path.join(FRONTEND_ROOT, 'playwright', '.auth', `${host}.storage.json`);
}

function statePaths(baseURL) {
  const storageStatePath = path.resolve(process.env.CHOIR_AUTH_STATE || defaultStatePath(baseURL));
  const metaPath = path.resolve(
    process.env.CHOIR_AUTH_META ||
      storageStatePath.replace(/\.json$/i, '.meta.json'),
  );
  return {
    storageStatePath,
    metaPath,
    lockPath: `${storageStatePath}.lock`,
  };
}

function uniqueEmail() {
  return `playwright-state-${Date.now()}-${Math.random().toString(36).slice(2, 8)}@example.com`;
}

function sleep(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

function desktopReadyTimeout() {
  const value = Number(process.env.CHOIR_DESKTOP_READY_TIMEOUT_MS || process.env.CHOIR_DESKTOP_READY_TIMEOUT);
  return Number.isFinite(value) && value > 0 ? value : 120_000;
}

export async function waitForDesktopReady(page, timeout = DEFAULT_DESKTOP_READY_TIMEOUT_MS) {
  await page.locator('[data-desktop][data-authenticated="true"][data-desktop-ready="true"]').waitFor({
    state: 'visible',
    timeout,
  });
}

function readStoredState(baseURL) {
  const { storageStatePath, metaPath } = statePaths(baseURL);
  const meta = JSON.parse(fs.readFileSync(metaPath, 'utf8'));
  return {
    email: meta.email,
    baseURL: meta.baseURL,
    storageStatePath,
    metaPath,
  };
}

async function validateStoredState(browser, baseURL) {
  const { storageStatePath, metaPath } = statePaths(baseURL);
  if (!fs.existsSync(storageStatePath)) return null;

  const context = await browser.newContext({ storageState: storageStatePath });
  const page = await context.newPage();
  try {
    await page.goto(baseURL);
    const session = await getSession(page, baseURL);
    if (!session.authenticated) return null;

    await context.storageState({ path: storageStatePath });
    fs.writeFileSync(
      metaPath,
      JSON.stringify({
        email: session.user?.email || null,
        baseURL,
        user: session.user || null,
      }, null, 2),
      'utf8',
    );
    return readStoredState(baseURL);
  } catch {
    return null;
  } finally {
    await context.close();
  }
}

async function waitForStoredState(browser, baseURL, timeoutMs = 30000) {
  const { storageStatePath } = statePaths(baseURL);
  const deadline = Date.now() + timeoutMs;
  while (Date.now() < deadline) {
    if (fs.existsSync(storageStatePath)) {
      const state = await validateStoredState(browser, baseURL);
      if (state) return state;
    }
    await sleep(250);
  }
  throw new Error('timed out waiting for shared Playwright auth state');
}

export async function createAuthenticatedState(browser, baseURL = DEFAULT_BASE_URL) {
  const { storageStatePath, metaPath, lockPath } = statePaths(baseURL);
  fs.mkdirSync(path.dirname(storageStatePath), { recursive: true });
  fs.mkdirSync(path.dirname(metaPath), { recursive: true });

  const existing = await validateStoredState(browser, baseURL);
  if (existing) return existing;

  let lockFd = null;
  try {
    lockFd = fs.openSync(lockPath, 'wx');
  } catch (err) {
    if (err && err.code === 'EEXIST') {
      return waitForStoredState(browser, baseURL);
    }
    throw err;
  }

  const stateAfterLock = await validateStoredState(browser, baseURL);
  if (stateAfterLock) {
    fs.closeSync(lockFd);
    fs.rmSync(lockPath, { force: true });
    return stateAfterLock;
  }

  const context = await browser.newContext();
  const page = await context.newPage();
  const { client, authenticatorId } = await setupVirtualAuthenticator(page);
  const email = uniqueEmail();

  try {
    await page.goto(baseURL);
    await registerPasskey(page, email, baseURL);
    await page.reload();
    await waitForDesktopReady(page);

    await context.storageState({ path: storageStatePath });
    fs.writeFileSync(
      metaPath,
      JSON.stringify({ email, baseURL }, null, 2),
      'utf8',
    );

    return {
      email,
      baseURL,
      storageStatePath,
      metaPath,
    };
  } finally {
    await removeVirtualAuthenticator(client, authenticatorId);
    await context.close();
    if (lockFd !== null) {
      fs.closeSync(lockFd);
      fs.rmSync(lockPath, { force: true });
    }
  }
}
