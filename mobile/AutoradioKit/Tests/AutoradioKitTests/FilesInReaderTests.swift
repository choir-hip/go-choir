import XCTest
@testable import AutoradioKit

// One way to open a document (owner, 2026-11-16): a PDF or an e-book opened from Files is a Reader document. The failure list:
//  1 a .pdf or .epub address is routed to the Reader and nothing else is (a folder, text, audio are not)
//  2 the Reader document that comes back is a ShelfDoc of the right kind, marked as from its Files path
//  3 opening the same file again adds no second document and downloads nothing
//  4 a file that changed replaces its copy (one document for the path), and the same bytes already in the library are one document, not two
//  5 a file that cannot be had opens nothing and says why
//  6 no symbol of the old Files PDF or e-book viewer remains in the sources, the contracts or the interface
//  7 an audio file in Files is a run sheet entry (Play and Add to queue), not a player of its own

final class FilesInReaderTests: XCTestCase {
    var tmp: URL!
    var shelf: ReaderShelf!
    var service: FilesService!
    var reader: FilesInReader!
    let now = Date()

    override func setUp() {
        tmp = FileManager.default.temporaryDirectory.appendingPathComponent("fir-\(UUID().uuidString)")
        try? FileManager.default.createDirectory(at: tmp, withIntermediateDirectories: true)
        let n = now
        service = FilesService(backend: DemoFilesBackend(now: n), cacheRoot: tmp.appendingPathComponent("cache"), appPrefix: "app:/cache/files",
                               recents: RecentFiles(file: tmp.appendingPathComponent("recent.json")), now: { n })
        shelf = ReaderShelf(root: tmp.appendingPathComponent("books"))
        let s = shelf!
        reader = FilesInReader(files: service, shelf: { s })
    }
    override func tearDown() { try? FileManager.default.removeItem(at: tmp) }

    func test1_OnlyAPdfOrAnEpubIsRoutedToTheReader() {
        XCTAssertTrue(FilesInReader.opensInReader("file:/Documents/Council minutes.pdf"))
        XCTAssertTrue(FilesInReader.opensInReader("file:/Books/The Lantern Keeper.epub"))
        XCTAssertTrue(FilesInReader.opensInReader("file:/Books/Upper.PDF"))
        XCTAssertFalse(FilesInReader.opensInReader("file:/Documents/"))
        XCTAssertFalse(FilesInReader.opensInReader("file:/Documents/Key list.txt"))
        XCTAssertFalse(FilesInReader.opensInReader("file:/Media/Tone 440.wav"))
        XCTAssertFalse(FilesInReader.opensInReader("doc:abc"))
    }

    #if canImport(PDFKit)
    func test2_AFilesPdfAndEpubAreReaderDocumentsFromFiles() async throws {
        let pdf = try await reader.open(address: "file:/Documents/Council minutes.pdf")
        XCTAssertEqual(pdf.kind, "pdf"); XCTAssertEqual(pdf.origin, "/Documents/Council minutes.pdf"); XCTAssertTrue(pdf.address.hasPrefix("doc:"))
        XCTAssertEqual(shelf.doc(pdf.id)?.origin, "/Documents/Council minutes.pdf")
        XCTAssertNotNil(shelf.text(pdf.id), "the Reader has its text: it can be read aloud, searched and listed by contents")
        let epub = try await reader.open(address: "file:/Books/The Lantern Keeper.epub")
        XCTAssertEqual(epub.kind, "epub"); XCTAssertEqual(epub.title, "The Lantern Keeper")
        XCTAssertEqual(shelf.list().count, 2)
    }

    func test3_OpeningItAgainAddsNothingAndDownloadsNothing() async throws {
        let a = try await reader.open(address: "file:/Documents/Council minutes.pdf")
        let cache = tmp.appendingPathComponent("cache")
        let before = (try? FileManager.default.subpathsOfDirectory(atPath: cache.path))?.count ?? 0
        try? FileManager.default.removeItem(at: cache)                                         // if it were fetched again, it would reappear
        let b = try await reader.open(address: "file:/Documents/Council minutes.pdf")
        XCTAssertEqual(a.id, b.id); XCTAssertEqual(shelf.list().count, 1)
        XCTAssertFalse(FileManager.default.fileExists(atPath: cache.path), "nothing was downloaded the second time (had \(before) cached files)")
    }

    func test4_AChangedFileReplacesItsCopyAndTheSameBytesAreOneDocument() async throws {
        let a = try await reader.open(address: "file:/Documents/Council minutes.pdf")
        // the same bytes arrive by another road first (iCloud upload): one document
        let copy = tmp.appendingPathComponent("again.pdf")
        try FileManager.default.copyItem(at: try XCTUnwrap(shelf.fileURL(a.id)), to: copy)
        XCTAssertEqual(try shelf.importFile(at: copy).id, a.id, "the same bytes are the same document")
        XCTAssertEqual(shelf.list().count, 1)
        // the file on the computer is replaced by a different one: the old copy goes, the new one is the path's
        let epubBytes = try XCTUnwrap(shelf.doc(a.id)).fileName            // placeholder to keep the name used below
        _ = epubBytes
        let other = try await reader.open(address: "file:/Books/The Lantern Keeper.epub")
        let swapped = try XCTUnwrap(shelf.adopt(id: other.id, origin: "/Documents/Council minutes.pdf", etag: "new"))
        XCTAssertEqual(swapped.id, other.id)
        XCTAssertNil(shelf.doc(a.id), "the changed file replaced the old copy for that path")
        XCTAssertEqual(shelf.list().filter { $0.origin == "/Documents/Council minutes.pdf" }.count, 1)
    }
    #endif

    func test5_AFileThatIsNotThereOpensNothingAndSaysWhy() async {
        do { _ = try await reader.open(address: "file:/Documents/gone.pdf"); XCTFail("opened") }
        catch { XCTAssertEqual((error as? ShelfError)?.description, FilesError.notFound.description) }
        XCTAssertTrue(shelf.list().isEmpty)
        do { _ = try await reader.open(address: "file:/Documents/Key list.txt"); XCTFail("opened") } catch { XCTAssertEqual(error as? ShelfError, .notSupported) }
    }

    func test6_NoSymbolOfTheOldFilesPdfOrEpubViewerRemains() throws {
        let mobile = URL(fileURLWithPath: #filePath).deletingLastPathComponent().deletingLastPathComponent().deletingLastPathComponent().deletingLastPathComponent()
        let fm = FileManager.default
        var hay = ""
        for dir in ["AutoradioKit/Sources", "AutoradioLab", "exact-package/interface", "exact-package/apple/AutoradioModule+Files.swift"] {
            let u = mobile.appendingPathComponent(dir)
            var isDir: ObjCBool = false
            guard fm.fileExists(atPath: u.path, isDirectory: &isDir) else { continue }
            if !isDir.boolValue { hay += (try? String(contentsOf: u, encoding: .utf8)) ?? ""; continue }
            for case let f as URL in fm.enumerator(at: u, includingPropertiesForKeys: nil) ?? NSEnumerator() where ["swift", "contract", "json"].contains(f.pathExtension) {
                if f.lastPathComponent.hasSuffix(".generated.swift") { continue }
                hay += (try? String(contentsOf: f, encoding: .utf8)) ?? ""
            }
        }
        for gone in ["PdfViewer", "EpubViewer", "viewer-pdf", "pdf-frame", "epub-text", "fileChapter", "filePages", "FilePages", "FileChapter", "fvChapter", "fvPages", "Find in this PDF", "TextViewer", "viewer-text", "text-share"] {
            XCTAssertFalse(hay.contains(gone), "\(gone) is a symbol of the deleted Files viewer")
        }
    }

    func test7_AnAudioFileInFilesIsARunSheetEntryNamedByItsFilesPathAndNeverByTheDeviceCopy() async throws {
        let got = await reader.audioEntry(address: "file:/Media/Tone 440.wav")
        let (e, local) = try XCTUnwrap(got)
        XCTAssertEqual(e.kind, .episode); XCTAssertEqual(e.title, "Tone 440"); XCTAssertEqual(e.source?.ref, "file:/Media/Tone 440.wav"); XCTAssertNil(e.source?.url)
        let none = await reader.audioEntry(address: "file:/Documents/Key list.txt")
        XCTAssertNil(none)
        let eng = await MainActor.run { FakeEngine() }
        let (posted, started): ([RunSheetOp], [String]) = await MainActor.run {
            let s = RunSheetStore(); let p = RunSheetPlayer(store: s); p.engine = eng
            p.localFiles[e.id] = local
            p.playNow(e)
            return (s.pending, [])
        }
        _ = started
        XCTAssertEqual(posted.count, 1)
        let json = String(decoding: try JSONEncoder().encode(posted[0]), as: UTF8.self)
        XCTAssertFalse(json.contains("file://")); XCTAssertFalse(json.contains(tmp.path)); XCTAssertFalse(json.contains(local.path))
        XCTAssertTrue(json.contains("Media") && json.contains("Tone 440.wav"), "it names the file by its Files path")
        // after a relaunch the phone's copy is not remembered: the Files path is resolved again (and fetched if the copy is gone)
        let resolved = XCTestExpectation(description: "resolved")
        let engine2 = await MainActor.run { FakeEngine() }
        let url = local
        await MainActor.run {
            let p2 = RunSheetPlayer(store: RunSheetStore()); p2.engine = engine2
            p2.fileResolver = { addr in XCTAssertEqual(addr, "file:/Media/Tone 440.wav"); resolved.fulfill(); return url }
            p2.playNow(e)
        }
        await fulfillment(of: [resolved], timeout: 5)
        try await Task.sleep(nanoseconds: 100_000_000)
        let log = await MainActor.run { engine2.log }
        XCTAssertTrue(log.contains { $0.hasPrefix("remote:") }, "\(log)")
    }

    func test8_NoOpTheStorePostsEverNamesAPlaceOnThePhone() async {
        await MainActor.run {
            let s = RunSheetStore(); let p = RunSheetPlayer(store: s); p.engine = FakeEngine()
            let bad = RunSheetEntry.webPage(url: URL(string: "file:///var/mobile/Containers/x.html")!, title: "x")
            XCTAssertEqual(s.place(bad, at: .end).dropped, "local_path", "a file URL is refused before it is applied or posted")
            var home = RunSheetEntry.paperStory(route: "r", title: "T"); home.title = NSHomeDirectory() + "/Documents/a"
            XCTAssertEqual(s.place(home, at: .end).dropped, "local_path")
            // every kind of thing the listener can place leaves nothing local in what is posted
            p.playNow(.paperStory(route: "a", title: "A")); p.queue(.webPage(url: URL(string: "https://x.test/p")!, title: "P")); p.queue(.book(id: "abc", title: "B", at: 3))
            p.queue(.readAloud(address: "mail:m1", title: "M")); p.queue(.audio(RunSheetAudio(url: URL(string: "https://x.test/a.mp3")!, title: "T", show: "S")))
            p.queue(.filesAudio(address: "file:/Media/a.wav", title: "A"))
            XCTAssertGreaterThanOrEqual(s.pending.count, 6)
            for op in s.pending {
                let j = String(decoding: (try? JSONEncoder().encode(op)) ?? Data(), as: UTF8.self)
                XCTAssertFalse(j.contains("file://"), j); XCTAssertFalse(j.contains(NSHomeDirectory()), j); XCTAssertFalse(j.contains(FileManager.default.temporaryDirectory.path), j)
            }
        }
    }
}
