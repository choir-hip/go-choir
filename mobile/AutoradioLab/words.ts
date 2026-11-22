// The app's words about the account (no imports: every page model reads them). One list; the card, Settings and the page models use it. Spelling: "programme", as everywhere in the app.
export const WORDS = {
  signedOut: 'You are signed out. Sign in to use your own computer.',
  radioSignedOut: 'This is the public paper. Sign in to hear your own programme.',
  noSignIn: 'This is the public paper. Signing in is not available on this device yet.',
  locked: 'Unlock your phone to continue.',
  unknown: 'Could not check your sign-in just now. Try again in a moment.',
  rejected: "This phone's sign-in is no longer accepted. Sign out to sign in again.",
  // The ended card: a sign-out ends the session, and the person closes the app and opens it again (a fresh process holds no trace of the account).
  endedSignedOut: 'Signed out. Close Autoradio and open it again.',
  endedUnfinished: 'Sign-out did not finish. Close Autoradio, open it again and try again.',
  starting: 'Starting Autoradio. If this stays, unlock your phone.',
  endedBlocked: 'Autoradio cannot read its sign-in on this phone. Unlock the phone and open Autoradio again.',
  stuck: 'If this does not clear, close and reopen the app. Nothing on your computer is affected.',
};
