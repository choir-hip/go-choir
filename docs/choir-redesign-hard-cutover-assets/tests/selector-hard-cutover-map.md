# Selector hard-cutover map

Do not preserve misleading bottom-only selectors. Update tests and code together.

| Old selector / name | New selector / name |
| --- | --- |
| `BottomBar.svelte` | `PromptSurface.svelte` |
| `bottomBarEl` | `promptSurfaceEl` |
| `bottomBarHeight` | `promptSurfaceSize` |
| `[data-bottom-bar]` | `[data-prompt-surface]` |
| `.bottom-bar` | `.prompt-surface` |
| `--choir-bottom-bar-height` | `--choir-prompt-surface-size` plus top/bottom offsets |
| `[data-show-desktop-btn]` | `[data-desk-menu-button]` |
| `[data-start-button]` | `[data-desk-menu-button]` |
| `[data-desk-button]` | `[data-desk-menu-button]` |
| `[data-start-menu]` | `[data-desk-sheet]` |
| `[data-desktop-menu]` | `[data-desk-sheet]` |
| `[data-start-app]` | `[data-desk-sheet-app]` |
| `[data-start-app-id]` | `[data-desk-app-id]` |
| `[data-window-indicator]` | `[data-window-tray-item]` |
| `[data-minimized-indicator]` | `[data-window-tray-item].minimized` or `[data-window-tray-item][data-window-mode="minimized"]` |
| `[data-bottom-user]` | `[data-prompt-surface-user]` |
| `[data-bottom-logout]` | `[data-prompt-surface-logout]` |
| `[data-connection-status]` | `[data-online-indicator]` |

Add top placement tests by applying a schema-v2 theme with `layout.promptSurfacePlacement = 'top'`, then asserting:

```js
await expect(page.locator('[data-prompt-surface][data-placement="top"]')).toBeVisible();
await page.locator('[data-desk-menu-button]').click();
await expect(page.locator('[data-desk-sheet].placement-top')).toBeVisible();
const boxes = await page.evaluate(() => {
  const surface = document.querySelector('[data-prompt-surface]').getBoundingClientRect();
  const sheet = document.querySelector('[data-desk-sheet]').getBoundingClientRect();
  return { surfaceBottom: surface.bottom, sheetTop: sheet.top };
});
expect(boxes.sheetTop).toBeGreaterThanOrEqual(boxes.surfaceBottom - 1);
```
