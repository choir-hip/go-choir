export const BUILD_INFO = Object.freeze({
  service: 'frontend',
  version: __CHOIR_BUILD_VERSION__,
  commit: __CHOIR_BUILD_COMMIT__,
  built_at: __CHOIR_BUILD_TIME__,
});

export function exposeBuildInfo() {
  // Deploy-impact proof: frontend-only changes must not rebuild guest images.
  // Frontend pointer proof: this bundle should deploy without host activation.
  window.__CHOIR_BUILD__ = BUILD_INFO;
  document.documentElement.dataset.choirBuildCommit = BUILD_INFO.commit;
  document.documentElement.dataset.choirBuildVersion = BUILD_INFO.version;
  const s2m = document.createElement('meta');
  s2m.name = 's2-frontend-marker';
  s2m.content = 'built-spa:' + BUILD_INFO.commit;
  document.head.appendChild(s2m);
}
