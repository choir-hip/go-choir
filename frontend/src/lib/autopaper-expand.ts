// "More research" on a published story (story expansion design, phase 1,
// docs/reports/story-expansion-request-design-2026-10-20.md). A reader asks the
// paper to expand one story; no sign-in is needed when the public door is on.
// The proxy decides who may, and answers allowed:false when the feature is
// off or the route is not a story, so the page shows nothing then.
// Everything here is display: the server's answer is text, never HTML, and
// any failure means "show nothing" or a plain notice, never a broken story.

/** How often an open request is re-read. */
export const EXPAND_POLL_MS = 30_000;

export type ExpandRequestView = {
  request_id: string;
  state: string;
  asked_at?: string;
  from_version?: number;
  to_version?: number;
  revision_id?: string;
};

export type ExpansionRead = {
  allowed: boolean;
  route?: string;
  asked?: boolean;
  requests?: ExpandRequestView[];
};

/**
 * What the proxy says about asking this story to be expanded: null on any
 * failure or malformed answer. Only a literal allowed:true shows the control.
 */
export async function fetchExpansion(route: string, signal?: AbortSignal): Promise<ExpansionRead | null> {
  try {
    const params = new URLSearchParams({ route });
    const res = await fetch(`/api/platform/publications/expansion?${params}`, {
      method: 'GET',
      credentials: 'same-origin',
      signal,
    });
    if (!res.ok) return null;
    const body = await res.json();
    if (!body || typeof body !== 'object' || typeof body.allowed !== 'boolean') return null;
    return {
      allowed: body.allowed === true,
      route: typeof body.route === 'string' ? body.route : undefined,
      asked: body.asked === true,
      requests: Array.isArray(body.requests) ? body.requests.filter(isRequestView) : [],
    };
  } catch {
    return null;
  }
}

function isRequestView(value: unknown): value is ExpandRequestView {
  return !!value && typeof value === 'object'
    && typeof (value as ExpandRequestView).request_id === 'string'
    && typeof (value as ExpandRequestView).state === 'string';
}

export type AskResult = { ok: boolean; status: string; message: string };

const DECLINED: Record<string, string> = {
  already_asked: 'This version was already asked to be expanded.',
  limit: 'Too many expansion requests right now. Try again later.',
  address_limit: 'Too many requests from your address; try later.',
  busy: 'The story\'s desk is busy. Try again shortly.',
  changed: 'The story just changed. Try again.',
  unavailable: 'The paper is not reachable right now. Try again shortly.',
};

/** The notice for a refused or failed ask; plain text. */
export function askNotice(httpStatus: number, status: string): string {
  if (DECLINED[status]) return DECLINED[status];
  if (httpStatus === 401 || httpStatus === 403 || httpStatus === 404) return 'Asking for more research is not available here.';
  if (httpStatus === 429) return DECLINED.limit;
  return 'Could not ask just now. Try again shortly.';
}

/** Ask the paper to expand a story. Never throws. */
export async function askExpansion(route: string): Promise<AskResult> {
  try {
    const res = await fetch('/api/platform/publications/expand', {
      method: 'POST',
      credentials: 'same-origin',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ route }),
    });
    let body: { status?: string } = {};
    try {
      body = await res.json();
    } catch {
      body = {};
    }
    const status = typeof body?.status === 'string' ? body.status : '';
    if (res.status === 202 && status === 'asked') return { ok: true, status, message: '' };
    return { ok: false, status, message: askNotice(res.status, status) };
  } catch {
    return { ok: false, status: '', message: askNotice(0, '') };
  }
}

/** The newest request of a read, or null. */
export function latestRequest(read: ExpansionRead | null | undefined): ExpandRequestView | null {
  const requests = Array.isArray(read?.requests) ? read!.requests! : [];
  return requests[0] || null;
}

/** The fate line for a request; empty for a state the page does not know. */
export function fateLine(request: ExpandRequestView | null | undefined): string {
  if (!request) return '';
  switch (request.state) {
    case 'asked':
      return 'Asked. Waiting for the desk to take it.';
    case 'accepted':
      return 'Request received. The desk may add research or leave the story as it is.';
    case 'revised': {
      const v = Number(request.to_version);
      return Number.isInteger(v) && v > 0 ? `Revised since your request: version ${v} is up.` : 'Revised since your request.';
    }
    case 'no_new_version':
      return 'No new version yet. The desk may have found nothing to add.';
    case 'unknown':
      return 'Could not check the status right now.';
    default:
      return '';
  }
}

/** Whether a read has a request still moving, so the page keeps polling. */
export function expansionIsOpen(read: ExpansionRead | null | undefined): boolean {
  const request = latestRequest(read);
  return !!request && (request.state === 'asked' || request.state === 'accepted');
}
