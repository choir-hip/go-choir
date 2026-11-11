import Foundation
#if canImport(Security)
import Security
#endif

// The phone as a client of the listener's computer. Nothing here prints, logs or records the key.

// MARK: Key storage

/// Where the device key lives. The key is a secret: implementations never expose it except through `get`.
public protocol KeyStore: AnyObject, Sendable {
    func get(_ account: String) -> String?
    func set(_ value: String, account: String) throws
    func delete(_ account: String)
}

public enum KeyAccount {
    public static let apiKey = "api-key"
    /// The key's server-side id (not a secret): lets Sign out revoke it.
    public static let keyID = "key-id"
    /// The listener's computer id (not a secret). The device-flow key is owner-wide, so every request must name it.
    public static let computerID = "computer-id"
}

/// Tests and one-shot commands.
public final class InMemoryKeyStore: KeyStore, @unchecked Sendable {
    private let lock = NSLock()
    private var v: [String: String] = [:]
    public init(_ initial: [String: String] = [:]) { v = initial }
    public func get(_ account: String) -> String? { lock.lock(); defer { lock.unlock() }; return v[account] }
    public func set(_ value: String, account: String) throws { lock.lock(); v[account] = value; lock.unlock() }
    public func delete(_ account: String) { lock.lock(); v[account] = nil; lock.unlock() }
}

/// macOS replay: a directory of one file per account (writable), or a single key file (`readOnlyKeyFile`)
/// whose trimmed contents are the API key. Files are created 0600.
public final class FileKeyStore: KeyStore, @unchecked Sendable {
    let url: URL
    let readOnlyKeyFile: Bool
    private let lock = NSLock()
    public init(url: URL, readOnlyKeyFile: Bool = false) { self.url = url; self.readOnlyKeyFile = readOnlyKeyFile }
    private func file(_ account: String) -> URL { readOnlyKeyFile ? url : url.appendingPathComponent(account) }
    public func get(_ account: String) -> String? {
        lock.lock(); defer { lock.unlock() }
        if readOnlyKeyFile && account != KeyAccount.apiKey { return nil }
        guard let d = try? Data(contentsOf: file(account)), let s = String(data: d, encoding: .utf8) else { return nil }
        let t = s.trimmingCharacters(in: .whitespacesAndNewlines)
        return t.isEmpty ? nil : t
    }
    public func set(_ value: String, account: String) throws {
        if readOnlyKeyFile { throw LabError.unavailable("key file is read-only") }
        lock.lock(); defer { lock.unlock() }
        try FileManager.default.createDirectory(at: url, withIntermediateDirectories: true, attributes: [.posixPermissions: 0o700])
        let f = file(account)
        try Data(value.utf8).write(to: f, options: .atomic)
        try FileManager.default.setAttributes([.posixPermissions: 0o600], ofItemAtPath: f.path)
    }
    public func delete(_ account: String) {
        if readOnlyKeyFile { return }
        lock.lock(); defer { lock.unlock() }
        try? FileManager.default.removeItem(at: file(account))
    }
}

/// Reads from a base store (a read-only key file) and keeps everything written in memory only.
public final class OverlayKeyStore: KeyStore, @unchecked Sendable {
    let base: KeyStore
    let mem = InMemoryKeyStore()
    public init(base: KeyStore) { self.base = base }
    public func get(_ account: String) -> String? { mem.get(account) ?? base.get(account) }
    public func set(_ value: String, account: String) throws { try mem.set(value, account: account) }
    public func delete(_ account: String) { mem.delete(account) }
}

#if canImport(Security)
/// The iOS (and macOS) Keychain. Device-only, available after first unlock so audio can run with the screen locked.
public final class KeychainKeyStore: KeyStore, @unchecked Sendable {
    let service: String
    public init(service: String = "news.choir.autoradio.choir") { self.service = service }
    private func base(_ account: String) -> [String: Any] {
        [kSecClass as String: kSecClassGenericPassword, kSecAttrService as String: service, kSecAttrAccount as String: account]
    }
    public func get(_ account: String) -> String? {
        var q = base(account); q[kSecReturnData as String] = true; q[kSecMatchLimit as String] = kSecMatchLimitOne
        var out: AnyObject?
        guard SecItemCopyMatching(q as CFDictionary, &out) == errSecSuccess, let d = out as? Data else { return nil }
        return String(data: d, encoding: .utf8)
    }
    public func set(_ value: String, account: String) throws {
        SecItemDelete(base(account) as CFDictionary)
        var add = base(account)
        add[kSecValueData as String] = Data(value.utf8)
        add[kSecAttrAccessible as String] = kSecAttrAccessibleAfterFirstUnlockThisDeviceOnly
        let st = SecItemAdd(add as CFDictionary, nil)
        if st != errSecSuccess { throw LabError.unavailable("keychain refused the write (\(st))") }
    }
    public func delete(_ account: String) { SecItemDelete(base(account) as CFDictionary) }
}
#endif

// MARK: Paths and hosts

/// One id as one path segment: everything but unreserved characters is percent-encoded, so an id can never add a "/" (a second
/// segment), a "?" or "#", and the segments "." and ".." cannot climb out of the route. Total: it has no failure to force-unwrap.
public enum PathSegment {
    private static let unreserved = CharacterSet(charactersIn: "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-._~")
    public static func encode(_ s: String) -> String {
        if s == "." { return "%2E" }
        if s == ".." { return "%2E%2E" }
        return s.addingPercentEncoding(withAllowedCharacters: unreserved) ?? s.unicodeScalars.map { String(format: "%%%02X", $0.value & 0xFF) }.joined()
    }
}

/// The one check before a credential is sent: the same scheme, host and port as the Choir host the app is configured for, and no
/// user-info in the URL. A bearer key goes only where this says yes.
public enum SameOrigin {
    static func port(_ u: URL) -> Int? { u.port ?? (u.scheme?.lowercased() == "https" ? 443 : (u.scheme?.lowercased() == "http" ? 80 : nil)) }
    public static func matches(_ a: URL, _ b: URL) -> Bool {
        guard let sa = a.scheme?.lowercased(), let sb = b.scheme?.lowercased(), sa == sb, sa == "https" || sa == "http",
              let ha = a.host?.lowercased(), let hb = b.host?.lowercased(), !ha.isEmpty, ha == hb,
              let pa = port(a), let pb = port(b), pa == pb, a.user == nil, a.password == nil else { return false }
        return true
    }
    public static func matches(_ s: String, _ b: URL) -> Bool { URL(string: s).map { matches($0, b) } ?? false }
}

// MARK: Errors

public enum ChoirError: Error, CustomStringConvertible, Sendable, Equatable {
    case notSignedIn
    case unauthorized
    case forbidden(String)
    case rateLimited(retryAfter: Double?)
    case server(status: Int, message: String)
    case transport(String)
    case timeout(String)
    case decode(String)
    case empty(String)
    case busy(String)
    case deviceFlow(String)
    case gap(String)
    /// A request that carries the key was aimed away from the configured Choir host; nothing was sent.
    case wrongHost(String)

    public var description: String {
        switch self {
        case .notSignedIn: return "not signed in"
        case .unauthorized: return "401: the key was refused; sign in again"
        case .forbidden(let p): return "403: the key lacks the scope for \(p)"
        case .rateLimited(let r): return "429: rate limited" + (r.map { String(format: " (retry after %.0f s)", $0) } ?? "")
        case .server(let s, let m): return "HTTP \(s)" + (m.isEmpty ? "" : ": \(m)")
        case .transport(let m): return "network: \(m)"
        case .timeout(let p): return "timed out: \(p)"
        case .decode(let m): return "unexpected response: \(m)"
        case .empty(let w): return "nothing to play: \(w) is empty"
        case .busy(let w): return "busy: \(w)"
        case .deviceFlow(let m): return "sign-in: \(m)"
        case .gap(let m): return "server gap: \(m)"
        case .wrongHost(let h): return "refused: \(h) is not the configured Choir host; the key was not sent"
        }
    }
}

// MARK: Environment (shared by the stages)

/// What the Choir stages share: the key store, the event log, the URL session and what is playing. Stage
/// factories take only parameters, so the host (LabRuntime, the replay command, tests) sets this once.
/// The key is never part of any parameter or configuration.
public final class ChoirEnvironment: @unchecked Sendable {
    public static let shared = ChoirEnvironment()
    public static let defaultBaseURL = "https://choir.news"
    private let lock = NSLock()
    private var _keys: KeyStore = InMemoryKeyStore()
    private var _configuredBase = ChoirEnvironment.defaultBaseURL
    private var _log: EventLog?
    private var _session: URLSession = .shared
    private var _playing: [String: PlayingProgramme] = [:]
    public init() {}

    public var keys: KeyStore { get { lock.lock(); defer { lock.unlock() }; return _keys } set { lock.lock(); _keys = newValue; lock.unlock() } }
    public var log: EventLog? { get { lock.lock(); defer { lock.unlock() }; return _log } set { lock.lock(); _log = newValue; lock.unlock() } }
    /// Tests inject a URLSession whose configuration carries a stub URLProtocol.
    public var session: URLSession { get { lock.lock(); defer { lock.unlock() }; return _session } set { lock.lock(); _session = newValue; lock.unlock() } }

    /// Learn the listener's computer now (at sign-in, app start, before a replay) so a turn makes no discovery call.
    public func warm(baseURL: String? = nil) async {
        let c = client(baseURL: baseURL)
        if c.hasKey { _ = try? await c.computerID() }
    }

    /// The Choir host this app is set up for: the only place the key is ever sent. A stage configuration may name another
    /// `baseURL` (a lab setting, or a configuration fetched from a link); a client made for it still refuses to carry the key.
    public var configuredBaseURL: String { get { lock.lock(); defer { lock.unlock() }; return _configuredBase } set { lock.lock(); _configuredBase = newValue; lock.unlock() } }

    public func client(baseURL: String? = nil, timeout: Double = 15) -> ChoirClient {
        ChoirClient(baseURL: (baseURL?.isEmpty == false ? baseURL! : configuredBaseURL), keys: keys, session: session, log: log, timeout: timeout,
                    trustedOrigin: URL(string: configuredBaseURL))
    }

    // What is playing, so the desk knows which document a turn is about.
    public func setPlaying(_ p: PlayingProgramme) { lock.lock(); _playing[p.programmeID] = p; lock.unlock() }
    public func playing(_ programmeID: String) -> PlayingProgramme? { lock.lock(); defer { lock.unlock() }; return _playing[programmeID] }
    public func updatePlaying(_ programmeID: String, _ f: (inout PlayingProgramme) -> Void) {
        lock.lock(); defer { lock.unlock() }
        if var p = _playing[programmeID] { f(&p); _playing[programmeID] = p }
    }
}

/// A programme the Choir stages loaded, with what the desk needs to prompt about it.
public struct PlayingProgramme: Sendable, Equatable {
    public var programmeID: String
    public var title: String
    public var route: String
    public var publicationVersionID: String
    public var manifestHash: String
    /// Spoken text of the blocks as last loaded or revised (for the place note and the diff).
    public var blocks: [String]
    /// The listener's own document once the story was forked.
    public var docID: String?
    public init(programmeID: String, title: String, route: String, publicationVersionID: String, manifestHash: String,
                blocks: [String], docID: String? = nil) {
        self.programmeID = programmeID; self.title = title; self.route = route; self.publicationVersionID = publicationVersionID
        self.manifestHash = manifestHash; self.blocks = blocks; self.docID = docID
    }
}

// MARK: Client

public struct ChoirResponse: Sendable {
    public let status: Int
    public let data: Data
    public let ms: Double
    public let headers: [String: String]
    public func json() throws -> Any {
        do { return try JSONSerialization.jsonObject(with: data) } catch { throw ChoirError.decode("not JSON") }
    }
    public func object() throws -> [String: Any] {
        guard let o = try json() as? [String: Any] else { throw ChoirError.decode("not a JSON object") }
        return o
    }
}

public final class ChoirClient: @unchecked Sendable {
    public let baseURL: URL
    let keys: KeyStore
    let session: URLSession
    let log: EventLog?
    let timeout: Double
    /// When set, a request that carries the key goes only if `baseURL` has this scheme, host and port.
    let trustedOrigin: URL?

    public init(baseURL: String, keys: KeyStore, session: URLSession = .shared, log: EventLog? = nil, timeout: Double = 15,
                trustedOrigin: URL? = nil) {
        self.trustedOrigin = trustedOrigin
        self.baseURL = URL(string: baseURL.hasSuffix("/") ? String(baseURL.dropLast()) : baseURL) ?? URL(string: ChoirEnvironment.defaultBaseURL)!
        self.keys = keys; self.session = session; self.log = log; self.timeout = timeout
    }

    public var hasKey: Bool { keys.get(KeyAccount.apiKey) != nil }

    /// Send a request. `route` is the path template used in events (no ids, no query), so a timing line
    /// never carries a document id or a secret. Throws a typed `ChoirError` for any non-2xx unless listed in `okStatuses`.
    @discardableResult
    public func send(_ method: String, _ path: String, query: [String: String] = [:], body: Data? = nil,
                     authenticated: Bool = true, route: String? = nil, timeout t: Double? = nil,
                     okStatuses: Set<Int> = []) async throws -> ChoirResponse {
        if authenticated, let t = trustedOrigin, !SameOrigin.matches(baseURL, t) { throw ChoirError.wrongHost(baseURL.host ?? "(none)") }
        guard var comps = URLComponents(url: baseURL, resolvingAgainstBaseURL: false) else { throw ChoirError.decode("bad base URL") }
        comps.percentEncodedPath = path
        if !query.isEmpty { comps.queryItems = query.sorted { $0.key < $1.key }.map { URLQueryItem(name: $0.key, value: $0.value) } }
        guard let url = comps.url else { throw ChoirError.decode("bad path") }
        var r = URLRequest(url: url, timeoutInterval: t ?? timeout)
        r.httpMethod = method
        r.setValue("application/json", forHTTPHeaderField: "Accept")
        if body != nil { r.setValue("application/json", forHTTPHeaderField: "Content-Type"); r.httpBody = body }
        if authenticated {
            guard let key = keys.get(KeyAccount.apiKey) else { throw ChoirError.notSignedIn }
            r.setValue("Bearer \(key)", forHTTPHeaderField: "Authorization")
            if !path.hasPrefix("/auth/") && path != Self.statusPath {
                r.setValue(try await computerID(), forHTTPHeaderField: "X-Choir-Computer")
            }
        }
        let label = "choir \(method) \(route ?? path)"
        let start = log?.clock.now
        let t0 = ProcessInfo.processInfo.systemUptime
        func finish(_ note: String) {
            guard let log, let start else { return }
            log.emit(.stageTiming(stage: label, startedAt: start, endedAt: log.clock.now))
            if !note.isEmpty { log.emit(.note("\(label): \(note)")) }
        }
        let data: Data, resp: URLResponse
        do { (data, resp) = try await session.data(for: r) }
        catch let e as URLError {
            finish("failed (\(e.code.rawValue))")
            if e.code == .cancelled { throw CancellationError() }
            if e.code == .timedOut { throw ChoirError.timeout(route ?? path) }
            throw ChoirError.transport(e.localizedDescription)
        } catch is CancellationError { finish("cancelled"); throw CancellationError() }
        catch { finish("failed"); throw ChoirError.transport("request failed") }
        let ms = (ProcessInfo.processInfo.systemUptime - t0) * 1000
        guard let h = resp as? HTTPURLResponse else { finish("no HTTP response"); throw ChoirError.decode("no HTTP response") }
        finish(String(format: "%d in %.0f ms", h.statusCode, ms))
        if (200..<300).contains(h.statusCode) || okStatuses.contains(h.statusCode) {
            var hs: [String: String] = [:]
            for (k, v) in h.allHeaderFields { if let k = k as? String, let v = v as? String { hs[k.lowercased()] = v } }
            return ChoirResponse(status: h.statusCode, data: data, ms: ms, headers: hs)
        }
        throw Self.error(status: h.statusCode, data: data, headers: h, path: route ?? path)
    }

    static let statusPath = "/api/compute/status"

    /// The listener's computer: stored after sign-in, or discovered with `GET /api/compute/status`
    /// (an owner-wide key may read it without naming a computer; `computers[]` lists the owner's).
    public func computerID() async throws -> String {
        if let c = keys.get(KeyAccount.computerID), !c.isEmpty { return c }
        // One discovery at a time: stages that start together share a single status call.
        let id = ObjectIdentifier(keys)
        Self.flightLock.lock()
        let t: Task<String, Error>
        if let f = Self.flights[id] { t = f } else {
            // The discovery clears its own entry when it ends (not when one caller stops waiting), so callers that come while it
            // runs share it and a caller that gives up cannot make the next one start a second call.
            t = Task { [self] in
                defer { Self.flightLock.lock(); Self.flights[id] = nil; Self.flightLock.unlock() }
                return try await discoverComputer()
            }
            Self.flights[id] = t
        }
        Self.flightLock.unlock()
        return try await t.value
    }
    nonisolated(unsafe) private static var flights: [ObjectIdentifier: Task<String, Error>] = [:]
    private static let flightLock = NSLock()

    func discoverComputer() async throws -> String {
        let o = try await send("GET", Self.statusPath, route: Self.statusPath).object()
        let list = (o["computers"] as? [[String: Any]] ?? []).filter { ($0["kind"] as? String ?? "interactive") == "interactive" && !(($0["computer_id"] as? String) ?? "").isEmpty }
        guard let pick = list.first(where: { ($0["current"] as? Bool) == true }) ?? list.first, let id = pick["computer_id"] as? String else {
            let warn = (o["warnings"] as? [String])?.first ?? ""
            throw ChoirError.gap("the account has no computer yet" + (warn.isEmpty ? "" : " (\(warn.prefix(120)))") + "; try again in a moment")
        }
        try? keys.set(id, account: KeyAccount.computerID)
        return id
    }

    static func error(status: Int, data: Data, headers: HTTPURLResponse, path: String) -> ChoirError {
        switch status {
        case 401: return .unauthorized
        case 403: return .forbidden(path)
        case 429: return .rateLimited(retryAfter: headers.value(forHTTPHeaderField: "Retry-After").flatMap { Double($0) })
        default:
            // Only the server's own `error` string, short, never the raw body (which could echo a request).
            let msg = ((try? JSONSerialization.jsonObject(with: data)) as? [String: Any])?["error"] as? String ?? ""
            return .server(status: status, message: String(msg.prefix(200)))
        }
    }

    public func postJSON(_ path: String, _ body: [String: Any], authenticated: Bool = true, route: String? = nil,
                         timeout: Double? = nil, okStatuses: Set<Int> = []) async throws -> ChoirResponse {
        let data = try JSONSerialization.data(withJSONObject: body, options: [.sortedKeys])
        return try await send("POST", path, body: data, authenticated: authenticated, route: route, timeout: timeout, okStatuses: okStatuses)
    }
}
