import XCTest
@testable import AutoradioKit

// The six places the native reader model departed from the web reader on the golden documents
// (internal/texturedoc/golden/known-differences.json). Each test names the web rule it matches.
//
// WAYS THIS CAN FAIL (written before the code):
//  1. The quote of a checked citation is the check's outcome word ("exact"): the server's checks.citations[].quote is the
//     outcome, not the words. A demo-shaped check whose quote IS the words must still work.
//  2. A citation's own attrs.ref.quote, or its manifest anchor's ref.quote, is not read when no transclusion or check has it.
//  3. An embed shows its alt text where the web shows the caption; the alt text must still be carried for accessibility.
//  4. A heading in a blockquote is forced to level 3 instead of keeping its own.
//  5. A code block of two text nodes runs them together; the web joins with a newline.
//  6. Adjacent citations of one source draw twice when the bundle has manifest anchors (the web draws one); without manifest
//     anchors the web draws both, so native must too; a word between them, or a hard break, ends the run.
//  7. Paired ** in unmarked text shows the asterisks; "2 ** 3", "a**b**c" (letters outside) and a lone ** stay literal.
final class WebParityTests: XCTestCase {
    func obj(_ s: String) -> [String: Any] { (try! JSONSerialization.jsonObject(with: Data(s.utf8))) as! [String: Any] }
    func doc(_ content: String, entities: String = "[]", payload: String = "{}", checks: String = "[]") -> ReaderDoc {
        let b = obj(#"{"publication":{"title":"T"},"artifact":{"body_doc":{"schema":"choir.texture_doc.v1","doc":{"type":"doc","content":[\#(content)]}}},"source_entities":\#(entities),"payload":\#(payload),"checks":{"citations":\#(checks)}}"#)
        return TextureDocParser.parseBundle(b, address: "pub:t")
    }
    let ents = #"[{"source_entity_id":"s1","entity":{"target":{"kind":"web_url","uri":"https://example.com/a"},"display":{"title":"A"}}}]"#
    func ref(_ id: String, _ quote: String? = nil) -> String {
        let r = quote.map { #","ref":{"doc":"d","cid":"c1","quote":"\#($0)"}"# } ?? ""
        return #"{"type":"source_ref","attrs":{"id":"\#(id)","source_entity_id":"s1"\#(r)}}"#
    }

    // 1, 2
    func testQuoteIsTheQuotedWordsNotTheCheckOutcome() {
        let manifest = #"{"manifest":{"anchors":[{"anchor_id":"a1","ref":{"doc":"d","cid":"c1","quote":"quoted words"}}]}}"#
        let d = doc(#"{"type":"paragraph","content":[{"type":"text","text":"x"},\#(ref("a1"))]}"#, entities: ents, payload: manifest,
                    checks: #"[{"anchor_id":"a1","state":"exact","quote":"exact"}]"#)
        XCTAssertEqual(d.cites.first?.quote, "quoted words")
        XCTAssertEqual(d.cites.first?.check, "exact")
        // a demo-shaped check whose quote is the words
        let d2 = doc(#"{"type":"paragraph","content":[{"type":"text","text":"x"},\#(ref("a1"))]}"#, entities: ents, checks: #"[{"anchor_id":"a1","state":"exact","quote":"the words"}]"#)
        XCTAssertEqual(d2.cites.first?.quote, "the words")
    }
    func testCitationsOwnRefQuote() {
        let d = doc(#"{"type":"paragraph","content":[{"type":"text","text":"x"},\#(ref("a1", "own quote"))]}"#, entities: ents)
        XCTAssertEqual(d.cites.first?.quote, "own quote")
    }

    // 3
    func testEmbedLabelIsTheCaptionAndAltIsKept() {
        let e = #"{"type":"embed","attrs":{"id":"e1","kind":"image","target":{"digest":"x"},"alt":"alt words","caption":"caption words"}}"#
        let e2 = #"{"type":"embed","attrs":{"id":"e2","kind":"image","target":{"digest":"x"},"alt":"only alt"}}"#
        let d = doc(e + "," + e2)
        XCTAssertEqual(d.blocks[0].text, "caption words")
        XCTAssertEqual(d.blocks[0].alt, "alt words")
        XCTAssertEqual(d.blocks[1].text, "only alt")
    }

    // 4, 5
    func testQuoteHeadingKeepsItsLevelAndCodeJoinsWithNewline() {
        let d = doc(#"{"type":"blockquote","content":[{"type":"heading","attrs":{"level":2},"content":[{"type":"text","text":"H"}]}]},{"type":"code_block","content":[{"type":"text","text":"a"},{"type":"text","text":"b"}]}"#)
        XCTAssertEqual(d.blocks[0].level, 2)
        XCTAssertEqual(d.blocks[1].text, "a\nb")
    }

    // 6
    func testAdjacentPublishedCitationsOfOneSourceCollapse() {
        let manifest = #"{"manifest":{"anchors":[{"anchor_id":"a1","ref":{"doc":"d","cid":"c1"}},{"anchor_id":"a2","ref":{"doc":"d","cid":"c1"}},{"anchor_id":"a3","ref":{"doc":"d","cid":"c1"}}]}}"#
        func marks(_ d: ReaderDoc) -> Int { d.blocks[0].runs.filter { $0.kind == "cite" }.count }
        let two = #"{"type":"paragraph","content":[{"type":"text","text":"x"},\#(ref("a1")),\#(ref("a2"))]}"#
        let d = doc(two, entities: ents, payload: manifest)
        XCTAssertEqual(marks(d), 1)
        XCTAssertEqual(d.cites.count, 2, "both cards stay for the Sources list")
        XCTAssertEqual(marks(doc(two, entities: ents)), 2, "no manifest anchors: the web draws both")
        let spaced = #"{"type":"paragraph","content":[{"type":"text","text":"x"},\#(ref("a1")),{"type":"text","text":" "},\#(ref("a2")),{"type":"text","text":" and "},\#(ref("a3"))]}"#
        XCTAssertEqual(marks(doc(spaced, entities: ents, payload: manifest)), 2, "a blank text keeps the run, a word ends it")
        let broken = #"{"type":"paragraph","content":[{"type":"text","text":"x"},\#(ref("a1")),{"type":"hard_break"},\#(ref("a2"))]}"#
        XCTAssertEqual(marks(doc(broken, entities: ents, payload: manifest)), 2)
    }

    // 7
    func testLegacyDoubleAsteriskIsBold() {
        func runs(_ t: String) -> [ReaderRun] { doc(#"{"type":"paragraph","content":[{"type":"text","text":"\#(t)"}]}"#).blocks[0].runs }
        let r = runs("**Legacy bold.** Then plain.")
        XCTAssertEqual(r.first?.text, "Legacy bold.")
        XCTAssertEqual(r.first?.weight, 700)
        XCTAssertEqual(r.map(\.text).joined(), "Legacy bold. Then plain.")
        for literal in ["2 ** 3 ** 4", "a**b**c", "a lone ** here"] {
            XCTAssertEqual(runs(literal).map(\.text).joined(), literal, literal)
            XCTAssertTrue(runs(literal).allSatisfy { $0.weight == 400 }, literal)
        }
    }
}
