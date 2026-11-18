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
    public func open(address: String) async throws -> ShelfDoc { try await openReporting(address: address).doc }

    /// The same, with where the bytes came from ("cache": the Reader already held this exact file; "network": fetched from the computer now, or
    /// "cache" when the Files download cache had it) and how many.
    public func openReporting(address: String) async throws -> (doc: ShelfDoc, from: String, bytes: Int64) {
        guard Self.opensInReader(address), let (path, _) = FilePath.fromAddress(address) else { throw ShelfError.notSupported }
        guard let entry = await files.entry(path: path) else { throw ShelfError.unreadable(FilesError.notFound.description) }
        let tag = Self.etag(entry)
        let lib = shelf()
        // A library row whose original file is gone from disk is not "had": the page would draw nothing. It is fetched and taken in again.
        if let d = lib.doc(origin: path), d.etag == tag, lib.fileURL(d.id) != nil, lib.text(d.id) != nil { files.noteOpened(entry); return (d, "cache", entry.size) }
        if let d = lib.doc(origin: path), lib.fileURL(d.id) == nil { lib.remove(id: d.id) }          // the same bytes would be "already in the library" and the original never restored
        let from = files.isCached(entry) ? "cache" : "network"
        let url: URL
        do { url = try await files.download(entry, maxBytes: Int64(ReaderShelf.maxBytes)) }
        catch let e as FilesError { if case .tooLarge = e { throw ShelfError.tooLarge }; throw ShelfError.unreadable(e.description) }
        catch { throw ShelfError.download }
        let doc = try await Task.detached { try lib.importFile(at: url, name: entry.name) }.value
        files.noteOpened(entry)
        return (lib.adopt(id: doc.id, origin: path, etag: tag) ?? doc, from, entry.size)
    }

    /// An audio file in Files as a run sheet entry, and the phone's own copy of it (kept locally by entry id, never posted); nil for anything
    /// else or a file that cannot be had.
    public func audioEntry(address: String) async -> (entry: RunSheetEntry, local: URL)? {
        guard let url = await audioURL(address: address) else { return nil }
        let name = (FilePath.name(FilePath.fromAddress(address)?.path ?? address) as NSString).deletingPathExtension
        return (.filesAudio(address: address, title: name), url)
    }

    /// The phone's copy of the audio file at the Files address, brought to disk again when it is gone.
    public func audioURL(address: String) async -> URL? {
        let i = await files.open(address: address)
        guard i.status == "ready", i.kind == "audio", !i.local.isEmpty else { return nil }
        return URL(fileURLWithPath: i.local)
    }
}
