// A URL typed into the bar (spine.contract `barUrlOf`, derived as `askUrl`; the rule's own cases are bar-url.test.contract). It was Swift (`BarURL.detect`) on the iPhone only.
// What cannot be run headless: Send. The web host holds no key (signed in cannot be made) and `answer source` is skipped on web, so these tests read the contract's `sendAsk` for its shape. The failure list:
//  S1  Send makes no round trip before it sends: no op is asked what the text is (a pending or failed answer could drop a question, or a second Send overwrite the first), so no `barUrl` source and no `bu` mutation.
//  S2  Send branches in the same step on the derived `askUrl`: an address goes to `queueAct("openWeb", askUrl)`; anything else to `choirAsk(text=ask, ...)`, the field as typed.
//  S3  the field is cleared after the chosen send is issued, in each branch (never before it: nothing depends on `ask` once it is cleared).
//  S4  each branch is gated on being signed in, as the ask always was; signed out opens the sign-in sheet and keeps the text.
//  S5  the Swift module no longer holds the rule, and native's `openWeb` and `closeWeb` verbs to the radio are gone (the typed URL was their one sender).
import { describe, expect, test } from 'bun:test';
import { existsSync, readFileSync } from 'node:fs';

const read = (p: string): string => readFileSync(new URL(p, import.meta.url), 'utf8');
const spine = read('./spine.contract');
const sendAsk = ((): string => { const i = spine.indexOf('  action sendAsk'); const j = spine.indexOf('\n  //', i); return spine.slice(i, j); })();

describe('the bar sends an address or an ask', () => {
  test('S1 no round trip before the send', () => {
    expect(sendAsk).not.toBe('');
    expect(sendAsk).not.toContain('barUrl(');
    expect(spine).not.toContain('mutation bu ');
    expect(spine).not.toContain('action barRoute');
    expect(read('./app.ts')).not.toContain('barUrl');
  });
  test('S2 one step, two branches on askUrl', () => {
    expect(spine).toContain('derive askUrl = barUrlOf(ask)');
    expect(sendAsk).toContain('if au.signedIn and askUrl != ""');
    expect(sendAsk).toContain('send qm = queueAct("openWeb", askUrl)');
    expect(sendAsk).toContain('if au.signedIn and askUrl == "" and canAsk');
    expect(sendAsk).toContain('send acct = choirAsk(text=ask, aim=pg.top)');
  });
  test('S3 the field is cleared after the chosen send, in each branch', () => {
    const a = sendAsk.indexOf('send qm = queueAct'), b = sendAsk.indexOf('send acct = choirAsk');
    expect(sendAsk.slice(a, b)).toContain('ask = ""');
    expect(sendAsk.slice(b)).toContain('ask = ""');
    expect(sendAsk.indexOf('ask = ""')).toBeGreaterThan(a);          // none before the first send
  });
  test('S4 signed out opens the sign-in sheet and keeps the text', () => {
    expect(sendAsk).toContain('if offerSignIn and (canAsk or askUrl != "")\n      gate = "signin"');
  });
  test('S5 the Swift module and the radio verbs', () => {
    expect(read('../exact-package/apple/AutoradioModule+Spine.swift')).not.toContain('BarURL');
    const pb = '../AutoradioKit/Sources/AutoradioKit/PlaybackSupport.swift';
    expect(existsSync(new URL(pb, import.meta.url)) ? read(pb) : '').not.toContain('BarURL');
    expect(read('./radio-bridge.ts')).not.toContain("case 'openWeb': r.web.url = str(m.url)");
    expect(read('./radio-bridge.ts').split("case 'closeWeb': r.web.url = ''; break;").length - 1).toBe(1);   // the queueAct one (a screen's close), not the native verb
  });
});
