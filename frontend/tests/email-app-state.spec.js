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
  const emailApp = page.locator('[data-email-app]').last();
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
  await expect(emailApp).toContainText('No messages');

  await expect.poll(() => requests.filter((entry) => entry === 'GET /api/email/aliases').length).toBe(1);
  await expect.poll(() => requests.filter((entry) => entry === 'GET /api/email/messages?folder=inbox').length).toBe(1);
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

  await emailApp.locator('[data-email-folder="sent"]').click();
  await expect(emailApp.locator('[data-email-folder="sent"]')).toHaveClass(/active/);
  await expect(emailApp).toContainText('Sent current message');

  await heldInboxRoute.fulfill({ json: { messages: [inboxMessage] } });
  await page.waitForTimeout(500);

  await expect(emailApp.locator('[data-email-folder="sent"]')).toHaveClass(/active/);
  await expect(emailApp).toContainText('Sent current message');
  await expect(emailApp).not.toContainText('Inbox stale message');
});

test('email inbox displays truthful counts and sandboxes html reading pane', async ({
  page,
  authenticator,
}) => {
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
    await route.fulfill({ status: 404, json: { error: 'unexpected email route' } });
  });

  const emailApp = await openEmail(page);
  // Verify truthful server total and unread counts in list header
  await expect(emailApp.locator('.list-header p')).toContainText('142 messages · 3 unread');

  // Verify reading pane sandboxed iframe
  const iframe = emailApp.locator('iframe.body-html-iframe');
  await expect(iframe).toBeVisible();
  await expect(iframe).toHaveAttribute('sandbox', 'allow-popups allow-popups-to-escape-sandbox');
  await expect(iframe).not.toHaveAttribute('autoputer', /allow-same-origin/);

  // Verify script tag was stripped from srcdoc
  const srcdoc = await iframe.getAttribute('srcdoc');
  expect(srcdoc).toContain('Message one <b>html</b> body');
  expect(srcdoc).not.toContain('<script>');
  expect(srcdoc).toContain("Content-Security-Policy");
});
