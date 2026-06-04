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

export const previewTraceTrajectories = [
  {
    trajectory_id: 'preview-writing-run',
    title: 'Example writing run',
    subtitle: 'A local-only trace showing how agents, tools, and revisions line up',
    state: 'completed',
    live: false,
    agent_count: 4,
    delegation_count: 3,
    moment_count: 7,
    message_count: 18,
    finding_count: 3,
    search_attempt_count: 2,
    latest_activity_at: '2026-05-29T09:00:00Z',
  },
  {
    trajectory_id: 'preview-review-run',
    title: 'Example review run',
    subtitle: 'A second local preview for switching traces without private data',
    state: 'completed',
    live: false,
    agent_count: 3,
    delegation_count: 1,
    moment_count: 5,
    message_count: 12,
    finding_count: 2,
    search_attempt_count: 1,
    latest_activity_at: '2026-05-29T08:45:00Z',
  },
];

export const previewTraceSnapshot = {
  trajectory: previewTraceTrajectories[0],
  agents: [
    { agent_id: 'conductor', label: 'conductor', role: 'router', profile: 'foreground', state: 'completed' },
    { agent_id: 'vtext', label: 'VText', role: 'document owner', profile: 'foreground app', state: 'completed' },
    { agent_id: 'researcher', label: 'researcher', role: 'research', profile: 'background', state: 'completed' },
    { agent_id: 'verifier', label: 'verifier', role: 'verification', profile: 'read-only', state: 'completed' },
  ],
  edges: [
    { from_agent_id: 'conductor', to_agent_id: 'vtext', kind: 'handoff' },
    { from_agent_id: 'vtext', to_agent_id: 'researcher', kind: 'research-request' },
    { from_agent_id: 'vtext', to_agent_id: 'verifier', kind: 'review-request' },
  ],
  moments: [
    { moment_id: 'p1', agent_id: 'conductor', agent_label: 'conductor', kind: 'prompt.received', state: 'completed', tone: 'message', title: 'Prompt routed', summary: 'A writing request is routed to VText.', timestamp: '2026-05-29T08:52:00Z', created_at: '2026-05-29T08:52:00Z', loop_id: 'preview-writing-run' },
    { moment_id: 'p2', agent_id: 'vtext', agent_label: 'VText', kind: 'document.draft', state: 'completed', tone: 'active', title: 'Draft opened', summary: 'A local draft surface appears before any durable save.', timestamp: '2026-05-29T08:53:00Z', created_at: '2026-05-29T08:53:00Z', loop_id: 'preview-writing-run' },
    { moment_id: 'p3', agent_id: 'researcher', agent_label: 'researcher', kind: 'source.scan', state: 'completed', tone: 'tool', title: 'Sources gathered', summary: 'Example source notes are attached to the draft.', timestamp: '2026-05-29T08:54:00Z', created_at: '2026-05-29T08:54:00Z', loop_id: 'preview-writing-run' },
    { moment_id: 'p4', agent_id: 'vtext', agent_label: 'VText', kind: 'revision.proposed', state: 'completed', tone: 'success', title: 'Revision proposed', summary: 'The draft advances from v1 to v2.', timestamp: '2026-05-29T08:55:00Z', created_at: '2026-05-29T08:55:00Z', loop_id: 'preview-writing-run' },
    { moment_id: 'p5', agent_id: 'researcher', agent_label: 'researcher', kind: 'finding.linked', state: 'completed', tone: 'message', title: 'Finding linked', summary: 'A source note is linked to the active revision.', timestamp: '2026-05-29T08:56:00Z', created_at: '2026-05-29T08:56:00Z', loop_id: 'preview-writing-run' },
    { moment_id: 'p6', agent_id: 'verifier', agent_label: 'verifier', kind: 'review.visual', state: 'completed', tone: 'success', title: 'Preview checked', summary: 'The verifier records that this is local preview evidence only.', timestamp: '2026-05-29T08:57:00Z', created_at: '2026-05-29T08:57:00Z', loop_id: 'preview-writing-run' },
    { moment_id: 'p7', agent_id: 'vtext', agent_label: 'VText', kind: 'publish.blocked', state: 'completed', tone: 'warn', title: 'Publish locked', summary: 'Publishing is held behind sign-in and owner-scoped state.', timestamp: '2026-05-29T08:58:00Z', created_at: '2026-05-29T08:58:00Z', loop_id: 'preview-writing-run' },
  ],
  search: {
    attempts: 2,
    successes: 2,
    providers: [
      { provider: 'local-preview', status: 'ok', successes: 2, attempts: 2, rate_limits: 0, merged_count: 3 },
      { provider: 'private-search', status: 'auth-required', successes: 0, attempts: 0, rate_limits: 0, merged_count: 0 },
    ],
  },
  acceptances: [
    {
      acceptance_id: 'preview-local-only',
      target_mission_id: 'public-preview',
      state: 'preview',
      acceptance_level: 'local-ui-preview',
      authority_profile: 'signed-out local preview',
      deployment_commit: 'none',
      staging_url: 'browser-local',
      loop_id: 'preview-writing-run',
      trajectory_id: 'preview-writing-run',
      checkpoints: [
        {
          kind: 'local_preview',
          state: 'preview',
          evidence_ref_ids: ['preview-vtext', 'preview-trace'],
          details: {
            note: 'Preview data is local UI data. It is never backend proof and is never written to a signed-in account.',
          },
        },
      ],
      evidence_refs: [
        { ref_id: 'preview-vtext', kind: 'local-ui-preview', summary: 'VText opens before sign-in.' },
        { ref_id: 'preview-trace', kind: 'local-ui-preview', summary: 'Trace layout renders without private trajectories.' },
      ],
      rollback_refs: [],
      verifier_contracts: [
        {
          name: 'Local preview boundary',
          state: 'preview',
          purpose: 'Show interface shape without claiming durable product evidence.',
        },
      ],
    },
  ],
};

export const previewVTextDocument = {
  doc_id: 'preview-vtext',
  title: 'What Choir Is',
  content: [
    '# What Choir Is',
    '',
    'Choir is a private, VText-centered computer for durable knowledge work. Documents are versioned artifacts, not chat transcripts, so drafts can be revised, compared, cited, published, and recovered.',
    '',
    'The signed-out desktop is a preview of the reading and writing surface. Sign in to connect your durable computer, save revisions, import sources, run agents, publish work, and keep the evidence attached to the artifact.',
  ].join('\n'),
  revisions: [
    { revision_id: 'v1', label: 'v1', title: 'Artifact', summary: 'VText is the durable surface for writing and revision.' },
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
