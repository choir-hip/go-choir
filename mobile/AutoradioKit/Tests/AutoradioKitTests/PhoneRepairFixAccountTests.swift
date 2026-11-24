import XCTest
@testable import AutoradioKit

// Phone repair, lane fix-account. Failure list (written before the fixes):
//  A2  the server cannot say (nil) is not "there is no notebook": no document is made; "found == false" makes one; two writes at once make one
//  A5  a finite number outside Int range in a clip's fingerprint neither crashes nor poisons the clip
//  (A4, a stranger's take text, and A5's candidate-clip sanitising were the programme request's: gone with it)
@MainActor
final class NotebookWriteTests: XCTestCase {
    var saved: KeyStore!
    override func setUp() { saved = ChoirEnvironment.shared.keys; ChoirEnvironment.shared.keys = InMemoryKeyStore([KeyAccount.apiKey: "k"]) }
    override func tearDown() { ChoirEnvironment.shared.keys = saved }
    func marker() -> URL { FileManager.default.temporaryDirectory.appendingPathComponent("nbm-\(UUID().uuidString)") }

    final class Counter: @unchecked Sendable { let l = NSLock(); private var n = 0; func bump() { l.lock(); n += 1; l.unlock() }; var count: Int { l.lock(); defer { l.unlock() }; return n } }

    func testServerCannotSayCreatesNothing() async {
        let s = NotebookStore(), made = Counter()
        s.lookupEdition = { nil }
        s.createNotebook = { made.bump(); return "d1" }
        let ok = await s.writeTopics(["alpha"], marker: marker())
        XCTAssertFalse(ok); XCTAssertEqual(made.count, 0, "a timeout, 401 or 5xx must not make a second \"My notebook\"")
    }
    func testNoNotebookCreatesOne() async {
        let s = NotebookStore(), made = Counter()
        s.lookupEdition = { EditionInfo(found: false) }
        s.createNotebook = { made.bump(); return nil }       // stop before the editor: the creation is what is asked
        _ = await s.writeTopics(["alpha"], marker: marker())
        XCTAssertEqual(made.count, 1)
    }
    func testTwoWritesAtOnceMakeOne() async {
        let s = NotebookStore(), made = Counter()
        s.lookupEdition = { try? await Task.sleep(nanoseconds: 100_000_000); return EditionInfo(found: false) }
        s.createNotebook = { made.bump(); return nil }
        let m = marker()
        async let a = s.writeTopics(["alpha"], marker: m), b = s.writeTopics(["alpha"], marker: m)
        _ = await (a, b)
        XCTAssertEqual(made.count, 1)
    }
}

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
