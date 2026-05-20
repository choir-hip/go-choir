export const OVERVIEW_MOBILE_BREAKPOINT = 768;
export const OVERVIEW_MOBILE_LIVE_LIMIT = 3;
export const OVERVIEW_DESKTOP_LIVE_LIMIT = 6;

const REDACTED_PREVIEW_APP_IDS = new Set(['terminal']);

function isVisibleWindow(win) {
  return win && win.mode !== 'closed' && win.mode !== 'hidden';
}

function canUseLivePreview(win) {
  if (!isVisibleWindow(win)) return false;
  if (win.mode === 'minimized') return false;
  if (win.restoreSuspended) return false;
  if (REDACTED_PREVIEW_APP_IDS.has(win.appId)) return false;
  return true;
}

export function overviewLiveLimit(viewportWidth = 1280) {
  return viewportWidth < OVERVIEW_MOBILE_BREAKPOINT
    ? OVERVIEW_MOBILE_LIVE_LIMIT
    : OVERVIEW_DESKTOP_LIVE_LIMIT;
}

export function createOverviewPreviewDecisions(
  windows = [],
  activeWindowId = '',
  options = {}
) {
  const viewportWidth = options.viewportWidth || 1280;
  const liveLimit = overviewLiveLimit(viewportWidth);
  const openWindows = (windows || [])
    .filter(isVisibleWindow)
    .sort((a, b) => (b.zIndex || 0) - (a.zIndex || 0));
  const safeLiveWindows = openWindows.filter(canUseLivePreview);
  const activeLive = safeLiveWindows.find((win) => win.windowId === activeWindowId);
  const orderedLive = [
    ...(activeLive ? [activeLive] : []),
    ...safeLiveWindows.filter((win) => win.windowId !== activeWindowId),
  ];
  const liveWindowIds = new Set(
    orderedLive.slice(0, liveLimit).map((win) => win.windowId)
  );
  const liveCount = liveWindowIds.size;
  const decisions = {};

  openWindows.forEach((win) => {
    let state = 'card';
    if (win.restoreSuspended) {
      state = 'suspended';
    } else if (REDACTED_PREVIEW_APP_IDS.has(win.appId)) {
      state = 'redacted';
    } else if (win.mode === 'minimized') {
      state = 'card';
    } else if (liveWindowIds.has(win.windowId)) {
      state = 'live';
    }

    decisions[win.windowId] = {
      state,
      liveIndex: state === 'live'
        ? orderedLive.findIndex((item) => item.windowId === win.windowId)
        : -1,
      liveCount,
      liveLimit,
    };
  });

  return decisions;
}

export function getOverviewPreviewDecision(decisions = {}, windowId = '') {
  return decisions[windowId] || {
    state: 'normal',
    liveIndex: -1,
    liveCount: 0,
    liveLimit: 0,
  };
}

export function summarizeOverviewPreviewDecisions(decisions = {}) {
  const summary = {
    live: 0,
    card: 0,
    suspended: 0,
    redacted: 0,
    normal: 0,
  };

  Object.values(decisions || {}).forEach((decision) => {
    const state = decision?.state || 'normal';
    summary[state] = (summary[state] || 0) + 1;
  });

  return summary;
}
