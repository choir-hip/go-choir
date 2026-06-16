export const previewFiles = [
  { name: 'Documents', type: 'directory', size: 0, modified: '2026-05-29T09:00:00Z' },
  { name: 'Media', type: 'directory', size: 0, modified: '2026-05-29T09:00:00Z' },
  { name: 'welcome-to-choir.md', type: 'file', size: 6200, modified: '2026-05-29T09:00:00Z' },
  { name: 'sample-brief.pdf', type: 'file', size: 384120, modified: '2026-05-29T09:00:00Z' },
  { name: 'reading-room.epub', type: 'file', size: 842880, modified: '2026-05-29T09:00:00Z' },
];

export const previewFolderEntries: Record<string, any[]> = {
  Documents: [
    { name: 'first-draft.md', type: 'file', size: 4200, modified: '2026-05-29T09:00:00Z' },
    { name: 'research-outline.md', type: 'file', size: 5800, modified: '2026-05-29T09:00:00Z' },
  ],
  Media: [
    { name: 'city-window.png', type: 'file', size: 312044, modified: '2026-05-29T09:00:00Z' },
    { name: 'audio-note.mp3', type: 'file', size: 4204450, modified: '2026-05-29T09:00:00Z' },
    { name: 'screen-recording.mp4', type: 'file', size: 9804450, modified: '2026-05-29T09:00:00Z' },
  ],
};

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

export const previewTextureDocument = {
  doc_id: 'preview-texture',
  title: 'What Choir Is',
  content: [
    '# What Choir Is',
    '',
    'Choir is a private, Texture-centered computer for durable knowledge work. Documents are versioned artifacts, not chat transcripts, so drafts can be revised, compared, cited, published, and recovered.',
    '',
    'The signed-out desktop is a preview of the reading and writing surface. Sign in to connect your durable computer, save revisions, import sources, run agents, publish work, and keep the evidence attached to the artifact.',
  ].join('\n'),
  revisions: [
    { revision_id: 'v1', label: 'v1', title: 'Artifact', summary: 'Texture is the durable surface for writing and revision.' },
    { revision_id: 'v2', label: 'v2', title: 'Sources', summary: 'Sources and evidence stay connected to the work.' },
    { revision_id: 'v3', label: 'v3', title: 'Publish', summary: 'Publishing and private computer actions unlock after sign-in.' },
  ],
};

export const previewEmailMessages = [
  {
    id: 'preview-email-1',
    direction: 'inbound',
    from_address: 'preview@choir.news',
    subject: 'Email preview',
    snippet: 'This mailbox is local preview data. Real aliases, drafts, and sending require sign-in.',
    trust_status: 'public-preview',
    received_at: '2026-05-29T09:00:00Z',
    has_attachments: false,
  },
  {
    id: 'preview-email-2',
    direction: 'draft',
    from_address: 'preview@choir.news',
    subject: 'Draft: sign-in required',
    snippet: 'Compose and send actions are locked until a real mailbox is connected.',
    trust_status: 'draft-preview',
    created_at: '2026-05-29T09:02:00Z',
    has_attachments: false,
  },
];

export const previewPodcastItems = [
  {
    content_id: 'preview-podcast-1',
    title: 'Choir preview feed',
    source_url: 'https://example.com/choir-preview.rss',
    media_type: 'application/rss+xml',
    app_hint: 'podcast',
    text_content: `<?xml version="1.0"?><rss><channel><title>Choir preview feed</title><description>Local sample feed for the public interface preview.</description><item><title>How previews become private work</title><description>Sign in to import real feeds, sync playback, or spend provider calls.</description><pubDate>Fri, 29 May 2026 09:00:00 GMT</pubDate><enclosure url="https://example.com/audio.mp3" type="audio/mpeg"/></item></channel></rss>`,
  },
];

export const previewFeaturePackages = [
  {
    package_id: 'preview-super-console-zot',
    app_id: 'super-console',
    manifest_json: JSON.stringify({
      title: 'Super Console zot session',
      summary: 'Shows the singleton repair console shape without exposing private machine state.',
    }),
    provenance_refs_json: JSON.stringify({
      screenshot: ['local-preview'],
      narrative: ['Local UI preview only'],
    }),
    candidate_source_ref: 'preview-super-console-zot',
    source_runtime_artifact_digest: 'local-preview',
    source_ui_artifact_digest: 'local-preview',
  },
  {
    package_id: 'preview-theme-switching',
    app_id: 'settings',
    manifest_json: JSON.stringify({
      title: 'Theme switching',
      summary: 'Settings remains public enough to inspect Futuristic Noir, Carbon Fiber Kintsugi, and London Salmon.',
    }),
    provenance_refs_json: JSON.stringify({
      screenshot: ['local-preview'],
      narrative: ['Local UI preview only'],
    }),
    candidate_source_ref: 'preview-theme-switching',
    source_runtime_artifact_digest: 'local-preview',
    source_ui_artifact_digest: 'local-preview',
  },
];

export const previewFeatureAdoptions = [
  {
    adoption_id: 'preview-theme-switching',
    package_id: 'preview-theme-switching',
    app_id: 'settings',
    status: 'preview',
    trace_id: 'preview-writing-run',
    updated_at: '2026-05-29T09:04:00Z',
    rollback_profile_json: JSON.stringify({ preview: true }),
    candidate_source_ref: 'preview-theme-switching',
    runtime_artifact_digest: 'local-preview',
    ui_artifact_digest: 'local-preview',
  },
];
