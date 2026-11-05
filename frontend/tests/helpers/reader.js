// Reader hooks for the specs that used to drive the PDF and EPUB apps (slice C7:
// those app ids alias the Reader package, so the windows are Reader windows).
import { expect } from '@playwright/test';

export function readerViewer(page, format) {
  return page.locator(`[data-reader-app][data-reader-format="${format}"]`).last();
}

/** Opens the search sheet (toolbar button, or the overflow menu in a compact window) and searches. */
export async function readerSearch(viewer, term) {
  const toggle = viewer.locator('[data-reader-search-toggle]').first();
  if (!(await toggle.isVisible())) {
    await viewer.locator('[data-reader-overflow-toggle]').click();
  }
  await viewer.locator('[data-reader-search-toggle]:visible').first().click();
  const input = viewer.locator('[data-reader-search]');
  await expect(input).toBeVisible();
  await input.fill(term);
  return viewer.locator('[data-reader-search-count]');
}

/** A PDF page drawn on the canvas. */
export async function expectPdfRendered(viewer) {
  await expect(viewer.locator('[data-reader-page-canvas]')).toHaveAttribute('data-reader-rendered', 'true', { timeout: 15_000 });
}
