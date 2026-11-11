import Foundation

// Speaks the phone's own stream sentence by sentence and can be cut at a sentence boundary.
//
// WAYS THIS CAN FAIL (each has a test in LocalAckTests):
//  1. The desk speaks (or the listener, or System 1 says nothing was asked) while a sentence is being heard: that sentence is
//     heard to its end (never cut mid-word); the sentences queued behind it are dropped; none starts after the cut.
//  2. The cut arrives between sentences: nothing more is played at once.
//  3. A sentence's synthesis fails: the sentence is skipped and the stream goes on; the failure is on the log.
//  4. Synthesis of sentence n+1 is slower than the playing of sentence n: there is a gap, never an overlap (sentences play in order,
//     and synthesis of the next one runs while the current one is heard).
//  5. What was heard is recorded when a sentence starts to be heard (`heard`), so the desk is told what the listener actually heard.

struct SpokenSentence: Sendable { var text: String; var frames: [AudioFrame] }

final class LocalSpeaker: @unchecked Sendable {
    let voice: Voice, playback: PlaybackControl, log: EventLog, endedAt: Double, heard: HeardLines
    /// Set to cut: the sentence being heard finishes, nothing after it starts.
    let cut: Flag
    /// A speaker of the phone's own stream (`local`) or of the streamed desk answer (events and the first-sound timing differ).
    let local: Bool
    private let count = Locked<Int>(0)
    init(voice: Voice, playback: PlaybackControl, log: EventLog, endedAt: Double, heard: HeardLines, cut: Flag, local: Bool = true) {
        self.voice = voice; self.playback = playback; self.log = log; self.endedAt = endedAt; self.heard = heard; self.cut = cut; self.local = local
    }
    var sentencesHeard: Int { count.value }

    static func stream(_ frames: [AudioFrame]) -> AsyncThrowingStream<AudioFrame, Error> {
        AsyncThrowingStream { c in for f in frames { c.yield(f) }; c.finish() }
    }

    /// Play the "heard you" cue, then every sentence from `sentences` as it arrives, until the stream ends or `cut` is set.
    /// `before` runs after synthesis has started and before anything is played (the desk speaker waits there for the local stream to
    /// finish its sentence, then plays the hand-over sound), so the first desk sentence is already synthesised when its turn comes.
    func run(cue: [AudioFrame], sentences: AsyncStream<String>, before: (() async -> Void)? = nil) async {
        let (bufOut, bufCont) = AsyncStream.makeStream(of: SpokenSentence.self)
        let (voice, log) = (voice, log)
        let cut = self.cut
        let synth = Task {
            for await s in sentences {
                if cut.isSet || Task.isCancelled { break }
                var frames: [AudioFrame] = []
                let one = AsyncStream<String> { $0.yield(s); $0.finish() }
                do { for try await f in voice.speak(one) { frames.append(f) } }
                catch { log.emit(.error(stage: "voice", message: "\(error)")); continue }       // failure 3
                if !frames.isEmpty { bufCont.yield(SpokenSentence(text: s, frames: frames)) }
            }
            bufCont.finish()
        }
        let started = log.clock.now
        await before?()
        if !cue.isEmpty { await playback.speak(Self.stream(cue)) }
        var firstSentence = true
        for await sp in bufOut {
            if cut.isSet || Task.isCancelled { break }                                           // failures 1, 2
            count.write { $0 += 1 }
            heard.add(sp.text)
            if local { log.emit(.localSentenceStarted(text: sp.text, msSinceTurnEnd: (log.clock.now - endedAt) * 1000)) }
            else { log.emit(.deskSentenceStarted(text: sp.text, msSinceTurnEnd: (log.clock.now - endedAt) * 1000)) }
            if firstSentence, local {
                firstSentence = false
                log.emit(.stageTiming(stage: "voice.firstAudio", startedAt: started, endedAt: log.clock.now))
            }
            await playback.speak(Self.stream(sp.frames))
        }
        synth.cancel()
    }
}
