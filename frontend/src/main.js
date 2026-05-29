import App from './App.svelte';
import './app.css';
import { exposeBuildInfo } from './lib/build-info.js';
import { installTetraMarkFavicon } from './lib/tetramark';

exposeBuildInfo();
installTetraMarkFavicon();

const app = new App({
  target: document.getElementById('app'),
});

export default app;
