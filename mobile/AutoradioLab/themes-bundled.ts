// The bundled themes: mobile/themes/*.json, imported as they are (the symlink `themes` beside this file, as `fixtures` is). They were embedded into Swift by scripts/embed-assets.py
// (BundledThemes.generated.swift); that output is gone and the generator no longer writes it. A new theme file needs one line here; themes.test.ts fails when a file in the folder is not listed.
import chromePastel from './themes/chrome-pastel.json';
import depth from './themes/depth.json';
import lantern from './themes/lantern.json';
import liquidMetal from './themes/liquid-metal.json';
import newsprint from './themes/newsprint.json';
import signal from './themes/signal.json';

/** File name (no extension) to the theme document. Sorted by name, as the Swift loader read them. */
export const BUNDLED: Record<string, unknown> = {
  'chrome-pastel': chromePastel, depth, lantern, 'liquid-metal': liquidMetal, newsprint, signal,
};
