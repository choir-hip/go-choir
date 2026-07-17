export const THEME_SCHEMA_VERSION = 2;

type PromptSurfacePlacement = 'top' | 'bottom';

export type ChoirTheme = {
  schema_version: number;
  id: string;
  name: string;
  colors: Record<string, string>;
  radii: Record<string, string>;
  motion: Record<string, string>;
  layout: {
    promptSurfacePlacement: PromptSurfacePlacement;
    promptSurfaceMinHeight: string;
    deskSheetHeight: string;
  };
  fonts: Record<string, string>;
  effects: Record<string, string>;
};

const base: Omit<ChoirTheme, 'id' | 'name' | 'colors' | 'effects'> = {
  schema_version: THEME_SCHEMA_VERSION,
  radii: {
    controlSm: '14px',
    control: '20px',
    panel: '26px',
    sheet: '32px',
    pill: '30px',
  },
  motion: {
    fast: '120ms ease',
    sheet: '260ms cubic-bezier(0.2, 0.8, 0.2, 1)',
  },
  layout: {
    promptSurfacePlacement: 'bottom',
    promptSurfaceMinHeight: '72px',
    deskSheetHeight: '56dvh',
  },
  fonts: {
    ui: "Inter, ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif",
    display: "Inter, ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif",
    mono: "ui-monospace, SFMono-Regular, Menlo, Consolas, monospace",
  },
};

export const FUTURISTIC_NOIR: ChoirTheme = {
  ...base,
  id: 'futuristic-noir',
  name: 'Futuristic Noir',
  colors: {
    bg: '#050912', bg2: '#081224', panel: '#0D1628', panelStrong: '#09101F', panelSoft: 'rgba(18, 31, 55, 0.68)',
    fg: '#F7FAFF', muted: '#9AA9C0', subtle: '#65748D', accent: '#6D8DFF', accent2: '#45D7FF',
    success: '#20D686', warning: '#FFB339', danger: '#FF5B6B', onDanger: '#FFFFFF',
    border: 'rgba(133, 159, 211, 0.22)', borderStrong: 'rgba(130, 156, 255, 0.44)',
    inputBg: 'rgba(10, 17, 31, 0.78)', selected: 'rgba(91, 123, 255, 0.22)', onAccent: '#FFFFFF',
    promptSurfaceBg: 'linear-gradient(180deg, rgba(17, 28, 52, 0.88), rgba(8, 14, 27, 0.92))',
    sheetBg: 'linear-gradient(180deg, rgba(10, 17, 32, 0.96), rgba(7, 11, 21, 0.98))',
    controlBg: 'rgba(19, 32, 59, 0.84)', tetramarkColor: '#93B2FF',
    chart1: '#6D8DFF', chart2: '#45D7FF', chart3: '#20D686', chart4: '#FFB339', chart5: '#FF5B6B',
  },
  effects: {
    blur: '22px',
    texture: 'none',
    shadowSoft: '0 18px 54px rgba(0,0,0,.38)',
    shadowFloating: '0 26px 90px rgba(0,0,0,.46)',
    shadowGlow: '0 0 42px rgba(89, 125, 255, .22)',
    controlShadow: '0 12px 32px rgba(0,0,0,.24), inset 0 10px 24px rgba(255,255,255,.035)',
  },
};

export const CARBON_FIBER_KINTSUGI: ChoirTheme = {
  ...base,
  id: 'carbon-fiber-kintsugi',
  name: 'Carbon Fiber Kintsugi',
  colors: {
    bg: '#0B0C0D', bg2: '#131416', panel: '#151719', panelStrong: '#0F1012', panelSoft: 'rgba(28, 30, 33, 0.74)',
    fg: '#F2EFE7', muted: '#B2AA98', subtle: '#766F62', accent: '#FFD86B', accent2: '#FFF1BC',
    success: '#55C27A', warning: '#FFD86B', danger: '#E36B5A', onDanger: '#14100A',
    border: 'rgba(255, 216, 107, 0.18)', borderStrong: 'rgba(255, 241, 188, 0.48)',
    inputBg: 'rgba(12, 13, 14, 0.78)', selected: 'rgba(255, 216, 107, 0.18)', onAccent: '#14100A',
    promptSurfaceBg: 'linear-gradient(180deg, rgba(27,29,31,.92), rgba(12,13,15,.96))',
    sheetBg: 'linear-gradient(180deg, rgba(22,24,26,.96), rgba(10,11,12,.98))',
    controlBg: 'rgba(23, 25, 27, 0.88)', tetramarkColor: '#FFE18A',
    chart1: '#FFD86B', chart2: '#FFF1BC', chart3: '#55C27A', chart4: '#A78BFA', chart5: '#E36B5A',
  },
  effects: {
    blur: '4px',
    texture: 'carbon-fiber',
    shadowSoft: '0 18px 54px rgba(0,0,0,.42)',
    shadowFloating: '0 28px 90px rgba(0,0,0,.55)',
    shadowGlow: '0 0 30px rgba(255,216,107,.18)',
    controlShadow: '0 12px 32px rgba(0,0,0,.28), inset 0 10px 24px rgba(255,255,255,.03)',
  },
};

export const LONDON_SALMON: ChoirTheme = {
  ...base,
  id: 'london-salmon',
  name: 'London Salmon',
  colors: {
    bg: '#FDF1EE', bg2: '#FFF7F4', panel: '#FFFCFA', panelStrong: '#FAECE8', panelSoft: 'rgba(255, 253, 251, 0.9)',
    fg: '#3A1517', muted: '#755B56', subtle: '#AD9088', accent: '#9C5852', accent2: '#244F4A',
    success: '#1D6E45', warning: '#B46122', danger: '#9F2F2D', onDanger: '#FFF8F4',
    border: 'rgba(156, 88, 82, 0.1)', borderStrong: 'rgba(91, 28, 31, 0.2)',
    inputBg: 'rgba(255, 254, 252, 0.88)', selected: 'rgba(156, 88, 82, 0.1)', onAccent: '#FFF8F4',
    promptSurfaceBg: 'linear-gradient(180deg, rgba(255,254,252,.98), rgba(253,243,240,.98))',
    sheetBg: 'linear-gradient(180deg, rgba(255,254,252,.99), rgba(254,246,243,.98))',
    controlBg: 'rgba(255, 253, 250, 0.95)', tetramarkColor: '#682A28',
    chart1: '#9C5852', chart2: '#244F4A', chart3: '#1D6E45', chart4: '#B46122', chart5: '#9F2F2D',
  },
  fonts: {
    ui: "Georgia, 'Times New Roman', ui-serif, serif",
    display: "Georgia, 'Times New Roman', ui-serif, serif",
    mono: "ui-monospace, SFMono-Regular, Menlo, Consolas, monospace",
  },
  effects: {
    blur: '0px',
    texture: 'paper',
    shadowSoft: '0 10px 24px rgba(91, 58, 40, .13)',
    shadowFloating: '0 18px 42px rgba(91, 58, 40, .2)',
    shadowGlow: 'none',
    controlShadow: '0 8px 18px rgba(91, 58, 40, .1), inset 0 1px 0 rgba(255,255,255,.36)',
  },
};

export const DEFAULT_THEME = FUTURISTIC_NOIR;
export const THEME_PRESETS = [FUTURISTIC_NOIR, CARBON_FIBER_KINTSUGI, LONDON_SALMON];
const PRESET_IDS = new Set(THEME_PRESETS.map((theme) => theme.id));
const THEME_GROUPS = ['colors', 'radii', 'motion', 'fonts', 'effects'] as const;

function isTheme(value: unknown): value is Partial<ChoirTheme> {
  return !!value && typeof value === 'object' && !Array.isArray(value);
}

function isSafeCSSValue(value: unknown): value is string {
  return typeof value === 'string' &&
    value.length > 0 &&
    value.length <= 512 &&
    !/[;{}\u0000-\u001f]/.test(value) &&
    !/(?:url|expression)\s*\(|@import/i.test(value);
}

function mergeStringGroup(baseGroup: Record<string, string>, candidate: unknown): Record<string, string> {
  if (!isTheme(candidate)) return { ...baseGroup };
  return Object.fromEntries(Object.entries(baseGroup).map(([key, fallback]) => [
    key,
    isSafeCSSValue(candidate[key]) ? candidate[key] : fallback,
  ]));
}

export function normalizeThemeConfig(theme: unknown = DEFAULT_THEME): ChoirTheme {
  if (!isTheme(theme) || theme.schema_version !== THEME_SCHEMA_VERSION || !theme.id || !PRESET_IDS.has(theme.id)) {
    return DEFAULT_THEME;
  }
  const preset = THEME_PRESETS.find((item) => item.id === theme.id) || DEFAULT_THEME;
  const placement = theme.layout?.promptSurfacePlacement === 'top' ? 'top' : preset.layout.promptSurfacePlacement;
  return {
    ...preset,
    name: typeof theme.name === 'string' && theme.name.trim() ? theme.name.trim().slice(0, 80) : preset.name,
    colors: mergeStringGroup(preset.colors, theme.colors),
    radii: mergeStringGroup(preset.radii, theme.radii),
    motion: mergeStringGroup(preset.motion, theme.motion),
    fonts: mergeStringGroup(preset.fonts, theme.fonts),
    effects: mergeStringGroup(preset.effects, theme.effects),
    layout: {
      ...preset.layout,
      promptSurfacePlacement: placement,
      promptSurfaceMinHeight: isSafeCSSValue(theme.layout?.promptSurfaceMinHeight)
        ? theme.layout.promptSurfaceMinHeight
        : preset.layout.promptSurfaceMinHeight,
      deskSheetHeight: isSafeCSSValue(theme.layout?.deskSheetHeight)
        ? theme.layout.deskSheetHeight
        : preset.layout.deskSheetHeight,
    },
  };
}

export function validateThemeConfig(theme: unknown): { ok: boolean; errors: string[] } {
  if (!isTheme(theme)) return { ok: false, errors: ['theme must be an object'] };
  const errors: string[] = [];
  if (theme.schema_version !== THEME_SCHEMA_VERSION) errors.push(`schema_version must be ${THEME_SCHEMA_VERSION}`);
  if (!theme.id || !PRESET_IDS.has(theme.id)) errors.push(`theme id must be one of ${[...PRESET_IDS].join(', ')}`);
  if (theme.name !== undefined && (typeof theme.name !== 'string' || !theme.name.trim() || theme.name.length > 80)) {
    errors.push('name must be a non-empty string of at most 80 characters');
  }

  const preset = THEME_PRESETS.find((item) => item.id === theme.id) || DEFAULT_THEME;
  for (const groupName of THEME_GROUPS) {
    const group = theme[groupName];
    if (group === undefined) continue;
    if (!isTheme(group)) {
      errors.push(`${groupName} must be an object`);
      continue;
    }
    const allowedKeys = new Set(Object.keys(preset[groupName]));
    for (const [key, value] of Object.entries(group)) {
      if (!allowedKeys.has(key)) errors.push(`${groupName}.${key} is not supported`);
      else if (!isSafeCSSValue(value)) errors.push(`${groupName}.${key} must be a safe CSS value`);
    }
  }

  if (theme.layout !== undefined && !isTheme(theme.layout)) errors.push('layout must be an object');
  const placement = theme.layout?.promptSurfacePlacement;
  if (placement && !['top', 'bottom'].includes(placement)) errors.push('layout.promptSurfacePlacement must be top or bottom');
  for (const key of ['promptSurfaceMinHeight', 'deskSheetHeight'] as const) {
    if (theme.layout?.[key] !== undefined && !isSafeCSSValue(theme.layout[key])) {
      errors.push(`layout.${key} must be a safe CSS value`);
    }
  }
  if (isTheme(theme.layout)) {
    const allowedLayoutKeys = new Set(['promptSurfacePlacement', 'promptSurfaceMinHeight', 'deskSheetHeight']);
    for (const key of Object.keys(theme.layout)) {
      if (!allowedLayoutKeys.has(key)) errors.push(`layout.${key} is not supported`);
    }
  }
  return { ok: errors.length === 0, errors };
}

export function themeCSSVariables(theme: unknown = DEFAULT_THEME): Record<string, string> {
  const normalized = normalizeThemeConfig(theme);
  const c = normalized.colors;
  const r = normalized.radii;
  const e = normalized.effects;
  const panelOpaque = opaqueColor(c.panel, DEFAULT_THEME.colors.panel);
  const panelStrongOpaque = opaqueColor(c.panelStrong, DEFAULT_THEME.colors.panelStrong);
  return {
    '--choir-bg': c.bg,
    '--choir-bg-2': c.bg2,
    '--choir-panel': c.panel,
    '--choir-panel-opaque': panelOpaque,
    '--choir-panel-strong': c.panelStrong,
    '--choir-panel-strong-opaque': panelStrongOpaque,
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
    '--choir-surface-app': panelOpaque,
    '--choir-surface-pane': panelStrongOpaque,
    '--choir-surface-card': c.panelSoft,
    '--choir-surface-control': c.controlBg,
    '--choir-surface-input': c.inputBg,
    '--choir-surface-inset': panelStrongOpaque,
    '--choir-surface-media': '#000000',
    '--choir-surface-document': panelOpaque,
    '--choir-text-primary': c.fg,
    '--choir-text-muted': c.muted,
    '--choir-text-subtle': c.subtle,
    '--choir-text-accent': c.accent2,
    '--choir-text-on-accent': c.onAccent,
    '--choir-state-selected': c.selected,
    '--choir-state-hover': colorMix(c.accent, 12),
    '--choir-state-focus': colorMix(c.accent, 32),
    '--choir-state-active-glow': colorMix(c.accent, 24),
    '--choir-status-success': c.success,
    '--choir-status-success-soft': colorMix(c.success, 22),
    '--choir-status-warning': c.warning,
    '--choir-status-warning-soft': colorMix(c.warning, 22),
    '--choir-status-danger': c.danger,
    '--choir-status-danger-soft': colorMix(c.danger, 24),
    '--choir-text-on-danger': c.onDanger,
    '--choir-shadow-color': 'rgb(0, 0, 0)',
    '--choir-light-overlay': colorMix(c.fg, 8),
    '--choir-grid-line': colorMix(c.borderStrong || c.border, 28),
    '--choir-body-background': themeBodyBackground(normalized),
    '--choir-body-overlay': themeBodyOverlay(normalized),
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
    '--choir-motion-fast': normalized.motion.fast,
    '--choir-motion-sheet': normalized.motion.sheet,
    '--choir-prompt-surface-size': normalized.layout.promptSurfaceMinHeight,
    '--choir-prompt-surface-top-offset': normalized.layout.promptSurfacePlacement === 'top' ? normalized.layout.promptSurfaceMinHeight : '0px',
    '--choir-prompt-surface-bottom-offset': normalized.layout.promptSurfacePlacement === 'bottom' ? normalized.layout.promptSurfaceMinHeight : '0px',
    '--choir-desk-sheet-height': normalized.layout.deskSheetHeight,
    '--choir-font-ui': normalized.fonts.ui,
    '--choir-font-display': normalized.fonts.display,
    '--choir-font-mono': normalized.fonts.mono,
  };
}

function colorMix(color: string, percent: number): string {
  return `color-mix(in srgb, ${color} ${percent}%, transparent)`;
}

function themeBodyBackground(theme: ChoirTheme): string {
  if (theme.id === 'carbon-fiber-kintsugi') {
    return [
      `linear-gradient(115deg, ${colorMix(theme.colors.accent2, 9)}, transparent 16%, transparent 78%, ${colorMix(theme.colors.accent2, 7)})`,
      `repeating-linear-gradient(45deg, ${colorMix(theme.colors.fg, 4)} 0 3px, transparent 3px 9px)`,
      `repeating-linear-gradient(-45deg, ${colorMix(theme.colors.fg, 3)} 0 3px, transparent 3px 9px)`,
      `repeating-linear-gradient(90deg, ${colorMix('#000000', 34)} 0 1px, transparent 1px 7px)`,
      theme.colors.bg,
    ].join(', ');
  }
  if (theme.id === 'london-salmon') {
    return [
      `linear-gradient(${colorMix(theme.colors.accent, 3)} 1px, transparent 1px)`,
      theme.colors.bg,
    ].join(', ');
  }
  return [
    `radial-gradient(120% 90% at 78% -10%, ${colorMix(theme.colors.accent, 13)}, transparent 52%)`,
    `radial-gradient(90% 80% at 8% 108%, ${colorMix(theme.colors.accent2, 9)}, transparent 46%)`,
    `linear-gradient(180deg, ${colorMix(theme.colors.bg2, 68)}, transparent 40%)`,
    theme.colors.bg,
  ].join(', ');
}

function themeBodyOverlay(theme: ChoirTheme): string {
  if (theme.id === 'carbon-fiber-kintsugi') {
    return `linear-gradient(115deg, ${colorMix(theme.colors.accent2, 8)}, transparent 18%, transparent 76%, ${colorMix(theme.colors.accent2, 6)})`;
  }
  if (theme.id === 'london-salmon') {
    return `linear-gradient(90deg, ${colorMix(theme.colors.accent, 8)} 1px, transparent 1px)`;
  }
  return 'none';
}

function opaqueColor(value: string | undefined, fallback: string): string {
  const color = String(value || '').trim();
  const rgba = color.match(/^rgba?\(\s*([0-9.]+)\s*,\s*([0-9.]+)\s*,\s*([0-9.]+)(?:\s*,\s*[0-9.]+\s*)?\)$/i);
  if (rgba) {
    return `rgb(${Math.round(Number(rgba[1]))}, ${Math.round(Number(rgba[2]))}, ${Math.round(Number(rgba[3]))})`;
  }
  const rgbSlash = color.match(/^rgb\(\s*([0-9.]+)\s+([0-9.]+)\s+([0-9.]+)(?:\s*\/\s*[0-9.]+%?\s*)?\)$/i);
  if (rgbSlash) {
    return `rgb(${Math.round(Number(rgbSlash[1]))}, ${Math.round(Number(rgbSlash[2]))}, ${Math.round(Number(rgbSlash[3]))})`;
  }
  return color || fallback;
}

export function themeStyleString(theme: unknown = DEFAULT_THEME): string {
  return Object.entries(themeCSSVariables(theme)).map(([key, value]) => `${key}: ${value}`).join('; ');
}

export function applyThemeToElement(element: HTMLElement | null | undefined, theme: unknown = DEFAULT_THEME): Record<string, string> {
  const normalized = normalizeThemeConfig(theme);
  const variables = themeCSSVariables(normalized);
  if (element) {
    for (const [key, value] of Object.entries(variables)) element.style.setProperty(key, value);
    element.dataset.themeId = normalized.id;
    element.dataset.promptSurfacePlacement = normalized.layout.promptSurfacePlacement;
  }
  return variables;
}
