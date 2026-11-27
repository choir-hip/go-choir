import Foundation
import AVFoundation
#if canImport(UIKit)
import UIKit
#endif

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
        audio.onNeedsEngine = { [weak self] in await self?.recoverForBlock() ?? false }
    }

    /// Another sound of the app is playing outside the engine (a podcast through AVPlayer): the session is not deactivated under it. Set by the host.
    public var remoteAudioActive: (@Sendable () -> Bool)?
    /// An interruption ended without the system's recommendation to go on (the person started other audio): nothing here takes the session
    /// back until Play, a foreground or a resume asks.
    private var _awaitingPlay = false
    static let explicitReasons: Set<String> = ["play", "foreground", "interruption ended", "resume"]

    /// A block found the engine down. It is brought back only while the radio still wants to play: a claim exists, the programme is wanted
    /// audible (not paused or stopped), and no interruption waits for the listener's Play. Otherwise false: the block gives up and the entry is held.
    func recoverForBlock() async -> Bool {
        lock.lock(); let has = _current != nil, waiting = _awaitingPlay; lock.unlock()
        guard has, !waiting, !isInterrupted, audio.isPlayWanted else {
            print("AUTORADIO block does not take the session back claim=\(has) awaiting_play=\(waiting) interrupted=\(isInterrupted) wanted=\(audio.isPlayWanted)")
            return false
        }
        return await ensureReady(reason: "engine down at a block")
    }

    /// Everything the console log needs when the session or the engine cannot be taken (ids, flags and numbers; no words).
    public func failureDiagnostic() async -> String {
        var app = "unknown"
        #if canImport(UIKit) && !os(macOS)
        app = await MainActor.run { switch UIApplication.shared.applicationState { case .active: "active"; case .inactive: "inactive"; case .background: "background"; @unknown default: "other" } }
        #endif
        var sess = ""
        #if os(iOS)
        let a = AVAudioSession.sharedInstance()
        sess = " cat=\(a.category.rawValue) mode=\(a.mode.rawValue) outs=\(a.currentRoute.outputs.map { $0.portType.rawValue }) other_audio=\(a.isOtherAudioPlaying) silence_hint=\(a.secondaryAudioShouldBeSilencedHint)"
        #endif
        return "app=\(app)\(sess) current=\(current?.rawValue ?? "none") suspect=\(suspect) interrupted=\(isInterrupted) \(audio.diagnostic()) last=\(lastFailure)"
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
        guard voiceOn(), !isBackgrounded, !_paused else { return .playing }       // in the background or paused the microphone is given up
        return policy() == .alwaysListening ? .listening : .talking
    }
    private var _backgrounded = false, _interrupted = false
    /// The last session move failed (the claim is gone and the engine was put back as it was): `reapply` does not call that closed.
    private var _moveFailed = false
    /// The radio is paused (the listener's pause or the system's). A paused radio keeps the claim, so the page
    /// stays alive and shared code keeps writing ahead, but it gives the MICROPHONE up: another app may record.
    private var _paused = false
    private var _recordingTake = false
    /// The last switch brought the engine up (or there was none to bring).
    var _engineOK = true
    /// A take is being recorded or reviewed: the session is the capture one (full-quality playback, no voice processing) until it is over.
    public var isRecordingTake: Bool { lock.lock(); defer { lock.unlock() }; return _recordingTake }
    /// Take the capture session for a take (`true`) or go back to what voice says (`false`). Throws when the session could not be taken.
    public func setRecordingTake(_ on: Bool) async throws {
        if on, TurnGate.shared.signedOutShut { throw LabError.unavailable("the account is signing out") }      // the place that opens the microphone for a take refuses, whoever calls
        lock.lock(); _recordingTake = on; let has = _current != nil; lock.unlock()
        if has { await reapply() }
        else if on {
            do { try await claim(.listening, preferBuiltInMic: false, reason: "take") }
            catch { lock.lock(); _recordingTake = false; lock.unlock(); throw error }
        }
        lock.lock(); let engineOK = _engineOK || !audio.needsRecovery; lock.unlock()
        if on, current != .listening || !engineOK {
            // The take must not start on a session that is not ready: the radio goes back to what it was.
            let why = engineOK ? (lastFailure.isEmpty ? "the microphone could not be taken" : lastFailure) : "the audio engine did not start"
            lock.lock(); _recordingTake = false; lock.unlock()
            if current != nil { await reapply() }
            print("AUTORADIO take refused: \(why)")
            throw LabError.unavailable(why)
        }
    }
    public var isBackgrounded: Bool { lock.lock(); defer { lock.unlock() }; return _backgrounded }
    /// Another app has the session (an interruption began and has not ended): nothing here asks for it back except a foreground, an
    /// interruption's end and a press of Play.
    public var isInterrupted: Bool { lock.lock(); defer { lock.unlock() }; return _interrupted }
    /// The app left or came back to the foreground: voice on gives the microphone up (a playback session) and takes it back.
    public func setBackgrounded(_ b: Bool) async { noteBackgrounded(b); await applyBackgrounded() }
    /// The fact, at once and in order: the lifecycle notices arrive on the main queue in the order they happened, so this is called from the notice itself. The work that follows (`applyBackgrounded`) is a Task and
    /// Tasks do not run in order: it reads the value as it is by then, so a quick lock and unlock cannot leave the flag `true` while the app is in front (review F6).
    public func noteBackgrounded(_ b: Bool) { lock.lock(); _backgrounded = b; lock.unlock() }
    private var _bgApplied = false
    /// Carry out what the flag says: the microphone given up in the background and taken back in front.
    public func applyBackgrounded() async {
        lock.lock(); let now = _backgrounded, was = _bgApplied; _bgApplied = now; lock.unlock()
        guard was != now, voiceOn() else { return }
        audio.micSuspended = now
        print(now ? "AUTORADIO voice mic released why=background" : "AUTORADIO voice mic taken why=foreground")
        await reapply()
    }

    public func claim(_ asked: AudioState, preferBuiltInMic: Bool, reason: String) async throws {
        await mutex.lock(); defer { mutex.unlock() }
        lock.lock(); _asked = asked; _awaitingPlay = false; lock.unlock()
        try await switchTo(target(for: asked), preferBuiltInMic: preferBuiltInMic, reason: reason)
    }

    /// The radio was paused or resumed. Paused: the claim is kept (the page and shared code live on it) and the
    /// session becomes a plain playback one, so the microphone is free for another app. Resumed: whatever the
    /// policy asks for now. Owner, 2026-11-25: "even if i pause playback, it still holds the audio session, and
    /// disrupts other apps that would eg want to use the microphone".
    public func setPaused(_ paused: Bool) async {
        lock.lock(); let had = _current != nil; _paused = paused; let asked = _asked; lock.unlock()
        guard had else { return }
        await mutex.lock(); defer { mutex.unlock() }
        try? await switchTo(paused ? .playing : target(for: asked), preferBuiltInMic: false, reason: paused ? "paused" : "resumed")
    }

    /// The policy or the voice switch changed: move to the state it now asks for.
    @discardableResult
    public func reapply() async -> Bool {
        await mutex.lock(); defer { mutex.unlock() }
        lock.lock(); let asked = _asked, cur = _current; lock.unlock()
        guard cur != nil else { lock.lock(); defer { lock.unlock() }; return !_moveFailed }      // no claim: closed, unless a move just failed (another caller's, which left the engine as it was)
        do { try await switchTo(target(for: asked), preferBuiltInMic: false, reason: "policy or voice changed"); return true } catch { return false }
    }

    /// Before anything plays (the bar's Play, the lock screen's, a podcast, a read-aloud): the session is claimed and active and the
    /// engine is up, taking the claim again when it was doubted or the engine is down. True when sound can come out.
    @discardableResult
    public func ensureReady(reason: String) async -> Bool {
        await mutex.lock(); defer { mutex.unlock() }
        if audio.virtual { return true }
        let explicit = Self.explicitReasons.contains(reason)
        if explicit { lock.lock(); _awaitingPlay = false; lock.unlock() }       // the listener (or the app coming back) asks: the wait is over
        lock.lock(); let asked = _asked, cur = _current, doubted = _suspect; lock.unlock()
        let want = target(for: asked)
        if cur == want, !doubted, !audio.needsRecovery { return true }
        lock.lock(); let awaiting = _awaitingPlay; lock.unlock()
        if isInterrupted || awaiting, !explicit {
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
        // (a podcast, the engine) is refused as busy, and from the background the claim after it is refused too. Nothing is released here;
        // below, when asserting it again did not bring the engine back, the claim is given up only if no take is being recorded and no remote
        // audio is playing.
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
                // The engine is down: a new one, not the old one started again.
                do { try audio.resume(voiceProcessing: state == .talking, mic: state != .playing, fresh: true) }
                catch {
                    ok = false
                    LiveAudio.printFault("engine did not start after reassert of \(state.rawValue) (\(reason))", error)
                    log.emit(.error(stage: "audio-engine", message: "engine did not start after reassert of \(state.rawValue) (\(reason)): \(error)"))
                }
            }
            if ok {
                print("AUTORADIO session reasserted \(state.rawValue) (\(reason))")
                onChange?()
                return true
            }
            // A podcast through AVPlayer is playing: giving the claim up would be refused as busy or would cut it. The engine was already made anew.
            if remoteAudioActive?() == true || isRecordingTake {
                print("AUTORADIO session reassert left the engine down; \(isRecordingTake ? "a take is being recorded" : "remote audio is playing"), the claim is not given up")
                onChange?()
                return false
            }
            // Asserting the same claim again did not bring the engine back, and would not on the next try: give the claim up and take it anew.
            print("AUTORADIO session reassert left the engine down; full release and re-claim of \(state.rawValue)")
        }
        audio.quiesce()
        audio.detachObserver()
        print("AUTORADIO released reason=reclaim (\(reason); \(why))")
        inner.release()
        do { try await inner.claim(state, preferBuiltInMic: false, reason: "reclaim: \(reason)") }
        catch {
            lock.lock(); _current = nil; lock.unlock()
            let ns = error as NSError
            lastFailure = "\(ns.domain) \(ns.code): \(ns.localizedDescription)"
            print("AUTORADIO claim FAILED \(state.rawValue) (\(reason)) \(ns.domain) \(ns.code): \(ns.localizedDescription)")
            log.emit(.error(stage: "audio-session", message: "claim \(state.rawValue) failed (\(reason)): \(error)"))
            try? audio.resume(voiceProcessing: state == .talking, mic: state != .playing, fresh: true)       // as `switchTo` does: the engine is put back, and observed again
            onChange?()
            return false
        }
        lock.lock(); _current = state; _suspect = false; _interrupted = false; lock.unlock()
        var ok = true
        do { try audio.resume(voiceProcessing: state == .talking, mic: state != .playing, fresh: true) }
        catch { ok = false; LiveAudio.printFault("engine did not start after reclaim of \(state.rawValue)", error); log.emit(.error(stage: "audio-engine", message: "engine did not start after reclaim of \(state.rawValue) (\(reason)): \(error)")) }
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
        let rateBefore = audio.outputRate()
        audio.quiesce()
        audio.detachObserver()                            // before the category changes: a notification of the old engine restarts nothing                                   // the category change would stop it anyway
        do { try await inner.claim(state, preferBuiltInMic: preferBuiltInMic, reason: reason) }
        catch {
            lock.lock(); _current = nil; _moveFailed = true; lock.unlock()
            log.emit(.error(stage: "audio-session", message: "claim \(state.rawValue) failed (\(reason)): \(error)"))
            try? audio.resume(voiceProcessing: from == .talking, mic: from.map { $0 != .playing })   // put the engine back as it was, if it can be
            throw error
        }
        lock.lock(); _current = state; _suspect = false; _moveFailed = false; lock.unlock()
        // A change of category moves the hardware format and the route: the engine's graph is built again, not started again.
        let buildsBefore = audio.engineBuilds
        lock.lock(); _engineOK = true; lock.unlock()
        do { try audio.resume(voiceProcessing: state == .talking, mic: state != .playing, fresh: from != nil) }
        catch { lock.lock(); _engineOK = false; lock.unlock(); LiveAudio.printFault("engine did not start after the switch to \(state.rawValue)", error); log.emit(.error(stage: "audio-engine", message: "engine did not start after the switch to \(state.rawValue): \(error)")) }
        print("AUTORADIO category change \(from?.rawValue ?? "none") -> \(state.rawValue) rate_before=\(rateBefore) rate_after=\(audio.outputRate()) engine_rebuilt=\(audio.engineBuilds > buildsBefore)")
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
        if !shouldResume { lock.lock(); _awaitingPlay = true; lock.unlock() }
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
