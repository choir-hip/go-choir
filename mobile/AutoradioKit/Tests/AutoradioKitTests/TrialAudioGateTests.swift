import XCTest
@testable import AutoradioKit

// The speaker side of the one gate (docs/problems/model-test-cancellation-2026-11-24.md): a Test whose account left starts no sound. `playVoice` reads the gate before it starts the node and before every buffer.
final class TrialAudioGateTests: XCTestCase {
    final class Count: @unchecked Sendable { let l = NSLock(); var made = 0; var ended = false; func made1() { l.lock(); made += 1; l.unlock() }; func end() { l.lock(); ended = true; l.unlock() } }
    func frames(_ c: Count, _ n: Int) -> AsyncThrowingStream<AudioFrame, Error> {
        AsyncThrowingStream { k in
            let t = Task { for i in 0..<n { c.made1(); k.yield(AudioFrame(samples: [Float](repeating: 0.1, count: 480), sampleRate: 24_000, t: Double(i))) }; k.finish() }
            k.onTermination = { _ in t.cancel(); c.end() }
        }
    }
    func testAVoiceWhoseAccountLeftIsNeverStartedOrConsumed() async {
        let c = Count(); let a = LiveAudio()
        let gate = TrialGate()
        AccountWipe.bumpGeneration()
        await a.playVoice(frames(c, 5), reason: "a test", gate: gate)
        XCTAssertTrue(c.ended, "the frames were let go at once, not played")
    }
    func testALiveGatePlaysAsBefore() async {
        let c = Count(); let a = LiveAudio()
        await a.playVoice(frames(c, 3), reason: "a test", gate: TrialGate())
        XCTAssertEqual(c.made, 3, "every frame was taken")
    }
    func testTheAccountLeavingMidSentenceStopsTakingFrames() async {
        let c = Count(); let a = LiveAudio(); let gate = TrialGate()
        let stream = AsyncThrowingStream<AudioFrame, Error> { k in
            let t = Task { for i in 0..<50 { c.made1(); k.yield(AudioFrame(samples: [Float](repeating: 0.1, count: 480), sampleRate: 24_000, t: Double(i))); if i == 2 { AccountWipe.bumpGeneration() }; try? await Task.sleep(nanoseconds: 2_000_000) }; k.finish() }
            k.onTermination = { _ in t.cancel(); c.end() }
        }
        await a.playVoice(stream, reason: "a test", gate: gate)
        XCTAssertLessThan(c.made, 50, "playback ended when the account left, not at the end of the sentence")
    }
}
