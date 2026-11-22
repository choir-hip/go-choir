import XCTest
@testable import AutoradioKit

/// Takes, what stays native (failure list: Takes.swift; the page, the wire for publishing and the listener's own data are shared code, takes.test.ts and take-session.test.ts).
final class TakesTests: XCTestCase {
    func testRespondsToFromTheEntryOnAir() async {
        await MainActor.run {
            var story = RunSheetEntry(id: "a", kind: .story, placedBy: .desk, title: "Harbor fees rise", text: "x", estSeconds: 3)
            story.cites = [RunSheetCite(ref: "pub:harbor-fees@v2", role: .subject, label: "Harbor fees rise")]
            XCTAssertEqual(TakeResponds.from(entry: story), TakeResponds(route: "harbor-fees", title: "Harbor fees rise"))
            XCTAssertNil(TakeResponds.from(entry: RunSheetEntry(id: "o", kind: .opening, placedBy: .phone, title: "Opening", text: "x", estSeconds: 3)))
            XCTAssertNil(TakeResponds.from(entry: nil))
        }
    }
}
