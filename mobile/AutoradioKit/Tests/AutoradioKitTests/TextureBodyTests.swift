import XCTest
@testable import AutoradioKit

// S2: the document stays Texture-format compatible always (TextureBody.swift lists the ways it can fail).
final class TextureBodyTests: XCTestCase {
    static let mobile = URL(fileURLWithPath: #filePath).deletingLastPathComponent().deletingLastPathComponent().deletingLastPathComponent().deletingLastPathComponent()

    /// Real Texture bodies: the repo's sample stories (citations) and a hand-built mixed document (table, embed, unknown node, nested lists, code).
    static func fixtures() throws -> [(name: String, body: [String: Any])] {
        var out: [(String, [String: Any])] = []
        let dir = Self.mobile.appendingPathComponent("fixtures/texture")
        for f in try FileManager.default.contentsOfDirectory(atPath: dir.path).sorted() where f.hasPrefix("story-") {
            let o = try JSONSerialization.jsonObject(with: Data(contentsOf: dir.appendingPathComponent(f))) as? [String: Any]
            if let b = (o?["artifact"] as? [String: Any])?["body_doc"] as? [String: Any] { out.append((f, b)) }
        }
        let letter = try JSONSerialization.jsonObject(with: Data(contentsOf: Self.mobile.appendingPathComponent("fixtures/texture/letter-citations-embed.json"))) as! [String: Any]
        out.append(("letter-citations-embed", letter["body_doc"] as! [String: Any]))
        let rich = try JSONSerialization.jsonObject(with: Data(contentsOf: Self.mobile.appendingPathComponent("fixtures/texture/rich-mixed.json"))) as! [String: Any]
        out.append(("rich-mixed", rich["body_doc"] as! [String: Any]))
        return out
    }
    func same(_ a: Any?, _ b: Any?) -> Bool { (a as? NSObject)?.isEqual(b) ?? false }
    func top(_ b: [String: Any]) -> [[String: Any]] { TextureBodyMerge.topNodes(b) }

    func test1_LoadEditNothingSaveIsTheOriginal() throws {
        let fx = try Self.fixtures()
        XCTAssertGreaterThanOrEqual(fx.count, 3)
        for (name, body) in fx {
            let md = TextureMd.markdown(from: body)
            let r = TextureBodyMerge.merge(original: body, originalMarkdown: md, newMarkdown: md)
            XCTAssertTrue(same(r.body, body), "\(name): the body_doc is the original")
            // the same when the text is re-sent through the block pipeline (an edit that puts the text back)
            let again = TextureBodyMerge.merge(original: body, originalMarkdown: md, newMarkdown: md + "\n\nx")
            let back = TextureBodyMerge.merge(original: again.body, originalMarkdown: md + "\n\nx", newMarkdown: md)
            XCTAssertEqual(top(back.body).count, top(body).count, "\(name): adding a paragraph and taking it away leaves the node list")
            for (a, b) in zip(top(back.body), top(body)) { XCTAssertTrue(same(a, b), "\(name): \(a["type"] ?? "") node is as it was") }
        }
    }

    func test2_ASmallEditChangesOnlyTheEditedNode() throws {
        for (name, body) in try Self.fixtures() {
            let md = TextureMd.markdown(from: body)
            let blocks = TextureMd.blocks(md)
            // edit the first paragraph block that has a word to change
            guard let bi = blocks.firstIndex(where: { TextureMd.kind(of: $0) == .paragraph && $0.contains(" ") }) else { continue }
            var edited = blocks; edited[bi] = blocks[bi].replacingOccurrences(of: " ", with: " very ", options: [], range: blocks[bi].range(of: " "))
            let r = TextureBodyMerge.merge(original: body, originalMarkdown: md, newMarkdown: edited.joined(separator: "\n\n"))
            let before = top(body), after = top(r.body)
            XCTAssertEqual(before.count, after.count, name)
            var changed: [Int] = []
            for (i, (a, b)) in zip(before, after).enumerated() where !same(a, b) { changed.append(i) }
            XCTAssertEqual(changed.count, 1, "\(name): exactly one node changed, got \(changed)")
            guard let c = changed.first else { continue }
            XCTAssertEqual((after[c]["type"] as? String), "paragraph")
            XCTAssertEqual((after[c]["attrs"] as? [String: Any])?["id"] as? String, (before[c]["attrs"] as? [String: Any])?["id"] as? String, "\(name): it keeps its id")
            let oldRefs = ((before[c]["content"] as? [[String: Any]]) ?? []).filter { ($0["type"] as? String) == "source_ref" }
            let newRefs = ((after[c]["content"] as? [[String: Any]]) ?? []).filter { ($0["type"] as? String) == "source_ref" }
            XCTAssertTrue(same(oldRefs, newRefs), "\(name): its citations are kept")
            XCTAssertTrue(TextureMd.plainText(after[c]).contains("very"))
            XCTAssertEqual(after.compactMap { $0["type"] as? String }, before.compactMap { $0["type"] as? String }, "\(name): no node changed type or moved")
        }
    }

    func test3_AnEmbedATableAndAnUnknownNodeSurviveEditsAround() throws {
        let body = try Self.fixtures().first { $0.name == "rich-mixed" }!.body
        let md = TextureMd.markdown(from: body)
        XCTAssertFalse(md.contains("Region"), "a table has no Markdown block of its own here: it is kept as it is")
        let edited = md.replacingOccurrences(of: "West", with: "West coast").replacingOccurrences(of: "Last line.", with: "Last line, edited.")
        let r = TextureBodyMerge.merge(original: body, originalMarkdown: md, newMarkdown: edited)
        let types = top(r.body).compactMap { $0["type"] as? String }
        XCTAssertEqual(types, top(body).compactMap { $0["type"] as? String }, "same nodes in the same order")
        for t in ["embed", "table", "future_widget"] {
            XCTAssertTrue(same(top(r.body).first { ($0["type"] as? String) == t }, top(body).first { ($0["type"] as? String) == t }), "\(t) is untouched")
        }
        XCTAssertTrue(TextureMd.plainText(top(r.body).first { ($0["type"] as? String) == "bullet_list" }!).contains("West coast"))
        // deleting the paragraph before an embed keeps the embed where it was
        let gone = md.replacingOccurrences(of: TextureMd.blocks(md)[1], with: "")
        let r2 = TextureBodyMerge.merge(original: body, originalMarkdown: md, newMarkdown: gone)
        XCTAssertNotNil(top(r2.body).first { ($0["type"] as? String) == "embed" })
    }

    func test4_AnEditedTableKeepsItsNodeAndTheEditLivesInTheMarkdown() throws {
        let body: [String: Any] = ["schema": "choir.texture_doc.v1", "doc": ["type": "doc", "attrs": ["id": "d"], "content": [
            ["type": "paragraph", "attrs": ["id": "p"], "content": [["type": "text", "text": "Intro"]]],
            ["type": "table", "attrs": ["id": "t"], "content": [["type": "table_row", "content": [["type": "table_cell", "content": [["type": "text", "text": "Region"]]]]]]]]]]
        let md = "Intro\n\n| Region |\n|---|"
        let r = TextureBodyMerge.merge(original: body, originalMarkdown: md, newMarkdown: "Intro\n\n| Region | Units |\n|---|---|")
        XCTAssertEqual(top(r.body).compactMap { $0["type"] as? String }, ["paragraph", "table"])
        XCTAssertEqual(r.opaqueEdits, 1)
        XCTAssertTrue(same(top(r.body)[1], top(body)[1]))
    }

    func test5_ANewDocumentGetsAnEditorCompatibleBody() {
        let md = "# Title\n\nA **bold** and *slanted* word with `code` and a [link](https://x.test).\n\n- one\n  - nested\n- two\n\n1. first\n2. second\n\n> quoted\n\n```\nlet x\n```\n\n---"
        let r = TextureBodyMerge.merge(original: nil, originalMarkdown: "", newMarkdown: md)
        XCTAssertEqual(r.body["schema"] as? String, "choir.texture_doc.v1")
        let nodes = top(r.body)
        XCTAssertEqual(nodes.compactMap { $0["type"] as? String }, ["heading", "paragraph", "bullet_list", "ordered_list", "blockquote", "code_block", "horizontal_rule"])
        XCTAssertEqual((nodes[0]["attrs"] as? [String: Any])?["level"] as? Int, 1)
        let inline = nodes[1]["content"] as! [[String: Any]]
        XCTAssertTrue(inline.contains { ($0["text"] as? String) == "bold" && (($0["marks"] as? [[String: Any]])?.first?["type"] as? String) == "strong" })
        XCTAssertTrue(inline.contains { ($0["text"] as? String) == "slanted" && (($0["marks"] as? [[String: Any]])?.first?["type"] as? String) == "emphasis" })
        XCTAssertTrue(inline.contains { ($0["text"] as? String) == "code" && (($0["marks"] as? [[String: Any]])?.first?["type"] as? String) == "code" })
        XCTAssertTrue(inline.contains { ($0["text"] as? String)?.contains("a link.") == true }, "a link keeps its text: the web has no link mark")
        XCTAssertFalse(inline.contains { ($0["text"] as? String)?.contains("https") == true })
        XCTAssertNil((inline.compactMap { $0["marks"] as? [[String: Any]] }.flatMap { $0 }).first { ($0["type"] as? String) == "link" })
        XCTAssertEqual((nodes[2]["content"] as! [[String: Any]]).count, 2)
        XCTAssertEqual((nodes[3]["attrs"] as? [String: Any])?["start"] as? Int, 1, "as the web writes it")
        for n in nodes { XCTAssertNotNil((n["attrs"] as? [String: Any])?["id"], "\(n["type"] ?? "") has an id") }
        // what was written reads back as the same Markdown blocks
        XCTAssertEqual(TextureMd.blocks(TextureMd.markdown(from: r.body)).count, 7)
    }

    func test6_TheMarkdownIsKeptByteForByte() {
        let md = "A  \nB  trailing\n\n\n\n# H  \n"
        let r = TextureBodyMerge.merge(original: ["schema": "choir.texture_doc.v1", "doc": ["type": "doc", "content": [["type": "paragraph", "content": [["type": "text", "text": "A"]]]]]],
                                       originalMarkdown: md, newMarkdown: md)
        XCTAssertEqual(top(r.body).count, 1, "the original body, whatever the Markdown said")
    }
}
