import Foundation

// Where the programme is, taken from the audio that was actually rendered, and what was heard before a cut.
//
// Ways this can fail (written before the code; tests in PlayheadTests.swift):
//  1. The position is estimated from wall time: it drifts at any rate other than 1x and across pause and skip. It is
//     computed from the player node's render position (source samples, upstream of the time-pitch unit, so it does not
//     depend on the rate), relative to where the node stood when this block was scheduled.
//  2. The node keeps counting across blocks (it is not stopped between them): the block's position must be
//     (rendered - rendered-at-schedule), never the raw node time.
//  3. The node stops advancing while paused: the position must hold, not creep, not reset.
//  4. The engine restarts mid-block (route change, call): the node's clock goes back to zero. The position is frozen at
//     the moment of the stop and playing resumes from there; a position read while the engine is down is the frozen one.
//  5. The position runs past the block (a late completion): it clamps to the block's length; a fraction never exceeds 1.
//  6. A negative or garbage render value (nil render time reads as 0 while the node is stopped): clamps at the start.
//  7. A skip leaves the old block's position behind: the player clears it when a block starts, and a reader asks for a
//     specific block, so another block's position never answers.
//  8. The cut text: fraction 0 is nothing spoken, 1 is everything, NaN/negative/over-one clamp; a word is never cut in the
//     middle (a half-heard word was not heard); no leading or trailing blanks; emoji and accents stay whole.
//  9. The listener cut into something that was not speech (a clip, a paused programme, an acknowledgement): there is
//     no cut, `DeskTurn.interrupted` stays false.

/// A position inside one scheduled block of programme audio. Frames are at `LiveAudio.playRate`.
struct Playhead: Equatable, Sendable {
    var tag: Int
    var totalFrames: Int
    /// Frames into the block at the moment of the last (re)schedule.
    var from = 0
    /// The node's render position at that moment.
    var base = 0
    /// While set, the position is this (engine stopped, or the virtual engine's own count).
    var frozen: Int?

    init(tag: Int, totalFrames: Int) { self.tag = tag; self.totalFrames = max(0, totalFrames) }

    func frames(rendered: Int) -> Int {
        let raw = frozen ?? (from + max(0, rendered - base))
        return min(totalFrames, max(0, raw))
    }
    func seconds(rendered: Int, rate: Double = LiveAudio.playRate) -> Double { Double(frames(rendered: rendered)) / rate }
    var totalSeconds: Double { Double(totalFrames) / LiveAudio.playRate }
    func fraction(rendered: Int) -> Double { totalFrames == 0 ? 0 : min(1, Double(frames(rendered: rendered)) / Double(totalFrames)) }
    mutating func freeze(rendered: Int) { frozen = frames(rendered: rendered) }
    mutating func reschedule(from f: Int, base b: Int) { from = max(0, min(f, totalFrames)); base = b; frozen = nil }
}

/// What the listener had heard of the line they cut into.
public struct SpokenCut: Codable, Sendable, Equatable {
    public var blockIndex: Int
    /// The whole line.
    public var full: String
    /// The words actually spoken before the cut (whole words only).
    public var spoken: String
    /// 0...1 of the line's audio that had played.
    public var fraction: Double
    public init(blockIndex: Int, full: String, spoken: String, fraction: Double) {
        self.blockIndex = blockIndex; self.full = full; self.spoken = spoken; self.fraction = fraction
    }
}

/// The player's position in the block it is playing, from rendered audio.
public struct RenderPosition: Sendable, Equatable {
    public var index: Int
    public var fraction: Double
    public var seconds: Double
    public var total: Double
    public init(index: Int, fraction: Double, seconds: Double, total: Double) {
        self.index = index; self.fraction = fraction; self.seconds = seconds; self.total = total
    }
}

public enum SpeechCut {
    /// The text of `full` that `fraction` of its audio covers, to a word boundary.
    public static func text(of full: String, upTo fraction: Double) -> String {
        let f = fraction.isNaN ? 0 : min(1, max(0, fraction))
        let chars = Array(full)
        if f >= 1 { return full.trimmingCharacters(in: .whitespacesAndNewlines) }
        var end = Int((Double(chars.count) * f).rounded(.down))
        if end <= 0 { return "" }
        if end < chars.count, !chars[end].isWhitespace, !chars[end - 1].isWhitespace {
            while end > 0, !chars[end - 1].isWhitespace { end -= 1 }   // inside a word: that word was not finished
        }
        return String(chars[0..<end]).trimmingCharacters(in: .whitespacesAndNewlines)
    }
}
