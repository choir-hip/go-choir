import XCTest
@testable import AutoradioKit

// The Epub reader (Kit/Epub.swift) stays with the Reader's shelf (books are not moved yet). Its sample book is the invented one in
// fixtures/files/sample-tree.json (made by scripts/make-files-fixtures.py).
private func sampleData(_ key: String) -> Data {
    let url = URL(fileURLWithPath: #filePath).deletingLastPathComponent().deletingLastPathComponent().deletingLastPathComponent().deletingLastPathComponent()
        .appendingPathComponent("fixtures/files/sample-tree.json")
    let tree = try! JSONSerialization.jsonObject(with: Data(contentsOf: url)) as! [String: String]
    return Data(base64Encoded: tree[key]!, options: .ignoreUnknownCharacters)!
}


/// A minimal ZIP writer (stored entries) so tests can build odd books on purpose.
private func zip(_ files: [(String, String)]) -> Data {
    var out = Data(); var central = Data()
    func le16(_ v: Int) -> [UInt8] { [UInt8(v & 255), UInt8((v >> 8) & 255)] }
    func le32(_ v: Int) -> [UInt8] { [UInt8(v & 255), UInt8((v >> 8) & 255), UInt8((v >> 16) & 255), UInt8((v >> 24) & 255)] }
    for (name, body) in files {
        let n = Array(name.utf8), b = Data(body.utf8), crc = Int(Zip.crc32(b)), off = out.count
        var h: [UInt8] = le32(0x04034b50)
        for v in [20, 0, 0, 0, 0] { h += le16(v) }
        h += le32(crc); h += le32(b.count); h += le32(b.count); h += le16(n.count); h += le16(0); h += n
        out.append(contentsOf: h); out.append(b)
        var c: [UInt8] = le32(0x02014b50)
        for v in [20, 20, 0, 0, 0, 0] { c += le16(v) }
        c += le32(crc); c += le32(b.count); c += le32(b.count); c += le16(n.count)
        for _ in 0..<4 { c += le16(0) }
        c += le32(0); c += le32(off); c += n
        central.append(contentsOf: c)
    }
    let cdOff = out.count
    out.append(central)
    var e: [UInt8] = le32(0x06054b50)
    for v in [0, 0, files.count, files.count] { e += le16(v) }
    e += le32(central.count); e += le32(cdOff); e += le16(0)
    out.append(contentsOf: e)
    return out
}

private let container = #"<?xml version="1.0"?><container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container"><rootfiles><rootfile full-path="OPS/book.opf" media-type="application/oebps-package+xml"/></rootfiles></container>"#
private func opf(_ spine: String, items: String) -> String {
    #"<?xml version="1.0"?><package xmlns="http://www.idpf.org/2007/opf" version="3.0"><metadata xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:title>T</dc:title></metadata><manifest>"# + items + "</manifest><spine>" + spine + "</spine></package>"
}


final class EpubTests: XCTestCase {
    func testSampleBook() throws {
        let book = try Epub.parse(data: sampleData("Books/The Lantern Keeper.epub"))
        XCTAssertEqual(book.title, "The Lantern Keeper")
        XCTAssertEqual(book.author, "A. Ferris (invented)")
        XCTAssertEqual(book.chapters.map(\.title), ["The lamp on the point", "Three visitors", "What the sea said"], "the nav document is not a chapter")
        let c1 = book.chapters[0].blocks
        XCTAssertEqual(c1.first?.kind, "heading"); XCTAssertEqual(c1.first?.level, 1)
        XCTAssertTrue(c1.contains { $0.kind == "quote" && $0.text.contains("A lamp is only a promise") })
        XCTAssertTrue(c1.contains { $0.kind == "rule" })
        XCTAssertTrue(c1.contains { $0.text.contains("*very*") }, "emphasis kept as a mark")
        XCTAssertTrue(c1.contains { $0.text.contains("\u{00A0}\u{2014}") || $0.text.contains("\u{2014}") }, "&mdash; and &nbsp; are mapped")
        XCTAssertEqual(book.chapters[1].blocks.filter { $0.kind == "item" }.map(\.marker), ["- ", "- ", "- "])
        let md = Epub.markdown(book.chapters[1].blocks)
        XCTAssertTrue(md.hasPrefix("# Three visitors\n\n"))
        XCTAssertTrue(md.contains("- How old is the lamp?\n- Does it ever flicker?"), "list items are tight")
        XCTAssertTrue(book.warnings.isEmpty, "\(book.warnings)")
    }
    // failures 1, 2, 3
    func testBrokenZips() throws {
        XCTAssertThrowsError(try Epub.parse(data: Data("hello".utf8))) { XCTAssertEqual($0 as? EpubError, .notAZip) }
        XCTAssertThrowsError(try Epub.parse(data: Data(repeating: 0, count: 100)))
        let good = sampleData("Books/The Lantern Keeper.epub")
        XCTAssertThrowsError(try Epub.parse(data: good.prefix(good.count / 2)))
        XCTAssertThrowsError(try Epub.parse(data: good.prefix(200)))
        // Flip a byte inside the container file's data: inflate or the CRC catches it.
        let container = try Zip.entries(good).first { $0.name == "META-INF/container.xml" }!
        var bad = good; bad[container.localOffset + 30 + container.name.utf8.count + container.compressedSize / 2] ^= 0xFF
        XCTAssertThrowsError(try Epub.parse(data: bad)) { XCTAssertTrue(($0 as? EpubError).map { if case .corrupt = $0 { return true }; return false } ?? false, "\($0)") }
        XCTAssertThrowsError(try Epub.parse(data: Data()))
    }
    // failure 5
    func testMissingPartsAreNamed() throws {
        XCTAssertThrowsError(try Epub.parse(data: zip([("a.txt", "x")]))) { XCTAssertEqual($0 as? EpubError, .missing("META-INF/container.xml")) }
        XCTAssertThrowsError(try Epub.parse(data: zip([("META-INF/container.xml", container)]))) { XCTAssertEqual($0 as? EpubError, .missing("OPS/book.opf")) }
        let noSpine = zip([("META-INF/container.xml", container), ("OPS/book.opf", opf("", items: #"<item id="a" href="a.xhtml" media-type="application/xhtml+xml"/>"#))])
        XCTAssertThrowsError(try Epub.parse(data: noSpine)) { XCTAssertEqual($0 as? EpubError, .missing("the reading order")) }
    }
    // failures 5, 6, 7, 8: a missing chapter is skipped, an unclosed tag and a bogus entity are read, a percent-encoded href resolves
    func testRoughBookStillReads() throws {
        let items = #"<item id="a" href="ch%201.xhtml#top" media-type="application/xhtml+xml"/><item id="b" href="gone.xhtml" media-type="application/xhtml+xml"/><item id="c" href="../OPS/c.xhtml" media-type="application/xhtml+xml"/>"#
        let book = try Epub.parse(data: zip([
            ("META-INF/container.xml", container),
            ("OPS/book.opf", opf(#"<itemref idref="a"/><itemref idref="b"/><itemref idref="c"/>"#, items: items)),
            ("OPS/ch 1.xhtml", #"<!DOCTYPE html PUBLIC "-//W3C//DTD XHTML 1.1//EN" "x.dtd"><html><head><title>Opening</title></head><body><p>Fish&nbsp;&amp;&bogus; chips<br/>next line <b>bold</p></body></html>"#),
            ("OPS/c.xhtml", "<html><body><div>Loose text, no paragraph</div><table><tr><td>a</td><td>b</td></tr></table><script>var x=1<2;</script></body></html>"),
        ]))
        XCTAssertEqual(book.chapters.count, 2)
        XCTAssertEqual(book.chapters[0].title, "Opening", "no heading: the <title>")
        XCTAssertTrue(book.chapters[0].blocks[0].text.hasPrefix("Fish"))
        XCTAssertEqual(book.chapters[1].title, "Chapter 2", "no heading and no title")
        XCTAssertTrue(book.chapters[1].blocks.contains { $0.text == "a | b" })
        XCTAssertFalse(book.chapters[1].blocks.contains { $0.text.contains("var x") }, "scripts are not text")
        XCTAssertTrue(book.warnings.contains { $0.contains("gone.xhtml") })
    }
    func testMarkdownEscapesLineStarts() {
        let md = Epub.markdown([EpubBlock(kind: "paragraph", text: "# not a heading"), EpubBlock(kind: "paragraph", text: "3. not a list"), EpubBlock(kind: "code", text: "x = 1")])
        XCTAssertEqual(md, "\\# not a heading\n\n\\3. not a list\n\n```\nx = 1\n```")
    }
    func testZipNamesWithDotDotAreNotUsed() throws {
        let z = zip([("../escape.txt", "x"), ("META-INF/container.xml", container)])
        XCTAssertThrowsError(try Epub.parse(data: z))   // fails on the missing package, never by reading ../escape.txt
    }
}

