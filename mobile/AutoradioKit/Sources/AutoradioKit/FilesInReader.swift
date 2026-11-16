import Foundation

// Opening a PDF or an e-book from the Files place (owner, 2026-11-16: "There's only one way to do something in autoradio app"). A document is a
// Reader document wherever it came from: Files has no PDF or e-book viewer. A Files entry becomes a Reader document by being taken into the
// Reader's library once, keyed by its Files path and its etag (size and modified time):
//   - the first open downloads the file (FilesService.download) and imports it through the Reader's own import (`ReaderShelf.importFile`);
//   - the document is marked as coming from that path (`ShelfDoc.origin`, `etag`), and the Reader's list says "From Files";
//   - opening the same file again finds it by path and etag and downloads nothing, so the library does not grow;
//   - a changed file (another etag) is imported again and replaces the old copy for that path;
//   - the same bytes already in the library (uploaded from iCloud, say) are the same document: it is adopted, not duplicated.
// Ways this can fail (each has a test in FilesInReaderTests): a second open adds a row; a changed file leaves two copies; a file too large or
// unreadable opens nothing and says why in the bar's line; a folder or another kind of file is not routed here.

public struct FilesInReader: Sendable {
    /// The app's one (the host sets it when Files is registered).
    nonisolated(unsafe) public static var current: FilesInReader?
    public let files: FilesService
    public let shelf: @Sendable () -> ReaderShelf
    public init(files: FilesService, shelf: @escaping @Sendable () -> ReaderShelf = { ReaderShelf.shared }) { self.files = files; self.shelf = shelf }

    /// A `file:` address that names a PDF or an e-book: these open in Reader, nowhere else.
    public static func opensInReader(_ address: String) -> Bool {
        guard let (path, isDir) = FilePath.fromAddress(address), !isDir else { return false }
        let k = FileTypes.kind(name: FilePath.name(path), isDirectory: false)
        return k == .pdf || k == .epub
    }

    public static func etag(_ e: FileEntry) -> String { "\(e.size)|\(e.modified.map { String($0.timeIntervalSince1970) } ?? "")" }

    /// The Reader document for the Files address; throws what the Reader says when it cannot be opened.
    public func open(address: String) async throws -> ShelfDoc {
        guard Self.opensInReader(address), let (path, _) = FilePath.fromAddress(address) else { throw ShelfError.notSupported }
        guard let entry = await files.entry(path: path) else { throw ShelfError.unreadable(FilesError.notFound.description) }
        let tag = Self.etag(entry)
        let lib = shelf()
        if let d = lib.doc(origin: path), d.etag == tag { files.noteOpened(entry); return d }
        let url: URL
        do { url = try await files.download(entry, maxBytes: Int64(ReaderShelf.maxBytes)) }
        catch let e as FilesError { if case .tooLarge = e { throw ShelfError.tooLarge }; throw ShelfError.unreadable(e.description) }
        catch { throw ShelfError.download }
        let doc = try await Task.detached { try lib.importFile(at: url, name: entry.name) }.value
        files.noteOpened(entry)
        return lib.adopt(id: doc.id, origin: path, etag: tag) ?? doc
    }

    /// An audio file in Files as a run sheet entry (brought to disk first); nil for anything else or a file that cannot be had.
    public func audioEntry(address: String) async -> RunSheetEntry? {
        let i = await files.open(address: address)
        guard i.status == "ready", i.kind == "audio", !i.local.isEmpty else { return nil }
        let name = (i.name as NSString).deletingPathExtension
        return .audio(RunSheetAudio(url: URL(fileURLWithPath: i.local), title: name, show: "Files"))
    }
}
