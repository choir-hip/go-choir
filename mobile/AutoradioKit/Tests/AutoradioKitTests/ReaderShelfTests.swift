import XCTest
import Compression
import CoreGraphics
import CoreText
@testable import AutoradioKit

// Reader lane tests. Failure list: Reader.swift (top) and Epub.swift. Every book and PDF here is made in code.

private func le16(_ v: Int) -> [UInt8] { [UInt8(v & 255), UInt8((v >> 8) & 255)] }
private func le32(_ v: Int) -> [UInt8] { [UInt8(v & 255), UInt8((v >> 8) & 255), UInt8((v >> 16) & 255), UInt8((v >> 24) & 255)] }

/// A ZIP writer: `deflate` true compresses each entry with raw deflate.
private func makeZip(_ files: [(String, String)], deflate: Bool = false) -> Data {
    var out = Data(), central = Data()
    for (name, body) in files {
        let n = Array(name.utf8), raw = Data(body.utf8), crc = Int(Zip.crc32(raw)), off = out.count
        var stored = raw, method = 0
        if deflate, !raw.isEmpty {
            var buf = [UInt8](repeating: 0, count: raw.count + 1024)
            let c = raw.withUnsafeBytes { s in compression_encode_buffer(&buf, buf.count, s.bindMemory(to: UInt8.self).baseAddress!, raw.count, nil, COMPRESSION_ZLIB) }
            if c > 0 { stored = Data(buf.prefix(c)); method = 8 }
        }
        var h: [UInt8] = le32(0x04034b50)
        for v in [20, 0, method, 0, 0] { h += le16(v) }
        h += le32(crc); h += le32(stored.count); h += le32(raw.count); h += le16(n.count); h += le16(0); h += n
        out.append(contentsOf: h); out.append(stored)
        var c: [UInt8] = le32(0x02014b50)
        for v in [20, 20, 0, method, 0, 0] { c += le16(v) }
        c += le32(crc); c += le32(stored.count); c += le32(raw.count); c += le16(n.count)
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

private let cont = #"<?xml version="1.0"?><container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container"><rootfiles><rootfile full-path="OPS/book.opf" media-type="application/oebps-package+xml"/></rootfiles></container>"#
private func chapter(_ title: String, _ body: String) -> String {
    #"<?xml version="1.0"?><!DOCTYPE html><html xmlns="http://www.w3.org/1999/xhtml"><head><title>x</title></head><body><h1>"# + title + "</h1>" + body + "</body></html>"
}
private func book(extra: [(String, String)] = [], deflate: Bool = false) -> Data {
    let opf = #"<?xml version="1.0"?><package xmlns="http://www.idpf.org/2007/opf" version="3.0"><metadata xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:title>The Test Book</dc:title><dc:creator>A. Writer</dc:creator></metadata><manifest><item id="c1" href="c1.xhtml" media-type="application/xhtml+xml"/><item id="c2" href="c2.xhtml" media-type="application/xhtml+xml"/></manifest><spine><itemref idref="c1"/><itemref idref="c2"/></spine></package>"#
    return makeZip([("mimetype", "application/epub+zip"), ("META-INF/container.xml", cont), ("OPS/book.opf", opf),
                    ("OPS/c1.xhtml", chapter("One", "<p>The <em>lantern</em> was lit &amp; the caf&eacute; closed&nbsp;early.</p><p></p><p>Second <b>paragraph</b> with <i>nested <b>tags</b></i>.</p><ul><li>apple</li><li>pear</li></ul>")),
                    ("OPS/c2.xhtml", chapter("Two", "<blockquote><p>Quoted words here.</p></blockquote><p>The harbour at dawn.</p>"))] + extra, deflate: deflate)
}

func makePDF(_ pages: [String]) -> Data {
    let data = NSMutableData()
    var box = CGRect(x: 0, y: 0, width: 612, height: 792)
    let ctx = CGContext(consumer: CGDataConsumer(data: data)!, mediaBox: &box, nil)!
    for t in pages {
        ctx.beginPDFPage(nil)
        if !t.isEmpty {
            let attr = NSAttributedString(string: t, attributes: [.font: CTFontCreateWithName("Helvetica" as CFString, 14, nil)])
            let fs = CTFramesetterCreateWithAttributedString(attr)
            let frame = CTFramesetterCreateFrame(fs, CFRange(location: 0, length: 0), CGPath(rect: CGRect(x: 50, y: 50, width: 500, height: 690), transform: nil), nil)
            CTFrameDraw(frame, ctx)
        }
        ctx.endPDFPage()
    }
    ctx.closePDF()
    return data as Data
}

private func tmpDir() -> URL {
    let u = FileManager.default.temporaryDirectory.appendingPathComponent("lib-test-\(UUID().uuidString)")
    try? FileManager.default.createDirectory(at: u, withIntermediateDirectories: true)
    return u
}
private func write(_ d: Data, _ name: String, in dir: URL) -> URL { let u = dir.appendingPathComponent(name); try! d.write(to: u); return u }

final class ShelfZipEpubTests: XCTestCase {
    func testStoredAndDeflatedBooksGiveTheSameText() throws {
        let a = try Epub.parse(data: book()), b = try Epub.parse(data: book(deflate: true))
        XCTAssertEqual(a.chapters.map(\.blocks), b.chapters.map(\.blocks))
        XCTAssertEqual(a.title, "The Test Book"); XCTAssertEqual(a.author, "A. Writer")
        XCTAssertEqual(a.chapters.count, 2)
    }
    func testTruncatedZipFailsCleanly() {
        let d = book(deflate: true)
        XCTAssertThrowsError(try Epub.parse(data: d.prefix(d.count / 2)))
        XCTAssertThrowsError(try Epub.parse(data: d.prefix(10)))
    }
    func testLongCentralDirectory() throws {
        let many = (0..<600).map { ("OPS/extra\($0).txt", "x") }
        let b = try Epub.parse(data: book(extra: many, deflate: true))
        XCTAssertEqual(b.chapters.count, 2)
    }
    func testEntitiesNestedTagsAndEmptyParagraphs() throws {
        let t = DocExtract.epub(try Epub.parse(data: book()))
        let texts = t.blocks.map(\.text)
        XCTAssertTrue(texts.contains("The lantern was lit & the café closed early."), "\(texts)")
        XCTAssertTrue(texts.contains("Second paragraph with nested tags."))
        XCTAssertFalse(texts.contains(""))
        XCTAssertEqual(t.sections.map(\.title), ["One", "Two"])
        XCTAssertEqual(t.blocks.filter { $0.kind == "item" }.count, 2)
        XCTAssertEqual(t.blocks.first { $0.text.hasPrefix("Quoted") }?.kind, "quote")
    }
    func testChapterTitlesComeFromNavAndNcx() throws {
        func make(_ manifestExtra: String, _ files: [(String, String)]) -> Data {
            let opf = #"<?xml version="1.0"?><package xmlns="http://www.idpf.org/2007/opf" version="3.0"><metadata xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:title>T</dc:title></metadata><manifest><item id="c1" href="c1.xhtml" media-type="application/xhtml+xml"/><item id="c2" href="c2.xhtml" media-type="application/xhtml+xml"/>"# + manifestExtra + #"</manifest><spine><itemref idref="c1"/><itemref idref="c2"/></spine></package>"#
            return makeZip([("META-INF/container.xml", cont), ("OPS/book.opf", opf), ("OPS/c1.xhtml", chapter("One", "<p>Alpha.</p>")), ("OPS/c2.xhtml", chapter("Two", "<p>Beta.</p>"))] + files)
        }
        let nav = #"<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops"><body><nav epub:type="toc"><ol><li><a href="c1.xhtml">The Beginning</a></li><li><a href="c2.xhtml#s1">The Harbour</a></li></ol></nav></body></html>"#
        let a = try Epub.parse(data: make(#"<item id="nav" href="nav.xhtml" media-type="application/xhtml+xml" properties="nav"/>"#, [("OPS/nav.xhtml", nav)]))
        XCTAssertEqual(a.chapters.map(\.title), ["The Beginning", "The Harbour"])
        let ncx = #"<ncx xmlns="http://www.daisy.org/z3986/2005/ncx/"><navMap><navPoint id="a"><navLabel><text>First Light</text></navLabel><content src="c1.xhtml"/></navPoint><navPoint id="b"><navLabel><text>Second Wind</text></navLabel><content src="c2.xhtml"/></navPoint></navMap></ncx>"#
        let b = try Epub.parse(data: make(#"<item id="ncx" href="toc.ncx" media-type="application/x-dtbncx+xml"/>"#, [("OPS/toc.ncx", ncx)]))
        XCTAssertEqual(b.chapters.map(\.title), ["First Light", "Second Wind"])
        XCTAssertEqual(try Epub.parse(data: make("", [])).chapters.map(\.title), ["One", "Two"])   // no contents: the headings
    }
    func testProtectedBookIsRefusedButFontObfuscationIsNot() throws {
        let drm = #"<encryption xmlns="urn:oasis:names:tc:opendocument:xmlns:container"><EncryptedData xmlns="http://www.w3.org/2001/04/xmlenc#"><EncryptionMethod Algorithm="http://www.w3.org/2001/04/xmlenc#aes128-cbc"/><CipherData><CipherReference URI="OPS/c1.xhtml"/></CipherData></EncryptedData></encryption>"#
        XCTAssertThrowsError(try Epub.parse(data: book(extra: [("META-INF/encryption.xml", drm)]))) { XCTAssertEqual($0 as? EpubError, .protected) }
        let fonts = #"<encryption xmlns="urn:oasis:names:tc:opendocument:xmlns:container"><EncryptedData xmlns="http://www.w3.org/2001/04/xmlenc#"><EncryptionMethod Algorithm="http://www.idpf.org/2008/embedding"/><CipherData><CipherReference URI="OPS/f.otf"/></CipherData></EncryptedData></encryption>"#
        XCTAssertNoThrow(try Epub.parse(data: book(extra: [("META-INF/encryption.xml", fonts)])))
    }
    func testPlainRemovesMarksAndKeepsEscapedOnes() {
        XCTAssertEqual(DocExtract.plain("a *b* **c** `d` \\*e\\*"), "a b c d *e*")
    }
}

final class ShelfPDFTests: XCTestCase {
    func testPagesAndParagraphs() {
        let t = DocExtract.pages(["First line\nsecond line\n\nNew paragraph here", "", "Third page text"])
        XCTAssertEqual(t.sections.map(\.title), ["Page 1", "Page 3"])
        XCTAssertEqual(t.blocks.map(\.text), ["First line second line", "New paragraph here", "Third page text"])
        XCTAssertEqual(t.section(of: 2), 1)
    }
    func testHyphenatedLineEndJoins() {
        XCTAssertEqual(DocExtract.paragraphs("a repre-\nsentation of it"), ["a representation of it"])
    }
    func testLongPageWithoutBlankLinesIsCutAtSentences() {
        let s = (0..<40).map { "This is sentence number \($0)." }.joined(separator: "\n")
        let ps = DocExtract.paragraphs(s)
        XCTAssertGreaterThan(ps.count, 1)
        XCTAssertTrue(ps.allSatisfy { $0.hasSuffix(".") })
    }
    func testRealPDFTextPerPageAndNoTextPDF() throws {
        let d = makePDF(["The harbour wall was cold.", "Lanterns along the quay."])
        let r = try DocExtract.pdf(d)
        XCTAssertEqual(r.pages.count, 2)
        XCTAssertTrue(r.pages[1].contains("Lanterns"), r.pages[1])
        let dir = tmpDir(); defer { try? FileManager.default.removeItem(at: dir) }
        let lib = ReaderShelf(root: dir.appendingPathComponent("lib"))
        let blank = write(makePDF(["", ""]), "scan.pdf", in: dir)
        XCTAssertThrowsError(try lib.importFile(at: blank)) { XCTAssertEqual($0 as? ShelfError, .noText) }
        XCTAssertTrue(lib.list().isEmpty)
    }
}

final class ShelfStoreTests: XCTestCase {
    func make() -> (ReaderShelf, URL) { let d = tmpDir(); return (ReaderShelf(root: d.appendingPathComponent("lib")), d) }

    func testImportEpubTwiceIsOneDocument() throws {
        let (lib, dir) = make(); defer { try? FileManager.default.removeItem(at: dir) }
        let f = write(book(deflate: true), "t.epub", in: dir)
        let a = try lib.importFile(at: f), b = try lib.importFile(at: f)
        XCTAssertEqual(a.id, b.id); XCTAssertEqual(lib.list().count, 1)
        XCTAssertEqual(a.title, "The Test Book"); XCTAssertEqual(a.kind, "epub"); XCTAssertEqual(a.sections, 2)
        XCTAssertNotNil(lib.fileURL(a.id))
    }
    func testImportPDFAndRefuseWrongThings() throws {
        let (lib, dir) = make(); defer { try? FileManager.default.removeItem(at: dir) }
        let d = try lib.importFile(at: write(makePDF(["Moon over water."]), "My_Notes.pdf", in: dir))
        XCTAssertEqual(d.kind, "pdf"); XCTAssertEqual(d.title, "My Notes"); XCTAssertEqual(d.length, 1)
        XCTAssertThrowsError(try lib.importFile(at: write(Data("hello".utf8), "a.txt", in: dir))) { XCTAssertEqual($0 as? ShelfError, .notSupported) }
        XCTAssertThrowsError(try lib.importFile(at: write(Data("hello".utf8), "a.pdf", in: dir)))
        XCTAssertThrowsError(try lib.importFile(at: write(Data("hello".utf8), "a.epub", in: dir)))
        XCTAssertEqual(lib.list().count, 1)
    }
    func testShelfFolderIsExcludedFromBackupAndFilesAreProtected() throws {
        let (lib, dir) = make(); defer { try? FileManager.default.removeItem(at: dir) }
        let root = dir.appendingPathComponent("lib")
        XCTAssertEqual(try root.resourceValues(forKeys: [.isExcludedFromBackupKey]).isExcludedFromBackup, true)
        let d = try lib.importFile(at: write(makePDF(["Moon over water."]), "n.pdf", in: dir))
        let original = try XCTUnwrap(lib.fileURL(d.id))
        // The flag is on the folder, so the copy, the text and the index inside it are left out of device backups with it.
        XCTAssertTrue(original.path.hasPrefix(root.path))
        XCTAssertEqual(try root.resourceValues(forKeys: [.isExcludedFromBackupKey]).isExcludedFromBackup, true)
        #if os(iOS)
        for u in [root, original.deletingLastPathComponent(), original, original.deletingLastPathComponent().appendingPathComponent("text.json")] {
            let a = try FileManager.default.attributesOfItem(atPath: u.path)
            XCTAssertEqual(a[.protectionKey] as? FileProtectionType, .completeUntilFirstUserAuthentication, u.lastPathComponent)
        }
        #endif
        XCTAssertTrue(ContentStore.writeOptions.contains(.completeFileProtectionUntilFirstUserAuthentication))
    }
    func testOpenDocumentsInMemoryAreBounded() throws {
        let (lib, dir) = make(); defer { try? FileManager.default.removeItem(at: dir) }
        var ids: [String] = []
        for i in 0..<(ReaderShelf.cacheLimit + 3) { ids.append(try lib.importFile(at: write(makePDF(["Page of book \(i)."]), "b\(i).pdf", in: dir)).id) }
        XCTAssertLessThanOrEqual(lib.cachedCount, ReaderShelf.cacheLimit)
        for id in ids { XCTAssertNotNil(lib.text(id), "an evicted document is read from disk again") }
        XCTAssertLessThanOrEqual(lib.cachedCount, ReaderShelf.cacheLimit)
    }
    func testProtectedEpubRefusedAndNothingStored() throws {
        let (lib, dir) = make(); defer { try? FileManager.default.removeItem(at: dir) }
        let drm = #"<encryption xmlns="urn:oasis:names:tc:opendocument:xmlns:container"><EncryptedData><EncryptionMethod Algorithm="urn:x:aes"/></EncryptedData></encryption>"#
        let f = write(book(extra: [("META-INF/encryption.xml", drm)]), "p.epub", in: dir)
        XCTAssertThrowsError(try lib.importFile(at: f)) { XCTAssertEqual($0 as? ShelfError, .protected) }
        XCTAssertTrue(lib.list().isEmpty)
    }
    func testPositionSaveClampAndRestore() throws {
        let (lib, dir) = make(); defer { try? FileManager.default.removeItem(at: dir) }
        let d = try lib.importFile(at: write(book(), "t.epub", in: dir))
        lib.setPosition(id: d.id, block: 3); XCTAssertEqual(lib.doc(d.id)?.position, 3)
        lib.setPosition(id: d.id, block: 999); XCTAssertEqual(lib.doc(d.id)?.position, d.length - 1)
        lib.setPosition(id: d.id, block: -4); XCTAssertEqual(lib.doc(d.id)?.position, 0)
        lib.setPosition(id: "nope", block: 1)
        lib.setPosition(id: d.id, block: 2)
        let again = ReaderShelf(root: dir.appendingPathComponent("lib"))
        XCTAssertEqual(again.doc(d.id)?.position, 2)
        XCTAssertGreaterThan(again.doc(d.id)!.progress, 0)
    }
    func testBlocksFromPositionForReadAloud() throws {
        let (lib, dir) = make(); defer { try? FileManager.default.removeItem(at: dir) }
        let d = try lib.importFile(at: write(book(), "t.epub", in: dir))
        let all = lib.blocks(id: d.id, from: 0)
        XCTAssertEqual(all.count, d.length)
        XCTAssertEqual(lib.blocks(id: d.id, from: 3), Array(all[3...]))
        XCTAssertEqual(lib.blocks(id: d.id, from: 500), [all.last!])
        XCTAssertEqual(lib.blocks(id: "nope", from: 0), [])
    }
    func testSearchCasePunctuationPhraseAndPrefix() throws {
        let (lib, dir) = make(); defer { try? FileManager.default.removeItem(at: dir) }
        let d = try lib.importFile(at: write(book(), "t.epub", in: dir))
        XCTAssertEqual(lib.find(in: d.id, query: "LANTERN").count, 1)
        XCTAssertEqual(lib.find(in: d.id, query: "cafe").count, 1)            // accent folded
        XCTAssertEqual(lib.find(in: d.id, query: "  harbour!! ").count, 1)
        XCTAssertEqual(lib.find(in: d.id, query: "harb").count, 1)             // prefix of the last word
        XCTAssertEqual(lib.find(in: d.id, query: "lit the").count, 1)          // both words present (not a phrase, still found)
        XCTAssertTrue(lib.find(in: d.id, query: "").isEmpty)
        XCTAssertTrue(lib.find(in: d.id, query: "?!").isEmpty)
        XCTAssertTrue(lib.find(in: d.id, query: "zebra").isEmpty)
        let p = lib.find(in: d.id, query: "second paragraph")[0]
        XCTAssertEqual(lib.text(d.id)!.blocks[p.block].text, p.text)
        XCTAssertTrue(p.snippet.lowercased().contains("second"))
    }
    func testPhraseRanksAboveScatteredWords() {
        let blocks = [DocBlock(kind: "paragraph", level: 0, text: "Dawn came late to the harbour", marker: "", section: 0),
                      DocBlock(kind: "paragraph", level: 0, text: "The harbour at dawn", marker: "", section: 0)]
        let idx = DocIndex.build(blocks)
        XCTAssertEqual(idx.search("harbour at dawn", in: blocks, limit: 5), [1])
        XCTAssertEqual(idx.search("dawn harbour", in: blocks, limit: 5), [0, 1])
    }
    func testPassagesAcrossReaderAndLauncherHits() throws {
        let (lib, dir) = make(); defer { try? FileManager.default.removeItem(at: dir) }
        let e = try lib.importFile(at: write(book(), "t.epub", in: dir))
        let p = try lib.importFile(at: write(makePDF(["The harbour of another city."]), "Port.pdf", in: dir))
        let ps = lib.passages(matching: "harbour", limit: 10)
        XCTAssertEqual(Set(ps.map(\.docId)), [e.id, p.id])
        XCTAssertEqual(lib.passages(matching: "harbour", limit: 1).count, 1)
        let byTitle = lib.hits(query: "writer")                   // author match
        XCTAssertEqual(byTitle.first?.address, "doc:\(e.id)")
        let body = lib.hits(query: "quoted words")
        XCTAssertTrue(body.contains { $0.address.hasPrefix("doc:\(e.id)@") })
        XCTAssertTrue(lib.hits(query: "").isEmpty)
        lib.remove(id: e.id)
        XCTAssertFalse(lib.passages(matching: "harbour", limit: 10).contains { $0.docId == e.id })
    }
    func testCorruptIndexFilesAreSkipped() throws {
        let (lib, dir) = make(); defer { try? FileManager.default.removeItem(at: dir) }
        let d = try lib.importFile(at: write(book(), "t.epub", in: dir))
        try Data("{".utf8).write(to: dir.appendingPathComponent("lib/\(d.id)/text.json"))
        let again = ReaderShelf(root: dir.appendingPathComponent("lib"))
        XCTAssertNil(again.text(d.id)); XCTAssertEqual(again.blocks(id: d.id, from: 0), [])
        try Data("garbage".utf8).write(to: dir.appendingPathComponent("lib/index.json"))
        XCTAssertTrue(ReaderShelf(root: dir.appendingPathComponent("lib")).list().isEmpty)
    }
    func testAddressesAndProviders() {
        XCTAssertEqual(ShelfAddress.parse("doc:abc@12")?.id, "abc"); XCTAssertEqual(ShelfAddress.parse("doc:abc@12")?.block, 12)
        XCTAssertNil(ShelfAddress.parse("doc:")); XCTAssertNil(ShelfAddress.parse("pub:x"))
        SearchProviders.register(name: "t") { q in [SearchHit(address: "doc:z", title: q, kind: "document", detail: "")] }
        SearchProviders.register(name: "t") { q in [SearchHit(address: "doc:y", title: q, kind: "document", detail: "")] }
        XCTAssertEqual(SearchProviders.hits(for: "k").map(\.address), ["doc:y"])
        XCTAssertTrue(ReaderShelf.canImport(URL(string: "https://example.com/a/b.PDF")!))
        XCTAssertFalse(ReaderShelf.canImport(URL(string: "https://example.com/a")!))
        XCTAssertFalse(ReaderShelf.canImport(URL(string: "ftp://example.com/a.pdf")!))
    }
}
