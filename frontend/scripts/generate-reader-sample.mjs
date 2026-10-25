#!/usr/bin/env node
// Writes the Reader preview's two sample documents:
//   public/read/sample-document.pdf   a real PDF, from sample-pdf.js
//   public/read/sample-document.html  the same sample as plain HTML, which the
//                                     preview page reads if the PDF cannot be
//                                     (or, in a browser without deflate, when
//                                     the PDF text cannot be extracted)
//
// The in-app sample (sampleModel) is the one source of the text; the PDF has
// its own short version because it must be written page by page.
import { writeFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { buildPdf, samplePdfPhrases } from '../src/packages/reader/sample-pdf.js';
import { sampleModel } from '../src/packages/reader/reader-prefs.js';

const here = dirname(fileURLToPath(import.meta.url));
const readDir = join(here, '..', 'public', 'read');

const escapeHtml = (text) =>
  String(text).split('&').join('&amp;').split('<').join('&lt;').split('>').join('&gt;');

const tagFor = (block) => {
  if (block.tag === 'h1' || block.tag === 'h2' || block.tag === 'h3' || block.tag === 'h4') return 'h2';
  if (block.tag === 'blockquote') return 'blockquote';
  if (block.tag === 'pre') return 'pre';
  return 'p';
};

const body = sampleModel.sections
  .map((section, index) => [
    // The sample model is one section whose title is the document title, so
    // this file carried the same heading twice. The first section's title
    // opens the document as a paragraph, and every other section keeps its
    // heading: a titled section that opens the document is not a second title.
    (index === 0 ? '<p>' : '<h2>') + escapeHtml(section.title) + (index === 0 ? '</p>' : '</h2>'),
  ].concat(
    section.blocks.map((block) => '<' + tagFor(block) + '>' + escapeHtml(block.text) + '</' + tagFor(block) + '>'),
  ).join('\n'))
  .join('\n');


const html = [
  '<!DOCTYPE html>',
  '<html lang="en">',
  '  <head>',
  '    <meta charset="UTF-8" />',
  '    <title>Reader sample – Choir</title>',
  '    <meta name="description" content="The Reader sample document, as plain HTML: the preview page reads it when the sample PDF cannot be read." />',
  '  </head>',
  '  <body>',
  // sampleModel is one section whose title is the document title, so the body
  // already opens with that title as its heading: an <h1> above it printed the
  // title twice, and a number in the first block turned the first line into a
  // list item when the preview read this file as plain text.
  '    <article data-reader-sample-document>',
  body,
  '    </article>',
  '  </body>',
  '</html>',
  '',
].join('\n');

writeFileSync(join(readDir, 'sample-document.html'), html);
const pdf = buildPdf(samplePdfPhrases(), { title: 'Reader sample' });
// latin1 keeps every byte: the PDF header carries bytes above 127 in its
// binary marker comment.
writeFileSync(join(readDir, 'sample-document.pdf'), Buffer.from(pdf, 'latin1'));
console.log('reader-sample: sample-document.html ' + html.length + ' B, sample-document.pdf ' + pdf.length + ' B');
