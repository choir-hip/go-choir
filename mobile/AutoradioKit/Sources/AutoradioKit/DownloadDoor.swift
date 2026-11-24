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
//  A4 A library download on a path the agreement does not cover is refused at the start.
//  A5 Sign-out (an empty record) closes the door.
public final class DownloadDoor: @unchecked Sendable {
    public enum Agreement: String, Sendable, Equatable { case wifi, mobile }
    public enum Refusal: Error, Equatable, CustomStringConvertible {
        case noConsent(group: String), notOffered(model: String), pathNotAgreed(group: String)
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
        c.allowsCellularAccess = cellular; c.allowsExpensiveNetworkAccess = cellular; c.allowsConstrainedNetworkAccess = cellular
        c.waitsForConnectivity = false
        return c
    }

    /// A library that downloads on its own session: refused without consent, and refused on a path the agreement does not cover (the library's session cannot be configured, so the check is at the start).
    public func authorizeLibrary(model id: String) throws {
        let a = try authorize(model: id)
        if a == .wifi, !allowAll, !onWiFi() { throw Refusal.pathNotAgreed(group: groups[id] ?? "") }
    }
}
