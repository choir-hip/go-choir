import Foundation
#if os(iOS)
import AVFoundation
#endif

/// The station: what plays when Autoradio opens. One station per user for now; later stations become a parameter here.
/// The queue's saved current item resumes from its position; with none, the DJ's stretch starts (the same entry Play uses).
@MainActor public final class Station {
    public static let shared = Station()

    public enum Reason: String, Sendable { case launch, foreground }

    /// The DJ's stretch: the entry Play uses today (the host sets it: `ChoirApp.startRadio`).
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
    public var queue: PlayQueue = .shared
    public private(set) var started = false
    public private(set) var lastReason: Reason?

    public init() {}

    /// Start the station. `.launch`: the app was opened. `.foreground`: the app came back; it starts only if this launch never has (a
    /// short return to the foreground never overrides a pause).
    @discardableResult
    public func start(reason: Reason) async -> Bool {
        if reason == .foreground, started { return false }
        guard isReady(), !otherAudioPlaying() else { return false }
        started = true; lastReason = reason
        if queue.hasRestoredCurrent { queue.resume(); return true }
        if queue.current != nil { return false }                // already playing or paused by the reader this launch
        await startDJ?()
        return true
    }
}
