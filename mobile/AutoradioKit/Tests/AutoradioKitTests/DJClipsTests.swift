import XCTest
@testable import AutoradioKit

// Lane autoradio-dj-clips. Failure list (the numbers are in the test names):
// C1 a story with 0, 1 or 5 clips sends 0, 1 or at most 3; invalid clips and repeated ids are left out
// C2 one bulk clips request per stretch, cached after; a 404 or an empty list leaves the request as it was
// C3 a podcast_clip segment is lead-in then the clip block; one with no clip, or an id the phone does not have, is skipped and the programme goes on
// C4 lead-in, then the clip, then the next segment, in that order; Keep listening (the clip still parked) holds the programme
// C5 a clip that fails to start: the DJ says one line and goes on; one that is too slow is given up at the deadline
// C6 the reader's queued items play before the DJ's next segment, not before the first
// C7 Play on the third story hands over the list from the third onward
// C8 a slow stretch: the story plays plain at once and the stretch joins at the story's end; a stretch that fails plays the story plain

private func clipRow(_ id: String, route: String = "/a", start: Double = 10, end: Double = 70, show: String = "Show", ep: String = "Episode", quote: String = "A quote.") -> [String: Any] {
    ["clip_id": id, "story": ["route": route, "version": 2, "title": "Story"], "show": show, "episode_title": ep, "episode_guid": "g-\(id)",
     "feed_url": "https://example.test/feed.xml", "published": "2026-11-16T09:00:00Z", "audio_url": "https://example.test/\(id).mp3",
     "duration_seconds": 900, "start": start, "end": end, "quote": quote, "support": 0.9]
}
private func hex(_ n: Int) -> String { "clip-" + String(format: "%012x", n) }

final class ClipCandidatesTests: XCTestCase {
    func store(_ rows: [[String: Any]]) -> ClipStore { let s = ClipStore(); s.replace(rows.compactMap(PodcastClip.parse)); return s }
    func cand(_ route: String) -> StretchCandidate { StretchCandidate(route: route, version: 2, title: "T", dek: "D", text: nil) }

    func testC1_ZeroOneFiveClipsSendAtMostThree() {
        for (n, want) in [(0, 0), (1, 1), (5, 3)] {
            let s = store((0..<n).map { clipRow(hex($0 + 1)) })
            let body = StretchCandidates.body([cand("a")], texts: [:], clips: s.candidateClips(forRoutes: ["a"]))
            XCTAssertEqual((body[0]["clips"] as? [[String: Any]])?.count ?? 0, want, "\(n) clips")
        }
    }
    func testC1_OnlyWhatTheServerAcceptsIsSent() {
        let long = String(repeating: "é", count: 300)
        let s = store([clipRow(hex(1), start: 0, end: 3), clipRow(hex(2), start: 0, end: 400), clipRow("bad-id"), clipRow(hex(3), show: " ", ep: "x"),
                       clipRow(hex(4), show: long, ep: long, quote: String(repeating: "q", count: 900)), clipRow(hex(4)), clipRow(hex(5), route: "/b")])
        let c = s.candidateClips(forRoutes: ["a", "b"])
        XCTAssertEqual(c["a"]?.map(\.clipID), [hex(4)], "too short, too long, a bad id, no show, a repeated id are out")
        XCTAssertLessThanOrEqual(c["a"]![0].show.utf8.count, 200); XCTAssertLessThanOrEqual(c["a"]![0].quote.utf8.count, 400)
        XCTAssertEqual(c["b"]?.map(\.clipID), [hex(5)])
        XCTAssertEqual(Set(c.values.flatMap { $0 }.map(\.clipID)).count, c.values.flatMap { $0 }.count, "ids are unique in the request")
    }
    func testC1_OnlyTheFieldsTheServerNamesAreCopied() {
        let s = store([clipRow(hex(1))])
        let j = s.candidateClips(forRoutes: ["a"])["a"]![0].json
        XCTAssertEqual(Set(j.keys), ["clip_id", "show", "episode_title", "published", "seconds", "quote"])
        XCTAssertEqual(j["seconds"] as? Double, 60)
    }
    func testC9_HearItRowsAreTheStoryOwnClips() {
        let s = store([clipRow(hex(1)), clipRow(hex(2), route: "/b")])
        XCTAssertEqual(HearIt.rows(route: "a", store: s), [HearItRow(clipID: hex(1), show: "Show", episode: "Episode", length: "1 min", quote: "A quote.", story: "Story")])
        XCTAssertEqual(HearIt.rows(route: "/b", store: s).count, 1)
        XCTAssertTrue(HearIt.rows(route: "zzz", store: s).isEmpty)
    }
    func testC3_ABlockForTheClipFollowsTheLeadIn() throws {
        let seg = IncomingSegment(stretchID: "s", segmentID: "s/1", kind: "podcast_clip", title: "Show on it", text: "Here is how Show put it.", cites: [], blockCount: 1,
                                  clip: ProgrammeClipRef(clipID: hex(1), storyRef: "pub:a@v2", seconds: 60))
        let p = try XCTUnwrap(SegmentPrep.prepare(seg))
        XCTAssertEqual(p.blocks, [.speech(text: "Here is how Show put it."), .podcastClip(clipID: hex(1))])
        XCTAssertEqual(p.segment.clip?.clipID, hex(1)); XCTAssertEqual(p.segment.kind, "podcast_clip")
        var noClip = seg; noClip.clip = nil
        XCTAssertNil(SegmentPrep.prepare(noClip))
    }
    func testC3_TheDecoderReadsTheClipAndDropsALineWithNone() {
        var d = StretchDecoder()
        let ok = segmentLine(["kind": "podcast_clip", "title": "T", "text": "Lead.", "clip": ["clip_id": hex(1), "story_ref": "pub:a@v2", "seconds": 56.5]])
        let bad = segmentLine(["kind": "podcast_clip", "title": "T", "text": "Lead."])
        let evs = d.feed(ok + "\n" + bad + "\n")
        XCTAssertEqual(evs.count, 1)
        if case .segment(let s) = evs[0] { XCTAssertEqual(s.clip, ProgrammeClipRef(clipID: hex(1), storyRef: "pub:a@v2", seconds: 56.5)) } else { XCTFail() }
        XCTAssertEqual(d.droppedCount, 1)
    }
}

// MARK: The requests

final class ClipRequestTests: XCTestCase {
    var env: ChoirEnvironment!
    var clipsStatus = 200
    var clipRows: [[String: Any]] = []
    override func setUp() {
        env = ChoirEnvironment.shared
        let cfg = URLSessionConfiguration.ephemeral
        cfg.protocolClasses = [StubProtocol.self]
        env.keys = InMemoryKeyStore([KeyAccount.apiKey: secretKey, KeyAccount.computerID: "comp-1"])
        env.log = EventLog(clock: LabClock()); env.session = URLSession(configuration: cfg)
        StubProtocol.seen = []
        StubProtocol.handler = { [self] req in
            switch req.path {
            case "/api/platform/publications/front":
                return .init(json: ["stories": ["a", "b", "c"].enumerated().map { i, r in
                    ["title": "Story \(r)", "dek": "Dek \(r).", "route": ["bare": r], "section": "world", "words": 40, "importance": 9 - i] as [String: Any] }])
            case "/api/platform/publications/resolve": return .init(json: ["payload": ["text": "Text."]])
            case "/api/platform/podcast-clips": return .init(status: clipsStatus, json: ["clips": clipRows])
            default: return .init(status: 404, json: ["error": "nope"])
            }
        }
    }
    override func tearDown() { env.session = .shared }
    var clipCalls: Int { StubProtocol.seen.filter { $0.path == "/api/platform/podcast-clips" }.count }

    func testC2_OneBulkRequestForTheWholeListAndNoneAfterwards() async throws {
        clipRows = [clipRow(hex(1), route: "/a"), clipRow(hex(2), route: "/b"), clipRow(hex(3), route: "/b")]
        let store = ClipStore()
        let src = RouteStretchSource(baseURL: ChoirEnvironment.defaultBaseURL)
        let client = env.client(baseURL: ChoirEnvironment.defaultBaseURL)
        let c1 = try await src.candidates(client: client, skipping: [], clips: store)
        XCTAssertEqual(clipCalls, 1, "one request, not one per story")
        XCTAssertEqual(c1.map { ($0["clips"] as? [[String: Any]])?.count ?? 0 }, [1, 2, 0])
        _ = try await src.candidates(client: client, skipping: ["a"], clips: store)
        XCTAssertEqual(clipCalls, 1, "cached for the session")
        store.markStale()
        _ = try await src.candidates(client: client, skipping: [], clips: store)
        XCTAssertEqual(clipCalls, 2, "refreshed with the front")
    }
    func testC2_ARouteThatAnswers404LeavesTheRequestAsItWas() async throws {
        clipsStatus = 404
        let src = RouteStretchSource(baseURL: ChoirEnvironment.defaultBaseURL)
        let c = try await src.candidates(client: env.client(baseURL: ChoirEnvironment.defaultBaseURL), skipping: [], clips: ClipStore())
        XCTAssertEqual(c.count, 3); XCTAssertTrue(c.allSatisfy { $0["clips"] == nil })
        clipsStatus = 200; clipRows = []
        let c2 = try await src.candidates(client: env.client(baseURL: ChoirEnvironment.defaultBaseURL), skipping: [], clips: ClipStore())
        XCTAssertTrue(c2.allSatisfy { $0["clips"] == nil }, "an empty list too")
    }
    func testC7_TheListIsHandedOverFromTheStoryOnward() {
        let rows = ["a", "b", "c", "d"].map { ["title": "T\($0)", "dek": "D.", "route": ["bare": $0], "importance": $0 == "a" ? 9 : 1] as [String: Any] }
        let c = StretchCandidates.fromFront(["stories": rows], onlyRoutes: ["c", "d"])
        XCTAssertEqual(c.map(\.route), ["c", "d"], "its stories, its order")
    }
}

// MARK: The programme player and the clip

final class ClipPlaybackTests: XCTestCase {
    func player(_ doc: ProgrammeDocument) -> ProgrammePlayer {
        let r = StageRegistry.builtIn(); var cfg = StageRegistry.defaultConfiguration(); cfg.set(.voice, .init(id: "stub-voice"))
        r.register(.voice, StubVoice.self) { StubVoice($0) }
        return ProgrammePlayer(audio: virtualAudio(), registry: r, log: EventLog(clock: LabClock()), configuration: cfg, document: doc)
    }
    func doc() -> ProgrammeDocument {
        var d = ProgrammeDocument(id: "radio:t", title: "Autoradio", blocks: [])
        func add(_ i: Int, _ kind: String, _ text: String, clip: ProgrammeClipRef? = nil) {
            _ = d.append(SegmentPrep.prepare(IncomingSegment(stretchID: "t", segmentID: "t/\(i)", kind: kind, title: "S\(i)", text: text, cites: [], blockCount: nil, clip: clip))!)
        }
        add(0, "story", "First story."); add(1, "podcast_clip", "Lead in.", clip: ProgrammeClipRef(clipID: hex(1))); add(2, "story", "Third story.")
        return d
    }
    final class Trace: @unchecked Sendable { let l = Locked<[String]>([]); func add(_ s: String) { l.write { $0.append(s) } }; var all: [String] { l.read { $0 } } }

    func testC4_LeadInThenTheClipThenTheNextSegmentAndKeepListeningHoldsIt() async {
        let p = player(doc()); let t = Trace(); let gate = OneShot<ClipOutcome>()
        p.clipHandler = { id in t.add("clip:\(id)"); return await gate.wait() }
        p.boundaryHandler = { _ in t.add("boundary") }
        p.begin(at: 0)
        let parked = await poll { t.all.contains("clip:\(hex(1))") }; XCTAssertTrue(parked)
        XCTAssertEqual(p.blockIndex, 2, "the lead-in (block 1) was spoken first; the clip is block 2")
        try? await Task.sleep(nanoseconds: 400_000_000)
        XCTAssertEqual(p.blockIndex, 2, "the programme waits while the episode plays on")
        XCTAssertEqual(t.all.filter { $0 == "boundary" }.count, 1, "one boundary before the clip's segment; none before the first")
        gate.fire(.played)
        let after = await poll { p.blockIndex == 3 }; XCTAssertTrue(after, "then the next segment")
        XCTAssertEqual(t.all.filter { $0 == "boundary" }.count, 2, "and one before the third segment")
        p.pause()
    }
    func testC10_ASegmentThePreflightRefusesIsSkippedWholeAndNeverSpoken() async {
        let p = player(doc()); let t = Trace()
        p.segmentGate = { seg in t.add("gate:\(seg.id)"); return false }
        p.clipHandler = { _ in t.add("clip"); return .played }
        p.begin(at: 0)
        let done = await poll { p.blockIndex >= 3 }; XCTAssertTrue(done)
        XCTAssertFalse(t.all.contains("clip"), "the clip never started")
        XCTAssertEqual(t.all, ["gate:t/1"])
        p.pause()
    }
    func testC5_AClipThatFailsIsSaidOnceAndTheProgrammeGoesOn() async {
        let p = player(doc())
        p.clipHandler = { _ in .failed }
        p.begin(at: 0)
        let done = await poll { p.blockIndex == 3 }; XCTAssertTrue(done)
        p.pause()
    }
    func testC3_AnUnknownClipIsSkippedByTheStageAndTheProgrammeContinues() async throws {
        final class Manual: StretchSource, @unchecked Sendable {
            var cont: AsyncThrowingStream<StretchEvent, Error>.Continuation!
            func events(cause: String) async throws -> AsyncThrowingStream<StretchEvent, Error> { AsyncThrowingStream { self.cont = $0 } }
        }
        let src = Manual(); let stage = RadioProgramme(source: src, fallback: { throw ChoirError.empty("x") })
        stage.clips = ClipStore(); stage.clips.replace([PodcastClip.parse(clipRow(hex(1)))!])
        let opening = Task { try await stage.open() }
        _ = await poll { src.cont != nil }
        func seg(_ n: Int, _ kind: String, clip: String? = nil) -> StretchEvent {
            .segment(IncomingSegment(stretchID: "m", segmentID: "m/\(n)", kind: kind, title: "S\(n)", text: "Words \(n).", cites: [], blockCount: nil,
                                     clip: clip.map { ProgrammeClipRef(clipID: $0) }))
        }
        src.cont.yield(seg(0, "story"))
        let o = try await opening.value
        let p = player(o.document)
        src.cont.yield(seg(1, "podcast_clip", clip: hex(99))); src.cont.yield(seg(2, "podcast_clip", clip: hex(1))); src.cont.yield(seg(3, "story")); src.cont.finish()
        await o.follow?(p)
        XCTAssertEqual(p.document.segments.map(\.id), ["m/0", "m/2", "m/3"], "the unknown clip's segment (lead-in too) is out")
    }
}

// MARK: The runner and the queue

@MainActor
final class DJFakeEngine: PlaybackEngine {
    var log: [String] = []
    var startsRemote = true
    var duration: Double?
    func remoteDuration(url: URL, deadline: TimeInterval) async -> Double? { log.append("duration"); return duration }
    var remoteDelay: UInt64 = 0
    func playStory(route: String, title: String) async -> Bool { log.append("story:\(route)"); return true }
    func speak(address: String, title: String, paragraphs: [String]) async -> Bool { log.append("speak"); return true }
    func playRemote(url: URL, title: String, show: String, start: Double?, end: Double?) async -> Bool {
        log.append("remote:\(url.lastPathComponent):\(start ?? -1):\(end ?? -1)")
        if remoteDelay > 0 { try? await Task.sleep(nanoseconds: remoteDelay) }
        return startsRemote
    }
    func pauseRemote() { log.append("pause") }
    func resumeRemote() { log.append("resume") }
    func stopRemote() { log.append("stop") }
    func seekRemote(by s: Double) {}
    func seekRemote(to s: Double) {}
    func clearRemoteEnd() { log.append("clearEnd") }
    func remotePosition() -> (elapsed: Double, duration: Double?)? { (0, 600) }
    func setRemoteRate(_ r: Double) {}
    func setRemoteLevel(_ l: PlayLevel) {}
}

@MainActor
final class DJClipRunnerTests: XCTestCase {
    func make(_ rows: [[String: Any]] = [clipRow(hex(1))]) -> (DJClipRunner, PlayQueue, DJFakeEngine) {
        let q = PlayQueue(); let e = DJFakeEngine(); q.engine = e
        let s = ClipStore(); s.replace(rows.compactMap(PodcastClip.parse))
        let r = DJClipRunner(queue: q, store: s); r.install()
        return (r, q, e)
    }
    func audio(_ n: String) -> PlayItem { .audio(url: URL(string: "https://x.test/\(n).mp3")!, title: "T", show: "S", start: nil, end: nil, episodeId: nil) }

    func testC4_TheClipPlaysItsRangeFromThePublishersURLAndTheProgrammeWaitsForItsEnd() async {
        let (r, q, e) = make()
        let t = Task { await r.playClip(hex(1)) }
        let started = await poll { e.log.contains("remote:\(hex(1)).mp3:10.0:70.0") }; XCTAssertTrue(started)
        XCTAssertEqual(q.current?.isClip, true)
        XCTAssertTrue(r.isParked)
        await q.remoteEnded(generation: q.generation)                  // the clip reached its end
        let outcome = await t.value
        XCTAssertEqual(outcome, .played)
        XCTAssertFalse(r.isParked)
    }
    func testC4_KeepListeningLetsTheEpisodeRunOnAndTheProgrammeWaitsUntilItEnds() async {
        let (r, q, e) = make()
        let t = Task { await r.playClip(hex(1)) }
        _ = await poll { r.isParked && q.current != nil }
        q.keepListening()
        XCTAssertEqual(e.log.last, "clearEnd")
        try? await Task.sleep(nanoseconds: 300_000_000)
        XCTAssertTrue(r.isParked, "still the reader's episode")
        await q.remoteEnded(generation: q.generation)
        let o = await t.value; XCTAssertEqual(o, .played)
    }
    func testC4_WhatTheReaderQueuedPlaysBeforeTheProgrammeGoesOn() async {
        let (r, q, e) = make()
        q.enqueue(audio("mine"))
        let t = Task { await r.playClip(hex(1)) }
        _ = await poll { e.log.contains { $0.hasPrefix("remote:\(hex(1))") } }
        await q.remoteEnded(generation: q.generation)
        XCTAssertTrue(e.log.contains { $0.hasPrefix("remote:mine.mp3") }, "the queued item took its turn")
        XCTAssertTrue(r.isParked, "the programme still waits")
        await q.remoteEnded(generation: q.generation)
        let o = await t.value; XCTAssertEqual(o, .played)
    }
    func testC5_AClipThatCannotStartIsFailedAtOnce() async {
        let (r, _, e) = make(); e.startsRemote = false
        let o = await r.playClip(hex(1)); XCTAssertEqual(o, .failed)
        XCTAssertFalse(r.isParked)
    }
    func testC5_AClipThatIsTooSlowIsGivenUpAtTheDeadline() async {
        let (r, _, e) = make(); e.remoteDelay = 3_000_000_000; r.startDeadline = 0.3
        let t0 = Date()
        let o = await r.playClip(hex(1))
        XCTAssertEqual(o, .failed); XCTAssertLessThan(Date().timeIntervalSince(t0), 2)
        XCTAssertTrue(e.log.contains("stop"))
    }
    func testC10_ATranscribedFileOfTheSameLengthPlays() async {
        let (r, _, e) = make(); e.duration = 900                      // the row says 900 s
        let ok = await r.preflight(hex(1)); XCTAssertTrue(ok)
        e.duration = 902.5; let ok2 = await r.preflight(hex(1)); XCTAssertTrue(ok2, "within 3 s")
        XCTAssertFalse(e.log.contains { $0.hasPrefix("remote:") }, "nothing was played to find out")
    }
    func testC10_AnotherFileSkipsTheSegmentAndMarksTheRow() async {
        let (r, q, e) = make(); e.duration = 910
        let ok = await r.preflight(hex(1)); XCTAssertFalse(ok)
        XCTAssertTrue(r.store.isMismatch(hex(1)))
        XCTAssertTrue(HearIt.rows(route: "a", store: r.store)[0].episodeOnly)
        await HearIt.play(hex(1), store: r.store, queue: q, runner: r); await q.settle()
        XCTAssertEqual(e.log.last, "remote:\(hex(1)).mp3:-1.0:-1.0", "the episode from its start, no clip times")
    }
    func testC10_AnUnknownLengthWithinTheDeadlineSkips() async {
        let (r, _, e) = make(); e.duration = nil
        let ok = await r.preflight(hex(1)); XCTAssertFalse(ok)
    }
    func testC3_AnIdThePhoneDoesNotHaveIsUnknown() async {
        let (r, _, e) = make()
        let o = await r.playClip(hex(77)); XCTAssertEqual(o, .unknown); XCTAssertTrue(e.log.isEmpty)
    }
    func testC4_ASkipEndsTheClipAndTheProgrammeGoesOn() async {
        let (r, q, _) = make()
        let t = Task { await r.playClip(hex(1)) }
        _ = await poll { r.isParked && q.current != nil }
        r.skipCurrent()
        let o = await t.value; XCTAssertEqual(o, .played); XCTAssertNil(q.current)
    }
    func testC6_QueuedItemsTakeTheirTurnAtASegmentBoundaryAndNothingQueuedIsNoWait() async {
        let (r, q, e) = make()
        await r.yieldToQueue()                                          // nothing queued: returns at once
        XCTAssertTrue(e.log.isEmpty)
        q.enqueue(audio("mine"))
        let t = Task { await r.yieldToQueue() }
        _ = await poll { e.log.contains { $0.hasPrefix("remote:mine") } }
        XCTAssertTrue(r.isParked)
        await q.remoteEnded(generation: q.generation)
        await t.value
        XCTAssertFalse(r.isParked)
    }
    func testC8_TheEndOfWhatTheReaderPlayedContinuesIntoTheDJ() async {
        let (r, q, _) = make()
        var started = 0
        r.whenIdle = { started += 1; return true }
        q.enqueue(audio("x"))
        q.playNow(audio("a")); await q.settle()
        await q.remoteEnded(generation: q.generation)                   // plays x
        await q.remoteEnded(generation: q.generation)                   // nothing left: the DJ
        XCTAssertEqual(started, 1)
    }
}

// MARK: Play, in the app

final class DJPlayAppTests: XCTestCase {
    var env: ChoirEnvironment!
    override func setUp() {
        env = ChoirEnvironment.shared
        let cfg = URLSessionConfiguration.ephemeral
        cfg.protocolClasses = [StubProtocol.self]
        env.keys = InMemoryKeyStore([KeyAccount.apiKey: secretKey, KeyAccount.computerID: "comp-1"])
        env.log = EventLog(clock: LabClock()); env.session = URLSession(configuration: cfg)
        StubProtocol.seen = []
        StubProtocol.handler = { req in
            switch req.path {
            case "/api/platform/publications/front":
                return .init(json: ["stories": ["a", "b", "c", "d"].enumerated().map { i, r in
                    ["title": "Story \(r)", "dek": "Dek \(r).", "route": ["bare": r], "section": "world", "words": 40, "importance": 9 - i] as [String: Any] }])
            case "/api/platform/publications/resolve":
                let r = req.query["route"] ?? ""
                return .init(json: ["publication": ["title": "Story \(r)"], "version": ["id": "pv-\(r)", "manifest_hash": "mh"], "payload": ["text": "Text of \(r)."]])
            default: return .init(status: 404, json: ["error": "nope"])
            }
        }
    }
    override func tearDown() { env.session = .shared }

    final class Recording: StretchSource, @unchecked Sendable {
        let scopes = Locked<[[String]?]>([])
        var cont: AsyncThrowingStream<StretchEvent, Error>.Continuation?
        var hold = false
        func events(cause: String) async throws -> AsyncThrowingStream<StretchEvent, Error> { try await events(cause: cause, excludingRoutes: [], onlyRoutes: nil) }
        func events(cause: String, excludingRoutes: Set<String>, onlyRoutes: [String]?) async throws -> AsyncThrowingStream<StretchEvent, Error> {
            scopes.write { $0.append(onlyRoutes) }
            if !hold { throw ChoirError.empty("no stretch") }
            return AsyncThrowingStream { self.cont = $0 }
        }
    }

    func makeApp(_ source: Recording, voice: String = "long-voice") async -> (ChoirApp, LabRuntime) {
        let dir = FileManager.default.temporaryDirectory.appendingPathComponent("dj-\(UUID().uuidString)")
        let rt = LabRuntime(supportDir: dir, session: NullSessionOwner())
        rt.registry.register(.voice, StubVoice.self) { StubVoice($0) }
        rt.registry.register(.voice, LongVoice.self) { LongVoice($0) }
        rt.setStage(.voice, id: voice)
        await rt.start(virtual: true)
        rt.player.pause()
        let app = ChoirApp(runtime: rt, keys: InMemoryKeyStore([KeyAccount.apiKey: secretKey, KeyAccount.computerID: "comp-1"]), openURL: nil)
        app.stretchSource = { source }
        await app.loadStories()
        return (app, rt)
    }

    func testC7_PlayOnTheThirdStoryAsksForTheListFromTheThirdOnward() async {
        let src = Recording()
        let (app, rt) = await makeApp(src)
        app.storyOrder = { ["a", "b", "c", "d"].map { StoryRef(route: $0, title: "T") } }
        await app.play(route: "c")
        let asked = await poll { !src.scopes.read { $0 }.isEmpty }; XCTAssertTrue(asked)
        XCTAssertEqual(src.scopes.read { $0 }.first ?? nil, ["c", "d"])
        let plain = await poll { rt.player.document.id == "choir:pv-c" }
        XCTAssertTrue(plain, "no stretch: the story plays plain, as before")
        rt.player.pause()
    }
    func testC8_ASlowStretchLetsTheStoryPlayAtOnceAndJoinsAtItsEnd() async {
        let src = Recording(); src.hold = true
        let (app, rt) = await makeApp(src)
        app.djFirstSegmentWait = 0.3
        await app.play(route: "a")
        let plain = await poll { rt.player.document.id == "choir:pv-a" }
        XCTAssertTrue(plain, "plain read at once")
        let ready = await poll { src.cont != nil }; XCTAssertTrue(ready)
        src.cont!.yield(.segment(IncomingSegment(stretchID: "m", segmentID: "m/0", kind: "story", title: "S", text: "Words.", cites: [], blockCount: nil)))
        let joined = await poll(15) { rt.player.document.id == "radio:m" }
        XCTAssertTrue(joined, "the stretch took over at the story's end")
        rt.player.pause()
    }
    func testC8_AStretchThatFailsPlaysTheStoryPlain() async {
        let src = Recording()
        let (app, rt) = await makeApp(src)
        await app.play(route: "b")
        let plain = await poll { rt.player.document.id == "choir:pv-b" }
        XCTAssertTrue(plain)
        XCTAssertFalse(rt.player.userPaused)
        rt.player.pause()
    }
}
