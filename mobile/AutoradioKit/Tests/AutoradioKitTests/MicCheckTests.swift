import XCTest
@testable import AutoradioKit

final class MicCheckTests: XCTestCase {
    func testRoomToneNeverPassesAndTimesOut() {
        var c = MicCheck()
        for _ in 0..<(25 * 10) { c.feed(db: -55, seconds: 0.1); c.tick(seconds: 0.1) }
        XCTAssertEqual(c.verdict, .silent)
        XCTAssertLessThan(c.level, 0.3)
    }
    func testSustainedLevelPasses() {
        var c = MicCheck()
        for _ in 0..<3 { c.feed(db: -30, seconds: 0.1); XCTAssertEqual(c.verdict, .listening) }
        for _ in 0..<5 { c.feed(db: -30, seconds: 0.1) }
        XCTAssertEqual(c.verdict, .heard)
    }
    func testOneClickDoesNotPass() {
        var c = MicCheck()
        c.feed(db: -10, seconds: 0.1)
        for _ in 0..<50 { c.feed(db: -55, seconds: 0.1) }
        XCTAssertEqual(c.verdict, .listening)
    }
    func testAnyTranscriptPasses() {
        var c = MicCheck()
        c.hear(transcript: "  ")
        XCTAssertEqual(c.verdict, .listening)
        c.hear(transcript: "hello")
        XCTAssertEqual(c.verdict, .heard)
    }
    func testNoFramesTimesOutByTick() {
        var c = MicCheck()
        c.tick(seconds: 19.9); XCTAssertEqual(c.verdict, .listening)
        c.tick(seconds: 0.2); XCTAssertEqual(c.verdict, .silent)
    }
    func testLevelRisesFastFallsSlowAndStaysInRange() {
        var c = MicCheck()
        c.feed(db: -20, seconds: 0.05); let up = c.level
        XCTAssertGreaterThan(up, 0.5)
        c.feed(db: -90, seconds: 0.05)
        XCTAssertGreaterThan(c.level, up * 0.8)
        XCTAssertLessThanOrEqual(c.level, 1); XCTAssertGreaterThanOrEqual(c.level, 0)
    }
}

final class MicCheckMeterTests: XCTestCase {
    func testTheMeterTheModuleDrivesPassesOnSustainedSoundAndGivesUpOnSilence() {
        let loud = MicCheckMeter()
        XCTAssertEqual(loud.verdict, .listening)
        for _ in 0..<8 { loud.frame(db: -30, seconds: 0.1); loud.tick(0.1) }
        XCTAssertEqual(loud.verdict, .heard); XCTAssertGreaterThan(loud.level, 0)
        let quiet = MicCheckMeter()
        for _ in 0..<210 { quiet.frame(db: -80, seconds: 0.1); quiet.tick(0.1) }
        XCTAssertEqual(quiet.verdict, .silent)
        let spoken = MicCheckMeter(); spoken.transcript("  "); XCTAssertEqual(spoken.verdict, .listening)
        spoken.transcript("hello"); XCTAssertEqual(spoken.verdict, .heard)
    }
}
