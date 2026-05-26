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

  // Screenshot 1: Auth page with cookies
  await page.goto('https://choir.news/');
  await page.waitForTimeout(3000);
  await page.screenshot({
    path: 'test-results/legacy-gateway-e2e-round5/06-auth-page-with-cookies.png',
    fullPage: true
  });
  console.log('Auth page screenshot saved');

  // Screenshot 2: Shell page
  await page.goto('https://choir.news/shell');
  await page.waitForTimeout(5000);
  await page.screenshot({
    path: 'test-results/legacy-gateway-e2e-round5/07-shell-page.png',
    fullPage: true
  });
  console.log('Shell page screenshot saved');

  // Screenshot 3: Submit prompt-bar intent and show result
  await page.goto('https://choir.news/');
  await page.waitForTimeout(2000);

  // Submit prompt-bar intent via console to show the flow
  const result = await page.evaluate(async () => {
    const res = await fetch('https://choir.news/api/prompt-bar', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ text: 'Explain quantum computing in one sentence' })
    });
    const data = await res.json();

    // Wait for completion
    let status;
    for (let i = 0; i < 10; i++) {
      const s = await fetch(`https://choir.news/api/prompt-bar/submissions/${data.submission_id}`);
      status = await s.json();
      if (status.decision || status.state === 'completed' || status.state === 'failed') break;
      await new Promise(r => setTimeout(r, 1000));
    }

    return { submission: data, status };
  });

  console.log('Final prompt result:', JSON.stringify(result, null, 2));

  // Save result to file
  const fs = await import('fs');
  fs.writeFileSync(
    'test-results/legacy-gateway-e2e-round5/prompt-result.json',
    JSON.stringify(result, null, 2)
  );

  await page.screenshot({
    path: 'test-results/legacy-gateway-e2e-round5/08-final-state.png',
    fullPage: true
  });
  console.log('Final state screenshot saved');

  await browser.close();
})();
