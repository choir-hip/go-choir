import XCTest
@testable import AutoradioKit

// Text, Markdown and code from Files open in Texture (owner, 2026-11-16), by the web's own call: POST /api/texture/files/open.
//  1 only text, Markdown and code are routed; a table, a PDF, a picture and a folder are not
//  2 the source path goes up as the web sends it, with no leading slash, with the file's own text and title
//  3 the answer's doc_id is the `texture:` page that opens; a failed call says why and opens nothing
final class FilesInTextureTests: XCTestCase {
    func test1_OnlyTextMarkdownAndCodeAreRouted() {
        for a in ["file:/Documents/Harbor notes.md", "file:/Documents/Key list.txt", "file:/Documents/Lamps.swift"] { XCTAssertTrue(FilesInTexture.opensInTexture(a), a) }
        for a in ["file:/Documents/Budget 2026.csv", "file:/Documents/Council minutes.pdf", "file:/Pictures/x.png", "file:/Media/Pattern.mp4", "file:/Documents/"] { XCTAssertFalse(FilesInTexture.opensInTexture(a), a) }
    }
    func test2_TheCallIsTheWebsCall() {
        let b = FilesInTexture.body(path: "/Documents/Harbor notes.md", title: "Harbor notes.md", content: "# Harbor\n")
        XCTAssertEqual(b["source_path"] as? String, "Documents/Harbor notes.md"); XCTAssertEqual(b["title"] as? String, "Harbor notes.md"); XCTAssertEqual(b["initial_content"] as? String, "# Harbor\n")
    }
    func test3_TheAnswersDocumentOpensAsATexturePage() async throws {
        let env = ChoirEnvironment.shared
        let saved = (env.session, env.keys)
        Demo.install(into: env)
        defer { env.session = saved.0; env.keys = saved.1 }
        let tmp = FileManager.default.temporaryDirectory.appendingPathComponent("fit-\(UUID().uuidString)")
        defer { try? FileManager.default.removeItem(at: tmp) }
        let service = FilesService(backend: DemoFilesBackend(), cacheRoot: tmp, appPrefix: "app:/cache/files", recents: RecentFiles(file: tmp.appendingPathComponent("r.json")))
        let r = try await FilesInTexture(files: service).open(address: "file:/Documents/Harbor notes.md")
        XCTAssertEqual(r.address, "texture:demo-notes"); XCTAssertEqual(r.title, "Harbor notes.md")
        do { _ = try await FilesInTexture(files: service).open(address: "file:/Documents/gone.md"); XCTFail("opened") } catch { XCTAssertEqual((error as? FilesError), .notFound) }
        do { _ = try await FilesInTexture(files: service).open(address: "file:/Documents/Budget 2026.csv"); XCTFail("opened") } catch {}
    }
}
