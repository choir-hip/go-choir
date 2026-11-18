import XCTest
import AVFoundation
@testable import AutoradioKit

/// Phone review repairs (2026-11-18): a replayed clip plays its audio again; Previous works in a lead-in;
/// a hold on remote audio (a voice line, a session switch) is not a system pause; a held player is not resumed by a rate or level change.
@MainActor
final class PlayerRepairFixTests: XCTestCase {
    final class Capture: RunSheetPoster, @unchecked Sendable {
        let lock = NSLock(); private var _ops: [RunSheetOp] = []
        var ops: [RunSheetOp] { lock.lock(); defer { lock.unlock() }; return _ops }
        func post(ops: [RunSheetOp]) async -> RunSheetPostOutcome { lock.lock(); _ops += ops; lock.unlock(); return .answered([]) }
        func fetchSheet() async -> RunSheet? { nil }
    }
    func make() -> (RunSheetPlayer, RunSheetStore, FakeEngine) {
        let s = RunSheetStore(); let p = RunSheetPlayer(store: s); let e = FakeEngine(); p.engine = e
        return (p, s, e)
    }
    func clip(_ n: String, lead: String? = nil) -> RunSheetEntry {
        var c = RunSheetEntry.audio(RunSheetAudio(url: URL(string: "https://x.test/\(n).mp3")!, title: "T \(n)", show: "Show", start: 10, end: 30, episodeId: "e\(n)"))
        c.text = lead
        return c
    }

    // P1
    func testACutMailPassageIsContinuedFromTheStoreWithoutPostingItsWordsAsText() async {
        let (p, s, e) = make()
        let poster = Capture(); s.poster = poster
        let words = ["Zebrafish", "quarterly", "invoice", "mentions", "Dolores", "tomorrow", "marmalade", "afternoon", "confidential"]
        let passage = words.joined(separator: " ") + ". " + "Pelican budget lantern sketch."
        let title = words.prefix(6).joined(separator: " ")                 // what LabRuntime builds
        p.playNow(.readAloud(address: "mail:m1", title: title), words: (title, [passage])); await p.settle()
        e.remainder = "Pelican budget lantern sketch."
        let answer = RunSheetEntry(id: "a-1", kind: .answer, placedBy: .desk, title: "Answer", text: "Here is the answer.", estSeconds: 5)
        s.place(answer, at: .now, by: .desk, cutting: AirReport(block: 0, seconds: 2))
        _ = await poll { s.onAir()?.id == "a-1" }; await p.settle()
        p.documentEnded(id: "rs:a-1", anyPlayed: true)
        _ = await poll { s.onAir()?.kind == .mail && s.onAir()?.id != "a-1" }; await p.settle()
        XCTAssertEqual(s.onAir()?.kind, .mail, "the message goes back on air from its place")
        XCTAssertTrue(e.speaks.count >= 3, "and is spoken again")
        await s.flushTask?.value; s.flush(); await s.flushTask?.value
        let sent = poster.ops
        XCTAssertFalse(sent.isEmpty)
        let enc = JSONEncoder()
        let all = sent.map { String(decoding: (try? enc.encode($0)) ?? Data(), as: UTF8.self).lowercased() }.joined(separator: "\n")
        XCTAssertTrue(all.contains(title.lowercased()), "the entry posts its real title")
        // The continuation names the address and carries no text: the phone reads the rest from its own store (this is the entry's shape, not a privacy rule).
        for w in ["Pelican", "budget", "lantern", "sketch"] { XCTAssertFalse(all.contains(w.lowercased()), "\(w) was posted as entry text: \(all)") }
    }

    // P2
    func testAClipReplayedAfterPreviousPlaysItsAudioAgain() async {
        let (p, s, e) = make()
        let c = clip("a", lead: "Lead in.")
        p.playNow(c); await p.settle()
        p.documentEnded(id: "rs:" + c.id, anyPlayed: true); await p.settle()
        XCTAssertEqual(e.log.filter { $0.hasPrefix("remote:") }.count, 1)
        p.remoteEnded(generation: p.generation)
        XCTAssertEqual(s.entry(c.id)?.state, .heard)
        p.previous(); _ = await poll { s.onAir()?.id == c.id }; await p.settle()
        XCTAssertEqual(s.onAir()?.id, c.id)
        p.documentEnded(id: "rs:" + c.id, anyPlayed: true); await p.settle()
        XCTAssertEqual(e.log.filter { $0.hasPrefix("remote:") }.count, 2, "the lead-in was followed by the audio the second time too")
    }

    // P3
    func testPreviousWorksWhileAClipsLeadInSpeaks() async {
        let (p, _, e) = make()
        let c = clip("a", lead: "Lead in.")
        p.playNow(c); await p.settle()
        XCTAssertNotNil(p.audio); XCTAssertNotNil(p.playingDocument)
        e.position = (0, 9)
        p.previous()
        XCTAssertTrue(e.log.contains("restart"), "the spoken lead-in restarts")
        XCTAssertFalse(e.log.contains { $0.hasPrefix("seekTo") }, "there is no remote player to seek")
    }

    // P5 and P7
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

    // review 1: a real pause inside the window is reported when it closes; an ordinary hold and release is not
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
    // review 3: a system pause seen during a hold is not undone by the release
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
    func testABooksTitleIsPosted() async {
        let (p, s, _) = make()
        let poster = Capture(); s.poster = poster
        p.registerTextSource(kind: "document") { _, _ in ["Block one."] }
        p.playNow(.book(id: "b1", title: "Confidential Merger Memoir", at: 2)); await p.settle()
        await s.flushTask?.value; s.flush(); await s.flushTask?.value
        let all = poster.ops.map { String(decoding: (try? JSONEncoder().encode($0)) ?? Data(), as: UTF8.self) }.joined()
        XCTAssertFalse(poster.ops.isEmpty)
        XCTAssertTrue(all.contains("Confidential Merger Memoir"), all)
        XCTAssertFalse(all.contains("Read aloud"), all)
        XCTAssertTrue(all.contains("doc:b1"))
    }
}
