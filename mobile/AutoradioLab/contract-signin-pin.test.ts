// The sign-in sheet is offered only when the account says a sign-in is right (account.contract `offerSignIn`, from `acs.action`): `au.signedIn` is false for a key that could not be read too.
// This pins every non-comment line of every non-test contract that reads `au.signedIn` or names the "signin" gate, with the reason a plain flag is right there. A NEW line fails until it is reviewed
// and added. (members-settings.contract is also edited by another lane: a change to its pinned lines there is a rebase to resolve here, not a defect.)
// The web host cannot fake a locked account (`answer source` is skipped on web), so the sites are held by this pin, by the derive checks below and by account-state.test.ts (the action of every state).
import { describe, expect, test } from 'bun:test';
import { readdirSync, readFileSync } from 'node:fs';

const PINNED: [string, string, string][] = [
  ["account.contract", "resource acs = accountState(`${au.signedIn}|${siv.phase}|${siv.message}|${signedOutTick}|${hpg.visibilityState}|${hpg.hasFocus}|${lockTick}|${hc.mode}|${hpg.platform}`, pg.top, hc.mode, hpg.platform) as shape AccountView else empty()", "the source argument is a tick: it only makes the account answer be asked again when the flag moves"],
  ["account.contract", "send signedRun = signInRun(\"signin\", \"\", \"\")", "starts the sign-in the listener asked for by pressing the sheet's Sign in; not a decision to offer it"],
  ["consent.contract", "if gate == \"signin\"", "draws the sheet that a gate already opened (the decision is at the site that set the gate)"],
  ["members-consent.contract", "resource cons = consentView(`${au.signedIn}|${signedOutTick}`) as shape ConsentView else empty(rev=0, signTitle=\"\", signWhy=\"\", signIn=\"\", make=\"\", later=\"\", micPre=\"\", micOff=\"\", micContinue=\"\", micNotNow=\"\", micOk=\"\")", "source argument: the consent view is asked again when the flag moves"],
  ["members-consent.contract", "resource micperm = micPermission(`${au.signedIn}|${bi.handsFree}|${micTick}|${hpg.visibilityState}|${hpg.hasFocus}`) as shape MicPerm else empty(state=\"granted\", gate=\"\")", "source argument: the microphone permission is asked again when the flag moves"],
  ["members-consent.contract", "send signedRun = signInRun(\"signin\", \"\", \"\")", "starts the sign-in the listener asked for by pressing the sheet's Sign in; not a decision to offer it"],
  ["members-mail.contract", "gate = \"signin\"", "sets the sheet; the line above it reads `offerSignIn` (the account's own answer), pinned by contract-signin-pin.test.ts"],
  ["members-mail.contract", "if au.signedIn", "positive test: shows or does the signed-in thing only when signed in"],
  ["members-mark.contract", "derive mkHot = (th.reducedMotion == false and (bi.state != \"idle\" or au.signedIn == false or mkT - mkSince < 1000 or pg.top.startsWith(\"onboarding\"))) or (th.reducedMotion and mkT - mkSince < 400)", "the mark animates while the flag is false: looks only, opens and says nothing"],
  ["members-notebook.contract", "resource nb = notebookPageShared(nbSel, nbTick, pg.top == \"notebook:\", signedOutTick, au.signedIn) as shape NotebookPageInfo else empty()", "source argument: the notebook page skips its fetch when false"],
  ["members-onboarding.contract", "task obOfferTask when au.signedIn or acs.state == \"signedOut\" key=acs.state", "runs the first-run offer when signed in or known signed out; does nothing for an unreadable key"],
  ["members-settings.contract", "resource acctInfo = settingsAccount(au.signedIn, acctTick) as shape SettingsAccount else empty()", "source argument; the answer itself asks the key state and words an unreadable key (settings.ts)"],
  ["members-settings.contract", "resource cmpt = settingsComputer(au.signedIn, acctTick) as shape SettingsComputer else empty()", "source argument: skips the computer fetch when false"],
  ["members-settings.contract", "resource act = activityPage(sfacts, au.signedIn, actTick, cmpt.state, cmpt.updateAvailable, cmpt.checked, current(stime)) with stime.epochAtZero + performanceNow() as shape ActivityInfo else empty()", "source argument: skips the activity fetch when false"],
  ["members-settings.contract", "resource about = aboutPage(sfacts, au.signedIn) as shape AboutInfo else empty()", "source argument: the about page shows the flag"],
  ["members-settings.contract", "send cmm = computerAct(\"refresh\", au.signedIn)", "source argument: skips the request when false (no key to ask with)"],
  ["members-settings.contract", "send cmm = computerAct(\"wake\", au.signedIn)", "source argument: skips the request when false (no key to ask with)"],
  ["members-settings.contract", "send cmm = computerAct(\"update\", au.signedIn)", "source argument: skips the request when false (no key to ask with)"],
  ["members-settings.contract", "send cmm = computerAct(\"poll\", au.signedIn)", "source argument: skips the request when false (no key to ask with)"],
  ["members-settings.contract", "send cmm = computerAct(\"refresh\", au.signedIn)", "source argument: skips the request when false (no key to ask with)"],
  ["members-takes.contract", "gate = \"signin\"", "sets the sheet; the line above it reads `offerSignIn` (the account's own answer), pinned by contract-signin-pin.test.ts"],
  ["members-takes.contract", "if (op == \"record\" or op == \"rerecord\") and au.signedIn and micperm.gate != \"\"", "positive test: the microphone line before the system prompt, only for a signed-in listener"],
  ["members-takes.contract", "if (op != \"record\" and op != \"rerecord\") or (au.signedIn and micperm.gate == \"\")", "positive test: a take act other than record, or record with the microphone ready"],
  ["members-takes.contract", "task takesLoadTask when au.signedIn key=au.signedIn", "positive test: loads the takes only when signed in"],
  ["onboarding.contract", "button press=signInPasskey testId=\"signin\" appearance=\"none\" width=\"100%\"", "a button's test id, not a state"],
  ["onboarding.contract", "if siv.message != \"\" and not siv.pending and not signRunning and (siv.mode == \"signin\" or (siv.mode == \"reviewer\" and nameLogin) or (siv.mode == \"create\" and not nameLogin))", "the sign-in screen's own mode string, not account state"],
  ["onboarding.contract", "if (ob.step == \"welcome\" or ob.step == \"skipped\") and au.signedIn == false and acs.state != \"locked\" and acs.state != \"unknown\"", "the welcome card for a listener with no key; not shown while the key could not be read"],
  ["onboarding.contract", "if (ob.step == \"welcome\" or ob.step == \"skipped\") and au.signedIn", "positive test: the signed-in welcome card"],
  ["paper.contract", "if pfront.viewId == \"front\" and au.signedIn", "positive test: the first-run card only for a signed-in listener"],
  ["settings.contract", "FactRow(label=\"Signed in\", value=(acs.state == \"rejected\" ? \"No\" : au.signedIn ? \"Yes\" : (acs.state == \"locked\" or acs.state == \"unknown\" ? \"Not known\" : \"No\")), mono=false, testId=\"settings-account-signed\")", "display: already says \"Not known\" for locked and unknown"],
  ["settings.contract", "if au.signedIn", "positive test: shows or does the signed-in thing only when signed in"],
  ["settings.contract", "if au.signedIn", "positive test: shows or does the signed-in thing only when signed in"],
  ["spine.contract", "derive askGate = au.signedIn ? \"\" : \"signin\"", "\"\" only when signed in: the ask and the microphone proceed only for a signed-in listener; the sheet is opened under `offerSignIn` (spine.contract micTap and sendAsk)"],
  ["spine.contract", "gate = \"signin\"", "sets the sheet; the line above it reads `offerSignIn` (the account's own answer), pinned by contract-signin-pin.test.ts"],
  ["spine.contract", "if au.signedIn and askUrl != \"\"", "positive test: a signed-in listener's send"],
  ["spine.contract", "if au.signedIn and askUrl == \"\" and canAsk", "positive test: a signed-in listener's send"],
];

const found = (): [string, string][] => {
  const out: [string, string][] = [];
  for (const f of readdirSync(import.meta.dir).filter((n) => n.endsWith('.contract') && !n.endsWith('.test.contract')).sort()) {
    for (const line of readFileSync(`${import.meta.dir}/${f}`, 'utf8').split('\n')) {
      const t = line.trim();
      if (t.startsWith('//')) continue;
      if (/au\.signedIn|"signin"/.test(t)) out.push([f, t]);
    }
  }
  return out;
};

describe('every contract line that reads the signed-in flag or sets the sign-in gate is reviewed', () => {
  test('the lines are exactly the pinned list', () => {
    expect(found().map(([f, t]) => `${f}\t${t}`)).toEqual(PINNED.map(([f, t]) => `${f}\t${t}`));
  });
  test('each pinned line has a reason', () => {
    for (const [f, t, why] of PINNED) expect([f, t, why.length > 15]).toEqual([f, t, true]);
  });
});

describe('the sheet is opened only under offerSignIn', () => {
  const acct = readFileSync(`${import.meta.dir}/account.contract`, 'utf8');
  test('the one derive reads the account answer\'s action', () => {
    expect(acct).toContain('derive offerSignIn = acs.action == "signIn"');
  });
  test('every line that sets gate = "signin" has offerSignIn in the condition just above it', () => {
    for (const f of readdirSync(import.meta.dir).filter((n) => n.endsWith('.contract') && !n.endsWith('.test.contract'))) {
      const lines = readFileSync(`${import.meta.dir}/${f}`, 'utf8').split('\n');
      lines.forEach((l, i) => { if (l.trim() === 'gate = "signin"') expect([f, i, /\boffer(SignIn)\b/.test(lines[i - 1])]).toEqual([f, i, true]); });
    }
  });
  test('the microphone button opens a gate only under offerSignIn', () => {
    expect(readFileSync(`${import.meta.dir}/spine.contract`, 'utf8')).toContain('if micOn == false and askGate != "" and offerSignIn');
  });
  test('the first-run welcome card is not shown for a key that could not be read', () => {
    expect(readFileSync(`${import.meta.dir}/onboarding.contract`, 'utf8')).toContain('and au.signedIn == false and acs.state != \"locked\" and acs.state != \"unknown\"');
  });
});
