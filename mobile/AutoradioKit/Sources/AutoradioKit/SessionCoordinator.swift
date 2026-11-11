import Foundation

/// A mutex for async code: claims must not overlap (a switch is release, then claim, then engine restart).
final class AsyncMutex: @unchecked Sendable {
    private let l = NSLock()
    private var held = false
    private var waiters: [CheckedContinuation<Void, Never>] = []
    func lock() async {
        await withCheckedContinuation { (c: CheckedContinuation<Void, Never>) in
            l.lock()
            if !held { held = true; l.unlock(); c.resume() } else { waiters.append(c); l.unlock() }
        }
    }
    func unlock() {
        l.lock()
        if waiters.isEmpty { held = false; l.unlock() } else { let c = waiters.removeFirst(); l.unlock(); c.resume() }
    }
}

/// Sits between the turn engine and the host's session owner. It applies the configuration's `AudioPolicy`
/// to every request, stops the engine before the category changes and restarts it after, times the switch,
/// and turns everything the system says (interruption, route change, media-services reset) into events and
/// engine recovery. The engine never sees any of it.
public final class SessionCoordinator: AudioSessionOwner, @unchecked Sendable {
    let inner: AudioSessionOwner
    let log: EventLog
    let audio: LiveAudio
    private let policy: @Sendable () -> AudioPolicy
    private let mutex = AsyncMutex()
    private let lock = NSLock()
    private var _current: AudioState?
    private var _asked: AudioState = .listening
    /// Called after a notice has been handled (the runtime refreshes its snapshot).
    public var onChange: (@Sendable () -> Void)?

    public init(inner: AudioSessionOwner, log: EventLog, audio: LiveAudio, policy: @escaping @Sendable () -> AudioPolicy) {
        self.inner = inner; self.log = log; self.audio = audio; self.policy = policy
        inner.observe { [weak self] kind, detail in self?.heard(kind, detail) }
        audio.onNotice = { [weak self] kind, detail in
            if kind == "error" { self?.log.emit(.error(stage: "audio-engine", message: detail)) }
            else { self?.log.emit(.audioNotice(kind: kind, detail: detail)) }
            self?.onChange?()
        }
    }

    /// The state the session is in now (nil before the first claim and after `release`).
    public var current: AudioState? { lock.lock(); defer { lock.unlock() }; return _current }

    public func claim(_ asked: AudioState, preferBuiltInMic: Bool, reason: String) async throws {
        await mutex.lock(); defer { mutex.unlock() }
        lock.lock(); _asked = asked; lock.unlock()
        try await switchTo(policy().effective(asked), preferBuiltInMic: preferBuiltInMic, reason: reason)
    }

    /// The policy changed (configuration edit): move to the state it now asks for.
    public func reapply() async {
        await mutex.lock(); defer { mutex.unlock() }
        lock.lock(); let asked = _asked, cur = _current; lock.unlock()
        guard cur != nil else { return }
        try? await switchTo(policy().effective(asked), preferBuiltInMic: false, reason: "policy changed")
    }

    private func switchTo(_ state: AudioState, preferBuiltInMic: Bool, reason: String) async throws {
        let from = current
        if from == state { return }
        let t0 = DispatchTime.now().uptimeNanoseconds
        audio.quiesce()                                   // the category change would stop it anyway
        do { try await inner.claim(state, preferBuiltInMic: preferBuiltInMic, reason: reason) }
        catch {
            lock.lock(); _current = nil; lock.unlock()
            try? audio.resume(voiceProcessing: from == .talking)   // put the engine back as it was, if it can be
            throw error
        }
        lock.lock(); _current = state; lock.unlock()
        do { try audio.resume(voiceProcessing: state == .talking) }
        catch { log.emit(.error(stage: "audio-engine", message: "engine did not start after the switch to \(state.rawValue): \(error)")) }
        let ms = Double(DispatchTime.now().uptimeNanoseconds - t0) / 1e6
        let d = inner.describe()
        log.emit(.audioSession(category: state.rawValue, mode: preferBuiltInMic ? "builtin-mic" : "route-mic", note: "\(reason): \(d)"))
        log.emit(.audioSwitch(from: from?.rawValue ?? "none", to: state.rawValue, ms: ms))
        if ms > 5000 { log.emit(.error(stage: "audio-session", message: String(format: "switch to %@ took %.1f s", state.rawValue, ms / 1000))) }
        onChange?()
    }

    public func release() {
        lock.lock(); let had = _current != nil; _current = nil; lock.unlock()
        if had { log.emit(.audioSession(category: "released", mode: "", note: "stop")) }
        inner.release()
    }
    public func describe() -> String { inner.describe() }
    public func observe(_ handler: @escaping @Sendable (String, String) -> Void) { inner.observe(handler) }

    // MARK: What the system says

    private func heard(_ kind: String, _ detail: String) {
        log.emit(.audioNotice(kind: kind, detail: detail))
        let audio = self.audio
        switch kind {
        case "interrupted":
            audio.interrupted()
        case "resumed":
            Task.detached { audio.recover(reason: "interruption ended") }
        case "reset":
            Task.detached { audio.rebuild() }
        case "route":
            // The engine's own configuration-change notification restarts it; only catch an engine that stayed down.
            if audio.down && audio.running == false {
                Task.detached { audio.recover(reason: "route changed while down") }
            }
        default: break
        }
        onChange?()
    }
}
