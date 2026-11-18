import Foundation

// The one way a Files address opens as a page. A PDF or an e-book is a Reader document, text, Markdown and code are Texture documents: for
// those the page that opens is the document's own, found or opened by its address, and a `file:<path>` page for them is never in the pile
// (not while the document is being fetched, not when it could not be). Anything else (a picture, audio, a table, a folder) is the `file:` page.
//
// Ways this can fail (each has a test in FilesOpenTests):
//  1. The document's page is already in the pile (an earlier session) and the open leaves a `file:` page on top: the open focuses the existing one.
//  2. A resolve that fails leaves a placard page: it opens no page, closes any `file:` page for the path, and says why.
//  3. A `file:` page for a resolvable type survives in the pile from an older build: `purge` closes it.

public enum FilesOpen {
    public enum Result: Equatable, Sendable {
        /// The address that is now the page in front.
        case opened(String)
        /// Nothing opened; the plain-words line to say.
        case failed(String)
    }

    /// True for a `file:` address whose page is some other page (a Reader or a Texture document).
    public static func resolvesElsewhere(_ address: String) -> Bool { FilesInReader.opensInReader(address) || FilesInTexture.opensInTexture(address) }

    /// How long an open may take (download, import, the Texture call) before it stops and says so. Never an endless "Opening".
    nonisolated(unsafe) public static var deadline: TimeInterval = 60
    struct TimedOut: Error {}

    static func race<T: Sendable>(_ seconds: TimeInterval, _ op: @escaping @Sendable () async throws -> T) async throws -> T {
        try await withThrowingTaskGroup(of: T.self) { g in
            g.addTask { try await op() }
            g.addTask { try await Task.sleep(nanoseconds: UInt64(max(0.001, seconds) * 1_000_000_000)); throw TimedOut() }
            defer { g.cancelAll() }
            return try await g.next()!
        }
    }

    /// One console line per open: kind, size, where the bytes came from, how long, whether it worked and why not. No file names.
    public static func line(kind: String, bytes: Int64, from: String, ms: Int, ok: Bool, why: String) -> String {
        "AUTORADIO file open kind=\(kind) bytes=\(bytes) from=\(from) ms=\(ms) ok=\(ok) why=\(ok ? "-" : why)"
    }
    /// The line when a Reader document's page is asked for: what was drawn, by counts.
    public static func pageLine(kind: String, pages: Int, shown: Bool) -> String { "AUTORADIO file page kind=\(kind) sections=\(pages) frame=\(shown ? "pdf" : "text")" }
    static func kindWord(_ address: String) -> String {
        guard let (path, isDir) = FilePath.fromAddress(address), !isDir else { return "other" }
        switch FileTypes.kind(name: FilePath.name(path), isDirectory: false) {
        case .pdf: return "pdf"; case .epub: return "epub"; case .markdown: return "md"; case .text: return "text"; case .code: return "code"
        case .image: return "image"; case .audio: return "audio"; default: return "other"
        }
    }
    static func why(_ e: Error) -> String {
        if e is TimedOut { return "timeout" }
        if let s = e as? ShelfError { switch s { case .tooLarge: return "too-large"; case .noText: return "no-text"; case .protected: return "protected"; case .notSupported: return "unsupported"; case .download: return "download"; case .unreadable(let m): return m.contains("not there") ? "not-found" : "unreadable" } }
        if let f = e as? FilesError { switch f { case .notFound: return "not-found"; case .tooLarge: return "too-large"; case .unsupported: return "unsupported"; case .server: return "server"; default: return "files" } }
        return "error"
    }

    public static func open(address: String, title: String?, pages: OpenPages, reader: FilesInReader? = FilesInReader.current, texture: FilesInTexture? = FilesInTexture.current,
                            say: @Sendable (String) -> Void = { print($0) }) async -> Result {
        let name = FilePath.name(FilePath.fromAddress(address)?.path ?? address)
        let kind = kindWord(address), t0 = Date()
        func ms() -> Int { Int(Date().timeIntervalSince(t0) * 1000) }
        func failed(_ e: Error, _ lead: String) -> Result {
            say(line(kind: kind, bytes: 0, from: "-", ms: ms(), ok: false, why: why(e)))
            let reason = e is TimedOut ? "It took too long." : (e as? ShelfError)?.description ?? FilesService.say(e)
            return .failed("\(name) could not be opened. \(reason) Tap the file to try again.")
        }
        if FilesInReader.opensInReader(address), let reader {
            pages.close(address)
            do {
                let r = try await race(deadline) { try await reader.openReporting(address: address) }
                pages.open(r.doc.address, title: r.doc.title)
                say(line(kind: kind, bytes: r.bytes, from: r.from, ms: ms(), ok: true, why: "-"))
                return .opened(r.doc.address)
            } catch { return failed(error, "") }
        }
        if FilesInTexture.opensInTexture(address), let texture {
            pages.close(address)
            do {
                let r = try await race(deadline) { try await texture.openReporting(address: address) }
                pages.open(r.address, title: r.title)
                say(line(kind: kind, bytes: r.bytes, from: r.from, ms: ms(), ok: true, why: "-"))
                return .opened(r.address)
            } catch { return failed(error, "") }
        }
        pages.open(address, title: title)
        say(line(kind: kind, bytes: 0, from: "-", ms: ms(), ok: true, why: "-"))
        return .opened(address)
    }

    /// Closes every `file:` page whose document opens elsewhere (an older build left them in the pile).
    public static func purge(pages: OpenPages) {
        for p in pages.list() where resolvesElsewhere(p.address) { pages.close(p.address) }
    }
}
