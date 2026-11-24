import XCTest
@testable import AutoradioKit

final class NativeOpLogTests: XCTestCase {
    private final class Box: @unchecked Sendable { var t = 0; var lines: [String] = [] }

    private func make(_ b: Box) -> NativeOpLog { NativeOpLog(now: { b.t }, sink: { b.lines.append($0) }, thresholdMs: 5000) }

    func testBeginAndEndAreLoggedWithElapsed() {
        let b = Box(); let log = make(b)
        var got: [String: Any]?
        let reply = log.wrap(op: "deck") { got = $0 }
        XCTAssertEqual(b.lines, ["AUTORADIO native begin op=deck n=1"])
        b.t = 120
        reply(["ok": true])
        XCTAssertEqual(b.lines.last, "AUTORADIO native end op=deck n=1 ms=120")
        XCTAssertEqual(got?["ok"] as? Bool, true)
    }

    func testACallOpenPastTheAgeIsReportedAndAnAnsweredOneIsNot() {
        let b = Box(); let log = make(b)
        _ = log.wrap(op: "pagesMirror") { _ in }
        let quick = log.wrap(op: "pageText") { _ in }
        quick([:])
        b.t = 4000; log.check()
        XCTAssertFalse(b.lines.contains { $0.contains("open") })
        b.t = 7000; log.check()
        XCTAssertEqual(b.lines.filter { $0.contains("native open") }, ["AUTORADIO native open op=pagesMirror n=1 for_ms=7000"])
    }

    func testASecondReplyIsNamed() {
        let b = Box(); let log = make(b)
        var calls = 0
        let reply = log.wrap(op: "embed") { _ in calls += 1 }
        reply([:]); reply([:])
        XCTAssertEqual(b.lines.last, "AUTORADIO native twice op=embed n=1")
        XCTAssertEqual(calls, 2)
    }
}
