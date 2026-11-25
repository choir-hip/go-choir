import Foundation

// ONE owner of the microphone for the setup screens (voice enrolment today; review round 2, 2026-11-24: B3 and B4). Every open has a matching close on every exit: the page gone, sign-out, a refusal, a timeout, a relearn with hands-free on.
// The engine's own switch is never trusted between takes: while the lease is held the engine's frames go to the tap only (no turn is looked for), and between takes the microphone is forced OFF even when the person's hands-free switch is on.
// `release` is unconditional and idempotent: it clears the tap, gives the stored switch back, and makes any answer still on its way (a slow system prompt) do nothing.
//
// WAYS THIS CAN GO WRONG (each has a test in SetupMicLeaseTests):
//  L1 Between takes the microphone is off and the tap stays: nothing said between sentences can become a turn, hands-free on or off.
//  L2 A permission answer that arrives after the page is gone, or after the take was given up, opens nothing.
//  L3 A refusal closes everything and says so.
//  L4 `release` with nothing held, or twice, is harmless; it always clears the tap and restores the stored switch.
//  L6 Nothing is asked of the person for a take that is not wanted any more.
//  L7 An older begin that was overtaken never turns the newer take's microphone off.
//  L8 The app going to the background releases the live lease (`releaseLive`).
//  L5 A second `begin` while the first still waits for the system prompt makes the first do nothing.
public actor SetupMicLease {
    public struct Hooks: Sendable {
        /// The one system prompt (`LiveAudio.requestMicPermission`); shared code showed its plain line before it was reached.
        public var ask: @Sendable () async -> Bool
        /// Start the engine so frames flow.
        public var openEngine: @Sendable () async -> Void
        /// Route every microphone frame to this closure and to nothing else; nil restores the engine.
        public var setTap: @Sendable ((@Sendable (AudioFrame) -> Void)?) async -> Void
        /// The microphone on or off for this screen only (not the stored switch).
        public var micOn: @Sendable (Bool) async -> Void
        /// Give back the stored switch.
        public var restore: @Sendable () async -> Void
        public init(ask: @escaping @Sendable () async -> Bool, openEngine: @escaping @Sendable () async -> Void,
                    setTap: @escaping @Sendable ((@Sendable (AudioFrame) -> Void)?) async -> Void, micOn: @escaping @Sendable (Bool) async -> Void, restore: @escaping @Sendable () async -> Void) {
            self.ask = ask; self.openEngine = openEngine; self.setTap = setTap; self.micOn = micOn; self.restore = restore
        }
    }
    public enum Outcome: Sendable, Equatable { case open, refused, stale }

    private let hooks: Hooks
    private var token = 0
    private var held = false
    public init(_ hooks: Hooks) { self.hooks = hooks }

    public var isHeld: Bool { held }
    /// The lease the app has live, so a lifecycle notice (the app going to the background) can let go without knowing the page.
    nonisolated(unsafe) public static weak var live: SetupMicLease?
    public static func releaseLive() async { await live?.release() }

    /// Open the microphone for one sentence. `wanted` is asked again after every wait: a take that was given up in the meantime opens nothing.
    public func begin(wanted: @escaping @Sendable () -> Bool, onFrame: @escaping @Sendable (AudioFrame) -> Void) async -> Outcome {
        token += 1; let mine = token; held = true
        guard wanted() else { return await settle(mine) }                                         // no prompt for a take nobody wants (L6)
        let granted = await hooks.ask()
        guard mine == token, wanted() else { return await settle(mine) }                         // L2, L5
        if !granted { await release(); return .refused }                                          // L3
        await hooks.openEngine()
        guard mine == token, wanted() else { return await settle(mine) }
        await hooks.setTap(onFrame)
        if mine != token { await hooks.setTap(nil); return .stale }                              // released while the tap was being set
        await hooks.micOn(true)
        if mine != token { return .stale }                                                         // whoever overtook this begin owns the microphone now (L7)
        return .open
    }
    /// The sentence is over: the microphone is off, the tap stays (L1).
    public func endTake() async { guard held else { return }; await hooks.micOn(false) }
    /// Everything closed, unconditionally (L4).
    public func release() async {
        token += 1; held = false
        await hooks.setTap(nil)
        await hooks.restore()
    }
    /// The microphone for `seconds`, for a recording the person asked for (the Test button on a model's row): the frames go to this recording alone, and the lease is given back WHOLE on every exit (the tap cleared, the stored hands-free
    /// switch restored), a cancelled one too. nil when the microphone was refused or nothing was heard.
    public func record(seconds: Double) async -> [Float]? {
        final class Take: @unchecked Sendable {
            let l = NSLock(); var samples: [Float] = []; var done = false
            func add(_ s: [Float]) { l.lock(); if !done { samples += s }; l.unlock() }
            func finish() -> [Float] { l.lock(); done = true; defer { l.unlock() }; return samples }
            var open: Bool { l.lock(); defer { l.unlock() }; return !done }
        }
        let take = Take()
        let out = await begin(wanted: { take.open }, onFrame: { take.add($0.samples) })
        guard out == .open else { _ = take.finish(); return nil }                      // refused (begin released) or overtaken (whoever overtook it owns the lease)
        let mine = token
        try? await Task.sleep(nanoseconds: UInt64(seconds * 1e9))                      // a cancelled sleep ends early and falls through to the release
        let samples = take.finish()
        if token == mine { await release() }                                            // only while this recording still holds the lease: a newer take's microphone is not ours to close
        return samples.isEmpty ? nil : samples
    }
    /// A begin that was overtaken or no longer wanted: if it is still the newest, the lease is given back; otherwise whoever overtook it owns the lease.
    private func settle(_ mine: Int) async -> Outcome {
        if mine == token { await release() }
        return .stale
    }
}
