// The overview's reading of the host's preview decisions (the host decides
// which windows show live previews: lib/desktop-overview-preview.js).
export function getOverviewPreviewDecision(decisions = {}, windowId = '') {
  return decisions[windowId] || { state: 'normal', liveIndex: -1, liveCount: 0, liveLimit: 0 };
}

export function summarizeOverviewPreviewDecisions(decisions = {}) {
  const summary = { live: 0, card: 0, suspended: 0, redacted: 0, normal: 0 };
  Object.values(decisions || {}).forEach((decision) => {
    const state = decision?.state || 'normal';
    summary[state] = (summary[state] || 0) + 1;
  });
  return summary;
}
