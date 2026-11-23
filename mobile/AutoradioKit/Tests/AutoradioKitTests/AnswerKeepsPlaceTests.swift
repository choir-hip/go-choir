import XCTest
@testable import AutoradioKit

// (The answers-keep-the-place tests were the Swift player's: radio-director.test.ts holds them now. These are the sound paths.)
final class SoundPathsTests: XCTestCase {
    func testEverySoundPathIsKnownAndGated() throws {
        let dir = URL(fileURLWithPath: #filePath).deletingLastPathComponent().deletingLastPathComponent().deletingLastPathComponent()
            .appendingPathComponent("Sources/AutoradioKit")
        let patterns = ["AVAudioPlayerNode(", "AVPlayer(", "AVAudioPlayer(", "AVQueuePlayer(", "AVSpeechSynthesizer(", "AVSpeechSynthesizer.", ".speakUtterance(", "AudioServicesPlaySystemSound"]
        var found: [String: Set<String>] = [:]
        for case let url as URL in FileManager.default.enumerator(at: dir, includingPropertiesForKeys: nil)! where url.pathExtension == "swift" {
            let text = try String(contentsOf: url, encoding: .utf8)
            for line in text.split(separator: "\n") where !line.trimmingCharacters(in: .whitespaces).hasPrefix("//") && !line.trimmingCharacters(in: .whitespaces).hasPrefix("///") {
                for p in patterns where line.contains(p) { found[url.lastPathComponent, default: []].insert(p) }
            }
        }
        // LiveAudio: the programme node and the voice node, both held by the gate (voice line, session switch). RemoteAudioPlayer: AVPlayer,
        // held through `setHeld` (a voice line) and `setLevel` (the listener speaking). AppleStages: the system voice is synthesised to buffers
        // (`write`), which LiveAudio plays; it never calls `speak(_:)` on the synthesizer.
        let known: [String: Set<String>] = [
            "LiveAudio.swift": ["AVAudioPlayerNode("],
            "RemoteAudioPlayer.swift": ["AVPlayer("],
            "AppleStages.swift": ["AVSpeechSynthesizer("],
            // The take's review playback (the listener's own recording): played only while the take holds the gate (the radio is paused, hands-free
            // off, the capture session taken: TakeGate), stopped by Re-record, Discard and Publish.
            "TakeSession.swift": ["AVAudioPlayer("],
        ]
        XCTAssertEqual(found.mapValues { $0 }, known, "a new place that makes sound must go behind the single-voice gate (LiveAudio.holdForSessionSwitch / onVoiceHold) and be named here")
        let apple = try String(contentsOf: dir.appendingPathComponent("AppleStages.swift"), encoding: .utf8)
        XCTAssertFalse(apple.contains(".speak(utterance") || apple.contains("speak(u)"), "the system voice must be written to buffers, never spoken by the synthesizer itself")
    }

    @MainActor func testRemoteAudioIsHeldWithAVoiceLine() async {
        let r = RemoteAudioPlayer()
        r.setHeld(true); XCTAssertTrue(r.isHeld)
        r.setHeld(false); XCTAssertFalse(r.isHeld)
    }

    func testTheSessionSwitchHoldsWhatIsOnAirAndGivesItBack() async {
        let a = virtualAudio()
        a.programmePlay(); XCTAssertTrue(a.programmeAudible)
        a.holdForSessionSwitch(true)
        XCTAssertFalse(a.programmeAudible, "not played through a mode switch")
        a.holdForSessionSwitch(false)
        XCTAssertTrue(a.programmeAudible, "goes on from the held position")
    }
}
