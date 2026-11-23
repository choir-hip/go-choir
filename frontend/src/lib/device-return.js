// The /device page's part of closing the phone's sign-in sheet
// (docs/reference/exact/auth-session.md). The link the app opens carries
// `return` (its own callback scheme: an identifier the SERVER checks against a
// fixed list) and `state` (its nonce). The page sends both with the decision and
// follows, after an approval only, the callback the server answers, only if it has the one shape of a
// "come back" signal. It is no key and no code; the app learns the result by its
// own poll.

// Only the SHAPE of a hint is checked here, not which apps are allowed: the server
// holds the allowlist of app ids (internal/auth device_login.go) and decides what
// is answered. The shape check keeps a crafted link's long or odd hint from being
// sent at all: it could push the body past the server's 1 KiB cap and make the
// approval itself fail.
const RETURN_ID = /^[a-z][a-z0-9.+-]{0,63}$/;
const STATE_SHAPE = /^[A-Za-z0-9_-]{16,128}$/;

function hintOf(search) {
  const q = new URLSearchParams(search || '');
  const ret = q.get('return') || '';
  const state = q.get('state') || '';
  return RETURN_ID.test(ret) && STATE_SHAPE.test(state) ? { ret, state } : null;
}

const RETURN_TO = /^([a-z][a-z0-9.+-]*):\/signed-in\?state=([A-Za-z0-9_-]{16,128})$/;

/** The approve request's body: the code and decision, plus return and state when the link has both in the server's shape. */
export function approveRequestBody(code, decision, search) {
  const body = { user_code: String(code).trim(), decision };
  const hint = hintOf(search);
  if (hint) {
    body.return = hint.ret;
    body.state = hint.state;
  }
  return body;
}

/** Was this page opened by the app's sign-in sheet (a well-formed return and state in the link)? */
export function hasReturnHint(search) {
  return hintOf(search) !== null;
}

/** The URL to navigate to, or null. `state` and `scheme` are the link's hint as the page sent it: the answer's callback must be exactly `<scheme>:/signed-in?state=<state>`; anything else is refused. The caller decides when to follow (returnTarget: only after an approval). */
export function followableReturn(answer, state, scheme) {
  const to = answer && typeof answer === 'object' ? answer.return_to : null;
  if (typeof to !== 'string' || !state || !scheme) return null;
  const m = RETURN_TO.exec(to);
  if (!m || m[1] !== scheme || m[2] !== state) return null;
  return to;
}

/** Where to go after a decision: only after an approval (a denial is shown; the app learns of it by its poll). */
export function returnTarget(decision, answer, search) {
  if (decision !== 'approve') return null;
  const hint = hintOf(search);
  return hint ? followableReturn(answer, hint.state, hint.ret) : null;
}
