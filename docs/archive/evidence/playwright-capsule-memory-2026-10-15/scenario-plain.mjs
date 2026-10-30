import { chromium } from 'playwright-core';
const scenario = process.argv[2];
const browser = await chromium.launch({ headless: true });
if (scenario === 'video-30s') {
  const ctx = await browser.newContext({ viewport: { width: 1280, height: 720 }, recordVideo: { dir: '/w/out/strace-video', size: { width: 1280, height: 720 } } });
  const page = await ctx.newPage();
  await page.goto('http://127.0.0.1:8080/', { waitUntil: 'networkidle' });
  await page.waitForTimeout(3000);
  await ctx.close();
} else {
  const page = await browser.newPage({ viewport: { width: 1280, height: 800 } });
  await page.goto('http://127.0.0.1:8080/', { waitUntil: 'networkidle' });
  await page.screenshot({ path: '/w/out/strace-shot.png' });
}
await browser.close();
