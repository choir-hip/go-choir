export const TETRA_MARK_VIEWBOX = '0 0 512 512';

export const TETRA_MARK_PATHS = [
  'M 269.72 36.86 L 258.57 63.16 L 250.28 143.79 L 251.71 206.97 L 262.58 236.99 L 275.44 235.84 L 288.88 195.25 L 291.17 109.19 L 308.03 105.48 L 376.93 226.41 L 424.39 279.87 L 449.84 324.47 L 475.85 327.05 L 437.54 244.99 L 360.35 138.07 L 328.62 77.17 L 291.17 34.86 Z',
  'M 174.23 148.93 L 135.07 175.52 L 79.60 281.02 L 54.73 361.07 L 35.29 396.80 L 37.29 425.97 L 53.87 437.97 L 181.38 387.94 L 218.55 355.63 L 217.98 342.20 L 206.25 339.62 L 146.50 361.07 L 92.75 395.09 L 77.60 394.80 L 79.32 373.36 L 175.66 166.09 Z',
  'M 476.43 455.41 L 367.21 359.64 L 324.90 339.34 L 326.33 360.21 L 389.23 420.82 L 388.66 435.69 L 199.68 442.83 L 177.38 455.98 L 175.38 466.56 L 185.10 477.14 L 304.89 473.14 L 457.56 477.14 L 473.28 469.42 Z',
];

function escapeSvg(value: string): string {
  return value.replace(/&/g, '&amp;').replace(/"/g, '&quot;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
}

export function tetraMarkFaviconSvg(background = '#050912', fill = '#93B2FF'): string {
  const paths = TETRA_MARK_PATHS.map((path) => `<path d="${escapeSvg(path)}" fill="${fill}"/>`).join('');
  return `<svg xmlns="http://www.w3.org/2000/svg" viewBox="${TETRA_MARK_VIEWBOX}"><rect width="512" height="512" rx="112" fill="${background}"/><g>${paths}</g></svg>`;
}

export function tetraMarkFaviconHref(background = '#050912', fill = '#93B2FF'): string {
  return `data:image/svg+xml,${encodeURIComponent(tetraMarkFaviconSvg(background, fill))}`;
}

export function installTetraMarkFavicon(): void {
  if (typeof document === 'undefined') return;
  let link = document.querySelector<HTMLLinkElement>('link[rel="icon"][data-tetramark-favicon]');
  if (!link) {
    link = document.createElement('link');
    link.rel = 'icon';
    link.setAttribute('data-tetramark-favicon', '');
    document.head.append(link);
  }
  link.href = tetraMarkFaviconHref();
}
