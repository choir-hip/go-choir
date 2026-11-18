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

/// Sits between the turn engine and the host's session owner. It is the one owner of the audio session for everything that makes sound
/// (the programme's voice, the DJ, podcasts through AVPlayer, read-aloud): whenever something plays or is about to, the session is a
/// playback-capable one, and the microphone is part of it only while voice is on.
///   playing    voice off: a playback session, no microphone (the app plays in the background and with the screen locked)
///   listening  voice on, no turn open: capture session, full-quality playback
///   talking    voice on, a turn open: capture session with voice processing
/// It applies the configuration's `AudioPolicy` to the two capture states, stops the engine before the category changes and restarts it
/// after, times the switch, and turns everything the system says (interruption, route change, media-services reset) into events and
/// engine recovery. `ensureReady` is what Play calls first: it does not trust what it remembers of the session.
public final class SessionCoordinator: AudioSessionOwner, @unchecked Sendable {
    let inner: AudioSessionOwner
    let log: EventLog
    let audio: LiveAudio
    private let policy: @Sendable () -> AudioPolicy
    private let voiceOn: @Sendable () -> Bool
    private let mutex = AsyncMutex()
    private let lock = NSLock()
    private var _current: AudioState?
    private var _asked: AudioState = .listening
    /// The session may be dead although nothing has said so (the app went to the background, an interruption began): the next Play
    /// takes the claim again instead of trusting it.
    private var _suspect = false
    /// What the system last recommended at an interruption's end (nil: not heard yet).
    private var _shouldResume: Bool?
    /// Called after a notice has been handled (the runtime refreshes its snapshot).
    public var onChange: (@Sendable () -> Void)?
    /// An interruption began (`began`), or ended (`shouldResume`: the system says to go on; the session and the engine are back).
    public var onInterruption: (@Sendable (_ began: Bool, _ shouldResume: Bool) -> Void)?

    public init(inner: AudioSessionOwner, log: EventLog, audio: LiveAudio, policy: @escaping @Sendable () -> AudioPolicy,
                voiceOn: @escaping @Sendable () -> Bool = { true }) {
        self.inner = inner; self.log = log; self.audio = audio; self.policy = policy; self.voiceOn = voiceOn
        inner.observe { [weak self] kind, detail in self?.heard(kind, detail) }
        audio.onNotice = { [weak self] kind, detail in
            if kind == "error" { self?.log.emit(.error(stage: "audio-engine", message: detail)) }
            else { self?.log.emit(.audioNotice(kind: kind, detail: detail)) }
            self?.onChange?()
        }
    }

    /// The state the session is in now (nil before the first claim and after `release`).
    public var current: AudioState? { lock.lock(); defer { lock.unlock() }; return _current }
    /// True while the session is not to be trusted (see `_suspect`).
    public var suspect: Bool { lock.lock(); defer { lock.unlock() }; return _suspect }
    /// The app left or came back to the foreground, or something else made the session doubtful.
    public func markSuspect(_ why: String) {
        lock.lock(); let had = _current != nil; _suspect = _suspect || had; lock.unlock()
        if had { log.emit(.audioNotice(kind: "session", detail: "doubted: \(why)")) }
    }
    /// Sound was heard to be coming out after the doubt: the claim is good, the next Play need not take it again.
    public func confirmHealthy() { lock.lock(); _suspect = false; lock.unlock() }
    /// The system's recommendation at the end of an interruption (read from its own notification by `SessionWatcher`).
    public func noteRecommendation(shouldResume: Bool) { lock.lock(); _shouldResume = shouldResume; lock.unlock() }

    /// The state a request for `asked` becomes: voice off is a playback session whatever the policy.
    /// Voice on is one state: the capture session with voice processing (echo cancellation), whether or not a turn is open. The session does not
    /// change category or mode again between listening and talking, so the programme is never played through a mode switch; the one switch is
    /// the Voice control's, made with the engine stopped. (`alwaysListening` is the lab's own choice and stays what it says.)
    func target(for asked: AudioState) -> AudioState {
        if isRecordingTake, !isBackgrounded, !isInterrupted { return .listening }       // a take: capture and playback; given up with the microphone in the background or when another app has the session
        guard voiceOn(), !isBackgrounded else { return .playing }       // in the background the microphone is given up and the radio plays on
        return policy() == .alwaysListening ? .listening : .talking
    }
    private var _backgrounded = false, _interrupted = false
    private var _recordingTake = false
    /// A take is being recorded or reviewed: the session is the capture one (full-quality playback, no voice processing) until it is over.
    public var isRecordingTake: Bool { lock.lock(); defer { lock.unlock() }; return _recordingTake }
    /// Take the capture session for a take (`true`) or go back to what voice says (`false`). Throws when the session could not be taken.
    public func setRecordingTake(_ on: Bool) async throws {
        lock.lock(); _recordingTake = on; let has = _current != nil; lock.unlock()
        if has { await reapply() } else if on { try await claim(.listening, preferBuiltInMic: false, reason: "take") }
        if on, current != .listening { throw LabError.unavailable(lastFailure.isEmpty ? "the microphone could not be taken" : lastFailure) }
    }
    public var isBackgrounded: Bool { lock.lock(); defer { lock.unlock() }; return _backgrounded }
    /// Another app has the session (an interruption began and has not ended): nothing here asks for it back except a foreground, an
    /// interruption's end and a press of Play.
    public var isInterrupted: Bool { lock.lock(); defer { lock.unlock() }; return _interrupted }
    /// The app left or came back to the foreground: voice on gives the microphone up (a playback session) and takes it back.
    public func setBackgrounded(_ b: Bool) async {
        lock.lock(); let was = _backgrounded; _backgrounded = b; lock.unlock()
        guard was != b, voiceOn() else { return }
        audio.micSuspended = b
        print(b ? "AUTORADIO voice mic released why=background" : "AUTORADIO voice mic taken why=foreground")
        await reapply()
    }

    public func claim(_ asked: AudioState, preferBuiltInMic: Bool, reason: String) async throws {
        await mutex.lock(); defer { mutex.unlock() }
        lock.lock(); _asked = asked; lock.unlock()
        try await switchTo(target(for: asked), preferBuiltInMic: preferBuiltInMic, reason: reason)
    }

    /// The policy or the voice switch changed: move to the state it now asks for.
    public func reapply() async {
        await mutex.lock(); defer { mutex.unlock() }
        lock.lock(); let asked = _asked, cur = _current; lock.unlock()
        guard cur != nil else { return }
        try? await switchTo(target(for: asked), preferBuiltInMic: false, reason: "policy or voice changed")
    }

    /// Before anything plays (the bar's Play, the lock screen's, a podcast, a read-aloud): the session is claimed and active and the
    /// engine is up, taking the claim again when it was doubted or the engine is down. True when sound can come out.
    @discardableResult
    public func ensureReady(reason: String) async -> Bool {
        await mutex.lock(); defer { mutex.unlock() }
        if audio.virtual { return true }
        lock.lock(); let asked = _asked, cur = _current, doubted = _suspect; lock.unlock()
        let want = target(for: asked)
        if cur == want, !doubted, !audio.needsRecovery { return true }
        if isInterrupted, !["play", "foreground", "interruption ended", "resume"].contains(reason) {
            lastFailure = "another app is using the audio"
            return false                                    // not fought for: the retry waits for the foreground, the interruption's end or Play
        }
        let why = cur == nil ? "no claim" : doubted ? "session doubted" : audio.needsRecovery ? "engine down" : "state changed"
        return await reclaim(want, reason: reason, why: why)
    }

    /// Give the claim up and take it again (which activates the session), then bring the engine back. Under the mutex.
    private func reclaim(_ state: AudioState, reason: String, why: String) async -> Bool {
        log.emit(.audioNotice(kind: "session", detail: "reclaim \(state.rawValue) (\(reason); \(why)); was \(inner.describe())"))
        touched()
        // A claim already held in this state is asserted again, not given up: deactivating a session that audio is still running through
        // (a podcast, the engine) is refused as busy, and from the background the claim after it is refused too. Nothing is released.
        if inner.holds(state) {
            do { try await inner.reassert() }
            catch {
                let ns = error as NSError
                print("AUTORADIO session reassert FAILED \(state.rawValue) (\(reason)) \(ns.domain) \(ns.code); the claim is kept")
                log.emit(.error(stage: "audio-session", message: "reassert \(state.rawValue) failed (\(reason)): \(ns.domain) \(ns.code)"))
                lastFailure = "\(ns.domain) \(ns.code)"
                onChange?()
                return false
            }
            lock.lock(); _current = state; _suspect = false; _interrupted = false; lock.unlock()
            var ok = true
            if audio.needsRecovery {
                do { try audio.resume(voiceProcessing: state == .talking, mic: state != .playing) }
                catch { ok = false; log.emit(.error(stage: "audio-engine", message: "engine did not start after reassert of \(state.rawValue) (\(reason)): \(error)")) }
            }
            print("AUTORADIO session reasserted \(state.rawValue) (\(reason))")
            onChange?()
            return ok
        }
        audio.quiesce()
        print("AUTORADIO released reason=reclaim (\(reason); \(why))")
        inner.release()
        do { try await inner.claim(state, preferBuiltInMic: false, reason: "reclaim: \(reason)") }
        catch {
            lock.lock(); _current = nil; lock.unlock()
            let ns = error as NSError
            lastFailure = "\(ns.domain) \(ns.code): \(ns.localizedDescription)"
            print("AUTORADIO claim FAILED \(state.rawValue) (\(reason)) \(ns.domain) \(ns.code): \(ns.localizedDescription)")
            log.emit(.error(stage: "audio-session", message: "claim \(state.rawValue) failed (\(reason)): \(error)"))
            onChange?()
            return false
        }
        lock.lock(); _current = state; _suspect = false; _interrupted = false; lock.unlock()
        var ok = true
        do { try audio.resume(voiceProcessing: state == .talking, mic: state != .playing) }
        catch { ok = false; log.emit(.error(stage: "audio-engine", message: "engine did not start after reclaim of \(state.rawValue) (\(reason)): \(error)")) }
        log.emit(.audioSession(category: state.rawValue, mode: "reclaim", note: "\(reason): \(inner.describe())"))
        onChange?()
        return ok
    }

    private func switchTo(_ state: AudioState, preferBuiltInMic: Bool, reason: String) async throws {
        let from = current
        if from == state { return }
        touched()
        let t0 = DispatchTime.now().uptimeNanoseconds
        // What is on air is held (as for a voice line) while the session is reconfigured, and goes on from the held position: the programme is
        // never played through a mode switch.
        audio.holdForSessionSwitch(true)
        defer { audio.holdForSessionSwitch(false) }
        audio.quiesce()                                   // the category change would stop it anyway
        do { try await inner.claim(state, preferBuiltInMic: preferBuiltInMic, reason: reason) }
        catch {
            lock.lock(); _current = nil; lock.unlock()
            log.emit(.error(stage: "audio-session", message: "claim \(state.rawValue) failed (\(reason)): \(error)"))
            try? audio.resume(voiceProcessing: from == .talking, mic: from.map { $0 != .playing })   // put the engine back as it was, if it can be
            throw error
        }
        lock.lock(); _current = state; _suspect = false; lock.unlock()
        do { try audio.resume(voiceProcessing: state == .talking, mic: state != .playing) }
        catch { log.emit(.error(stage: "audio-engine", message: "engine did not start after the switch to \(state.rawValue): \(error)")) }
        let ms = Double(DispatchTime.now().uptimeNanoseconds - t0) / 1e6
        let d = inner.describe()
        log.emit(.audioSession(category: state.rawValue, mode: preferBuiltInMic ? "builtin-mic" : "route-mic", note: "\(reason): \(d)"))
        log.emit(.audioSwitch(from: from?.rawValue ?? "none", to: state.rawValue, ms: ms))
        print("AUTORADIO session \(from?.rawValue ?? "none") -> \(state.rawValue) took_ms=\(Int(ms))")
        if ms > 5000 { log.emit(.error(stage: "audio-session", message: String(format: "switch to %@ took %.1f s", state.rawValue, ms / 1000))) }
        onChange?()
    }

    /// The last session failure in plain words (the system's error domain and code), for the notice.
    public private(set) var lastFailure = ""
    private var lastTouch = Date.distantPast
    private func touched() { lock.lock(); lastTouch = Date(); lock.unlock() }
    /// The radio itself changed the session in the last `seconds`: a pause that arrives now is the system reacting to that, not the listener.
    public var msSinceSessionChange: Int { lock.lock(); defer { lock.unlock() }; return Int(min(Date().timeIntervalSince(lastTouch), 3600) * 1000) }
    public func changedWithin(_ seconds: TimeInterval) -> Bool { lock.lock(); defer { lock.unlock() }; return Date().timeIntervalSince(lastTouch) < seconds }

    public func release() {
        print("AUTORADIO released reason=stop")
        lock.lock(); let had = _current != nil; _current = nil; lock.unlock()
        if had { log.emit(.audioSession(category: "released", mode: "", note: "stop")) }
        inner.release()
    }
    public func describe() -> String { inner.describe() }
    public func observe(_ handler: @escaping @Sendable (String, String) -> Void) { inner.observe(handler) }

    // MARK: What the system says

    func heard(_ kind: String, _ detail: String) {
        log.emit(.audioNotice(kind: kind, detail: detail))
        let audio = self.audio
        switch kind {
        case "interrupted":
            lock.lock(); _suspect = true; _shouldResume = nil; _interrupted = true; lock.unlock()
            audio.micSuspended = true
            print("AUTORADIO interruption began")
            audio.interrupted()
            onInterruption?(true, false)
        case "resumed":
            // The system's own recommendation arrives in a notification of its own; one more turn of the main queue and it has been read.
            DispatchQueue.main.async { [weak self] in
                guard let self else { return }
                self.lock.lock(); let go = self._shouldResume ?? false; self._shouldResume = nil; self.lock.unlock()
                Task.detached { await self.interruptionEnded(shouldResume: go) }
            }
        case "reset":
            lock.lock(); _suspect = true; lock.unlock()
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

    /// An interruption ended. With the system's recommendation to go on, the session is taken again and the engine brought back (the
    /// programme is told, and resumes what it had); without it (the person started other audio) nothing is taken: the session and the
    /// engine are re-established by the next Play.
    func interruptionEnded(shouldResume: Bool) async {
        lock.lock(); _interrupted = false; lock.unlock()
        audio.micSuspended = isBackgrounded
        print("AUTORADIO interruption ended resume=\(shouldResume)")
        log.emit(.audioNotice(kind: "interruption-ended", detail: "shouldResume=\(shouldResume)"))
        if shouldResume {
            await mutex.lock()
            lock.lock(); let cur = _current; lock.unlock()
            var ok = false
            if let cur { ok = await reclaim(cur, reason: "interruption ended", why: "system recommends resuming") }
            mutex.unlock()
            if !ok { onInterruption?(false, false); return }
        }
        onInterruption?(false, shouldResume)
    }
}
