import App from './App.svelte';
import './app.css';
import './lib/desktop-motion.css';
import { exposeBuildInfo } from './lib/build-info.js';
import { installTetraMarkFavicon } from './lib/tetramark';

import { installIdleWatcher } from './lib/idle';

exposeBuildInfo();
installIdleWatcher();
installTetraMarkFavicon();

const app = new App({
  target: document.getElementById('app'),
});

export default app;
