import XCTest
import CryptoKit
@testable import AutoradioKit

// Round 5: readiness by MANIFEST, not by "90% of a round number". A library-managed model is ready iff a manifest (every file's relative path, exact length, SHA-256 of the files as they were when the check passed) exists for the current pin and every
// file is present with that length. Trust on first consented fetch: the digests are of what the first consented fetch left on disk.
final class ModelManifestTests: XCTestCase {
    let kkPin = ModelPins.libraryPin + "|kk", pkPin = ModelPins.libraryPin + "|pk"
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
        let man = ModelManifests.read("kk", pin: kkPin, root: m.store)!
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
        XCTAssertNil(ModelManifests.read("kk", pin: kkPin, root: m.store), "the manifest is gone")
        XCTAssertFalse(r.isReady(model: "kk"), "the files are all there and it is still not ready: a re-fetch that fails must not leave it ready")
    }
    func testM5APinChangeIsNotReady() {
        let m = make(), r = pins(m); r.markReady(model: "kk")
        let other = FileVoiceReadiness(file: m.store.appendingPathComponent("ready.json"), pins: { ModelPins.managed($0, spec: m.spec, root: m.store, libraryPin: "fluidaudio-9.9.9") })
        XCTAssertFalse(other.isReady(model: "kk"))
    }
    func testM6HashesAreVerifiedWhenAskedAndCatchAChangeThatKeepsTheLength() {
        let m = make(), r = pins(m); r.markReady(model: "kk")
        let man = ModelManifests.read("kk", pin: kkPin, root: m.store)!
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
        XCTAssertNil(ModelManifests.read("kk", pin: kkPin, root: m.store))
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
    func testM12AFailedCheckAfterAPinChangeKeepsTheFilesAndSaysTheAppWasUpdated() async {
        let m = make(), r = pins(m); r.markReady(model: "kk")
        let moved = FileVoiceReadiness(file: m.store.appendingPathComponent("ready.json"), pins: { ModelPins.managed($0, spec: m.spec, root: m.store, libraryPin: "fluidaudio-9.9.9") })
        let v = ReadinessVerifier(record: moved)
        struct Boom: Error {}
        let res = await v.verify(model: "kk", check: { throw Boom() })
        XCTAssertTrue(res.note.contains("updated"), res.note)
        XCTAssertTrue(FileManager.default.fileExists(atPath: m.base.appendingPathComponent("kokoro").path), "round 8: an app update is not a reason to delete: only digests that differ, or a half pack, delete")
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
        XCTAssertTrue(res.note.contains("intact") && res.note.contains("could not load it now"), res.note); XCTAssertTrue(FileManager.default.fileExists(atPath: m.base.appendingPathComponent("kokoro").path), "intact files are not thrown away")
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
        do { try await ModelFetch.guarded(model: "pk", verifier: v, pins: { ModelPins.managed($0, spec: m.spec, root: m.store) }) { run in run.begin(); try await m.lib.fetch() }; XCTFail("fetched") } catch {}
        XCTAssertFalse(FileManager.default.fileExists(atPath: m.base.appendingPathComponent("pack").path), "the half pack is gone: it must not look complete to a library that only checks presence")
        m.lib.failAfterFiles = nil
        try await ModelFetch.guarded(model: "pk", verifier: v, pins: { ModelPins.managed($0, spec: m.spec, root: m.store) }) { run in run.begin(); try await m.lib.fetch() }
        XCTAssertNoThrow(try m.lib.load(), "the second fetch downloaded everything again")
        let r = await v.verify(model: "pk", check: { try m.lib.load(); return SetupCheck(loadMs: 1, speed: 1, unit: "x", memoryMB: 0, outputOK: true) })
        XCTAssertTrue(r.outputOK); XCTAssertTrue(rec.isReady(model: "pk"))
    }
    func testR2ACancelledFetchPurgesToo() async {
        let m = lib(); let v = ReadinessVerifier(record: FileVoiceReadiness(file: m.store.appendingPathComponent("ready.json"), pins: { ModelPins.managed($0, spec: m.spec, root: m.store) }))
        let t = Task { try await ModelFetch.guarded(model: "pk", verifier: v, pins: { ModelPins.managed($0, spec: m.spec, root: m.store) }) { run in run.begin(); try FileManager.default.createDirectory(at: m.base.appendingPathComponent("pack"), withIntermediateDirectories: true); try await Task.sleep(nanoseconds: 2_000_000_000) } }
        try? await Task.sleep(nanoseconds: 50_000_000); t.cancel(); _ = try? await t.value
        XCTAssertFalse(FileManager.default.fileExists(atPath: m.base.appendingPathComponent("pack").path))
    }
    func testR3AFailedLoadDeletesOnlyWhatADigestProvesDifferent() async throws {
        struct Boom: Error {}
        // no manifest, no baseline: unproven -> deleted
        var m = lib(); var rec = FileVoiceReadiness(file: m.store.appendingPathComponent("ready.json"), pins: { ModelPins.managed($0, spec: m.spec, root: m.store) })
        try await m.lib.fetch()
        var r = await ReadinessVerifier(record: rec).verify(model: "pk", check: { throw Boom() })
        XCTAssertTrue(FileManager.default.fileExists(atPath: m.base.appendingPathComponent("pack").path), "round 8: a fetch that finished and a load that failed once is not a half pack: nothing proves it bad, so it is kept"); XCTAssertTrue(r.note.contains("kept"), r.note)
        // a baseline (the last good manifest, kept across the demotion) that matches: intact -> kept
        m = lib(); rec = FileVoiceReadiness(file: m.store.appendingPathComponent("ready.json"), pins: { ModelPins.managed($0, spec: m.spec, root: m.store) })
        try await m.lib.fetch(); rec.markReady(model: "pk"); rec.markNotReady(model: "pk")                     // sealed once, then demoted by a fetch
        XCTAssertNotNil(ModelManifests.baseline("pk", pin: pkPin, root: m.store), "the last good manifest stays as the baseline")
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

    // ---- Round 8: ONE deletion rule. Files are deleted only when (a) a digest read to the end proves them different from the sealed manifest or baseline, or (b) nothing ever sealed them and the fetch that wrote them did not finish.
    /// A model sealed once (so a baseline exists after a fetch demotes it), with its record.
    func sealed() async throws -> (m: (base: URL, store: URL, spec: ManagedModelSpec, lib: PresenceOnlyLibrary), rec: FileVoiceReadiness, v: ReadinessVerifier, pins: (String) -> ModelPin?) {
        let m = lib(); try await m.lib.fetch()
        let pins: (String) -> ModelPin? = { ModelPins.managed($0, spec: m.spec, root: m.store) }
        let rec = FileVoiceReadiness(file: m.store.appendingPathComponent("ready.json"), pins: pins); rec.markReady(model: "pk")
        XCTAssertTrue(rec.isReady(model: "pk")); return (m, rec, ReadinessVerifier(record: rec), pins)
    }
    func packExists(_ m: (base: URL, store: URL, spec: ManagedModelSpec, lib: PresenceOnlyLibrary)) -> Bool { FileManager.default.fileExists(atPath: m.base.appendingPathComponent("pack/a.bin").path) }
    func testD1ARefusalBeforeTheSlotOrAQueueTimeoutDeletesNothing() async throws {            // B1
        let s = try await sealed()
        for e: Error in [DownloadDoor.Refusal.noConsent(group: "voice"), DownloadDoor.Refusal.notOffered(model: "pk"), DownloadDoor.Refusal.pathNotAgreed(group: "voice"), DownloadDoor.Refusal.timedOut, CancellationError()] {
            do { try await ModelFetch.guarded(model: "pk", verifier: s.v, pins: s.pins) { _ in throw e } as Void; XCTFail("ran") } catch {}      // the body never began: nothing was written
            XCTAssertTrue(packExists(s.m), "a ready model survived \(e)"); XCTAssertTrue(s.rec.isReady(model: "pk"), "and is still ready: \(e)")
        }
    }
    func testD2ATransientLoadFailureInTheFetchBodyKeepsAReadyModelWhoseFilesAreProvenIntact() async throws {         // B2
        let s = try await sealed()
        do { try await ModelFetch.guarded(model: "pk", verifier: s.v, pins: s.pins) { run in run.begin(); throw URLError(.cannotLoadFromNetwork) } as Void; XCTFail("ran") } catch {}        // memory pressure, backgrounding: it began, the files were not touched
        XCTAssertTrue(packExists(s.m), "the baseline proves the 425 MB intact: it is not deleted for a load that failed once")
    }
    func testD3AFetchThatBeganAndLeftTheFilesDifferentFromTheSealedOnesDeletesThem() async throws {
        let s = try await sealed()
        do { try await ModelFetch.guarded(model: "pk", verifier: s.v, pins: s.pins) { run in run.begin(); try Data(count: 5).write(to: s.m.base.appendingPathComponent("pack/a.bin")); throw URLError(.networkConnectionLost) } as Void } catch {}
        XCTAssertFalse(packExists(s.m), "(a): a digest proves the files differ from the sealed ones")
    }
    func testD4AStalePinWithAThrownLoadDeletesNothing() async throws {                         // B3
        let s = try await sealed(); struct Boom: Error {}
        let moved = FileVoiceReadiness(file: s.m.store.appendingPathComponent("ready.json"), pins: { ModelPins.managed($0, spec: s.m.spec, root: s.m.store, libraryPin: "fluidaudio-9.9.9") })
        s.rec.markNotReady(model: "pk")                                                         // a fetch demoted it under the old pin: the baseline is the old pin's
        let r = await ReadinessVerifier(record: moved).verify(model: "pk", check: { throw Boom() })
        XCTAssertTrue(packExists(s.m), "an app update is not proof the files are bad"); XCTAssertTrue(r.note.contains("intact") || r.note.contains("kept"), r.note)
        // and with no manifest and no baseline at all (nothing ever sealed them), a failed load of a FINISHED fetch also keeps them
        let m2 = lib(); try await m2.lib.fetch()
        let rec2 = FileVoiceReadiness(file: m2.store.appendingPathComponent("ready.json"), pins: { ModelPins.managed($0, spec: m2.spec, root: m2.store) })
        _ = await ReadinessVerifier(record: rec2).verify(model: "pk", check: { throw Boom() })
        XCTAssertTrue(packExists(m2), "only a fetch that did not finish deletes an unproven set; a check is not a fetch")
    }
    func testD5AHashThatFailsMidReadIsUnreadableNotDifferent() throws {
        struct IO: Error {}
        let d = tmp(); put(d, "f.bin", 3_000_000)                                              // three megabytes: three reads
        let good = ModelManifests.sha256(d.appendingPathComponent("f.bin"))!
        let entry = ModelManifest(pin: "p", files: [.init(path: "f.bin", bytes: 3_000_000, sha256: good)])
        XCTAssertEqual(ModelManifests.check(entry, base: d), .intact)
        final class N: @unchecked Sendable { var n = 0 }; let n = N()
        let failing: (FileHandle) throws -> Data? = { h in n.n += 1; if n.n == 2 { throw IO() }; return try h.read(upToCount: 1 << 20) }
        XCTAssertNil(ModelManifests.sha256(d.appendingPathComponent("f.bin"), read: failing), "a read that fails part-way gives no digest at all, never a partial one")
        n.n = 0
        XCTAssertEqual(ModelManifests.check(entry, base: d, read: failing), .unreadable, "an I/O error is never read as a change")
    }
    func testD6TheOneRuleItself() async {
        let base = tmp(); put(base, "p/a.bin", 10)
        let man = ModelManifests.build(pin: "x", base: base, folders: ["p"])!
        let intact = ModelPin(pin: "x", present: { true }, proof: { ModelManifests.check(man, base: base) }, purge: { try? FileManager.default.removeItem(at: base.appendingPathComponent("p")) })
        let none = ModelPin(pin: "x", present: { true }, proof: { nil }, purge: { try? FileManager.default.removeItem(at: base.appendingPathComponent("p")) })
        var deleted = await ModelDeletion.reconcile(model: "d6", pin: intact, fetch: .unfinished); XCTAssertFalse(deleted, "proven intact: kept whatever the fetch did")
        deleted = await ModelDeletion.reconcile(model: "d6", pin: none, fetch: .notStarted); XCTAssertFalse(deleted, "a fetch that never began wrote nothing")
        deleted = await ModelDeletion.reconcile(model: "d6", pin: none, fetch: .finished); XCTAssertFalse(deleted, "a finished fetch with no proof is not a half pack")
        XCTAssertTrue(FileManager.default.fileExists(atPath: base.appendingPathComponent("p/a.bin").path))
        deleted = await ModelDeletion.reconcile(model: "d6", pin: none, fetch: .unfinished); XCTAssertTrue(deleted, "(b) the half pack")
    }

    // ---- Round 9: records are per PIN; nothing is compared across versions; a fetch under a new pin starts clean; "unfinished" is evidence.
    func newPinPins(_ m: (base: URL, store: URL, spec: ManagedModelSpec, lib: PresenceOnlyLibrary)) -> (String) -> ModelPin? { { ModelPins.managed($0, spec: m.spec, root: m.store, libraryPin: "fluidaudio-9.9.9") } }
    func testP1AFreshDownloadUnderANewPinIsNotComparedWithTheOldPinsBaselineAndEndsReady() async throws {      // F1
        let s = try await sealed()                                                          // the old voice, sealed under the old pin
        let newPins = newPinPins(s.m); let rec = FileVoiceReadiness(file: s.m.store.appendingPathComponent("ready.json"), pins: newPins); let v = ReadinessVerifier(record: rec)
        XCTAssertFalse(rec.isReady(model: "pk"), "a pin bump: not ready under the new pin")
        // the person downloads: the library writes DIFFERENT bytes (a new version), the check passes
        try await ModelFetch.guarded(model: "pk", verifier: v, pins: newPins) { run in run.begin(); try FileManager.default.createDirectory(at: s.m.base.appendingPathComponent("pack"), withIntermediateDirectories: true); try Data(count: 7).write(to: s.m.base.appendingPathComponent("pack/a.bin")); try Data(count: 9).write(to: s.m.base.appendingPathComponent("pack/b.bin")); try Data(count: 11).write(to: s.m.base.appendingPathComponent("pack/weights.bin")) }
        let r = await v.verify(model: "pk", check: { SetupCheck(loadMs: 1, speed: 1, unit: "x", memoryMB: 0, outputOK: true) })
        XCTAssertTrue(r.outputOK); XCTAssertTrue(rec.isReady(model: "pk"), "the download ends ready")
        // and a failed load now (new pin, sealed): the new files are judged by the NEW pin's seal only
        struct Boom: Error {}
        let r2 = await ReadinessVerifier(record: rec).verify(model: "pk", check: { throw Boom() })
        XCTAssertTrue(FileManager.default.fileExists(atPath: s.m.base.appendingPathComponent("pack/a.bin").path), "never 'damaged' against the old version's baseline"); XCTAssertFalse(r2.note.contains("damaged"), r2.note)
    }
    func testP2AFetchUnderANewPinClearsTheOldVersionFirst_NotKeptForever() async throws {                          // F2
        let s = try await sealed(); let newPins = newPinPins(s.m)
        XCTAssertTrue(packExists(s.m)); XCTAssertFalse(ModelManifests.records("pk", root: s.m.store).isEmpty)
        let v = ReadinessVerifier(record: FileVoiceReadiness(file: s.m.store.appendingPathComponent("ready.json"), pins: newPins))
        _ = try? await ModelFetch.guarded(model: "pk", verifier: v, pins: newPins) { run in run.begin(); XCTAssertFalse(self.packExists(s.m), "the old version's files were removed before the new body wrote"); throw CancellationError() } as Void
        XCTAssertTrue(ModelManifests.records("pk", root: s.m.store).filter { $0.pin != "fluidaudio-9.9.9|pk" }.isEmpty, "and its records")
    }
    func testP3ARefusalThatNeverBeganDoesNotDeleteFilesThatWereAlreadyDifferent() async throws {                 // F1, second trigger
        let s = try await sealed()
        try Data(count: 5).write(to: s.m.base.appendingPathComponent("pack/a.bin"))                  // a crash mid-write, a cache rewrite: different from the seal
        do { try await ModelFetch.guarded(model: "pk", verifier: s.v, pins: s.pins) { _ in throw DownloadDoor.Refusal.pathNotAgreed(group: "voice") } as Void } catch {}
        XCTAssertTrue(packExists(s.m), "a refusal deletes nothing, whatever the files look like")
    }
    func testP4ACompleteDownloadWhoseWarmLoadThrowsIsKept_AnInterruptedOneIsNot() async throws {                  // F3
        struct CoreMLError: Error {}
        var m = lib(); var rec = FileVoiceReadiness(file: m.store.appendingPathComponent("ready.json"), pins: { ModelPins.managed($0, spec: m.spec, root: m.store) })
        do { try await ModelFetch.guarded(model: "pk", verifier: ReadinessVerifier(record: rec), pins: { ModelPins.managed($0, spec: m.spec, root: m.store) }) { run in run.begin(); try await m.lib.fetch(); throw CoreMLError() } as Void } catch {}
        XCTAssertTrue(FileManager.default.fileExists(atPath: m.base.appendingPathComponent("pack").path), "the download finished and the load failed: the 350 MB is kept")
        m = lib(); rec = FileVoiceReadiness(file: m.store.appendingPathComponent("ready.json"), pins: { ModelPins.managed($0, spec: m.spec, root: m.store) })
        do { try await ModelFetch.guarded(model: "pk", verifier: ReadinessVerifier(record: rec), pins: { ModelPins.managed($0, spec: m.spec, root: m.store) }) { run in run.begin(); m.lib.failAfterFiles = 1; try await m.lib.fetch() } as Void } catch {}
        XCTAssertFalse(FileManager.default.fileExists(atPath: m.base.appendingPathComponent("pack").path), "a transport failure mid-download is a half pack")
        m = lib(); rec = FileVoiceReadiness(file: m.store.appendingPathComponent("ready.json"), pins: { ModelPins.managed($0, spec: m.spec, root: m.store) })
        do { try await ModelFetch.guarded(model: "pk", verifier: ReadinessVerifier(record: rec), pins: { ModelPins.managed($0, spec: m.spec, root: m.store) }) { run in run.begin(); try? FileManager.default.createDirectory(at: m.base.appendingPathComponent("pack"), withIntermediateDirectories: true); run.interrupt(); throw CoreMLError() } as Void } catch {}
        XCTAssertFalse(FileManager.default.fileExists(atPath: m.base.appendingPathComponent("pack").path), "the door killed the window (the registry closed): whatever error the library gave, it did not finish")
    }
    func testP5ARefusalLeavesAReadyModelReady() async throws {                                                      // F5
        let s = try await sealed()
        for e: Error in [DownloadDoor.Refusal.pathNotAgreed(group: "voice"), DownloadDoor.Refusal.timedOut] {
            do { try await ModelFetch.guarded(model: "pk", verifier: s.v, pins: s.pins) { _ in throw e } as Void } catch {}
            XCTAssertTrue(s.rec.isReady(model: "pk"), "a working voice is still the voice after a press that was refused: \(e)")
        }
    }
    func testP6RecordsAreScopedByPin() async throws {
        let s = try await sealed()
        let other = ModelPins.managed("pk", spec: s.m.spec, root: s.m.store, libraryPin: "fluidaudio-9.9.9")!
        XCTAssertNil(other.proof?(), "another pin has no seal: unknown, never damage"); XCTAssertFalse(other.present())
    }

    // ---- Round 9 (2) and round 10: Remove. Driven by "its files are on the phone" (not by ready), for every model with files, except the one in use; run under the door with the in-use set read again inside it.
    func testX1RemoveDeletesAModelsFilesAndRecordsAndItsReadiness_ExceptTheOneInUse() async throws {
        let s = try await sealed()
        var r = await ModelDeletion.remove(model: "pk", pin: s.pins("pk"), inUse: { ["pk"] }, verifier: s.v)
        XCTAssertEqual(r, .inUse); XCTAssertTrue(packExists(s.m)); XCTAssertTrue(s.rec.isReady(model: "pk"))
        r = await ModelDeletion.remove(model: "pk", pin: s.pins("pk"), inUse: { [] }, verifier: s.v)
        XCTAssertEqual(r, .removed); XCTAssertFalse(packExists(s.m)); XCTAssertFalse(s.rec.isReady(model: "pk"))
        XCTAssertTrue(ModelManifests.records("pk", root: s.m.store).isEmpty, "no manifest and no baseline is left to judge a later download by")
        let none = await ModelDeletion.remove(model: "pk", pin: nil, inUse: { [] }, verifier: s.v); XCTAssertEqual(none, .notRemovable)
    }
    func testX1bTheInUseSetIsReadAgainInsideTheDoor() async throws {                    // B2: a setup that finishes between the first look and the delete
        let s = try await sealed()
        final class Calls: @unchecked Sendable { let l = NSLock(); var n = 0; func next() -> Int { l.lock(); defer { l.unlock() }; n += 1; return n } }; let c = Calls()
        let r = await ModelDeletion.remove(model: "pk", pin: s.pins("pk"), inUse: { c.next() >= 2 ? ["pk"] : [] }, verifier: s.v)       // not in use at the first look, in use inside the slot
        XCTAssertEqual(r, .inUse); XCTAssertTrue(packExists(s.m), "the model that became the voice while the press was on its way is not deleted")
        let busy = await ModelDeletion.remove(model: "pk", pin: s.pins("pk"), inUse: { [] }, busy: { true }, verifier: s.v)
        XCTAssertEqual(busy, .busy); XCTAssertTrue(packExists(s.m), "and nothing is removed while a setup is running")
    }
    func testX2WhatIsInUseIsDerivedFromTheRegistryNotAHandList() {
        let reg = StageRegistry()
        reg.register(.voice, id: "kokoro", descriptor: StageDescriptor(displayName: "k", needsModels: ["kokoro-82m-ane"])) { _ in EnergyVAD(StageParams()) }
        reg.register(.system1, id: "llama-qwen3-0.6b", descriptor: StageDescriptor(displayName: "d", needsModels: ["qwen3-0.6b-q8_0"])) { _ in EnergyVAD(StageParams()) }
        reg.register(.transcriber, id: "parakeet-eou", descriptor: StageDescriptor(displayName: "e", needsModels: ["parakeet-eou-120m"])) { _ in EnergyVAD(StageParams()) }
        reg.register(.transcriber, id: "fluid-speech-mini", descriptor: StageDescriptor(displayName: "f", needsModels: ["fluid-speech-mini"])) { _ in EnergyVAD(StageParams()) }
        reg.register(.transcriber, id: ListenerPipeline.liveThenFinalStageID, descriptor: StageDescriptor(displayName: "c")) { _ in EnergyVAD(StageParams()) }          // the composite needs none itself
        var c = LabConfiguration(name: "t")
        c.set(.voice, StageSetting(id: "kokoro", params: ["voice": .string("a-voice-nobody-lists")]))                      // a saved voice that is not one of the catalog's few cards
        c.set(.system1, StageSetting(id: "llama-qwen3-0.6b"))
        c.set(.transcriber, StageSetting(id: ListenerPipeline.liveThenFinalStageID, params: ["live": .string("parakeet-eou"), "final": .string("fluid-speech-mini")]))
        let ready = MemoryVoiceReadiness(); for id in ["kokoro-82m-ane", "parakeet-eou-120m"] { ready.markReady(model: id) }           // library models count once they are ready (round 11)
        let ids = ModelInUse.ids(config: c, registry: reg, responder: "lfm3-1.7b-q5_k_m", readiness: ready)
        XCTAssertTrue(ids.isSuperset(of: ["kokoro-82m-ane", "qwen3-0.6b-q8_0", "parakeet-eou-120m", "fluid-speech-mini", "lfm3-1.7b-q5_k_m"]), "\(ids)")
        XCTAssertFalse(ids.contains("pocket-tts"))
    }
    func testX3AnOnDiskModelOffersRemoveWhetherOrNotItIsReady() {
        let phone = DeviceProfile(chip: "A", memoryGB: 8, freeDiskGB: 50, onWiFi: true)
        let onDisk: (String) -> Bool = { ["fluid-speech-mini", VoiceCatalog.pocketModel, KnownModels.embeddingGemma2.id].contains($0) }
        let jobs = ModelOffer.jobs(profile: phone, ready: ["transcriber": "fluid-speech-mini"], inUse: ["fluid-speech-mini"], onDisk: onDisk)
        let f = jobs.first { $0.job == "transcriber" }!.options.first { $0.id == "fluid-speech-mini" }!
        XCTAssertFalse(f.removable, "in use"); XCTAssertEqual(f.dict["removable"] as? Bool, false)
        let p = jobs.first { $0.job == "voice" }!.options.first { $0.id == VoiceCatalog.pocketModel }!
        XCTAssertFalse(p.ready); XCTAssertTrue(p.removable, "not ready (a download that never finished), and its files are on the phone: it can be removed")
        let k = jobs.first { $0.job == "voice" }!.options.first { $0.id == VoiceCatalog.kokoroModel }!; XCTAssertFalse(k.removable, "nothing on disk")
        let search = jobs.first { $0.job == "search" }!.options[0]; XCTAssertTrue(search.removable, "the search model too")
        XCTAssertFalse(jobs.first { $0.job == "speaker" }!.options[0].removable)
    }
    // ---- Round 10 (B1): a download killed with the app leaves a partial set no library will complete. A durable per-pin PENDING record, written when the fetch begins and cleared when it returns, makes the next fetch start clean.
    func testY1APartialSetLeftByAKilledDownloadIsCleanedByTheNextFetchAndThenDownloadsInFull() async throws {
        let m = lib(); let pins: (String) -> ModelPin? = { ModelPins.managed($0, spec: m.spec, root: m.store) }
        let rec = FileVoiceReadiness(file: m.store.appendingPathComponent("ready.json"), pins: pins); let v = ReadinessVerifier(record: rec)
        // the first fetch: it begins (pending is written), writes part of the pack, and the process dies: nothing after the write runs
        let p = pins("pk")!; p.pendingMark?()
        try FileManager.default.createDirectory(at: m.base.appendingPathComponent("pack"), withIntermediateDirectories: true); try Data(count: 100).write(to: m.base.appendingPathComponent("pack/a.bin"))
        XCTAssertTrue(p.hasPending?() == true)
        XCTAssertThrowsError(try m.lib.load(), "the half pack does not load, and a presence-only library would skip it")
        try await m.lib.fetch(); XCTAssertEqual(m.lib.fetches, 1); XCTAssertThrowsError(try m.lib.load(), "(the library's own fetch skipped the half pack)")
        // the relaunch: the next fetch finds the pending record and starts clean
        try await ModelFetch.guarded(model: "pk", verifier: v, pins: pins) { run in run.begin(); try await m.lib.fetch() }
        XCTAssertNoThrow(try m.lib.load(), "cleaned, then the library downloaded everything"); XCTAssertFalse(p.hasPending?() == true, "and the record is cleared when the fetch returned")
        let r = await v.verify(model: "pk", check: { try m.lib.load(); return SetupCheck(loadMs: 1, speed: 1, unit: "x", memoryMB: 0, outputOK: true) }); XCTAssertTrue(r.outputOK); XCTAssertTrue(rec.isReady(model: "pk"))
    }
    func testY2PendingIsClearedWhateverTheFetchDoes_AndSurvivesOnlyAProcessDeath() async throws {
        struct Boom: Error {}
        let m = lib(); let pins: (String) -> ModelPin? = { ModelPins.managed($0, spec: m.spec, root: m.store) }; let v = ReadinessVerifier(record: FileVoiceReadiness(file: m.store.appendingPathComponent("ready.json"), pins: pins))
        for e: Error? in [nil, Boom(), URLError(.notConnectedToInternet), CancellationError()] {
            do { try await ModelFetch.guarded(model: "pk", verifier: v, pins: pins) { run in run.begin(); XCTAssertTrue(pins("pk")!.hasPending?() == true, "written when it began"); if let e { throw e } } as Void } catch {}
            XCTAssertFalse(pins("pk")!.hasPending?() == true, "cleared once the body has returned, by any road: only a death mid-body leaves it")
        }
        do { try await ModelFetch.guarded(model: "pk", verifier: v, pins: pins) { _ in throw DownloadDoor.Refusal.timedOut } as Void } catch {}
        XCTAssertFalse(pins("pk")!.hasPending?() == true, "a fetch that never began wrote none")
    }

    // ---- Round 11
    func testZ1AConfiguredButUnreadyLibraryVoiceIsNotInUse_ButTheResponderFileStaysProtected() {         // D1
        let reg = StageRegistry()
        reg.register(.voice, id: "kokoro", descriptor: StageDescriptor(displayName: "k", needsModels: [VoiceCatalog.kokoroModel])) { _ in EnergyVAD(StageParams()) }
        reg.register(.system1, id: "llama-qwen3-0.6b", descriptor: StageDescriptor(displayName: "d", needsModels: ["qwen3-0.6b-q8_0"])) { _ in EnergyVAD(StageParams()) }
        var c = LabConfiguration(name: "t"); c.set(.voice, StageSetting(id: "kokoro")); c.set(.system1, StageSetting(id: "llama-qwen3-0.6b"))
        let none = MemoryVoiceReadiness()
        var ids = ModelInUse.ids(config: c, registry: reg, responder: "lfm3-1.7b-q5_k_m", readiness: none)
        XCTAssertFalse(ids.contains(VoiceCatalog.kokoroModel), "the radio is speaking with the phone's voice: a kept half pack of the voice the person tapped can be removed")
        XCTAssertTrue(ids.contains("qwen3-0.6b-q8_0") && ids.contains("lfm3-1.7b-q5_k_m"), "the files the answering stages load whether or not they passed a check stay protected")
        none.markReady(model: VoiceCatalog.kokoroModel)
        ids = ModelInUse.ids(config: c, registry: reg, responder: nil, readiness: none)
        XCTAssertTrue(ids.contains(VoiceCatalog.kokoroModel), "once it is ready and configured, it is the voice")
    }
    func testZ2APendingRecordSurvivesAPinBump_AKilledDownloadThenAnUpdateIsCleanedByTheRetry() async throws {   // D2
        let m = lib()
        let pinA: (String) -> ModelPin? = { ModelPins.managed($0, spec: m.spec, root: m.store, libraryPin: "fluidaudio-1.0.0") }
        let pinB: (String) -> ModelPin? = { ModelPins.managed($0, spec: m.spec, root: m.store, libraryPin: "fluidaudio-2.0.0") }
        pinA("pk")!.pendingMark?()                                                           // killed with the app during the first download
        try FileManager.default.createDirectory(at: m.base.appendingPathComponent("pack"), withIntermediateDirectories: true); try Data(count: 50).write(to: m.base.appendingPathComponent("pack/a.bin"))
        XCTAssertTrue(pinB("pk")!.hasPending?() == true, "the record is about the model's files, not about a version: an update does not hide it")
        let v = ReadinessVerifier(record: FileVoiceReadiness(file: m.store.appendingPathComponent("ready.json"), pins: pinB))
        try await ModelFetch.guarded(model: "pk", verifier: v, pins: pinB) { run in run.begin(); try await m.lib.fetch() }
        XCTAssertNoThrow(try m.lib.load(), "cleaned, then downloaded in full")
    }
    func testZ3OnDiskIsANonEmptyFolderCheck() throws {                                                       // D6
        let m = lib(); let p = ModelPins.managed("pk", spec: m.spec, root: m.store)!
        XCTAssertFalse(p.onDisk!()); try FileManager.default.createDirectory(at: m.base.appendingPathComponent("pack"), withIntermediateDirectories: true); XCTAssertFalse(p.onDisk!(), "an empty folder is nothing")
        try Data(count: 3).write(to: m.base.appendingPathComponent("pack/x")); XCTAssertTrue(p.onDisk!())
    }
}

