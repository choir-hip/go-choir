// The passkey ceremony, in one small module: the server's options in, the platform's passkey screen (Exact's `@exact/passkeys`), the credential out.
// It is shared: sign-in and account creation (app-signin.ts) use it, and so does account deletion (a fresh assertion). It holds no key and
// makes no key: the credential it returns goes to a finish route the caller names, and the caller keeps what that route answers.
//
// `create` is the web's own register/begin (an email, then creation options); `get` is a discoverable assertion with no email
// (`/auth/app/login/begin`, internal/auth/app_ceremony.go). The relying party is choir.news, the same one the web site uses, so a passkey made here
// signs in at choir.news and the reverse. A grant `auth.passkey choir.news` and the Associated Domains entitlement (the build derives it) let
// the platform accept the app for that site.
//
// Plain errors, each one a `kind` the screen words (`ceremonyMessage`): the person cancelled, there is no passkey for this site on this phone,
// the passkey exists already, sign-up is closed (the server's waitlist answer), the phone is offline. Nothing here logs a challenge or a credential.

export type CeremonyKind = 'cancelled' | 'exists' | 'closed' | 'offline' | 'unsupported' | 'busy' | 'refused' | 'email' | 'failed';

export class CeremonyError extends Error {
  kind: CeremonyKind;
  constructor(kind: CeremonyKind, message = '') { super(message || kind); this.name = 'CeremonyError'; this.kind = kind; }
}

/** `credentials` of `@exact/passkeys`, as far as this file uses it. `publicKey` is the options' inner object, as the server sends it. */
export type Passkeys = {
  create(options: { publicKey: unknown }): Promise<unknown>;
  get(options: { publicKey: unknown }): Promise<unknown>;
};

export type CeremonyEnv = { origin: string; passkeys: Passkeys; fetch?: typeof fetch };

/** A deadline, not a limit. */
export const CEREMONY_TIMEOUT_MS = 15000;
export const LOGIN_BEGIN_PATH = '/auth/app/login/begin';
export const REGISTER_BEGIN_PATH = '/auth/register/begin';

export type Reply = { status: number; body: unknown };

/** POST JSON to the server's public route. Throws CeremonyError('offline') when no answer came. */
export async function postPublic(env: CeremonyEnv, path: string, body: unknown): Promise<Reply> {
  const doFetch = env.fetch ?? fetch;
  let response: Response;
  try {
    response = await doFetch(env.origin + path, {
      method: 'POST', headers: { 'content-type': 'application/json', accept: 'application/json' },
      body: JSON.stringify(body), exactTimeout: CEREMONY_TIMEOUT_MS,
    } as RequestInit);
  } catch { throw new CeremonyError('offline'); }
  let parsed: unknown = null;
  try { parsed = JSON.parse(await response.text()); } catch { /* the status says enough */ }
  return { status: response.status, body: parsed };
}

const field = (o: unknown, name: string): unknown => (o && typeof o === 'object' ? (o as Record<string, unknown>)[name] : undefined);

/** A refusal from the server's begin or finish route, as a CeremonyError. The server's own `error` string is kept only for the cases where it is the person's to read. */
export function refusal(reply: Reply): CeremonyError {
  const code = field(reply.body, 'code'), text = String(field(reply.body, 'error') ?? '').slice(0, 200);
  if (reply.status === 403 && code === 'registration_waitlist') return new CeremonyError('closed', text);
  if (reply.status === 409) return new CeremonyError('exists', text);
  if (reply.status === 429) return new CeremonyError('busy', text);
  if (reply.status === 400 && /valid email|email is required/i.test(text)) return new CeremonyError('email', text);
  if (reply.status === 503) return new CeremonyError('busy', text);
  return new CeremonyError('failed', `the server answered ${reply.status}`);
}

/** What the platform's rejection is: Exact answers WebAuthn's DOMException names (`name`, and Apple's `code`). */
export function platformError(e: unknown): CeremonyError {
  const name = String(field(e, 'name') ?? field(e, 'code') ?? '');
  if (name === 'NotAllowedError' || name === 'AbortError') return new CeremonyError('cancelled');
  if (name === 'InvalidStateError') return new CeremonyError('exists');
  if (name === 'NotSupportedError') return new CeremonyError('unsupported');
  if (name === 'SecurityError') return new CeremonyError('refused');
  return new CeremonyError('failed', name || 'the passkey screen failed');
}

function publicKeyOf(body: unknown): unknown {
  const pk = field(body, 'publicKey');
  if (!pk || typeof pk !== 'object') throw new CeremonyError('failed', 'the server sent no passkey options');
  return pk;
}

/** Create a passkey for `email`: the server's creation options, the platform's screen. Answers the credential the finish route takes. */
export async function createCeremony(env: CeremonyEnv, email: string): Promise<unknown> {
  const begin = await postPublic(env, REGISTER_BEGIN_PATH, { email });
  if (begin.status !== 200) throw refusal(begin);
  const publicKey = publicKeyOf(begin.body);
  try { return await env.passkeys.create({ publicKey }); } catch (e) { throw platformError(e); }
}

/** Sign in with a passkey the phone holds for this site; no email is typed (a discoverable credential). `beginPath` is the route that makes the options (a deletion's fresh assertion names its own). */
export async function getCeremony(env: CeremonyEnv, beginPath: string = LOGIN_BEGIN_PATH, beginBody: unknown = {}): Promise<unknown> {
  const begin = await postPublic(env, beginPath, beginBody);
  if (begin.status !== 200) throw refusal(begin);
  const publicKey = publicKeyOf(begin.body);
  try { return await env.passkeys.get({ publicKey }); } catch (e) { throw platformError(e); }
}

/** The words for a failure, plain and short. `mode`: what the person was doing. A cancel on sign-in also covers "no passkey for choir.news on this phone": the platform answers the two alike. */
export function ceremonyMessage(e: unknown, mode: 'create' | 'signin'): string {
  const kind: CeremonyKind = e instanceof CeremonyError ? e.kind : 'failed';
  switch (kind) {
    case 'cancelled': return mode === 'create' ? 'No passkey was made.' : 'No passkey was used. If this phone has none for Choir, create an account first.';
    case 'exists': return mode === 'create' ? 'That email already has an account, or this phone already has its passkey. Sign in instead.' : 'This phone has no passkey that works here. Create an account first.';
    case 'closed': return 'Choir is full for now. You are on the waiting list, and we will email you when there is room.';
    case 'offline': return 'No connection. Check the network and try again.';
    case 'unsupported': return 'This phone cannot make passkeys here. Check that iCloud Keychain is on.';
    case 'refused': return 'The app could not be verified for choir.news. Try again later.';
    case 'busy': return 'Choir is busy. Try again in a moment.';
    case 'email': return 'Enter a valid email address.';
    default: return 'Something went wrong. Try again.';
  }
}
