import XCTest
@testable import AutoradioKit

// The start gap (owner, first listening, 2026-11-18: 10 s to start, then 20 s of silence after the first story). The failure list:
//  1  a launch with the phone's own opening sends an extend and no open ask (the desk's opening would be dropped: one per start)
//  2  an ask whose only entry the player dropped does not block the next extend
//  3  a write-ahead refused while the writer is busy is written when the writer goes idle
//  4  a kept first entry plays on a start where the kept opening is too old (the evening before is still a start)
//  5  the launch timings are relative to the radio's open, not to the process
//  6  a landed stretch keeps a first entry when none is kept, and asks the desk for nothing
//  7  a start with nothing to play says why in one line

@MainActor
final class StartGapTests: XCTestCase {
    var dir: URL!
    var now = Date(timeIntervalSince1970: 1_800_000_000)
    var cal: Calendar = { var c = Calendar(identifier: .gregorian); c.timeZone = TimeZone(identifier: "UTC")!; return c }()
    override func setUp() { dir = FileManager.default.temporaryDirectory.appendingPathComponent("sg-" + UUID().uuidString) }
    override func tearDown() { try? FileManager.default.removeItem(at: dir) }
    func cache() -> OpeningCache { OpeningCache(dir: dir, clock: { [unowned self] in self.now }, calendar: cal) }
    func at(_ day: Int, _ h: Int) -> Date { cal.date(from: DateComponents(year: 2030, month: 3, day: day, hour: h))! }
    func story(_ id: String = "d-s1") -> RunSheetEntry {
        RunSheetEntry(id: id, kind: .story, placedBy: .desk, title: "Tide tables", text: "The tide tables change at the start of the season. Fishers are asked to check.", estSeconds: 9)
    }
    func opening() -> RunSheetEntry {
        RunSheetEntry(id: "d-o1", kind: .opening, placedBy: .desk, title: "Opening", text: "This is Autoradio.\n\nThe Harbour Bridge reopens after its repairs.", estSeconds: 8)
    }
    func pcm(_ t: String) -> PreparedAudio { PreparedAudio(text: t, samples: [Float](repeating: 0.2, count: 100), rate: 22050) }
    func rig() -> (RunSheetPlayer, RunSheetStore, RunSheetWriter, FakeFeed) {
        let s = RunSheetStore(); let p = RunSheetPlayer(store: s); p.engine = FakeEngine(); p.pageText = { _, t in (t, ["one", "two", "three"]) }
        let f = FakeFeed(); let w = RunSheetWriter(store: s) { f }
        return (p, s, w, f)
    }

    // 1
    func testALaunchWithThePhonesOwnOpeningAsksToExtendNotToOpen() async {
        let (p, s, w, f) = rig()
        f.script = [.ops([ProgrammeRouteFeed.placeOp(story("d-1"), cause: "extend")]), .done]
        ChoirApp.placeLaunch(plan: .none, lines: [.init(title: "A", dek: "a.")], player: p, store: s, writer: w)
        await w.settle()
        XCTAssertEqual(f.calls.map(\.cause), ["extend"])
        XCTAssertEqual(s.onAir()?.placedBy, .phone)
    }
    func testAKeptFirstEntryStartStillAsksForTheDesksOpening() async {
        let (p, s, w, f) = rig()
        ChoirApp.placeLaunch(plan: .entry(story("d-k")), lines: [], player: p, store: s, writer: w)
        await w.settle()
        XCTAssertEqual(f.calls.map(\.cause), ["open"], "the phone has no opening of its own here: the desk's follows as a transition")
    }

    // 2
    func testAnAskWhoseEntriesWereAllDroppedDoesNotBlockTheNextExtend() async {
        let (p, s, w, f) = rig()
        p.open(PhoneOpening.entry(lines: []))
        f.script = [.ops([ProgrammeRouteFeed.placeOp(story("d-1"), cause: "extend")]), .done]
        w.admit = { _, _ in nil }
        w.openAhead(); await w.settle()
        XCTAssertEqual(f.calls.count, 1)
        _ = s
        w.checkAhead(); await w.settle()
        XCTAssertEqual(f.calls.count, 2, "a dropped answer is not the desk having nothing more")
    }

    // 3
    func testAWriteAheadRefusedWhileBusyIsWrittenWhenTheWriterGoesIdle() async {
        let c = cache(); now = at(4, 8)
        let f = FakeFeed(); f.script = [.ops([RunSheetOp(opId: "o1", by: .desk, op: .place, at: .end, entry: opening(), reason: "x")]), .done]
        var busy = true
        let w = OpeningWriteAhead(cache: c, store: RunSheetStore(), feed: { f }, voice: { "v1" }, synth: { [unowned self] t in self.pcm(t) }, busy: { busy }, clock: { [unowned self] in self.now })
        await w.refresh(reason: "opening aired")
        XCTAssertTrue(f.calls.isEmpty); XCTAssertNil(c.opening(voice: "v1"))
        busy = false
        await w.writerIdle()
        XCTAssertEqual(f.calls.map(\.cause), ["open"]); XCTAssertNotNil(c.opening(voice: "v1"))
        await w.writerIdle()
        XCTAssertEqual(f.calls.count, 1, "once: the refusal is remembered until it runs")
    }

    // 4
    func testAKeptFirstEntryPlaysOnTheMorningAfterTheEvening() {
        let c = cache(); now = at(3, 20)
        c.put(.opening, entry: opening(), titles: [], audio: [], voice: "v1")
        c.put(.first, entry: story(), titles: [], audio: [pcm(OpeningWriteAhead.blocks(of: story())[0])], voice: "v1")
        now = at(4, 8)
        guard case .entry(let e) = c.take(voice: "v1") else { return XCTFail("12 hours old: the opening is stale, the story is not") }
        XCTAssertEqual(e.id, "d-s1")
        XCTAssertNotNil(c.prepared(text: OpeningWriteAhead.blocks(of: story())[0]), "with its sound")
    }
    func testAKeptEntryTwoDaysOldIsNotPlayed() {
        let c = cache(); now = at(2, 8)
        c.put(.first, entry: story(), titles: [], audio: [], voice: "v1")
        now = at(4, 8)
        XCTAssertEqual(c.peek(voice: "v1"), .none)
    }

    // 5
    func testTheLaunchTimingsAreRelativeToTheRadiosOpen() {
        var out: [String] = []
        var c = Date(timeIntervalSince1970: 100)
        let t = LaunchTimings(start: Date(timeIntervalSince1970: 100 - 4_742))      // the host process had been alive 79 minutes
        t.clock = { c }; t.sink = { out.append($0) }
        t.begin()
        c = Date(timeIntervalSince1970: 100.4); t.mark("engine")
        c = Date(timeIntervalSince1970: 100.9); t.printLine(source: "live")
        XCTAssertEqual(out, ["AUTORADIO launch engine_ms=400 session_ms=-1 sheet_ms=-1 request_ms=-1 first_segment_ms=-1 voice_load_ms=-1 synth_ms=-1 first_audio_ms=900 source=live play_ms=-1 since_play_ms=-1"])
    }

    // 6
    func testALandedStretchKeepsAFirstEntryWhenNoneIsKeptAndAsksNothing() async {
        let c = cache(); now = at(4, 8)
        let s = RunSheetStore(); s.place(story("d-1"), at: .end, by: .desk, reason: "x")
        let f = FakeFeed()
        let w = OpeningWriteAhead(cache: c, store: s, feed: { f }, voice: { "v1" }, synth: { [unowned self] t in self.pcm(t) }, clock: { [unowned self] in self.now })
        await w.writerIdle()
        XCTAssertEqual(c.first(voice: "v1")?.entry.id, "d-1"); XCTAssertEqual(c.first(voice: "v1")?.files.count, 1)
        XCTAssertTrue(f.calls.isEmpty, "a story already written needs no model call")
    }

    // 7
    func testAStartWithNothingToPlaySaysWhy() {
        var lines: [String] = []
        let c = cache(); c.log = { if $0.contains("refused") { lines.append($0) } }
        now = at(4, 8)
        _ = c.take(voice: "v1")
        c.put(.first, entry: story(), titles: [], audio: [], voice: "v2")
        _ = c.take(voice: "v1")
        now = at(4, 8)
        c.put(.opening, entry: opening(), titles: [], audio: [], voice: "v1")
        now = at(8, 8)
        _ = c.take(voice: "v1")
        XCTAssertEqual(lines.count, 3)
        XCTAssertTrue(lines[0].contains("no kept file"), lines[0]); XCTAssertTrue(lines[1].contains("voice differs"), lines[1])
        XCTAssertTrue(lines[2].contains("too old") && lines[2].contains("opening_age_s="), lines[2])
    }
}
