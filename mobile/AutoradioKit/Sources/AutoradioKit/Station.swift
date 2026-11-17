import Foundation
#if os(iOS)
import AVFoundation
#endif

/// The station: what plays when Autoradio opens. One station per user for now; later stations become a parameter here.
/// The phone always speaks first: it places its own opening on the run sheet and plays it at once (the same entry Play uses).
@MainActor public final class Station {
    public static let shared = Station()

    public enum Reason: String, Sendable { case launch, foreground }

    /// Opens the radio: the opening, then the desk's entries (the host sets it: `ChoirApp.startRadio`).
    public var startDJ: (() async -> Void)?
    /// Onboarding is complete (the host sets it). Before then nothing starts by itself.
    public var isReady: () -> Bool = { true }
    /// Other audio is playing on the phone (music, a call): the station does not start over it, and says nothing.
    public var otherAudioPlaying: () -> Bool = {
        #if os(iOS)
        return AVAudioSession.sharedInstance().isOtherAudioPlaying
        #else
        return false
        #endif
    }
    public var player: RunSheetPlayer = .shared
    /// What the launch waits for before it starts the radio: the front page it speaks from is held (or kept from the last launch), or nothing
    /// will arrive (not signed in). Added in place of a fixed 2 s sleep (4c2c06ae9: "the sign-in and the first load get their moment").
    public var launchCondition: () -> Bool = { true }
    /// The most the launch waits for it, and how it waits (replaceable: a test's clock).
    public var launchDeadline: TimeInterval = 2
    public var launchPoll: TimeInterval = 0.05
    public var sleep: (TimeInterval) async -> Void = { try? await Task.sleep(nanoseconds: UInt64($0 * 1_000_000_000)) }
    /// Seconds the last launch waited.
    public private(set) var waited: TimeInterval = 0
    public private(set) var started = false
    public private(set) var lastReason: Reason?

    public init() {}

    /// Start the station. `.launch`: the app was opened. `.foreground`: the app came back; it starts only if this launch never has (a
    /// short return to the foreground never overrides a pause).
    @discardableResult
    public func start(reason: Reason) async -> Bool {
        if reason == .foreground, started { return false }
        guard isReady(), !otherAudioPlaying() else { return false }
        waited = 0
        if reason == .launch {
            while !launchCondition(), waited < launchDeadline { await sleep(launchPoll); waited += launchPoll }
        }
        started = true; lastReason = reason
        if player.store.onAir() != nil { return false }         // already playing or paused by the reader this launch
        await startDJ?()
        return true
    }
}
