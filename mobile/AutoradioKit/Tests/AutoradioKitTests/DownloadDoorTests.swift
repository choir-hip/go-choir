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
    func testA1TheLibraryPathsAreRefusedWithoutConsent() {
        let d = door()
        for id in ["parakeet-tdt-0.6b-v2", "parakeet-eou-120m", "fluid-speech-mini", VoiceCatalog.kokoroModel, VoiceCatalog.pocketModel] {
            XCTAssertThrowsError(try d.authorizeLibrary(model: id), id)
        }
    }
    // A2: the transport itself, not a check before starting.
    func testA2AWifiAgreementHandsOutATransportThatCannotUseMobileData() throws {
        let d = door(); d.sync(agreed: ["voice-rest": "agreed", "search": "agreed-mobile"])
        let w = try d.session(for: responder).configuration
        XCTAssertFalse(w.allowsCellularAccess); XCTAssertFalse(w.allowsExpensiveNetworkAccess); XCTAssertFalse(w.allowsConstrainedNetworkAccess)
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
    func testA3EveryVoiceCardModelThatCanBeFetchedIsOffered() {
        let d = door()
        for o in VoiceCatalog.options where !o.modelID.isEmpty && o.modelID != VoiceCatalog.phononModel { XCTAssertNotNil(d.group(ofModel: o.modelID), "\(o.id) would be fetched by its card with no group to agree to") }
    }
    // A4
    func testA4ALibraryStartOnMobileDataUnderAWifiAgreementIsRefused() {
        let d = door(wifi: false); d.sync(agreed: ["voice-rest": "agreed"])
        XCTAssertThrowsError(try d.authorizeLibrary(model: "fluid-speech-mini")) { XCTAssertEqual($0 as? DownloadDoor.Refusal, .pathNotAgreed(group: "voice-rest")) }
        d.sync(agreed: ["voice-rest": "agreed-mobile"]); XCTAssertNoThrow(try d.authorizeLibrary(model: "fluid-speech-mini"))
        let w = door(wifi: true); w.sync(agreed: ["voice-rest": "agreed"]); XCTAssertNoThrow(try w.authorizeLibrary(model: "fluid-speech-mini"))
    }
    // A5
    func testA5SignOutClosesTheDoor() throws {
        let d = door(); d.sync(agreed: ["voice-rest": "agreed"]); XCTAssertNoThrow(try d.authorize(model: responder))
        d.sync(agreed: [:]); XCTAssertThrowsError(try d.authorize(model: responder))
        d.sync(agreed: ["voice-rest": "maybe"]); XCTAssertThrowsError(try d.authorize(model: responder), "an unknown word is no agreement")
    }
}
