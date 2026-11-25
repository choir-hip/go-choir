import Foundation

// DOOR A: the ONE way a model file reaches the phone (review round 3; docs/problems/onboard-consent-rules-escape-by-another-path-2026-11-24.md).
// Every fetch in the app (setup, Settings > Voice and models, the voice cards, a retry, a resume, the search pass, the voice print) asks this door for its transport, and the door
//  - refuses a model the person has not agreed to (the agreement is Exact's consent record, pushed here by shared code: `sync`), and a model that is in no group (a model that is not offered cannot be fetched).
//    The agreement is PER MODEL: a press on a model's row in Voice and models records the agreement under that model's id, and it covers that model's files only (owner, 2026-11-24). A record under a group's
//    name still covers the models of that group (the search pass keeps its agreement under "search"); nothing the app writes now is a group record except that one;
//  - hands back a URLSession whose configuration forbids what the agreement forbids: Wi-Fi only (`agreed`) turns cellular, expensive and constrained access OFF in the transport itself, so a network change during a
//    download, a retry and a resume cannot spend mobile data; only `agreed-mobile` leaves it on.
// What the door cannot reach: a library that downloads on its own session (FluidAudio's recogniser and voices). For those `authorizeLibrary` refuses a start without consent and a start on a path the agreement does not cover.
//
// WAYS THIS CAN GO WRONG (each has a test in DownloadDoorTests, through an injected fetcher, one per former path):
//  A1 No agreement, or one for another group: refused, nothing requested (the store, the bundle, the voice print, the provider).
//  A2 A Wi-Fi agreement hands out a session that cannot use cellular or an expensive or constrained network; a mobile agreement does.
//  A3 A model that is in no group is refused.
//  A4 A library is never able to fetch outside a window; a window needs consent and a covered path, and is cut when the path stops being covered.
//  A5 Sign-out (an empty record) closes the door.
public final class DownloadDoor: @unchecked Sendable {
    public enum Agreement: String, Sendable, Equatable { case wifi, mobile }
    public enum Refusal: Error, Equatable, CustomStringConvertible {
        case noConsent(group: String), notOffered(model: String), pathNotAgreed(group: String), timedOut
        /// What the person is told (the voice cards and the rows say this, not the code's words).
        public var person: String {
            switch self {
            case .noConsent: return "Press Download on this model's own row first: that press is your agreement to this download."
            case .notOffered: return "This is not offered for download."
            case .pathNotAgreed: return "This needs Wi-Fi: you agreed to it on Wi-Fi only. It will go on when you are back on Wi-Fi."
            case .timedOut: return "This took too long and was stopped. Try it again."
            }
        }
        public var description: String {
            switch self {
            case .noConsent(let g): return "the person has not agreed to the \(g) downloads"
            case .notOffered(let m): return "\(m) is not offered for download"
            case .pathNotAgreed(let g): return "the \(g) downloads were agreed to on Wi-Fi only and the phone is not on Wi-Fi"
            case .timedOut: return "the download window ran past its deadline"
            }
        }
    }

    public static let shared = DownloadDoor()
    private let lock = NSLock()
    private var agreed: [String: Agreement] = [:]
    private let onWiFi: @Sendable () -> Bool
    private let allowAll: Bool
    private let groups: [String: String]
    /// How long a fetch may wait for the window (five minutes, then it fails and can be retried), and how long a window's body may be SILENT (no progress reported) before it is failed and the slot freed.
    private let deadlines: (wait: TimeInterval, body: TimeInterval)

    public init(onWiFi: @escaping @Sendable () -> Bool = { WiFiMonitor.shared.onWiFi }, list: ModelList = .builtIn(), deadlines: (wait: TimeInterval, body: TimeInterval) = (wait: 300, body: 600)) {
        self.onWiFi = onWiFi; self.allowAll = false; self.groups = Self.groups(of: list); self.deadlines = deadlines
    }
    /// A store used by tests with its own fake transport, which is not a model of the app: everything passes on the transport it was given.
    init(allowAll: Bool) { self.onWiFi = { true }; self.allowAll = allowAll; self.groups = [:]; self.deadlines = (wait: 300, body: 600) }
    /// For tests of a store with a transport of its own.
    public static var open: DownloadDoor { DownloadDoor(allowAll: true) }

    static func groups(of list: ModelList) -> [String: String] {
        var out: [String: String] = [ModelOffer.speakerID: "voice-rest", KnownModels.embeddingGemma2.id: "search"]
        for e in list.models { if let role = ConfigRole.allCases.first(where: { $0.rawValue == e.job }) { out[e.id] = ModelOffer.group(role.stage.rawValue) } }
        return out
    }

    /// Exact's consent record, as it stands: group -> "agreed" (Wi-Fi only) or "agreed-mobile". Anything else is no agreement. Called at every change and at load; an empty record (sign-out) closes the door.
    public func sync(agreed record: [String: String]) {
        var next: [String: Agreement] = [:]
        for (g, v) in record { if v == "agreed" { next[g] = .wifi } else if v == "agreed-mobile" { next[g] = .mobile } }
        lock.lock(); agreed = next; lock.unlock()
        enforce()                                                       // an agreement withdrawn or lowered cuts the window it covered
    }

    public func group(ofModel id: String) -> String? { groups[id] }
    /// Sign-out and a new account: nothing is agreed.
    public func reset() { sync(agreed: [:]) }

    /// The agreement that covers this model, or the reason it has none.
    public func authorize(model id: String) throws -> Agreement {
        if allowAll { return .mobile }
        guard let g = groups[id] else { throw Refusal.notOffered(model: id) }
        lock.lock(); let a = agreed[id] ?? agreed[g]; lock.unlock()
        guard let a else { throw Refusal.noConsent(group: g) }
        return a
    }

    /// The transport for this model's download: authorised, and unable to use a network the agreement does not cover. `injected` is a test's own transport (authorisation still applies).
    public func session(for id: String, injected: URLSession? = nil) throws -> URLSession {
        let a = try authorize(model: id)
        if let injected { return injected }
        return URLSession(configuration: Self.configuration(for: a))
    }
    static func configuration(for a: Agreement) -> URLSessionConfiguration {
        let c = URLSessionConfiguration.default
        let cellular = a == .mobile
        c.allowsCellularAccess = cellular; c.allowsExpensiveNetworkAccess = cellular
        c.allowsConstrainedNetworkAccess = true          // Low Data Mode on a Wi-Fi network is still Wi-Fi; cellular and expensive (a hotspot) are the switches that matter
        c.waitsForConnectivity = false
        return c
    }

    // MARK: Libraries that download by themselves (FluidAudio)
    // A library's own downloader runs on a session we cannot configure and finds its files by name. So it is never left able to fetch: its registry (`ModelRegistry.baseURL`) points nowhere (`closedRegistry`) from the start, and is repointed at the real host only inside a
    // WINDOW. A window is one model group at a time (a fetch for another group waits for the open one), it is opened only after consent and the path are checked INSIDE the lock that opens the registry, and it is KILLED (the registry closed at once, whatever the library
    // does about cancellation, and the transfer's Task cancelled) the moment the path stops being covered, the agreement is withdrawn or lowered, or the account changes.
    // A library that is initialised outside a window finds the registry closed: with files missing it fails; it never fetches (and the stages initialise only for a model that is ready by manifest or being checked: `LibraryScope`).
    public static let closedRegistry = "https://models.invalid"
    public static let openRegistry = "https://huggingface.co"
    private var registryHook: (@Sendable (String) -> Void)?
    private struct Window { let group: String; let agreement: Agreement; let cancel: () -> Void }
    private var live: [UUID: Window] = [:]
    private var slotBusy = false
    public var openWindowCount: Int { lock.lock(); defer { lock.unlock() }; return live.count }

    /// The models module installs the way to point the library's registry; it is closed at once (and stays closed while a window is open: installing again must not cut or reopen one).
    public func installLibraryGuard(_ setRegistry: @escaping @Sendable (String) -> Void) {
        lock.lock(); registryHook = setRegistry; let none = live.isEmpty; lock.unlock()
        if none { setRegistry(Self.closedRegistry) }
    }

    private final class CancelBox: @unchecked Sendable { let lock = NSLock(); var cancel: (@Sendable () -> Void)?; var killed = false; var onKilled: (@Sendable () -> Void)?
        func kill() { lock.lock(); killed = true; let c = cancel; let k = onKilled; lock.unlock(); k?(); c?() }
        func set(_ c: @escaping @Sendable () -> Void) { lock.lock(); cancel = c; let k = killed; lock.unlock(); if k { c() } } }

    private struct Waiter { let id: UUID; let c: CheckedContinuation<Void, Error> }
    private var waiters: [Waiter] = []
    /// Wait for the one slot. A cancelled waiter leaves the queue and throws (it never opens a window); one that waits past the deadline throws `timedOut`. `whileQueued` is told every `every` seconds that it is waiting (a stall watchdog must not read a wait as a stall).
    private func acquireSlot(every: TimeInterval, whileQueued: (@Sendable () -> Void)?) async throws {
        let id = UUID()
        let ticker: Task<Void, Never>? = whileQueued.map { tell in Task { while !Task.isCancelled { try? await Task.sleep(nanoseconds: UInt64(every * 1e9)); if !Task.isCancelled { tell() } } } }
        let timeout = Task { try? await Task.sleep(nanoseconds: UInt64(deadlines.wait * 1e9)); if !Task.isCancelled { self.dropWaiter(id, throwing: Refusal.timedOut) } }
        defer { ticker?.cancel(); timeout.cancel() }
        try await withTaskCancellationHandler {
            try await withCheckedThrowingContinuation { (c: CheckedContinuation<Void, Error>) in
                lock.lock()
                if Task.isCancelled { lock.unlock(); c.resume(throwing: CancellationError()); return }
                if !slotBusy { slotBusy = true; lock.unlock(); c.resume() } else { waiters.append(Waiter(id: id, c: c)); lock.unlock() }
            }
        } onCancel: { self.dropWaiter(id, throwing: CancellationError()) }
        if Task.isCancelled { releaseSlot(); throw CancellationError() }
    }
    private func dropWaiter(_ id: UUID, throwing e: Error) {
        lock.lock(); let w = waiters.firstIndex { $0.id == id }.map { waiters.remove(at: $0) }; lock.unlock()
        w?.c.resume(throwing: e)
    }
    private func releaseSlot() {
        lock.lock()
        if waiters.isEmpty { slotBusy = false; lock.unlock() } else { let w = waiters.removeFirst(); lock.unlock(); w.c.resume() }
    }

    /// Report progress (bytes arrived, a step finished): the body's deadline is about SILENCE, not wall time (a 480 MB model on a slow line is not a hang).
    public func noteProgress() { lock.lock(); progressAt = Date(); lock.unlock() }
    private var progressAt = Date()

    private final class Gate<T: Sendable>: @unchecked Sendable {
        let l = NSLock(); var c: CheckedContinuation<T, Error>?; var done = false; var pending: Result<T, Error>?
        func set(_ k: CheckedContinuation<T, Error>) { l.lock(); if let p = pending { l.unlock(); k.resume(with: p) } else { c = k; l.unlock() } }
        /// The first finish wins; `before` runs after it has won and BEFORE the caller is resumed (the window's kill, so the flag it sets is there when the caller looks).
        @discardableResult func finish(_ r: Result<T, Error>, before: (() -> Void)? = nil) -> Bool { l.lock(); if done { l.unlock(); return false }; done = true; let k = c; if k == nil { pending = r }; l.unlock(); before?(); k?.resume(with: r); return true }
    }

    /// Run `body` and answer when it ends, when the caller is cancelled, or when it has been silent past its deadline, WITHOUT waiting for a body that ignores cancellation. `kill` is what to do to it (cancel its task, close the registry).
    private func raced<T: Sendable>(_ body: @escaping @Sendable () async throws -> T, box: CancelBox? = nil, onBodyEnded: (@Sendable () -> Void)? = nil, onGiveUp: (@Sendable () -> Void)? = nil, kill: @escaping @Sendable () -> Void) async throws -> T {
        let result = Task<T, Error> { try await body() }
        let gate = Gate<T>(); let silence = deadlines.body
        box?.set { gate.finish(.failure(CancellationError()), before: { onGiveUp?() }); result.cancel() }              // a path change, a withdrawal or a reset ends the call at once too
        noteProgress()
        let watch = Task { [self] in
            while !Task.isCancelled {
                try? await Task.sleep(nanoseconds: UInt64(max(0.02, min(1.0, silence / 4)) * 1e9))
                lock.lock(); let quiet = Date().timeIntervalSince(progressAt); lock.unlock()
                if quiet > silence { gate.finish(.failure(Refusal.timedOut), before: { onGiveUp?(); kill(); result.cancel() }); return }
            }
        }
        Task { let r = await result.result; watch.cancel(); gate.finish(r); onBodyEnded?() }
        return try await withTaskCancellationHandler {
            try await withCheckedThrowingContinuation { (k: CheckedContinuation<T, Error>) in gate.set(k) }
        } onCancel: { gate.finish(.failure(CancellationError()), before: { onGiveUp?(); kill(); result.cancel() }) }
    }

    /// Models whose cold-load body outlived the hard cap: its slot was released, so nothing wedges, but its windows are refused (and the registry stays closed) until the body ends.
    private var stuck: [String: Int] = [:]
    private final class Release: @unchecked Sendable { let l = NSLock(); var done = false; func once() -> Bool { l.lock(); defer { l.unlock() }; if done { return false }; done = true; return true } }
    private func stickOn(_ model: String) { lock.lock(); stuck[model, default: 0] += 1; lock.unlock() }
    private func unstick(_ model: String) { lock.lock(); if let n = stuck[model] { stuck[model] = n > 1 ? n - 1 : nil }; lock.unlock() }
    private func isStuck(_ model: String) -> Bool { lock.lock(); defer { lock.unlock() }; return stuck[model] != nil }

    /// Run `body` holding the slot with NO window open anywhere (the registry stays closed): what a check needs, so that its load is a cold load that cannot fetch and cannot borrow another model's window. It has the same silence deadline.
    /// The slot is freed when the body has ended; a body that never ends frees it at the hard cap (twice the deadline) and its `model` stays refused until it does (never released with the registry open).
    public func exclusive<T: Sendable>(model: String = "*", _ body: @escaping @Sendable () async throws -> T) async throws -> T {
        try await acquireSlot(every: 2, whileQueued: nil)
        if isStuck(model) { releaseSlot(); throw Refusal.timedOut }            // a second cold load beside the one that is still running
        let rel = Release()
        let cap = Task { [self] in
            try? await Task.sleep(nanoseconds: UInt64(deadlines.body * 2 * 1e9))
            if !Task.isCancelled, rel.once() { stickOn(model); closeNow(); releaseSlot() }
        }
        return try await raced(body, onBodyEnded: { [self] in
            cap.cancel()
            if rel.once() { releaseSlot() } else { unstick(model) }          // it ended after the cap released the slot: the model is allowed again
        }, kill: { })
    }

    public func libraryWindow<T: Sendable>(model id: String, queuedEvery: TimeInterval = 2, whileQueued: (@Sendable () -> Void)? = nil, onKilled: (@Sendable () -> Void)? = nil, _ body: @escaping @Sendable () async throws -> T) async throws -> T {
        if isStuck(id) || isStuck("*") { throw Refusal.timedOut }          // its last cold load has not ended yet
        let early = try authorize(model: id)                             // an early, cheap refusal (the authoritative look is inside the lock, after the wait)
        if early == .wifi, !allowAll, !onWiFi() { throw Refusal.pathNotAgreed(group: groups[id] ?? "") }
        try await acquireSlot(every: queuedEvery, whileQueued: whileQueued); defer { releaseSlot() }
        let key = UUID(), box = CancelBox(); box.onKilled = onKilled
        try enterWindow(key, model: id, box)                            // the authoritative check, under the lock that opens the registry
        defer { leaveWindow(key) }
        // A body past its silence deadline, or a cancelled caller, fails the call at once: the window is killed (the registry closed whatever the library does) and the call does not wait for a body that ignores cancellation.
        // A body given up on (deadline, cancel) that still runs keeps ITS model refused until it ends, as a cold load does.
        let gaveUp = Release()
        return try await raced({ if box.killed { throw CancellationError() }; return try await body() }, box: box,
                               onBodyEnded: { [self] in if !gaveUp.once() { unstick(id) } },
                               onGiveUp: { [self] in if gaveUp.once() { stickOn(id) } },
                               kill: { [self] in box.kill(); closeNow() })
    }

    private func closeNow() { lock.lock(); let hook = registryHook; lock.unlock(); hook?(Self.closedRegistry) }

    private func enterWindow(_ key: UUID, model id: String, _ box: CancelBox) throws {
        lock.lock()
        if stuck[id] != nil || stuck["*"] != nil { lock.unlock(); throw Refusal.timedOut }          // checked again under the lock, after the wait: a body that outlived its cap is still running
        let a: Agreement, g: String
        if allowAll { a = .mobile; g = groups[id] ?? "" }
        else {
            guard let grp = groups[id] else { lock.unlock(); throw Refusal.notOffered(model: id) }
            let key = agreed[id] != nil ? id : grp                                                 // the model's own agreement, else its group's
            guard let ag = agreed[key] else { lock.unlock(); throw Refusal.noConsent(group: grp) }
            if ag == .wifi, !onWiFi() { lock.unlock(); throw Refusal.pathNotAgreed(group: grp) }
            a = ag; g = key
        }
        live[key] = Window(group: g, agreement: a, cancel: { box.kill() })
        let hook = registryHook
        hook?(Self.openRegistry)                                         // under the lock: nothing can slip between the check and the open
        lock.unlock()
    }
    private func leaveWindow(_ key: UUID) {
        lock.lock(); live[key] = nil; let none = live.isEmpty; let hook = registryHook; lock.unlock()
        if none { hook?(Self.closedRegistry) }                           // idempotent: a killed window already closed it
    }

    /// Kill every window the present facts no longer cover: close the registry NOW (whatever the library does about cancellation), then cancel the transfers. Called by the path monitor, by `sync` and by `reset`.
    private func enforce(wifi known: Bool? = nil) {
        lock.lock()
        let wifi = known ?? onWiFi()
        let doomed = live.values.filter { w in
            guard let now = agreed[w.group] else { return true }                                  // withdrawn
            return now == .wifi && !wifi                                                          // off Wi-Fi under a Wi-Fi agreement (a mobile agreement is not cut by a path)
        }
        let hook = registryHook; let kill = doomed.map { $0.cancel }
        if !kill.isEmpty { hook?(Self.closedRegistry) }
        lock.unlock()
        for k in kill { k() }
    }
    /// The phone's path changed (WiFiMonitor).
    public func pathChanged(onWiFi now: Bool) { enforce(wifi: now) }
}
