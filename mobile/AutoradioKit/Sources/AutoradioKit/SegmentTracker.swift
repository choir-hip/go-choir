import Foundation

// Where playback is, in the vocabulary of the learning engine (design 12.3): which segment, which server block began, how it ended.
// Station P2 turns these into the events it sends and buffers; P1 only observes. The failure list is in mobile/README.md (F17, F18);
// the tests are SegmentTrackerTests and SegmentPlayerTests in ProgrammePlayerTests.swift.
//
// The tracker is pure: the player tells it when a playback block began, when one ended (and whether audio really played), when the
// listener paused, when the engine yielded to a turn, and when the document was replaced. It decides what each of those means for the
// segment that is open. It never reads the audio and never touches the network.

public enum PlaybackKind: String, Sendable {
    /// The first block of a segment began to play (audibly: not while the listener has paused).
    case segmentBegan
    /// A server block began that had not begun before in this segment. `lastBlock` is its index. Not sent once per playback block.
    case blockBegan
    /// The last playback block of the segment played to its end. `lastBlock` is the last server block that began.
    case finished
    /// The listener went forward to another segment before this one ended.
    case skipped
    /// The listener went back to an earlier segment (or another document) before this one ended.
    case rewound
    /// The document was replaced while this segment was open (the listener asked for something else).
    case superseded
    /// A block of the segment could not be played (no voice, a failure): not a finish.
    case failed
    case paused, resumed
    /// The engine paused the programme for a turn, and gave it back.
    case cut, cutEnded
}

public struct PlaybackEvent: Sendable, Equatable {
    public var kind: PlaybackKind
    public var documentID: String
    public var stretchID: String
    public var segmentID: String
    /// The last server block that began playing, from 0.
    public var lastBlock: Int
    /// The server's block count for the segment.
    public var blockCount: Int
    public var at: Date
}

public final class SegmentTracker: @unchecked Sendable {
    private struct Open {
        var segment: ProgrammeSegment
        var docID: String
        var index: Int
        var lastBlock = -1            // announced
        var pending = -1              // begun, not announced yet (the listener is paused)
        var announced = false
        var pausedReported = false
        var cutReported = false
    }
    private let lock = NSLock()
    private var open: Open?
    private var userPaused = false
    private var observers: [@Sendable (PlaybackEvent) -> Void] = []
    private let now: @Sendable () -> Date

    public init(now: @escaping @Sendable () -> Date = { Date() }) { self.now = now }

    public func observe(_ f: @escaping @Sendable (PlaybackEvent) -> Void) { lock.lock(); observers.append(f); lock.unlock() }

    private func event(_ k: PlaybackKind, _ o: Open) -> PlaybackEvent {
        PlaybackEvent(kind: k, documentID: o.docID, stretchID: o.segment.stretchID, segmentID: o.segment.id,
                      lastBlock: max(o.lastBlock, 0), blockCount: o.segment.serverBlockCount, at: now())
    }
    private func deliver(_ events: [PlaybackEvent]) {
        guard !events.isEmpty else { return }
        lock.lock(); let obs = observers; lock.unlock()
        for e in events { for f in obs { f(e) } }
    }

    /// Tell the observers of what began while paused, now that the listener plays again.
    private func announce(_ o: inout Open, into out: inout [PlaybackEvent]) {
        if !o.announced { o.announced = true; o.lastBlock = o.pending; out.append(event(.segmentBegan, o)); out.append(event(.blockBegan, o)); return }
        if o.pending > o.lastBlock { o.lastBlock = o.pending; out.append(event(.blockBegan, o)) }
    }
    private func close(_ kind: PlaybackKind, into out: inout [PlaybackEvent]) {
        if let o = open, o.announced { out.append(event(kind, o)) }
        open = nil
    }

    /// Playback block `unit` of `doc` began.
    public func unitBegan(_ unit: Int, in doc: ProgrammeDocument) {
        var out: [PlaybackEvent] = []
        lock.lock()
        if let si = doc.segmentIndex(containingBlock: unit) {
            let seg = doc.segments[si]
            if let o = open, o.segment.id != seg.id || o.docID != doc.id {
                close(o.docID != doc.id ? .superseded : (si > o.index ? .skipped : .rewound), into: &out)
            }
            var o = open ?? Open(segment: seg, docID: doc.id, index: si)
            let local = unit - seg.firstBlock
            let block = seg.unitServerBlocks.indices.contains(local) ? seg.unitServerBlocks[local] : 0
            o.pending = max(o.pending, block)
            if !userPaused { announce(&o, into: &out) }
            open = o
        } else if open != nil {
            close(.superseded, into: &out)                                     // moved to a block that belongs to no segment
        }
        lock.unlock()
        deliver(out)
    }

    /// Playback block `unit` ended. `played` is false when it made no sound (no voice, a failure).
    public func unitEnded(_ unit: Int, played: Bool, in doc: ProgrammeDocument) {
        var out: [PlaybackEvent] = []
        lock.lock()
        if let o = open, o.docID == doc.id, let si = doc.segmentIndex(containingBlock: unit), doc.segments[si].id == o.segment.id {
            if !played { close(.failed, into: &out) }
            else if unit == o.segment.firstBlock + o.segment.blockCount - 1 {
                var m = o
                if !userPaused { announce(&m, into: &out) }
                open = m
                close(.finished, into: &out)
            }
        }
        lock.unlock()
        deliver(out)
    }

    /// The listener paused (true) or played (false).
    public func paused(_ on: Bool) {
        var out: [PlaybackEvent] = []
        lock.lock()
        userPaused = on
        if var o = open {
            if on {
                if o.announced, !o.pausedReported { o.pausedReported = true; out.append(event(.paused, o)) }
            } else {
                if o.announced, o.pausedReported { o.pausedReported = false; out.append(event(.resumed, o)) }
                announce(&o, into: &out)
            }
            open = o
        }
        lock.unlock()
        deliver(out)
    }

    /// The engine paused the programme for the listener's turn (true) and gave it back (false).
    public func cut(_ on: Bool) {
        var out: [PlaybackEvent] = []
        lock.lock()
        if var o = open, o.announced, o.cutReported != on { o.cutReported = on; out.append(event(on ? .cut : .cutEnded, o)); open = o }
        lock.unlock()
        deliver(out)
    }

    /// The player was given another document.
    public func documentReplaced() {
        var out: [PlaybackEvent] = []
        lock.lock(); close(.superseded, into: &out); lock.unlock()
        deliver(out)
    }
}
