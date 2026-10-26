// The Reader preview page: /read.html.
//
// A sign-in-free surface that proves the Reader where nothing else can reach
// it: in a visual capture, offline, or before an account exists. It mounts the
// Reader's own window component with authenticated={false}: the same toolbar,
// the same reading surface and the same sample document the desktop app shows,
// so the preview cannot drift from the app. A signed-out window opens the
// sample, and its Open file asks the shell to sign in.
//
// The shell applies the theme's --choir-* variables to the desktop root and
// draws its icon; this page has no shell, so it applies the same variables (and
// the page's own chrome, reader-chrome.css, reads them) from the theme module
// and draws the same icon. The Reader package itself never reaches the shell:
// it is handed the variables.
import '../reader-chrome.css';
import { applyThemeToElement, DEFAULT_THEME } from './lib/theme';
import { installTetraMarkFavicon } from './lib/tetramark';
import ReaderApp from './packages/reader/App.svelte';

applyThemeToElement(document.documentElement, DEFAULT_THEME);
installTetraMarkFavicon();

const preview = document.getElementById('reader-root');
const signIn = document.querySelector('[data-reader-signin]');

if (signIn) {
  signIn.addEventListener('click', (event) => {
    event.preventDefault();
    window.location.assign('/');
  });
}

const app = new ReaderApp({
  target: preview,
  props: { authenticated: false },
});

// Open file (and a dropped file) on the preview has no session behind it, so
// the Reader asks the shell for one. This page has no shell: sign-in lives on
// the desktop, so the request goes there.
app.$on('authrequired', () => {
  window.location.assign('/');
});

export default app;
