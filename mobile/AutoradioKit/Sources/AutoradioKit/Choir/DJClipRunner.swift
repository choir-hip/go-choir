import Foundation

// Playing a `podcast_clip` segment, and giving the queue's items their turn between segments (lane autoradio-dj-clips).
//
// The programme's loop speaks the DJ's lead-in as ordinary speech blocks, then reaches the clip block and awaits `playClip`. The clip
// plays through the shared queue (`PlayItem.audio`), so the queue's player shows its show and episode, "Keep listening" works, and
// whatever the reader queued by hand plays after it. The loop stays parked until the queue goes idle (nothing queued behind what ended):
// the queue then calls its `defaultSource`, which is `queueIdle()` here, and the programme goes on with the next segment.

public enum ClipOutcome: Sendable, Equatable { case played, unknown, failed }

@MainActor public final class DJClipRunner {
    public static let shared = DJClipRunner()
    public let queue: PlayQueue
    let store: ClipStore
    /// How long a clip's audio may take to start before the DJ gives up on it.
    public var startDeadline: TimeInterval = 6
    /// What the queue does when it is idle and the programme is not waiting (the end of a hand-played item continues into the DJ).
    public var whenIdle: (() async -> Bool)?

    private var waiting: CheckedContinuation<Void, Never>?
    private var expecting = false
    private var idleSeen = false
    private var failed = false
    private var abandonedUntil = Date.distantPast

    public init(queue: PlayQueue = .shared, store: ClipStore = .shared) { self.queue = queue; self.store = store }

    /// The programme is parked behind a clip or a queued item.
    public var isParked: Bool { expecting }

    /// Make the queue's idle moment ours.
    public func install() { queue.defaultSource = { [weak self] in await self?.queueIdle() ?? false } }

    /// The queue has nothing left. True: someone took it.
    public func queueIdle() async -> Bool {
        if expecting { idleSeen = true; failed = !queue.message.isEmpty; resume(); return true }
        if Date() < abandonedUntil { return true }               // the late end of a clip the DJ gave up on
        return await whenIdle?() ?? false
    }

    private func resume() { let w = waiting; waiting = nil; w?.resume() }

    private func park() async {
        if idleSeen || Task.isCancelled { return }
        await withTaskCancellationHandler {
            await withCheckedContinuation { (c: CheckedContinuation<Void, Never>) in
                if idleSeen || Task.isCancelled { c.resume() } else { waiting = c }
            }
        } onCancel: { Task { @MainActor in self.cancelled() } }
    }

    /// The loop was cancelled (the reader skipped, picked a story): the clip stops and the queue forgets it.
    private func cancelled() {
        let was = expecting
        expecting = false; resume()
        if was, queue.current?.isClip == true { queue.engine?.stopRemote(); queue.noteStarted(nil) }
    }

    /// The reader's skip while the programme is parked: what is queued by hand next, else the programme's next segment.
    public func skipCurrent() {
        guard expecting else { return }
        if queue.hasUpcoming { queue.skip(); return }
        queue.engine?.stopRemote(); queue.noteStarted(nil)
        idleSeen = true; failed = false; resume()
    }

    /// Before the lead-in is spoken: is this the file the clip was cut from? Its length is read without playing (same deadline as a start).
    /// False: unknown clip, no engine, no answer in time, or a length more than 3 s from the transcribed file's; the segment is skipped
    /// silently and the clip is marked so its "Hear it" row offers the episode only.
    public func preflight(_ id: String) async -> Bool {
        guard let clip = store.clip(id), let engine = queue.engine else { return false }
        guard let d = await engine.remoteDuration(url: clip.audioURL, deadline: startDeadline) else { return false }
        if clip.duration > 0, abs(d - clip.duration) > PodcastClip.durationTolerance { store.markMismatch(id); return false }
        return true
    }

    /// Play the clip and wait until it, and anything the reader queued behind it, is over.
    public func playClip(_ id: String) async -> ClipOutcome {
        guard let clip = store.clip(id) else { return .unknown }
        guard queue.engine != nil else { return .failed }
        expecting = true; idleSeen = false; failed = false
        let item = clip.playItem
        queue.playNow(item)
        let started = OneShot<Bool>()
        Task { @MainActor in await self.queue.settle(); started.fire(true) }
        let timer = Task { try? await Task.sleep(nanoseconds: UInt64(self.startDeadline * 1_000_000_000)); started.fire(false) }
        let ok = await started.wait()
        timer.cancel()
        if Task.isCancelled { cancelled(); return .played }
        if !ok {                                                  // it did not start in time
            abandonedUntil = Date().addingTimeInterval(5)
            queue.engine?.stopRemote(); queue.noteStarted(nil)
            expecting = false
            return .failed
        }
        await park()
        let bad = failed
        expecting = false
        return bad ? .failed : .played
    }

    /// At a segment boundary: what the reader queued by hand plays before the DJ's next segment. Returns when the queue is idle again.
    public func yieldToQueue() async {
        guard queue.hasUpcoming, queue.engine != nil, !expecting else { return }
        expecting = true; idleSeen = false; failed = false
        queue.skip()
        await queue.settle()
        await park()
        expecting = false
    }
}
