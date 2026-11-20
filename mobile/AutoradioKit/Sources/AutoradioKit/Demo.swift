import Foundation

// Demo mode: the app signed in as a demo listener, served from bundled fixtures, with no network and no microphone
// (the engine is already virtual under the Exact agent). Every lane builds and proves its screens on the simulator
// against this. Fixtures live in mobile/fixtures/demo/*.json and are embedded by scripts/embed-assets.py.
//
//   On (simulator): xcrun simctl spawn <udid> defaults write news.choir.autoradio.lab AutoradioDemo -bool YES; off: defaults delete.
//   No control in the app turns it on, and on a physical phone it is never on (see `allowedHere`).
// What is stubbed: GET /api/platform/publications/front, /resolve and /api/compute/status (from fixtures); every other
// route answers 404 {"error":"demo mode serves fixtures only"} so a lane that needs a route adds a fixture here
// (add the file under mobile/fixtures/demo/ and one `case` in DemoURLProtocol.route, nowhere else).

public enum Demo {
    public static let defaultsKey = "AutoradioDemo"

    /// A proof aid for the simulator and the Mac; never on a real phone (the owner's iPhone, stuck in it, 2026-11-20).
    /// Native: only native code knows it runs on a device, and it is read before any page exists.
    public static var allowedHere: Bool {
        #if os(macOS) || targetEnvironment(simulator)
        return true
        #else
        return false          // a phone or any platform not named: closed
        #endif
    }
    public static var isOn: Bool { isOn(allowed: allowedHere) }
    /// The decision, with the device fact passed in for tests. Where demo is not allowed a stored flag is removed.
    static func isOn(allowed: Bool, defaults: UserDefaults = .standard, env: [String: String] = ProcessInfo.processInfo.environment) -> Bool {
        guard allowed else {
            if defaults.object(forKey: defaultsKey) != nil { defaults.removeObject(forKey: defaultsKey) }
            return false
        }
        return defaults.bool(forKey: defaultsKey) || env["AUTORADIO_DEMO"] == "1"
    }
    static let clearedKey = "AutoradioDemoLeftoversCleared"          // recorded once `clearLeftovers` has run on this device

    /// Native because the platform forces it: this removes on-disk state the native code wrote (the shelf, the stores' files).
    /// Conservative: the sample book and `demo-` addresses go always; onboarding, edition topics and voice setup carry no mark of
    /// origin, so they go only when no real key is stored; `recent-files.json` stays. The Keychain is never touched.
    /// Once per device (demo not allowed), whatever the stored flag was; never on the simulator or the Mac.
    public static func clearLeftovers(support: URL, hasRealKey: Bool, allowed: Bool = allowedHere, defaults: UserDefaults = .standard) {
        guard !allowed, !defaults.bool(forKey: clearedKey) else { return }; defaults.set(true, forKey: clearedKey)
        ReaderShelf(root: support.appendingPathComponent("books")).remove(id: ReaderSample.id)
        if !hasRealKey { AccountWipe.removeFiles(["onboarding.json", "edition-topics.json", "voice-setup.json"].map { support.appendingPathComponent($0) }) }
        let file = support.appendingPathComponent("open-pages.json")
        if let d = try? Data(contentsOf: file), var o = try? JSONSerialization.jsonObject(with: d) as? [String: Any] {
            o["pages"] = ((o["pages"] as? [[String: Any]]) ?? []).filter { !isDemoAddress($0["address"] as? String ?? "") }
            o["recency"] = ((o["recency"] as? [String]) ?? []).filter { !isDemoAddress($0) }
            try? JSONSerialization.data(withJSONObject: o).write(to: file, options: .atomic)
        }
        let log = support.appendingPathComponent("signals.jsonl")
        if let text = try? String(contentsOf: log, encoding: .utf8) {
            let keep = text.split(separator: "\n").filter { (try? JSONDecoder().decode(SignalEvent.self, from: Data($0.utf8))).map { !isDemoAddress($0.subject) } ?? true }
            try? (keep.map { $0 + "\n" }.joined()).write(to: log, atomically: true, encoding: .utf8)
        }
    }

    /// `pub:demo-x`, `pub:/demo-x`, `pub:/demo-x#b2`: what follows the scheme starts with `demo-`.
    static func isDemoAddress(_ a: String) -> Bool {
        a.firstIndex(of: ":").map { a[a.index(after: $0)...].drop { $0 == "/" }.lowercased().hasPrefix("demo-") } ?? false
    }

    /// A key store already signed in as the demo listener (the key is a placeholder; nothing leaves the phone).
    public static func keys() -> KeyStore {
        InMemoryKeyStore([KeyAccount.apiKey: "choir_sk_demo", KeyAccount.computerID: "demo-computer", KeyAccount.keyID: "demo-key"])
    }

    /// Wire the shared Choir environment to the fixtures (a session whose every request is answered from them). Call before ChoirApp is made.
    public static func install(into env: ChoirEnvironment = .shared) {
        let c = URLSessionConfiguration.ephemeral
        c.protocolClasses = [DemoURLProtocol.self]
        env.session = URLSession(configuration: c)
        env.keys = keys()
    }

    /// TEST AFFORDANCE, inside demo mode only (not a product switch: nothing ships on or off by it). The launch default `AutoradioDemoSheet <name>`
    /// (set like `AutoradioDemo`) loads the bundled fixture `runsheet-<name>` as the run sheet for this launch: the station does not start by itself
    /// and the desk is not asked, so a tour photographs exactly that sheet. Names: cold, day, answer, empty (fixtures/demo/runsheet-<name>.json).
    public static let sheetKey = "AutoradioDemoSheet"
    public static var sheetName: String? {
        guard isOn, let n = UserDefaults.standard.string(forKey: sheetKey), !n.isEmpty else { return nil }
        return n
    }
    @MainActor public static func runSheet(_ name: String) -> RunSheet? {
        guard let d = fixture("runsheet-" + name) else { return nil }
        return lossySheet(d)
    }

    /// The sheet of a saved file, entry by entry: an entry that does not decode is dropped and the rest load (one bad entry never empties the page).
    @MainActor static func lossySheet(_ data: Data) -> RunSheet? {
        guard let top = try? JSONSerialization.jsonObject(with: data) as? [String: Any], var sheet = top["sheet"] as? [String: Any] else { return nil }
        let raw = (sheet["entries"] as? [Any]) ?? []
        let good: [Any] = raw.filter { e in
            guard JSONSerialization.isValidJSONObject(e), let d = try? JSONSerialization.data(withJSONObject: e) else { return false }
            return (try? JSONDecoder().decode(RunSheetEntry.self, from: d)) != nil
        }
        sheet["entries"] = good
        guard let d = try? JSONSerialization.data(withJSONObject: sheet) else { return nil }
        return try? JSONDecoder().decode(RunSheet.self, from: d)
    }

    public static func fixture(_ name: String) -> Data? { DemoFixtureData.files[name].map { Data($0.utf8) } }
}

final class DemoURLProtocol: URLProtocol {
    override class func canInit(with request: URLRequest) -> Bool { true }
    override class func canonicalRequest(for request: URLRequest) -> URLRequest { request }
    override func stopLoading() {}

    override func startLoading() {
        let url = request.url ?? URL(string: "https://demo.invalid/")!
        var sent = request.httpBody ?? Data()
        if let st = request.httpBodyStream { st.open(); var buf = [UInt8](repeating: 0, count: 4096)
            while st.hasBytesAvailable { let n = st.read(&buf, maxLength: buf.count); if n <= 0 { break }; sent.append(buf, count: n) }; st.close() }
        func reply(_ status: Int, _ body: Data, _ type: String = "application/json") {
            client?.urlProtocol(self, didReceive: HTTPURLResponse(url: url, statusCode: status, httpVersion: "HTTP/1.1", headerFields: ["Content-Type": type])!, cacheStoragePolicy: .notAllowed)
            client?.urlProtocol(self, didLoad: body)
            client?.urlProtocolDidFinishLoading(self)
        }
        // The programme route (station P1): a short stretch in the wire shape, so the simulator plays segments with no computer.
        if url.path == "/api/autoradio/programme", request.httpMethod == "POST" {
            let cause = ((try? JSONSerialization.jsonObject(with: sent)) as? [String: Any])?["cause"] as? String ?? "open"
            return reply(200, Data(DemoStretch.body(cause: cause).utf8), "text/event-stream")
        }
        // A Texture revision posted from the phone's editor: demo mode keeps it and answers the new head (TextureDemo.posted).
        if request.httpMethod == "POST", let answer = TextureDemo.post(path: url.path, body: sent) { return reply(201, answer) }
        if let (status, body) = TakesDemo.answer(method: request.httpMethod ?? "GET", path: url.path + ((url.query ?? "").contains("mine=1") ? "?mine" : ""), body: sent) { return reply(status, body) }
        let (status, body) = Self.route(path: url.path, query: URLComponents(url: url, resolvingAgainstBaseURL: false)?.queryItems ?? [])
        reply(status, body)
    }

    /// One case per stubbed route. A lane adds its fixture file and a case here.
    static func route(path: String, query: [URLQueryItem]) -> (Int, Data) {
        func q(_ n: String) -> String { query.first { $0.name == n }?.value ?? "" }
        switch path {
        case "/api/platform/publications/front": if let d = Demo.fixture("front") { return (200, d) }
        case "/api/compute/status": if let d = Demo.fixture("status") { return (200, d) }
        case "/api/autoradio/learning": if let d = Demo.fixture("learning") { return (200, d) }          // station L6: the scorecard (a fixture; the real route is the server's)
        case "/api/texture/edition": if let d = Demo.fixture("texture-edition") { return (200, d) }      // the demo notebook
        case "/api/autoradio/steer": return (200, Data("{}".utf8))
        case "/api/platform/publications/resolve": if let d = Demo.fixture("resolve-" + TextureDemo.bareRoute(q("route"))) { return (200, d) }
        case "/api/platform/publications/related": if let d = Demo.fixture("related-" + TextureDemo.bareRoute(q("route"))) { return (200, d) }
        default: if let d = TextureDemo.route(path) { return (200, d) }
        }
        return (404, Data(#"{"error":"demo mode serves fixtures only"}"#.utf8))
    }
}

/// `pub:<route>` as plain paragraphs, from the network, or the fixtures in demo mode (the session is the stub).
/// The spine's text renderer shows it; lane 3 replaces it with the real reader and its own source.
public struct PublicationTextSource: PageSource {
    nonisolated(unsafe) public static var offline: (@Sendable (String) -> ReaderDoc?)?
    public init() {}
    public func handles(_ address: String) -> Bool { address.hasPrefix("pub:") }
    public func load(_ address: String) async throws -> PageModel {
        let route = PageAddress.rest(of: address)
        let o: [String: Any]
        do {
            o = try await ChoirEnvironment.shared.client().send("GET", "/api/platform/publications/resolve", query: ["route": route],
                                                                  authenticated: false, route: "/api/platform/publications/resolve").object()
        } catch {
            // No network: the story the phone holds (the ContentStore) is read from there.
            if let d = Self.offline?(address) { return PageModel(address: address, kind: "reader", title: d.title, status: "ready", lines: d.lines) }
            throw error
        }
        let title = ((o["publication"] as? [String: Any])?["title"] as? String) ?? "Story"
        let text = ((o["payload"] as? [String: Any])?["text"] as? String) ?? ""
        return PageModel(address: address, kind: "reader", title: title, status: "ready", lines: ChoirText.paragraphs(text))
    }
}
