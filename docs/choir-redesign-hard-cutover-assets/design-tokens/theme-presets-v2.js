export const THEME_SCHEMA_VERSION = 2;

const base = {
  schema_version: THEME_SCHEMA_VERSION,
  radii: {
    controlSm: '10px',
    control: '16px',
    panel: '22px',
    sheet: '30px',
    pill: '999px',
  },
  motion: {
    fast: '120ms ease',
    sheet: '260ms cubic-bezier(0.2, 0.8, 0.2, 1)',
  },
  layout: {
    promptSurfacePlacement: 'bottom',
    promptSurfaceMinHeight: '64px',
    deskSheetHeight: '56dvh',
  },
  fonts: {
    ui: "Inter, ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif",
    display: "Inter, ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif",
    mono: "ui-monospace, SFMono-Regular, Menlo, Consolas, monospace",
  },
};

export const FUTURISTIC_NOIR = {
  ...base,
  id: 'futuristic-noir',
  name: 'Futuristic Noir',
  colors: {
    bg: '#050912', bg2: '#081224', panel: '#0D1628', panelStrong: '#09101F', panelSoft: 'rgba(18, 31, 55, 0.68)',
    fg: '#F7FAFF', muted: '#9AA9C0', subtle: '#65748D', accent: '#6D8DFF', accent2: '#45D7FF',
    success: '#20D686', warning: '#FFB339', danger: '#FF5B6B',
    border: 'rgba(133, 159, 211, 0.22)', borderStrong: 'rgba(130, 156, 255, 0.44)',
    inputBg: 'rgba(10, 17, 31, 0.78)', selected: 'rgba(91, 123, 255, 0.22)', onAccent: '#FFFFFF',
    promptSurfaceBg: 'linear-gradient(180deg, rgba(17, 28, 52, 0.88), rgba(8, 14, 27, 0.92))',
    sheetBg: 'linear-gradient(180deg, rgba(10, 17, 32, 0.96), rgba(7, 11, 21, 0.98))',
    controlBg: 'rgba(19, 32, 59, 0.84)', tetramarkColor: '#93B2FF',
    chart1: '#6D8DFF', chart2: '#45D7FF', chart3: '#20D686', chart4: '#FFB339', chart5: '#FF5B6B',
  },
  effects: { blur: '22px', texture: 'none', shadowSoft: '0 18px 54px rgba(0,0,0,.38)', shadowFloating: '0 26px 90px rgba(0,0,0,.46)', shadowGlow: '0 0 42px rgba(89, 125, 255, .22)', controlShadow: 'inset 0 1px 0 rgba(255,255,255,.05)' },
};

export const CARBON_FIBER_KINTSUGI = {
  ...base,
  id: 'carbon-fiber-kintsugi',
  name: 'Carbon Fiber Kintsugi',
  colors: {
    bg: '#0B0C0D', bg2: '#131416', panel: '#151719', panelStrong: '#0F1012', panelSoft: 'rgba(28, 30, 33, 0.74)',
    fg: '#F2EFE7', muted: '#B2AA98', subtle: '#766F62', accent: '#D8AD45', accent2: '#F4D477',
    success: '#55C27A', warning: '#D8AD45', danger: '#E36B5A',
    border: 'rgba(219, 180, 83, 0.18)', borderStrong: 'rgba(244, 212, 119, 0.44)',
    inputBg: 'rgba(12, 13, 14, 0.78)', selected: 'rgba(216, 173, 69, 0.18)', onAccent: '#14100A',
    promptSurfaceBg: 'linear-gradient(180deg, rgba(27,29,31,.92), rgba(12,13,15,.96))',
    sheetBg: 'linear-gradient(180deg, rgba(22,24,26,.96), rgba(10,11,12,.98))',
    controlBg: 'rgba(23, 25, 27, 0.88)', tetramarkColor: '#E4BE5B',
    chart1: '#D8AD45', chart2: '#F4D477', chart3: '#55C27A', chart4: '#A78BFA', chart5: '#E36B5A',
  },
  effects: { blur: '18px', texture: 'carbon-fiber', shadowSoft: '0 18px 54px rgba(0,0,0,.42)', shadowFloating: '0 28px 90px rgba(0,0,0,.55)', shadowGlow: '0 0 34px rgba(216,173,69,.24)', controlShadow: 'inset 0 1px 0 rgba(255,255,255,.035)' },
};

export const LONDON_SALMON = {
  ...base,
  id: 'london-salmon',
  name: 'London Salmon',
  colors: {
    bg: '#E9D0C2', bg2: '#F4E1D6', panel: '#F5E4D8', panelStrong: '#EBCFBE', panelSoft: 'rgba(255, 247, 238, 0.72)',
    fg: '#241A16', muted: '#72584D', subtle: '#9B7969', accent: '#A44F38', accent2: '#214D48',
    success: '#1D6E45', warning: '#B46B16', danger: '#9F2F2D',
    border: 'rgba(99, 58, 43, 0.20)', borderStrong: 'rgba(99, 58, 43, 0.38)',
    inputBg: 'rgba(255, 248, 240, 0.74)', selected: 'rgba(164, 79, 56, 0.16)', onAccent: '#FFF8F0',
    promptSurfaceBg: 'linear-gradient(180deg, rgba(248,231,218,.94), rgba(232,205,189,.98))',
    sheetBg: 'linear-gradient(180deg, rgba(250,236,224,.98), rgba(235,211,196,.98))',
    controlBg: 'rgba(255, 247, 238, 0.84)', tetramarkColor: '#5A302B',
    chart1: '#A44F38', chart2: '#214D48', chart3: '#1D6E45', chart4: '#B46B16', chart5: '#9F2F2D',
  },
  fonts: {
    ui: "Aptos, ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif",
    display: "Georgia, 'Times New Roman', ui-serif, serif",
    mono: "ui-monospace, SFMono-Regular, Menlo, Consolas, monospace",
  },
  effects: { blur: '8px', texture: 'paper', shadowSoft: '0 12px 32px rgba(91, 58, 40, .16)', shadowFloating: '0 20px 60px rgba(91, 58, 40, .22)', shadowGlow: 'none', controlShadow: 'inset 0 1px 0 rgba(255,255,255,.38)' },
};

export const DEFAULT_THEME = FUTURISTIC_NOIR;
export const THEME_PRESETS = [FUTURISTIC_NOIR, CARBON_FIBER_KINTSUGI, LONDON_SALMON];
const PRESET_IDS = new Set(THEME_PRESETS.map((theme) => theme.id));

export function normalizeThemeConfig(theme = DEFAULT_THEME) {
  if (!theme || theme.schema_version !== THEME_SCHEMA_VERSION || !PRESET_IDS.has(theme.id)) return DEFAULT_THEME;
  const preset = THEME_PRESETS.find((item) => item.id === theme.id) || DEFAULT_THEME;
  return {
    ...preset,
    layout: { ...preset.layout, ...(theme.layout || {}) },
  };
}

export function validateThemeConfig(theme) {
  if (!theme || typeof theme !== 'object') return { ok: false, errors: ['theme must be an object'] };
  if (theme.schema_version !== THEME_SCHEMA_VERSION) return { ok: false, errors: [`schema_version must be ${THEME_SCHEMA_VERSION}`] };
  if (!PRESET_IDS.has(theme.id)) return { ok: false, errors: [`theme id must be one of ${[...PRESET_IDS].join(', ')}`] };
  const placement = theme.layout?.promptSurfacePlacement;
  if (placement && !['top', 'bottom'].includes(placement)) return { ok: false, errors: ['layout.promptSurfacePlacement must be top or bottom'] };
  return { ok: true, errors: [] };
}

export function themeCSSVariables(theme = DEFAULT_THEME) {
  theme = normalizeThemeConfig(theme);
  const c = theme.colors;
  const r = theme.radii;
  const e = theme.effects;
  return {
    '--choir-bg': c.bg,
    '--choir-bg-2': c.bg2,
    '--choir-panel': c.panel,
    '--choir-panel-strong': c.panelStrong,
    '--choir-panel-soft': c.panelSoft,
    '--choir-fg': c.fg,
    '--choir-muted': c.muted,
    '--choir-subtle': c.subtle,
    '--choir-accent': c.accent,
    '--choir-accent-2': c.accent2,
    '--choir-success': c.success,
    '--choir-warning': c.warning,
    '--choir-danger': c.danger,
    '--choir-border': c.border,
    '--choir-border-strong': c.borderStrong,
    '--choir-input-bg': c.inputBg,
    '--choir-selected': c.selected,
    '--choir-on-accent': c.onAccent,
    '--choir-prompt-surface-bg': c.promptSurfaceBg,
    '--choir-sheet-bg': c.sheetBg,
    '--choir-control-bg': c.controlBg,
    '--choir-tetramark-color': c.tetramarkColor,
    '--choir-chart-1': c.chart1,
    '--choir-chart-2': c.chart2,
    '--choir-chart-3': c.chart3,
    '--choir-chart-4': c.chart4,
    '--choir-chart-5': c.chart5,
    '--choir-radius-control-sm': r.controlSm,
    '--choir-radius-control': r.control,
    '--choir-radius-panel': r.panel,
    '--choir-radius-sheet': r.sheet,
    '--choir-radius-pill': r.pill,
    '--choir-blur': e.blur,
    '--choir-shadow-soft': e.shadowSoft,
    '--choir-shadow-floating': e.shadowFloating,
    '--choir-shadow-glow': e.shadowGlow,
    '--choir-control-shadow': e.controlShadow,
    '--choir-prompt-surface-size': theme.layout.promptSurfaceMinHeight,
    '--choir-prompt-surface-top-offset': theme.layout.promptSurfacePlacement === 'top' ? theme.layout.promptSurfaceMinHeight : '0px',
    '--choir-prompt-surface-bottom-offset': theme.layout.promptSurfacePlacement === 'bottom' ? theme.layout.promptSurfaceMinHeight : '0px',
    '--choir-desk-sheet-height': theme.layout.deskSheetHeight,
    '--choir-font-ui': theme.fonts.ui,
    '--choir-font-display': theme.fonts.display,
    '--choir-font-mono': theme.fonts.mono,
  };
}

export function themeStyleString(theme = DEFAULT_THEME) {
  return Object.entries(themeCSSVariables(theme)).map(([key, value]) => `${key}: ${value}`).join('; ');
}

export function applyThemeToElement(element, theme = DEFAULT_THEME) {
  const normalized = normalizeThemeConfig(theme);
  const variables = themeCSSVariables(normalized);
  if (element) {
    for (const [key, value] of Object.entries(variables)) element.style.setProperty(key, value);
    element.dataset.themeId = normalized.id;
    element.dataset.promptSurfacePlacement = normalized.layout.promptSurfacePlacement || 'bottom';
  }
  return variables;
}
