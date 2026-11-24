import XCTest
import CryptoKit
@testable import AutoradioKit

// Round 5: readiness by MANIFEST, not by "90% of a round number". A library-managed model is ready iff a manifest (every file's relative path, exact length, SHA-256 of the files as they were when the check passed) exists for the current pin and every
// file is present with that length. Trust on first consented fetch: the digests are of what the first consented fetch left on disk.
final class ModelManifestTests: XCTestCase {
    func tmp() -> URL { let d = FileManager.default.temporaryDirectory.appendingPathComponent("man-\(UUID().uuidString)", isDirectory: true); try? FileManager.default.createDirectory(at: d, withIntermediateDirectories: true); return d }
    func put(_ base: URL, _ path: String, _ bytes: Int) { let u = base.appendingPathComponent(path); try? FileManager.default.createDirectory(at: u.deletingLastPathComponent(), withIntermediateDirectories: true); try? Data((0..<bytes).map { UInt8($0 % 251) }).write(to: u) }
    /// A fake library cache: two folders under a base; the "model card" says 350 MB and the library keeps a few KB: the round number must not matter.
    func make() -> (base: URL, store: URL, spec: ManagedModelSpec) {
        let base = tmp(), store = tmp()
        put(base, "kokoro/a.mlmodelc/weights/w.bin", 3000); put(base, "kokoro/a.mlmodelc/model.mil", 700); put(base, "kokoro-82m-coreml/voices.json", 120)
        return (base, store, ManagedModelSpec(id: "kk", label: "k", source: "hf:x", approxBytes: 350_000_000, license: "x", cachePath: base, cacheFolders: ["kokoro", "kokoro-82m-coreml"]))
    }
    func pins(_ m: (base: URL, store: URL, spec: ManagedModelSpec)) -> FileVoiceReadiness {
        FileVoiceReadiness(file: m.store.appendingPathComponent("ready.json"), pins: { ModelPins.managed($0, spec: m.spec, root: m.store) })
    }

    func testM1ReadyWhenTheManifestIsWrittenAtThePassWhateverTheCardSays() {
        let m = make(), r = pins(m)
        XCTAssertFalse(r.isReady(model: "kk"), "nothing passed yet")
        r.markReady(model: "kk")                                                       // the verifier's pass: seals the manifest
        XCTAssertTrue(r.isReady(model: "kk"), "3.8 KB on disk of a 350 MB card figure: exact, not 90%")
        let man = ModelManifests.read("kk", root: m.store)!
        XCTAssertEqual(Set(man.files.map(\.path)), ["kokoro/a.mlmodelc/weights/w.bin", "kokoro/a.mlmodelc/model.mil", "kokoro-82m-coreml/voices.json"])
        XCTAssertEqual(man.files.first { $0.path.hasSuffix("w.bin") }?.bytes, 3000)
    }
    func testM2AMissingFileIsNotReadyEvenWhenAlmostEverythingIsThere() {
        let m = make(), r = pins(m); r.markReady(model: "kk")
        try? FileManager.default.removeItem(at: m.base.appendingPathComponent("kokoro-82m-coreml/voices.json"))      // 3% of the bytes: the old 90% rule called it ready
        XCTAssertFalse(r.isReady(model: "kk"))
    }
    func testM3ATruncatedFileIsNotReady() {
        let m = make(), r = pins(m); r.markReady(model: "kk")
        put(m.base, "kokoro/a.mlmodelc/weights/w.bin", 2999); XCTAssertFalse(r.isReady(model: "kk"))
    }
    func testM4AFetchDemotesFirst_SoAFailedReFetchIsNotReady() {
        let m = make(), r = pins(m); r.markReady(model: "kk"); XCTAssertTrue(r.isReady(model: "kk"))
        r.markNotReady(model: "kk")                                                    // what the provider does before every fetch
        XCTAssertNil(ModelManifests.read("kk", root: m.store), "the manifest is gone")
        XCTAssertFalse(r.isReady(model: "kk"), "the files are all there and it is still not ready: a re-fetch that fails must not leave it ready")
    }
    func testM5APinChangeIsNotReady() {
        let m = make(), r = pins(m); r.markReady(model: "kk")
        let other = FileVoiceReadiness(file: m.store.appendingPathComponent("ready.json"), pins: { ModelPins.managed($0, spec: m.spec, root: m.store, libraryPin: "fluidaudio-9.9.9") })
        XCTAssertFalse(other.isReady(model: "kk"))
    }
    func testM6HashesAreVerifiedWhenAskedAndCatchAChangeThatKeepsTheLength() {
        let m = make(), r = pins(m); r.markReady(model: "kk")
        let man = ModelManifests.read("kk", root: m.store)!
        XCTAssertTrue(ModelManifests.verifyHashes(man, base: m.base))
        var d = try! Data(contentsOf: m.base.appendingPathComponent("kokoro/a.mlmodelc/model.mil")); d[0] ^= 0xFF; try! d.write(to: m.base.appendingPathComponent("kokoro/a.mlmodelc/model.mil"))
        XCTAssertTrue(r.isReady(model: "kk"), "lengths only on every launch")
        XCTAssertFalse(ModelManifests.verifyHashes(man, base: m.base), "the hash check (at the check that grants readiness, and after a failed load) sees it")
    }
    func testM7AnEmptyFolderCannotBeSealed() {
        let base = tmp(), store = tmp()
        let spec = ManagedModelSpec(id: "kk", label: "k", source: "hf:x", approxBytes: 5, license: "x", cachePath: base, cacheFolders: ["nothing"])
        let r = FileVoiceReadiness(file: store.appendingPathComponent("ready.json"), pins: { ModelPins.managed($0, spec: spec, root: store) })
        r.markReady(model: "kk"); XCTAssertFalse(r.isReady(model: "kk"), "no files, no manifest, no readiness")
    }

    // Round 5: a library is initialised only for a model that is ready by manifest, or while it is being fetched or checked (LibraryScope). Anything else is not a load: it would be a fetch of missing files through someone else's window.
    func testS1ALibraryInitialiseForAModelThatIsNotReadyIsRefused() async {
        let rec = MemoryVoiceReadiness()
        XCTAssertThrowsError(try LibraryScope.require("kokoro-82m-ane", readiness: rec))
        XCTAssertNoThrow(try LibraryScope.$checking.withValue("kokoro-82m-ane") { try LibraryScope.require("kokoro-82m-ane", readiness: rec) }, "inside its own fetch or check")
        XCTAssertThrowsError(try LibraryScope.$checking.withValue("pocket-tts") { try LibraryScope.require("kokoro-82m-ane", readiness: rec) }, "another model's scope does not cover it")
        rec.markReady(model: "kokoro-82m-ane"); XCTAssertNoThrow(try LibraryScope.require("kokoro-82m-ane", readiness: rec))
    }
    func testS2WithheldIsDerivedFromTheManifestAndTheLoadEntryPoint() {
        let noLocal = ManagedModelSpec(id: "k", label: "k", source: "x", approxBytes: 1, license: "x", cachePath: nil, cacheFolders: ["k"], loadsWithoutFetching: false)
        let local = ManagedModelSpec(id: "t", label: "t", source: "x", approxBytes: 1, license: "x", cachePath: nil, cacheFolders: ["t"], loadsWithoutFetching: true)
        XCTAssertTrue(ModelOffer.isWithheld(noLocal, ready: false), "a library with no local-only load, with no manifest: withheld")
        XCTAssertFalse(ModelOffer.isWithheld(noLocal, ready: true), "offered once its manifest exists")
        XCTAssertFalse(ModelOffer.isWithheld(local, ready: false), "a library that loads from the folder without fetching is offered")
        for id in ["kokoro-82m-ane", "pocket-tts", "phonon-31m", "silero-vad-v6.2.1"] { XCTAssertFalse(KnownModels.managed.first { $0.id == id }!.loadsWithoutFetching, id) }
        for id in ["parakeet-eou-120m", "parakeet-tdt-0.6b-v2", "fluid-speech-mini"] { XCTAssertTrue(KnownModels.managed.first { $0.id == id }!.loadsWithoutFetching, id) }
    }

    // ---- Round 6
    func testM8TheSealFailsOnAnUnreadableEntry() {
        let m = make(), r = pins(m)
        let f = m.base.appendingPathComponent("kokoro/a.mlmodelc/model.mil"); try? FileManager.default.setAttributes([.posixPermissions: 0], ofItemAtPath: f.path)
        defer { try? FileManager.default.setAttributes([.posixPermissions: 0o644], ofItemAtPath: f.path) }
        r.markReady(model: "kk"); XCTAssertFalse(r.isReady(model: "kk"), "a seal over a subset is not a seal")
        XCTAssertNil(ModelManifests.read("kk", root: m.store))
    }
    func testM9ASymlinkedEntryFailsTheSealInsteadOfPoisoningThePaths() {
        let m = make(), r = pins(m)
        let other = tmp(); put(other, "x.bin", 10)
        try? FileManager.default.createSymbolicLink(at: m.base.appendingPathComponent("kokoro/linked"), withDestinationURL: other)
        r.markReady(model: "kk"); XCTAssertFalse(r.isReady(model: "kk"))
    }
    func testM10AnEmptyManifestIsNeverPresent() {
        XCTAssertFalse(ModelManifests.present(ModelManifest(pin: "p", files: []), base: tmp()))
        XCTAssertFalse(ModelManifests.verifyHashes(ModelManifest(pin: "p", files: []), base: tmp()))
    }
    func testM11TheLibraryPinIsThePinnedFluidAudioVersion() throws {
        let url = URL(fileURLWithPath: #filePath).deletingLastPathComponent().deletingLastPathComponent().deletingLastPathComponent().appendingPathComponent("Package.resolved")
        let d = try JSONSerialization.jsonObject(with: Data(contentsOf: url)) as! [String: Any]
        let pins = d["pins"] as! [[String: Any]]
        let v = (pins.first { ($0["identity"] as? String)?.lowercased() == "fluidaudio" }!["state"] as! [String: Any])["version"] as! String
        XCTAssertEqual(ModelPins.libraryPin, "fluidaudio-" + v, "a FluidAudio bump is a pin change: bump the literal with it")
    }
    func testM12AFailedCheckAfterAPinChangeDeletesTheOldFilesAndSaysTheLayoutChanged() async {
        let m = make(), r = pins(m); r.markReady(model: "kk")
        let moved = FileVoiceReadiness(file: m.store.appendingPathComponent("ready.json"), pins: { ModelPins.managed($0, spec: m.spec, root: m.store, libraryPin: "fluidaudio-9.9.9") })
        let v = ReadinessVerifier(record: moved)
        struct Boom: Error {}
        let res = await v.verify(model: "kk", check: { throw Boom() })
        XCTAssertTrue(res.note.contains("layout"), res.note)
        XCTAssertFalse(FileManager.default.fileExists(atPath: m.base.appendingPathComponent("kokoro").path), "the old folders are gone, so the next fetch starts clean and is not a loop over files that will not load")
    }
    struct Boom: Error {}
    func testM13AFailedLoadWithDamagedFilesDeletesThemAndSaysSo_AndWithIntactFilesSaysTheyDidNotLoad() async {
        // damaged: the hashes no longer match
        var m = make(); var r = pins(m); r.markReady(model: "kk")
        var d = try! Data(contentsOf: m.base.appendingPathComponent("kokoro/a.mlmodelc/model.mil")); d[0] ^= 0xFF; try! d.write(to: m.base.appendingPathComponent("kokoro/a.mlmodelc/model.mil"))
        var res = await ReadinessVerifier(record: r).verify(model: "kk", check: { throw Boom() })
        XCTAssertTrue(res.note.contains("damaged"), res.note); XCTAssertFalse(FileManager.default.fileExists(atPath: m.base.appendingPathComponent("kokoro").path))
        // intact: the hashes match, the load still failed: the files are kept and the words say they did not load
        m = make(); r = pins(m); r.markReady(model: "kk")
        res = await ReadinessVerifier(record: r).verify(model: "kk", check: { throw Boom() })
        XCTAssertTrue(res.note.contains("did not load from this phone's files"), res.note); XCTAssertTrue(FileManager.default.fileExists(atPath: m.base.appendingPathComponent("kokoro").path), "intact files are not thrown away")
    }
    func testM14ACheckThatRanAndJudgedTheOutputKeepsItsOwnWords() async {
        let m = make(), r = pins(m); r.markReady(model: "kk")
        let res = await ReadinessVerifier(record: r).verify(model: "kk", check: { SetupCheck(loadMs: 0, speed: 0, unit: "", memoryMB: 0, outputOK: false, note: "The voice made audio of the wrong length.") })
        XCTAssertEqual(res.note, "The voice made audio of the wrong length.", "it loaded; its output was wrong: that is what the row says")
    }

    // ---- Round 7. A registry close mid-download leaves a pack the library's presence checks call complete. The repair decision: unproven files are deleted; only proven-intact files are kept.
    /// A fake library that looks only at whether the folder is there (FluidAudio's Pocket and Phonon do): present => it downloads nothing.
    final class PresenceOnlyLibrary: @unchecked Sendable {
        let base: URL; var fetches = 0; var failAfterFiles: Int?
        init(_ base: URL) { self.base = base }
        func fetch() async throws {
            fetches += 1
            let dir = base.appendingPathComponent("pack")
            if FileManager.default.fileExists(atPath: dir.path) { return }                                   // presence only: a half pack is "complete"
            try FileManager.default.createDirectory(at: dir, withIntermediateDirectories: true)
            for (i, name) in ["a.bin", "b.bin", "weights.bin"].enumerated() {
                if let n = failAfterFiles, i >= n { throw URLError(.cannotConnectToHost) }        // the registry closed mid-download
                try Data(repeating: UInt8(i + 1), count: 100 + i).write(to: dir.appendingPathComponent(name))
            }
        }
        func load() throws { for n in ["a.bin", "b.bin", "weights.bin"] where !FileManager.default.fileExists(atPath: base.appendingPathComponent("pack/" + n).path) { throw URLError(.cannotLoadFromNetwork) } }
    }
    func lib() -> (base: URL, store: URL, spec: ManagedModelSpec, lib: PresenceOnlyLibrary) {
        let base = tmp(), store = tmp()
        return (base, store, ManagedModelSpec(id: "pk", label: "p", source: "x", approxBytes: 1, license: "x", cachePath: base, cacheFolders: ["pack"]), PresenceOnlyLibrary(base))
    }
    func testR1AFetchThatFailsMidwayLeavesNoHalfPackAndTheSecondFetchRepairs() async throws {
        let m = lib(); let rec = FileVoiceReadiness(file: m.store.appendingPathComponent("ready.json"), pins: { ModelPins.managed($0, spec: m.spec, root: m.store) }); let v = ReadinessVerifier(record: rec)
        m.lib.failAfterFiles = 1
        do { try await ModelFetch.guarded(model: "pk", verifier: v, pins: { ModelPins.managed($0, spec: m.spec, root: m.store) }) { try await m.lib.fetch() }; XCTFail("fetched") } catch {}
        XCTAssertFalse(FileManager.default.fileExists(atPath: m.base.appendingPathComponent("pack").path), "the half pack is gone: it must not look complete to a library that only checks presence")
        m.lib.failAfterFiles = nil
        try await ModelFetch.guarded(model: "pk", verifier: v, pins: { ModelPins.managed($0, spec: m.spec, root: m.store) }) { try await m.lib.fetch() }
        XCTAssertNoThrow(try m.lib.load(), "the second fetch downloaded everything again")
        let r = await v.verify(model: "pk", check: { try m.lib.load(); return SetupCheck(loadMs: 1, speed: 1, unit: "x", memoryMB: 0, outputOK: true) })
        XCTAssertTrue(r.outputOK); XCTAssertTrue(rec.isReady(model: "pk"))
    }
    func testR2ACancelledFetchPurgesToo() async {
        let m = lib(); let v = ReadinessVerifier(record: FileVoiceReadiness(file: m.store.appendingPathComponent("ready.json"), pins: { ModelPins.managed($0, spec: m.spec, root: m.store) }))
        let t = Task { try await ModelFetch.guarded(model: "pk", verifier: v, pins: { ModelPins.managed($0, spec: m.spec, root: m.store) }) { try FileManager.default.createDirectory(at: m.base.appendingPathComponent("pack"), withIntermediateDirectories: true); try await Task.sleep(nanoseconds: 2_000_000_000) } }
        try? await Task.sleep(nanoseconds: 50_000_000); t.cancel(); _ = try? await t.value
        XCTAssertFalse(FileManager.default.fileExists(atPath: m.base.appendingPathComponent("pack").path))
    }
    func testR3AFailedLoadWithNothingThatProvesIntegrityDeletesTheFiles_AndProvenIntactFilesAreKept() async throws {
        struct Boom: Error {}
        // no manifest, no baseline: unproven -> deleted
        var m = lib(); var rec = FileVoiceReadiness(file: m.store.appendingPathComponent("ready.json"), pins: { ModelPins.managed($0, spec: m.spec, root: m.store) })
        try await m.lib.fetch()
        var r = await ReadinessVerifier(record: rec).verify(model: "pk", check: { throw Boom() })
        XCTAssertFalse(FileManager.default.fileExists(atPath: m.base.appendingPathComponent("pack").path), "unproven files are not kept"); XCTAssertTrue(r.note.contains("deleted"), r.note)
        // a baseline (the last good manifest, kept across the demotion) that matches: intact -> kept
        m = lib(); rec = FileVoiceReadiness(file: m.store.appendingPathComponent("ready.json"), pins: { ModelPins.managed($0, spec: m.spec, root: m.store) })
        try await m.lib.fetch(); rec.markReady(model: "pk"); rec.markNotReady(model: "pk")                     // sealed once, then demoted by a fetch
        XCTAssertNotNil(ModelManifests.baseline("pk", root: m.store), "the last good manifest stays as the baseline")
        r = await ReadinessVerifier(record: rec).verify(model: "pk", check: { throw Boom() })
        XCTAssertTrue(FileManager.default.fileExists(atPath: m.base.appendingPathComponent("pack").path), "proven intact: kept"); XCTAssertTrue(r.note.contains("intact"), r.note)
        // a baseline that no longer matches: changed -> deleted
        m = lib(); rec = FileVoiceReadiness(file: m.store.appendingPathComponent("ready.json"), pins: { ModelPins.managed($0, spec: m.spec, root: m.store) })
        try await m.lib.fetch(); rec.markReady(model: "pk"); rec.markNotReady(model: "pk")
        var d = try Data(contentsOf: m.base.appendingPathComponent("pack/a.bin")); d[0] ^= 0xFF; try d.write(to: m.base.appendingPathComponent("pack/a.bin"))
        r = await ReadinessVerifier(record: rec).verify(model: "pk", check: { throw Boom() })
        XCTAssertFalse(FileManager.default.fileExists(atPath: m.base.appendingPathComponent("pack").path)); XCTAssertTrue(r.note.contains("damaged"), r.note)
    }
    func testR4ACheckThatNeverRanDeletesNothing() async throws {
        let m = lib(); let rec = FileVoiceReadiness(file: m.store.appendingPathComponent("ready.json"), pins: { ModelPins.managed($0, spec: m.spec, root: m.store) })
        try await m.lib.fetch(); rec.markReady(model: "pk")
        for e: Error in [CancellationError(), DownloadDoor.Refusal.timedOut] {
            let r = await ReadinessVerifier(record: rec).verify(model: "pk", check: { throw e })
            XCTAssertTrue(FileManager.default.fileExists(atPath: m.base.appendingPathComponent("pack").path), "a check that did not run proves nothing against the files")
            XCTAssertTrue(r.note.contains("did not run"), r.note)
        }
    }
    func testR5AnUnreadableFileIsNotProofOfDamage() async throws {
        struct Boom: Error {}
        let m = lib(); let rec = FileVoiceReadiness(file: m.store.appendingPathComponent("ready.json"), pins: { ModelPins.managed($0, spec: m.spec, root: m.store) })
        try await m.lib.fetch(); rec.markReady(model: "pk")
        let f = m.base.appendingPathComponent("pack/b.bin"); try FileManager.default.setAttributes([.posixPermissions: 0], ofItemAtPath: f.path); defer { try? FileManager.default.setAttributes([.posixPermissions: 0o644], ofItemAtPath: f.path) }
        let r = await ReadinessVerifier(record: rec).verify(model: "pk", check: { throw Boom() })
        XCTAssertTrue(FileManager.default.fileExists(atPath: m.base.appendingPathComponent("pack").path), "unreadable is not changed: nothing is deleted on a transient read error"); XCTAssertTrue(r.note.contains("could not be read"), r.note)
    }
    func testR6AFailedSealIsAFailedCheckWithItsReason() async throws {
        let m = lib(); try await m.lib.fetch()
        let rec = FileVoiceReadiness(file: m.store.appendingPathComponent("ready.json"), pins: { ModelPins.managed($0, spec: m.spec, root: m.store) })
        let f = m.base.appendingPathComponent("pack/a.bin"); try FileManager.default.setAttributes([.posixPermissions: 0], ofItemAtPath: f.path); defer { try? FileManager.default.setAttributes([.posixPermissions: 0o644], ofItemAtPath: f.path) }
        let r = await ReadinessVerifier(record: rec).verify(model: "pk", check: { SetupCheck(loadMs: 1, speed: 1, unit: "x", memoryMB: 0, outputOK: true) })
        XCTAssertFalse(r.outputOK, "the row must not say Ready for a model that could not be sealed"); XCTAssertTrue(r.note.contains("could not be sealed"), r.note); XCTAssertFalse(rec.isReady(model: "pk"))
    }
}
