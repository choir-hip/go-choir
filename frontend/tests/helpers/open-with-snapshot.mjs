// The Open-with answers of the shell for every file extension it knows, as one
// JSON-able object: the differential of slice S3 compares this before and
// after the file-route table became one (tests/fixtures/open-with-golden.json
// was written from the code before that change).
export function openWithSnapshot(mods) {
  const { openersForFile, extensionsOpenedBy, canOpenWith } = mods.openWith;
  const { mediaRouteForFileName, MEDIA_FILE_ROUTES } = mods.media;
  const { PACKAGE_MEDIA_ROUTES } = mods.registry;
  const exts = new Set(['txt', 'md', 'png', 'jpg', 'jpeg', 'gif', 'webp', 'svg', 'bin', 'xyz', 'json', 'go', 'PDF', 'Mp3']);
  const types = new Set();
  for (const r of [...MEDIA_FILE_ROUTES, ...PACKAGE_MEDIA_ROUTES]) {
    r.extensions.forEach((e) => exts.add(e));
    types.add(r.mediaType);
  }
  const apps = new Set();
  const out = { byExtension: {}, byMediaType: {}, route: {}, extensionsOpenedBy: {}, canOpenWith: {} };
  for (const e of [...exts].sort()) {
    const name = `file.${e}`;
    out.byExtension[e] = openersForFile(name);
    out.route[e] = mediaRouteForFileName(name);
    out.byExtension[e].forEach((o) => apps.add(o.appId));
  }
  out.byExtension[''] = openersForFile('README');
  for (const t of [...types].sort()) out.byMediaType[t] = openersForFile('blob', { mediaType: t });
  for (const a of [...apps, 'image', 'audio', 'video', 'pdf', 'epub', 'slides', 'calendar', 'reader', 'files'].sort()) {
    out.extensionsOpenedBy[a] = extensionsOpenedBy(a).sort();
    out.canOpenWith[a] = canOpenWith(a, 'deck.pdf');
  }
  out.mediaFileRoutes = MEDIA_FILE_ROUTES;
  out.packageRoutes = PACKAGE_MEDIA_ROUTES;
  return out;
}
