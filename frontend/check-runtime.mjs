import { chromium } from '@playwright/test';

(async () => {
  const browser = await chromium.launch({
    headless: true,
    ignoreHTTPSErrors: true
  });

  const context = await browser.newContext({
    storageState: 'test-results/legacy-gateway-e2e-round5/storage-state.json'
  });

  const page = await context.newPage();

  // Check all relevant APIs
  const endpoints = [
    '/api/health',
    '/api/sandbox/health',
    '/api/shell/bootstrap',
    '/api/vm/status',
    '/api/trace/trajectories',
  ];

  for (const endpoint of endpoints) {
    console.log(`\nChecking ${endpoint}...`);
    const response = await page.evaluate(async (url) => {
      try {
        const res = await fetch(url);
        return { status: res.status, text: await res.text().catch(() => 'BINARY') };
      } catch (e) {
        return { error: e.message };
      }
    }, endpoint);
    console.log('Response:', JSON.stringify(response));
  }

  // Try to submit prompt-bar intent
  console.log('\n\nTrying to submit prompt-bar intent...');
  const submissionResponse = await page.evaluate(async () => {
    try {
      const res = await fetch('/api/prompt-bar', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ text: 'Say hello world' })
      });
      return { status: res.status, text: await res.text() };
    } catch (e) {
      return { error: e.message };
    }
  });
  console.log('Prompt submission response:', JSON.stringify(submissionResponse));

  await browser.close();
})();
