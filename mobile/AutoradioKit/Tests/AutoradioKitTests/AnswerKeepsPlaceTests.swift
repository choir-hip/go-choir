import XCTest
@testable import AutoradioKit

/// An answer that goes on air over a programme entry does not destroy its place: the player puts a continuation back when the answers are over.
/// (The reducer's `place now` cuts the entry; `previous` would restore the whole entry from its start and would restore the answer too, so the
/// continuation is a `place next` of a fresh entry carrying the words still to say.)
@MainActor
final class AnswerKeepsPlaceTests: XCTestCase {
    func make() -> (RunSheetPlayer, RunSheetStore, FakeEngine) {
        let s = RunSheetStore(); let p = RunSheetPlayer(store: s); let e = FakeEngine(); p.engine = e
        return (p, s, e)
    }
    func story(_ id: String) -> RunSheetEntry {
        var e = RunSheetEntry(id: id, kind: .story, placedBy: .desk, title: "Bridge", text: "One. Two. Three.", estSeconds: 10)
        e.source = RunSheetSource(ref: "pub:/pub/texture/bridge", label: "Bridge")
        return e
    }
    func answer(_ id: String) -> RunSheetEntry { RunSheetEntry(id: id, kind: .answer, placedBy: .desk, title: "Answer", text: "Here is the answer.", estSeconds: 5) }
    func onAirStory(_ p: RunSheetPlayer, _ s: RunSheetStore, _ e: FakeEngine) async {
        p.queue(story("s-1")); p.next(); p.settle0()
        _ = await poll { s.onAir()?.id == "s-1" }
        await p.settle()
        e.remainder = "Two. Three."
    }
    func cutWithAnswer(_ p: RunSheetPlayer, _ s: RunSheetStore, _ id: String) async {
        s.place(answer(id), at: .now, by: .desk, cutting: AirReport(block: 1, seconds: 2))
        _ = await poll { s.onAir()?.id == id }
        await p.settle()
    }

    func testWhenTheAnswerEndsTheCutEntryGoesBackFromTheStartOfItsSentence() async {
        let (p, s, e) = make()
        p.playNow(story("s-1")); await p.settle(); e.remainder = "Two. Three."
        await cutWithAnswer(p, s, "a-1")
        XCTAssertEqual(s.onAir()?.kind, .answer)
        p.documentEnded(id: "rs:a-1", anyPlayed: true)
        let back = await poll { s.onAir()?.kind == .story && s.onAir()?.id != "s-1" }
        XCTAssertTrue(back, "the entry is back on air")
        let c = s.onAir()!
        XCTAssertEqual(c.text, "Two. Three.", "from the start of the sentence it was in")
        XCTAssertEqual(c.source?.ref, "pub:/pub/texture/bridge", "the same source")
        XCTAssertNotEqual(c.id, "s-1", "a fresh id: the reducer has no way to put an ended entry back on air")
    }

    func testAListenersSkipDuringTheAnswerDropsTheKeptEntry() async {
        let (p, s, e) = make()
        p.playNow(story("s-1")); await p.settle(); e.remainder = "Two. Three."
        await cutWithAnswer(p, s, "a-1")
        p.next()
        await p.settle()
        XCTAssertNil(p.remembered)
        XCTAssertFalse(s.sheet.entries.contains { $0.id.hasPrefix("c-") })
    }

    func testPlayingSomethingElseDropsTheKeptEntry() async {
        let (p, s, e) = make()
        p.playNow(story("s-1")); await p.settle(); e.remainder = "Two. Three."
        await cutWithAnswer(p, s, "a-1")
        p.playNow(story("s-2")); await p.settle()
        XCTAssertNil(p.remembered)
        XCTAssertEqual(s.onAir()?.id, "s-2")
    }

    func testTwoAnswersInARowKeepTheOneEntryAndPutItBackOnce() async {
        let (p, s, e) = make()
        p.playNow(story("s-1")); await p.settle(); e.remainder = "Two. Three."
        await cutWithAnswer(p, s, "a-1")
        e.remainder = "the answer, interrupted"
        await cutWithAnswer(p, s, "a-2")                       // the listener spoke again: the second answer cuts the first
        p.documentEnded(id: "rs:a-2", anyPlayed: true)
        let back = await poll { s.onAir()?.kind == .story }
        XCTAssertTrue(back)
        XCTAssertEqual(s.onAir()?.text, "Two. Three.", "the programme entry, not the interrupted answer")
        XCTAssertEqual(s.sheet.entries.filter { $0.id.hasPrefix("c-") }.count, 1)
    }

    func testAnAnswerThatDoesNotCutKeepsNothing() async {
        let (p, s, e) = make()
        p.playNow(story("s-1")); await p.settle(); e.remainder = "Two. Three."
        s.place(answer("a-1"), at: .end, by: .desk)
        XCTAssertNil(p.remembered)
        _ = e
    }
}
extension RunSheetPlayer { func settle0() {} }

@MainActor
final class AnswerKeepsRemotePlaceTests: XCTestCase {
    func testAPodcastCutByAnAnswerComesBackFromWhereItWas() async {
        let s = RunSheetStore(); let p = RunSheetPlayer(store: s); let e = FakeEngine(); p.engine = e
        let a = RunSheetAudio(url: URL(string: "https://x.test/e.mp3")!, title: "Ep", show: "Show", start: 10, end: nil, episodeId: "e1")
        p.playNow(.audio(a)); await p.settle()
        e.elapsed = 95
        s.place(RunSheetEntry(id: "a-1", kind: .answer, placedBy: .desk, title: "Answer", text: "Answer.", estSeconds: 3), at: .now, by: .desk, cutting: AirReport(block: 0, seconds: 95))
        _ = await poll { s.onAir()?.id == "a-1" }; await p.settle()
        p.documentEnded(id: "rs:a-1", anyPlayed: true)
        let back = await poll { s.onAir()?.kind == .episode || s.onAir()?.kind == .clip }
        XCTAssertTrue(back)
        XCTAssertEqual(s.onAir()?.source?.start, 95, "the same audio from where it was")
    }
}

/// No sound path outside the one gate: every place the Kit can make sound is named here, with the reason it is behind the gate. A new one fails.
final class SoundPathsTests: XCTestCase {
    func testEverySoundPathIsKnownAndGated() throws {
        let dir = URL(fileURLWithPath: #filePath).deletingLastPathComponent().deletingLastPathComponent().deletingLastPathComponent()
            .appendingPathComponent("Sources/AutoradioKit")
        let patterns = ["AVAudioPlayerNode(", "AVPlayer(", "AVAudioPlayer(", "AVQueuePlayer(", "AVSpeechSynthesizer(", "AVSpeechSynthesizer.", ".speakUtterance(", "AudioServicesPlaySystemSound"]
        var found: [String: Set<String>] = [:]
        for case let url as URL in FileManager.default.enumerator(at: dir, includingPropertiesForKeys: nil)! where url.pathExtension == "swift" {
            let text = try String(contentsOf: url, encoding: .utf8)
            for line in text.split(separator: "\n") where !line.trimmingCharacters(in: .whitespaces).hasPrefix("//") && !line.trimmingCharacters(in: .whitespaces).hasPrefix("///") {
                for p in patterns where line.contains(p) { found[url.lastPathComponent, default: []].insert(p) }
            }
        }
        // LiveAudio: the programme node and the voice node, both held by the gate (voice line, session switch). RemoteAudioPlayer: AVPlayer,
        // held through `setHeld` (a voice line) and `setLevel` (the listener speaking). AppleStages: the system voice is synthesised to buffers
        // (`write`), which LiveAudio plays; it never calls `speak(_:)` on the synthesizer.
        let known: [String: Set<String>] = [
            "LiveAudio.swift": ["AVAudioPlayerNode("],
            "RemoteAudioPlayer.swift": ["AVPlayer("],
            "AppleStages.swift": ["AVSpeechSynthesizer("],
        ]
        XCTAssertEqual(found.mapValues { $0 }, known, "a new place that makes sound must go behind the single-voice gate (LiveAudio.holdForSessionSwitch / onVoiceHold) and be named here")
        let apple = try String(contentsOf: dir.appendingPathComponent("AppleStages.swift"), encoding: .utf8)
        XCTAssertFalse(apple.contains(".speak(utterance") || apple.contains("speak(u)"), "the system voice must be written to buffers, never spoken by the synthesizer itself")
    }

    @MainActor func testRemoteAudioIsHeldWithAVoiceLine() async {
        let r = RemoteAudioPlayer()
        r.setHeld(true); XCTAssertTrue(r.isHeld)
        r.setHeld(false); XCTAssertFalse(r.isHeld)
    }

    func testTheSessionSwitchHoldsWhatIsOnAirAndGivesItBack() async {
        let a = virtualAudio()
        a.programmePlay(); XCTAssertTrue(a.programmeAudible)
        a.holdForSessionSwitch(true)
        XCTAssertFalse(a.programmeAudible, "not played through a mode switch")
        a.holdForSessionSwitch(false)
        XCTAssertTrue(a.programmeAudible, "goes on from the held position")
    }
}
