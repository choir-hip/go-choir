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

/**
 * The signed-out "What Choir Is" preview document.
 *
 * This is the first real sentence a signed-out visitor reads about the
 * product, so it is written as an argument, not a glossary. The previous
 * copy opened with "a private, Texture-centered computer for durable
 * knowledge work" — accurate, and meaningless to anyone who has not been
 * briefed on the ontology. It named the mechanism before it named the
 * problem, so a visitor had no reason to care.
 *
 * The shape now is: the shared failure → the reframe → the mechanism →
 * what the visitor can do next. Jargon is introduced only after the
 * reader has a reason to want it.
 */
export const previewTextureDocument = {
  doc_id: 'preview-texture',
  title: 'What Choir Is',
  content: [
    '# What Choir Is',
    '',
    'Every AI session starts from zero. You re-explain the project, it re-guesses the rules, and the thread dies with the tab. That is fine for ten minutes of thinking and useless for three months of work.',
    '',
    '**Choir is a computer, not a conversation.** A persistent machine made of many agents that coordinate over months instead of minutes. It keeps a versioned record of everything it does — every draft, every claim, every reversal — so the work survives you closing the tab.',
    '',
    '## How it is put together',
    '',
    'Four desks run on one machine, and you sit above all of them. **Texture** writes the durable documents. **Management** decides what is worth running. **Engineering** does the building, inside a sandbox it cannot escape. **Research** goes and gets evidence from the world. You state the intent; they do the work and show you the receipts.',
    '',
    '## Why you can trust it',
    '',
    'Nothing moves that you cannot read, cite, or undo. Every state change is a typed event with evidence attached, and every one of them rolls back. Before the agents act, they commit to typed predictions; afterwards those commitments are scored. The accumulated log is how the machine learns, and it is yours to inspect.',
    '',
    'That discipline applies to Choir itself. The computer proposes changes to its own environment, tools, and operating rules under the same evidence-and-approval rules as any other change. It is the essential capability, not a feature.',
    '',
    '## What you get',
    '',
    'A **web desktop** for durable writing, sources, files, and a repair console. A **native macOS app** wrapping the same computer. A **CLI** for agents and scripts. All three are projections of one persistent machine — not three products that disagree.',
    '',
    '---',
    '',
    '*You are looking at a local preview. Sign in to connect your own durable computer: your documents keep their revisions, sources stay attached, and the agents have somewhere to work.*',
  ].join('\n'),
  revisions: [
    {
      revision_id: 'v1',
      label: 'v1',
      title: 'The problem',
      summary: 'Why a session is the wrong unit of work.',
    },
    {
      revision_id: 'v2',
      label: 'v2',
      title: 'The machine',
      summary: 'Four desks, one computer, you on top.',
    },
    {
      revision_id: 'v3',
      label: 'v3',
      title: 'The receipts',
      summary: 'Every change typed, evidenced, and reversible.',
    },
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

