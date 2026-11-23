import Foundation

// Sign-in by the device flow: begin, show the code, poll for the key, store it. The approval page is opened by shared code in the system sign-in sheet
// (AutoradioLab/signin-sheet.ts); this stays native only because the device code, the poll and the Keychain write are here and must not pass through shared code.
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
            update { $0.userCode = user; $0.verificationURL = full; $0.message = "approve this sign-in in the window that opened" }
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

    /// Delete the local key and wipe the account's state at once; then revoke on the server in the background (best effort). The old key is held
    /// only for that one call: it is in no store, and nothing else can use it.
    public func signOut() async {
        lock.lock(); task?.cancel(); task = nil; lock.unlock()
        let id = env.keys.get(KeyAccount.keyID), key = env.keys.get(KeyAccount.apiKey)
        let revoke = (id != nil && key != nil)
        env.keys.delete(KeyAccount.apiKey); env.keys.delete(KeyAccount.keyID); env.keys.delete(KeyAccount.computerID)
        await onSignOut?()           // everything of the account on the phone goes (AccountWipe)
        env.log?.emit(.note("choir: signed out" + (revoke ? "; revoking the key in the background" : "")))
        update { $0.status = "idle"; $0.userCode = ""; $0.verificationURL = ""; $0.message = "signed out" }
        if revoke, let id, let key {
            let held = ChoirClient(baseURL: baseURL, keys: InMemoryKeyStore([KeyAccount.apiKey: key]), session: env.session, log: nil,
                                   trustedOrigin: URL(string: env.configuredBaseURL))
            let enc = PathSegment.encode(id)
            let log = env.log
            let t = Task.detached {
                do { try await held.send("DELETE", "/auth/api-keys/\(enc)", route: "/auth/api-keys/{id}", timeout: 8); log?.emit(.note("choir: the key was revoked")) }
                catch { log?.emit(.note("choir: the key was not revoked on the server (\(error))")) }
            }
            lock.lock(); revokeTask = t; lock.unlock()
        }
    }
    /// The background revoke (tests wait on it).
    private var revokeTask: Task<Void, Never>?
    public func waitForRevoke() async { let t = { lock.lock(); defer { lock.unlock() }; return revokeTask }(); await t?.value }
    /// Set by the app: wipes what belongs to the account (AccountWipe.run).
    public var onSignOut: (@Sendable () async -> Void)?
}
