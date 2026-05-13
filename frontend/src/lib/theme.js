export const THEME_SCHEMA_VERSION = 1;

export const DEFAULT_THEME = {
  schema_version: THEME_SCHEMA_VERSION,
  id: 'system-noir',
  name: 'System Noir',
  colors: {
    bg: '#0b0d10',
    panel: '#171827',
    panelStrong: '#11111b',
    fg: '#f8fafc',
    muted: '#a8b3c7',
    accent: '#60a5fa',
    danger: '#f87171',
    border: 'rgba(148, 163, 184, 0.18)',
  },
  radii: {
    md: '12px',
    lg: '18px',
  },
  shadows: {
    soft: '0 16px 42px rgba(0, 0, 0, 0.28)',
  },
  motion: {
    fast: '120ms ease',
  },
  layout: {
    bottomBarHeight: '56px',
  },
};

export const THEME_STORAGE_KEY = 'choir.theme.v1';

export const THEME_PRESETS = [
  DEFAULT_THEME,
  {
    ...DEFAULT_THEME,
    id: 'next-workstation',
    name: 'NeXT Workstation',
    colors: {
      bg: '#1f2024',
      panel: '#d3d3d3',
      panelStrong: '#a8a8a8',
      fg: '#111111',
      muted: '#3f3f46',
      accent: '#f2a900',
      danger: '#b42318',
      border: '#5b5b60',
    },
    radii: { md: '2px', lg: '4px' },
    shadows: { soft: '0 10px 24px rgba(0, 0, 0, 0.36)' },
  },
  {
    ...DEFAULT_THEME,
    id: 'classic-mac',
    name: 'Classic Mac',
    colors: {
      bg: '#6d7480',
      panel: '#eeeeee',
      panelStrong: '#d6d6d6',
      fg: '#111111',
      muted: '#4b5563',
      accent: '#0f62fe',
      danger: '#c62828',
      border: '#2f343a',
    },
    radii: { md: '0px', lg: '0px' },
    shadows: { soft: '3px 3px 0 rgba(0, 0, 0, 0.42)' },
  },
  {
    ...DEFAULT_THEME,
    id: 'aqua-glass',
    name: 'Aqua Glass',
    colors: {
      bg: '#d8ecff',
      panel: '#f7fbff',
      panelStrong: '#b9d9ff',
      fg: '#0c1726',
      muted: '#35506f',
      accent: '#007aff',
      danger: '#ff3b30',
      border: 'rgba(17, 64, 116, 0.28)',
    },
    radii: { md: '10px', lg: '14px' },
    shadows: { soft: '0 18px 38px rgba(21, 80, 140, 0.24)' },
  },
  {
    ...DEFAULT_THEME,
    id: 'frutiger-aero',
    name: 'Frutiger Aero',
    colors: {
      bg: '#0e6f8f',
      panel: '#d8fff5',
      panelStrong: '#5bd6d6',
      fg: '#062b36',
      muted: '#26636d',
      accent: '#7bd923',
      danger: '#e03a3e',
      border: 'rgba(255, 255, 255, 0.42)',
    },
    radii: { md: '14px', lg: '20px' },
    shadows: { soft: '0 20px 44px rgba(0, 70, 96, 0.32)' },
  },
  {
    ...DEFAULT_THEME,
    id: 'gtk-slate',
    name: 'GTK Slate',
    colors: {
      bg: '#2b3035',
      panel: '#f2f0ed',
      panelStrong: '#d9d6d0',
      fg: '#1f2328',
      muted: '#58606a',
      accent: '#3584e4',
      danger: '#c01c28',
      border: '#9a9996',
    },
    radii: { md: '6px', lg: '8px' },
    shadows: { soft: '0 12px 28px rgba(0, 0, 0, 0.28)' },
  },
  {
    ...DEFAULT_THEME,
    id: 'y3k-console',
    name: 'Y3K Console',
    colors: {
      bg: '#030712',
      panel: '#10172a',
      panelStrong: '#18112f',
      fg: '#e8fbff',
      muted: '#91a4bd',
      accent: '#00f5d4',
      danger: '#ff2a6d',
      border: 'rgba(0, 245, 212, 0.28)',
    },
    radii: { md: '4px', lg: '6px' },
    shadows: { soft: '0 0 28px rgba(0, 245, 212, 0.14)' },
  },
];

const REQUIRED_COLOR_KEYS = ['bg', 'panel', 'panelStrong', 'fg', 'muted', 'accent', 'danger', 'border'];

function isObject(value) {
  return value !== null && typeof value === 'object' && !Array.isArray(value);
}

function hasString(object, key) {
  return typeof object?.[key] === 'string' && object[key].trim() !== '';
}

export function validateThemeConfig(theme) {
  const errors = [];
  if (!isObject(theme)) {
    return { ok: false, errors: ['theme must be an object'] };
  }
  if (theme.schema_version !== THEME_SCHEMA_VERSION) {
    errors.push(`schema_version must be ${THEME_SCHEMA_VERSION}`);
  }
  if (!hasString(theme, 'id')) errors.push('id is required');
  if (!hasString(theme, 'name')) errors.push('name is required');
  if (!isObject(theme.colors)) {
    errors.push('colors object is required');
  } else {
    for (const key of REQUIRED_COLOR_KEYS) {
      if (!hasString(theme.colors, key)) {
        errors.push(`colors.${key} is required`);
      }
    }
  }
  for (const [section, keys] of Object.entries({
    radii: ['md', 'lg'],
    shadows: ['soft'],
    motion: ['fast'],
  })) {
    if (theme[section] !== undefined) {
      if (!isObject(theme[section])) {
        errors.push(`${section} object is required when provided`);
      } else {
        for (const key of keys) {
          if (theme[section][key] !== undefined && !hasString(theme[section], key)) {
            errors.push(`${section}.${key} must be a string`);
          }
        }
      }
    }
  }
  return { ok: errors.length === 0, errors };
}

export function normalizeThemeConfig(theme = DEFAULT_THEME) {
  return {
    ...DEFAULT_THEME,
    ...theme,
    colors: { ...DEFAULT_THEME.colors, ...(theme?.colors || {}) },
    radii: { ...DEFAULT_THEME.radii, ...(theme?.radii || {}) },
    shadows: { ...DEFAULT_THEME.shadows, ...(theme?.shadows || {}) },
    motion: { ...DEFAULT_THEME.motion, ...(theme?.motion || {}) },
    layout: { ...DEFAULT_THEME.layout, ...(theme?.layout || {}) },
  };
}

export function themeCSSVariables(theme = DEFAULT_THEME) {
  theme = normalizeThemeConfig(theme);
  return {
    '--choir-bg': theme.colors.bg,
    '--choir-panel': theme.colors.panel,
    '--choir-panel-strong': theme.colors.panelStrong,
    '--choir-fg': theme.colors.fg,
    '--choir-muted': theme.colors.muted,
    '--choir-accent': theme.colors.accent,
    '--choir-danger': theme.colors.danger,
    '--choir-border': theme.colors.border,
    '--choir-radius-md': theme.radii.md,
    '--choir-radius-lg': theme.radii.lg,
    '--choir-shadow-soft': theme.shadows.soft,
    '--choir-motion-fast': theme.motion.fast,
    '--choir-bottom-bar-height': theme.layout?.bottomBarHeight || DEFAULT_THEME.layout.bottomBarHeight,
  };
}

export function themeStyleString(theme = DEFAULT_THEME) {
  return Object.entries(themeCSSVariables(theme))
    .map(([key, value]) => `${key}: ${value}`)
    .join('; ');
}

export function applyThemeToElement(element, theme = DEFAULT_THEME) {
  theme = normalizeThemeConfig(theme);
  const variables = themeCSSVariables(theme);
  if (!element) return variables;
  for (const [key, value] of Object.entries(variables)) {
    element.style.setProperty(key, value);
  }
  return variables;
}
