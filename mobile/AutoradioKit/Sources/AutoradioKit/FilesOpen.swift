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

    public static func open(address: String, title: String?, pages: OpenPages, reader: FilesInReader? = FilesInReader.current, texture: FilesInTexture? = FilesInTexture.current) async -> Result {
        let name = FilePath.name(FilePath.fromAddress(address)?.path ?? address)
        if FilesInReader.opensInReader(address), let reader {
            pages.close(address)
            do { let d = try await reader.open(address: address); pages.open(d.address, title: d.title); return .opened(d.address) }
            catch { return .failed(BarDocumentImport.failureLine(error, url: URL(fileURLWithPath: FilePath.fromAddress(address)?.path ?? address))) }
        }
        if FilesInTexture.opensInTexture(address), let texture {
            pages.close(address)
            do { let d = try await texture.open(address: address); pages.open(d.address, title: d.title); return .opened(d.address) }
            catch { return .failed("\(name) could not be opened. \(FilesService.say(error))") }
        }
        pages.open(address, title: title)
        return .opened(address)
    }

    /// Closes every `file:` page whose document opens elsewhere (an older build left them in the pile).
    public static func purge(pages: OpenPages) {
        for p in pages.list() where resolvesElsewhere(p.address) { pages.close(p.address) }
    }
}
