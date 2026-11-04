// Logged-out preview listing for Files: local UI data owned by the app.
export const previewFiles = [
  { name: 'Documents', type: 'directory', size: 0, modified: '2026-05-29T09:00:00Z' },
  { name: 'Media', type: 'directory', size: 0, modified: '2026-05-29T09:00:00Z' },
  { name: 'welcome-to-choir.md', type: 'file', size: 6200, modified: '2026-05-29T09:00:00Z' },
  { name: 'sample-brief.pdf', type: 'file', size: 384120, modified: '2026-05-29T09:00:00Z' },
  { name: 'reading-room.epub', type: 'file', size: 842880, modified: '2026-05-29T09:00:00Z' },
  { name: 'quarterly-review.pptx', type: 'file', size: 1204880, modified: '2026-05-29T09:00:00Z' },
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
