import { test, expect } from '@playwright/test';

const BASE_URL = process.env.PLAYWRIGHT_BASE_URL || 'http://localhost:5173';
function parseRgb(value) {
  const match = value.match(/rgba?\((\d+),\s*(\d+),\s*(\d+)/);
  if (match) return match.slice(1, 4).map(Number);
  const srgb = value.match(/color\(srgb\s+([\d.]+)\s+([\d.]+)\s+([\d.]+)/);
  if (srgb) return srgb.slice(1, 4).map((channel) => Math.round(Number(channel) * 255));
  return null;
}

async function openDeskApp(page, appId) {
  await page.locator('[data-desk-menu-button]').click();
  await expect(page.locator('[data-desk-sheet]')).toBeVisible();
  await page.locator(`[data-desk-sheet-app][data-desk-app-id="${appId}"]`).click();
}

async function deskAppIds(page) {
  return page.locator('[data-desk-sheet-app]').evaluateAll((buttons) =>
    buttons.map((button) => button.getAttribute('data-desk-app-id')).filter(Boolean)
  );
}

test('logged-out shell uses PromptSurface, DeskSheet, and fixture previews', async ({ page }) => {
  await page.goto(BASE_URL);
  await expect(page.locator('[data-prompt-surface]')).toBeVisible();
  await expect(page.locator('[data-bottom-bar]')).toHaveCount(0);
  await expect(page.locator('[data-desk-menu-button]')).toBeVisible();
  await expect(page.locator('[data-window-tray-item]')).toHaveCount(3);
  await expect(page.locator('[data-vtext-editor]')).toContainText('Node A redesign morning review');
  await expect(page.locator('[data-trace-app]')).toContainText('Preview fixture');
  const favicon = await page.locator('link[rel="icon"][data-tetramark-favicon]').getAttribute('href');
  expect(decodeURIComponent(favicon || '')).toContain('M 269.72 36.86');
  expect(decodeURIComponent(favicon || '')).toContain('M 476.43 455.41');

  const vtextToolbar = page.locator('[data-vtext-toolbar]');
  const vtextEditor = page.locator('[data-vtext-editor-area]');
  await vtextEditor.evaluate((node) => {
    node.innerHTML = `<h1>Scroll proof</h1>${Array.from({ length: 40 }, (_, i) => `<p>Paragraph ${i + 1}: the toolbar should recede while reading.</p>`).join('')}`;
    node.scrollTop = 0;
    node.dispatchEvent(new Event('scroll', { bubbles: true }));
  });
  await expect(vtextToolbar).not.toHaveClass(/toolbar-hidden/);
  const toolbarHeight = await vtextToolbar.evaluate((el) => el.getBoundingClientRect().height);
  await vtextEditor.evaluate((node) => {
    node.scrollTop = 320;
    node.dispatchEvent(new Event('scroll', { bubbles: true }));
  });
  await expect(vtextToolbar).toHaveClass(/toolbar-hidden/);
  await vtextEditor.evaluate((node) => {
    node.scrollTop = 300;
    node.dispatchEvent(new Event('scroll', { bubbles: true }));
  });
  await expect(vtextToolbar).toHaveClass(/toolbar-hidden/);
  await page.waitForTimeout(220);
  const hiddenToolbarHeight = await vtextToolbar.evaluate((el) => el.getBoundingClientRect().height);
  expect(hiddenToolbarHeight).toBeLessThan(toolbarHeight / 3);
  await page.waitForTimeout(120);
  await vtextEditor.evaluate((node) => {
    node.scrollTop = 160;
    node.dispatchEvent(new Event('scroll', { bubbles: true }));
  });
  await expect(vtextToolbar).not.toHaveClass(/toolbar-hidden/);

  const surfaceHeight = await page.locator('[data-prompt-surface]').evaluate((el) => el.getBoundingClientRect().height);
  expect(surfaceHeight).toBeLessThanOrEqual(78);

  await page.locator('[data-desk-menu-button]').click();
  await expect(page.locator('[data-desk-sheet].placement-bottom')).toBeVisible();
  await expect(page.locator('[data-desk-sheet-app][data-desk-app-id="email"]')).toBeVisible();
});

test('PromptSurface supports top placement without old geometry variables', async ({ page }) => {
  await page.goto(BASE_URL);
  await page.evaluate(() => {
    window.dispatchEvent(new CustomEvent('choir-theme-change', {
      detail: {
        theme: {
          schema_version: 2,
          id: 'futuristic-noir',
          name: 'Futuristic Noir',
          layout: { promptSurfacePlacement: 'top' },
        },
      },
    }));
  });

  await expect(page.locator('[data-prompt-surface][data-placement="top"]')).toBeVisible();
  await page.locator('[data-desk-menu-button]').click();
  await expect(page.locator('[data-desk-sheet].placement-top')).toBeVisible();

  const boxes = await page.evaluate(() => {
    const surface = document.querySelector('[data-prompt-surface]').getBoundingClientRect();
    const sheet = document.querySelector('[data-desk-sheet]').getBoundingClientRect();
    const root = getComputedStyle(document.documentElement);
    return {
      surfaceBottom: surface.bottom,
      sheetTop: sheet.top,
      promptTop: root.getPropertyValue('--choir-prompt-surface-top-offset').trim(),
      promptBottom: root.getPropertyValue('--choir-prompt-surface-bottom-offset').trim(),
      legacyBottom: root.getPropertyValue('--choir-prompt-surface-height').trim(),
    };
  });
  expect(boxes.sheetTop).toBeGreaterThanOrEqual(boxes.surfaceBottom - 1);
  expect(boxes.promptTop).toMatch(/px$/);
  expect(boxes.promptBottom).toBe('0px');
  expect(boxes.legacyBottom).toBe('');
});

test('desktop icons reflow inside the prompt-safe viewport', async ({ page }) => {
  await page.setViewportSize({ width: 420, height: 540 });
  await page.goto(BASE_URL);
  await expect(page.locator('[data-prompt-surface]')).toBeVisible();

  const layout = await page.evaluate(() => {
    const prompt = document.querySelector('[data-prompt-surface]').getBoundingClientRect();
    const icons = [...document.querySelectorAll('[data-desktop-icon]')].map((icon) => {
      const rect = icon.getBoundingClientRect();
      return {
        id: icon.getAttribute('data-desktop-icon-id'),
        left: rect.left,
        top: rect.top,
        right: rect.right,
        bottom: rect.bottom,
      };
    });
    return {
      viewportWidth: window.innerWidth,
      viewportHeight: window.innerHeight,
      promptTop: prompt.top,
      icons,
    };
  });

  expect(layout.icons.length).toBeGreaterThan(0);
  for (const icon of layout.icons) {
    expect(icon.left, `${icon.id} left`).toBeGreaterThanOrEqual(0);
    expect(icon.top, `${icon.id} top`).toBeGreaterThanOrEqual(0);
    expect(icon.right, `${icon.id} right`).toBeLessThanOrEqual(layout.viewportWidth);
    expect(icon.bottom, `${icon.id} bottom`).toBeLessThanOrEqual(layout.promptTop);
    expect(icon.bottom, `${icon.id} viewport bottom`).toBeLessThanOrEqual(layout.viewportHeight);
  }
});

test('logged-out Desk opens every app and keeps Settings themes available', async ({ page }) => {
  await page.goto(BASE_URL);
  await expect(page.locator('[data-prompt-surface]')).toBeVisible();

  await page.locator('[data-desk-menu-button]').click();
  await expect(page.locator('[data-desk-sheet]')).toBeVisible();
  const deskOverflow = await page.locator('[data-desk-sheet]').evaluate((el) => el.scrollHeight - el.clientHeight);
  expect(deskOverflow).toBeLessThanOrEqual(1);
  const appIds = await deskAppIds(page);
  expect(appIds.length).toBeGreaterThanOrEqual(15);
  expect(new Set(appIds).size).toBe(appIds.length);
  for (const appId of appIds) {
    await expect(page.locator(`[data-desk-sheet-app][data-desk-app-id="${appId}"]`), appId).toBeVisible();
  }
  await page.locator('[data-desk-sheet-close]').click();

  for (const appId of appIds) {
    await openDeskApp(page, appId);
  }

  const appHostIds = await page.locator('[data-app-host]').evaluateAll((hosts) =>
    hosts.map((host) => host.getAttribute('data-app-id')).filter(Boolean)
  );
  for (const appId of appIds) {
    expect(appHostIds, `${appId} app host`).toContain(appId);
  }
  await expect(page.locator('[data-terminal-preview]')).toBeVisible();
  await expect(page.locator('[data-settings-window]')).toHaveCount(1);

  await expect(page.locator('[data-settings-window] [data-theme-preset]')).toHaveCount(3);
  await expect(page.locator('[data-settings-window] [data-theme-preset="futuristic-noir"]')).toBeVisible();
  await expect(page.locator('[data-settings-window] [data-theme-preset="carbon-fiber-kintsugi"]')).toBeVisible();
  await expect(page.locator('[data-settings-window] [data-theme-preset="london-salmon"]')).toBeVisible();
  await expect(page.locator('[data-settings-window] [data-theme-editor]')).toBeHidden();

  const assertThemeOnShells = async (themeId, expectedVars) => {
    const themeName = {
      'futuristic-noir': 'Futuristic Noir',
      'carbon-fiber-kintsugi': 'Carbon Fiber Kintsugi',
      'london-salmon': 'London Salmon',
    }[themeId];
    await page.locator(`[data-settings-window] [data-theme-preset="${themeId}"]`).click();
    await expect(page.locator('html')).toHaveAttribute('data-theme-id', themeId);
    await expect(page.locator('[data-settings-theme-validation]')).toContainText(`${themeName}: valid config`);
    const sample = await page.evaluate(() => {
      const root = getComputedStyle(document.documentElement);
      return {
        vars: {
          bg: root.getPropertyValue('--choir-bg').trim(),
          accent: root.getPropertyValue('--choir-accent').trim(),
          panel: root.getPropertyValue('--choir-panel').trim(),
          blur: root.getPropertyValue('--choir-blur').trim(),
          uiFont: root.getPropertyValue('--choir-font-ui').trim(),
        },
        vtextFont: getComputedStyle(document.querySelector('[data-vtext-editor]')).fontFamily,
        settingsFont: getComputedStyle(document.querySelector('[data-settings-window]')).fontFamily,
        vtextToolbar: {
          backgroundColor: getComputedStyle(document.querySelector('[data-vtext-toolbar]')).backgroundColor,
          color: getComputedStyle(document.querySelector('[data-vtext-toolbar]')).color,
        },
        fileToolbar: {
          backgroundColor: getComputedStyle(document.querySelector('[data-files-app] .toolbar')).backgroundColor,
          color: getComputedStyle(document.querySelector('[data-files-app] .toolbar')).color,
        },
        vtextHeadingColor: getComputedStyle(document.querySelector('[data-vtext-editor-area] h1')).color,
        shells: [...document.querySelectorAll('[data-app-host]')].map((element) => {
          const style = element ? getComputedStyle(element) : null;
          return {
            appId: element.getAttribute('data-app-id') || '',
            exists: !!element,
            backgroundColor: style?.backgroundColor || '',
            color: style?.color || '',
          };
        }),
      };
    });
    expect(sample.vars).toMatchObject(expectedVars);
    for (const shell of sample.shells) {
      expect(shell.exists, `${themeId} ${shell.appId} exists`).toBe(true);
      const rgb = parseRgb(shell.backgroundColor);
      expect(rgb, `${themeId} ${shell.appId} background ${shell.backgroundColor}`).not.toBeNull();
      if (themeId === 'london-salmon') {
        expect(rgb[0], `${shell.appId} red channel`).toBeGreaterThanOrEqual(250);
        expect(rgb[1], `${shell.appId} green channel`).toBeGreaterThanOrEqual(235);
        expect(rgb[2], `${shell.appId} blue channel`).toBeGreaterThanOrEqual(230);
      } else {
        expect(Math.max(...rgb), `${themeId} ${shell.appId} should not retain light salmon panel`).toBeLessThan(245);
      }
    }
    expect(sample.vtextFont).toContain('Georgia');
    if (themeId === 'london-salmon') {
      expect(sample.vars.blur).toBe('0px');
      expect(sample.vars.uiFont).toContain('Georgia');
      expect(sample.settingsFont).toContain('Georgia');
      const toolbarBg = parseRgb(sample.vtextToolbar.backgroundColor);
      const toolbarColor = parseRgb(sample.vtextToolbar.color);
      const fileToolbarBg = parseRgb(sample.fileToolbar.backgroundColor);
      const fileToolbarColor = parseRgb(sample.fileToolbar.color);
      const headingColor = parseRgb(sample.vtextHeadingColor);
      expect(toolbarBg[0]).toBeGreaterThanOrEqual(248);
      expect(toolbarBg[1]).toBeGreaterThanOrEqual(228);
      expect(toolbarBg[2]).toBeGreaterThanOrEqual(222);
      expect(fileToolbarBg[0]).toBeGreaterThanOrEqual(248);
      expect(fileToolbarBg[1]).toBeGreaterThanOrEqual(235);
      expect(fileToolbarBg[2]).toBeGreaterThanOrEqual(230);
      expect(toolbarColor[0]).toBeLessThan(75);
      expect(toolbarColor[1]).toBeLessThan(35);
      expect(toolbarColor[2]).toBeLessThan(38);
      expect(fileToolbarColor[0]).toBeLessThan(75);
      expect(fileToolbarColor[1]).toBeLessThan(35);
      expect(fileToolbarColor[2]).toBeLessThan(38);
      expect(headingColor[0]).toBeLessThan(100);
      expect(headingColor[1]).toBeLessThan(45);
      expect(headingColor[2]).toBeLessThan(50);
    }
  };

  await assertThemeOnShells('futuristic-noir', { bg: '#050912', accent: '#6D8DFF', panel: '#0D1628' });
  await assertThemeOnShells('carbon-fiber-kintsugi', { bg: '#0B0C0D', accent: '#FFD86B', panel: '#151719', blur: '4px' });
  await assertThemeOnShells('london-salmon', { bg: '#FDF1EE', accent: '#9C5852', panel: '#FFFCFA', blur: '0px' });

  await page.locator('[data-desk-menu-button]').click();
  await expect(page.locator('[data-desk-sheet]')).toBeVisible();
  const salmonAffordance = await page.evaluate(() => {
    const read = (selector) => {
      const element = document.querySelector(selector);
      const style = element ? getComputedStyle(element) : null;
      return {
        fontFamily: style?.fontFamily || '',
        fontStyle: style?.fontStyle || '',
        fontWeight: style?.fontWeight || '',
        backgroundColor: style?.backgroundColor || '',
        boxShadow: style?.boxShadow || '',
      };
    };
    return {
      deskLabel: read('[data-desk-sheet-app] strong'),
      deskButton: read('[data-desk-sheet-app]'),
      desktopIconLabel: read('[data-desktop-icon-label]'),
      vtextButton: read('[data-vtext-toolbar] button'),
      settingsButton: read('[data-settings-window] button'),
    };
  });
  for (const [name, style] of Object.entries({
    deskLabel: salmonAffordance.deskLabel,
    desktopIconLabel: salmonAffordance.desktopIconLabel,
    vtextButton: salmonAffordance.vtextButton,
    settingsButton: salmonAffordance.settingsButton,
  })) {
    expect(style.fontFamily, name).toContain('Georgia');
    expect(style.fontStyle, name).toBe('italic');
    expect(Number.parseInt(style.fontWeight, 10), name).toBeLessThanOrEqual(500);
  }
  expect(salmonAffordance.deskButton.backgroundColor).toBe('rgba(0, 0, 0, 0)');
  expect(salmonAffordance.deskButton.boxShadow).toBe('none');
  expect(salmonAffordance.settingsButton.backgroundColor).toBe('rgba(0, 0, 0, 0)');
  expect(salmonAffordance.settingsButton.boxShadow).toBe('none');
  await page.locator('[data-desk-sheet-close]').click();
});

test('Trace renders swimlanes and mobile TetraMark switches open apps', async ({ page, browser }) => {
  await page.goto(BASE_URL);
  await openDeskApp(page, 'trace');
  const traceWindow = page.locator('[data-trace-window]').last();
  await expect(traceWindow.locator('[data-trace-swimlane-chart]')).toBeVisible();
  await expect(traceWindow.locator('[data-trace-swimlane]')).toHaveCount(4);
  await expect(traceWindow.locator('[data-trace-swimlane-chart] [data-trace-moment]')).toHaveCount(7);

  const mobile = await browser.newPage({ viewport: { width: 390, height: 844 } });
  await mobile.goto(BASE_URL);
  await expect(mobile.locator('[data-prompt-surface]')).toBeVisible();
  const promptBox = await mobile.locator('[data-prompt-surface]').boundingBox();
  expect(promptBox.height).toBeLessThanOrEqual(64);
  await mobile.locator('[data-desk-menu-button]').click();
  await expect(mobile.locator('[data-desk-sheet]')).toBeVisible();
  await expect(mobile.locator('[data-mobile-app-switcher]')).toBeVisible();
  await expect(mobile.locator('[data-mobile-switcher-open="true"] [data-prompt-input]')).toHaveCount(0);
  await expect(mobile.locator('[data-mobile-app-switcher] button')).not.toHaveCount(0);
  await mobile.close();
});

test('Desktop Overview is theme-native and action-oriented', async ({ page }) => {
  await page.goto(BASE_URL);
  await openDeskApp(page, 'settings');

  const expectations = {
    'futuristic-noir': { light: false },
    'carbon-fiber-kintsugi': { light: false },
    'london-salmon': { light: true },
  };

  for (const [themeId, expected] of Object.entries(expectations)) {
    await page.locator(`[data-settings-window] [data-theme-preset="${themeId}"]`).click();
    await expect(page.locator('html')).toHaveAttribute('data-theme-id', themeId);

    await page.locator('[data-desk-menu-button]').click();
    await expect(page.locator('[data-desk-sheet]')).toBeVisible();
    await page.locator('[data-desk-overview]').click();
    const overview = page.locator('[data-desktop-overview]');
    await expect(overview).toBeVisible();
    await expect(overview).toContainText('Switch or clean up');
    await expect(overview).not.toContainText('Restore pressure');
    await expect(overview).not.toContainText('honest card');
    await expect(page.locator('[data-overview-map-window]').first()).toBeEnabled();

    const sample = await overview.evaluate((node) => {
      const panel = node.querySelector('.overview-panel');
      const card = node.querySelector('[data-overview-card]');
      const action = node.querySelector('[data-overview-card-focus]');
      const panelStyle = getComputedStyle(panel);
      const cardStyle = getComputedStyle(card);
      const actionStyle = getComputedStyle(action);
      return {
        panelBg: panelStyle.backgroundColor,
        cardBg: cardStyle.backgroundColor,
        actionBg: actionStyle.backgroundColor,
        actionColor: actionStyle.color,
        actionFontStyle: actionStyle.fontStyle,
      };
    });
    const panelRgb = parseRgb(sample.panelBg);
    const cardRgb = parseRgb(sample.cardBg);
    expect(panelRgb, `${themeId} panel background`).not.toBeNull();
    expect(cardRgb, `${themeId} card background`).not.toBeNull();
    if (expected.light) {
      expect(panelRgb[0]).toBeGreaterThanOrEqual(248);
      expect(panelRgb[1]).toBeGreaterThanOrEqual(235);
      expect(cardRgb[0]).toBeGreaterThanOrEqual(245);
      expect(sample.actionFontStyle).toBe('italic');
    } else {
      expect(Math.max(...panelRgb)).toBeLessThan(245);
      expect(Math.max(...cardRgb)).toBeLessThan(245);
    }

    await page.locator('[data-overview-close]').click();
    await expect(overview).toHaveCount(0);
  }
});
