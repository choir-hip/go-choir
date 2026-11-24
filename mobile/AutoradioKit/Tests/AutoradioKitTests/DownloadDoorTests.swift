import XCTest
@testable import AutoradioKit

// DOOR A: every model fetch goes through DownloadDoor. One test per FORMER PATH proves the escape is closed: with no consent, each path makes zero requests; a Wi-Fi agreement cannot spend mobile data in the transport;
// sign-out closes the door. The fetcher is a counting URL protocol on the transport the door hands out.
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
    func testA4ANestedWindowKeepsItOpenUntilTheLastEnds() async throws {
        let r = Registry(); let d = door(); d.sync(agreed: ["voice-rest": "agreed"]); d.installLibraryGuard { r.set($0) }
        let seen: [String] = try await d.libraryWindow(model: "fluid-speech-mini") {
            let a = r.now
            let b: String = try await d.libraryWindow(model: "parakeet-eou-120m") { r.now }
            return [a, b, r.now]
        }
        XCTAssertEqual(seen, [DownloadDoor.openRegistry, DownloadDoor.openRegistry, DownloadDoor.openRegistry]); XCTAssertEqual(r.now, DownloadDoor.closedRegistry)
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
}
