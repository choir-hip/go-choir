import XCTest
@testable import AutoradioKit

// What is left of the joins lane in Swift: a clip record the locator is handed is parsed whole or not at all. The clip row, Play, Queue, the episode key and Open episode are shared code
// (AutoradioLab/clips.ts, clip-play.ts; tests clips.test.ts C5 and clip-play.test.ts P1 to P5), and the native store of clips (ClipStore, HearIt) is gone: nothing read it.
final class JoinClipRecordTests: XCTestCase {
    func record(_ rows: [String: Any]) -> String { String(data: try! JSONSerialization.data(withJSONObject: rows), encoding: .utf8)! }
    func testJ6_ABadRecordIsNoClip() {
        for r in ["not json", "{}", "[]", "", record(["clip_id": ""])] { XCTAssertNil(PodcastClip.parse(record: r)) }
    }
}
