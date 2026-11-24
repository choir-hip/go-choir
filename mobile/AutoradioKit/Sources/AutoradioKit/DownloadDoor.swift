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
    // window, which exists only with consent for the model's group and, under a Wi-Fi agreement, only on Wi-Fi, and which is CANCELLED the moment the path leaves Wi-Fi (the library's transfer sees its Task cancelled and drops the connection).
    // Everything else the library does (load from the folder, synthesise, transcribe) is local: with the registry closed a library asked to initialise for files that are not there fails; it never fetches.
    public static let closedRegistry = "https://models.invalid"
    public static let openRegistry = "https://huggingface.co"
    private var registryHook: (@Sendable (String) -> Void)?
    private var windows = 0

    /// The models module installs the way to point the library's registry; it is closed at once.
    public func installLibraryGuard(_ setRegistry: @escaping @Sendable (String) -> Void) {
        lock.lock(); registryHook = setRegistry; lock.unlock()
        setRegistry(Self.closedRegistry)
    }

    public func libraryWindow<T: Sendable>(model id: String, _ body: @escaping @Sendable () async throws -> T) async throws -> T {
        let a = try authorize(model: id)
        if a == .wifi, !allowAll, !onWiFi() { throw Refusal.pathNotAgreed(group: groups[id] ?? "") }
        let key = UUID()
        let task = Task { try await body() }
        enterWindow(key, a, { task.cancel() })
        defer { leaveWindow(key) }
        return try await withTaskCancellationHandler { try await task.value } onCancel: { task.cancel() }
    }
    private func enterWindow(_ key: UUID, _ a: Agreement, _ cancel: @escaping () -> Void) {
        lock.lock(); windows += 1; let first = windows == 1; let hook = registryHook; cancels[key] = (a, cancel); lock.unlock()
        if first { hook?(Self.openRegistry) }
    }
    private func leaveWindow(_ key: UUID) {
        lock.lock(); cancels[key] = nil; windows -= 1; let last = windows == 0; let hook = registryHook; lock.unlock()
        if last { hook?(Self.closedRegistry) }
    }
    private var cancels: [UUID: (Agreement, () -> Void)] = [:]

    /// The phone's path changed (WiFiMonitor). Off Wi-Fi, every library transfer running under a Wi-Fi agreement is cut.
    public func pathChanged(onWiFi now: Bool) {
        guard !now else { return }
        lock.lock(); let cut = cancels.values.filter { $0.0 == .wifi }.map { $0.1 }; lock.unlock()
        for c in cut { c() }
    }
}
