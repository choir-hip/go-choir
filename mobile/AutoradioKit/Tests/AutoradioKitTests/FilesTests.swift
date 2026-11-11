import XCTest
@testable import AutoradioKit

// Lane 5 tests. The ways each piece can fail are listed at the top of ChoirFiles.swift and Epub.swift; each test names the
// failure it covers. The demo files are the generated ones (scripts/make-files-fixtures.py).

private func demoData(_ key: String) -> Data { Data(base64Encoded: FilesDemoData.base64[key]!, options: .ignoreUnknownCharacters)! }

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

final class FilesPathTests: XCTestCase {
    // failure 1
    func testNormalizeRefusesEscapes() {
        XCTAssertEqual(FilePath.normalize("/a/b/../c"), "/a/c")
        XCTAssertEqual(FilePath.normalize("a//b/./c/"), "/a/b/c")
        XCTAssertEqual(FilePath.normalize(""), "/")
        XCTAssertNil(FilePath.normalize("/../etc"))
        XCTAssertNil(FilePath.normalize("/a/../../b"))
        XCTAssertNil(FilePath.normalize("/a\0b"))
    }
    // failure 2
    func testNames() {
        for bad in ["", " ", ".", "..", "a/b", " lead", "trail ", "nul\0"] { XCTAssertFalse(FilePath.validName(bad), bad) }
        XCTAssertFalse(FilePath.validName(String(repeating: "x", count: 256)))
        XCTAssertTrue(FilePath.validName("Budget 2026.csv"))
    }
    // failure 3
    func testEncodingAndCacheNames() {
        XCTAssertEqual(FilePath.encode("/My Docs/a?b#c%d.txt"), "/My%20Docs/a%3Fb%23c%25d.txt")
        XCTAssertEqual(FilePath.cacheName("Council minutes (final).PDF"), "Council-minutes--final-.pdf")
        XCTAssertEqual(FilePath.cacheName("日本語.png"), "file.png")
        XCTAssertEqual(FilePath.cacheName("???"), "file")
    }
    func testAddressesRoundTrip() {
        XCTAssertEqual(FilePath.address("/", isDirectory: true), "file:/")
        XCTAssertEqual(FilePath.address("/Docs", isDirectory: true), "file:/Docs/")
        XCTAssertEqual(FilePath.address("/Docs/a.pdf", isDirectory: false), "file:/Docs/a.pdf")
        XCTAssertEqual(FilePath.fromAddress("file:/Docs/")?.path, "/Docs")
        XCTAssertEqual(FilePath.fromAddress("file:/Docs/")?.isDirectory, true)
        XCTAssertEqual(FilePath.fromAddress("file:/Docs/a.pdf")?.isDirectory, false)
        XCTAssertEqual(FilePath.fromAddress("file:")?.path, "/")
        XCTAssertNil(FilePath.fromAddress("file:/../x"))
        XCTAssertNil(FilePath.fromAddress("mail:1"))
        XCTAssertEqual(FilePath.crumbs("/a/b").map(\.path), ["/", "/a", "/a/b"])
        XCTAssertEqual(PageAddress.kind(of: "file:/a/b.pdf"), "file")
    }
    func testKindsAndLabels() {
        XCTAssertEqual(FileTypes.kind(name: "A.PNG", isDirectory: false), .image)
        XCTAssertEqual(FileTypes.kind(name: "x.epub", isDirectory: false), .epub)
        XCTAssertEqual(FileTypes.kind(name: "x.svg", isDirectory: false), .code)
        XCTAssertEqual(FileTypes.kind(name: "x.mkv", isDirectory: false), .other)
        XCTAssertEqual(FileTypes.kind(name: "noext", isDirectory: false), .other)
        XCTAssertEqual(FileTypes.kind(name: "dir.png", isDirectory: true), .folder)
        XCTAssertEqual(FileLabels.size(0), "0 B"); XCTAssertEqual(FileLabels.size(999), "999 B"); XCTAssertEqual(FileLabels.size(1200), "1.2 kB")
        XCTAssertEqual(FileLabels.size(34_000), "34 kB"); XCTAssertEqual(FileLabels.size(5_600_000), "5.6 MB"); XCTAssertEqual(FileLabels.size(-5), "0 B")
        let now = Date(timeIntervalSince1970: 1_800_000_000)
        XCTAssertEqual(FileLabels.date(now, now: now), "Today")
        XCTAssertEqual(FileLabels.date(now.addingTimeInterval(-86400), now: now), "Yesterday")
        XCTAssertEqual(FileLabels.date(nil, now: now), "")
        XCTAssertTrue(FileLabels.date(now.addingTimeInterval(-86400 * 800), now: now).split(separator: " ").count == 3)
    }
    // failure 8
    func testSortIsTotalAndFoldersFirst() {
        let d = Date(timeIntervalSince1970: 1_800_000_000)
        func f(_ n: String, dir: Bool = false, size: Int64 = 1, age: Double = 0, at p: String = "/") -> FileEntry {
            FileEntry(name: n, path: FilePath.join(p, n), isDirectory: dir, size: size, modified: age < 0 ? nil : d.addingTimeInterval(-age))
        }
        let es = [f("10.txt"), f("2.txt"), f("b.txt"), f("b.txt", at: "/x"), f("Zeta", dir: true), f("alpha", dir: true)]
        XCTAssertEqual(FileSort.name.sorted(es, desc: false).map(\.name), ["alpha", "Zeta", "2.txt", "10.txt", "b.txt", "b.txt"])
        XCTAssertEqual(FileSort.name.sorted(es, desc: true).map(\.name).prefix(2), ["Zeta", "alpha"], "folders stay first when descending")
        let m = [f("old", age: 1000), f("new", age: 1), f("none", age: -1)]
        XCTAssertEqual(FileSort.modified.sorted(m, desc: true).map(\.name), ["new", "old", "none"])
        XCTAssertEqual(FileSort.size.sorted([f("big", size: 9), f("small", size: 1)], desc: false).map(\.name), ["small", "big"])
        XCTAssertEqual(FileSort.kind.sorted([f("a.txt"), f("b.png")], desc: false).map(\.name), ["b.png", "a.txt"])
    }
    func testCsv() {
        let r = Csv.parse("a,b,c\r\n1,\"x, y\",\"he said \"\"hi\"\"\"\n\n2,,\n3")
        XCTAssertEqual(r, [["a", "b", "c"], ["1", "x, y", "he said \"hi\""], ["2", "", ""], ["3"]])
        XCTAssertEqual(Csv.parse("a\tb", separator: "\t"), [["a", "b"]])
        XCTAssertEqual(Csv.parse("x,\"unterminated\nstill"), [["x", "unterminated\nstill"]])
        XCTAssertEqual(Csv.parse("1,2\n3,4\n5,6", maxRows: 2).count, 2)
        XCTAssertEqual(Csv.parse("a\"b,c"), [["a\"b", "c"]], "a stray quote in an unquoted field stays literal")
        XCTAssertTrue(Csv.parse("").isEmpty)
    }
}

final class EpubTests: XCTestCase {
    func testDemoBook() throws {
        let book = try Epub.parse(data: demoData("Books/The Lantern Keeper.epub"))
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
        let good = demoData("Books/The Lantern Keeper.epub")
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

final class FilesServiceTests: XCTestCase {
    var tmp: URL!
    var service: FilesService!
    var recents: RecentFiles!
    let now = Date()

    override func setUp() {
        tmp = FileManager.default.temporaryDirectory.appendingPathComponent("files-tests-\(UUID().uuidString)")
        try? FileManager.default.createDirectory(at: tmp, withIntermediateDirectories: true)
        recents = RecentFiles(file: tmp.appendingPathComponent("recent.json"))
        let n = now
        service = FilesService(backend: DemoFilesBackend(now: n), cacheRoot: tmp.appendingPathComponent("cache"), appPrefix: "app:/cache/files", recents: recents, now: { n })
    }
    override func tearDown() { try? FileManager.default.removeItem(at: tmp) }

    func testRootListing() async {
        let l = await service.listing(path: "/", mode: "browse", sort: "name", desc: false, query: "")
        XCTAssertEqual(l.status, "ready")
        XCTAssertEqual(l.rows.map(\.name), ["Books", "Documents", "Media", "Pictures", "Readme.txt"])
        XCTAssertEqual(l.rows[1].address, "file:/Documents/")
        XCTAssertEqual(l.rows[4].kind, "text")
        XCTAssertEqual(l.crumbs.map(\.name), ["Files"])
        let d = await service.listing(path: "/Documents", mode: "browse", sort: "modified", desc: true, query: "")
        XCTAssertEqual(d.crumbs.map(\.name), ["Files", "Documents"])
        XCTAssertEqual(d.rows.first?.name, "Drafts", "folders first")
        XCTAssertEqual(d.rows[1].name, "Harbor notes.md", "newest file first (today)")
        XCTAssertEqual(d.rows[1].modifiedLabel, "Today")
        XCTAssertEqual(d.rows.first(where: { $0.name == "Council minutes.pdf" })?.address, "file:/Documents/Council minutes.pdf")
    }

    // failure 7 and the empty folder
    func testMissingFolderIsAnError() async {
        let l = await service.listing(path: "/Nope", mode: "browse", sort: "name", desc: false, query: "")
        XCTAssertEqual(l.status, "error"); XCTAssertFalse(l.message.isEmpty)
        let e = await service.listing(path: "/Documents/Drafts", mode: "browse", sort: "name", desc: false, query: "")
        XCTAssertEqual(e.status, "empty"); XCTAssertEqual(e.message, "This folder is empty.")
    }

    // failure 9
    func testSearchWalksFolders() async {
        let l = await service.listing(path: "/", mode: "browse", sort: "name", desc: false, query: "LANTERN")
        XCTAssertEqual(l.mode, "search")
        XCTAssertEqual(l.rows.map(\.name), ["The Lantern Keeper.epub"])
        XCTAssertEqual(l.rows[0].place, "Books")
        let none = await service.listing(path: "/", mode: "browse", sort: "name", desc: false, query: "zzzz")
        XCTAssertEqual(none.status, "empty")
        let accent = await service.listing(path: "/", mode: "browse", sort: "name", desc: false, query: "harbör")
        XCTAssertGreaterThanOrEqual(accent.rows.count, 2, "accents are ignored")
    }

    func testOpenTextImageCsv() async throws {
        let md = await service.open(address: "file:/Documents/Harbor notes.md")
        XCTAssertEqual(md.status, "ready"); XCTAssertEqual(md.kind, "markdown"); XCTAssertTrue(md.text.hasPrefix("# Harbor notes"))
        let png = await service.open(address: "file:/Pictures/Harbor test card.png")
        XCTAssertEqual(png.kind, "image"); XCTAssertEqual(png.width, 640); XCTAssertEqual(png.height, 400)
        XCTAssertTrue(png.src.hasPrefix("app:/cache/files/") && png.src.hasSuffix("/Harbor-test-card.png"), png.src)
        XCTAssertTrue(FileManager.default.fileExists(atPath: png.local))
        let head = try Data(contentsOf: URL(fileURLWithPath: png.local)).prefix(4)
        XCTAssertEqual([UInt8](head), [0x89, 0x50, 0x4E, 0x47])
        let csv = await service.open(address: "file:/Documents/Budget 2026.csv")
        XCTAssertEqual(csv.rows.first?.cells, ["Quarter", "Item", "Planned", "Actual", "Note"])
        XCTAssertEqual(csv.rows[4].cells[1], "Benches, east side")
        XCTAssertEqual(csv.rows[4].cells[4], "Oak, \"harbour grade\"")
        for (path, kind) in [("Media/Tone 440.wav", "audio"), ("Media/Pattern.mp4", "video")] {
            let m = await service.open(address: "file:/" + path)
            XCTAssertEqual(m.kind, kind); XCTAssertEqual(m.status, "ready"); XCTAssertFalse(m.src.isEmpty)
        }
    }

    func testOtherKindsAreFactsOnly() async {
        let o = await service.open(address: "file:/Documents/Survey readings.dat")
        XCTAssertEqual(o.status, "ready"); XCTAssertEqual(o.kind, "other"); XCTAssertEqual(o.src, ""); XCTAssertEqual(o.sizeLabel, "512 B")
        XCTAssertEqual(o.kindLabel, "File"); XCTAssertEqual(o.folder, "/Documents")
    }

    func testFolderAddressAndMissingFile() async {
        let f = await service.open(address: "file:/Documents/")
        XCTAssertEqual(f.status, "folder")
        let m = await service.open(address: "file:/Documents/gone.pdf")
        XCTAssertEqual(m.status, "notfound")
        let bad = await service.open(address: "file:/../x.pdf")
        XCTAssertEqual(bad.status, "error")
    }

    // failure 5
    func testTooLargeIsRefusedBeforeDownload() async {
        final class Big: FilesBackend, @unchecked Sendable {
            var fetched = false
            func list(_ path: String) async throws -> [FileEntry] { [FileEntry(name: "huge.mp4", path: "/huge.mp4", isDirectory: false, size: 5_000_000_000, modified: nil)] }
            func fetch(_ path: String, size: Int64, to dest: URL, maxBytes: Int64) async throws { fetched = true }
            func put(_ path: String, from file: URL) async throws {}
            func mkdir(_ path: String) async throws {}
            func delete(_ path: String) async throws {}
            func move(_ from: String, to: String) async throws {}
        }
        let big = Big()
        let s = FilesService(backend: big, cacheRoot: tmp.appendingPathComponent("c2"), appPrefix: "app:/cache/files", recents: RecentFiles(file: nil))
        let i = await s.open(address: "file:/huge.mp4")
        XCTAssertEqual(i.status, "toolarge"); XCTAssertFalse(big.fetched); XCTAssertTrue(i.message.contains("5.0 GB") || i.message.contains("5 GB"), i.message)
    }

    // failure 4: a changed file is a different cache entry
    func testCacheKeyChangesWithTheFile() async throws {
        let a = await service.open(address: "file:/Documents/Key list.txt")
        let tmpFile = tmp.appendingPathComponent("new.txt"); try Data("changed\n".utf8).write(to: tmpFile)
        _ = await service.upload(from: tmpFile, name: "Key list.txt", into: "/Documents")
        let b = await service.open(address: "file:/Documents/Key list.txt")
        XCTAssertNotEqual(a.src, b.src); XCTAssertEqual(b.text, "changed\n")
    }

    func testEpubAndChapters() async {
        let i = await service.open(address: "file:/Books/The Lantern Keeper.epub")
        XCTAssertEqual(i.status, "ready"); XCTAssertEqual(i.title, "The Lantern Keeper"); XCTAssertEqual(i.author, "A. Ferris (invented)")
        XCTAssertEqual(i.chapters.count, 3)
        let c = await service.chapter(address: i.address, index: 1)
        XCTAssertEqual(c.title, "Three visitors"); XCTAssertEqual(c.count, 3); XCTAssertTrue(c.markdown.contains("What the boy asked"))
        let past = await service.chapter(address: i.address, index: 99)
        XCTAssertEqual(past.index, 2, "an index past the end clamps to the last chapter")
        let none = await service.chapter(address: "file:/Documents/Key list.txt", index: 0)
        XCTAssertEqual(none.status, "error")
    }

    #if canImport(PDFKit)
    func testPdfPagesAndSearch() async {
        let i = await service.open(address: "file:/Documents/Council minutes.pdf")
        XCTAssertEqual(i.status, "ready"); XCTAssertEqual(i.pages, 3); XCTAssertEqual(i.title, "Harbor Council minutes")
        let hit = await service.findPages(address: i.address, query: "lamps")
        XCTAssertEqual(hit.pages, [2])
        let none = await service.findPages(address: i.address, query: "unicorn")
        XCTAssertEqual(none.pages, []); XCTAssertFalse(none.message.isEmpty)
        let empty = await service.findPages(address: i.address, query: "  ")
        XCTAssertEqual(empty.pages, [])
    }
    #endif

    // failures 2, 11, 12
    func testChanges() async throws {
        var l = await service.makeFolder(in: "/", name: "Projects")
        XCTAssertTrue(l.rows.contains { $0.name == "Projects" }); XCTAssertEqual(l.message, "Made the folder \u{201C}Projects\u{201D}.")
        l = await service.makeFolder(in: "/", name: "Projects")
        XCTAssertEqual(l.message, FilesError.exists.description)
        l = await service.makeFolder(in: "/", name: "a/b")
        XCTAssertEqual(l.message, FilesError.badName.description)
        l = await service.rename(path: "/Readme.txt", to: "About.txt")
        XCTAssertTrue(l.rows.contains { $0.name == "About.txt" } && !l.rows.contains { $0.name == "Readme.txt" })
        l = await service.rename(path: "/About.txt", to: "Books")
        XCTAssertEqual(l.message, FilesError.exists.description)
        l = await service.rename(path: "/Books", to: "Library")
        XCTAssertEqual(l.message, "Folders cannot be renamed here yet.")
        l = await service.rename(path: "/About.txt", to: "")
        XCTAssertEqual(l.message, FilesError.badName.description)
        l = await service.delete(path: "/Media")
        XCTAssertFalse(l.rows.contains { $0.name == "Media" })
        let gone = await service.open(address: "file:/Media/Tone 440.wav")
        XCTAssertEqual(gone.status, "notfound", "deleting a folder deletes what is in it")
        l = await service.delete(path: "/")
        XCTAssertEqual(l.message, "That cannot be deleted.")
        l = await service.delete(path: "/nothing")
        XCTAssertEqual(l.message, FilesError.notFound.description)
    }

    // failure 10
    func testRecents() async {
        _ = await service.open(address: "file:/Documents/Key list.txt")
        _ = await service.open(address: "file:/Pictures/Harbor test card.png")
        _ = await service.open(address: "file:/Documents/Key list.txt")
        var r = await service.listing(path: "/", mode: "recent", sort: "name", desc: false, query: "")
        XCTAssertEqual(r.rows.map(\.name), ["Key list.txt", "Harbor test card.png"])
        XCTAssertEqual(r.rows[1].place, "Pictures")
        XCTAssertEqual(RecentFiles(file: tmp.appendingPathComponent("recent.json")).list().count, 2, "kept on disk")
        _ = await service.delete(path: "/Pictures")
        _ = await service.open(address: "file:/Pictures/Harbor test card.png")
        r = await service.listing(path: "/", mode: "recent", sort: "name", desc: false, query: "")
        XCTAssertEqual(r.rows.map(\.name), ["Key list.txt"])
        let empty = await FilesService(backend: DemoFilesBackend(), cacheRoot: tmp, appPrefix: "app:/cache/files", recents: RecentFiles(file: nil))
            .listing(path: "/", mode: "recent", sort: "name", desc: false, query: "")
        XCTAssertEqual(empty.status, "empty"); XCTAssertEqual(empty.message, "Files you open appear here.")
    }

    func testRecentsAreCapped() {
        let r = RecentFiles(file: nil)
        for i in 0..<45 { r.touch(FileEntry(name: "f\(i)", path: "/f\(i)", isDirectory: false, size: 1, modified: nil)) }
        XCTAssertEqual(r.list().count, RecentFiles.limit); XCTAssertEqual(r.list().first?.path, "/f44")
    }
}

final class ChoirFilesBackendTests: XCTestCase {
    var env: ChoirEnvironment!
    override func setUp() {
        let cfg = URLSessionConfiguration.ephemeral
        cfg.protocolClasses = [StubProtocol.self]
        env = ChoirEnvironment()
        env.keys = InMemoryKeyStore([KeyAccount.apiKey: secretKey, KeyAccount.computerID: "comp-1"])
        env.session = URLSession(configuration: cfg)
        StubProtocol.seen = []
        StubProtocol.handler = { _ in .init() }
    }

    func testListRequestsAndDecoding() async throws {
        StubProtocol.handler = { _ in .init(json: [["name": "a b.pdf", "type": "file", "size": 12, "modified": "2026-11-03T10:00:00Z"],
                                                   ["name": "Sub", "type": "directory", "size": 0, "modified": "2026-11-01T09:30:00Z"],
                                                   ["name": "bad/name", "type": "file", "size": 1, "modified": ""]]) }
        let b = ChoirFilesBackend(env: env)
        let root = try await b.list("/")
        XCTAssertEqual(root.map(\.name), ["a b.pdf", "Sub"], "an entry whose name is a path is dropped")
        XCTAssertEqual(root[0].path, "/a b.pdf"); XCTAssertEqual(root[0].size, 12); XCTAssertTrue(root[1].isDirectory)
        XCTAssertNotNil(root[0].modified)
        _ = try await b.list("/Docs/a b?.txt")
        let paths = StubProtocol.seen.map(\.path)
        XCTAssertEqual(paths, ["/api/files", "/api/files/Docs/a b?.txt"])
        XCTAssertEqual(StubProtocol.seen[1].headers["authorization"], "Bearer \(secretKey)")
        XCTAssertEqual(StubProtocol.seen[1].headers["x-choir-computer"], "comp-1")
    }

    func testMutationsUseTheRightVerbs() async throws {
        let b = ChoirFilesBackend(env: env)
        try await b.mkdir("/New"); try await b.delete("/Old.txt"); try await b.move("/a.txt", to: "/b.txt")
        XCTAssertEqual(StubProtocol.seen.map { "\($0.method) \($0.path)" }, ["POST /api/files/New", "DELETE /api/files/Old.txt", "PATCH /api/files/a.txt"])
        XCTAssertEqual(StubProtocol.seen[2].body["move_to"] as? String, "/b.txt")
    }

    func testErrorsAreTyped() async {
        let b = ChoirFilesBackend(env: env)
        let notFound: (Error) -> Bool = { ($0 as? FilesError) == .notFound }
        let exists: (Error) -> Bool = { ($0 as? FilesError) == .exists }
        let unsupported: (Error) -> Bool = { if case .unsupported = ($0 as? FilesError) ?? .notFound { return true }; return false }
        let unauthorized: (Error) -> Bool = { ($0 as? ChoirError) == .unauthorized }
        let cases: [(Int, (Error) -> Bool)] = [(404, notFound), (409, exists), (405, unsupported), (401, unauthorized)]
        for (status, check) in cases {
            StubProtocol.handler = { _ in .init(status: status, json: ["error": "x"]) }
            do { _ = try await b.list("/"); XCTFail("expected an error for \(status)") } catch { XCTAssertTrue(check(error), "\(status): \(error)") }
        }
    }

    func testNoKeyMeansNotSignedIn() async {
        env.keys = InMemoryKeyStore()
        do { _ = try await ChoirFilesBackend(env: env).list("/"); XCTFail() } catch { XCTAssertEqual(error as? ChoirError, .notSignedIn) }
        XCTAssertTrue(StubProtocol.seen.isEmpty, "no request without a key")
    }
}
