import App from './App.svelte';
import './app.css';
import './lib/media-app.css';
import { exposeBuildInfo } from './lib/build-info.js';

exposeBuildInfo();

const app = new App({
  target: document.getElementById('app'),
});

export default app;
