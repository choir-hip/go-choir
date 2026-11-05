/**
 * Deployed staging proof: outbound mail attachments end-to-end on choir.news.
 *
 * Registers a fresh passkey account (virtual authenticator), provisions a
 * numbered plus-code alias for it via host maildctl (reserve-test-number, bind-number) (test fixture — fresh accounts get
 * no alias by default), then drives the real Mail app: compose → attach one
 * client-uploaded file and one autoputer-FS file → approve → send to the
 * account's own alias and 000@choir.news → verify the inbound message lands
 * with attachment metadata. Zero mocks: real Resend send + real webhook
 * inbound on the deployed product path.
 *
 * Run: BASE_URL=https://choir.news npx playwright test mail-attachments-deployed
 */
import { test, expect } from './helpers/fixtures.js';
import { registerPasskey, getSession } from './helpers/auth.js';
import { execSync } from 'node:child_process';

const BASE_URL = process.env.BASE_URL || 'https://choir.news';
const ROOT_ADDR = '000@choir.news';

// Unique per run: aliases are global and cannot be re-owned. The alias lives
// under the fresh account's own reserved test number (NNN+uitest<RUN>); a
// plus-alias may be created only by the owner of its number.
const RUN_ID = `${Date.now().toString(36)}${Math.floor(Math.random() * 1296).toString(36)}`;
let ALIAS_LOCAL = '';
let ALIAS_ADDR = '';

function uniqueEmail() {
  return `mail-attach-${Date.now()}-${Math.floor(Math.random() * 1e6)}@example.com`;
}

function nodeB(cmd) {
  return execSync(`ssh -o ConnectTimeout=15 -o BatchMode=yes node-b "${cmd}"`, { stdio: 'pipe', timeout: 30_000 }).toString();
}

function provisionAlias(userID) {
  // Test fixture only: fresh accounts have no mail alias; the product has no
  // self-serve alias route. Reserve a test number, bind it to the fresh
  // account, then create <number>+uitest<RUN> host-side.
  const address = JSON.parse(nodeB('maildctl reserve-test-number')).address;
  const number = address.split('@')[0];
  nodeB(`maildctl bind-number --address ${address} --owner ${userID}`);
  ALIAS_LOCAL = `${number}+uitest${RUN_ID}`;
  ALIAS_ADDR = `${ALIAS_LOCAL}@choir.news`;
  nodeB(`maildctl configure-workflow-alias --owner ${userID} --local-part ${ALIAS_LOCAL} --sender ${ALIAS_ADDR}`);
}

async function apiFetch(page, path, options = {}) {
  return page.evaluate(
    async ({ path, options }) => {
      const res = await fetch(path, { credentials: 'include', ...options });
      const text = await res.text();
      let body = text;
      try {
        body = JSON.parse(text);
      } catch (_e) {
        /* raw bytes */
      }
      return { status: res.status, body };
    },
    { path, options },
  );
}

// The fresh account's VM cold-boots on first /api/files call; poll until up.
async function waitForFilesAPI(page, timeoutMs = 300_000) {
  const deadline = Date.now() + timeoutMs;
  let last = 0;
  while (Date.now() < deadline) {
    const res = await apiFetch(page, '/api/files');
    last = res.status;
    if (res.status === 200) return;
    await page.waitForTimeout(5000);
  }
  throw new Error(`/api/files never became ready (last status ${last})`);
}

test('deployed Mail app sends attachments through approval to inbound', async ({
  page,
  authenticator,
}) => {
  test.setTimeout(600_000);
  const email = uniqueEmail();

  // Register a fresh account on the deployed origin.
  await page.goto(BASE_URL);
  const reg = await registerPasskey(page, email, BASE_URL);
  expect(reg.ok || reg.user).toBeTruthy();
  const session = await getSession(page, BASE_URL);
  const userID = session.user.id;
  expect(userID).toBeTruthy();

  // Provision the send alias (fixture) and wait for the guest files API.
  provisionAlias(userID);
  await page.goto(BASE_URL);
  await page.locator('[data-desktop][data-authenticated="true"]').waitFor({
    state: 'visible',
    timeout: 120_000,
  });
  await waitForFilesAPI(page);

  // Stage a file in the autoputer filesystem for the From-Files picker.
  const fsBody = `autoputer fs attachment ${Date.now()}`;
  const put = await apiFetch(page, '/api/files/mail-attach-test.txt', {
    method: 'PUT',
    headers: { 'Content-Type': 'text/plain' },
    body: fsBody,
  });
  expect([200, 201]).toContain(put.status);
  // Open the Mail app from the desktop icon (id `mail`; `email` is only an alias).
  await page.goto(BASE_URL);
  await page.locator('[data-desktop-icon-id="mail"]').click();
  const mailApp = page.locator('[data-mail-app]').last();
  await expect(mailApp).toBeVisible({ timeout: 60_000 });

  // Compose.
  await mailApp.locator('button.mail-compose-btn[data-mail-compose]').click();
  const compose = mailApp.locator('[data-mail-compose-body]');
  await expect(compose).toBeVisible({ timeout: 15_000 });
  await mailApp.locator('[data-mail-compose-to], input[placeholder*="recipient" i], input[placeholder*="to" i]').first().fill(`${ALIAS_ADDR}, ${ROOT_ADDR}`);
  await mailApp.locator('[data-mail-compose-subject], input[placeholder*="subject" i]').first().fill(`attachment proof ${Date.now()}`);
  await compose.fill('attachment round-trip proof');

  // Regression: the OS file dialog blurs then refocuses the window, which fires
  // the app's focus/visibilitychange refresh. That background refresh must not
  // close the compose panel (it used to drop the user back to the inbox and
  // lose the attachment). Dispatch the same events the dialog triggers.
  await page.evaluate(() => {
    // Pin visibilityState so the handler's guard passes in headless runs too.
    Object.defineProperty(document, 'visibilityState', { get: () => 'visible', configurable: true });
    window.dispatchEvent(new Event('focus'));
    document.dispatchEvent(new Event('visibilitychange'));
  });
  await expect(compose).toBeVisible({ timeout: 5_000 });

  // Attach one client-uploaded file.
  const uploadBody = `client upload ${Date.now()}`;
  await mailApp.locator('[data-mail-attachment-input]').setInputFiles({
    name: 'client-upload.txt',
    mimeType: 'text/plain',
    buffer: Buffer.from(uploadBody),
  });
  await expect(mailApp.locator('[data-mail-staged-attachment]')).toHaveCount(1, { timeout: 15_000 });

  // Attach one file from the autoputer filesystem.
  await mailApp.locator('[data-mail-attachment-from-files]').click();
  const picker = mailApp.locator('[data-mail-files-picker]');
  await expect(picker).toBeVisible({ timeout: 15_000 });
  await picker.locator('[data-mail-file-picker-entry="mail-attach-test.txt"]').click();
  await expect(mailApp.locator('[data-mail-staged-attachment]')).toHaveCount(2, { timeout: 15_000 });

  // Save the draft (binds attachments), then approve/send.
  await mailApp.locator('[data-mail-compose-save]').click();
  await expect(mailApp.locator('[data-mail-send]')).toBeVisible({ timeout: 30_000 });
  await mailApp.locator('[data-mail-send]').click();
  await expect(mailApp.locator('text=Draft sent')).toBeVisible({ timeout: 60_000 });

  // Inbound: poll the account's mailbox for the message with attachments.
  const deadline = Date.now() + 240_000;
  let delivered = null;
  while (Date.now() < deadline) {
    const list = await apiFetch(page, '/api/email/messages?folder=quarantine&limit=20');
    const msgs = list.body?.messages || [];
    delivered = msgs.find((m) => (m.subject || '').includes('attachment proof'));
    if (delivered) break;
    const inbox = await apiFetch(page, '/api/email/messages?folder=inbox&limit=20');
    delivered = (inbox.body?.messages || []).find((m) => (m.subject || '').includes('attachment proof'));
    if (delivered) break;
    await page.waitForTimeout(8000);
  }
  expect(delivered, 'inbound message with attachment subject not delivered').toBeTruthy();

  const detail = await apiFetch(page, `/api/email/messages/${delivered.id}`);
  const atts = detail.body?.attachments || [];
  expect(atts.length).toBeGreaterThanOrEqual(2);
  const names = atts.map((a) => a.filename);
  expect(names).toContain('client-upload.txt');
  expect(names).toContain('mail-attach-test.txt');
});
