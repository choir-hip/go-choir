// The words of the sign-in prompt shown when a signed-out listener asks, in one place. The same strings are in strings/en.json (keys onboarding.askSignIn.*); consent.test.ts fails when the two differ, so a change is
// made in both. (There is no consent sheet before the radio answers: the owner ruled it out on 2026-11-23; the agreement to outside AI companies is the create-account screen's line.)
export const ASK_TEXT = {
  'onboarding.askSignIn.title': 'Sign in to ask the radio',
  'onboarding.askSignIn.why': 'Listening needs no account; asking does, so the radio has a computer of yours to keep your questions on.',
  'onboarding.askSignIn.signIn': 'Sign in',
  'onboarding.askSignIn.make': 'Make an account',
  'onboarding.askSignIn.later': 'Not now',
} as const;
export type AskText = { signTitle: string; signWhy: string; signIn: string; make: string; later: string };
export const askText = (): AskText => ({
  signTitle: ASK_TEXT['onboarding.askSignIn.title'], signWhy: ASK_TEXT['onboarding.askSignIn.why'], signIn: ASK_TEXT['onboarding.askSignIn.signIn'],
  make: ASK_TEXT['onboarding.askSignIn.make'], later: ASK_TEXT['onboarding.askSignIn.later'],
});
