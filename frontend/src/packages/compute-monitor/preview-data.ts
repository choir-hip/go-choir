export const previewComputeStatus = {
  status: 'preview',
  current_computer: {
    current: true,
    role: 'public-preview',
    desktop_id: 'public-preview',
    state: 'local',
    warmness_class: 'browser',
    protection: 'This preview is local to the browser. Private computer state starts after sign-in.',
    reclaimable: false,
  },
  computers: [
    {
      current: true,
      role: 'public-preview',
      desktop_id: 'public-preview',
      state: 'local',
      warmness_class: 'browser',
      protection: 'Local preview only',
    },
    {
      role: 'private-computer',
      desktop_id: 'sign-in-required',
      state: 'locked',
      warmness_class: 'auth-required',
      protection: 'Sign in to inspect or mutate your durable computer.',
    },
  ],
  runtime: {
    reachable: true,
    runtime_health: 'public-preview',
    running_runs: 0,
  },
  persistent_disk: {
    source: 'public-preview',
    used_bytes: 2 * 1024 * 1024 * 1024,
    total_bytes: 8 * 1024 * 1024 * 1024,
    avail_bytes: 6 * 1024 * 1024 * 1024,
    cap_bytes: 8 * 1024 * 1024 * 1024,
    used_percent: 25,
    warning: false,
    critical: false,
    default_cap_bytes: 8 * 1024 * 1024 * 1024,
  },
  capabilities: {
    wake_current_computer: false,
  },
  samples: [
    { label: 'CPU', value: 12 },
    { label: 'Memory', value: 28 },
    { label: 'I/O', value: 8 },
    { label: 'Queue', value: 0 },
  ],
  events: [
    'Public shell loaded locally',
    'Private compute telemetry is locked until sign-in',
    'Durable recovery controls are disabled in preview',
  ],
};
