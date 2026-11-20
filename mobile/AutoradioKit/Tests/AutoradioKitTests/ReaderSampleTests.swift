import XCTest
@testable import AutoradioKit

// The sample book. Failure list: (2) one document with 3 chapters and 12 paragraphs, its id the one the leftover cleanup removes;
// (3) seeding twice is still one document; (4) the sample is found by find and by the launcher's search; (5) reading from a block gives words.
final class ReaderSampleTests: XCTestCase {
    func shelf() -> (ReaderShelf, URL) {
        let d = FileManager.default.temporaryDirectory.appendingPathComponent("sample-\(UUID().uuidString)")
        return (ReaderShelf(root: d), d)
    }
    func testSeedsOneBookOfThreeChapters() throws {
        let (s, d) = shelf(); defer { try? FileManager.default.removeItem(at: d) }
        let doc = try XCTUnwrap(s.seedSample())
        XCTAssertEqual(doc.title, "Using Reader"); XCTAssertEqual(doc.kind, "epub"); XCTAssertEqual(doc.sections, 3)
        let t = try XCTUnwrap(s.text(doc.id))
        XCTAssertEqual(t.sections.map(\.title), ["Adding a book", "Reading on the screen", "Listening and finding"])
        XCTAssertEqual(t.blocks.filter { $0.kind == "paragraph" }.count, 12)
        XCTAssertEqual(t.blocks.filter { $0.kind == "heading" }.count, 3)
        XCTAssertEqual(doc.id, ReaderSample.id)
        XCTAssertEqual(s.seedSample()?.id, doc.id)
        XCTAssertEqual(s.list().count, 1)
    }
    func testSampleIsFoundAndReadable() throws {
        let (s, d) = shelf(); defer { try? FileManager.default.removeItem(at: d) }
        let doc = try XCTUnwrap(s.seedSample())
        XCTAssertFalse(s.find(in: doc.id, query: "queue").isEmpty)
        XCTAssertFalse(s.hits(query: "using reader").isEmpty)
        XCTAssertFalse(s.hits(query: "half-remembered sentence").isEmpty)
        XCTAssertGreaterThan(s.blocks(id: doc.id, from: 4).count, 5)
    }
}
