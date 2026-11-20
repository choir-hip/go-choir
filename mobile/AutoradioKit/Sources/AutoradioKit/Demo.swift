import Foundation

// Demo mode: the app signed in as a demo listener, served from bundled fixtures, with no network and no microphone
// (the engine is already virtual under the Exact agent). Every lane builds and proves its screens on the simulator
// against this. Fixtures live in mobile/fixtures/demo/*.json and are embedded by scripts/embed-assets.py.
//
//   Turn it on (simulator):  xcrun simctl spawn <udid> defaults write news.choir.autoradio.lab AutoradioDemo -bool YES
//   Turn it off:             ... defaults delete news.choir.autoradio.lab AutoradioDemo
//   or at run time:          the `demoSet` send (Components page, "Demo mode"), then relaunch.
//
// What is stubbed: GET /api/platform/publications/front, /resolve and /api/compute/status (from fixtures); every other
// route answers 404 {"error":"demo mode serves fixtures only"} so a lane that needs a route adds a fixture here
// (add the file under mobile/fixtures/demo/ and one `case` in DemoURLProtocol.route, nowhere else).

public enum Demo {
    public static let defaultsKey = "AutoradioDemo"

    /// Demo mode is a proof aid for the simulator and the Mac. On a real phone it is never on: the owner's iPhone
    /// carried a stored flag across every reinstall (2026-11-20) and showed sample stories with no way to sign in.
    /// Native because only native code knows it runs on a physical device, and it is read before any page exists.
    public static var allowedHere: Bool {
        #if os(iOS) && !targetEnvironment(simulator)
        return false
        #else
        return true
        #endif
    }
    public static var isOn: Bool { isOn(allowed: allowedHere) }
    /// The decision, with the device fact passed in so it can be tested. A stored flag on a device that does not
    /// allow demo mode is removed, so it cannot come back.
    static func isOn(allowed: Bool, defaults: UserDefaults = .standard, env: [String: String] = ProcessInfo.processInfo.environment) -> Bool {
        guard allowed else {
            if defaults.object(forKey: defaultsKey) != nil { defaults.removeObject(forKey: defaultsKey) }
            return false
        }
        return defaults.bool(forKey: defaultsKey) || env["AUTORADIO_DEMO"] == "1"
    }
    public static func setOn(_ on: Bool) { setOn(on, allowed: allowedHere) }
    static func setOn(_ on: Bool, allowed: Bool, defaults: UserDefaults = .standard) {
        if on && allowed { defaults.set(true, forKey: defaultsKey) } else { defaults.removeObject(forKey: defaultsKey) }
    }

    /// A key store already signed in as the demo listener (the key is a placeholder; nothing leaves the phone).
    public static func keys() -> KeyStore {
        InMemoryKeyStore([KeyAccount.apiKey: "choir_sk_demo", KeyAccount.computerID: "demo-computer", KeyAccount.keyID: "demo-key"])
    }

    /// A URLSession whose every request is answered from the fixtures.
    public static func session() -> URLSession {
        let c = URLSessionConfiguration.ephemeral
        c.protocolClasses = [DemoURLProtocol.self]
        return URLSession(configuration: c)
    }

    /// Wire the shared Choir environment to the fixtures. Call before ChoirApp is made.
    public static func install(into env: ChoirEnvironment = .shared) {
        env.session = session()
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

    /// The demo front page as stories, for pages that list them without a request.
    public static func stories() -> [ChoirStory] {
        guard let d = fixture("front"), let o = try? JSONSerialization.jsonObject(with: d) as? [String: Any],
              let list = o["stories"] as? [[String: Any]] else { return [] }
        return list.compactMap { s in
            guard let title = s["title"] as? String, let route = (s["route"] as? [String: Any])?["bare"] as? String else { return nil }
            return ChoirStory(title: title, dek: s["dek"] as? String ?? "", route: route, section: s["section"] as? String ?? "",
                              words: s["words"] as? Int ?? 0, updated: s["updated_at"] as? String ?? "")
        }
    }
}

final class DemoURLProtocol: URLProtocol {
    override class func canInit(with request: URLRequest) -> Bool { true }
    override class func canonicalRequest(for request: URLRequest) -> URLRequest { request }
    override func stopLoading() {}

    override func startLoading() {
        let url = request.url ?? URL(string: "https://demo.invalid/")!
        // The programme route (station P1): a short stretch in the wire shape, so the simulator plays segments with no computer.
        if url.path == "/api/autoradio/programme", request.httpMethod == "POST" {
            let resp = HTTPURLResponse(url: url, statusCode: 200, httpVersion: "HTTP/1.1", headerFields: ["Content-Type": "text/event-stream"])!
            client?.urlProtocol(self, didReceive: resp, cacheStoragePolicy: .notAllowed)
            var sent = request.httpBody ?? Data()
            if let st = request.httpBodyStream { st.open(); var buf = [UInt8](repeating: 0, count: 4096)
                while st.hasBytesAvailable { let n = st.read(&buf, maxLength: buf.count); if n <= 0 { break }; sent.append(buf, count: n) }; st.close() }
            let cause = ((try? JSONSerialization.jsonObject(with: sent)) as? [String: Any])?["cause"] as? String ?? "open"
            client?.urlProtocol(self, didLoad: Data(DemoStretch.body(cause: cause).utf8))
            client?.urlProtocolDidFinishLoading(self)
            return
        }
        let items = URLComponents(url: url, resolvingAgainstBaseURL: false)?.queryItems ?? []
        var sent = request.httpBody ?? Data()
        if let st = request.httpBodyStream { st.open(); var buf = [UInt8](repeating: 0, count: 4096)
            while st.hasBytesAvailable { let n = st.read(&buf, maxLength: buf.count); if n <= 0 { break }; sent.append(buf, count: n) }; st.close() }
        // A Texture revision posted from the phone's editor: demo mode keeps it and answers the new head (TextureDemo.posted).
        if request.httpMethod == "POST", let answer = TextureDemo.post(path: url.path, body: sent) {
            let resp = HTTPURLResponse(url: url, statusCode: 201, httpVersion: "HTTP/1.1", headerFields: ["Content-Type": "application/json"])!
            client?.urlProtocol(self, didReceive: resp, cacheStoragePolicy: .notAllowed)
            client?.urlProtocol(self, didLoad: answer)
            client?.urlProtocolDidFinishLoading(self)
            return
        }
        if let (status, body) = TakesDemo.answer(method: request.httpMethod ?? "GET", path: url.path + ((url.query ?? "").contains("mine=1") ? "?mine" : ""), body: sent) {
            let resp = HTTPURLResponse(url: url, statusCode: status, httpVersion: "HTTP/1.1", headerFields: ["Content-Type": "application/json"])!
            client?.urlProtocol(self, didReceive: resp, cacheStoragePolicy: .notAllowed)
            client?.urlProtocol(self, didLoad: body)
            client?.urlProtocolDidFinishLoading(self)
            return
        }
        let (status, body) = Self.route(path: url.path, query: items)
        let resp = HTTPURLResponse(url: url, statusCode: status, httpVersion: "HTTP/1.1", headerFields: ["Content-Type": "application/json"])!
        client?.urlProtocol(self, didReceive: resp, cacheStoragePolicy: .notAllowed)
        client?.urlProtocol(self, didLoad: body)
        client?.urlProtocolDidFinishLoading(self)
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
