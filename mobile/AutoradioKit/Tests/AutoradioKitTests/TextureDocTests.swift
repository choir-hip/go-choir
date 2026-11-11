import XCTest
@testable import AutoradioKit

// Tests for the reader's document model. The failure lists are in TextureDoc.swift, Citations.swift and
// TextureReaderSource.swift; each test names the item it covers.
final class TextureDocTests: XCTestCase {
    func obj(_ s: String) -> [String: Any] { (try! JSONSerialization.jsonObject(with: Data(s.utf8))) as! [String: Any] }

    func bodyDoc(_ content: String) -> String { #"{"schema":"choir.texture_doc.v1","doc":{"type":"doc","content":[\#(content)]}}"# }

    func bundle(body: String, entities: String = "[]", trans: String = "[]", checks: String = "[]", resolved: String = "[]",
                nav: String = #"{"revision":2,"prev":1,"next":0,"latest":2}"#, text: String = "") -> [String: Any] {
        obj(#"""
        {"publication":{"title":"Headline","kind":"story","dek":"A dek."},
         "route":{"bare":"headline","permalink":"headline/v2"},
         "version":{"published_at":"2026-11-10T06:00:00Z"},
         "artifact":{"content":"\#(text)","body_doc":\#(body)},
         "source_entities":\#(entities),"transclusions":\#(trans),"checks":{"citations":\#(checks)},
         "payload":{"text":"","resolved_blocks":\#(resolved)},"nav":\#(nav)}
        """#)
    }

    // 2, 4: paragraphs, headings, unknown nodes keep text, empties vanish
    func testBlocksParagraphHeadingUnknownAndEmpties() {
        let body = bodyDoc(#"""
        {"type":"heading","attrs":{"id":"h","level":2},"content":[{"type":"text","text":"Why it matters"}]},
        {"type":"paragraph","attrs":{"id":"p1"},"content":[{"type":"text","text":"First "},{"type":"text","text":"bold","marks":[{"type":"strong"}]},{"type":"text","text":" and "},{"type":"text","text":"slanted","marks":[{"type":"emphasis"}]},{"type":"text","text":"."}]},
        {"type":"paragraph","content":[{"type":"hard_break"}]},
        {"type":"paragraph","content":[{"type":"text","text":"   "}]},
        {"type":"callout","content":[{"type":"text","text":"A future block"}]}
        """#)
        let d = TextureDocParser.parseBundle(bundle(body: body), address: "pub:headline")
        XCTAssertEqual(d.status, "ready")
        XCTAssertEqual(d.blocks.map(\.kind), ["h", "p", "p"], "empty paragraphs vanish, the unknown block keeps its text as a paragraph")
        XCTAssertEqual(d.blocks[0].level, 2)
        let runs = d.blocks[1].runs
        XCTAssertEqual(runs.first(where: { $0.text == "bold" })?.weight, 700)
        XCTAssertEqual(runs.first(where: { $0.text == "slanted" })?.slant, "italic")
        XCTAssertEqual(d.blocks[2].text, "A future block")
        XCTAssertEqual(d.kind, "story")
        XCTAssertEqual(d.kicker, "Autopaper")
        XCTAssertTrue(d.disclosure.hasPrefix("Written by AI."))
        XCTAssertEqual(d.dek, "A dek.")
    }

    // 1: no body_doc falls back to markdown; nothing at all gives a visible empty block
    func testFallbackToMarkdownAndEmptyDocument() {
        let o = obj(##"{"publication":{"title":"T"},"artifact":{"content":"# T\n\nOne **two** three.\n\n- a\n- b\n\n1. x\n2. y"},"nav":{}}"##)
        let d = TextureDocParser.parseBundle(o, address: "pub:t")
        XCTAssertEqual(d.blocks.map(\.kind), ["p", "li", "li", "li", "li"], "the heading equal to the title is dropped (13)")
        XCTAssertEqual(d.blocks[1].marker, "\u{2022}")
        XCTAssertEqual(d.blocks[3].marker, "1.")
        XCTAssertEqual(d.blocks[0].runs.first(where: { $0.text == "two" })?.weight, 700)
        let empty = TextureDocParser.parseBundle(obj(#"{"publication":{"title":"T"},"artifact":{"body_doc":"not json"},"payload":{}}"#), address: "pub:t")
        XCTAssertEqual(empty.status, "ready")
        XCTAssertEqual(empty.blocks.count, 1)
        XCTAssertTrue(empty.blocks[0].text.contains("no text yet"))
    }

    // 5, 6, 7: lists, quotes, code
    func testListsQuotesAndCode() {
        let body = bodyDoc(#"""
        {"type":"ordered_list","attrs":{"id":"o","start":3},"content":[
          {"type":"list_item","content":[{"type":"paragraph","content":[{"type":"text","text":"Three"}]},{"type":"bullet_list","content":[{"type":"list_item","content":[{"type":"paragraph","content":[{"type":"text","text":"Nested"}]}]}]}]},
          {"type":"list_item","content":[{"type":"paragraph","content":[{"type":"text","text":"Four"}]},{"type":"paragraph","content":[{"type":"text","text":"More of four"}]}]}]},
        {"type":"blockquote","content":[{"type":"paragraph","content":[{"type":"text","text":"Said one."}]},{"type":"blockquote","content":[{"type":"paragraph","content":[{"type":"text","text":"Said two."}]}]}]},
        {"type":"code_block","content":[{"type":"text","text":"let a = 1\n  let b = 2"}]}
        """#)
        let d = TextureDocParser.parseBundle(bundle(body: body), address: "pub:x")
        XCTAssertEqual(d.blocks.map(\.kind), ["li", "li", "li", "li", "quote", "quote", "code"])
        XCTAssertEqual(d.blocks[0].marker, "3.")
        XCTAssertEqual(d.blocks[1].depth, 2, "nested list is one deeper")
        XCTAssertEqual(d.blocks[2].marker, "4.")
        XCTAssertEqual(d.blocks[3].marker, "", "the second block of an item has no marker")
        XCTAssertEqual(d.blocks[4].depth, 1)
        XCTAssertEqual(d.blocks[5].depth, 2)
        XCTAssertEqual(d.blocks[6].text, "let a = 1\n  let b = 2", "code keeps newlines and indentation")
    }

    // 3, 6: citations, numbering, unique anchors, missing entity, shared entity
    func testCitationsNumberingAndMissingEntity() {
        let body = bodyDoc(#"""
        {"type":"paragraph","content":[{"type":"text","text":"Claim one."},{"type":"source_ref","attrs":{"id":"a1","source_entity_id":"e1"}},{"type":"text","text":" Claim two."},{"type":"source_ref","attrs":{"id":"a2","source_entity_id":"e2"}},{"type":"source_ref","attrs":{"id":"a1","source_entity_id":"e1"}},{"type":"source_ref","attrs":{"source_entity_id":"gone"}}]}
        """#)
        let ents = #"[{"source_entity_id":"e1","entity":{"target":{"kind":"web_url","uri":"https://www.Example.com:8080/a/b"},"display":{"title":"Example story"}}},{"source_entity_id":"e2","entity":{"target":{"kind":"web_url","uri":"https://news.example.org/x"},"display":{"label":"Other"}}}]"#
        let tr = #"[{"anchor_id":"a1","source_entity_id":"e1","snapshot_text":"the exact words","ref":{"cid":"c1","quote":"the exact words"}}]"#
        let checks = #"[{"anchor_id":"a1","state":"EXACT"}]"#
        let resolved = #"[{"cid":"c1","text":"Before it. The Exact  WORDS here. After it.","url":"https://example.com/a/b","title":"Example story"}]"#
        let d = TextureDocParser.parseBundle(bundle(body: body, entities: ents, trans: tr, checks: checks, resolved: resolved), address: "pub:x")
        XCTAssertEqual(d.cites.count, 4)
        XCTAssertEqual(Set(d.cites.map(\.anchor)).count, 4, "duplicate and missing ids are made unique")
        let c1 = d.cites[0]
        XCTAssertEqual(c1.number, 1); XCTAssertEqual(c1.host, "example.com"); XCTAssertEqual(c1.title, "Example story")
        XCTAssertEqual(c1.check, "exact"); XCTAssertEqual(c1.checkLabel, "Exact quote")
        XCTAssertEqual(c1.hit, "The Exact  WORDS", "the highlight is cut from the original text, case and spacing kept")
        XCTAssertEqual(c1.before, "Before it. "); XCTAssertEqual(c1.after, " here. After it.")
        XCTAssertEqual(d.cites[1].number, 2)
        XCTAssertEqual(d.cites[2].number, 1, "a second anchor on one entity shares its number")
        XCTAssertEqual(d.cites[3].title, "Source unavailable")
        XCTAssertEqual(d.sources.map(\.number), [1, 2, 3])
        XCTAssertEqual(d.blocks[0].cites.count, 4)
        XCTAssertEqual(d.blocks[0].runs.filter { $0.kind == "cite" }.map(\.label), ["1", "2", "1", "3"])
    }

    // 7: a Texture transclusion
    func testTextureTransclusion() {
        let body = bodyDoc(#"{"type":"paragraph","content":[{"type":"text","text":"See also"},{"type":"source_ref","attrs":{"id":"t1","source_entity_id":"tx"}}]}"#)
        let ents = #"[{"source_entity_id":"tx","entity":{"target":{"kind":"texture_span","uri":"texture:doc-9@rev-2"},"display":{"title":"Battery explainer"}}}]"#
        let d = TextureDocParser.parseBundle(bundle(body: body, entities: ents, trans: #"[{"anchor_id":"t1","snapshot_text":"Storage shifts the peak."}]"#), address: "pub:x")
        XCTAssertEqual(d.cites[0].kind, "texture")
        XCTAssertEqual(d.cites[0].target, "texture:doc-9@rev-2")
        XCTAssertEqual(d.cites[0].quote, "Storage shifts the peak.")
        XCTAssertEqual(d.blocks[0].runs.last?.kind, "ref")
    }

    // 9: sentences are numbered from the plain text; a citation belongs to the sentence it closes
    func testSentenceIndices() {
        let body = bodyDoc(#"{"type":"paragraph","content":[{"type":"text","text":"It rained on Monday."},{"type":"source_ref","attrs":{"id":"a","source_entity_id":"e"}},{"type":"text","text":" Then it stopped. The end."}]}"#)
        let d = TextureDocParser.parseBundle(bundle(body: body, entities: #"[{"source_entity_id":"e","entity":{"target":{"kind":"web_url","uri":"https://e.example.com"},"display":{"title":"E"}}}]"#), address: "pub:x")
        let b = d.blocks[0]
        XCTAssertEqual(b.sentences, 3)
        XCTAssertEqual(b.runs.first(where: { $0.kind == "cite" })?.sent, 0)
        XCTAssertEqual(b.sentenceTexts().count, 3)
        XCTAssertEqual(b.runs.last?.sent, 2)
        XCTAssertEqual(b.runs.map(\.id).count, Set(b.runs.map(\.id)).count, "run ids are unique")
    }

    // 10, 11: tables and markdown corner cases
    func testMarkdownTablesAndLiteralMarkers() {
        let md = "Intro with snake_case and 5 * 3 and an *unmatched star.\n\n| Name | Count |\n|---|---|\n| Apples | 3 |\n| Pears |\n\n```\nunterminated code\n  kept"
        let blocks = Markdown.blocks(md)
        XCTAssertEqual(blocks.map(\.kind), ["p", "table", "code"])
        XCTAssertEqual(blocks[1].rows.count, 3)
        XCTAssertEqual(blocks[1].rows[2], ["Pears"], "ragged rows are kept as they are")
        XCTAssertTrue(blocks[2].text.hasSuffix("  kept"))
        let runs = Markdown.inline(blocks[0].text, idPrefix: "p")
        XCTAssertEqual(runs.map(\.text).joined(), blocks[0].text, "unmatched markers stay literal")
        XCTAssertTrue(runs.allSatisfy { $0.slant == "normal" })
        let link = Markdown.inline("see [the docs](https://example.com/d) now", idPrefix: "l")
        XCTAssertEqual(link.first(where: { $0.kind == "link" })?.href, "https://example.com/d")
    }

    // 12: versions
    func testVersionsAndNav() {
        let d = TextureDocParser.parseBundle(bundle(body: bodyDoc(#"{"type":"paragraph","content":[{"type":"text","text":"x"}]}"#),
                                                    nav: #"{"revision":2,"prev":1,"next":3,"latest":3}"#), address: "pub:headline/v2")
        XCTAssertEqual(d.revision, 2); XCTAssertEqual(d.latest, 3); XCTAssertTrue(d.newer)
        XCTAssertEqual(d.versions.map(\.number), [3, 2, 1])
        XCTAssertEqual(d.versions[0].address, "pub:headline/v3")
        XCTAssertTrue(d.versions[1].current)
        let clamp = TextureDocParser.parseBundle(bundle(body: bodyDoc(#"{"type":"paragraph","content":[{"type":"text","text":"x"}]}"#),
                                                        nav: #"{"revision":9,"latest":2}"#), address: "pub:headline")
        XCTAssertEqual(clamp.revision, 2, "a revision above latest is clamped")
        XCTAssertFalse(clamp.newer)
    }

    // 14: addresses
    func testAddresses() {
        XCTAssertEqual(TextureAddress.parse("pub:demo-x/v3").kind, .publication)
        XCTAssertEqual(TextureAddress.splitRoute("demo-x/v3").bare, "demo-x")
        XCTAssertEqual(TextureAddress.splitRoute("demo-x/v3").version, 3)
        XCTAssertEqual(TextureAddress.splitRoute("demo-x/vivid").version, 0)
        let t = TextureAddress.parse("texture:doc1@rev7")
        XCTAssertEqual(t.doc, "doc1"); XCTAssertEqual(t.rev, "rev7")
        XCTAssertEqual(TextureAddress.parse("texture:@x").kind, .none)
        XCTAssertEqual(TextureAddress.parse("pub:").kind, .none)
    }

    // 2 (citations): hosts, check labels, quote context
    func testCitationHelpers() {
        XCTAssertEqual(Citations.host("https://user:pw@WWW.Example.com:8443/p"), "example.com")
        XCTAssertEqual(Citations.host("example.org/path"), "example.org")
        XCTAssertEqual(Citations.host("not a url"), "")
        XCTAssertEqual(Citations.host(""), "")
        XCTAssertEqual(Citations.checkLabel("").state, "pending")
        XCTAssertEqual(Citations.checkLabel("TOLERANT").label, "Close match")
        XCTAssertEqual(Citations.checkLabel("weird").label, "Not yet checked")
        let c = Citations.context(of: "it\u{2019}s \u{201C}fine\u{201D} \u{2026} done", in: "Start. It's \"fine\" and then some, done. End.")
        XCTAssertEqual(c.before, "Start. ")
        XCTAssertEqual(c.hit, "It's \"fine\" and then some, done")
        XCTAssertEqual(c.after, ". End.")
        let none = Citations.context(of: "absent words", in: "Some other text.")
        XCTAssertEqual(none.hit, ""); XCTAssertEqual(none.before, "Some other text.")
        XCTAssertEqual(Citations.context(of: "", in: "abc").hit, "")
        XCTAssertEqual(Citations.context(of: "a very long quote that cannot fit", in: "short").hit, "")
    }

    // Private documents: revision, versions, revising
    func testPrivateDocument() {
        let rev = obj(###"{"revision_id":"r2","version_number":1,"content":"## Notes\n\nSome text.","created_at":"2026-11-09T10:00:00Z"}"###)
        let doc = obj(#"{"doc_id":"d1","title":"Plan","agent_revision_pending":true,"trajectory_id":"traj-1"}"#)
        let revs = [obj(#"{"revision_id":"r2","version_number":1,"author_label":"appagent","created_at":"2026-11-09T10:00:00Z"}"#),
                    obj(#"{"revision_id":"r1","version_number":0,"author_label":"you","created_at":"2026-11-08T10:00:00Z"}"#)]
        let d = TextureDocParser.parseRevision(rev, document: doc, address: "texture:d1", revisions: revs)
        XCTAssertEqual(d.title, "Plan"); XCTAssertEqual(d.kind, "document")
        XCTAssertEqual(d.revision, 2); XCTAssertEqual(d.latest, 2); XCTAssertFalse(d.newer)
        XCTAssertTrue(d.revising); XCTAssertEqual(d.trajectory, "traj-1")
        XCTAssertEqual(d.versions.map(\.address), ["texture:d1", "texture:d1@r1"])
        XCTAssertEqual(d.versions[0].author, "appagent")
        XCTAssertEqual(d.blocks.map(\.kind), ["h", "p"])
        let old = TextureDocParser.parseRevision(obj(#"{"revision_id":"r1","version_number":0,"content":"Old."}"#), document: doc, address: "texture:d1@r1", revisions: revs)
        XCTAssertEqual(old.revision, 1); XCTAssertTrue(old.newer); XCTAssertTrue(old.historical)
    }

    // Textures list: the edition is pinned first; archived documents are hidden
    func testTextureList() {
        let l = TextureList.parse(obj(#"""
        {"documents":[{"doc_id":"a","title":"Meeting notes","updated_at":"2026-11-09T10:00:00Z","revision_count":3},
                      {"doc_id":"b","title":"My edition","updated_at":"2026-11-08T10:00:00Z","revision_count":1},
                      {"doc_id":"c","title":"Old","archived_at":"2026-01-01T00:00:00Z"},
                      {"title":"no id"},
                      {"doc_id":"d","title":"","agent_revision_pending":true}]}
        """#))
        XCTAssertEqual(l.rows.map(\.address), ["texture:b", "texture:a", "texture:d"])
        XCTAssertTrue(l.hasEdition)
        XCTAssertEqual(l.rows[1].meta, "3 versions · Nov 9, 2026")
        XCTAssertEqual(l.rows[2].title, "Untitled"); XCTAssertTrue(l.rows[2].revising)
        let none = TextureList.parse(obj(#"{"documents":[]}"#))
        XCTAssertFalse(none.hasEdition); XCTAssertEqual(none.status, "ready")
    }

    // 6 (source): read-along locates by text, not by the radio's chunking
    func testFollowLocatesByText() {
        let body = bodyDoc(#"{"type":"paragraph","content":[{"type":"text","text":"The first sentence is here. The second sentence follows it."}]},{"type":"paragraph","content":[{"type":"text","text":"A later paragraph about batteries."}]}"#)
        let d = TextureDocParser.parseBundle(bundle(body: body), address: "pub:x")
        let hit = ReaderFollow.locate(spoken: "The second sentence follows it.", in: d)
        XCTAssertEqual(hit?.block, d.blocks[0].id); XCTAssertEqual(hit?.sentence, 1)
        XCTAssertEqual(ReaderFollow.locate(spoken: "later paragraph about", in: d)?.block, d.blocks[1].id)
        XCTAssertNil(ReaderFollow.locate(spoken: "nothing like this anywhere", in: d))
        XCTAssertNil(ReaderFollow.locate(spoken: "", in: d))
        XCTAssertNil(ReaderFollow.locate(spoken: "The", in: nil))
    }

    // 8: a very long document stays linear
    func testLongDocumentIsFast() {
        var paras: [String] = []
        for i in 0..<3000 { paras.append(#"{"type":"paragraph","content":[{"type":"text","text":"Sentence number \#(i) is here. And another one follows."}]}"#) }
        let start = Date()
        let d = TextureDocParser.parseBundle(bundle(body: bodyDoc(paras.joined(separator: ","))), address: "pub:x")
        XCTAssertEqual(d.blocks.count, 3000)
        XCTAssertLessThan(Date().timeIntervalSince(start), 5)
        XCTAssertEqual(d.blocks.last?.ordinal, 2999)
    }
}
