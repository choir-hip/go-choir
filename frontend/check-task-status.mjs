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
  await page.goto('https://draft.choir-ip.com/');

  const submissionId = process.argv[2] || 'ee914e04-5add-423f-8a81-baaddf3d6b17';

  // Check prompt submission status multiple times
  console.log('Checking prompt submission status over time...\n');
  for (let i = 0; i < 6; i++) {
    const statusResponse = await page.evaluate(async (url) => {
      try {
        const res = await fetch(url);
        return { status: res.status, text: await res.text() };
      } catch (e) {
        return { error: e.message };
      }
    }, `https://draft.choir-ip.com/api/prompt-bar/submissions/${submissionId}`);

    console.log(`Status check ${i + 1} (${(i + 1) * 5}s):`, JSON.stringify(statusResponse));

    if (i < 5) {
      await page.waitForTimeout(5000);
    }
  }

  // Try to check trace
  console.log('\n\nChecking trace...');
  const traceResponse = await page.evaluate(async (url) => {
    try {
      const res = await fetch(url);
      return { status: res.status, text: await res.text() };
    } catch (e) {
      return { error: e.message };
    }
  }, `https://draft.choir-ip.com/api/trace/trajectories/${submissionId}`);
  console.log('Trace response:', JSON.stringify(traceResponse));

  await browser.close();
})();
