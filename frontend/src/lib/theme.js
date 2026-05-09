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
};

const REQUIRED_COLOR_KEYS = ['bg', 'panel', 'panelStrong', 'fg', 'muted', 'accent', 'danger'];

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
  return { ok: errors.length === 0, errors };
}

export function themeCSSVariables(theme = DEFAULT_THEME) {
  return {
    '--choir-bg': theme.colors.bg,
    '--choir-panel': theme.colors.panel,
    '--choir-panel-strong': theme.colors.panelStrong,
    '--choir-fg': theme.colors.fg,
    '--choir-muted': theme.colors.muted,
    '--choir-accent': theme.colors.accent,
    '--choir-danger': theme.colors.danger,
    '--choir-radius-md': theme.radii.md,
    '--choir-radius-lg': theme.radii.lg,
    '--choir-shadow-soft': theme.shadows.soft,
    '--choir-motion-fast': theme.motion.fast,
  };
}
