import JSZip from 'jszip';

const encoder = new TextEncoder();

function byteLength(value) {
  return encoder.encode(value).length;
}

export function buildPdfBytes(text = 'Choir PDF reader proof') {
  const safeText = String(text).replace(/[()\\]/g, (match) => `\\${match}`);
  const stream = `BT /F1 24 Tf 40 180 Td (${safeText}) Tj ET`;
  const objects = [
    '<< /Type /Catalog /Pages 2 0 R >>',
    '<< /Type /Pages /Kids [3 0 R] /Count 1 >>',
    '<< /Type /Page /Parent 2 0 R /MediaBox [0 0 420 260] /Resources << /Font << /F1 4 0 R >> >> /Contents 5 0 R >>',
    '<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>',
    `<< /Length ${byteLength(stream)} >>\nstream\n${stream}\nendstream`,
  ];

  let pdf = '%PDF-1.4\n';
  const offsets = [0];
  for (let index = 0; index < objects.length; index++) {
    offsets.push(byteLength(pdf));
    pdf += `${index + 1} 0 obj\n${objects[index]}\nendobj\n`;
  }

  const xrefOffset = byteLength(pdf);
  pdf += `xref\n0 ${objects.length + 1}\n`;
  pdf += '0000000000 65535 f \n';
  for (let index = 1; index < offsets.length; index++) {
    pdf += `${String(offsets[index]).padStart(10, '0')} 00000 n \n`;
  }
  pdf += `trailer\n<< /Size ${objects.length + 1} /Root 1 0 R >>\nstartxref\n${xrefOffset}\n%%EOF\n`;

  return encoder.encode(pdf);
}

export async function buildDocxBytes({
  paragraphs = ['Choir DOCX import proof'],
  table = [
    ['Term', 'Definition'],
    ['Work product', 'Durable professional output'],
  ],
} = {}) {
  const zip = new JSZip();
  const escapeXML = (value) => String(value)
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&apos;');

  const paragraphXML = paragraphs.map((paragraph) =>
    `<w:p><w:r><w:t>${escapeXML(paragraph)}</w:t></w:r></w:p>`
  ).join('');
  const tableXML = table.length
    ? `<w:tbl>${table.map((row) =>
      `<w:tr>${row.map((cell) =>
        `<w:tc><w:p><w:r><w:t>${escapeXML(cell)}</w:t></w:r></w:p></w:tc>`
      ).join('')}</w:tr>`
    ).join('')}</w:tbl>`
    : '';

  zip.file(
    '[Content_Types].xml',
    `<?xml version="1.0" encoding="UTF-8"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="xml" ContentType="application/xml"/>
</Types>`
  );
  zip.file(
    'word/document.xml',
    `<?xml version="1.0" encoding="UTF-8"?>
<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:body>${paragraphXML}${tableXML}</w:body>
</w:document>`
  );

  return zip.generateAsync({
    type: 'uint8array',
    mimeType: 'application/vnd.openxmlformats-officedocument.wordprocessingml.document',
    compression: 'DEFLATE',
  });
}

export async function buildEpubBytes({
  title = 'Choir EPUB Reader Proof',
  chapters = [
    {
      title: 'Opening Chapter',
      body: [
        'The real EPUB reader opens archive files, reads the spine, and renders chapter text.',
        'This chapter includes searchable proof text for Playwright.',
      ],
    },
    {
      title: 'Second Chapter',
      body: [
        'The second chapter proves table of contents navigation and recipient reader state.',
      ],
    },
  ],
} = {}) {
  const zip = new JSZip();
  zip.file('mimetype', 'application/epub+zip', { compression: 'STORE' });
  zip.file(
    'META-INF/container.xml',
    `<?xml version="1.0" encoding="UTF-8"?>
<container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container">
  <rootfiles>
    <rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/>
  </rootfiles>
</container>`
  );

  const manifestItems = chapters.map((_, index) =>
    `<item id="chapter-${index + 1}" href="chapter-${index + 1}.xhtml" media-type="application/xhtml+xml"/>`
  ).join('\n    ');
  const spineItems = chapters.map((_, index) => `<itemref idref="chapter-${index + 1}"/>`).join('\n    ');

  zip.file(
    'OEBPS/content.opf',
    `<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0" unique-identifier="book-id">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:identifier id="book-id">choir-epub-proof</dc:identifier>
    <dc:title>${title}</dc:title>
    <dc:language>en</dc:language>
  </metadata>
  <manifest>
    <item id="nav" href="nav.xhtml" media-type="application/xhtml+xml" properties="nav"/>
    ${manifestItems}
  </manifest>
  <spine>
    ${spineItems}
  </spine>
</package>`
  );

  zip.file(
    'OEBPS/nav.xhtml',
    `<?xml version="1.0" encoding="UTF-8"?>
<html xmlns="http://www.w3.org/1999/xhtml">
  <head><title>${title}</title></head>
  <body>
    <nav epub:type="toc">
      <ol>
        ${chapters.map((chapter, index) => `<li><a href="chapter-${index + 1}.xhtml">${chapter.title}</a></li>`).join('\n        ')}
      </ol>
    </nav>
  </body>
</html>`
  );

  chapters.forEach((chapter, index) => {
    zip.file(
      `OEBPS/chapter-${index + 1}.xhtml`,
      `<?xml version="1.0" encoding="UTF-8"?>
<html xmlns="http://www.w3.org/1999/xhtml">
  <head><title>${chapter.title}</title></head>
  <body>
    <h1>${chapter.title}</h1>
    ${chapter.body.map((paragraph) => `<p>${paragraph}</p>`).join('\n    ')}
  </body>
</html>`
    );
  });

  return zip.generateAsync({
    type: 'uint8array',
    mimeType: 'application/epub+zip',
    compression: 'DEFLATE',
  });
}

export async function putBinaryFile(page, name, contentType, bytes) {
  await page.evaluate(async ({ name, contentType, values }) => {
    const res = await fetch('/api/files/' + encodeURIComponent(name), {
      method: 'PUT',
      credentials: 'include',
      headers: { 'Content-Type': contentType },
      body: new Uint8Array(values),
    });
    if (!res.ok) {
      const body = await res.text();
      throw new Error(`failed to seed ${name}: ${res.status} ${body}`);
    }
  }, { name, contentType, values: Array.from(bytes) });
}
