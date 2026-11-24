import XCTest
@testable import AutoradioKit

// DOOR A: every model fetch goes through DownloadDoor. One test per FORMER PATH proves the escape is closed: with no consent, each path makes zero requests; a Wi-Fi agreement cannot spend mobile data in the transport;
// sign-out closes the door. The fetcher is a counting URL protocol on the transport the door hands out.
/// A body that ignores cancellation WITHOUT spinning: it waits on a continuation (which cancellation cannot interrupt) until the test lets it go. A `while true { try? await sleep }` would spin hot forever once cancelled (the sleep throws at once and `try?` swallows it) and starve the rest of the suite.
final class Hang: @unchecked Sendable {
    private let l = NSLock(); private var cont: CheckedContinuation<Void, Never>?; private var released = false
    func wait() async { await withCheckedContinuation { (c: CheckedContinuation<Void, Never>) in l.lock(); if released { l.unlock(); c.resume() } else { cont = c; l.unlock() } } }
    func release() { l.lock(); released = true; let c = cont; cont = nil; l.unlock(); c?.resume() }
}

final class CountingProtocol: URLProtocol, @unchecked Sendable {
    nonisolated(unsafe) static var count = 0
    static let lock = NSLock()
    override class func canInit(with request: URLRequest) -> Bool { true }
    override class func canonicalRequest(for request: URLRequest) -> URLRequest { request }
    override func startLoading() {
        Self.lock.lock(); Self.count += 1; Self.lock.unlock()
        client?.urlProtocol(self, didFailWithError: URLError(.cannotConnectToHost))
    }
    override func stopLoading() {}
    static func reset() { lock.lock(); count = 0; lock.unlock() }
    static var requests: Int { lock.lock(); defer { lock.unlock() }; return count }
}

final class DownloadDoorTests: XCTestCase {
    func counting() -> URLSession { let c = URLSessionConfiguration.ephemeral; c.protocolClasses = [CountingProtocol.self]; return URLSession(configuration: c) }
    func tmp() -> URL { FileManager.default.temporaryDirectory.appendingPathComponent("door-\(UUID().uuidString)", isDirectory: true) }
    func door(wifi: Bool = true) -> DownloadDoor { DownloadDoor(onWiFi: { wifi }) }
    let responder = "lfm3-1.7b-q5_k_m"

    // A1: the store (setup's pinned model, the search model, a retry and a resume are all this call).
    func testA1TheStoreFetchesNothingWithoutConsent() async {
        CountingProtocol.reset()
        let d = door(); let store = ModelStore(root: tmp(), registerKnown: true, session: counting(), door: d)
        do { _ = try await store.ensure(responder); XCTFail("fetched") } catch { XCTAssertEqual(error as? DownloadDoor.Refusal, .noConsent(group: "voice-rest")) }
        do { _ = try await store.ensure(KnownModels.embeddingGemma2.id); XCTFail("fetched") } catch { XCTAssertEqual(error as? DownloadDoor.Refusal, .noConsent(group: "search")) }
        XCTAssertEqual(CountingProtocol.requests, 0)
    }
    func testA1AnAgreementForAnotherGroupIsNotConsent() async {
        CountingProtocol.reset()
        let d = door(); d.sync(agreed: ["voice": "agreed-mobile"])                    // the voice press never covers the answering model or the search model
        let store = ModelStore(root: tmp(), registerKnown: true, session: counting(), door: d)
        do { _ = try await store.ensure(responder); XCTFail("fetched") } catch {}
        do { _ = try await store.ensure(KnownModels.embeddingGemma2.id); XCTFail("fetched") } catch {}
        XCTAssertEqual(CountingProtocol.requests, 0)
    }
    func testA1TheBundleFetchesNothingWithoutConsent() async {
        CountingProtocol.reset()
        let store = ModelStore(root: tmp(), registerKnown: true, session: counting(), door: door())
        do { _ = try await store.ensureBundle(KnownModels.fluidSpeechMini); XCTFail("fetched") } catch { XCTAssertEqual(error as? DownloadDoor.Refusal, .noConsent(group: "voice-rest")) }
        XCTAssertEqual(CountingProtocol.requests, 0)
    }
    func testA1TheVoicePrintFetchesNothingWithoutConsent() async {
        let root = tmp(); let e = WeSpeakerEmbedder(root: root, door: door())
        let ok = await e.download(); XCTAssertFalse(ok); XCTAssertFalse(e.onDisk)
        XCTAssertFalse(FileManager.default.fileExists(atPath: root.appendingPathComponent(WeSpeakerEmbedder.folder).path))
    }
    func testA1TheSetupBackendAndItsRetryGoThroughTheDoor() async {
        CountingProtocol.reset()
        let d = DownloadDoor.shared; d.sync(agreed: [:]); defer { d.sync(agreed: [:]) }
        let store = ModelStore(root: tmp(), registerKnown: true, session: counting())        // the shared door: nothing agreed
        let b = LiveSetupBackend(store: store)
        let cand = SetupCandidate(id: responder, label: "x", bytes: 1, minMemoryGB: 0)
        for _ in 0..<2 { do { try await b.download(cand) { _ in }; XCTFail("fetched") } catch {} }          // the second is a retry
        XCTAssertEqual(CountingProtocol.requests, 0)
    }
    func testA1TheLibraryPathsAreRefusedWithoutConsent() async {
        let d = door()
        for id in ["parakeet-tdt-0.6b-v2", "parakeet-eou-120m", "fluid-speech-mini", VoiceCatalog.kokoroModel, VoiceCatalog.pocketModel, "lfm3-audio-2b"] {
            do { try await d.libraryWindow(model: id) { } as Void; XCTFail("\(id) opened a window with no consent") } catch {}
        }
    }
    // A2: the transport itself, not a check before starting.
    func testA2AWifiAgreementHandsOutATransportThatCannotUseMobileData() throws {
        let d = door(); d.sync(agreed: ["voice-rest": "agreed", "search": "agreed-mobile"])
        let w = try d.session(for: responder).configuration
        XCTAssertFalse(w.allowsCellularAccess); XCTAssertFalse(w.allowsExpensiveNetworkAccess)
        XCTAssertTrue(w.allowsConstrainedNetworkAccess, "a Wi-Fi network in Low Data Mode is still the Wi-Fi the person agreed to: cellular is what the first two switches cut")
        let m = try d.session(for: KnownModels.embeddingGemma2.id).configuration
        XCTAssertTrue(m.allowsCellularAccess); XCTAssertTrue(m.allowsExpensiveNetworkAccess)
    }
    func testA2ARetryAfterTheNetworkChangedGetsTheSameWifiOnlyTransport() throws {
        let d = door(wifi: false); d.sync(agreed: ["voice-rest": "agreed"])
        // a fresh session for every call: a resume after the phone left Wi-Fi is still Wi-Fi only
        for _ in 0..<2 { XCTAssertFalse(try d.session(for: responder).configuration.allowsCellularAccess) }
    }
    // A3
    func testA3AModelNobodyOffersIsRefused() {
        let d = door(); d.sync(agreed: ["voice": "agreed-mobile", "voice-rest": "agreed-mobile", "search": "agreed-mobile"])
        XCTAssertThrowsError(try d.authorize(model: "a-model-in-no-list"))
        XCTAssertEqual(d.group(ofModel: ModelOffer.speakerID), "voice-rest"); XCTAssertEqual(d.group(ofModel: KnownModels.embeddingGemma2.id), "search")
    }
    func testA3NoVoiceCardIsOfferedForAModelNoGroupContains() {
        let d = door()
        let offered = VoiceCatalog.offered(by: d)
        XCTAssertFalse(offered.isEmpty); XCTAssertFalse(offered.contains { $0.modelID == VoiceCatalog.phononModel }, "the Phonon cards are not offered while no group contains Phonon")
        for o in offered where !o.modelID.isEmpty { XCTAssertNotNil(d.group(ofModel: o.modelID), "\(o.id) is offered with no group to agree to") }
        XCTAssertTrue(VoiceCatalog.options.contains { $0.modelID == VoiceCatalog.phononModel }, "(it is in the catalog)")
    }
    func testA3APathRefusalSaysItNeedsWifiNotAgreeInSettings() {
        XCTAssertTrue(DownloadDoor.Refusal.pathNotAgreed(group: "voice").person.contains("Wi-Fi"))
        XCTAssertTrue(DownloadDoor.Refusal.noConsent(group: "voice").person.contains("Settings"))
        XCTAssertTrue(DownloadDoor.Refusal.notOffered(model: "x").person.contains("not offered"))
    }
    func testA3TheNativeResetClosesTheDoor() throws {
        let d = door(); d.sync(agreed: ["voice-rest": "agreed"]); XCTAssertNoThrow(try d.authorize(model: responder))
        d.reset(); XCTAssertThrowsError(try d.authorize(model: responder))
    }
    // A4 (round 4): NO LIBRARY IS EVER INITIALISED IN A STATE WHERE IT CAN FETCH. The libraries' registry is closed from the start; it is open only inside a door window that has consent and a path the agreement covers.
    final class Registry: @unchecked Sendable {
        let lock = NSLock(); var log: [String] = []; var current = ""
        func set(_ v: String) { lock.lock(); log.append(v); current = v; lock.unlock() }
        var now: String { lock.lock(); defer { lock.unlock() }; return current }
    }
    func testA4TheRegistryIsClosedAtInstallAndOpenOnlyInsideAWindow() async throws {
        let r = Registry(); let d = door(); d.sync(agreed: ["voice-rest": "agreed"])
        d.installLibraryGuard { r.set($0) }
        XCTAssertEqual(r.now, DownloadDoor.closedRegistry, "closed before anything runs")
        let inside: String = try await d.libraryWindow(model: "fluid-speech-mini") { r.now }
        XCTAssertEqual(inside, DownloadDoor.openRegistry); XCTAssertEqual(r.now, DownloadDoor.closedRegistry, "closed again after")
        struct Boom: Error {}
        do { try await d.libraryWindow(model: "fluid-speech-mini") { throw Boom() } as Void } catch {}
        XCTAssertEqual(r.now, DownloadDoor.closedRegistry, "closed after a throw too")
    }
    func testA4NoConsentOpensNothingAndRunsNothing() async {
        let r = Registry(); let d = door(); d.installLibraryGuard { r.set($0) }
        final class Ran: @unchecked Sendable { var v = false }; let ran = Ran()
        do { try await d.libraryWindow(model: "fluid-speech-mini") { ran.v = true } as Void; XCTFail("opened") } catch { XCTAssertEqual(error as? DownloadDoor.Refusal, .noConsent(group: "voice-rest")) }
        XCTAssertFalse(ran.v); XCTAssertEqual(r.log, [DownloadDoor.closedRegistry], "the registry was never opened")
        do { try await d.libraryWindow(model: "lfm3-audio-2b") { ran.v = true } as Void; XCTFail("opened") } catch { XCTAssertEqual(error as? DownloadDoor.Refusal, .notOffered(model: "lfm3-audio-2b"), "the 2.9 GB audio model is in no group: it cannot be fetched") }
    }
    func testA4AWifiAgreementDoesNotOpenAWindowOnMobileData() async {
        let r = Registry(); let d = door(wifi: false); d.sync(agreed: ["voice-rest": "agreed"]); d.installLibraryGuard { r.set($0) }
        do { try await d.libraryWindow(model: "fluid-speech-mini") { } as Void; XCTFail("opened") } catch { XCTAssertEqual(error as? DownloadDoor.Refusal, .pathNotAgreed(group: "voice-rest")) }
        XCTAssertEqual(r.log, [DownloadDoor.closedRegistry])
        d.sync(agreed: ["voice-rest": "agreed-mobile"]); XCTAssertNoThrow(try awaitSync { try await d.libraryWindow(model: "fluid-speech-mini") { } as Void })
    }
    func awaitSync(_ f: @escaping @Sendable () async throws -> Void) throws { let sem = DispatchSemaphore(value: 0); final class E: @unchecked Sendable { var e: Error? }; let e = E(); Task { do { try await f() } catch { e.e = error }; sem.signal() }; sem.wait(); if let x = e.e { throw x } }
    // A transfer that is already running on a library's own session is cut when the phone leaves Wi-Fi under a Wi-Fi agreement.
    func testA4AWifiToCellularChangeMidTransferCutsTheLibraryTransfer() async throws {
        let r = Registry(); let d = door(wifi: true); d.sync(agreed: ["voice-rest": "agreed"]); d.installLibraryGuard { r.set($0) }
        final class Flag: @unchecked Sendable { let lock = NSLock(); var started = false, cut = false }; let f = Flag()
        let t = Task { try await d.libraryWindow(model: "fluid-speech-mini") {
            f.lock.lock(); f.started = true; f.lock.unlock()
            for _ in 0..<500 { if Task.isCancelled { f.lock.lock(); f.cut = true; f.lock.unlock(); throw CancellationError() }; try? await Task.sleep(nanoseconds: 10_000_000) }
        } as Void }
        for _ in 0..<100 { f.lock.lock(); let s = f.started; f.lock.unlock(); if s { break }; try? await Task.sleep(nanoseconds: 5_000_000) }
        d.pathChanged(onWiFi: false)
        do { try await t.value; XCTFail("not cut") } catch {}
        for _ in 0..<100 { f.lock.lock(); let c = f.cut; f.lock.unlock(); if c { break }; try? await Task.sleep(nanoseconds: 10_000_000) }          // the call returned at once; the body sees its cancellation a moment later
        f.lock.lock(); XCTAssertTrue(f.cut, "the transfer was cancelled when the path left Wi-Fi"); f.lock.unlock()
        XCTAssertEqual(r.now, DownloadDoor.closedRegistry)
    }
    func testA4AMobileAgreementIsNotCutByAPathChange() async throws {
        let d = door(wifi: true); d.sync(agreed: ["voice-rest": "agreed-mobile"])
        final class Flag: @unchecked Sendable { var cut = false }; let f = Flag()
        let t = Task { try await d.libraryWindow(model: "fluid-speech-mini") { try? await Task.sleep(nanoseconds: 80_000_000); f.cut = Task.isCancelled } as Void }
        try? await Task.sleep(nanoseconds: 20_000_000); d.pathChanged(onWiFi: false); try await t.value
        XCTAssertFalse(f.cut)
    }
    // A5
    func testA5SignOutClosesTheDoor() throws {
        let d = door(); d.sync(agreed: ["voice-rest": "agreed"]); XCTAssertNoThrow(try d.authorize(model: responder))
        d.sync(agreed: [:]); XCTAssertThrowsError(try d.authorize(model: responder))
        d.sync(agreed: ["voice-rest": "maybe"]); XCTAssertThrowsError(try d.authorize(model: responder), "an unknown word is no agreement")
    }

    // ---- Round 5: the window's edges. Consent and path are checked INSIDE the lock that opens the registry; a path change, a reset or a withdrawn agreement CLOSES it at once; windows are one at a time.
    final class Flip: @unchecked Sendable { let lock = NSLock(); var calls = 0; var after: Int; init(after: Int) { self.after = after }
        func wifi() -> Bool { lock.lock(); defer { lock.unlock() }; calls += 1; return calls <= after } }
    /// Wi-Fi until `arm()`, then Wi-Fi at the first look and gone from the second: so only the check made INSIDE the lock that opens the registry can refuse (what `sync` and the early look consume before it does not count).
    final class Armed: @unchecked Sendable { let lock = NSLock(); var armed = false, calls = 0
        func arm() { lock.lock(); armed = true; lock.unlock() }
        func wifi() -> Bool { lock.lock(); defer { lock.unlock() }; if !armed { return true }; calls += 1; return calls <= 1 } }
    func testW1OffWifiBetweenTheCheckAndTheOpenOpensNothing() async {
        let r = Registry(); let f = Armed()
        let d = DownloadDoor(onWiFi: { f.wifi() }); d.sync(agreed: ["voice-rest": "agreed"]); d.installLibraryGuard { r.set($0) }
        f.arm()                                                                         // from here: the early look sees Wi-Fi, the look inside the lock does not
        final class Ran: @unchecked Sendable { var v = false }; let ran = Ran()
        do { try await d.libraryWindow(model: "fluid-speech-mini") { ran.v = true } as Void; XCTFail("opened") } catch { XCTAssertEqual(error as? DownloadDoor.Refusal, .pathNotAgreed(group: "voice-rest")) }
        XCTAssertFalse(ran.v); XCTAssertEqual(r.log, [DownloadDoor.closedRegistry], "the registry was never opened"); XCTAssertEqual(d.openWindowCount, 0)
        XCTAssertEqual(f.calls, 2, "it looked twice: before the lock, and inside it")
    }
    func testW2AResetDuringAFetchClosesTheRegistryAtOnceAndACancelFollows() async throws {
        let r = Registry(); let d = door(); d.sync(agreed: ["voice-rest": "agreed-mobile"]); d.installLibraryGuard { r.set($0) }
        final class S: @unchecked Sendable { var started = false, cancelled = false }; let st = S()
        let t = Task { try await d.libraryWindow(model: "fluid-speech-mini") { st.started = true; for _ in 0..<400 { if Task.isCancelled { st.cancelled = true; throw CancellationError() }; try? await Task.sleep(nanoseconds: 5_000_000) } } as Void }
        for _ in 0..<200 where !st.started { try? await Task.sleep(nanoseconds: 5_000_000) }
        XCTAssertEqual(r.now, DownloadDoor.openRegistry)
        d.reset()                                                                         // sign-out
        XCTAssertEqual(r.now, DownloadDoor.closedRegistry, "closed the instant the account changed, not when the transfer happens to end")
        do { try await t.value; XCTFail("ran on") } catch {}
        for _ in 0..<100 where !st.cancelled { try? await Task.sleep(nanoseconds: 10_000_000) }
        XCTAssertTrue(st.cancelled); XCTAssertEqual(r.now, DownloadDoor.closedRegistry); XCTAssertEqual(d.openWindowCount, 0, "and the count did not underflow")
    }
    func testW3AWithdrawnAgreementKillsTheWindow() async throws {
        let r = Registry(); let d = door(); d.sync(agreed: ["voice-rest": "agreed-mobile"]); d.installLibraryGuard { r.set($0) }
        final class S: @unchecked Sendable { var started = false }; let st = S()
        let t = Task { try await d.libraryWindow(model: "fluid-speech-mini") { st.started = true; for _ in 0..<400 { try Task.checkCancellation(); try? await Task.sleep(nanoseconds: 5_000_000) } } as Void }
        for _ in 0..<200 where !st.started { try? await Task.sleep(nanoseconds: 5_000_000) }
        d.sync(agreed: [:]); XCTAssertEqual(r.now, DownloadDoor.closedRegistry)
        do { try await t.value; XCTFail("ran on") } catch {}
    }
    func testW4ABodyThatIgnoresCancellationStillHasTheRegistryClosedAtOnce() async throws {
        let r = Registry(); let d = door(wifi: true); d.sync(agreed: ["voice-rest": "agreed"]); d.installLibraryGuard { r.set($0) }
        final class S: @unchecked Sendable { var started = false; var done = false }; let st = S()
        let t = Task { try await d.libraryWindow(model: "fluid-speech-mini") { st.started = true; try? await Task.sleep(nanoseconds: 150_000_000); st.done = true } as Void }
        for _ in 0..<200 where !st.started { try? await Task.sleep(nanoseconds: 5_000_000) }
        d.pathChanged(onWiFi: false)
        XCTAssertEqual(r.now, DownloadDoor.closedRegistry, "the library that does not honour cancellation has nowhere to send its next request")
        _ = try? await t.value; XCTAssertEqual(r.now, DownloadDoor.closedRegistry); XCTAssertEqual(d.openWindowCount, 0)
    }
    func testW5ErrorAndCancellationLeaveTheRegistryClosedAndTheCountAtZero() async {
        let r = Registry(); let d = door(); d.sync(agreed: ["voice-rest": "agreed"]); d.installLibraryGuard { r.set($0) }
        struct Boom: Error {}
        do { try await d.libraryWindow(model: "fluid-speech-mini") { throw Boom() } as Void } catch {}
        XCTAssertEqual(r.now, DownloadDoor.closedRegistry); XCTAssertEqual(d.openWindowCount, 0)
        let t = Task { try await d.libraryWindow(model: "fluid-speech-mini") { try await Task.sleep(nanoseconds: 2_000_000_000) } as Void }
        try? await Task.sleep(nanoseconds: 30_000_000); t.cancel(); _ = try? await t.value
        XCTAssertEqual(r.now, DownloadDoor.closedRegistry); XCTAssertEqual(d.openWindowCount, 0)
        let ok: Int = (try? await d.libraryWindow(model: "fluid-speech-mini") { 7 }) ?? 0; XCTAssertEqual(ok, 7, "and the door still works afterwards")
    }
    // A window is one group at a time: a fetch for another group waits for it; the other group's models never run inside it.
    func testW6AFetchForAnotherGroupWaitsForTheOpenWindow() async throws {
        let r = Registry(); let d = door(); d.sync(agreed: ["voice-rest": "agreed", "voice": "agreed"]); d.installLibraryGuard { r.set($0) }
        final class L: @unchecked Sendable { let lock = NSLock(); var log: [String] = []; func add(_ s: String) { lock.lock(); log.append(s); lock.unlock() } }; let l = L()
        let a = Task { try await d.libraryWindow(model: "fluid-speech-mini") { l.add("A+"); try? await Task.sleep(nanoseconds: 100_000_000); l.add("A-") } as Void }
        try? await Task.sleep(nanoseconds: 30_000_000)
        let b = Task { try await d.libraryWindow(model: VoiceCatalog.kokoroModel) { l.add("B+"); l.add("B-") } as Void }
        _ = try await a.value; _ = try await b.value
        XCTAssertEqual(l.log, ["A+", "A-", "B+", "B-"], "B's body ran only after A's window closed")
        XCTAssertEqual(r.now, DownloadDoor.closedRegistry)
    }

    // ---- Round 6: the queue. A cancelled waiter never opens a window; a body that outlives its deadline is failed and frees the slot; a queued fetch tells the watchdog it is queued; `exclusive` (the check) runs with no window anywhere.
    func testQ1ACancelledWaiterNeverOpensAWindow() async throws {
        let r = Registry(); let d = door(); d.sync(agreed: ["voice-rest": "agreed"]); d.installLibraryGuard { r.set($0) }
        final class S: @unchecked Sendable { var ranB = false, aStarted = false }; let st = S()
        let a = Task { try await d.libraryWindow(model: "fluid-speech-mini") { st.aStarted = true; try? await Task.sleep(nanoseconds: 150_000_000) } as Void }
        for _ in 0..<200 where !st.aStarted { try? await Task.sleep(nanoseconds: 5_000_000) }
        let b = Task { try await d.libraryWindow(model: "parakeet-eou-120m") { st.ranB = true } as Void }
        try? await Task.sleep(nanoseconds: 30_000_000); b.cancel()
        do { try await b.value; XCTFail("ran") } catch {}
        _ = try await a.value
        XCTAssertFalse(st.ranB); XCTAssertEqual(r.log.filter { $0 == DownloadDoor.openRegistry }.count, 1, "the registry was opened once, for A"); XCTAssertEqual(d.openWindowCount, 0)
    }
    func testQ2ABodyThatOutlivesItsDeadlineIsFailedAndTheSlotIsFreed() async throws {
        let r = Registry(); let d = DownloadDoor(onWiFi: { true }, deadlines: (wait: 5, body: 0.1)); d.sync(agreed: ["voice-rest": "agreed", "voice": "agreed"]); d.installLibraryGuard { r.set($0) }
        let t0 = Date()
        let hang = Hang(); defer { hang.release() }
        do { try await d.libraryWindow(model: "fluid-speech-mini") { await hang.wait() } as Void; XCTFail("ran on") } catch { XCTAssertEqual(error as? DownloadDoor.Refusal, .timedOut) }     // a body that ignores cancellation
        XCTAssertLessThan(Date().timeIntervalSince(t0), 2, "it did not wait for a body that never returns")
        XCTAssertEqual(r.now, DownloadDoor.closedRegistry); XCTAssertEqual(d.openWindowCount, 0)
        let ok: Int = try await d.libraryWindow(model: VoiceCatalog.kokoroModel) { 3 }; XCTAssertEqual(ok, 3, "and the next fetch of another model is not stuck behind it")
    }
    func testQ3AQueuedFetchTellsTheWatchdogItIsQueued() async throws {
        let d = door(); d.sync(agreed: ["voice-rest": "agreed"]); d.installLibraryGuard { _ in }
        final class S: @unchecked Sendable { var started = false; var told = 0 }; let st = S()
        let a = Task { try await d.libraryWindow(model: "fluid-speech-mini") { st.started = true; try? await Task.sleep(nanoseconds: 400_000_000) } as Void }
        for _ in 0..<200 where !st.started { try? await Task.sleep(nanoseconds: 5_000_000) }
        let b = Task { try await d.libraryWindow(model: "parakeet-eou-120m", queuedEvery: 0.05, whileQueued: { st.told += 1 }) { } as Void }
        _ = try await a.value; _ = try await b.value
        XCTAssertGreaterThanOrEqual(st.told, 3, "the waiter reported while it waited, so a stall watchdog does not read the wait as a stall")
    }
    func testQ4ExclusiveRunsWithNoWindowAnywhereAndWindowsWaitForIt() async throws {
        let r = Registry(); let d = door(); d.sync(agreed: ["voice-rest": "agreed"]); d.installLibraryGuard { r.set($0) }
        final class L: @unchecked Sendable { let lock = NSLock(); var log: [String] = []; func add(_ s: String) { lock.lock(); log.append(s); lock.unlock() } }; let l = L()
        let e = Task { try await d.exclusive { l.add("E+:" + r.now); try? await Task.sleep(nanoseconds: 100_000_000); l.add("E-") } }
        try? await Task.sleep(nanoseconds: 30_000_000)
        let w = Task { try await d.libraryWindow(model: "fluid-speech-mini") { l.add("W+") } as Void }
        _ = try await e.value; _ = try await w.value
        XCTAssertEqual(l.log, ["E+:" + DownloadDoor.closedRegistry, "E-", "W+"], "the check ran with the registry closed and no window open; the window waited")
    }

    // ---- Round 7: the deadline is about PROGRESS (a 480 MB model on a slow line is not a hang); `exclusive` has one too; a cancelled caller does not wait for it.
    func testQ5AnActiveBodyIsNotKilledByTheDeadlineButASilentOneIs() async throws {
        let d = DownloadDoor(onWiFi: { true }, deadlines: (wait: 5, body: 0.15)); d.sync(agreed: ["voice-rest": "agreed"]); d.installLibraryGuard { _ in }
        let ok: Int = try await d.libraryWindow(model: "fluid-speech-mini") { for _ in 0..<12 { try await Task.sleep(nanoseconds: 40_000_000); d.noteProgress() }; return 1 }
        XCTAssertEqual(ok, 1, "0.48 s of work against a 0.15 s no-progress deadline: it kept reporting, so it lives")
        do { try await d.libraryWindow(model: "fluid-speech-mini") { try await Task.sleep(nanoseconds: 2_000_000_000) } as Void; XCTFail("silent body survived") } catch { XCTAssertEqual(error as? DownloadDoor.Refusal, .timedOut) }
    }
    func testQ6ExclusiveHasADeadline_AndItsSlotIsFreedOnlyWhenTheBodyHasEnded() async throws {
        let d = DownloadDoor(onWiFi: { true }, deadlines: (wait: 0.2, body: 0.3))
        final class F: @unchecked Sendable { let l = NSLock(); var stop = false; var ended = false
            func set() { l.lock(); stop = true; l.unlock() }; var s: Bool { l.lock(); defer { l.unlock() }; return stop }
            func end() { l.lock(); ended = true; l.unlock() } }
        let f = F()
        let hang = Hang(); defer { hang.release() }
        do { try await d.exclusive(model: "fluid-speech-mini") { await hang.wait(); f.end() } as Void; XCTFail("ran on") } catch { XCTAssertEqual(error as? DownloadDoor.Refusal, .timedOut) }   // a body that ignores cancellation
        // the caller was told at once, but the body is still running: the slot is NOT free, so nothing else runs beside it
        do { let _: Int = try await d.exclusive { 5 }; XCTFail("a second body ran beside the one that outlived its deadline") } catch { XCTAssertEqual(error as? DownloadDoor.Refusal, .timedOut, "it waited, and its patience ran out") }
        hang.release(); try? await Task.sleep(nanoseconds: 100_000_000)                                     // the body returns: the slot is freed
        let ok: Int = try await d.exclusive { 5 }; XCTAssertEqual(ok, 5)
    }
    func testQ7ACancelledCallerClosesTheRegistryAndReturnsAtOnce() async throws {
        let r = Registry(); let d = door(); d.sync(agreed: ["voice-rest": "agreed"]); d.installLibraryGuard { r.set($0) }
        final class S: @unchecked Sendable { var started = false }; let st = S()
        let hang = Hang(); defer { hang.release() }
        let t = Task { try await d.libraryWindow(model: "fluid-speech-mini") { st.started = true; await hang.wait() } as Void }     // a body that ignores cancellation
        for _ in 0..<200 where !st.started { try? await Task.sleep(nanoseconds: 5_000_000) }
        let t0 = Date(); t.cancel(); _ = try? await t.value
        XCTAssertLessThan(Date().timeIntervalSince(t0), 1); XCTAssertEqual(r.now, DownloadDoor.closedRegistry); XCTAssertEqual(d.openWindowCount, 0)
    }

    // ---- Round 9 (F4): a body that never returns cannot wedge the door for ever. At the hard cap (twice the deadline) the slot is released, but that model's windows stay refused until the body ends, and the registry is closed.
    func testH1ABodyThatNeverReturnsReleasesTheSlotAtTheCapAndKeepsItsOwnModelRefused() async throws {
        let r = Registry(); let d = DownloadDoor(onWiFi: { true }, deadlines: (wait: 5, body: 0.1)); d.sync(agreed: ["voice-rest": "agreed", "voice": "agreed"]); d.installLibraryGuard { r.set($0) }
        let hang = Hang(); defer { hang.release() }
        do { try await d.exclusive(model: "fluid-speech-mini") { await hang.wait() } as Void; XCTFail("ran on") } catch { XCTAssertEqual(error as? DownloadDoor.Refusal, .timedOut) }
        let t0 = Date()
        let ok: Int = try await d.exclusive(model: "kokoro-82m-ane") { 5 }                       // another model: after the cap the slot is free, no five minute wait
        XCTAssertEqual(ok, 5); XCTAssertLessThan(Date().timeIntervalSince(t0), 2)
        do { try await d.libraryWindow(model: "fluid-speech-mini") { } as Void; XCTFail("opened for the model whose body is still running") } catch { XCTAssertEqual(error as? DownloadDoor.Refusal, .timedOut) }
        XCTAssertEqual(r.now, DownloadDoor.closedRegistry)
        let other: Int = try await d.libraryWindow(model: VoiceCatalog.kokoroModel) { 9 }; XCTAssertEqual(other, 9, "another model's window is not refused")
        hang.release(); try? await Task.sleep(nanoseconds: 100_000_000)
        let again: Int = try await d.libraryWindow(model: "fluid-speech-mini") { 1 }; XCTAssertEqual(again, 1, "refused only until the body ends")
    }

    // ---- Round 10 (B3): the stuck check is made again, under the lock, after the wait; and `exclusive` refuses a stuck model.
    func testH2AFetchThatQueuedBeforeTheCapIsRefusedWhenItsTurnComesWhileTheBodyIsStillRunning() async throws {
        let r = Registry(); let d = DownloadDoor(onWiFi: { true }, deadlines: (wait: 5, body: 0.1)); d.sync(agreed: ["voice-rest": "agreed"]); d.installLibraryGuard { r.set($0) }
        let hang = Hang(); defer { hang.release() }
        let stuck = Task { try await d.exclusive(model: "fluid-speech-mini") { await hang.wait() } as Void }
        try? await Task.sleep(nanoseconds: 30_000_000)
        let queued = Task { try await d.libraryWindow(model: "fluid-speech-mini") { } as Void }          // it asked while the slot was busy: the early look saw nothing stuck
        do { try await queued.value; XCTFail("a window opened beside the body that is still running") } catch { XCTAssertEqual(error as? DownloadDoor.Refusal, .timedOut) }
        XCTAssertEqual(r.log.filter { $0 == DownloadDoor.openRegistry }.count, 0, "the registry was never opened")
        do { try await d.exclusive(model: "fluid-speech-mini") { 1 } as Int; XCTFail("a second cold load beside the stuck one") } catch { XCTAssertEqual(error as? DownloadDoor.Refusal, .timedOut) }
        _ = try? await stuck.value
    }

    // ---- Round 11 (D4): a window body that was given up on (deadline, cancel) and still runs keeps ITS model refused until it ends, as a cold load does.
    func testH3AHungFetchWindowKeepsItsModelRefusedUntilItsBodyEnds() async throws {
        let d = DownloadDoor(onWiFi: { true }, deadlines: (wait: 5, body: 0.1)); d.sync(agreed: ["voice-rest": "agreed"]); d.installLibraryGuard { _ in }
        let hang = Hang(); defer { hang.release() }
        do { try await d.libraryWindow(model: "fluid-speech-mini") { await hang.wait() } as Void; XCTFail("ran on") } catch { XCTAssertEqual(error as? DownloadDoor.Refusal, .timedOut) }
        do { try await d.libraryWindow(model: "fluid-speech-mini") { } as Void; XCTFail("a second window beside a body that is still writing") } catch { XCTAssertEqual(error as? DownloadDoor.Refusal, .timedOut) }
        hang.release(); try? await Task.sleep(nanoseconds: 100_000_000)
        let ok: Int = try await d.libraryWindow(model: "fluid-speech-mini") { 4 }; XCTAssertEqual(ok, 4, "allowed again once the body ended")
    }
}

