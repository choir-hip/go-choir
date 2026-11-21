import XCTest
@testable import AutoradioKit

// Instant start: the opening is written and spoken before the launch that plays it. The failure list (each is a test below):
//  1  a cache younger than the bound and in the same part of the day plays, with its audio, through the opening entry
//  2  a cache older than the bound, or from another part of the day, is not played as new
//  3  a stale opening with a ready first entry starts that entry at once, and the live opening arrives as a transition
//  4  a changed voice drops everything, files included
//  5  a taken cache is gone from disk (played or stale files do not stay), and what is kept is bounded
//  6  one request per trigger; none while one is in flight (its own or the writer's); a background with a fresh opening asks nothing
//  7  the opening's speech is made block by block, the first entry's first block only; a voice that fails keeps the text
//  8  the cached audio reaches the output node through the programme player (the voice is not asked), the same path as made audio
//  9  the launch line carries every field once
// 10  the launch starts as soon as its condition holds (no fixed sleep), and gives up at the deadline; a warm voice does not fill the launch line

@MainActor
final class OpeningCacheTests: XCTestCase {
    var dir: URL!
    var now = Date(timeIntervalSince1970: 1_800_000_000)
    var cal: Calendar = { var c = Calendar(identifier: .gregorian); c.timeZone = TimeZone(identifier: "UTC")!; return c }()

    override func setUp() { dir = FileManager.default.temporaryDirectory.appendingPathComponent("oc-" + UUID().uuidString) }
    override func tearDown() { try? FileManager.default.removeItem(at: dir) }

    func cache(maxBytes: Int = OpeningCache.defaultMaxBytes) -> OpeningCache {
        OpeningCache(dir: dir, maxBytes: maxBytes, clock: { [unowned self] in self.now }, calendar: cal)
    }
    func at(_ h: Int, _ m: Int = 0) -> Date { cal.date(from: DateComponents(year: 2030, month: 3, day: 4, hour: h, minute: m))! }
    func opening(_ text: String = "This is Autoradio.\n\nThe Harbour Bridge reopens after its repairs.") -> RunSheetEntry {
        RunSheetEntry(id: "d-o1", kind: .opening, placedBy: .desk, title: "Opening", text: text, cites: [RunSheetCite(ref: "pub:harbour", role: .mentioned, label: "Harbour Bridge")], estSeconds: 8)
    }
    func story(_ id: String = "d-s1") -> RunSheetEntry {
        RunSheetEntry(id: id, kind: .story, placedBy: .desk, title: "Tide tables", text: "The tide tables change at the start of the season. Fishers are asked to check.", estSeconds: 9)
    }
    func pcm(_ text: String, n: Int = 1000) -> PreparedAudio { PreparedAudio(text: text, samples: [Float](repeating: 0.25, count: n), rate: 22050) }
    func audio(_ e: RunSheetEntry, n: Int = 1000) -> [PreparedAudio] { OpeningWriteAhead.blocks(of: e).map { pcm($0, n: n) } }

    // 11 the owner: the time is computed when the app opens, never taken from the cache
    func testTheTimeAndDayPartAreThoseOfTheMomentTheAppOpensNotOfTheWrite() {
        let c = cache()
        now = at(21, 20)
        let written = opening("Good evening. It is 9:20 and this is Autoradio.\n\nFrom the paper\u{2019}s front page: the Harbour Bridge reopens. Six stories are lined up for you tonight.")
        c.put(.opening, entry: written, titles: [], audio: [], voice: "v1")
        now = at(22, 7)
        guard case .opening(let o, _) = c.take(voice: "v1") else { return XCTFail("an opening of the same evening plays") }
        let spoken = o.text ?? ""
        XCTAssertTrue(spoken.contains("10:07 PM"), spoken); XCTAssertTrue(spoken.contains("Good evening"), spoken)
        XCTAssertFalse(spoken.contains("9:20")); XCTAssertFalse(spoken.contains("9:20 PM"))
    }
    func testAnOpeningHeardAfterMidnightSaysHelloAndTheTimeThen() {
        let c = cache(); now = at(23, 50)
        c.put(.opening, entry: opening(), titles: [], audio: [], voice: "v1")
        now = at(23, 59)
        XCTAssertTrue(Moment.clause(now: at(0, 53), calendar: cal).hasPrefix("Hello, it\u{2019}s 12:53 AM"))
        XCTAssertTrue(Moment.clause(now: at(8, 5), calendar: cal).hasPrefix("Good morning, it\u{2019}s 8:05 AM"))
    }
    func testNothingThatDependsOnTheMomentIsPreRendered() async {
        let c = cache(); now = at(21, 20)
        let feed = FakeFeed()
        let o = RunSheetEntry(id: "d-o9", kind: .opening, placedBy: .desk, title: "Opening", text: "Good evening. It is 9:20, about 45 minutes since the last one. This is Autoradio.\n\nThe harbour board met. Two stories are new today.", estSeconds: 9)
        feed.script = [.ops([RunSheetOp(opId: "o1", by: .desk, op: .place, at: .end, entry: o, reason: "x")]), .done]
        let store = RunSheetStore()
        var made: [String] = []
        let w = OpeningWriteAhead(cache: c, store: store, feed: { feed }, voice: { "v1" }, synth: { t in made.append(t); return PreparedAudio(text: t, samples: [0.1, 0.1], rate: 22050) }, clock: { [unowned self] in self.now })
        await w.refresh(reason: "aired")
        XCTAssertEqual(made, ["This is Autoradio.", "The harbour board met."], "only what is true whenever it is played is made ahead")
        for t in made { XCTAssertFalse(Moment.dependsOnMoment(t), t) }
    }
    func testTheCachedFirstBlockIsReadyWithNoSynthesisAndTheFreshClauseIsOneShortSentence() {
        let c = cache(); now = at(8)
        c.put(.opening, entry: opening(), titles: [], audio: audio(opening()), voice: "v1")
        now = at(9, 15)
        guard case .opening(let o, _) = c.take(voice: "v1") else { return XCTFail() }
        let blocks = OpeningWriteAhead.blocks(of: o)
        XCTAssertEqual(blocks[0], "This is Autoradio.", "the cached constant line starts the sound at once")
        XCTAssertNotNil(c.prepared(text: blocks[0]), "ready with no synthesis")
        XCTAssertEqual(blocks[1], Moment.clause(now: at(9, 15), calendar: cal)); XCTAssertNil(c.prepared(text: blocks[1]), "the clause is made while the first block plays")
        XCTAssertEqual(BarTracker.sentences(blocks[1]).count, 1); XCTAssertLessThan(blocks[1].count, 40)
        XCTAssertNotNil(c.prepared(text: blocks[2]), "the rest of the body is cached too")
    }
    func testAFirstBlockThatNamesTheMomentIsKeptAsTextNotSound() async {
        let c = cache(); now = at(8)
        let store = RunSheetStore()
        store.place(RunSheetEntry(id: "d-s2", kind: .story, placedBy: .desk, title: "T", text: "This morning the tide changed. More follows.", estSeconds: 9), at: .end, by: .desk, reason: "x")
        var made: [String] = []
        let w = OpeningWriteAhead(cache: c, store: store, feed: { nil }, voice: { "v1" }, synth: { t in made.append(t); return PreparedAudio(text: t, samples: [0.1], rate: 22050) }, clock: { [unowned self] in self.now })
        await w.refresh(reason: "aired")
        XCTAssertTrue(made.isEmpty); XCTAssertNotNil(c.first(voice: "v1")); XCTAssertEqual(c.first(voice: "v1")?.files.count, 0)
    }

    // 1
    func testAFreshOpeningPlaysWithItsAudio() {
        let c = cache(); now = at(8)
        c.put(.opening, entry: opening(), titles: ["Harbour Bridge"], audio: audio(opening()), voice: "v1")
        c.put(.first, entry: story(), titles: [], audio: [pcm(OpeningWriteAhead.blocks(of: story())[0])], voice: "v1")
        now = at(9, 30)
        guard case .opening(let o, let next) = c.take(voice: "v1") else { return XCTFail("a 90 minute old opening of the same morning plays") }
        XCTAssertEqual(o.id, "d-o1"); XCTAssertEqual(next?.id, "d-s1")
        let first = OpeningWriteAhead.blocks(of: opening())[0]
        XCTAssertEqual(c.prepared(text: first)?.samples.count, 1000)
        XCTAssertNil(c.prepared(text: first), "one use")
    }
    // 2
    func testAStaleOrOtherPartOfDayOpeningIsNotPlayed() {
        let c = cache(); now = at(8)
        c.put(.opening, entry: opening(), titles: [], audio: audio(opening()), voice: "v1")
        now = at(10, 30)
        XCTAssertEqual(c.peek(voice: "v1"), .none, "2.5 hours is past the bound")
        now = at(8); c.put(.opening, entry: opening(), titles: [], audio: [], voice: "v1")
        now = at(8)
        c.put(.opening, entry: opening(), titles: [], audio: [], voice: "v1")
        let late = at(11, 59); now = late
        c.put(.opening, entry: opening(), titles: [], audio: [], voice: "v1")
        now = at(12, 10)
        XCTAssertEqual(c.peek(voice: "v1"), .none, "written in the morning, heard in the afternoon")
    }
    // 3
    func testAStaleOpeningWithAReadyEntryStartsTheEntry() {
        let c = cache(); now = at(6)
        c.put(.opening, entry: opening(), titles: [], audio: audio(opening()), voice: "v1")
        now = at(7)
        c.put(.first, entry: story(), titles: [], audio: [pcm(OpeningWriteAhead.blocks(of: story())[0])], voice: "v1")
        now = at(10)
        guard case .entry(let e) = c.take(voice: "v1") else { return XCTFail("the entry starts at once") }
        XCTAssertEqual(e.id, "d-s1")
        let p = RunSheetPlayer(store: RunSheetStore())
        p.engine = FakeEngine()
        p.open(e, asOpening: false)
        XCTAssertEqual(p.store.onAir()?.id, "d-s1")
        let live = RunSheetOp(opId: "ox", by: .desk, op: .place, at: .end, entry: RunSheetEntry(id: "d-live", kind: .opening, placedBy: .desk, title: "Opening", text: "Good morning."), reason: "x")
        let admitted = p.admitDesk(live, cause: "open")
        XCTAssertEqual(admitted?.entry?.kind, .transition)
        XCTAssertEqual(admitted?.at, .next)
        now = at(20); c.put(.opening, entry: opening(), titles: [], audio: [], voice: "v1")
    }
    // 4 and 5
    func testAChangedVoiceInvalidatesAndATakenCacheLeavesNoFiles() {
        let c = cache(); now = at(8)
        c.put(.opening, entry: opening(), titles: [], audio: audio(opening()), voice: "v1")
        XCTAssertGreaterThan(c.bytesOnDisk, 0)
        XCTAssertEqual(c.take(voice: "v2"), .none)
        XCTAssertEqual(c.bytesOnDisk, 0); XCTAssertEqual(c.heldCount, 0)
        c.put(.opening, entry: opening(), titles: [], audio: audio(opening()), voice: "v1")
        _ = c.take(voice: "v1")
        XCTAssertEqual(c.bytesOnDisk, 0, "taken: deleted at once")
        XCTAssertEqual(c.take(voice: "v1"), .none)
    }
    func testWhatIsKeptIsBounded() {
        let c = cache(maxBytes: 3000); now = at(8)
        let big = audio(opening(), n: 5000)
        let kept = c.put(.opening, entry: opening(), titles: [], audio: big, voice: "v1")
        XCTAssertTrue(kept.files.isEmpty, "over the bound the audio is not kept")
        XCTAssertEqual(c.bytesOnDisk, 0); XCTAssertNotNil(c.opening(voice: "v1"), "the text is")
        for _ in 0..<5 { c.put(.opening, entry: opening(), titles: [], audio: audio(opening(), n: 400), voice: "v1") }
        XCTAssertLessThanOrEqual(c.bytesOnDisk, 3000)
    }

    // 6 and 7
    func writeAhead(_ c: OpeningCache, feed: FakeFeed, store: RunSheetStore? = nil, busy: Bool = false, synth: ((String) -> PreparedAudio?)? = nil) -> (OpeningWriteAhead, Box) {
        let box = Box()
        let w = OpeningWriteAhead(cache: c, store: store ?? RunSheetStore(), feed: { feed }, voice: { "v1" },
                                  synth: { t in box.made.append(t); return synth.map { $0(t) } ?? self.pcm(t) }, busy: { busy }, clock: { [unowned self] in self.now })
        return (w, box)
    }
    final class Box: @unchecked Sendable { var made: [String] = [] }
    func feed() -> FakeFeed {
        let f = FakeFeed()
        f.script = [.ops([RunSheetOp(opId: "oo", by: .desk, op: .place, at: .end, entry: opening(), reason: "x")]), .done]
        return f
    }
    func testOneRequestPerTriggerAndNoneWhileBusyOrFresh() async {
        let c = cache(); now = at(8)
        let f = feed()
        let (w, _) = writeAhead(c, feed: f)
        await w.refresh(reason: "opening aired")
        XCTAssertEqual(f.calls.count, 1); XCTAssertEqual(f.calls[0].cause, "open")
        now = at(8, 20)
        await w.refresh(reason: "background")
        XCTAssertEqual(f.calls.count, 1, "a fresh opening is kept: no request")
        now = at(9, 30)
        await w.refresh(reason: "background")
        XCTAssertEqual(f.calls.count, 2, "past half the bound it is written again, once")
        let f2 = feed()
        let (busy, _) = writeAhead(cache(), feed: f2, busy: true)
        await busy.refresh(reason: "background")
        XCTAssertEqual(f2.calls.count, 0, "the writer has a request in flight")
    }
    func testSpeechIsMadeForTheBlocksThePlayerSpeaksAndAFailedVoiceKeepsTheText() async {
        let c = cache(); now = at(8)
        let store = RunSheetStore()
        store.applyFromDesk(RunSheetOp(opId: "p1", by: .desk, op: .place, at: .end, entry: story(), reason: "x"))
        let (w, box) = writeAhead(c, feed: feed(), store: store)
        await w.refresh(reason: "opening aired")
        XCTAssertEqual(Array(box.made.prefix(OpeningWriteAhead.blocks(of: story()).count > 0 ? 1 : 0)), [OpeningWriteAhead.blocks(of: story())[0]], "the first entry: its first block only")
        XCTAssertEqual(box.made.count, 1 + OpeningWriteAhead.blocks(of: opening()).count)
        XCTAssertEqual(c.opening(voice: "v1")?.files.count, OpeningWriteAhead.blocks(of: opening()).count)
        XCTAssertEqual(c.first(voice: "v1")?.files.count, 1)
        let c2 = OpeningCache(dir: dir.appendingPathComponent("b"), clock: { [unowned self] in self.now }, calendar: cal)
        let (w2, _) = writeAhead(c2, feed: feed(), synth: { _ in nil })
        await w2.refresh(reason: "background")
        XCTAssertNotNil(c2.opening(voice: "v1")); XCTAssertEqual(c2.opening(voice: "v1")?.files.count, 0, "text kept, no sound")
    }

    // 8
    func testCachedAudioGoesThroughTheProgrammePlayer() async {
        let r = StageRegistry.builtIn()
        r.register(.voice, MuteVoice.self) { MuteVoice($0) }
        r.register(.voice, SystemStandIn.self) { SystemStandIn($0) }
        var cfg = StageRegistry.defaultConfiguration(); cfg.set(.voice, .init(id: "phonon"))
        let p = ProgrammePlayer(audio: virtualAudio(), registry: r, log: EventLog(clock: LabClock()), configuration: cfg,
                                document: ProgrammeDocument(id: "rs:x", title: "T", blocks: [.speech(text: "Good morning.")]))
        p.preparedSpeech = { t in t == "Good morning." ? PreparedAudio(text: t, samples: [Float](repeating: 0.1, count: 1234), rate: 22050) : nil }
        p.begin(at: 0)
        let ok = await poll { p.audio.programmeSamplesHanded == 1234 }
        XCTAssertTrue(ok, "the output node was handed the cached samples, not a made voice's")
    }

    // 9b: a start asked for later than the launch (a press of Play) says when, and how long the start itself took
    func testTheLaunchLineSaysWhenTheStartWasAskedFor() {
        var out: [String] = []
        let t = LaunchTimings(start: Date(timeIntervalSince1970: 100))
        var c = Date(timeIntervalSince1970: 100)
        t.clock = { c }; t.sink = { out.append($0) }
        c = Date(timeIntervalSince1970: 105.0); t.mark("play"); t.mark("engine"); t.mark("session"); t.mark("sheet")
        c = Date(timeIntervalSince1970: 105.8); t.printLine(source: "live")
        XCTAssertEqual(out, ["AUTORADIO launch engine_ms=5000 session_ms=5000 sheet_ms=5000 request_ms=-1 first_segment_ms=-1 voice_load_ms=-1 synth_ms=-1 first_audio_ms=5800 source=live play_ms=5000 since_play_ms=800"])
    }

    // 9
    func testTheLaunchLineCarriesEveryField() {
        var out: [String] = []
        let t = LaunchTimings(start: Date(timeIntervalSince1970: 100))
        var c = Date(timeIntervalSince1970: 100)
        t.clock = { c }; t.sink = { out.append($0) }
        c = Date(timeIntervalSince1970: 100.4); t.mark("engine")
        c = Date(timeIntervalSince1970: 100.5); t.mark("session")
        c = Date(timeIntervalSince1970: 100.6); t.mark("sheet")
        c = Date(timeIntervalSince1970: 100.9); t.printLine(source: "cached"); t.printLine(source: "cached")
        XCTAssertEqual(out, ["AUTORADIO launch engine_ms=400 session_ms=500 sheet_ms=600 request_ms=-1 first_segment_ms=-1 voice_load_ms=-1 synth_ms=-1 first_audio_ms=900 source=cached play_ms=-1 since_play_ms=-1"])
    }

    // 10
    func testTheStationStartsAsSoonAsTheConditionHoldsAndNeverPastTheDeadline() async {
        let st = Station(); var slept: [TimeInterval] = []; var started = 0
        st.startDJ = { started += 1 }
        st.sleep = { slept.append($0) }
        var polls = 0
        st.launchCondition = { polls += 1; return polls > 3 }
        _ = await st.start(reason: .launch)
        XCTAssertEqual(started, 1); XCTAssertEqual(slept.count, 3, "waited for the condition, three polls, not two seconds")
        XCTAssertLessThan(st.waited, 0.5)
        let never = Station(); var n = 0
        never.startDJ = { n += 1 }; never.sleep = { _ in }
        never.launchCondition = { false }
        _ = await never.start(reason: .launch)
        XCTAssertEqual(n, 1, "the deadline starts it anyway"); XCTAssertEqual(never.waited, 2, accuracy: 0.06)
        let ready = Station(); var m = 0; var s2 = 0
        ready.startDJ = { m += 1 }; ready.sleep = { _ in s2 += 1 }
        _ = await ready.start(reason: .launch)
        XCTAssertEqual(s2, 0, "nothing to wait for: no wait")
    }
    // 11 (the stories held on the phone play next when the desk cannot be reached) is a bun test now: content-run.test.ts S7, `localStories`.
}
