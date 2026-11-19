// The shared reader for stories (reader.ts): `pub:` pages read by a host with no native reader (Android). Failure list, written first:
//  R1 a story's text becomes blocks (headings, paragraphs, bullets, quotes) with one text run each, ids stable, the title from the publication, status ready.
//  R2 a failed or empty resolve is an error doc with words, not a spinner; the key is never sent (the route is public).   R3 an address that is not a story is "loading" (another reader owns it).
//  R4 plain text (the share text) has the blocks' text; 'textBlocks' counts the paragraphs.   R5 no code, rules or embeds from the markdown reach the page as raw markup.
import { beforeEach, describe, expect, test } from 'bun:test';
import { readerDocShared } from './reader.ts';

let handler: (url: string) => Response;
beforeEach(() => { handler = () => new Response('{}', { status: 404 }); globalThis.fetch = (async (u: string) => handler(String(u))) as never; });
const story = (text: string, title = 'Harbor barrier approved') => new Response(JSON.stringify({ publication: { title }, payload: { text } }), { status: 200 });

describe('shared reader', () => {
  test('R1 a story becomes blocks', async () => {
    handler = () => story('# Harbor barrier approved\n\nThe council voted seven to two.\n\n- Work starts in spring.\n- Fees rise.\n\n> "A long time coming."\n\nLast paragraph.');
    const d = await readerDocShared('pub:/pub/texture/harbor-pub1');
    expect(d.status).toBe('ready'); expect(d.title).toBe('Harbor barrier approved'); expect(d.address).toBe('pub:/pub/texture/harbor-pub1');
    expect(d.blocks.map((b) => b.kind)).toEqual(['h', 'p', 'li', 'li', 'quote', 'p']);
    expect(d.blocks[1].runs[0].text).toBe('The council voted seven to two.'); expect(d.blocks[2].runs[0].text).toBe('Work starts in spring.'); expect(d.blocks[0].level).toBe(1);
    expect(new Set(d.blocks.map((b) => b.id)).size).toBe(6);
  });
  test('R2 a failed resolve says so; the request carries no key', async () => {
    let auth: string | null = null;
    globalThis.fetch = (async (_u: string, init: RequestInit) => { auth = (init.headers as Record<string, string> | undefined)?.Authorization ?? null; return new Response('{}', { status: 500 }); }) as never;
    const d = await readerDocShared('pub:/pub/texture/x');
    expect(d.status).toBe('error'); expect(d.message).not.toBe(''); expect(auth).toBeNull();
    handler = () => story('   ');
    expect((await readerDocShared('pub:/pub/texture/y')).status).toBe('error');
  });
  test('R3 an address that is not a story is loading', async () => {
    for (const a of ['texture:abc', 'paper:front', '']) expect((await readerDocShared(a)).status).toBe('loading');
  });
  test('R4 plain text and the paragraph count', async () => {
    handler = () => story('One.\n\nTwo.\n\nThree.');
    const d = await readerDocShared('pub:/pub/texture/z');
    expect(d.plain).toBe('One.\n\nTwo.\n\nThree.'); expect(d.textBlocks).toBe(3);
  });
  test('R5 code fences and rules are not shown as markup', async () => {
    handler = () => story('Before.\n\n```js\nalert(1)\n```\n\n---\n\nAfter.');
    const d = await readerDocShared('pub:/pub/texture/c');
    expect(d.blocks.map((b) => b.kind)).toEqual(['p', 'p']); expect(JSON.stringify(d)).not.toContain('alert');
  });
});
