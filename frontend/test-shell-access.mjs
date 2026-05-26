import { chromium } from '@playwright/test';

(async () => {
  const browser = await chromium.launch({
    headless: true,
    ignoreHTTPSErrors: true
  });

  // Load the saved storage state from previous auth
  const context = await browser.newContext({
    storageState: 'test-results/legacy-gateway-e2e-round5/storage-state.json'
  });

  const page = await context.newPage();

  // Navigate to shell directly
  console.log('Navigating to /shell...');
  await page.goto('https://choir.news/shell');
  await page.waitForTimeout(10000);

  const url = page.url();
  console.log('Current URL:', url);
  console.log('Page title:', await page.title());

  // Take screenshot
  await page.screenshot({
    path: 'test-results/legacy-gateway-e2e-round5/04-shell-attempt.png',
    fullPage: true
  });
  console.log('Shell screenshot saved');

  // Check if shell loaded by looking for key elements
  const html = await page.content();
  console.log('Page HTML length:', html.length);
  console.log('Contains "desktop":', html.toLowerCase().includes('desktop'));
  console.log('Contains "launcher":', html.toLowerCase().includes('launcher'));
  console.log('Contains "window":', html.toLowerCase().includes('window'));
  console.log('Contains "error":', html.toLowerCase().includes('error'));

  // Also try the root path with auth cookies
  console.log('\nNavigating to / with auth cookies...');
  await page.goto('https://choir.news/');
  await page.waitForTimeout(5000);

  console.log('Root URL:', page.url());
  console.log('Root title:', await page.title());

  await page.screenshot({
    path: 'test-results/legacy-gateway-e2e-round5/05-root-with-auth.png',
    fullPage: true
  });

  // Get all cookies to verify auth state
  const cookies = await context.cookies();
  console.log('\nAll cookies:', JSON.stringify(cookies.map(c => c.name)));

  await browser.close();
})();
