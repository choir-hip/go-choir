import { test, expect } from './helpers/fixtures.js';
import { registerPasskey } from './helpers/auth.js';

const BASE_URL = process.env.PLAYWRIGHT_BASE_URL || 'http://localhost:4173';

function uniqueEmail() {
  return `email-state-${Date.now()}-${Math.random().toString(36).slice(2, 8)}@example.com`;
}

async function registerAndLoadDesktop(page, email) {
  await page.goto(BASE_URL);
  await registerPasskey(page, email, BASE_URL);
  await page.reload();
  await page.locator('[data-desktop][data-authenticated="true"][data-desktop-ready="true"]').waitFor({
    state: 'visible',
    timeout: 120000,
  });
}

async function openEmail(page) {
  await page.locator('[data-desktop-icon-id="email"]').dblclick();
  const emailApp = page.locator('[data-mail-app]').last();
  await expect(emailApp).toBeVisible({ timeout: 10000 });
  return emailApp;
}

function messageSummary(id, subject, folder = 'inbox') {
  return {
    id,
    direction: folder === 'sent' ? 'outbound' : 'inbound',
    from_address: folder === 'sent' ? 'owner@example.com' : 'sender@example.com',
    subject,
    snippet: `${subject} snippet`,
    trust_status: folder === 'sent' ? 'trusted' : 'public',
    created_at: '2026-06-23T00:00:00Z',
    received_at: folder === 'sent' ? '' : '2026-06-23T00:00:00Z',
    sent_at: folder === 'sent' ? '2026-06-23T00:00:00Z' : '',
  };
}

function messageDetail(summary) {
  return {
    message: summary,
    text_body: `${summary.subject} body`,
    raw_headers: {},
    recipients: { to: [{ address: 'owner@example.com' }], cc: [], bcc: [] },
    attachments: [],
  };
}

test('email bootstrap performs one aliases request and one mailbox request', async ({
  page,
  authenticator,
}) => {
  const email = uniqueEmail();
  await registerAndLoadDesktop(page, email);

  const requests = [];
  await page.route('**/api/email/**', async (route) => {
    const url = new URL(route.request().url());
    requests.push(`${route.request().method()} ${url.pathname}${url.search}`);
    if (url.pathname === '/api/email/aliases') {
      await route.fulfill({ json: { aliases: [] } });
      return;
    }
    if (url.pathname === '/api/email/messages') {
      await route.fulfill({ json: { messages: [] } });
      return;
    }
    if (url.pathname === '/api/email/drafts') {
      await route.fulfill({ json: { drafts: [] } });
      return;
    }
    await route.fulfill({ status: 404, json: { error: 'unexpected email route' } });
  });

  const emailApp = await openEmail(page);
  await expect(emailApp).toContainText('Nothing in Inbox');

  await expect.poll(() => requests.filter((entry) => entry === 'GET /api/email/aliases').length).toBe(1);
  await expect.poll(() => requests.filter((entry) => entry === 'GET /api/email/messages?folder=inbox&limit=100').length).toBe(1);
});

test('Mail stages client and Files attachments, binds them to a draft, and downloads outbound bytes', async ({
  page,
  authenticator,
}) => {
  const email = uniqueEmail();
  await registerAndLoadDesktop(page, email);

  const attachmentRequests = [];
  let draftPayload = null;
  let draft = null;
  const staged = [];

  await page.route('**/api/files**', async (route) => {
    const url = new URL(route.request().url());
    if (url.pathname === '/api/files') {
      await route.fulfill({ json: [{ name: 'from-autoputer.txt', type: 'file', size: 16 }] });
      return;
    }
    if (url.pathname === '/api/files/from-autoputer.txt') {
      await route.fulfill({ body: 'from autoputer fs', contentType: 'text/plain' });
      return;
    }
    await route.fulfill({ status: 404, body: 'file not found' });
  });

  await page.route('**/api/email/**', async (route) => {
    const url = new URL(route.request().url());
    const method = route.request().method();
    if (url.pathname === '/api/email/aliases') {
      await route.fulfill({ json: { aliases: [{ address: 'owner@example.com' }] } });
      return;
    }
    if (url.pathname === '/api/email/messages') {
      await route.fulfill({ json: { messages: [], total: 0, unread: 0 } });
      return;
    }
    if (url.pathname === '/api/email/attachments' && method === 'POST') {
      const id = `attachment-${staged.length + 1}`;
      const attachment = {
        id,
        filename: route.request().headerValue('x-choir-filename'),
        content_type: route.request().headerValue('content-type'),
        size_bytes: route.request().postDataBuffer()?.length || 0,
        sha256: `hash-${id}`,
      };
      staged.push(attachment);
      attachmentRequests.push(`${method} ${url.pathname}`);
      await route.fulfill({ status: 201, json: attachment });
      return;
    }
    if (url.pathname.startsWith('/api/email/attachments/') && method === 'GET') {
      attachmentRequests.push(`${method} ${url.pathname}`);
      await route.fulfill({ body: 'outbound attachment', contentType: 'text/plain' });
      return;
    }
    if (url.pathname === '/api/email/drafts' && method === 'POST') {
      draftPayload = route.request().postDataJSON();
      draft = {
        id: 'draft-with-attachments',
        status: 'draft_pending_owner_approval',
        from_address: 'owner@example.com',
        to_addresses: draftPayload.to_addresses,
        subject: draftPayload.subject,
        text_body: draftPayload.text_body,
        attachments: staged.map((attachment) => ({ ...attachment, status: 'bound' })),
      };
      await route.fulfill({ json: draft });
      return;
    }
    if (url.pathname === '/api/email/drafts' && method === 'GET') {
      await route.fulfill({ json: { drafts: draft ? [draft] : [] } });
      return;
    }
    if (url.pathname === '/api/email/drafts/draft-with-attachments') {
      await route.fulfill({ json: draft });
      return;
    }
    await route.fulfill({ status: 404, json: { error: 'unexpected email route' } });
  });

  const mailApp = await openEmail(page);
  await mailApp.locator('[data-mail-compose]').first().click();
  await mailApp.locator('[data-mail-attachment-input]').setInputFiles({
    name: 'from-client.txt',
    mimeType: 'text/plain',
    buffer: Buffer.from('from client upload'),
  });
  await expect(mailApp.locator('[data-mail-staged-attachment="attachment-1"]')).toContainText('from-client.txt');

  await mailApp.locator('[data-mail-attachment-from-files]').click();
  await mailApp.locator('[data-mail-file-picker-entry="from-autoputer.txt"]').click();
  await expect(mailApp.locator('[data-mail-staged-attachment="attachment-2"]')).toContainText('from-autoputer.txt');

  await mailApp.locator('[data-mail-compose-to]').fill('recipient@example.com');
  await mailApp.locator('[data-mail-compose-subject]').fill('Attachment contract');
  await mailApp.locator('[data-mail-compose-body]').fill('Review both files.');
  await mailApp.locator('[data-mail-compose-save]').click();

  await expect.poll(() => draftPayload?.attachment_ids).toEqual(['attachment-1', 'attachment-2']);
  await expect(mailApp.locator('[data-mail-attachment-download="attachment-1"]')).toBeVisible();
  await mailApp.locator('[data-mail-attachment-download="attachment-1"]').click();
  await expect.poll(() => attachmentRequests).toContain('GET /api/email/attachments/attachment-1');
});

test('stale slower mailbox response cannot overwrite newer folder state', async ({
  page,
  authenticator,
}) => {
  const email = uniqueEmail();
  await registerAndLoadDesktop(page, email);

  const inboxMessage = messageSummary('inbox-1', 'Inbox stale message', 'inbox');
  const sentMessage = messageSummary('sent-1', 'Sent current message', 'sent');
  let heldInboxRoute = null;

  await page.route('**/api/email/**', async (route) => {
    const url = new URL(route.request().url());
    if (url.pathname === '/api/email/aliases') {
      await route.fulfill({ json: { aliases: [{ address: 'owner@example.com' }] } });
      return;
    }
    if (url.pathname === '/api/email/messages' && url.searchParams.get('folder') === 'inbox') {
      heldInboxRoute = route;
      return;
    }
    if (url.pathname === '/api/email/messages' && url.searchParams.get('folder') === 'sent') {
      await route.fulfill({ json: { messages: [sentMessage] } });
      return;
    }
    if (url.pathname === '/api/email/messages/sent-1') {
      await route.fulfill({ json: messageDetail(sentMessage) });
      return;
    }
    if (url.pathname === '/api/email/messages/inbox-1') {
      await route.fulfill({ json: messageDetail(inboxMessage) });
      return;
    }
    await route.fulfill({ status: 404, json: { error: 'unexpected email route' } });
  });

  const emailApp = await openEmail(page);
  await expect.poll(() => Boolean(heldInboxRoute)).toBe(true);

  await emailApp.locator('[data-mail-folder="sent"]').click();
  await expect(emailApp.locator('[data-mail-folder="sent"]')).toHaveClass(/mail-selected/);
  await expect(emailApp).toContainText('Sent current message');

  await heldInboxRoute.fulfill({ json: { messages: [inboxMessage] } });
  await page.waitForTimeout(500);

  await expect(emailApp.locator('[data-mail-folder="sent"]')).toHaveClass(/mail-selected/);
  await expect(emailApp).toContainText('Sent current message');
  await expect(emailApp).not.toContainText('Inbox stale message');
});

test('email inbox displays truthful counts and sandboxes html reading pane', async ({
  page,
  authenticator,
}) => {
  const pageErrors = [];
  page.on('pageerror', (err) => pageErrors.push(err));

  const email = uniqueEmail();
  await registerAndLoadDesktop(page, email);

  const msg1 = messageSummary('inbox-1', 'Message One', 'inbox');
  const msg2 = messageSummary('inbox-2', 'Message Two', 'inbox');

  await page.route('**/api/email/**', async (route) => {
    const url = new URL(route.request().url());
    if (url.pathname === '/api/email/aliases') {
      await route.fulfill({ json: { aliases: [{ address: 'owner@example.com' }] } });
      return;
    }
    if (url.pathname === '/api/email/messages' && url.searchParams.get('folder') === 'inbox') {
      await route.fulfill({
        json: {
          messages: [msg1, msg2],
          next_cursor: 'cursor-token-page2',
          total: 142,
          unread: 3,
        },
      });
      return;
    }
    if (url.pathname === '/api/email/messages/inbox-1') {
      await route.fulfill({
        json: {
          message: msg1,
          text_body: 'Message one plain text',
          html_body: '<p>Message one <b>html</b> body <script>alert(1)</script></p>',
          raw_headers: {},
          recipients: { to: [{ address: 'owner@example.com' }], cc: [], bcc: [] },
          attachments: [],
        },
      });
      return;
    }
    if (url.pathname === '/api/email/messages/inbox-1/read') {
      await route.fulfill({ json: { status: 'read' } });
      return;
    }
    await route.fulfill({ status: 404, json: { error: 'unexpected email route' } });
  });

  const emailApp = await openEmail(page);
  // Verify truthful server total and unread counts in list header
  await expect(emailApp.locator('.mail-listhead-count')).toContainText('142 messages · 3 unread');

  // Verify reading pane sandboxed iframe
  const iframe = emailApp.locator('iframe.mail-body-iframe');
  await expect(iframe).toBeVisible();
  await expect(iframe).toHaveAttribute('sandbox', 'allow-popups allow-popups-to-escape-sandbox');
  await expect(iframe).not.toHaveAttribute('autoputer', /allow-same-origin/);

  // Verify script tag was stripped from srcdoc
  const srcdoc = await iframe.getAttribute('srcdoc');
  expect(srcdoc).toContain('Message one <b>html</b> body');
  expect(srcdoc).not.toContain('<script>');
  expect(srcdoc).toContain("Content-Security-Policy");
  expect(pageErrors).toEqual([]);
});

test('background refresh prepends newer messages preserving descending sort order', async ({
  page,
  authenticator,
}) => {
  const pageErrors = [];
  page.on('pageerror', (err) => pageErrors.push(err));

  const email = uniqueEmail();
  await registerAndLoadDesktop(page, email);

  const msgOld = messageSummary('inbox-1', 'Old Message', 'inbox');
  const msgNew = messageSummary('inbox-2', 'New Message Arrived', 'inbox');

  let callCount = 0;
  await page.route('**/api/email/**', async (route) => {
    const url = new URL(route.request().url());
    if (url.pathname === '/api/email/aliases') {
      await route.fulfill({ json: { aliases: [{ address: 'owner@example.com' }] } });
      return;
    }
    if (url.pathname === '/api/email/messages' && url.searchParams.get('folder') === 'inbox') {
      callCount += 1;
      if (callCount === 1) {
        await route.fulfill({ json: { messages: [msgOld], total: 1, unread: 0 } });
      } else {
        await route.fulfill({ json: { messages: [msgNew, msgOld], total: 2, unread: 1 } });
      }
      return;
    }
    await route.fulfill({ status: 404, json: { error: 'unexpected email route' } });
  });

  const emailApp = await openEmail(page);
  await expect(emailApp.locator('.mail-row-subject')).toHaveText(['Old Message']);

  // Simulate window visibility event triggering background refresh
  await page.evaluate(() => {
    document.dispatchEvent(new Event('visibilitychange'));
  });

  // Verify newer message is prepended to the top, preserving descending order
  await expect(emailApp.locator('.mail-row-subject')).toHaveText(['New Message Arrived', 'Old Message']);
  expect(pageErrors).toEqual([]);
});

test('opening unread email marks it read and decrements unread count', async ({
  page,
  authenticator,
}) => {
  const pageErrors = [];
  page.on('pageerror', (err) => pageErrors.push(err));

  const email = uniqueEmail();
  await registerAndLoadDesktop(page, email);

  const unreadMsg = { ...messageSummary('inbox-unread', 'Urgent update', 'inbox'), read_at: null, direction: 'inbound' };
  let readEndpointCalled = false;
  let unreadEndpointCalled = false;

  await page.route('**/api/email/**', async (route) => {
    const url = new URL(route.request().url());
    if (url.pathname === '/api/email/aliases') {
      await route.fulfill({ json: { aliases: [{ address: 'owner@example.com' }] } });
      return;
    }
    if (url.pathname === '/api/email/messages' && url.searchParams.get('folder') === 'inbox') {
      await route.fulfill({
        json: {
          messages: [unreadMsg],
          total: 1,
          unread: 1,
        },
      });
      return;
    }
    if (url.pathname === '/api/email/messages/inbox-unread/read' && route.request().method() === 'POST') {
      readEndpointCalled = true;
      await route.fulfill({ json: { status: 'read' } });
      return;
    }
    if (url.pathname === '/api/email/messages/inbox-unread/unread' && route.request().method() === 'POST') {
      unreadEndpointCalled = true;
      await route.fulfill({ json: { status: 'unread' } });
      return;
    }
    if (url.pathname === '/api/email/messages/inbox-unread') {
      await route.fulfill({
        json: {
          message: unreadMsg,
          text_body: 'This is an unread message body.',
          raw_headers: {},
          recipients: { to: [{ address: 'owner@example.com' }], cc: [], bcc: [] },
          attachments: [],
        },
      });
      return;
    }
    await route.fulfill({ status: 404, json: { error: 'unexpected email route' } });
  });

  const emailApp = await openEmail(page);

  // Detail loads on select, so read endpoint is called
  await expect.poll(() => readEndpointCalled).toBe(true);

  // The unread dot should disappear from the message row
  const row = emailApp.locator('[data-mail-row="inbox-unread"]');
  await expect(row).not.toHaveClass(/mail-unread/);
  await expect(row.locator('.mail-unread-dot')).toHaveCount(0);

  // Header should update to 0 unread (so "1 messages" without "unread")
  await expect(emailApp.locator('.mail-listhead-count')).toHaveText('1 messages');

  // Reading pane footer toggle button should say "Mark unread"
  const toggleBtn = emailApp.locator('[data-mail-read-toggle]');
  await expect(toggleBtn).toHaveText('Mark unread');

  // Click "Mark unread"
  await toggleBtn.click();
  await expect.poll(() => unreadEndpointCalled).toBe(true);

  // Unread dot re-appears, row has class unread, and count increments
  await expect(row).toHaveClass(/mail-unread/);
  await expect(row.locator('.mail-unread-dot')).toBeVisible();
  await expect(emailApp.locator('.mail-listhead-count')).toContainText('1 unread');
  await expect(toggleBtn).toHaveText('Mark read');

  expect(pageErrors).toEqual([]);
});
