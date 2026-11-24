import Foundation

// DOOR A: the ONE way a model file reaches the phone (review round 3; docs/problems/onboard-consent-rules-escape-by-another-path-2026-11-24.md).
// Every fetch in the app (setup, Settings > Downloads, the voice cards, a retry, a resume, the search pass, the voice print) asks this door for its transport, and the door
//  - refuses a model whose group the person has not agreed to (the agreement is Exact's consent record, pushed here by shared code: `sync`), and a model that is in no group (a model that is not offered cannot be fetched);
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
        case noConsent(group: String), notOffered(model: String), pathNotAgreed(group: String)
        /// What the person is told (the voice cards and the rows say this, not the code's words).
        public var person: String {
            switch self {
            case .noConsent: return "Agree to this download in Settings, Downloads first: that is where you see its size and who it comes from."
            case .notOffered: return "This is not offered for download."
            case .pathNotAgreed: return "This needs Wi-Fi: you agreed to it on Wi-Fi only. It will go on when you are back on Wi-Fi."
            }
        }
        public var description: String {
            switch self {
            case .noConsent(let g): return "the person has not agreed to the \(g) downloads"
            case .notOffered(let m): return "\(m) is not offered for download"
            case .pathNotAgreed(let g): return "the \(g) downloads were agreed to on Wi-Fi only and the phone is not on Wi-Fi"
            }
        }
    }

    public static let shared = DownloadDoor()
    private let lock = NSLock()
    private var agreed: [String: Agreement] = [:]
    private let onWiFi: @Sendable () -> Bool
    private let allowAll: Bool
    private let groups: [String: String]

    public init(onWiFi: @escaping @Sendable () -> Bool = { WiFiMonitor.shared.onWiFi }, list: ModelList = .builtIn()) {
        self.onWiFi = onWiFi; self.allowAll = false; self.groups = Self.groups(of: list)
    }
    /// A store used by tests with its own fake transport, which is not a model of the app: everything passes on the transport it was given.
    init(allowAll: Bool) { self.onWiFi = { true }; self.allowAll = allowAll; self.groups = [:] }
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
        lock.lock(); let a = agreed[g]; lock.unlock()
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
    private var slotWaiters: [CheckedContinuation<Void, Never>] = []
    public var openWindowCount: Int { lock.lock(); defer { lock.unlock() }; return live.count }

    /// The models module installs the way to point the library's registry; it is closed at once (and stays closed while a window is open: installing again must not cut or reopen one).
    public func installLibraryGuard(_ setRegistry: @escaping @Sendable (String) -> Void) {
        lock.lock(); registryHook = setRegistry; let none = live.isEmpty; lock.unlock()
        if none { setRegistry(Self.closedRegistry) }
    }

    private final class CancelBox: @unchecked Sendable { let lock = NSLock(); var cancel: (@Sendable () -> Void)?; var killed = false
        func kill() { lock.lock(); killed = true; let c = cancel; lock.unlock(); c?() }
        func set(_ c: @escaping @Sendable () -> Void) { lock.lock(); cancel = c; let k = killed; lock.unlock(); if k { c() } } }

    private func acquireSlot() async {
        await withCheckedContinuation { (c: CheckedContinuation<Void, Never>) in
            lock.lock()
            if !slotBusy { slotBusy = true; lock.unlock(); c.resume() } else { slotWaiters.append(c); lock.unlock() }
        }
    }
    private func releaseSlot() {
        lock.lock()
        if slotWaiters.isEmpty { slotBusy = false; lock.unlock() } else { let c = slotWaiters.removeFirst(); lock.unlock(); c.resume() }
    }

    public func libraryWindow<T: Sendable>(model id: String, _ body: @escaping @Sendable () async throws -> T) async throws -> T {
        _ = try authorize(model: id)                                     // an early, cheap refusal
        await acquireSlot(); defer { releaseSlot() }
        let key = UUID(), box = CancelBox()
        try enterWindow(key, model: id, box)                            // the authoritative check, under the lock that opens the registry
        defer { leaveWindow(key) }
        let result = Task<T, Error> { if box.killed { throw CancellationError() }; return try await body() }
        box.set { result.cancel() }                                      // killing cancels the transfer's Task
        return try await withTaskCancellationHandler { try await result.value } onCancel: { result.cancel(); box.kill() }
    }

    private func enterWindow(_ key: UUID, model id: String, _ box: CancelBox) throws {
        lock.lock()
        let a: Agreement, g: String
        if allowAll { a = .mobile; g = groups[id] ?? "" }
        else {
            guard let grp = groups[id] else { lock.unlock(); throw Refusal.notOffered(model: id) }
            guard let ag = agreed[grp] else { lock.unlock(); throw Refusal.noConsent(group: grp) }
            if ag == .wifi, !onWiFi() { lock.unlock(); throw Refusal.pathNotAgreed(group: grp) }
            a = ag; g = grp
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
