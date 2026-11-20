import Foundation

// The computer, for Settings (lane 7). The account (GET /auth/whoami) moved to shared code (AutoradioLab/settings.ts).
// A client behind an interface, on the routes the plan names:
//   ComputerClient  GET /api/compute/status, POST /api/compute/recovery {action: wake_current_computer},
//                   GET /api/computers/{id}/update, GET /api/computers/{id}/lifecycle/status,
//                   POST /api/computers/{id}/lifecycle/update {idempotency_key}   (the web's UpdateChip and compute monitor)
//
// Ways it can fail (tests in SettingsDataTests):
//  C1 Wake or Restart to update is tapped twice: while one is in flight the second is ignored (phase is not idle).
//  C2 The update route answers 409 (the computer is busy with a self-development apply or a live capsule): the view says it
//     will update when idle; it never retries by itself and never says it updated.
//  C3 A restart that the proxy gave up on (502/504) is taken as failed: the connection dropping while restarting is
//     expected; the view then polls the update signal and reports "current" only when the signal clears.
//  C4 Status has no computer yet (a new account): "try again in a moment", not a crash.

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
        let enc = PathSegment.encode(id)
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
        let enc = PathSegment.encode(id)
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
