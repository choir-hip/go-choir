import XCTest
@testable import AutoradioKit

// Station P1, review round 2 (REPAIR): written before the repair. B1 resumeAt outlives its document, S1 the closed marker swallows a real
// replay, S2 sign-out leaves `loading` set, and S3 format-character stripping is too broad. They fail on d2bf58f61. The hand-over of a late
// programme that this round also added was removed in round 3; its tests went with it.

final class Repair2PlayerTests: XCTestCase {
    func makePlayer(voice: String = "long-voice", doc: ProgrammeDocument, log: EventLog = EventLog(clock: LabClock())) -> ProgrammePlayer {
        let r = StageRegistry.builtIn()
        r.register(.voice, LongVoice.self) { LongVoice($0) }
        r.register(.voice, StubVoice.self) { StubVoice($0) }
        var cfg = StageRegistry.defaultConfiguration(); cfg.set(.voice, .init(id: voice))
        return ProgrammePlayer(audio: virtualAudio(), registry: r, log: log, configuration: cfg, document: doc)
    }
    let article = ProgrammeDocument(id: "choir:a", title: "A", blocks: [.speech(text: "One."), .speech(text: "Two."), .speech(text: "Three.")])
    let eight = (1...8).map { "Paragraph \($0) of the message." }.joined(separator: "\n\n")

    func runDry(_ p: ProgrammePlayer) async {
        p.play()
        let ended = await poll(10) { !p.loopIsActive }
        XCTAssertTrue(ended, "the article ran dry")
    }

    // B1: resumeAt (3) must not outlive its document.
    func testB1_ATextReadAfterTheProgrammeRanDryStartsAtItsFirstParagraph() async {
        let p = makePlayer(voice: "stub-voice", doc: article)
        await runDry(p)
        XCTAssertTrue(p.speakText(address: "m", text: eight))
        XCTAssertEqual(p.document.id, "text:m"); XCTAssertEqual(p.blockIndex, 0, "paragraphs 0 to 2 are not skipped")
        p.pause()
    }
    func testB1_PlayFalseThenPlayLaterStartsAtTheFirstParagraph() async {
        let p = makePlayer(voice: "stub-voice", doc: article)
        await runDry(p)
        p.pause()
        XCTAssertTrue(p.speakText(address: "m", text: eight, startNow: true, play: false))
        p.play()
        XCTAssertEqual(p.blockIndex, 0)
        p.pause()
    }
    func testB1_ANewDocumentSetAfterDryStartsAtItsFirstBlock() async {
        let p = makePlayer(voice: "stub-voice", doc: article)
        await runDry(p)
        p.pause()
        p.setDocument(ProgrammeDocument(id: "choir:b", title: "B", blocks: (0..<8).map { .speech(text: "Block \($0).") }))
        p.play()
        XCTAssertEqual(p.blockIndex, 0)
        p.pause()
    }

    // S1 at the player: play at the end with nothing new, and previous, are plays: they come on air again.
    func testS1_PreviousAfterTheLastSegmentFinishedIsReported() async {
        let log = EventLog(clock: LabClock())
        let p = makePlayer(voice: "stub-voice", doc: segmentedDocument(["One."]), log: log)
        let w = PlayerWatch(log, p)
        p.play()
        let a = await poll(10) { w.ranDry.count == 1 }; XCTAssertTrue(a)
        let b = await poll(5) { !p.loopIsActive }; XCTAssertTrue(b)
        p.previous()
        let c = await poll(10) { w.ranDry.count == 2 }
        XCTAssertTrue(c, "the replay ran again and ended dry: \(w.air)")
        XCTAssertEqual(w.air, [0, 0], "the replay came on air a second time")
    }
}

final class Repair2StageTests: XCTestCase {
    var env: ChoirEnvironment!
    override func setUp() {
        env = ChoirEnvironment.shared
        env.keys = InMemoryKeyStore([KeyAccount.apiKey: secretKey, KeyAccount.computerID: "comp-1"])
        env.log = EventLog(clock: LabClock())
    }
    let article = { @Sendable () async throws -> ProgrammeDocument in ProgrammeDocument(id: "choir:a", title: "A", blocks: [.speech(text: "Article.")]) }
    func player(_ doc: ProgrammeDocument) -> ProgrammePlayer {
        let r = StageRegistry.builtIn(); var cfg = StageRegistry.defaultConfiguration(); cfg.set(.voice, .init(id: "stub-voice"))
        r.register(.voice, StubVoice.self) { StubVoice($0) }
        return ProgrammePlayer(audio: virtualAudio(), registry: r, log: EventLog(clock: LabClock()), configuration: cfg, document: doc)
    }

    // Round 3 (S2): sign-out empties what was queued behind the programme (a previous account's text must not play).
    func testSignOutClearsTheQueue() async {
        let cfg = URLSessionConfiguration.ephemeral
        cfg.protocolClasses = [StubProtocol.self]
        env.session = URLSession(configuration: cfg)
        StubProtocol.handler = { _ in .init(status: 404, json: ["error": "x"]) }
        defer { env.session = .shared }
        let dir = FileManager.default.temporaryDirectory.appendingPathComponent("p1-\(UUID().uuidString)")
        let rt = LabRuntime(supportDir: dir, session: NullSessionOwner())
        rt.registry.register(.voice, LongVoice.self) { LongVoice($0) }
        rt.setStage(.voice, id: "long-voice")
        let app = ChoirApp(runtime: rt, keys: InMemoryKeyStore([KeyAccount.apiKey: secretKey, KeyAccount.computerID: "comp-1"]))
        rt.player.setDocument(ProgrammeDocument(id: "choir:a", title: "A", blocks: [.speech(text: "One."), .speech(text: "Two.")]))
        rt.player.play()
        let started = await poll { rt.player.loopIsActive }; XCTAssertTrue(started)
        XCTAssertTrue(rt.player.speakText(address: "mail-1", text: "A message for the previous account."))
        XCTAssertEqual(rt.player.queuedTexts, ["text:mail-1"])
        await app.signOut()
        XCTAssertEqual(rt.player.queuedTexts, [], "nothing of the previous account is queued")
    }

    // Review 4 (2026-11-24): signed out the radio only listens: no key, no turn, and the microphone cannot be turned on.
    func testSignedOutTheMicrophoneCannotBeTurnedOnAndNoTurnIsSent() async {
        let cfg = URLSessionConfiguration.ephemeral
        cfg.protocolClasses = [StubProtocol.self]
        env.session = URLSession(configuration: cfg)
        StubProtocol.handler = { _ in .init(status: 404, json: ["error": "x"]) }
        defer { env.session = .shared; TurnGate.shared.keyPresent = nil; HandsFreeSwitch.shared.set(false) }
        let dir = FileManager.default.temporaryDirectory.appendingPathComponent("p2-\(UUID().uuidString)")
        let rt = LabRuntime(supportDir: dir, session: NullSessionOwner())
        let app = ChoirApp(runtime: rt, keys: InMemoryKeyStore([KeyAccount.apiKey: secretKey, KeyAccount.computerID: "comp-1"]))
        TurnGate.shared.keyPresent = { ChoirEnvironment.shared.keys.get(KeyAccount.apiKey) != nil }     // what the module wires at launch
        XCTAssertTrue(TurnGate.shared.allowed)
        rt.setHandsFree(true); XCTAssertTrue(HandsFreeSwitch.shared.isOn, "with a key the switch turns on")
        rt.setHandsFree(false)
        await app.signOut()
        XCTAssertFalse(TurnGate.shared.allowed, "no key, no turn")
        rt.setHandsFree(true); XCTAssertFalse(HandsFreeSwitch.shared.isOn, "and the microphone cannot be turned on")
    }

    // S3: only bidi controls, the zero-width space and the byte-order mark are removed; joiners are part of the text.
    func testS3_JoinersAreKept() {
        let family = "\u{1F468}\u{200D}\u{1F469}\u{200D}\u{1F467}"
        XCTAssertEqual(SegmentPrep.title("Family \(family) day", kind: "story"), "Family \(family) day", "a ZWJ emoji sequence stays whole")
        let persian = "می\u{200C}خواهم"
        XCTAssertEqual(SegmentPrep.title(persian, kind: "story"), persian, "a ZWNJ inside a word stays")
        let hindi = "क्\u{200D}ष"
        XCTAssertEqual(SegmentPrep.title(hindi, kind: "story"), hindi)
        for bad in ["\u{202A}", "\u{202B}", "\u{202C}", "\u{202D}", "\u{202E}", "\u{2066}", "\u{2067}", "\u{2068}", "\u{2069}", "\u{200E}", "\u{200F}", "\u{061C}", "\u{200B}", "\u{FEFF}"] {
            XCTAssertEqual(SegmentPrep.title("Ha\(bad)rbour", kind: "story"), "Harbour", "U+\(String(bad.unicodeScalars.first!.value, radix: 16)) is removed")
        }
    }
}
