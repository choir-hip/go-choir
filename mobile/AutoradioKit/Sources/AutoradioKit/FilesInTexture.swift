import Foundation

// Opening text, Markdown and code from Files (owner, 2026-11-16: "Text and Markdown should open in Texture because that's the way we do it in
// choir.news"). The web does it in frontend/src/packages/texture/App.svelte `openSourceFile` (read the file, then `openFileDocument`, which is
// POST /api/texture/files/open {source_path, title, initial_content} in packages/texture/texture.js): Texture opens the document aliased to the
// path. The phone does the same call and opens the answer's `doc_id` as a `texture:` page, the Reader view a story or a Texture document uses.
// Files has no text viewer. A spreadsheet (CSV) and video are not routed here.
//
// Ways this can fail (each has a test in FilesInTextureTests): a binary or an oversized file is sent as text; the path goes up with a leading
// slash (the web sends 'Documents/notes.md'); a failed call opens an empty page instead of saying why; a PDF or a table is routed here.

public struct FilesInTexture: Sendable {
    nonisolated(unsafe) public static var current: FilesInTexture?
    public let files: FilesService
    /// The local content store: a text or Markdown file it already holds at the same version is not fetched again (nil: never).
    public let store: ContentStore?
    public init(files: FilesService, store: ContentStore? = nil) { self.files = files; self.store = store }

    /// Text, Markdown and code (what the web's `isTextFileName` routes to Texture, less the table kinds).
    public static func opensInTexture(_ address: String) -> Bool {
        guard let (path, isDir) = FilePath.fromAddress(address), !isDir else { return false }
        let k = FileTypes.kind(name: FilePath.name(path), isDirectory: false)
        return k == .markdown || k == .text || k == .code
    }

    /// The body of POST /api/texture/files/open.
    static func body(path: String, title: String, content: String) -> [String: Any] {
        ["source_path": path.hasPrefix("/") ? String(path.dropFirst()) : path, "title": title, "initial_content": content]
    }

    /// The `texture:` address of the document for the file, and its title. Throws a plain sentence when it cannot be opened.
    public func open(address: String) async throws -> (address: String, title: String) {
        let r = try await openReporting(address: address)
        return (r.address, r.title)
    }

    /// The same, with where the text came from ("store", "cache" or "network") and its size in bytes.
    public func openReporting(address: String) async throws -> (address: String, title: String, from: String, bytes: Int64) {
        guard Self.opensInTexture(address), let (path, _) = FilePath.fromAddress(address) else { throw FilesError.unsupported("That file does not open in Texture.") }
        guard let entry = await files.entry(path: path) else { throw FilesError.notFound }
        let cap = Int64(1_000_000)
        let text: String, from: String
        if let s = store, s.usable, let held = s.get("file:" + path), held.kind == .file, held.version == ContentSync.fileVersion(entry), !held.text.isEmpty {
            text = held.text; from = "store"            // the text synced earlier; only text kinds are ever held, never a PDF's or an e-book's bytes
        } else {
            from = files.isCached(entry) ? "cache" : "network"
            let url = try await files.download(entry, maxBytes: cap)
            guard let data = try? Data(contentsOf: url), let t = String(data: data, encoding: .utf8) else { throw FilesError.unsupported("This file is not text.") }
            text = t
        }
        let r: ChoirResponse
        do { r = try await ChoirEnvironment.shared.client().postJSON("/api/texture/files/open", Self.body(path: path, title: entry.name, content: text), route: "/api/texture/files/open") }
        catch let e as ChoirError { throw FilesError.server(FilesService.say(e)) }
        guard let id = (try? r.object())?["doc_id"] as? String, !id.isEmpty else { throw FilesError.server("Texture did not open the document.") }
        files.noteOpened(entry)
        return ("texture:" + id, entry.name, from, Int64(text.utf8.count))
    }
}
