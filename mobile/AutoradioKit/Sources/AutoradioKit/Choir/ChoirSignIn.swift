import Foundation

// Sign-in by the device flow: begin, show the code and open the approval page, poll for the key, store it.
// The key goes to the KeyStore and nowhere else: not an event, not a record, not the UI state.

public struct ChoirAuthState: Codable, Sendable, Equatable {
    public var signedIn = false
    public var baseURL = ChoirEnvironment.defaultBaseURL
    /// pending | idle | failed
    public var status = "idle"
    public var userCode = ""
    public var verificationURL = ""
    public var message = ""
    public init() {}
}

public final class ChoirSignIn: @unchecked Sendable {
    private let lock = NSLock()
    private var state = ChoirAuthState()
    private var task: Task<Void, Never>?
    private let env: ChoirEnvironment
    public var baseURL: String
    /// The host opens the approval page (UIApplication.open on iOS); nil on the Mac.
    public var openURL: (@Sendable (URL) -> Void)?
    public var onChange: (@Sendable () -> Void)?
    /// Poll pacing. Tests set `sleep` to return at once.
    public var sleep: @Sendable (Double) async -> Void = { s in try? await Task.sleep(nanoseconds: UInt64(s * 1e9)) }
    public var deviceLabel: String

    public init(env: ChoirEnvironment = .shared, baseURL: String = ChoirEnvironment.defaultBaseURL, deviceLabel: String = "Autoradio lab") {
        self.env = env; self.baseURL = baseURL; self.deviceLabel = String(deviceLabel.prefix(64))
        state.signedIn = env.keys.get(KeyAccount.apiKey) != nil
        state.baseURL = baseURL
    }

    public var current: ChoirAuthState {
        lock.lock(); defer { lock.unlock() }
        var s = state
        s.signedIn = env.keys.get(KeyAccount.apiKey) != nil
        s.baseURL = baseURL
        return s
    }
    private func update(_ f: (inout ChoirAuthState) -> Void) { lock.lock(); f(&state); lock.unlock(); onChange?() }

    private func client() -> ChoirClient { env.client(baseURL: baseURL) }

    /// Start the flow. Returns once the code is known (or begin failed); polling continues in the background.
    @discardableResult
    public func begin() async -> ChoirAuthState {
        lock.lock(); task?.cancel(); task = nil; lock.unlock()   // a second sign-in cancels the first
        update { $0.status = "pending"; $0.userCode = ""; $0.verificationURL = ""; $0.message = "contacting \(baseURL)" }
        let c = client()
        do {
            let r = try await c.postJSON("/auth/device/begin", ["label": deviceLabel], authenticated: false)
            let o = try r.object()
            guard let device = o["device_code"] as? String, let user = o["user_code"] as? String else { throw ChoirError.decode("begin: no codes") }
            let full = (o["verification_uri_complete"] as? String) ?? (o["verification_uri"] as? String) ?? ""
            let interval = max(1, (o["interval"] as? Double) ?? 5)
            let expires = (o["expires_in"] as? Double) ?? 600
            update { $0.userCode = user; $0.verificationURL = full; $0.message = "approve this sign-in in the browser" }
            if let u = URL(string: full), ["https", "http"].contains(u.scheme ?? "") { openURL?(u) }
            let t = Task { [self] in await poll(device: device, interval: interval, expires: expires) }
            lock.lock(); task = t; lock.unlock()
        } catch {
            update { $0.status = "failed"; $0.message = "\(error)" }
        }
        return current
    }

    /// Wait for the background poll to finish (tests, the replay command).
    public func wait() async { let t = { lock.lock(); defer { lock.unlock() }; return task }(); await t?.value }

    func poll(device: String, interval: Double, expires: Double) async {
        var wait = interval
        var waited = 0.0
        let c = client()
        while !Task.isCancelled {
            await sleep(wait); waited += wait
            if Task.isCancelled { return }
            if waited > expires + 5 { update { $0.status = "failed"; $0.userCode = ""; $0.message = "the code expired; sign in again" }; return }
            do {
                let r = try await c.postJSON("/auth/device/token", ["device_code": device], authenticated: false, route: "/auth/device/token", okStatuses: [400])
                let o = try r.object()
                if r.status == 400 {
                    switch o["error"] as? String ?? "" {
                    case "authorization_pending": continue
                    case "slow_down": wait += 5; continue
                    case "access_denied": update { $0.status = "failed"; $0.userCode = ""; $0.message = "the sign-in was denied" }; return
                    case "expired_token": update { $0.status = "failed"; $0.userCode = ""; $0.message = "the code expired; sign in again" }; return
                    default: update { $0.status = "failed"; $0.userCode = ""; $0.message = "the code is no longer valid" }; return
                    }
                }
                guard let key = o["api_key"] as? String, !key.isEmpty else { throw ChoirError.decode("token: no key") }
                do {
                    try env.keys.set(key, account: KeyAccount.apiKey)
                    if let id = o["key_id"] as? String { try? env.keys.set(id, account: KeyAccount.keyID) }
                } catch {
                    update { $0.status = "failed"; $0.userCode = ""; $0.message = "could not store the key: \(error)" }
                    return
                }
                _ = try? await c.computerID()   // best effort now; the first call retries it
                env.log?.emit(.note("choir: signed in to \(baseURL)"))
                update { $0.status = "idle"; $0.userCode = ""; $0.verificationURL = ""; $0.message = "signed in" }
                return
            } catch is CancellationError { return }
            catch let e as ChoirError {
                // A transient network error while waiting for the approval does not end the flow.
                switch e {
                case .transport, .timeout: update { $0.message = "waiting; \(e)" }; continue
                default: update { $0.status = "failed"; $0.userCode = ""; $0.message = "\(e)" }; return
                }
            } catch { update { $0.status = "failed"; $0.message = "\(error)" }; return }
        }
    }

    /// Stop waiting for an approval; keeps any key already stored.
    public func cancel() {
        lock.lock(); task?.cancel(); task = nil; lock.unlock()
        update { $0.status = "idle"; $0.userCode = ""; $0.verificationURL = ""; $0.message = "" }
    }

    /// Delete the local key at once; revoke on the server if it can be reached (best effort).
    public func signOut() async {
        lock.lock(); task?.cancel(); task = nil; lock.unlock()
        let id = env.keys.get(KeyAccount.keyID)
        var note = "signed out"
        if let id, env.keys.get(KeyAccount.apiKey) != nil {
            let enc = id.addingPercentEncoding(withAllowedCharacters: .urlPathAllowed) ?? ""
            do { try await client().send("DELETE", "/auth/api-keys/\(enc)", route: "/auth/api-keys/{id}", timeout: 8); note = "signed out; key revoked" }
            catch { note = "signed out; the key was not revoked on the server (\(error))" }
        }
        env.keys.delete(KeyAccount.apiKey); env.keys.delete(KeyAccount.keyID); env.keys.delete(KeyAccount.computerID)
        env.log?.emit(.note("choir: \(note)"))
        update { $0.status = "idle"; $0.userCode = ""; $0.verificationURL = ""; $0.message = note }
    }
}
