import Foundation
import AVFoundation
#if canImport(UIKit)
import UIKit
#endif

// Playback through the background, the lock screen, an interruption and a route change (owner, 2026-11-16: "podcasts and everything else
// should play in the background ... once you backgrounded the app and moved it back, you can't resume playing").
//
// WAYS THIS CAN FAIL (written before the code; each has a test in PlaybackContinuityTests unless it says "phone"):
//  B1 An interruption (a call, Siri, another app) stops the engine and the player; the bar still says "playing", so the next tap on
//     Play is a Pause and the one after finds no engine. The state must say paused, and say what was playing.
//  B2 An interruption ends WITHOUT the system's recommendation to resume (the person started other audio): the app must not take the
//     session back by itself; the session and the engine are re-established by the next Play.
//  B3 An interruption ends WITH the recommendation: the session is taken again, the engine is back, and what was playing plays on.
//  B4 Play after the background: the claim may be dead and the engine down with nothing said. Play asks for the session first
//     (`ensureReady`) and does not trust the remembered state.
//  B5 Headphones or the Bluetooth route go away: the radio pauses and stays paused (the platform convention), it does not move to the speaker.
//  B6 Return to the foreground with the radio meant to be playing but the engine down: the session is claimed again; when that fails the
//     bar says paused (never "playing" over silence).
//  B7 An interruption's memory of what was playing outlives the interruption and resumes something the reader paused: it is consumed
//     once, and cleared by the reader's own tap.
//  B8 (phone) The system suspends the process, a synthesizer or a model cannot run without the foreground: events record it.

/// What the system's notifications mean, as pure functions (the raw values are `AVAudioSession`'s).
public enum SessionNotices {
    /// `AVAudioSessionInterruptionOptionKey` carries `shouldResume` (1) when the system recommends going on.
    public static func shouldResume(optionsRaw: UInt?) -> Bool { ((optionsRaw ?? 0) & 1) != 0 }
    /// `AVAudioSessionRouteChangeReason.oldDeviceUnavailable` (2): headphones unplugged, a Bluetooth route lost.
    public static func routeLost(reasonRaw: UInt?) -> Bool { reasonRaw == 2 }
}

/// What was playing when an interruption began, and what to play again when it ends. Consumed once.
public struct InterruptionMemory: Equatable, Sendable {
    public private(set) var programme = false
    public private(set) var remote = false
    public private(set) var open = false
    public init() {}
    /// A second "began" before the end keeps the first memory (the first one saw the playing state; the second sees the pause).
    public mutating func began(programmePlaying: Bool, remotePlaying: Bool) {
        if !open { open = true; programme = programmePlaying; remote = remotePlaying; return }
        programme = programme || programmePlaying; remote = remote || remotePlaying
    }
    /// The system paused the remote player by itself (it may be before the interruption's own notice).
    public mutating func remoteSystemPaused() { remote = true }
    /// The end. With the system's recommendation what was playing plays again; without it nothing does. Either way the memory is gone.
    public mutating func ended(shouldResume: Bool) -> (programme: Bool, remote: Bool) {
        defer { self = InterruptionMemory() }
        return shouldResume ? (programme, remote) : (false, false)
    }
    /// The reader tapped Play or Pause, or the route was lost: the memory no longer says what they want.
    public mutating func forget() { self = InterruptionMemory() }
}

#if os(iOS)
/// Observes the system's notices that the session owner (Exact) does not hand on in detail, and the app's lifecycle, and logs each.
/// It sets nothing on the session: it reads, tells the coordinator, and tells the host.
public final class SessionWatcher: @unchecked Sendable {
    private var tokens: [NSObjectProtocol] = []
    private let log: EventLog
    private let describe: @Sendable () -> String
    public var onRecommendation: (@Sendable (Bool) -> Void)?
    public var onRouteLost: (@Sendable () -> Void)?
    /// "willResignActive", "didEnterBackground", "willEnterForeground", "didBecomeActive", "protectedDataWillBecomeUnavailable" ...
    public var onLifecycle: (@Sendable (String) -> Void)?

    public init(log: EventLog, describe: @escaping @Sendable () -> String) {
        self.log = log; self.describe = describe
        let nc = NotificationCenter.default
        func add(_ n: Notification.Name, _ f: @escaping @Sendable (Notification) -> Void) { tokens.append(nc.addObserver(forName: n, object: nil, queue: .main) { f($0) }) }
        for (name, label) in [(UIApplication.willResignActiveNotification, "willResignActive"), (UIApplication.didEnterBackgroundNotification, "didEnterBackground"),
                              (UIApplication.willEnterForegroundNotification, "willEnterForeground"), (UIApplication.didBecomeActiveNotification, "didBecomeActive"),
                              (UIApplication.protectedDataWillBecomeUnavailableNotification, "screenLocking"),
                              (UIApplication.protectedDataDidBecomeAvailableNotification, "screenUnlocked")] {
            add(name) { [weak self] _ in
                guard let self else { return }
                self.log.emit(.audioNotice(kind: "lifecycle", detail: "\(label); \(self.describe())"))
                self.onLifecycle?(label)
            }
        }
        // The system's own recommendation at the end of an interruption (iOS 27 and later: a resumption recommendation; before: the
        // interruption's options). Exact reads the same notices to tell its claims; this is where the recommendation itself is kept.
        if #available(iOS 27, *) {
            add(AVAudioSession.resumptionRecommendationNotification) { [weak self] n in
                let ctx = n.userInfo?[AVAudioSession.resumptionContextKey] as? AVAudioSession.ResumptionContext
                let go = ctx?.recommendation == .shouldResume
                self?.log.emit(.audioNotice(kind: "interruption-recommendation", detail: "shouldResume=\(go)"))
                self?.onRecommendation?(go)
            }
            add(AVAudioSession.didBecomeInactiveNotification) { [weak self] n in
                let ctx = n.userInfo?[AVAudioSession.deactivationContextKey] as? AVAudioSession.DeactivationContext
                self?.log.emit(.audioNotice(kind: "session-inactive", detail: "source=\(ctx.map { "\($0.source)" } ?? "unknown")"))
            }
        } else {
            add(AVAudioSession.interruptionNotification) { [weak self] n in
                guard let raw = n.userInfo?[AVAudioSessionInterruptionTypeKey] as? UInt, let type = AVAudioSession.InterruptionType(rawValue: raw) else { return }
                if type == .began { self?.log.emit(.audioNotice(kind: "interruption-began", detail: "reason=\(n.userInfo?[AVAudioSessionInterruptionReasonKey] as? UInt ?? 0)")); return }
                let go = SessionNotices.shouldResume(optionsRaw: n.userInfo?[AVAudioSessionInterruptionOptionKey] as? UInt)
                self?.log.emit(.audioNotice(kind: "interruption-recommendation", detail: "shouldResume=\(go)"))
                self?.onRecommendation?(go)
            }
        }
        add(AVAudioSession.routeChangeNotification) { [weak self] n in
            let raw = n.userInfo?[AVAudioSessionRouteChangeReasonKey] as? UInt
            guard let self else { return }
            self.log.emit(.audioNotice(kind: "route-change", detail: "reason=\(raw ?? 0); \(self.describe())"))
            if SessionNotices.routeLost(reasonRaw: raw) { self.onRouteLost?() }
        }
        add(AVAudioSession.mediaServicesWereResetNotification) { [weak self] _ in self?.log.emit(.audioNotice(kind: "media-services-reset", detail: "")) }
    }
    deinit { for t in tokens { NotificationCenter.default.removeObserver(t) } }
}
#endif

/// Joins the session coordinator, the programme player and the queue's remote player: what an interruption, a lost route, a return to the
/// foreground and a press of Play do to all three. The decisions are in `InterruptionMemory` and the coordinator; this is the wiring.
@MainActor public final class PlaybackContinuity {
    private let runtime: LabRuntime
    private let queue: RunSheetPlayer
    private var memory = InterruptionMemory()
    #if os(iOS)
    private var watcher: SessionWatcher?
    #endif

    public init(runtime: LabRuntime, queue: RunSheetPlayer? = nil) {
        self.runtime = runtime; self.queue = queue ?? RunSheetPlayer.shared
        let coordinator = runtime.coordinator
        coordinator.onInterruption = { [weak self] began, go in Task { @MainActor in self?.interruption(began: began, shouldResume: go) } }
        #if os(iOS)
        let w = SessionWatcher(log: runtime.log, describe: { [weak coordinator] in coordinator?.describe() ?? "" })
        w.onRecommendation = { [weak coordinator] go in coordinator?.noteRecommendation(shouldResume: go) }
        w.onRouteLost = { [weak self] in Task { @MainActor in self?.routeLost() } }
        w.onLifecycle = { [weak self] name in Task { @MainActor in self?.lifecycle(name) } }
        watcher = w
        let modes = (Bundle.main.object(forInfoDictionaryKey: "UIBackgroundModes") as? [String]) ?? []
        runtime.log.emit(.audioNotice(kind: "background-modes", detail: modes.isEmpty ? "NONE declared: the system will suspend the app in the background" : modes.joined(separator: ",")))
        #endif
    }

    var programmePlaying: Bool { !runtime.player.userPaused }
    var remotePlaying: Bool { queue.isRemote && !queue.paused }

    /// The remote player says the system paused it.
    public func remoteSystemPaused() {
        memory.remoteSystemPaused()
        queue.pause()
    }

    /// The reader's own tap on Play or Pause: what was remembered no longer says what they want.
    public func readerTapped() { memory.forget() }

    func interruption(began: Bool, shouldResume: Bool) {
        if began {
            memory.began(programmePlaying: programmePlaying, remotePlaying: remotePlaying)
            if programmePlaying { runtime.player.pause(reason: "system interruption") }                       // B1: the state says what is true
            if remotePlaying { queue.pause() }
            runtime.log.emit(.audioNotice(kind: "interruption", detail: "began; will resume programme=\(memory.programme) remote=\(memory.remote)"))
            return
        }
        if shouldResume { Task { @MainActor in RunSheetPlayer.shared.sessionMayBeBack() } }      // without the recommendation the listener's Play asks
        let again = memory.ended(shouldResume: shouldResume)
        runtime.log.emit(.audioNotice(kind: "interruption", detail: "ended; shouldResume=\(shouldResume) programme=\(again.programme) remote=\(again.remote)"))
        if again.programme { runtime.player.play() }
        if again.remote { queue.resume() }
    }

    func routeLost() {
        // The radio's own session change (voice on moves a Bluetooth headset from A2DP to the hands-free profile) removes the old route: that is
        // not the headset going away, and the radio does not pause for it.
        if runtime.coordinator.changedWithin(4) {
            print("AUTORADIO route change ignored: the radio's own session change ms_since_session_change=\(runtime.coordinator.msSinceSessionChange)")
            return
        }
        memory.forget()
        runtime.log.emit(.audioNotice(kind: "route-lost", detail: "pausing (programme playing \(programmePlaying), remote playing \(remotePlaying))"))
        if programmePlaying { runtime.player.pause(reason: "route lost") }        // the listener's Play ends this one, as the system's recommendation never comes
        if remotePlaying { queue.pause() }
    }

    func lifecycle(_ name: String) {
        switch name {
        case "didEnterBackground":
            runtime.coordinator.markSuspect("app in the background")
            Task { await runtime.coordinator.setBackgrounded(true) }       // voice on: the microphone is given up, the radio plays on
        case "didBecomeActive":
            Task { @MainActor in
                await runtime.coordinator.setBackgrounded(false)
                returnedToForeground()
            }
        default: break
        }
    }

    /// B6: back in front. A programme that should be playing and has no engine gets its session again; failing that, it says paused.
    /// A podcast the system paused is already marked paused. Nothing starts by itself.
    func returnedToForeground() {
        let rt = runtime
        guard programmePlaying else { return }                  // a paused programme (the listener's, or an interruption that gave no go-ahead) is not revived by opening the app
        Task { @MainActor in RunSheetPlayer.shared.sessionMayBeBack() }
        if !rt.audio.needsRecovery { if rt.audio.programmeAudible { rt.coordinator.confirmHealthy() }; return }
        Task { @MainActor in
            let ok = await rt.ensureSessionReady(reason: "foreground")
            rt.log.emit(.audioNotice(kind: "foreground-recover", detail: ok ? "session and engine are back" : "could not recover; showing paused"))
            if !ok { rt.player.pause(reason: "foreground recovery failed") }
        }
    }
}
