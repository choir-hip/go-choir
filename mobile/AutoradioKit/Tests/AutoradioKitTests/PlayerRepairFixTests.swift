import XCTest
import AVFoundation
@testable import AutoradioKit

/// Phone review repairs (2026-11-18), the part that guards RemoteAudioPlayer (kept native): a hold on remote audio (a voice line, a session switch) is not a system pause;
/// a held player is not resumed by a rate or level change; a system pause seen during a hold is reported at the release, not undone.
/// The run sheet player cases of the original file (a cut mail passage, a replayed clip, Previous in a lead-in, a book's title) went with that player to shared code.
@MainActor
final class PlayerRepairFixTests: XCTestCase {
    func testAHoldAndItsReleaseIsNotASystemPause() async {
        let r = RemoteAudioPlayer()
        r.installPlayerForTest()
        var told = 0; r.onSystemPause = { told += 1 }
        r.setHeld(true); r.setHeld(false)
        r.checkSystemPause()                         // the observer's hop lands after the release
        XCTAssertEqual(told, 0); XCTAssertTrue(r.isPlaying, "still wanted playing")
        r.setHeld(true)
        r.checkSystemPause()
        XCTAssertEqual(told, 0); XCTAssertTrue(r.isPlaying)
    }

    func testAHeldPlayerIsNotResumedByARateOrLevelChange() async {
        let r = RemoteAudioPlayer()
        r.installPlayerForTest()
        r.setHeld(true)
        r.setRate(1.5); r.setLevel(.play)
        XCTAssertEqual(r.playerRateForTest, 0, "paused under a voice line stays paused")
    }

    func testARealPauseInsideTheWindowIsReportedWhenItCloses() async {
        let r = RemoteAudioPlayer(); r.installPlayerForTest(); r.ownPauseWindow = 0.1
        var told = 0; r.onSystemPause = { told += 1 }
        r.isPausedNow = { true }
        r.setHeld(true); r.setHeld(false)
        r.checkSystemPause()
        XCTAssertEqual(told, 0, "inside the window it waits")
        try? await Task.sleep(nanoseconds: 400_000_000)
        XCTAssertEqual(told, 1, "still paused at the end of the window: the system's pause")
        XCTAssertFalse(r.isPlaying)
    }

    func testAnOrdinaryHoldAndReleaseIsNotReportedWhenTheWindowCloses() async {
        let r = RemoteAudioPlayer(); r.installPlayerForTest(); r.ownPauseWindow = 0.1
        var told = 0; r.onSystemPause = { told += 1 }
        r.isPausedNow = { false }                      // it went on playing after the release
        r.setHeld(true); r.setHeld(false)
        r.checkSystemPause()
        try? await Task.sleep(nanoseconds: 400_000_000)
        XCTAssertEqual(told, 0); XCTAssertTrue(r.isPlaying)
    }

    func testASystemPauseDuringAHoldIsReportedAtReleaseNotResumed() async {
        let r = RemoteAudioPlayer(); r.installPlayerForTest(); r.ownPauseWindow = 0.05
        var told = 0; r.onSystemPause = { told += 1 }
        r.setHeld(true)
        try? await Task.sleep(nanoseconds: 150_000_000)
        r.checkSystemPause()                           // the pause arrives well after the hold began
        XCTAssertEqual(told, 0)
        r.setHeld(false)
        XCTAssertEqual(told, 1); XCTAssertFalse(r.isPlaying)
        XCTAssertEqual(r.playerRateForTest, 0, "not restarted into a call")
    }
    // a book's title is posted like any entry's: the phone talks to the listener's own computer
}
