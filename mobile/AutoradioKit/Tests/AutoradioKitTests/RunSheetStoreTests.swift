import XCTest
@testable import AutoradioKit

// Slice B: the phone on one run sheet. The failure list (each is a test below, in this order):
//  1  the phone's opening is on air at once, with no network and no model, built from the front page it holds; with none it still opens
//  2  the desk's entries are placed at the end and never change what is on air
//  3  skip, next and a swipe are one op, `skip(on air)`; the next is the next entry, never the paper's neighbour
//  4  skip with nothing ahead stops the sound and swaps nothing in
//  5  an entry that ends is reported heard and the next one is aired
//  6  a desk that fails is said once, plainly, and the sheet keeps playing what it has
//  7  a desk that is slow does not swap a story in: the opening keeps playing
//  8  only a play changes what is on air (reads and views change nothing)
//  9  a transition whose neighbours changed is shown stale
// 10  a skipped clip ends as skipped and its sound stops; a clip is found in the served file by fingerprint (ClipLocatorTests)
// 11  pending ops survive a relaunch, stay while the route says "not yet", and go when it answers
// 12  the old kind names map: ident, bridge, update, headlines, podcast_clip
// 13  the ahead list is bounded at 100; a listener's own placement is never refused
// 14  an entry left on air by a dead app is closed as left, not resumed
// 15  "queue this" goes after the listener's last placement; "play this" cuts and keeps what is ahead
// 16  previous airs the entry before
// 17  more is asked for when the time ahead is short, one at a time, and not again when it added nothing
// 18  Up next is the first three ahead
// 19  the turn route's queue words translate to and from the sheet, and a replayed turn applies once
// 20  a duplicate op_id changes nothing; a batch is at most 64 ops

@MainActor
final class FakeEngine: PlaybackEngine {
    var log: [String] = []
    var failSpeak: Set<String> = []
    var duration: Double?
    var elapsed = 0.0
    var position: (block: Int, seconds: Double) = (0, 0)
    var holdSpeak: (() async -> Void)?
    func speak(documentID: String, entry: RunSheetEntry, paragraphs: [String]) async -> Bool { await holdSpeak?(); log.append("speak:\(entry.id)"); return !failSpeak.contains(entry.id) }
    func playStory(route: String, title: String) async -> String? { log.append("story:\(route)"); return failSpeak.contains(route) ? nil : "choir:\(route)" }
    var report: (block: Int, final: Int, seconds: Double) = (0, 0, 0)
    func spokenReport() -> (block: Int, final: Int, seconds: Double) { report }
    func restartSpoken() { log.append("restart") }
    func resumeSpoken() { log.append("resumeSpoken") }
    var remainder: String?
    var rendersSound = false
    func rendering() -> Bool { rendersSound }
    func pauseSpoken() { log.append("pauseSpoken") }
    func spokenRemainder() -> String? { remainder }
    func spokenPosition() -> (block: Int, seconds: Double) { position }
    func stopSpoken() { log.append("stopSpoken") }
    func endSpokenAfterCurrentBlock() { log.append("endAfterBlock") }
    func playRemote(url: URL, title: String, show: String, start: Double?, end: Double?) async -> Bool {
        log.append("remote:\(url.lastPathComponent):\(start ?? -1):\(end ?? -1)"); return true
    }
    func pauseRemote() { log.append("pauseRemote") }
    func resumeRemote() { log.append("resumeRemote") }
    func stopRemote() { log.append("stopRemote") }
    func seekRemote(by s: Double) { log.append("seekBy:\(s)") }
    func seekRemote(to s: Double) { log.append("seekTo:\(s)") }
    func clearRemoteEnd() { log.append("clearEnd") }
    func remotePosition() -> (elapsed: Double, duration: Double?)? { (elapsed, 600) }
    func setRemoteRate(_ r: Double) {}
    func setRemoteLevel(_ l: PlayLevel) {}
    func remoteDuration(url: URL, deadline: TimeInterval) async -> Double? { log.append("duration"); return duration }
    var speaks: [String] { log.filter { $0.hasPrefix("speak:") } }
}

final class FakeFeed: RunSheetFeed, @unchecked Sendable {
    let lock = NSLock()
    var script: [RunSheetFeedEvent] = []
    var scripts: [[RunSheetFeedEvent]] = []         // one per ask, when given
    var failure: Error?
    var hold = false
    private(set) var calls: [RunSheetAsk] = []
    private var cont: AsyncThrowingStream<RunSheetFeedEvent, Error>.Continuation?
    func ask(_ a: RunSheetAsk) async throws -> AsyncThrowingStream<RunSheetFeedEvent, Error> {
        lock.lock(); calls.append(a); let n = calls.count; let fail = failure; let events = scripts.indices.contains(n - 1) ? scripts[n - 1] : script; let h = hold; lock.unlock()
        if let fail { throw fail }
        return AsyncThrowingStream { c in
            self.lock.lock(); self.cont = c; self.lock.unlock()
            for e in events { c.yield(e) }
            if !h { c.finish() }
        }
    }
    func release() { lock.lock(); let c = cont; lock.unlock(); c?.finish() }
}

@MainActor
final class RunSheetStoreTests: XCTestCase {
    func make(storage: URL? = nil) -> (RunSheetPlayer, RunSheetStore, FakeEngine) {
        let s = RunSheetStore(storage: storage)
        let p = RunSheetPlayer(store: s)
        let e = FakeEngine(); p.engine = e
        p.pageText = { _, t in (t, ["one", "two", "three"]) }
        p.registerTextSource(kind: "document") { _, at in Array(["a", "b", "c", "d"].dropFirst(at)) }
        return (p, s, e)
    }
    func story(_ id: String, text: String = "Words.", kind: RunSheetKind = .story, secs: Double = 10) -> RunSheetEntry {
        RunSheetEntry(id: id, kind: kind, placedBy: .desk, title: "T \(id)", text: text, estSeconds: secs)
    }
    func deskPlace(_ s: RunSheetStore, _ e: RunSheetEntry) { s.applyFromDesk(ProgrammeRouteFeed.placeOp(e, cause: "open")) }
    func clipEntry(_ n: String, end: Double? = 30) -> RunSheetEntry { .audio(RunSheetAudio(url: URL(string: "https://x.test/\(n).mp3")!, title: "T \(n)", show: "Show", start: 10, end: end, episodeId: "e\(n)")) }
    func lines(_ n: Int) -> [PhoneOpening.Line] { (1...n).map { PhoneOpening.Line(title: "Story \($0)", dek: "Dek \($0).") } }
    /// An opening on air with `ahead` desk entries behind it.
    func opened(_ ahead: [String], _ p: RunSheetPlayer, _ s: RunSheetStore) async -> String {
        let o = PhoneOpening.entry(lines: lines(3), now: Date(timeIntervalSince1970: 1_700_000_000))
        p.open(o); await p.settle()
        for a in ahead { deskPlace(s, story(a)) }
        return o.id
    }

    // 1
    func test01_TheOpeningIsOnAirAtOnceFromTheFrontPageTheListenerHolds() async {
        let (p, s, e) = make()
        let o = PhoneOpening.entry(lines: lines(3))
        XCTAssertEqual(o.kind, .opening); XCTAssertEqual(o.placedBy, .phone)
        XCTAssertTrue(o.text!.contains("From the paper's front page: Story 1. Dek 1."), o.text!)
        XCTAssertTrue(o.text!.contains("Story 2") && o.text!.contains("Story 3"))
        p.open(o); await p.settle()
        XCTAssertEqual(s.onAir()?.id, o.id)
        XCTAssertEqual(e.speaks, ["speak:" + o.id], "no network and no model were asked: the engine spoke the phone's own words")
        XCTAssertEqual(s.pending.first?.op, .place)
    }
    func test01_TheOpeningReadsNaturallyForOneTwoAndManyStoriesAndNeverReadsADekTwice() {
        let d = Date(timeIntervalSince1970: 1_700_000_000)
        let one = PhoneOpening.entry(lines: [.init(title: "Harbour barrier approved", dek: "Council voted seven to two. A long second sentence.")], now: d).text!
        XCTAssertTrue(one.hasSuffix("From the paper's front page: Harbour barrier approved. Council voted seven to two."), one)
        let two = PhoneOpening.entry(lines: [.init(title: "A", dek: "Dek a."), .init(title: "B", dek: "Dek b.")], now: d).text!
        XCTAssertTrue(two.hasSuffix("A. Dek a. Also: B."), two); XCTAssertFalse(two.contains("Dek b"))
        let many = PhoneOpening.entry(lines: (1...8).map { .init(title: "Story \($0)", dek: "Dek \($0).") }, now: d).text!
        XCTAssertTrue(many.hasSuffix("Story 1. Dek 1. Also: Story 2; and Story 3."), many); XCTAssertFalse(many.contains("Story 4"))
        let echo = PhoneOpening.entry(lines: [.init(title: "Tide barrier approved", dek: "Tide barrier approved by the council.")], now: d).text!
        XCTAssertEqual(echo.components(separatedBy: "Tide barrier approved").count - 1, 1, "a dek that repeats the headline is not read")
    }
    func test01_WithNoFrontPageItStillOpens() async {
        let (p, s, _) = make()
        let o = PhoneOpening.entry(lines: [])
        XCTAssertTrue(o.text!.contains("This is Autoradio."))
        p.open(o); await p.settle()
        XCTAssertEqual(s.onAir()?.kind, .opening)
    }

    // 2
    func test02_TheDeskEntriesArePlacedAtTheEndAndTheOpeningStaysOnAir() async {
        let (p, s, _) = make()
        let o = await opened([], p, s)
        let feed = FakeFeed()
        feed.script = [.ops([ProgrammeRouteFeed.placeOp(story("d-1"), cause: "open")]), .ops([ProgrammeRouteFeed.placeOp(story("d-2"), cause: "open")]), .done]
        let w = RunSheetWriter(store: s) { feed }
        w.open(); await w.settle()
        XCTAssertEqual(s.sheet.entries.map(\.id), [o, "d-1", "d-2"])
        XCTAssertEqual(s.onAir()?.id, o)
        XCTAssertEqual(s.pending.filter { $0.by == .desk }.count, 0, "the desk's ops are the desk's: not posted back")
        XCTAssertEqual(feed.calls.first?.cause, "open")
    }

    // 3
    func test03_SkipIsOneOpAndTheNextIsTheNextEntry() async {
        let (p, s, e) = make()
        let o = await opened(["d-1", "d-2"], p, s)
        let before = s.pending.count
        p.next(); await p.settle()
        XCTAssertEqual(s.onAir()?.id, "d-1")
        XCTAssertEqual(s.entry(o)?.state, .skipped); XCTAssertEqual(s.entry(o)?.ended?.outcome, .skipped)
        XCTAssertEqual(s.pending[before...].map(\.op), [.skip, .aired], "one skip, then the phone reports the next on air")
        XCTAssertFalse(e.log.contains { $0.hasPrefix("story:") }, "the paper's list is never asked what is next")
        XCTAssertEqual(e.speaks.last, "speak:d-1")
    }
    func test03_TheLockScreensNextAndASwipeAreTheSameOp() async {
        let (p, s, _) = make()
        _ = await opened(["d-1", "d-2"], p, s)
        // ChoirApp.moveStory(.next) is `RunSheetPlayer.next()`: the swipe, the card's next and the lock screen all call it.
        p.next(); await p.settle()
        XCTAssertEqual(s.pending.filter { $0.op == .skip }.count, 1)
        XCTAssertEqual(s.onAir()?.id, "d-1")
    }

    // 4
    func test04_SkipWithNothingAheadStopsTheSoundAndSwapsNothingIn() async {
        let (p, s, e) = make()
        let o = await opened([], p, s)
        let spokeBefore = e.speaks.count
        p.next(); await p.settle()
        XCTAssertNil(s.onAir()); XCTAssertEqual(s.entry(o)?.state, .skipped)
        XCTAssertEqual(e.speaks.count, spokeBefore, "nothing new was spoken")
        XCTAssertTrue(e.log.contains("stopSpoken"))
        // and when the desk's entry arrives later, the radio that was playing carries on
        deskPlace(s, story("d-late")); await p.settle()
        XCTAssertEqual(s.onAir()?.id, "d-late")
    }

    // 5
    func test05_AnEntryThatEndsIsReportedHeardAndTheNextIsAired() async {
        let (p, s, e) = make()
        let o = await opened(["d-1"], p, s)
        p.documentEnded(id: "rs:" + o, anyPlayed: true); await p.settle()
        XCTAssertEqual(s.entry(o)?.state, .heard); XCTAssertEqual(s.entry(o)?.ended?.outcome, .finished)
        XCTAssertEqual(s.onAir()?.id, "d-1"); XCTAssertEqual(e.speaks.last, "speak:d-1")
        // a late end of a document that was replaced closes nothing
        p.documentEnded(id: "rs:" + o, anyPlayed: true)
        XCTAssertEqual(s.onAir()?.id, "d-1")
    }
    func test05_TheEndWaitsForTheListenersTurn() async {
        let (p, s, _) = make()
        let o = await opened(["d-1"], p, s)
        var open = true
        p.turnOpen = { open }
        p.documentEnded(id: "rs:" + o, anyPlayed: true)
        XCTAssertEqual(s.onAir()?.id, o, "the listener is talking: the entry is not closed over their words")
        open = false
        p.turnEnded(); await p.settle()
        XCTAssertEqual(s.onAir()?.id, "d-1")
    }
    func test05_AnEntryThatCannotPlayIsSkippedOnceAndTwoInARowStop() async {
        let (p, s, e) = make()
        e.failSpeak = ["d-1", "d-2"]
        _ = await opened(["d-1", "d-2", "d-3"], p, s)
        p.next(); await p.settle(); await p.settle()
        XCTAssertEqual(s.entry("d-1")?.state, .skipped); XCTAssertEqual(s.entry("d-2")?.state, .skipped)
        XCTAssertNil(s.onAir(), "two in a row that make no sound stop the radio")
        XCTAssertEqual(s.entry("d-3")?.state, .ahead)
        XCTAssertFalse(p.message.isEmpty)
    }

    // 6, 7
    func test06_ADeskThatFailsIsAQuietNoticeNotAnEntryAndTheOpeningKeepsPlaying() async {
        let (p, s, e) = make()
        let o = await opened([], p, s)
        let feed = FakeFeed(); feed.failure = ChoirError.transport("offline")
        let w = RunSheetWriter(store: s) { feed }
        var told: [String?] = []
        w.onNotice = { told.append($0) }
        w.open(); await w.settle()
        XCTAssertEqual(s.onAir()?.id, o)
        XCTAssertTrue(s.ahead(limit: 5).isEmpty, "no row for it: it is not in Up next and cannot be skipped to")
        XCTAssertEqual(w.notice, "The desk can't be reached right now.")
        w.open(); await w.settle()
        XCTAssertEqual(told.count, 1, "told once")
        XCTAssertFalse(e.log.contains { $0.hasPrefix("story:") }, "no story was swapped in")
        feed.failure = nil; feed.script = [.ops([ProgrammeRouteFeed.placeOp(story("d-1"), cause: "open")]), .done]
        w.open(); await w.settle()
        XCTAssertNil(w.notice, "the notice goes when the desk answers")
    }
    func test07_ASlowDeskSwapsNothingIn() async {
        let (p, s, e) = make()
        let o = await opened([], p, s)
        let feed = FakeFeed(); feed.hold = true
        let w = RunSheetWriter(store: s) { feed }
        w.open()
        try? await Task.sleep(nanoseconds: 200_000_000)
        XCTAssertTrue(w.inFlight)
        XCTAssertEqual(s.onAir()?.id, o); XCTAssertEqual(e.speaks.count, 1)
        XCTAssertFalse(e.log.contains { $0.hasPrefix("story:") })
        feed.release(); await w.settle()
    }

    // 8
    func test08_OnlyAPlayChangesWhatIsOnAir() async {
        let (p, s, _) = make()
        let o = await opened(["d-1"], p, s)
        let v = s.sheet.version
        _ = p.info(); _ = p.turnQueueJSON(); _ = s.ahead(limit: 3); _ = s.heard(limit: 3)      // everything the screens read
        XCTAssertEqual(s.sheet.version, v, "reading changes nothing")
        XCTAssertEqual(s.onAir()?.id, o)
        p.playNow(.readAloud(address: "mail:m1", title: "A message"), words: ("A message", ["Hello."])); await p.settle()
        XCTAssertEqual(s.onAir()?.kind, .mail, "reading a message aloud is a play, and a play is `place now`")
        XCTAssertEqual(s.entry(o)?.ended?.outcome, .cut)
        XCTAssertEqual(s.ahead(limit: 5).map(\.id), ["d-1"], "what was ahead stays ahead")
    }

    // 9
    func test09_ATransitionWhoseNeighboursChangedIsShownStale() {
        let (_, s, _) = make()
        deskPlace(s, story("d-a"))
        var t = story("d-t", text: "And now.", kind: .transition); t.between = RunSheetBetween(after: "d-a", before: "d-b")
        deskPlace(s, t)
        deskPlace(s, story("d-b"))
        XCTAssertFalse(s.isStale(s.entry("d-t")!))
        s.applyFromDesk(RunSheetOp(opId: "o-rm", by: .desk, op: .remove, id: "d-b", reason: "revised"))
        XCTAssertTrue(s.isStale(s.entry("d-t")!), "the story after it is gone: the phone says so and leaves it")
        XCTAssertEqual(s.entry("d-t")?.state, .ahead)
    }

    // 10
    func test10_ASkippedClipEndsAsSkippedAndItsSoundStops() async {
        let (p, s, e) = make()
        let c = clipEntry("a")
        p.playNow(c); await p.settle()
        XCTAssertEqual(e.log.last, "remote:a.mp3:10.0:30.0")
        XCTAssertTrue(p.isRemote)
        p.next(); await p.settle()
        XCTAssertEqual(s.entry(c.id)?.state, .skipped); XCTAssertEqual(s.entry(c.id)?.ended?.outcome, .skipped)
        XCTAssertTrue(e.log.contains("stopRemote")); XCTAssertFalse(p.isRemote)
    }
    func test10_AClipThatEndsIsHeardAndKeepListeningHoldsIt() async {
        let (p, s, e) = make()
        let c = clipEntry("a")
        p.playNow(c); await p.settle()
        XCTAssertEqual(p.keepListeningTitle, "T a (Show)")
        p.keepListening()
        XCTAssertNil(p.keepListeningTitle); XCTAssertEqual(e.log.last, "clearEnd")
        p.remoteEnded(generation: p.generation)
        XCTAssertEqual(s.entry(c.id)?.state, .heard)
    }
    // (a clip of another file is no longer judged by its length: ClipLocatorTests)

    // 11
    final class FakePoster: RunSheetPoster, @unchecked Sendable {
        let lock = NSLock()
        var answer: RunSheetPostOutcome = .notYet
        private(set) var posts: [[String]] = []
        func post(ops: [RunSheetOp]) async -> RunSheetPostOutcome { lock.lock(); posts.append(ops.map(\.opId)); let a = answer; lock.unlock(); return a }
    }
    func test11_PendingOpsSurviveARelaunchAndGoWhenTheRouteAnswers() async {
        let f = FileManager.default.temporaryDirectory.appendingPathComponent("rs-\(UUID().uuidString).json")
        let (p, s, _) = make(storage: f)
        let poster = FakePoster(); s.poster = poster; s.retryAfter = 3600
        p.playNow(.paperStory(route: "a", title: "A")); await p.settle(); await s.flushTask?.value
        XCTAssertEqual(s.pending.count, 1, "the route said not yet: the op stays, with its op_id")
        let opId = s.pending[0].opId
        // a relaunch
        let s2 = RunSheetStore(storage: f); let p2 = RunSheetPlayer(store: s2); p2.engine = FakeEngine()
        let poster2 = FakePoster(); s2.poster = poster2; s2.retryAfter = 3600
        s2.restore(); await s2.flushTask?.value
        XCTAssertEqual(s2.pending.map(\.opId).first, opId, "the same op, in order")
        XCTAssertNil(s2.onAir(), "an entry that was on air is closed, not resumed")
        XCTAssertEqual(s2.entry(s2.sheet.entries[0].id)?.ended?.outcome, .failed)
        poster2.answer = .answered([]); s2.flush(); await s2.flushTask?.value
        XCTAssertTrue(s2.pending.isEmpty)
        XCTAssertEqual(poster2.posts.last?.first, s2.sheet.appliedOps.first, "the first op sent is the first applied")
        _ = p
    }
    func test11_Offline() async {
        let (p, s, _) = make()
        let poster = FakePoster(); poster.answer = .offline; s.poster = poster; s.retryAfter = 3600
        p.playNow(.paperStory(route: "a", title: "A")); await s.flushTask?.value
        XCTAssertEqual(s.pending.count, 1)
    }

    // 12
    func test12_TheOldKindNamesMap() {
        let clips = ClipStore(); clips.replace([PodcastClip.parse(["clip_id": "clip-0000000000b1", "story": ["route": "/a", "version": 1, "title": "S"], "show": "Show",
            "audio_url": "https://example.test/b.mp3", "duration_seconds": 900, "start": 10, "end": 70, "quote": "q", "support": 0.9, "fingerprint": testFingerprintJSON()])!])
        func seg(_ id: String, _ kind: String, clip: String? = nil, between: IncomingBetween? = nil) -> IncomingSegment {
            IncomingSegment(stretchID: "st", segmentID: id, kind: kind, title: "Title \(id)", text: "Words for \(id).", cites: [ProgrammeCite(ref: "pub:a@v2", role: "subject", label: "A")],
                            blockCount: nil, clip: clip.map { ProgrammeClipRef(clipID: $0) }, between: between)
        }
        func k(_ s: IncomingSegment) -> RunSheetKind? { ProgrammeRouteFeed.entry(from: s, clips: clips)?.kind }
        XCTAssertEqual(k(seg("a", "ident")), .opening); XCTAssertEqual(k(seg("b", "opening")), .opening)
        XCTAssertEqual(k(seg("c", "bridge")), .transition); XCTAssertEqual(k(seg("d", "transition")), .transition)
        XCTAssertEqual(k(seg("e", "story")), .story); XCTAssertEqual(k(seg("f", "update")), .story); XCTAssertEqual(k(seg("g", "headlines")), .story)
        XCTAssertEqual(k(seg("h", "podcast_clip", clip: "clip-0000000000b1")), .clip)
        XCTAssertNil(k(seg("i", "podcast_clip", clip: "clip-nothere")), "a clip the phone does not have is not an entry")
        let t = ProgrammeRouteFeed.entry(from: seg("t", "transition", between: IncomingBetween(after: "st/1", before: "st/2")), clips: clips)!
        XCTAssertEqual(t.between, RunSheetBetween(after: "d-st-1", before: "d-st-2"))
        XCTAssertEqual(t.id, "d-t"); XCTAssertEqual(t.placedBy, .desk); XCTAssertEqual(t.source?.ref, "pub:a@v2"); XCTAssertTrue(RunSheet.validId(t.id))
        // the decoder reads `between` and the new names
        var d = StretchDecoder()
        let ev = d.feed("event: segment\ndata: {\"segment_id\":\"s2\",\"kind\":\"transition\",\"text\":\"Next.\",\"between\":{\"after_segment\":\"s1\",\"before_segment\":\"s3\"}}\n\n") + d.finish()
        guard case .segment(let got)? = ev.first else { return XCTFail("no segment") }
        XCTAssertEqual(got.between, IncomingBetween(after: "s1", before: "s3"))
    }

    // 13
    func test13_TheAheadListIsBoundedAt100() {
        let (_, s, _) = make()
        for n in 0..<100 { deskPlace(s, story("d-\(n)")) }
        XCTAssertEqual(s.ahead(limit: 1000).count, 100)
        let r = s.applyFromDesk(ProgrammeRouteFeed.placeOp(story("d-over"), cause: "extend"))
        XCTAssertFalse(r.applied); XCTAssertEqual(r.dropped, "ahead_full")
        let mine = RunSheetEntry.paperStory(route: "r", title: "Mine")
        XCTAssertTrue(s.place(mine, at: .end).applied, "a listener's own placement is never refused")
        let ahead = s.ahead(limit: 1000)
        XCTAssertEqual(ahead.count, 100); XCTAssertEqual(ahead.last?.id, mine.id); XCTAssertNil(s.entry("d-99"), "it took the place of the last entry")
    }

    // 14
    func test14_AnEntryLeftOnAirByADeadAppIsClosedAsFailedNeverLeft() async {
        let f = FileManager.default.temporaryDirectory.appendingPathComponent("rs-\(UUID().uuidString).json")
        let (p, s, _) = make(storage: f)
        let o = await opened(["d-1"], p, s)
        let s2 = RunSheetStore(storage: f); let p2 = RunSheetPlayer(store: s2); let e2 = FakeEngine(); p2.engine = e2
        s2.restore(); await p2.settle()
        XCTAssertNil(s2.onAir()); XCTAssertEqual(s2.entry(o)?.state, .skipped); XCTAssertEqual(s2.entry(o)?.ended?.outcome, .failed)
        XCTAssertTrue(e2.speaks.isEmpty, "a crash restart never resumes work")
        XCTAssertEqual(s2.entry("d-1")?.state, .ahead)
    }

    func test14_AnOpeningTakesOffWhatAnEarlierListeningLeftAheadButKeepsTheListenersOwn() async {
        let (p, s, _) = make()
        deskPlace(s, story("d-old")); s.place(.paperStory(route: "mine", title: "Mine"), at: .end)
        let o = PhoneOpening.entry(lines: lines(1))
        p.open(o); await p.settle()
        XCTAssertNil(s.entry("d-old")); XCTAssertEqual(s.ahead(limit: 5).count, 1); XCTAssertEqual(s.onAir()?.id, o.id)
    }

    // 15
    func test15_QueueThisGoesAfterTheListenersLastAndPlayThisKeepsWhatIsAhead() async {
        let (p, s, _) = make()
        _ = await opened(["d-1", "d-2"], p, s)
        let a = RunSheetEntry.paperStory(route: "a", title: "A"), b = RunSheetEntry.paperStory(route: "b", title: "B")
        p.queue(a); p.queue(b)
        // with no listener placement yet, `queued` is directly after the entry on air; the second goes after the first
        XCTAssertEqual(s.ahead(limit: 10).map(\.id), [a.id, b.id, "d-1", "d-2"])
        let c = RunSheetEntry.paperStory(route: "c", title: "C")
        p.playNow(c); await p.settle()
        XCTAssertEqual(s.onAir()?.id, c.id)
        XCTAssertEqual(s.ahead(limit: 10).map(\.id), [a.id, b.id, "d-1", "d-2"], "play this cuts what was on air and keeps what was ahead")
    }

    // 16
    func test16_PreviousAirsTheEntryBefore() async {
        let (p, s, e) = make()
        let o = await opened(["d-1"], p, s)
        p.next(); await p.settle()
        XCTAssertEqual(s.onAir()?.id, "d-1")
        e.position = (0, 0.5)
        p.previous(); await p.settle()
        XCTAssertEqual(s.onAir()?.id, o, "barely begun: the one before")
        XCTAssertEqual(e.speaks.last, "speak:" + o)
        e.position = (2, 8)
        let n = e.log.count
        p.previous()
        XCTAssertEqual(e.log.dropFirst(n).first, "restart", "well into it: its own start")
    }

    // 17
    func test17_MoreIsAskedForWhenTheTimeAheadIsShortOneAtATimeAndNotAgainWhenItAddedNothing() async {
        let (p, s, _) = make()
        _ = await opened([], p, s)
        let feed = FakeFeed()
        feed.scripts = [[.ops([ProgrammeRouteFeed.placeOp(story("d-1", secs: 3), cause: "open")]), .done], [.done], [.done]]
        let t0 = Date()
        var tick = 0
        let w = RunSheetWriter(store: s) { feed }
        w.clock = { tick += 1; return t0.addingTimeInterval(Double(tick) * 4) }       // each look at the clock is 4 s later: the write took a few seconds
        p.aheadChanged = { [weak w] in w?.checkAhead() }
        w.open(); await w.settle(); await w.settle(); await w.settle()
        XCTAssertGreaterThan(w.latency, 0)
        XCTAssertEqual(feed.calls.map(\.cause), ["open", "extend"], "3 s ahead is less than twice the latency: one more is asked; it added nothing, so no third")
        XCTAssertEqual(feed.calls[1].aheadTitles, ["T d-1"])
        XCTAssertFalse(w.inFlight)
        p.next(); await p.settle(); await w.settle()                                  // another entry starts: the desk may be asked again
        XCTAssertEqual(feed.calls.count, 3)
    }

    // 18
    func test18_UpNextIsEveryEntryAheadInOrder() async {
        let (p, s, _) = make()
        _ = await opened(["d-1", "d-2", "d-3", "d-4", "d-5"], p, s)
        let i = p.info()
        let page = RadioPage.make(RadioPageInput(sheet: s.sheet))
        XCTAssertEqual(page.ahead.map(\.id), ["d-1", "d-2", "d-3", "d-4", "d-5"], "the page lists every entry ahead, in order")
        XCTAssertTrue(page.ahead.allSatisfy { $0.by == "desk" })
        XCTAssertEqual(i.current, "Opening")
    }

    // 19
    func test19_TheTurnRoutesQueueWordsTranslateAndAReplayedTurnAppliesOnce() async throws {
        let (p, s, _) = make()
        _ = await opened(["d-1"], p, s)
        let mine = RunSheetEntry.paperStory(route: "r1", title: "Mine"); p.queue(mine)
        let o = try XCTUnwrap(JSONSerialization.jsonObject(with: try XCTUnwrap(p.turnQueueJSON())) as? [String: Any])
        let up = try XCTUnwrap(o["upcoming"] as? [[String: Any]])
        XCTAssertEqual(up.first?["id"] as? String, mine.id)
        XCTAssertEqual((up.first?["item"] as? [String: Any])?["kind"] as? String, "story")
        XCTAssertEqual(up.first?["added_by"] as? String, "listener")
        XCTAssertEqual(up.count, 1, "spoken entries have no address to hand over")
        let body = #"{"turn_id":"t9","ops":[{"op":"enqueue","item":{"kind":"story","route":"r2","title":"Two"},"reason":"you asked"},{"op":"remove","id":"\#(mine.id)"},{"op":"remove","id":"nope"}]}"#
        let r = p.applyTurnOps(Data(body.utf8))
        XCTAssertEqual(r.applied, 2); XCTAssertEqual(r.dropped, 1)
        XCTAssertNil(s.entry(mine.id)); XCTAssertEqual(s.ahead(limit: 10).last?.paperRoute, "r2")
        let again = p.applyTurnOps(Data(body.utf8))
        XCTAssertEqual(again.applied, 0, "a replayed turn changes nothing")
        let skip = p.applyTurnOps(Data(#"[{"op":"skip"}]"#.utf8)); await p.settle()
        XCTAssertEqual(skip.applied, 1); XCTAssertEqual(s.onAir()?.id, "d-1")
    }

    // 20
    func test20_ADuplicateOpIdChangesNothingAndABatchIsAtMost64() async {
        let (_, s, _) = make()
        let e = RunSheetEntry.paperStory(route: "a", title: "A")
        let op = RunSheetOp(opId: "p-dup00001", by: .listener, op: .place, at: .end, entry: e)
        XCTAssertTrue(s.apply(op).applied)
        let again = s.apply(op)
        XCTAssertFalse(again.applied); XCTAssertEqual(again.dropped, RunSheetDropped.duplicate)
        XCTAssertEqual(s.pending.count, 1)
        for n in 0..<150 { s.place(.paperStory(route: "r\(n)", title: "R"), at: .end) }
        let poster = FakePoster(); poster.answer = .answered([]); s.poster = poster
        s.flush(); await s.flushTask?.value
        XCTAssertTrue(s.pending.isEmpty)
        XCTAssertTrue(poster.posts.allSatisfy { $0.count <= 64 }); XCTAssertGreaterThan(poster.posts.count, 2)
        XCTAssertEqual(poster.posts.flatMap { $0 }, s.sheet.appliedOps, "every applied op was sent once, in order")
    }

    // 21 the phone's opening is what plays until the desk's arrives; the station is never introduced twice
    func deskOpening(_ id: String = "d-open") -> RunSheetEntry {
        RunSheetEntry(id: id, kind: .opening, placedBy: .desk, title: "Opening", text: "This is Autoradio.", estSeconds: 5)
    }
    func writerWith(_ s: RunSheetStore, _ p: RunSheetPlayer, _ ops: [RunSheetEntry]) -> (RunSheetWriter, FakeFeed) {
        let feed = FakeFeed()
        feed.script = ops.map { .ops([ProgrammeRouteFeed.placeOp($0, cause: "open")]) } + [.done]
        let w = RunSheetWriter(store: s) { feed }
        w.admit = { op, cause in p.admitDesk(op, cause: cause) }
        return (w, feed)
    }
    func test21_TheDeskOpeningBeforeThePhonesBeganReplacesIt() async {
        let (p, s, _) = make()
        let phone = PhoneOpening.entry(lines: lines(1))
        s.place(phone, at: .end, by: .phone); p.setOpeningForTest(phone.id)          // placed, not yet on air
        let (w, _) = writerWith(s, p, [deskOpening()])
        w.open(); await w.settle()
        XCTAssertNil(s.entry(phone.id), "the phone's is taken off")
        XCTAssertEqual(s.ahead(limit: 5).map(\.id), ["d-open"])
    }
    func test21_TheDeskOpeningWhileThePhonesIsOnAirIsDroppedAndWhatFollowsIsAdmitted() async {
        let (p, s, e) = make()
        let o = await opened([], p, s)
        let (w, _) = writerWith(s, p, [deskOpening(), story("d-1")])
        w.open(); await w.settle()
        XCTAssertFalse(e.log.contains("endAfterBlock"), "the phone's opening plays whole: no second one follows it")
        XCTAssertEqual(s.onAir()?.id, o)
        XCTAssertEqual(s.ahead(limit: 5).map(\.id), ["d-1"], "one opening heard; the entries after it are admitted as usual")
        XCTAssertEqual(p.droppedFromDesk.map(\.reason), ["opening_one_per_start"])
    }
    func test21_AnOpeningHeardTenMinutesAgoMeansNoOpeningNowAndThirtyOneMinutesMeansOne() async {
        for (minutes, expectOpening) in [(10.0, false), (31.0, true)] {
            let (p, s, _) = make()
            let o = await opened([], p, s)
            p.documentEnded(id: "rs:" + o, anyPlayed: true); await p.settle()
            XCTAssertEqual(s.entry(o)?.state, .heard)
            let ended = ISO8601DateFormatter().date(from: s.entry(o)?.ended?.at ?? "") ?? Date()
            s.now = { ended.addingTimeInterval(minutes * 60) }
            p.open(PhoneOpening.entry(lines: lines(1)))                                   // the radio is opened again
            let openings = s.sheet.entries.filter { $0.kind == .opening && $0.state != .heard }
            XCTAssertEqual(openings.count, expectOpening ? 1 : 0, "\(minutes) min after the last opening")
            let (w, _) = writerWith(s, p, [deskOpening("d-open2")])
            w.open(); await w.settle()
            let total = s.sheet.entries.filter { $0.kind == .opening && $0.id != o }.count
            XCTAssertEqual(total, expectOpening ? 1 : 0, "never two openings in a row (\(minutes) min)")
        }
    }
    func test21_TheCachedOpeningIsTheOpeningOfThisStartAndObeysTheSameRule() async {
        let (p, s, _) = make()
        let cached = RunSheetEntry(id: "cache-1", kind: .opening, placedBy: .phone, title: "Opening", text: "Good evening. Written ahead.", estSeconds: 5)
        p.open(cached)                                                                      // the written-ahead opening starts the launch
        XCTAssertEqual(s.onAir()?.id, "cache-1")
        let (w, _) = writerWith(s, p, [deskOpening(), story("d-1")])
        w.open(); await w.settle()
        XCTAssertNil(s.entry("d-open"), "the live opening is not a second one behind the cached one")
        XCTAssertEqual(s.sheet.entries.filter { $0.kind == .opening }.count, 1)
        // it was heard ten minutes ago and the radio is opened again with the cached opening: none is placed
        p.documentEnded(id: "rs:cache-1", anyPlayed: true); await p.settle()
        let ended = ISO8601DateFormatter().date(from: s.entry("cache-1")?.ended?.at ?? "") ?? Date()
        s.now = { ended.addingTimeInterval(600) }
        var again = cached; again.id = "cache-2"
        p.open(again)
        XCTAssertNil(s.entry("cache-2"))
        var first = story("d-first"); first.placedBy = .phone
        p.open(first, asOpening: false)                                                      // an entry that is not an opening is never refused by it
        XCTAssertNotNil(s.entry("d-first"))
    }
    func test21_PlayWithoutALaunchOpeningStillFollowsAnEndedOpeningWithTheNextEntryUntouched() async {
        let (p, s, _) = make()                                                              // no open(): the station did not start (launch condition, other audio)
        s.place(deskOpening("d-open"), at: .end, by: .desk, reason: "test"); s.place(story("d-1"), at: .end, by: .desk, reason: "test")
        p.advanceIfIdle(); await p.settle()                                                 // the Play button
        XCTAssertEqual(s.onAir()?.id, "d-open")
        p.documentEnded(id: "rs:d-open", anyPlayed: true); await p.settle()
        XCTAssertEqual(s.onAir()?.id, "d-1", "the opening ends and the next entry is on air with no press")
    }
    func test21_AfterThePhonesWasHeardTheDeskOpeningIsDroppedNotPlayedAsASecondGreeting() async {
        let (p, s, _) = make()
        let o = await opened([], p, s)
        p.documentEnded(id: "rs:" + o, anyPlayed: true); await p.settle()
        XCTAssertEqual(s.entry(o)?.state, .heard)
        let (w, _) = writerWith(s, p, [deskOpening(), story("d-1")])
        w.open(); await w.settle()
        XCTAssertNil(s.entry("d-open"))
        XCTAssertEqual(p.droppedFromDesk.map(\.reason), ["opening_one_per_start"])
        XCTAssertEqual(s.onAir()?.id, "d-1", "the story the desk wrote plays")
    }

    // 22 transitions
    func test22_ATransitionIsNotAnUpNextRowAndPlaysBetweenItsNeighbours() async {
        let (p, s, e) = make()
        let o = await opened([], p, s)
        deskPlace(s, story("d-a"))
        var t = story("d-t", text: "And now.", kind: .transition); t.title = ""; t.between = RunSheetBetween(after: "d-a", before: "d-b")
        deskPlace(s, t); deskPlace(s, story("d-b"))
        XCTAssertEqual(RadioPage.make(RadioPageInput(sheet: s.sheet)).ahead.map(\.id), ["d-a", "d-b"], "a transition is not listed")
        p.next(); await p.settle()                                   // d-a on air
        p.documentEnded(id: "rs:d-a", anyPlayed: true); await p.settle()
        XCTAssertEqual(s.onAir()?.id, "d-t", "heard to its end: the transition plays")
        p.documentEnded(id: "rs:d-t", anyPlayed: true); await p.settle()
        XCTAssertEqual(s.onAir()?.id, "d-b")
        _ = (o, e)
    }
    func test22_ASkippedPastNeighbourDropsItsTransition() async {
        let (p, s, _) = make()
        _ = await opened([], p, s)
        deskPlace(s, story("d-a"))
        var t = story("d-t", text: "And now.", kind: .transition); t.between = RunSheetBetween(after: "d-a", before: "d-b")
        deskPlace(s, t); deskPlace(s, story("d-b"))
        p.next(); await p.settle()                                   // d-a on air
        p.next(); await p.settle()                                   // d-a skipped: its transition is orphaned
        XCTAssertEqual(s.onAir()?.id, "d-b")
        XCTAssertNil(s.entry("d-t")); XCTAssertEqual(p.droppedFromDesk.map(\.reason), ["transition_orphaned"])
    }
    func test22_AnOpeningAndATransitionHaveTheTitlesTheListenerSees() {
        let seg = IncomingSegment(stretchID: "s", segmentID: "a", kind: "ident", title: "Autoradio", text: "Hello.", cites: [], blockCount: nil)
        XCTAssertEqual(ProgrammeRouteFeed.entry(from: seg, clips: ClipStore())?.title, "Opening")
        let tr = IncomingSegment(stretchID: "s", segmentID: "b", kind: "bridge", title: "", text: "Next.", cites: [], blockCount: nil)
        XCTAssertEqual(ProgrammeRouteFeed.entry(from: tr, clips: ClipStore())?.title, "")
        var d = segmentedDocument(["One."], kinds: ["transition"])
        d.segments[0].title = "Autoradio"
        var snap = LabSnapshot(); snap.running = true; var pv = ChoirPlayerView(); pv.playing = true
        XCTAssertEqual(BarTracker().radioInfo(player: pv, snapshot: snap, document: d, route: "", nextTitle: "Harbour story").kind, "Next: Harbour story")
    }

    // 23 write-ahead keeps the sheet fed
    func wired(_ s: RunSheetStore, _ p: RunSheetPlayer, _ feed: FakeFeed) -> RunSheetWriter {
        let w = RunSheetWriter(store: s) { feed }
        var tick = 0; let t0 = Date()
        w.clock = { tick += 1; return t0.addingTimeInterval(Double(tick) * 4) }
        w.admit = { op, cause in p.admitDesk(op, cause: cause) }
        w.onWriting = { p.deskWriting = $0 }; w.onDry = { p.deskDry = $0 }
        p.aheadChanged = { [weak w] in w?.checkAhead() }
        return w
    }
    func ops(_ ids: [String], secs: Double, cause: String = "open") -> [RunSheetFeedEvent] {
        ids.map { .ops([ProgrammeRouteFeed.placeOp(story($0, secs: secs), cause: cause)]) } + [.done]
    }
    // the trigger: spoken time ahead under max(2 x latency, 180 s), or fewer than two playable entries ahead
    func test23_ThreeEntriesOf20SecondsAheadAskForMore() async {
        let (p, s, _) = make()
        _ = await opened([], p, s)
        let feed = FakeFeed(); feed.scripts = [ops(["d-1", "d-2", "d-3"], secs: 20)]
        let w = wired(s, p, feed)
        w.open(); await w.settle(); await w.settle()
        XCTAssertEqual(feed.calls.map(\.cause), ["open", "extend"], "60 s ahead is under 180: written ahead")
    }
    func test23_ASkipToTheSecondLastEntryAsksAtOnceAndAFullSheetDoesNot() async {
        let (p, s, _) = make()
        _ = await opened([], p, s)
        let feed = FakeFeed(); feed.scripts = [ops(["d-1", "d-2", "d-3"], secs: 100)]       // 300 s, three playable
        let w = wired(s, p, feed)
        w.open(); await w.settle()
        XCTAssertEqual(feed.calls.count, 1, "a full sheet (over 180 s, two or more entries) asks for nothing")
        p.next(); await p.settle(); await w.settle()                     // d-1 on air, two ahead (200 s)
        XCTAssertEqual(feed.calls.count, 1)
        p.next(); await p.settle(); await w.settle()                     // d-2 on air, one ahead: fewer than two playable
        XCTAssertEqual(feed.calls.map(\.cause), ["open", "extend"], "asked at once, though 100 s is still ahead")
    }
    func test23_NoSecondAskWhileOneIsInFlightAndWritingShows() async {
        let (p, s, _) = make()
        _ = await opened([], p, s)
        let feed = FakeFeed(); feed.hold = true
        let w = wired(s, p, feed)
        w.open()
        try? await Task.sleep(nanoseconds: 100_000_000)
        for _ in 0..<5 { w.checkAhead(); deskPlace(s, story("d-\(UUID().uuidString.prefix(4))")) }
        XCTAssertEqual(feed.calls.count, 1, "one ask in flight")
        XCTAssertTrue(p.deskWriting && p.info().writing)
        feed.release(); await w.settle()
    }
    func test23_NothingIsComingOnlyWhenNoAskIsInFlightAndTheLastReturnedNothingNew() async {
        let (p, s, _) = make()
        _ = await opened([], p, s)
        let feed = FakeFeed()
        let told = story("d-1", secs: 20)
        var again = story("d-1b", secs: 20); again.title = told.title                                // a desk repeating a title already on the sheet
        feed.scripts = [[.ops([ProgrammeRouteFeed.placeOp(told, cause: "open")]), .done], [.ops([ProgrammeRouteFeed.placeOp(again, cause: "extend")]), .done]]
        let w = wired(s, p, feed)
        w.open(); await w.settle(); await w.settle()
        XCTAssertEqual(feed.calls.map(\.cause), ["open", "extend"])
        XCTAssertEqual(p.droppedFromDesk.map(\.reason), ["title_already_on_sheet"], "the repeat is a dropped reason")
        XCTAssertTrue(p.info().dry); XCTAssertFalse(p.info().writing)
        w.checkAhead(); await w.settle()
        XCTAssertEqual(feed.calls.count, 2, "no loop on a desk that keeps repeating: it waits for the next entry")
        p.next(); await p.settle(); await w.settle()                                                 // d-1 goes on air: a natural trigger
        XCTAssertEqual(feed.calls.count, 3)
    }
    // 24 honest air reports: every op the phone posts about what happened to an entry is evidence and must be true
    func lastOp(_ s: RunSheetStore, _ kind: RunSheetOpName, file: StaticString = #filePath, line: UInt = #line) -> RunSheetOp? {
        s.pending.last { $0.op == kind }
    }
    func test24_AFailureIsReportedAsFailedByThePhoneNeverAsAListenerSkip() async {
        let (p, s, e) = make()
        e.failSpeak = ["d-1"]
        _ = await opened(["d-1"], p, s)
        let before = s.pending.count
        p.next(); await p.settle()
        let ops = s.pending[before...]
        XCTAssertEqual(ops.filter { $0.op == .skip }.count, 1, "the listener's own skip of the opening")
        let failed = try? XCTUnwrap(ops.first { $0.op == .aired && $0.id == "d-1" && $0.state == .skipped })
        XCTAssertEqual(failed?.by, .phone); XCTAssertEqual(failed?.ended?.outcome, .failed)
        XCTAssertEqual(s.entry("d-1")?.ended?.outcome, .failed)
    }
    func test24_SignOutLeavesTheAirAsLeftByThePhoneAtTheTruePosition() async {
        let (p, s, e) = make()
        let o = await opened(["d-1"], p, s)
        e.report = (2, 5, 31.5)
        let before = s.pending.count
        p.stopAll()
        let ops = Array(s.pending[before...])
        XCTAssertFalse(ops.contains { $0.op == .skip }, "no listener skip")
        let left = try? XCTUnwrap(ops.first { $0.op == .aired })
        XCTAssertEqual(left?.id, o); XCTAssertEqual(left?.by, .phone); XCTAssertEqual(left?.state, .heard)
        XCTAssertEqual(left?.ended?.outcome, .left); XCTAssertEqual(left?.ended?.lastBlock, 2); XCTAssertEqual(left?.positionSeconds, 31.5)
        XCTAssertEqual(ops.first { $0.op == .clear }?.by, .phone)
    }
    func test24_ANaturalEndCarriesTheFinalBlockAndPosition() async {
        let (p, s, e) = make()
        let o = await opened([], p, s)
        e.report = (3, 3, 42)
        p.documentEnded(id: "rs:" + o, anyPlayed: true)
        let op = lastOp(s, .aired)
        XCTAssertEqual(op?.id, o); XCTAssertEqual(op?.by, .phone); XCTAssertEqual(op?.state, .heard)
        XCTAssertEqual(op?.ended?.outcome, .finished); XCTAssertEqual(op?.ended?.lastBlock, 3); XCTAssertEqual(op?.positionSeconds, 42)
    }
    func test24_AListenerSkipCarriesTheBlockBeingSpokenAndTheSecondsIn() async {
        let (p, s, e) = make()
        _ = await opened([], p, s)
        e.report = (1, 4, 7.5)
        p.next()
        let op = lastOp(s, .skip)
        XCTAssertEqual(op?.by, .listener); XCTAssertEqual(op?.ended?.outcome, .skipped); XCTAssertEqual(op?.ended?.lastBlock, 1); XCTAssertEqual(op?.positionSeconds, 7.5)
        // a skipped clip carries its position
        let (p2, s2, e2) = make()
        let c = clipEntry("a"); p2.playNow(c); await p2.settle(); e2.elapsed = 12
        p2.next()
        XCTAssertEqual(lastOp(s2, .skip)?.positionSeconds, 12); XCTAssertEqual(lastOp(s2, .skip)?.by, .listener)
    }
    func test24_PlayNowReportsWhereItCutTheEntryOnAir() async {
        let (p, s, e) = make()
        let o = await opened([], p, s)
        e.report = (2, 5, 19)
        p.playNow(.paperStory(route: "r", title: "R")); await p.settle()
        let op = lastOp(s, .place)
        XCTAssertEqual(op?.by, .listener); XCTAssertEqual(op?.ended?.outcome, .cut); XCTAssertEqual(op?.ended?.lastBlock, 2); XCTAssertEqual(op?.positionSeconds, 19)
        XCTAssertEqual(s.entry(o)?.ended?.outcome, .cut)
    }
    func test24_PreviousIsAPreviousOpByTheListener() async {
        let (p, s, e) = make()
        _ = await opened(["d-1"], p, s)
        p.next(); await p.settle()
        e.position = (0, 0.5)
        p.previous()
        let op = lastOp(s, .previous)
        XCTAssertEqual(op?.by, .listener); XCTAssertNil(s.pending.last { $0.op == .aired && $0.state == .heard && $0.ended?.outcome == .skipped })
    }
    func test24_RemovesAreByWhoDidThem() async {
        let (p, s, _) = make()
        _ = await opened([], p, s)
        // the listener removes a row
        let mine = RunSheetEntry.paperStory(route: "m", title: "Mine"); p.queue(mine)
        p.remove(at: 0)
        XCTAssertEqual(lastOp(s, .remove)?.by, .listener); XCTAssertNil(lastOp(s, .remove)?.reason)
        // an orphaned transition is dropped by the phone, with its reason
        var t = story("d-t", text: "And now.", kind: .transition); t.between = RunSheetBetween(after: "d-a", before: "d-b")
        deskPlace(s, story("d-a")); deskPlace(s, t); deskPlace(s, story("d-b"))
        p.next(); await p.settle(); p.next(); await p.settle()
        let orphan = s.pending.last { $0.op == .remove && $0.id == "d-t" }
        XCTAssertEqual(orphan?.by, .phone); XCTAssertEqual(orphan?.reason, "transition_orphaned")
        // a desk opening replacing the phone's unaired one
        let (p2, s2, _) = make()
        let phone = PhoneOpening.entry(lines: lines(1)); s2.place(phone, at: .end, by: .phone); p2.setOpeningForTest(phone.id)
        _ = p2.admitDesk(ProgrammeRouteFeed.placeOp(deskOpening(), cause: "open"), cause: "open")
        let rep = s2.pending.last { $0.op == .remove && $0.id == phone.id }
        XCTAssertEqual(rep?.by, .phone); XCTAssertEqual(rep?.reason, "opening_replaced_by_desk")
        // stale entries of an earlier listening are taken off by the phone
        let (p3, s3, _) = make()
        deskPlace(s3, story("d-old")); p3.open(PhoneOpening.entry(lines: lines(1)))
        let stale = s3.pending.last { $0.op == .remove && $0.id == "d-old" }
        XCTAssertEqual(stale?.by, .phone); XCTAssertNotNil(stale?.reason)
    }


    // 25 no op the phone builds can break a server bound: cut at the bound, never dropped there
    func test25_AnEntryBuiltTooBigIsCutAtTheBoundsAndApplies() {
        let (_, s, _) = make()
        let long = String(repeating: "word ", count: 600)
        var e = RunSheetEntry(id: "x", kind: .story, placedBy: .desk, title: long, text: String(repeating: "a", count: 20_000),
                              source: RunSheetSource(ref: String(repeating: "r", count: 900), label: long, url: String(repeating: "u", count: 900)),
                              cites: (0..<20).map { RunSheetCite(ref: "pub:\($0)", role: .mentioned, label: long) }, estSeconds: 1e9)
        e.forecast = RunSheetForecast(pHeard: 4, causedBy: String(repeating: "c", count: 400), claimId: "k")
        let r = s.place(e, at: .end, by: .desk, reason: String(repeating: "r", count: 500))
        XCTAssertTrue(r.applied, r.dropped ?? "")
        let got = s.entry("x")!
        XCTAssertLessThanOrEqual(got.title.utf8.count, 300); XCTAssertLessThanOrEqual(got.text!.utf8.count, 8192)
        XCTAssertEqual(got.source!.ref.utf8.count, 900, "a ref is never cut"); XCTAssertLessThanOrEqual(got.source!.label.utf8.count, 512); XCTAssertEqual(got.source!.url!.utf8.count, 900)
        XCTAssertEqual(got.cites!.count, 12); XCTAssertEqual(got.estSeconds, 86400); XCTAssertEqual(got.forecast!.pHeard, 1)
        XCTAssertLessThanOrEqual(got.forecast!.causedBy.utf8.count, 160)
        XCTAssertLessThanOrEqual(RunSheet.entryJSONSize(got), RunSheetBounds.maxEntryJSON)
        let multibyte = RunSheetEntry.paperStory(route: "r", title: String(repeating: "\u{1F600}", count: 200))
        XCTAssertTrue(s.place(multibyte, at: .end).applied); XCTAssertLessThanOrEqual(s.entry(multibyte.id)!.title.utf8.count, 300)
    }

    // 26 the server's 2048 bound on refs and urls, the carried size, what an op may carry
    func test26_ALongUrlIsLeftOutAndTheRefStaysAndALongRefIsNotPlaced() async {
        let (p, s, e) = make()
        let long = URL(string: "https://x.test/" + String(repeating: "a", count: 2500) + ".mp3")!
        let audio = RunSheetEntry.audio(RunSheetAudio(url: long, title: "Long", show: "S", episodeId: "ep-1"))
        XCTAssertTrue(s.place(audio, at: .end).applied)
        XCTAssertNil(s.entry(audio.id)?.source?.url, "over 2048 bytes the url is not sent")
        XCTAssertEqual(s.entry(audio.id)?.source?.ref, "episode:ep-1"); XCTAssertEqual(audio.audioSpec?.url, long, "the phone still has it to play")
        let page = RunSheetEntry.webPage(url: URL(string: "https://x.test/" + String(repeating: "b", count: 2200))!, title: "P")
        p.queue(page)
        XCTAssertNil(s.entry(page.id), "a ref cannot be cut: it is not placed"); XCTAssertEqual(p.message, "That address is too long to add.")
        var withCites = story("c1"); withCites.cites = [RunSheetCite(ref: "pub:" + String(repeating: "r", count: 2100), role: .mentioned, label: "L"), RunSheetCite(ref: "pub:ok", role: .subject, label: "OK")]
        XCTAssertTrue(s.place(withCites, at: .end, by: .phone).applied); XCTAssertEqual(s.entry("c1")?.cites?.map(\.ref), ["pub:ok"])
        _ = e
    }

    func test26_TheSizeBoundIsOfTheCarriedFormAndTheEntryIsCutToFit() {
        let (_, s, _) = make()
        XCTAssertGreaterThan(RunSheet.entryJSONSize(story("a", text: String(repeating: "<", count: 8192))), RunSheetBounds.maxEntryJSON)
        XCTAssertLessThan(RunSheet.entryJSONSize(story("b", text: String(repeating: "<", count: 4000))), RunSheetBounds.maxEntryJSON)
        let quotes = story("q", text: String(repeating: "\"", count: 8000))                  // 8000 quotes cost 32000 and the rest of the entry tips it over
        XCTAssertTrue(s.place(quotes, at: .end, by: .phone).applied, "the phone cuts the text until it fits rather than have the server drop it")
        XCTAssertLessThanOrEqual(RunSheet.entryJSONSize(s.entry("q")!), RunSheetBounds.maxEntryJSON)
    }

    func test26_AnOpCarriesNothingItDoesNotUse() {
        let sheet = RunSheet()
        let e = RunSheetEntry.paperStory(route: "r", title: "T")
        let a = sheet.applying(RunSheetOp(opId: "o1", by: .listener, op: .remove, id: "x", entry: e)).result
        XCTAssertEqual(a.dropped, "invalid", "an entry on any op but place")
        let b = sheet.applying(RunSheetOp(opId: "o2", by: .listener, op: .remove, id: "x", text: "t")).result
        XCTAssertEqual(b.dropped, "invalid", "a text on any op but rewrite")
    }

    // 27 answers
    final class EpochPoster: RunSheetPoster, @unchecked Sendable {
        var answer: RunSheetPostOutcome = .answered([])
        var respond: (([RunSheetOp]) -> RunSheetPostOutcome)?
        var fetched: RunSheet?
        private(set) var epochs: [Int64] = []
        func post(ops: [RunSheetOp], epoch: Int64) async -> RunSheetPostOutcome { epochs.append(epoch); return respond?(ops) ?? answer }
        func fetchSheet() async -> RunSheet? { fetched }
    }
    func test27_AnEntryTheServerDroppedIsNotKeptAndNothingIsShown() async {
        for why in ["replay_skipped", "record_too_large", "invalid", "ahead_full"] {
            let (p, s, _) = make()
            let poster = EpochPoster(); s.poster = poster
            poster.respond = { ops in .answered(ops.map { RunSheetResult(opId: $0.opId, applied: false, dropped: why, version: 0) }) }
            let mine = RunSheetEntry.paperStory(route: "r", title: "Mine")
            s.place(mine, at: .end); await s.flushTask?.value
            XCTAssertNil(s.entry(mine.id), why); XCTAssertTrue(s.pending.isEmpty); XCTAssertEqual(p.message, "", "nothing alarming is shown")
            // a duplicate is the server already having it
            poster.respond = { ops in .answered(ops.map { RunSheetResult(opId: $0.opId, applied: false, dropped: "duplicate", version: 3) }) }
            let kept = RunSheetEntry.paperStory(route: "k", title: "Kept")
            s.place(kept, at: .end); await s.flushTask?.value
            XCTAssertNotNil(s.entry(kept.id), "duplicate: the server has it")
        }
    }

    func test28_TheEpochIsRememberedSentAndAChangedOneTakesTheServersSheet() async {
        let (_, s, _) = make()
        let poster = EpochPoster(); s.poster = poster
        poster.answer = .answeredEpoch([], epoch: 7)
        var theirs = RunSheet(); theirs.sheetEpoch = 7; theirs.version = 40
        theirs.entries = [RunSheetEntry(id: "srv1", kind: .story, placedBy: .desk, title: "Theirs", text: "t")]
        poster.fetched = theirs
        let mine = RunSheetEntry.paperStory(route: "r", title: "Mine")
        s.place(mine, at: .end); await s.flushTask?.value
        XCTAssertEqual(s.sheetEpoch, 7); XCTAssertNotNil(s.entry("srv1"), "the server's sheet replaced the local one")
        // what the server has not answered is applied on top again; this one was answered, so it is not
        poster.answer = .answered([])
        s.place(.paperStory(route: "n", title: "Next"), at: .end); await s.flushTask?.value
        XCTAssertEqual(poster.epochs.last, 7, "the epoch it holds goes with every post")
        // the same epoch again changes nothing
        poster.answer = .answeredEpoch([], epoch: 7); poster.fetched = nil
        s.place(.paperStory(route: "m", title: "More"), at: .end); await s.flushTask?.value
        XCTAssertEqual(s.sheetEpoch, 7); XCTAssertNotNil(s.entry("srv1"))
    }

    func test29_TheStreamRulesAreTheServers() {
        XCTAssertEqual(RunSheetStreamPolicy.path(after: nil, epoch: 0), "/api/autoradio/sheet/stream")
        XCTAssertEqual(RunSheetStreamPolicy.path(after: 12, epoch: 7), "/api/autoradio/sheet/stream?after=12&epoch=7", "the version and the epoch it holds")
        XCTAssertEqual(RunSheetStreamPolicy.retry(status: 503, attempt: 1), .after(2)); XCTAssertEqual(RunSheetStreamPolicy.retry(status: 503, attempt: 9), .after(60))
        XCTAssertEqual(RunSheetStreamPolicy.retry(status: 429, attempt: 2), .tooManyStreams(60)); XCTAssertEqual(RunSheetStreamPolicy.retry(status: 404, attempt: 0), .giveUp)
        let (_, s, _) = make()
        let full = #"{"version":9,"sheet_epoch":5,"entries":[{"id":"e1","kind":"story","placed_by":"desk","state":"ahead","title":"T","text":"x","est_seconds":3}]}"#
        XCTAssertEqual(s.streamEvent(name: "sheet", data: Data(full.utf8)), .sheet)
        XCTAssertEqual(s.sheetEpoch, 5); XCTAssertNotNil(s.entry("e1"))
        XCTAssertEqual(s.streamEvent(name: "sheet_ops", data: Data(#"{"from_version":99,"to_version":100,"ops":[]}"#.utf8)), .resync(from: s.sheet.version))
        XCTAssertEqual(s.streamEvent(name: "ping", data: Data()), .ignored)
    }
}

// What the bar and the radio page say with nothing on air, and what the desk's notice and writing flags do to them.
@MainActor
final class RunSheetOverlayTests: XCTestCase {
    func testNothingOnAirIsNeverShownAsPlayingAndUpNextSaysWhatIsTrue() async {
        let s = RunSheetStore(); let p = RunSheetPlayer(store: s); p.engine = FakeEngine()
        // (the bar's own overlay is bar.ts: bar.test.ts B9; this is the radio view's)
        var r = RadioInfo(); r.title = "Autoradio"; r.playing = true
        let ro = p.overlay(r)
        XCTAssertFalse(ro.playing); XCTAssertFalse(ro.paused); XCTAssertEqual(ro.title, "")
        p.deskWriting = true
        XCTAssertTrue(p.info().writing)
        p.deskWriting = false; p.deskNotice = "The desk can't be reached right now."
        XCTAssertEqual(p.info().notice, "The desk can't be reached right now.")
        XCTAssertTrue(RadioPage.make(RadioPageInput(sheet: s.sheet)).ahead.isEmpty)
    }
}
