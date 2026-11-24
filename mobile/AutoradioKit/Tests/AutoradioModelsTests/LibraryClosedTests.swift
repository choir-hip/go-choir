import XCTest
import FluidAudio
@testable import AutoradioKit
@testable import AutoradioModels

// Round 4: NO LIBRARY IS EVER INITIALISED IN A STATE WHERE IT CAN FETCH. The libraries' registry is closed from the registration on; a library asked to initialise for files that are not there fails and fetches NOTHING (an empty folder stays empty).
// (The network of this Mac would let a fetch succeed: a test that passes here with an empty folder shows the registry is what stopped it.)
final class LibraryClosedTests: XCTestCase {
    func empty() -> URL { let d = FileManager.default.temporaryDirectory.appendingPathComponent("lib-\(UUID().uuidString)", isDirectory: true); try? FileManager.default.createDirectory(at: d, withIntermediateDirectories: true); return d }
    func files(in d: URL) -> Int { ((FileManager.default.enumerator(at: d, includingPropertiesForKeys: [.isRegularFileKey])?.allObjects ?? []) as? [URL] ?? []).filter { (try? $0.resourceValues(forKeys: [.isRegularFileKey]).isRegularFile) == true }.count }
    override func setUp() { ModelHub.register(into: StageRegistry()) }                 // what the app does at launch
    override func tearDown() { DownloadDoor.shared.sync(agreed: [:]) }

    func testL1TheRegistryIsClosedAfterRegistration() {
        XCTAssertEqual(ModelRegistry.baseURL, DownloadDoor.closedRegistry)
    }
    func testL2TheLocalEntryPointsFailOnMissingFilesAndFetchNothing() async {
        let d1 = empty(); XCTAssertThrowsError(try AsrModels.loadLocal(from: d1, version: .v2)); XCTAssertEqual(files(in: d1), 0)          // AsrModels.swift:383
        let d2 = empty(); let m = StreamingEouAsrManager(chunkSize: .ms320)
        do { try await m.loadModels(from: d2); XCTFail("loaded from nothing") } catch {}; XCTAssertEqual(files(in: d2), 0)                      // StreamingEouAsrManager.swift:330
    }
    func testL3ALibraryThatCanFetchByItselfFetchesNothingWithTheRegistryClosed() async {
        let d = empty()
        do { _ = try await VadManager(config: VadConfig(defaultThreshold: 0.6, computeUnits: .cpuOnly), modelDirectory: d); XCTFail("a VAD appeared from nothing") } catch {}     // VadManager.swift:110: it resolves through ModelHub and would download
        XCTAssertEqual(files(in: d), 0, "nothing was fetched")
    }
    func testL4AStageInitialisesOnlyForAReadyModelAndChangesNoFile() async {
        // The TDT stage loads with `loadLocal` and only for a model that is ready by manifest (or being fetched or checked). On a Mac that has the files, they are not touched; on one that does not, nothing appears. Either way: not ready here, so it refuses.
        let tdt = AsrModels.defaultCacheDirectory(for: .v2).deletingLastPathComponent()
        let before = ModelStore.directoryBytes(tdt)
        do { _ = try await ParakeetTDTTranscriber(StageParams()).begin(); XCTFail("began for a model that is not ready") } catch { XCTAssertTrue("\(error)".contains("not ready"), "\(error)") }
        XCTAssertEqual(ModelStore.directoryBytes(tdt), before, "no file was added, changed or removed")
        await ModelStore.shared.unload("silero-vad-v6.2.1")                 // a manager another test left in the slot would answer without asking
        if !Voices.readiness.isReady(model: "silero-vad-v6.2.1") {           // a Mac that really holds a sealed Silero is allowed to load it
        do { _ = try await SileroVAD.probabilities([Float](repeating: 0, count: 4096)); XCTFail("silero initialised for a model that is not ready") } catch { XCTAssertTrue("\(error)".contains("not ready"), "\(error)") }
        }
    }
    func testL5NoConsentNoWindow() async {
        do { try await HubModelProvider.shared.fetch("parakeet-tdt-0.6b-v2", role: "", onBytes: { _ in }); XCTFail("fetched") } catch { XCTAssertEqual(error as? DownloadDoor.Refusal, .noConsent(group: "voice-rest")) }
        do { try await HubModelProvider.shared.fetch("lfm3-audio-2b", role: "", onBytes: { _ in }); XCTFail("fetched") } catch { XCTAssertEqual(error as? DownloadDoor.Refusal, .notOffered(model: "lfm3-audio-2b")) }
        XCTAssertEqual(ModelRegistry.baseURL, DownloadDoor.closedRegistry)
    }
}
