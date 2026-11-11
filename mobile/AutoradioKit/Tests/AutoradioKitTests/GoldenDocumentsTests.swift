import XCTest
@testable import AutoradioKit

// The golden documents through the native reader's model layer (internal/texturedoc/golden/README.md).
//
// What this is: TextureDocParser.parseBundle is the code the phone runs on a published bundle (TextureReaderSource.fetch
// calls it, then the Contract draws the ReaderDoc's blocks). This test parses each golden file with it and writes the
// SAME structural projection the web spec (frontend/tests/golden-documents.spec.js) reads out of the DOM: the ordered
// blocks with kind, level, depth, list marker, whitespace-normalised text with a [n] token per citation, the styled
// spans, each citation's number, title and quote, an embed's kind and label, a table's cells. scripts/golden-docs
// then diffs the two sides. What it does NOT see: the Exact Contract that draws the blocks (reader.contract), fonts,
// colours, spacing, the page chrome (headline, dek, byline, disclosure, sources sheet, versions).
//
// WAYS THIS CAN FAIL (written before the code):
//  1. A golden file that cannot be read or parsed: the test fails by name; a document is never skipped silently.
//  2. The projection drifts from the web spec's (a kind renamed, a marker written differently): the compare script
//     reports every document as different; the two projectors are kept to one written definition (the README).
//  3. A block the parser drops (empty paragraph, heading equal to the title): the projection shows the drop; the
//     compare script lists it unless known-differences.json names it with a reason.
//  4. The golden directory moves: found from this file's path (#filePath), and the test fails if it is empty.
//  5. The output directory is not writable: the test fails rather than pretend it compared.
final class GoldenDocumentsTests: XCTestCase {
    static func goldenDir() -> URL {
        // .../mobile/AutoradioKit/Tests/AutoradioKitTests/GoldenDocumentsTests.swift -> repo root
        var u = URL(fileURLWithPath: #filePath)
        for _ in 0..<5 { u.deleteLastPathComponent() }
        return u.appendingPathComponent("internal/texturedoc/golden")
    }

    static func squash(_ s: String) -> String {
        let parts = s.components(separatedBy: CharacterSet.whitespacesAndNewlines.union(CharacterSet(charactersIn: "\u{00A0}")))
        return parts.filter { !$0.isEmpty }.joined(separator: " ")
    }

    static func style(_ r: ReaderRun) -> String {
        var s: [String] = []
        if r.kind == "code" { s.append("code") }
        if r.weight >= 700 { s.append("strong") }
        if r.slant == "italic" { s.append("emphasis") }
        return s.sorted().joined(separator: "+")
    }

    /// The block's text, its styled spans and its citations, from the runs.
    static func inline(_ runs: [ReaderRun], doc: ReaderDoc, checks: Bool) -> (text: String, spans: [[String: String]], cites: [[String: Any]]) {
        var text = ""
        var spans: [(style: String, text: String)?] = []
        var cites: [[String: Any]] = []
        for r in runs {
            switch r.kind {
            case "cite", "ref":
                let n = Int(r.label) ?? 0
                text += "[\(n)]"
                spans.append(nil)
                let c = doc.cites.first { $0.anchor == r.anchor }
                var o: [String: Any] = ["n": n, "title": squash(c?.title ?? ""), "quote": squash(c?.quote ?? "")]
                if checks { o["check"] = c?.check ?? "" }
                cites.append(o)
            case "br":
                text += "\n"
                spans.append(nil)
            default:
                text += r.text
                let st = style(r)
                if st.isEmpty {
                    if !squash(r.text).isEmpty { spans.append(nil) }
                } else if let last = spans.last, let l = last, l.style == st {
                    spans[spans.count - 1] = (st, l.text + r.text)
                } else {
                    spans.append((st, r.text))
                }
            }
        }
        let styled = spans.compactMap { $0 }.map { ["style": $0.style, "text": squash($0.text)] }.filter { $0["text"] != "" }
        return (squash(text), styled, cites)
    }

    static func project(_ doc: ReaderDoc, checks: Bool) -> [[String: Any]] {
        var out: [[String: Any]] = []
        for b in doc.blocks {
            var o: [String: Any] = [:]
            switch b.kind {
            case "h":
                o["kind"] = "heading"; o["level"] = b.level
                if b.depth > 0 { o["depth"] = b.depth }
            case "p": o["kind"] = "paragraph"
            case "li":
                o["kind"] = "list_item"; o["depth"] = b.depth
                o["marker"] = (b.marker == "\u{2022}" || b.marker == "\u{2013}") ? "bullet" : b.marker
            case "quote": o["kind"] = "quote"; o["depth"] = b.depth
            case "code":
                o["kind"] = "code"; o["text"] = b.text.replacingOccurrences(of: "\\n+$", with: "", options: .regularExpression)
                out.append(o); continue
            case "rule": o["kind"] = "rule"; o["text"] = ""; out.append(o); continue
            case "table":
                o["kind"] = "table"; o["text"] = ""
                o["rows"] = b.rows.map { r in r.cells.map { c in inline(c.runs, doc: doc, checks: false).text } }
                out.append(o); continue
            case "embed": o["kind"] = "embed"; o["embed"] = b.marker; o["text"] = squash(b.text); out.append(o); continue
            default: o["kind"] = b.kind
            }
            let i = inline(b.runs, doc: doc, checks: checks)
            o["text"] = i.text
            if !i.spans.isEmpty { o["spans"] = i.spans }
            if !i.cites.isEmpty { o["cites"] = i.cites }
            out.append(o)
        }
        return out
    }

    func testGoldenDocumentsProjection() throws {
        let dir = Self.goldenDir()
        let files = try FileManager.default.contentsOfDirectory(at: dir, includingPropertiesForKeys: nil)
            .filter { $0.pathExtension == "json" && $0.lastPathComponent != "known-differences.json" }
            .sorted { $0.lastPathComponent < $1.lastPathComponent }
        XCTAssertGreaterThanOrEqual(files.count, 8, "golden documents not found at \(dir.path)")
        let outRoot = URL(fileURLWithPath: ProcessInfo.processInfo.environment["GOLDEN_OUT"] ?? NSTemporaryDirectory() + "golden")
            .appendingPathComponent("native")
        try FileManager.default.createDirectory(at: outRoot, withIntermediateDirectories: true)
        for f in files {
            let obj = try JSONSerialization.jsonObject(with: Data(contentsOf: f)) as! [String: Any]
            let g = obj["golden"] as! [String: Any]
            let name = g["name"] as! String
            let checks = (g["checks"] as? Bool) ?? false
            let bare = ((obj["route"] as? [String: Any])?["bare"] as? String) ?? name
            let doc = TextureDocParser.parseBundle(obj, address: "pub:\(bare)")
            XCTAssertEqual(doc.status, "ready", name)
            XCTAssertFalse(doc.blocks.isEmpty, name)
            let projection: [String: Any] = ["name": name, "blocks": Self.project(doc, checks: checks)]
            let data = try JSONSerialization.data(withJSONObject: projection, options: [.prettyPrinted, .sortedKeys, .withoutEscapingSlashes])
            try data.write(to: outRoot.appendingPathComponent("\(name).json"))
        }
    }
}
