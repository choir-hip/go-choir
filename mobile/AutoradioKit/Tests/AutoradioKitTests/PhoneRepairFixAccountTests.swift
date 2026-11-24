import XCTest
@testable import AutoradioKit

// Phone repair, lane fix-account. Failure list (written before the fixes):
//  A2  (the notebook write, its tests gone with the Swift notebook store: shared code owns it, notebook-topics.test.ts)
//  A5  a finite number outside Int range in a clip's fingerprint neither crashes nor poisons the clip
//  (A4, a stranger's take text, and A5's candidate-clip sanitising were the programme request's: gone with it)
final class PhoneRepairSanitiseTests: XCTestCase {
    func clip(show: String = "Show", published: String = "2026-11-16T09:00:00Z", fp: [String: Any]? = nil) -> PodcastClip? {
        var row: [String: Any] = ["clip_id": "clip-0000000000a1", "story": ["route": "/a", "version": 1, "title": "S"], "show": show, "episode_title": "Ep",
                                  "published": published, "audio_url": "https://example.test/x.mp3", "duration_seconds": 900, "start": 10, "end": 70, "quote": "q\u{0007}uote", "support": 0.9]
        if let fp { row["fingerprint"] = fp }
        return PodcastClip.parse(row)
    }
    func testOutOfRangeFingerprintNumbersAreNoFingerprint() throws {
        let base = testFingerprintJSON()
        for key in ["version", "file_bytes", "first_frame_offset", "bitrate_kbps"] {
            var fp = base; fp[key] = 1e300
            let c = try XCTUnwrap(clip(fp: fp), "the clip itself still parses")
            XCTAssertNil(c.fingerprint, "\(key)")
        }
    }
}
