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

    /// Open the microphone for one sentence. `wanted` is asked again after every wait: a take that was given up in the meantime opens nothing.
    public func begin(wanted: @escaping @Sendable () -> Bool, onFrame: @escaping @Sendable (AudioFrame) -> Void) async -> Outcome {
        token += 1; let mine = token; held = true
        let granted = await hooks.ask()
        guard mine == token, wanted() else { return await settle(mine) }                         // L2, L5
        if !granted { await release(); return .refused }                                          // L3
        await hooks.openEngine()
        guard mine == token, wanted() else { return await settle(mine) }
        await hooks.setTap(onFrame)
        if mine != token { await hooks.setTap(nil); return .stale }                              // released while the tap was being set
        await hooks.micOn(true)
        if mine != token { await hooks.micOn(false); return .stale }
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
    /// A begin that was overtaken or no longer wanted: if it is still the newest, the lease is given back; otherwise whoever overtook it owns the lease.
    private func settle(_ mine: Int) async -> Outcome {
        if mine == token { await release() }
        return .stale
    }
}
