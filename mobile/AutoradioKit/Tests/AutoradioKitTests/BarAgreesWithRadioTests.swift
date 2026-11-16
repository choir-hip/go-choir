import XCTest
@testable import AutoradioKit

/// The bar's strip (BarInfo.state) and its button / the player (RadioInfo.playing) are read from one truth: a paused radio says
/// "Paused" in the strip and Play on the button, a playing one "Now playing" and Pause, for every kind of item the queue plays.
@MainActor
final class BarAgreesWithRadioTests: XCTestCase {
    func snap(paused: Bool) -> LabSnapshot { var s = LabSnapshot(); s.running = true; s.paused = paused; s.phase = "playing"; s.blockCount = 3; return s }
    func player(_ playing: Bool) -> ChoirPlayerView { var p = ChoirPlayerView(); p.signedIn = true; p.playing = playing; p.title = "T"; return p }
    let doc = ProgrammeDocument(id: "text:x", title: "T", blocks: [.speech(text: "One."), .speech(text: "Two."), .speech(text: "Three.")])
    let items: [PlayItem] = [
        .story(route: "r", title: "Story"), .page(url: URL(string: "https://x.test/p")!, title: "Page"), .document(id: "d", title: "Book", at: 0),
        .audio(url: URL(string: "https://x.test/a.mp3")!, title: "Clip", show: "Show", start: nil, end: nil, episodeId: nil),
    ]

    func both(_ q: PlayQueue, _ playing: Bool) -> (BarInfo, RadioInfo) {
        let tr = BarTracker()
        let p = player(playing), s = snap(paused: !playing)
        return (q.overlay(tr.barInfo(player: p, snapshot: s)), q.overlay(tr.radioInfo(player: p, snapshot: s, document: doc, route: "")))
    }

    func testPausedAndPlayingAgreeForEveryKind() async {
        for item in items {
            let q = PlayQueue(); let e = FakeEngine(); q.engine = e
            q.pageText = { _, t in (t, ["a", "b"]) }
            q.registerTextSource(kind: "document") { _, _ in ["a", "b"] }
            q.playNow(item); await q.settle()
            // playing
            var (bar, radio) = both(q, true)
            XCTAssertEqual(bar.state, "playing", "\(item.kind)"); XCTAssertTrue(radio.playing, "\(item.kind)"); XCTAssertTrue(bar.playing)
            // paused: remote audio is paused through the queue, the rest through the programme player
            if item.isRemoteAudio { q.pause() }
            (bar, radio) = both(q, false)
            XCTAssertEqual(bar.state, "paused", "\(item.kind)"); XCTAssertFalse(radio.playing, "\(item.kind)"); XCTAssertFalse(bar.playing)
            XCTAssertEqual(radio.paused, true)
            if item.isRemoteAudio { q.resume(); (bar, radio) = both(q, true); XCTAssertEqual(bar.state, "playing"); XCTAssertTrue(radio.playing) }
        }
    }

    func testABridgeIsNamedAfterTheProgramme() {
        XCTAssertEqual(SegmentKinds.label("bridge"), "Bridge")      // the kind's own word stays for the player's kind line
        XCTAssertEqual(SegmentPrep.title("", kind: "bridge"), "Autoradio")
        XCTAssertEqual(SegmentPrep.title("Next up: the long read", kind: "bridge"), "Next up: the long read")
        XCTAssertEqual(SegmentPrep.title("", kind: "ident"), "Station break")
    }
}
