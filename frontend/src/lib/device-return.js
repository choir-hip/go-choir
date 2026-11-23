// The /device page's part of closing the phone's sign-in sheet
// (docs/reference/exact/auth-session.md). The link the app opens carries
// `return` (its own callback scheme: an identifier the SERVER checks against a
// fixed list) and `state` (its nonce). The page sends both with the decision and
// follows the callback the server answers, only if it has the one shape of a
// "come back" signal. It is no key and no code; the app learns the result by its
// own poll.

const RETURN_TO = /^([a-z][a-z0-9.+-]*):\/signed-in\?state=([A-Za-z0-9_-]{16,128})$/;

/** The approve request's body: the code and decision, plus return and state when the link has both. */
export function approveRequestBody(code, decision, search) {
  const body = { user_code: String(code).trim(), decision };
  const q = new URLSearchParams(search || '');
  const ret = q.get('return');
  const state = q.get('state');
  if (ret && state) {
    body.return = ret;
    body.state = state;
  }
  return body;
}

/** Was this page opened by the app's sign-in sheet (both fields in the link)? */
export function hasReturnHint(search) {
  const q = new URLSearchParams(search || '');
  return !!(q.get('return') && q.get('state'));
}

/** The URL to navigate to, or null. `state` and `scheme` are what the page sent: the callback must carry that state and be that scheme (no deny-list: anything else is refused). */
export function followableReturn(answer, state, scheme) {
  const to = answer && typeof answer === 'object' ? answer.return_to : null;
  if (typeof to !== 'string' || !state || !scheme) return null;
  const m = RETURN_TO.exec(to);
  if (!m || m[1] !== scheme || m[2] !== state) return null;
  return to;
}
