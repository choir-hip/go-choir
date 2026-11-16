import XCTest

// S3: the pure rules of the Exact patch 0001 (mobile/exact-patches): MarkupSyntaxHiding.swift is the patch's own file, linked into this test target.
// What only a device or simulator can show (that the hidden ranges draw nothing, the caret over them, Return in a list) is in the design file.
final class MarkupSyntaxHidingTests: XCTestCase {
    typealias H = MarkupSyntaxHiding
    func r(_ s: NSString, _ sub: String, nth: Int = 0) -> NSRange {
        var from = 0
        var found = NSRange(location: NSNotFound, length: 0)
        for _ in 0...nth { found = s.range(of: sub, options: [], range: NSRange(location: from, length: s.length - from)); from = NSMaxRange(found) }
        return found
    }

    func test1_AHiddenRunKeepsItsLengthSoEveryOffsetStaysASourceOffset() {
        XCTAssertEqual(H.drawn(length: 2, bullet: false).utf16.count, 2)
        XCTAssertEqual(H.drawn(length: 1, bullet: true), "\u{2022}")
        XCTAssertEqual(H.drawn(length: 3, bullet: true).utf16.count, 3)
        XCTAssertTrue(H.drawn(length: 3, bullet: true).hasPrefix("\u{2022}"))
        XCTAssertFalse(H.drawn(length: 4, bullet: false).contains("#")); XCTAssertEqual(H.drawn(length: 0, bullet: true), "")
    }

    func test2_OnlyTheMarkersThatOpenABlockAreBlockPrefixes() {
        let src = "## Title\n\n> quoted **bold** text\n\n- item\n\n> - nested\n" as NSString
        let hidden = [r(src, "## "), r(src, "> "), r(src, "**"), r(src, "**", nth: 1), r(src, "> ", nth: 1)]
        let bullets = [r(src, "-"), r(src, "-", nth: 1)]
        let sorted = (hidden + bullets).sorted { $0.location < $1.location }
        let p = H.blockPrefixes(of: src, hidden: sorted.filter { !bullets.contains($0) }, bullets: bullets)
        XCTAssertEqual(p.map { src.substring(with: $0) }, ["## ", "> ", "-", "> ", "-"], "the ** inside a line is not a block prefix")
    }

    func test3_BackspaceJustAfterABlocksPrefixRemovesThatPrefix() {
        let src = "## Title\n\n- item\n\n> - nested\n" as NSString
        let heading = r(src, "## "), bullet = r(src, "-"), quote = r(src, "> "), nested = r(src, "-", nth: 1)
        let prefixes = H.blockPrefixes(of: src, hidden: [heading, quote], bullets: [bullet, nested])
        XCTAssertEqual(H.backspaceRemoves(caret: NSMaxRange(heading), prefixes: prefixes), heading)
        XCTAssertEqual(H.backspaceRemoves(caret: NSMaxRange(bullet), prefixes: prefixes), bullet)
        XCTAssertEqual(H.backspaceRemoves(caret: NSMaxRange(nested), prefixes: prefixes), nested, "the innermost goes first: the bullet, then the quote")
        XCTAssertEqual(H.backspaceRemoves(caret: NSMaxRange(quote), prefixes: prefixes), quote)
        XCTAssertNil(H.backspaceRemoves(caret: NSMaxRange(heading) + 3, prefixes: prefixes), "in the middle of text it is an ordinary backspace")
    }

    func test4_IndentedAndNumberedListsAndInlineMarkersAreNotMistaken() {
        let src = "  - indented\n\n1. first\n\n*em* and `code`\n" as NSString
        let bullet = r(src, "-")
        let star1 = r(src, "*"), star2 = r(src, "*", nth: 1), tick = r(src, "`")
        let p = H.blockPrefixes(of: src, hidden: [star1, star2, tick], bullets: [bullet])
        XCTAssertEqual(p, [bullet], "an indented bullet is a prefix; emphasis and code markers are not, even at the start of a line")
    }

    func test5_ThePatchCarriesThisVeryFile() throws {
        let mobile = URL(fileURLWithPath: #filePath).deletingLastPathComponent().deletingLastPathComponent().deletingLastPathComponent().deletingLastPathComponent()
        let patch = try String(contentsOf: mobile.appendingPathComponent("exact-patches/0001-markdown-editor-hides-all-syntax.patch"), encoding: .utf8)
        let source = try String(contentsOf: mobile.appendingPathComponent("exact-patches/sources/MarkupSyntaxHiding.swift"), encoding: .utf8)
        for line in source.components(separatedBy: "\n") {
            XCTAssertTrue(patch.contains("+" + line + "\n"), "the patch has this line of the file: \(line)")
        }
        XCTAssertTrue(patch.contains("UInt32.max, UInt32.max"), "the editor asks the styler with nothing to reveal")
    }
}
