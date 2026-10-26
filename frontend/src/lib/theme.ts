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
    // Composite values — duration and timing in one string, for the simple
    // `transition: box-shadow var(--choir-motion-fast)` case.
    fast: '120ms ease',
    sheet: '260ms cubic-bezier(0.2, 0.8, 0.2, 1)',
    // Separated roles. A composite value cannot be safely embedded in a
    // shorthand that also names a duration — `opacity 0.8s var(--motion-x)`
    // expands to two time values and silently voids the whole declaration,
    // which is an extremely quiet way to lose every animation on a surface.
    // Anything that needs a per-property duration composes these instead.
    durationFast: '120ms',
    durationSheet: '260ms',
    durationEntrance: '1000ms',
    durationExit: '700ms',
    durationWindowOpen: '340ms',
    durationWindowClose: '200ms',
    ease: 'cubic-bezier(0.16, 1, 0.3, 1)',
    easeIn: 'cubic-bezier(0.7, 0, 0.84, 0)',
    easeSpring: 'cubic-bezier(0.34, 1.56, 0.64, 1)',
    easeDrift: 'cubic-bezier(0.45, 0.05, 0.55, 0.95)',
  },
  layout: {
    promptSurfacePlacement: 'bottom',
    // The prompt bar's real height. It publishes its measured height into
    // --choir-prompt-surface-size / -bottom-offset on mount; this is the
    // fallback before that first measurement, and reserving more than the bar
    // is what left a dead strip between a window and the bar.
    promptSurfaceMinHeight: '56px',
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
    // Landing intro field. Depth roles for the choir graph: far / mid / near
    // node alphas plus the desk-orb hues. Kept in the token system so the
    // canvas never hard-codes a colour (theme-contract contract).
    fieldEdge: 'rgba(109,141,255,0.16)',
    fieldNodeFar: 'rgba(133,159,211,0.5)',
    fieldNodeMid: 'rgba(109,141,255,0.66)',
    fieldNodeNear: 'rgba(69,215,255,0.9)',
    fieldHalo: 'rgba(109,141,255,0.05)',
    deskTexture: '#6D8DFF',
    deskManagement: '#45D7FF',
    deskEngineering: '#20D686',
    deskResearch: '#FFB339',
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
    fieldEdge: 'rgba(255,216,107,0.14)',
    fieldNodeFar: 'rgba(182,170,138,0.46)',
    fieldNodeMid: 'rgba(255,216,107,0.6)',
    fieldNodeNear: 'rgba(255,241,188,0.86)',
    fieldHalo: 'rgba(255,216,107,0.045)',
    deskTexture: '#FFD86B',
    deskManagement: '#FFF1BC',
    deskEngineering: '#55C27A',
    deskResearch: '#A78BFA',
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
    fieldEdge: 'rgba(156,88,82,0.16)',
    fieldNodeFar: 'rgba(173,144,136,0.5)',
    fieldNodeMid: 'rgba(156,88,82,0.6)',
    fieldNodeNear: 'rgba(36,79,74,0.82)',
    fieldHalo: 'rgba(156,88,82,0.05)',
    deskTexture: '#9C5852',
    deskManagement: '#244F4A',
    deskEngineering: '#1D6E45',
    deskResearch: '#B46122',
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

export const FRUTIGER_AERO: ChoirTheme = {
  ...base,
  id: 'frutiger-aero',
  name: 'Frutiger Aero',
  colors: {
    bg: '#BCEBFA', bg2: '#EAF8FD', panel: '#F4FBFE', panelStrong: '#E1F3FA', panelSoft: 'rgba(245, 252, 254, 0.94)',
    fg: '#123744', muted: '#3C6270', subtle: '#5B7C88', accent: '#087A9B', accent2: '#0F6E8A',
    success: '#0F6B45', warning: '#7A4A00', danger: '#9E2436', onDanger: '#FFFFFF',
    border: 'rgba(8, 122, 155, 0.2)', borderStrong: 'rgba(8, 96, 126, 0.44)',
    inputBg: 'rgba(255, 255, 255, 0.96)', selected: 'rgba(8, 122, 155, 0.16)', onAccent: '#FFFFFF',
    promptSurfaceBg: 'linear-gradient(180deg, rgba(255, 255, 255, 0.88), rgba(224, 246, 252, 0.92))',
    sheetBg: 'linear-gradient(180deg, rgba(255, 255, 255, 0.94), rgba(228, 247, 252, 0.96))',
    controlBg: 'rgba(255, 255, 255, 0.97)', tetramarkColor: '#075F7A',
    fieldEdge: 'rgba(8, 122, 155, 0.2)',
    fieldNodeFar: 'rgba(60, 98, 112, 0.46)',
    fieldNodeMid: 'rgba(8, 122, 155, 0.6)',
    fieldNodeNear: 'rgba(5, 84, 108, 0.86)',
    fieldHalo: 'rgba(8, 122, 155, 0.06)',
    deskTexture: '#087A9B',
    deskManagement: '#0F6E8A',
    deskEngineering: '#0F6B45',
    deskResearch: '#7A4A00',
    chart1: '#087A9B', chart2: '#0F6E8A', chart3: '#0F6B45', chart4: '#7A4A00', chart5: '#9E2436',
  },
  fonts: {
    ui: "Inter, ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif",
    display: "Inter, ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif",
    mono: "ui-monospace, SFMono-Regular, Menlo, Consolas, monospace",
  },
  effects: {
    blur: '14px',
    texture: 'aero-glass',
    shadowSoft: '0 12px 30px rgba(7, 63, 84, .16)',
    shadowFloating: '0 20px 54px rgba(7, 63, 84, .2)',
    shadowGlow: '0 0 30px rgba(8, 122, 155, .16)',
    controlShadow: '0 8px 18px rgba(7, 63, 84, .12), inset 0 1px 0 rgba(255, 255, 255, .9)',
  },
};

export const VECTOR_HEART: ChoirTheme = {
  ...base,
  id: 'vector-heart',
  name: 'Vector Heart',
  colors: {
    bg: '#F5F0FF', bg2: '#FFF5FB', panel: '#FFFBFF', panelStrong: '#F3E8FC', panelSoft: 'rgba(255, 251, 255, 0.94)',
    fg: '#26113E', muted: '#5D3F7C', subtle: '#7C6298', accent: '#B30079', accent2: '#8A0F62',
    success: '#0B6B45', warning: '#7A4600', danger: '#A3123A', onDanger: '#FFFFFF',
    border: 'rgba(179, 0, 121, 0.2)', borderStrong: 'rgba(120, 26, 120, 0.44)',
    inputBg: 'rgba(255, 255, 255, 0.97)', selected: 'rgba(179, 0, 121, 0.14)', onAccent: '#FFFFFF',
    promptSurfaceBg: 'linear-gradient(180deg, rgba(255, 255, 255, 0.9), rgba(247, 236, 255, 0.94))',
    sheetBg: 'linear-gradient(180deg, rgba(255, 255, 255, 0.94), rgba(245, 235, 255, 0.96))',
    controlBg: 'rgba(255, 255, 255, 0.97)', tetramarkColor: '#7C0A55',
    fieldEdge: 'rgba(179, 0, 121, 0.2)',
    fieldNodeFar: 'rgba(93, 63, 124, 0.46)',
    fieldNodeMid: 'rgba(179, 0, 121, 0.58)',
    fieldNodeNear: 'rgba(60, 16, 92, 0.86)',
    fieldHalo: 'rgba(179, 0, 121, 0.06)',
    deskTexture: '#B30079',
    deskManagement: '#8A0F62',
    deskEngineering: '#0B6B45',
    deskResearch: '#7A4600',
    chart1: '#B30079', chart2: '#8A0F62', chart3: '#0B6B45', chart4: '#7A4600', chart5: '#A3123A',
  },
  fonts: {
    ui: "Inter, ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif",
    display: "'Avenir Next', Inter, ui-sans-serif, system-ui, sans-serif",
    mono: "ui-monospace, SFMono-Regular, Menlo, Consolas, monospace",
  },
  effects: {
    blur: '12px',
    texture: 'vector-hearts',
    shadowSoft: '0 12px 28px rgba(57, 16, 84, .16)',
    shadowFloating: '0 20px 52px rgba(57, 16, 84, .2)',
    shadowGlow: '0 0 28px rgba(179, 0, 121, .18)',
    controlShadow: '0 8px 18px rgba(57, 16, 84, .12), inset 0 1px 0 rgba(255, 255, 255, .9)',
  },
};

export const LIQUID_METAL: ChoirTheme = {
  ...base,
  id: 'liquid-metal',
  name: 'Liquid Metal',
  colors: {
    bg: '#11161D', bg2: '#1A222B', panel: '#1C242E', panelStrong: '#161D25', panelSoft: 'rgba(32, 42, 53, 0.94)',
    fg: '#F5F8FA', muted: '#B3BFCB', subtle: '#8B98A6', accent: '#65C5D8', accent2: '#9FE3EE',
    success: '#4FD3A0', warning: '#F0C464', danger: '#F47A88', onDanger: '#14181D',
    border: 'rgba(101, 197, 216, 0.22)', borderStrong: 'rgba(159, 227, 238, 0.46)',
    inputBg: 'rgba(20, 26, 33, 0.97)', selected: 'rgba(101, 197, 216, 0.2)', onAccent: '#0B1116',
    promptSurfaceBg: 'linear-gradient(180deg, rgba(34, 44, 56, 0.9), rgba(16, 21, 28, 0.94))',
    sheetBg: 'linear-gradient(180deg, rgba(30, 39, 50, 0.95), rgba(14, 19, 25, 0.97))',
    controlBg: 'rgba(30, 40, 51, 0.97)', tetramarkColor: '#A8ECF6',
    fieldEdge: 'rgba(101, 197, 216, 0.22)',
    fieldNodeFar: 'rgba(139, 152, 166, 0.46)',
    fieldNodeMid: 'rgba(101, 197, 216, 0.62)',
    fieldNodeNear: 'rgba(216, 242, 248, 0.9)',
    fieldHalo: 'rgba(101, 197, 216, 0.06)',
    deskTexture: '#65C5D8',
    deskManagement: '#9FE3EE',
    deskEngineering: '#4FD3A0',
    deskResearch: '#F0C464',
    chart1: '#65C5D8', chart2: '#9FE3EE', chart3: '#4FD3A0', chart4: '#F0C464', chart5: '#F47A88',
  },
  fonts: {
    ui: "Inter, ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif",
    display: "Inter, ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif",
    mono: "ui-monospace, SFMono-Regular, Menlo, Consolas, monospace",
  },
  effects: {
    blur: '16px',
    texture: 'liquid-chrome',
    shadowSoft: '0 16px 38px rgba(0, 0, 0, .46)',
    shadowFloating: '0 24px 70px rgba(0, 0, 0, .54)',
    shadowGlow: '0 0 32px rgba(101, 197, 216, .2)',
    controlShadow: '0 10px 24px rgba(0, 0, 0, .32), inset 0 1px 0 rgba(255, 255, 255, .08)',
  },
};

export const AERO_CHROME_HEART: ChoirTheme = {
  ...base,
  id: 'aero-chrome-heart',
  name: 'Aero Chrome Heart',
  colors: {
    bg: '#C4ECF9', bg2: '#FBEFFA', panel: '#FFF7FE', panelStrong: '#EDE1F5', panelSoft: 'rgba(255, 250, 254, 0.94)',
    fg: '#20143A', muted: '#55396F', subtle: '#76608A', accent: '#A60072', accent2: '#7A0A57',
    success: '#0C6644', warning: '#744400', danger: '#A3142F', onDanger: '#FFFFFF',
    border: 'rgba(166, 0, 114, 0.2)', borderStrong: 'rgba(100, 30, 110, 0.44)',
    inputBg: 'rgba(255, 255, 255, 0.97)', selected: 'rgba(166, 0, 114, 0.14)', onAccent: '#FFFFFF',
    promptSurfaceBg: 'linear-gradient(180deg, rgba(255, 255, 255, 0.9), rgba(250, 238, 250, 0.94))',
    sheetBg: 'linear-gradient(180deg, rgba(255, 255, 255, 0.94), rgba(246, 236, 250, 0.96))',
    controlBg: 'rgba(255, 255, 255, 0.97)', tetramarkColor: '#7A0A57',
    fieldEdge: 'rgba(166, 0, 114, 0.2)',
    fieldNodeFar: 'rgba(85, 57, 111, 0.46)',
    fieldNodeMid: 'rgba(166, 0, 114, 0.58)',
    fieldNodeNear: 'rgba(32, 20, 58, 0.88)',
    fieldHalo: 'rgba(166, 0, 114, 0.06)',
    deskTexture: '#A60072',
    deskManagement: '#55396F',
    deskEngineering: '#0C6644',
    deskResearch: '#744400',
    chart1: '#A60072', chart2: '#55396F', chart3: '#0C6644', chart4: '#744400', chart5: '#A3142F',
  },
  fonts: {
    ui: "Inter, ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif",
    display: "'Avenir Next', Inter, ui-sans-serif, system-ui, sans-serif",
    mono: "ui-monospace, SFMono-Regular, Menlo, Consolas, monospace",
  },
  effects: {
    blur: '16px',
    texture: 'aero-chrome',
    shadowSoft: '0 14px 34px rgba(32, 20, 58, .18)',
    shadowFloating: '0 22px 60px rgba(32, 20, 58, .22)',
    shadowGlow: '0 0 32px rgba(166, 0, 114, .18)',
    controlShadow: '0 8px 20px rgba(32, 20, 58, .14), inset 0 1px 0 rgba(255, 255, 255, .9)',
  },
};

export const DEFAULT_THEME = AERO_CHROME_HEART;
export const THEME_PRESETS = [FUTURISTIC_NOIR, CARBON_FIBER_KINTSUGI, LONDON_SALMON, FRUTIGER_AERO, VECTOR_HEART, LIQUID_METAL, AERO_CHROME_HEART];
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
  const paperOpaque = opaqueColor(c.bg2, DEFAULT_THEME.colors.bg2);
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
    // Reading paper and the sheet a document is drawn on. The document surface
    // is the theme's own page; the paper is what a PDF page is drawn against,
    // a shade lighter so a rendered page reads as a sheet laid on the desk.
    '--choir-surface-paper': paperOpaque,
    // Sepia is the Reader's warm tone: a paper, its ink and its muted ink are
    // tokenised here so reader.css never carries a colour literal.
    '--choir-surface-document-sepia': SEPIA_PAPER,
    '--choir-text-sepia': SEPIA_INK,
    '--choir-text-sepia-muted': SEPIA_MUTED,
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
    '--choir-field-edge': c.fieldEdge,
    '--choir-field-node-far': c.fieldNodeFar,
    '--choir-field-node-mid': c.fieldNodeMid,
    '--choir-field-node-near': c.fieldNodeNear,
    '--choir-field-halo': c.fieldHalo,
    '--choir-desk-texture': c.deskTexture,
    '--choir-desk-management': c.deskManagement,
    '--choir-desk-engineering': c.deskEngineering,
    '--choir-desk-research': c.deskResearch,
    '--choir-motion-fast': normalized.motion.fast,
    '--choir-motion-sheet': normalized.motion.sheet,
    '--choir-duration-fast': normalized.motion.durationFast,
    '--choir-duration-sheet': normalized.motion.durationSheet,
    '--choir-duration-entrance': normalized.motion.durationEntrance,
    '--choir-duration-exit': normalized.motion.durationExit,
    '--choir-duration-window-open': normalized.motion.durationWindowOpen,
    '--choir-duration-window-close': normalized.motion.durationWindowClose,
    '--choir-ease': normalized.motion.ease,
    '--choir-ease-in': normalized.motion.easeIn,
    '--choir-ease-spring': normalized.motion.easeSpring,
    '--choir-ease-drift': normalized.motion.easeDrift,
    '--choir-prompt-surface-size': normalized.layout.promptSurfaceMinHeight,
    '--choir-prompt-surface-top-offset': normalized.layout.promptSurfacePlacement === 'top' ? normalized.layout.promptSurfaceMinHeight : '0px',
    '--choir-prompt-surface-bottom-offset': normalized.layout.promptSurfacePlacement === 'bottom' ? normalized.layout.promptSurfaceMinHeight : '0px',
    '--choir-desk-sheet-height': normalized.layout.deskSheetHeight,
    '--choir-font-ui': normalized.fonts.ui,
    '--choir-font-display': normalized.fonts.display,
    '--choir-font-mono': normalized.fonts.mono,
  };
}

// The Reader's sepia page. One warm paper with its ink and its muted ink,
// defined once so the calm reading tone is the same on every theme.
const SEPIA_PAPER = 'rgb(242, 231, 211)';
const SEPIA_INK = 'rgb(47, 38, 28)';
const SEPIA_MUTED = 'rgb(107, 91, 71)';

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
  return theme.colors.bg;
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
