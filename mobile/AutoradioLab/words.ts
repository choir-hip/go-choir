// The app's words about the account (no imports: every page model reads them). One list; the banner, Settings and the page models use it. Spelling: "programme", as everywhere in the app.
export const WORDS = {
  signedOut: 'You are signed out. Sign in to use your own computer.',
  radioSignedOut: 'This is the public paper. Sign in to hear your own programme.',
  noSignIn: 'This is the public paper. Signing in is not available on this device yet.',
  locked: 'Unlock your phone to continue.',
  unknown: 'Could not check your sign-in just now. Try again in a moment.',
  rejected: "This phone's sign-in is no longer accepted. Clear this phone to sign in again.",
  clearing: "Clearing the previous account's data from this phone.",
  adopt: 'This phone could not record the account. If this stays, close and reopen the app.',
  // After five tries a refusing state names what failed and the one way out that always exists (a reinstall removes the app's container; the Keychain survives it, so the phone comes back with empty stores and the same account).
  stuck: 'If this does not clear, close and reopen the app. If it still does not, delete and reinstall it. Nothing on your computer is affected.',
};
