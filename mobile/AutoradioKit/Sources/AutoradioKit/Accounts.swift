import Foundation

// Accounts and the computer, for Settings (lane 7). Two clients behind interfaces, each with a demo form that reads the
// bundled fixtures and a live form on the routes the plan names:
//   AccountClient   GET /auth/whoami (bearer key) -> {handle, key_id, key_label, scopes, computer_id}   (server lane, deployed
//                   on the branch lane/ar-server; a server without it answers 404 and the page says so in plain words)
//   ComputerClient  GET /api/compute/status, POST /api/compute/recovery {action: wake_current_computer},
//                   GET /api/computers/{id}/update, GET /api/computers/{id}/lifecycle/status,
//                   POST /api/computers/{id}/lifecycle/update {idempotency_key}   (the web's UpdateChip and compute monitor)
//
// Ways it can fail (written before the code; tests in SettingsDataTests):
//  A1 The key, or the account's email, is shown or logged: the views carry the handle, key id, key label and scopes only;
//     the key itself is never read here, and no event carries a response body.
//  A2 whoami answers 401 (key revoked) or 404 (route not deployed) or garbage: the view says which in plain words, the page
//     still renders, and sign out stays available.
//  A3 Two refreshes overlap and the older answer lands last: each refresh carries a generation; a stale answer is dropped.
//  C1 Wake or Restart to update is tapped twice: while one is in flight the second is ignored (phase is not idle).
//  C2 The update route answers 409 (the computer is busy with a self-development apply or a live capsule): the view says it
//     will update when idle; it never retries by itself and never says it updated.
//  C3 A restart that the proxy gave up on (502/504) is taken as failed: the connection dropping while restarting is
//     expected; the view then polls the update signal and reports "current" only when the signal clears.
//  C4 Status has no computer yet (a new account): "try again in a moment", not a crash.

public struct WhoAmI: Codable, Sendable, Equatable {
    public var handle = "", keyID = "", keyLabel = "", computerID = ""
    public var scopes: [String] = []
    public init(handle: String = "", keyID: String = "", keyLabel: String = "", scopes: [String] = [], computerID: String = "") {
        self.handle = handle; self.keyID = keyID; self.keyLabel = keyLabel; self.scopes = scopes; self.computerID = computerID
    }
    public static func parse(_ data: Data) throws -> WhoAmI {
        guard let o = try JSONSerialization.jsonObject(with: data) as? [String: Any] else { throw ChoirError.decode("whoami: not an object") }
        guard let h = o["handle"] as? String, !h.isEmpty else { throw ChoirError.decode("whoami: no handle") }
        return WhoAmI(handle: h, keyID: o["key_id"] as? String ?? "", keyLabel: o["key_label"] as? String ?? "",
                      scopes: o["scopes"] as? [String] ?? [], computerID: o["computer_id"] as? String ?? "")
    }
}

public protocol AccountClient: Sendable { func whoami() async throws -> WhoAmI }

public struct ChoirAccountClient: AccountClient {
    public init() {}
    public func whoami() async throws -> WhoAmI {
        let r = try await ChoirEnvironment.shared.client().send("GET", "/auth/whoami", route: "/auth/whoami")
        return try WhoAmI.parse(r.data)
    }
}

public struct DemoAccountClient: AccountClient {
    public init() {}
    public func whoami() async throws -> WhoAmI {
        guard let d = Demo.fixture("whoami") else { throw ChoirError.decode("no demo fixture") }
        return try WhoAmI.parse(d)
    }
}

/// What the Account page shows.
public struct AccountView: Codable, Sendable, Equatable {
    /// idle | loading | ok | failed
    public var status = "idle"
    public var signedIn = false
    public var handle = "", keyID = "", keyLabel = "", scopes = "", computerID = ""
    public var message = ""
    public var demo = false
    public var baseURL = ""
    public init() {}
}

public final class AccountModel: @unchecked Sendable {
    private let client: AccountClient
    private let lock = NSLock()
    private var v = AccountView()
    private var generation = 0
    public var signedIn: @Sendable () -> Bool
    public var baseURL: @Sendable () -> String
    public var onChange: (@Sendable () -> Void)?
    public init(client: AccountClient, demo: Bool, signedIn: @escaping @Sendable () -> Bool, baseURL: @escaping @Sendable () -> String) {
        self.client = client; self.signedIn = signedIn; self.baseURL = baseURL; v.demo = demo
    }
    public func view() -> AccountView {
        lock.lock(); var x = v; lock.unlock()
        x.baseURL = baseURL()
        if signedIn() { x.signedIn = true } else { let d = x.demo; x = AccountView(); x.demo = d; x.baseURL = baseURL(); x.message = "Not signed in." }
        return x
    }
    public func refresh() async {
        guard signedIn() else { onChange?(); return }
        lock.lock(); generation += 1; let g = generation; v.status = "loading"; v.message = ""; lock.unlock(); onChange?()
        var next = AccountView()
        do {
            let w = try await client.whoami()
            next.status = "ok"; next.handle = w.handle; next.keyID = w.keyID; next.keyLabel = w.keyLabel
            next.scopes = w.scopes.joined(separator: ", "); next.computerID = w.computerID
        } catch {
            next.status = "failed"; next.message = Self.plain(error)
        }
        lock.lock(); let current = (g == generation); if current { next.demo = v.demo; v = next }; lock.unlock()   // A3
        if current { onChange?() }
    }
    public func clear() { lock.lock(); generation += 1; let d = v.demo; v = AccountView(); v.demo = d; lock.unlock(); onChange?() }

    static func plain(_ e: Error) -> String {
        switch e as? ChoirError {
        case .unauthorized?: return "This phone's sign-in is no longer accepted. Sign out and sign in again."
        case .server(let status, _)? where status == 404: return "This server cannot say which account a key is yet."
        case .notSignedIn?: return "Not signed in."
        case .decode?: return "The server's answer was not what was expected."
        default: return "Could not reach choir.news."
        }
    }
}

// MARK: Computer

public struct ComputerSnapshot: Sendable, Equatable {
    public var id = "", state = "", epoch = 0, updateTarget = ""
    public init(id: String = "", state: String = "", epoch: Int = 0, updateTarget: String = "") {
        self.id = id; self.state = state; self.epoch = epoch; self.updateTarget = updateTarget
    }
}
public enum UpdateOutcome: Sendable, Equatable { case started, current, paused(String) }

public protocol ComputerClient: Sendable {
    func snapshot() async throws -> ComputerSnapshot
    func wake() async throws -> String
    func restartToUpdate() async throws -> UpdateOutcome
}

public struct ChoirComputerClient: ComputerClient {
    public init() {}
    public func snapshot() async throws -> ComputerSnapshot {
        let c = ChoirEnvironment.shared.client()
        let status = try await c.send("GET", "/api/compute/status", route: "/api/compute/status").object()
        let list = (status["computers"] as? [[String: Any]] ?? []).filter { ($0["kind"] as? String ?? "interactive") == "interactive" && !(($0["computer_id"] as? String) ?? "").isEmpty }
        guard let pick = list.first(where: { ($0["current"] as? Bool) == true }) ?? list.first, let id = pick["computer_id"] as? String else {
            throw ChoirError.gap("the account has no computer yet; try again in a moment")        // C4
        }
        var s = ComputerSnapshot(id: id, state: (pick["state"] as? String) ?? (pick["status"] as? String) ?? "")
        let enc = id.addingPercentEncoding(withAllowedCharacters: .urlPathAllowed) ?? id
        if let r = try? await c.send("GET", "/api/computers/\(enc)/update", route: "/api/computers/{id}/update"), let o = try? r.object() {
            s.updateTarget = ((o["update_available"] as? [String: Any])?["target"] as? String) ?? ""
        }
        if let r = try? await c.send("GET", "/api/computers/\(enc)/lifecycle/status", route: "/api/computers/{id}/lifecycle/status"), let o = try? r.object() {
            if let st = o["state"] as? String, !st.isEmpty { s.state = st }
            s.epoch = Int((o["realization_epoch"] as? Double) ?? 0)
        }
        return s
    }
    public func wake() async throws -> String {
        let r = try await ChoirEnvironment.shared.client().postJSON("/api/compute/recovery", ["action": "wake_current_computer"], route: "/api/compute/recovery")
        _ = r
        return "Your computer was asked to wake."
    }
    public func restartToUpdate() async throws -> UpdateOutcome {
        let c = ChoirEnvironment.shared.client()
        let id = try await c.computerID()
        let enc = id.addingPercentEncoding(withAllowedCharacters: .urlPathAllowed) ?? id
        do {
            let r = try await c.postJSON("/api/computers/\(enc)/lifecycle/update", ["idempotency_key": UUID().uuidString],
                                         route: "/api/computers/{id}/lifecycle/update", timeout: 30, okStatuses: [200, 201])
            if r.status == 200, ((try? r.object())?["outcome"] as? String) == "current" { return .current }
            return .started
        } catch ChoirError.server(let status, let m) where status == 409 {
            return .paused(m)                                                                      // C2
        } catch ChoirError.timeout, ChoirError.transport {
            return .started                                                                         // C3: the act does not stop
        }
    }
}

/// Demo: a computer that is running, with an update waiting; Restart to update takes about a second and then it is current.
public final class DemoComputerClient: ComputerClient, @unchecked Sendable {
    private let lock = NSLock()
    private var updated = false
    private var woke = false
    public init() {}
    public func snapshot() async throws -> ComputerSnapshot {
        let o = (Demo.fixture("computer-update")).flatMap { try? JSONSerialization.jsonObject(with: $0) as? [String: Any] } ?? [:]
        lock.lock(); let u = updated; lock.unlock()
        return ComputerSnapshot(id: "demo-computer", state: (o["state"] as? String) ?? "running", epoch: Int((o["realization_epoch"] as? Double) ?? 7) + (u ? 1 : 0),
                                updateTarget: u ? "" : (((o["update_available"] as? [String: Any])?["target"] as? String) ?? ""))
    }
    public func wake() async throws -> String { lock.lock(); woke = true; lock.unlock(); return "Your computer is awake." }
    public func restartToUpdate() async throws -> UpdateOutcome {
        lock.lock(); let u = updated; lock.unlock()
        if u { return .current }
        try? await Task.sleep(nanoseconds: 900_000_000)
        lock.lock(); updated = true; lock.unlock()
        return .started
    }
}

public struct ComputerView: Codable, Sendable, Equatable {
    public var id = "", state = "", target = ""
    /// idle | waking | updating | current | failed | paused
    public var phase = "idle"
    public var updateAvailable = false
    public var message = ""
    public var epoch = 0
    public var checked = false
    public init() {}
}

public final class ComputerModel: @unchecked Sendable {
    private let client: ComputerClient
    private let lock = NSLock()
    private var v = ComputerView()
    private var generation = 0
    public var onChange: (@Sendable () -> Void)?
    /// Poll pacing for "did the update clear". Tests set it to return at once.
    public var sleep: @Sendable (Double) async -> Void = { s in try? await Task.sleep(nanoseconds: UInt64(s * 1e9)) }
    public init(client: ComputerClient) { self.client = client }
    public func view() -> ComputerView { lock.lock(); defer { lock.unlock() }; return v }

    private func set(_ f: (inout ComputerView) -> Void) { lock.lock(); f(&v); lock.unlock(); onChange?() }

    public func refresh() async {
        lock.lock(); generation += 1; let g = generation; lock.unlock()
        do {
            let s = try await client.snapshot()
            lock.lock()
            if g == generation {
                v.id = s.id; v.state = s.state; v.epoch = s.epoch; v.target = s.updateTarget; v.updateAvailable = !s.updateTarget.isEmpty; v.checked = true
                if v.phase == "failed" { v.phase = "idle"; v.message = "" }
            }
            lock.unlock(); onChange?()
        } catch { set { $0.phase = "failed"; $0.message = Self.plain(error); $0.checked = true } }
    }

    public func wake() async {
        guard view().phase == "idle" || view().phase == "failed" || view().phase == "current" else { return }       // C1
        set { $0.phase = "waking"; $0.message = "Waking your computer…" }
        do { let m = try await client.wake(); set { $0.phase = "idle"; $0.message = m } }
        catch { set { $0.phase = "failed"; $0.message = Self.plain(error) }; return }
        await refresh()
    }

    public func restartToUpdate() async {
        guard view().phase == "idle" || view().phase == "failed" || view().phase == "current" else { return }       // C1
        set { $0.phase = "updating"; $0.message = "Restarting to update…" }
        do {
            switch try await client.restartToUpdate() {
            case .current: set { $0.phase = "current"; $0.message = "Already up to date."; $0.updateAvailable = false }
            case .paused(let m): set { $0.phase = "paused"; $0.message = m.isEmpty ? "Your computer is busy and will update when it is idle." : "Your computer is busy (\(m.prefix(80))) and will update when it is idle." }
            case .started:
                // C3: poll the update signal; the update is done when it clears.
                for _ in 0..<40 {
                    await sleep(1.5)
                    if let s = try? await client.snapshot(), s.updateTarget.isEmpty {
                        set { $0.phase = "current"; $0.message = "Updated. Your computer restarted."; $0.updateAvailable = false; $0.target = ""; $0.state = s.state; $0.epoch = s.epoch }
                        return
                    }
                }
                set { $0.phase = "idle"; $0.message = "Still restarting. Check again in a minute." }
            }
        } catch { set { $0.phase = "failed"; $0.message = Self.plain(error) } }
    }

    static func plain(_ e: Error) -> String {
        switch e as? ChoirError {
        case .unauthorized?: return "This phone's sign-in is no longer accepted."
        case .notSignedIn?: return "Not signed in."
        case .gap(let m)?: return String(m.prefix(1)).uppercased() + String(m.dropFirst()) + "."
        case .rateLimited?: return "Your computer is busy right now. Try again in a moment."
        default: return "Could not reach your computer."
        }
    }
}
