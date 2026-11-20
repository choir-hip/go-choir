import Foundation
@testable import AutoradioKit

// A short sample book for the shelf's tests: original text about using Reader, built as a real EPUB (stored ZIP entries) and put on the shelf
// through the shelf's own import path, so the row, the page, contents, find and the blocks all run the real code.

enum ReaderSample {
    static let fileName = "Using Reader.epub"
    /// The sample's document id (the shelf's hash of its bytes); `Demo.sampleBookID` is the same id, which the leftover cleanup removes.
    static let id = Demo.sampleBookID

    static let chapters: [(title: String, paragraphs: [String])] = [
        ("Adding a book", [
            "Reader keeps your books and documents on this phone. Tap Add on the Reader page and choose an EPUB or a PDF from Files, and it appears on your shelf within a moment.",
            "A book is opened once and then remembered. Reader stores its own copy, reads out the words, and builds a small index so that you can search it later without a connection.",
            "Some books are protected by their publisher. Reader cannot open those, and it will say so plainly rather than show you a blank page.",
            "A scanned PDF has pictures of pages but no words behind them. Reader will tell you that it has no text to read, because it cannot read aloud what it cannot find.",
        ]),
        ("Reading on the screen", [
            "Each chapter is shown as a column of paragraphs. Tap a paragraph to mark where you are, and the mark stays when you turn the page or close the book.",
            "Contents lists every chapter. Choose one and you are taken to its first line. Previous and Next move you along one chapter at a time.",
            "A PDF can be shown as its real pages or as plain text. The pages keep the layout the author chose; the text is easier to search and to listen to.",
            "Your place is saved as you read, so the next time you open the book it is there at the paragraph where you left off.",
        ]),
        ("Listening and finding", [
            "Read aloud begins at the paragraph you marked, or at the start of the chapter if you have marked none. The paragraph being spoken is lit as the voice moves along.",
            "Add to queue puts the book behind whatever is playing now, so it starts by itself when the current item ends. You can keep reading something else in the meantime.",
            "Find looks inside the open book for a word or a phrase and lists each place it occurs. Choose a result to jump straight to that paragraph.",
            "The search field in the launcher looks across every book on your shelf, in the titles and in the words themselves, so a half-remembered sentence is usually enough.",
        ]),
    ]

    private static func xhtml(_ title: String, _ paragraphs: [String]) -> String {
        "<?xml version=\"1.0\" encoding=\"utf-8\"?><html xmlns=\"http://www.w3.org/1999/xhtml\"><head><title>\(title)</title></head><body><h1>\(title)</h1>"
            + paragraphs.map { "<p>\($0)</p>" }.joined() + "</body></html>"
    }

    /// The sample as the bytes of an EPUB file.
    public static func epubData() -> Data {
        let container = "<?xml version=\"1.0\"?><container version=\"1.0\" xmlns=\"urn:oasis:names:tc:opendocument:xmlns:container\"><rootfiles><rootfile full-path=\"OPS/book.opf\" media-type=\"application/oebps-package+xml\"/></rootfiles></container>"
        let items = chapters.indices.map { "<item id=\"c\($0)\" href=\"c\($0).xhtml\" media-type=\"application/xhtml+xml\"/>" }.joined()
        let refs = chapters.indices.map { "<itemref idref=\"c\($0)\"/>" }.joined()
        let opf = "<?xml version=\"1.0\"?><package xmlns=\"http://www.idpf.org/2007/opf\" version=\"3.0\"><metadata xmlns:dc=\"http://purl.org/dc/elements/1.1/\"><dc:title>Using Reader</dc:title><dc:creator>The Autoradio team</dc:creator><dc:language>en</dc:language></metadata><manifest>\(items)</manifest><spine>\(refs)</spine></package>"
        var files: [(String, String)] = [("mimetype", "application/epub+zip"), ("META-INF/container.xml", container), ("OPS/book.opf", opf)]
        for (i, c) in chapters.enumerated() { files.append(("OPS/c\(i).xhtml", xhtml(c.title, c.paragraphs))) }
        return storedZip(files)
    }

    /// A ZIP with stored (uncompressed) entries.
    static func storedZip(_ files: [(String, String)]) -> Data {
        func le16(_ v: Int) -> [UInt8] { [UInt8(v & 255), UInt8((v >> 8) & 255)] }
        func le32(_ v: Int) -> [UInt8] { [UInt8(v & 255), UInt8((v >> 8) & 255), UInt8((v >> 16) & 255), UInt8((v >> 24) & 255)] }
        var out = Data(), central = Data()
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
}

extension ReaderShelf {
    /// Put the sample book on the shelf (through the shelf's own import; a second call finds it already there).
    @discardableResult
    func seedSample() -> ShelfDoc? {
        let tmp = FileManager.default.temporaryDirectory.appendingPathComponent("reader-sample-\(UUID().uuidString)")
        try? FileManager.default.createDirectory(at: tmp, withIntermediateDirectories: true)
        defer { try? FileManager.default.removeItem(at: tmp) }
        let f = tmp.appendingPathComponent(ReaderSample.fileName)
        guard (try? ReaderSample.epubData().write(to: f)) != nil else { return nil }
        return try? importFile(at: f, name: ReaderSample.fileName)
    }
}
