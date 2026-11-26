import { describe, expect, test } from "bun:test";
import { storyParagraphs, storyWords } from "./story-text";

// Failure list before the implementation:
//  W1 A story's title and text are found in a bundle whose other fields are large.
//  W2 A key named in the text of another field does not capture the search.
//  W3 Escapes in the title and text decode as JSON does.
//  W4 The artifact's content stands in when payload.text is absent.
//  W5 Markdown is read as plain paragraphs.
//  W6 A body without a story is refused.

const bundle = (extra: Record<string, unknown> = {}, payload: Record<string, unknown> = {}) => JSON.stringify({
  route: { path: "/pub/texture/harbor" },
  publication: { id: "p", title: "Harbor barrier", slug: "harbor" },
  version: { revision: 2 },
  artifact: { manifest_id: "m", content: "Title\n\nThe council voted.\n\nA second paragraph." },
  payload: { revision: 2, text: "Title\n\nThe council voted.\n\nA second paragraph.", manifest: { body_doc: { text: "not the story" } } },
  ...extra,
  ...(Object.keys(payload).length ? { payload: { ...payload } } : {}),
});

describe("story-text", () => {
  test("W1 title and text come from a large bundle", () => {
    const raw = JSON.stringify({ ...JSON.parse(bundle()), filler: Array.from({ length: 20000 }, (_, i) => `filler ${i}`) });
    expect(raw.length).toBeGreaterThan(200_000);
    expect(storyWords(raw)).toEqual({ title: "Harbor barrier", paragraphs: ["Title", "The council voted.", "A second paragraph."] });
  });

  test("W2 text that names a key does not capture the scan", () => {
    const body = JSON.parse(bundle());
    body.filler = ['"payload":{"text":"wrong"}', '"publication":{"title":"wrong"}'];
    expect(storyWords(JSON.stringify(body))?.title).toBe("Harbor barrier");
    expect(storyWords(JSON.stringify(body))?.paragraphs[1]).toBe("The council voted.");
  });

  test("W3 escapes decode as JSON does", () => {
    const raw = JSON.stringify({ publication: { id: 'p', title: 'A "quoted" — title' }, payload: { revision: 1, text: 'Line with \\ backslash\n\nUnicode héllo 🙂' } });
    const words = storyWords(raw);
    expect(words?.title).toBe('A "quoted" — title');
    expect(words?.paragraphs).toEqual(["Line with \\ backslash", "Unicode héllo 🙂"]);
  });

  test("W4 the artifact's content stands in", () => {
    const raw = JSON.stringify({ publication: { id: "p", title: "T" }, artifact: { manifest_id: "m", content: "One.\n\nTwo." }, payload: { revision: 1 } });
    expect(storyWords(raw)?.paragraphs).toEqual(["One.", "Two."]);
  });

  test("W5 markdown reads as plain paragraphs", () => {
    expect(storyParagraphs("# Heading\n\nThe **council** voted. See [the record](https://example.test/x) and ![a photo](x.png).\n\n> A quote.")).toEqual([
      "Heading", "The council voted. See the record and .", "A quote.",
    ]);
  });

  test("W6 a body without a story is refused", () => {
    expect(storyWords(JSON.stringify({ nav: { revision: 1 } }))).toBeUndefined();
    expect(storyWords("not json")).toBeUndefined();
  });
});
