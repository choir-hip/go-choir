// Deck parsing for Slides: PDF pages, PPTX slides and HTML sections become one
// list of slide objects. Ported unchanged from the original SlidesApp.

let pdfjsLibPromise = null;

export function fileExtension(path) {
  const ext = String(path || '').toLowerCase().split('.').pop();
  return ext === String(path || '').toLowerCase() ? '' : ext;
}

async function loadPdfjs() {
  if (!pdfjsLibPromise) {
    pdfjsLibPromise = Promise.all([
      import('pdfjs-dist'),
      import('pdfjs-dist/build/pdf.worker.mjs?url'),
    ]).then(([pdfjs, worker]) => {
      pdfjs.GlobalWorkerOptions.workerSrc = worker.default;
      return pdfjs;
    });
  }
  return pdfjsLibPromise;
}

// ---- PDF parsing: each page becomes a slide (canvas image) ----
export async function parsePdf(arrayBuffer) {
  const pdfjsLib = await loadPdfjs();
  const loadingTask = pdfjsLib.getDocument({ data: arrayBuffer });
  const doc = await loadingTask.promise;
  const result = [];
  const scale = 2;
  for (let i = 1; i <= doc.numPages; i++) {
    const page = await doc.getPage(i);
    const viewport = page.getViewport({ scale });
    const canvas = document.createElement('canvas');
    canvas.width = Math.floor(viewport.width);
    canvas.height = Math.floor(viewport.height);
    const ctx = canvas.getContext('2d');
    await page.render({ canvasContext: ctx, viewport }).promise;
    result.push({
      type: 'image',
      src: canvas.toDataURL('image/png'),
      width: viewport.width,
      height: viewport.height,
    });
  }
  return result;
}

// ---- PPTX parsing: extract slides from OOXML ----
export async function parsePptx(arrayBuffer) {
  const JSZip = (await import('jszip')).default;
  const zip = await JSZip.loadAsync(arrayBuffer);
  const slideFiles = Object.keys(zip.files)
    .filter((name) => /^ppt\/slides\/slide\d+\.xml$/.test(name))
    .sort((a, b) => {
      const na = parseInt(a.match(/slide(\d+)/)[1], 10);
      const nb = parseInt(b.match(/slide(\d+)/)[1], 10);
      return na - nb;
    });

  // Load rels for each slide to resolve image references
  const result = [];
  for (const slidePath of slideFiles) {
    const slideXml = await zip.file(slidePath).async('string');
    const slideNum = slidePath.match(/slide(\d+)/)[1];
    const relsPath = `ppt/slides/_rels/slide${slideNum}.xml.rels`;
    const relsFile = zip.file(relsPath);
    let relsMap = {};
    if (relsFile) {
      const relsXml = await relsFile.async('string');
      relsMap = parseRels(relsXml);
    }
    result.push(parseSlideXml(slideXml, relsMap, zip));
  }
  return result;
}

function parseRels(relsXml) {
  const map = {};
  const parser = new DOMParser();
  const doc = parser.parseFromString(relsXml, 'application/xml');
  const relationships = doc.getElementsByTagName('Relationship');
  for (let i = 0; i < relationships.length; i++) {
    const rel = relationships[i];
    const id = rel.getAttribute('Id');
    const target = rel.getAttribute('Target');
    if (id && target) map[id] = target;
  }
  return map;
}

function parseSlideXml(slideXml, relsMap, zip) {
  const parser = new DOMParser();
  const doc = parser.parseFromString(slideXml, 'application/xml');
  const texts = [];
  const images = [];

  // Extract text runs
  const textElements = doc.getElementsByTagName('a:t');
  for (let i = 0; i < textElements.length; i++) {
    const text = textElements[i].textContent || '';
    if (text.trim()) texts.push(text.trim());
  }

  // Extract images
  const blipElements = doc.getElementsByTagName('a:blip');
  for (let i = 0; i < blipElements.length; i++) {
    const blip = blipElements[i];
    const embed = blip.getAttribute('r:embed');
    if (embed && relsMap[embed]) {
      const imagePath = relsMap[embed].startsWith('/') ?
        relsMap[embed].slice(1) :
        'ppt/slides/' + relsMap[embed];
      const imgFile = zip.file(imagePath);
      if (imgFile) {
        const blob = imgFile.async('blob');
        images.push(blob);
      }
    }
  }

  return {
    type: 'pptx',
    texts,
    images: images,
    title: texts[0] || `Slide`,
  };
}

// ---- HTML parsing: split by <section>, <hr>, or heading boundaries ----
export async function parseHtml(text) {
  const parser = new DOMParser();
  const doc = parser.parseFromString(text, 'text/html');
  const body = doc.body;

  // Try <section> elements first
  let sections = body.querySelectorAll('section');
  if (sections.length > 0) {
    return Array.from(sections).map((section) => ({
      type: 'html',
      html: section.innerHTML,
      title: section.querySelector('h1, h2, h3')?.textContent || `Slide`,
    }));
  }

  // Try splitting by <hr> separators
  const hrSplit = [];
  let current = document.createElement('div');
  for (const node of Array.from(body.childNodes)) {
    if (node.nodeName === 'HR') {
      if (current.innerHTML.trim()) {
        hrSplit.push({
          type: 'html',
          html: current.innerHTML,
          title: current.querySelector('h1, h2, h3')?.textContent || `Slide`,
        });
      }
      current = document.createElement('div');
    } else {
      current.appendChild(node.cloneNode(true));
    }
  }
  if (current.innerHTML.trim()) {
    hrSplit.push({
      type: 'html',
      html: current.innerHTML,
      title: current.querySelector('h1, h2, h3')?.textContent || `Slide`,
    });
  }
  if (hrSplit.length > 0) return hrSplit;

  // Try splitting by h1/h2 headings
  const headings = body.querySelectorAll('h1, h2');
  if (headings.length > 1) {
    const headingSplit = [];
    for (let i = 0; i < headings.length; i++) {
      const start = headings[i];
      const end = i + 1 < headings.length ? headings[i + 1] : null;
      const fragment = document.createElement('div');
      let sibling = start;
      while (sibling && sibling !== end) {
        fragment.appendChild(sibling.cloneNode(true));
        sibling = sibling.nextElementSibling;
      }
      headingSplit.push({
        type: 'html',
        html: fragment.innerHTML,
        title: start.textContent || `Slide`,
      });
    }
    return headingSplit;
  }

  // Fallback: whole document as one slide
  return [{
    type: 'html',
    html: body.innerHTML,
    title: body.querySelector('h1, h2, h3, title')?.textContent || 'Slide',
  }];
}
