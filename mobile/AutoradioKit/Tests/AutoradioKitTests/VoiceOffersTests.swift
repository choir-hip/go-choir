import XCTest
@testable import AutoradioKit

// Phonon's voices are reachable, and Kokoro's offered size is what lands on the phone. Failure list (each line a test):
//  P1 Phonon is in the voice download group: the door knows its group, so its cards can be agreed to and fetched from Settings > Downloads (before, no group held it and its 25 cards were never shown).
//  P2 Phonon is an option of the voice job on the Downloads page, with its credit, but never the recommendation and withheld until its pack is sealed by the cold load: it is not in the first run's total (the withholding rule is unchanged).
//  P3 Kokoro's offered size counts the voice files its first load fetches (every Kokoro voice of the catalog, from the pack's listing), and the progress bar's denominator is the same number.
//  P4 Kokoro and Phonon both carry their credit on the Downloads page.
final class VoiceOffersTests: XCTestCase {
    let phone = DeviceProfile(chip: "iPhone15,2", memoryGB: 5.6, freeDiskGB: 40, onWiFi: true)

    func testP1PhononIsInTheVoiceGroup() {
        XCTAssertEqual(DownloadDoor(onWiFi: { true }).group(ofModel: VoiceCatalog.phononModel), "voice")
        let offered = VoiceCatalog.offered(by: DownloadDoor(onWiFi: { true }))
        XCTAssertEqual(offered.filter { $0.modelID == VoiceCatalog.phononModel }.count, 25)
    }
    func testP2PhononIsOfferedOnTheDownloadsPageButNeverFirstRun() throws {
        let v = try XCTUnwrap(ModelOffer.jobs(profile: phone).first { $0.job == "voice" })
        let p = try XCTUnwrap(v.options.first { $0.id == VoiceCatalog.phononModel })
        XCTAssertTrue(p.withheld, "a library model with no local-only load is withheld until its manifest exists")
        XCTAssertFalse(p.recommended)
        XCTAssertTrue(v.options.first { $0.id == VoiceCatalog.kokoroModel }!.withheld, "the same rule as Kokoro's: a library voice is fetched from Settings, and sealed before it is recommended")
        XCTAssertEqual(p.bytes, 173_000_000)
        // once Kokoro is sealed it is the recommendation and Phonon still is not (its rank is below Kokoro's): the first run never fetches Phonon
        let ready = ModelOffer.jobs(profile: phone, ready: ["voice": VoiceCatalog.kokoroModel])
        XCTAssertEqual(ready.first { $0.job == "voice" }!.options.first { $0.recommended }?.id, VoiceCatalog.kokoroModel)
        XCTAssertFalse(ready.first { $0.job == "voice" }!.options.first { $0.id == VoiceCatalog.phononModel }!.recommended)
    }
    func testP3KokorosOfferedSizeCountsItsVoiceFiles() throws {
        let voices = VoiceRows.all.filter { $0.modelID == VoiceCatalog.kokoroModel }.reduce(Int64(0)) { $0 + $1.bytes }
        XCTAssertGreaterThan(voices, 70_000_000); XCTAssertLessThan(voices, 80_000_000)
        let k = try XCTUnwrap(ModelOffer.jobs(profile: phone).first { $0.job == "voice" }?.options.first { $0.id == VoiceCatalog.kokoroModel })
        XCTAssertEqual(k.bytes, 350_000_000 + voices)
        XCTAssertEqual(KnownModels.managed.first { $0.id == VoiceCatalog.kokoroModel }?.approxBytes, 350_000_000 + voices)
        XCTAssertEqual(CandidateTable.candidate(VoiceCatalog.kokoroModel, .voice)?.bytes, 350_000_000 + voices)
    }
    func testP4BothVoiceModelsCarryTheirCreditOnTheDownloadsPage() throws {
        let v = try XCTUnwrap(ModelOffer.jobs(profile: phone).first { $0.job == "voice" })
        for id in [VoiceCatalog.kokoroModel, VoiceCatalog.phononModel] {
            let o = try XCTUnwrap(v.options.first { $0.id == id })
            XCTAssertTrue(o.credit.contains("Apache-2.0"), "\(id): \(o.credit)")
        }
        XCTAssertEqual(v.options.first { $0.id == VoiceCatalog.phononModel }?.credit, VoiceRows.all.first { $0.modelID == VoiceCatalog.phononModel }?.credit)
    }
}
