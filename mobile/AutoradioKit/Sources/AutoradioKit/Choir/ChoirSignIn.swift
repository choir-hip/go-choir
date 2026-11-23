import Foundation

// Sign-in by the device flow, native only where it must be: the device code, ONE poll request per call, and the Keychain write. Which attempt is live, when to poll and when to give up are
// shared code's (AutoradioLab/signin-attempt.ts, the attempt's state machine); each call here names its attempt, and a call for any other attempt does nothing. The key goes to the KeyStore
// and nowhere else: not an event, not a record, not the UI state, not an answer.

public struct ChoirAuthState: Codable, Sendable, Equatable {
    public var signedIn = false
    public var baseURL = ChoirEnvironment.defaultBaseURL
    public var status = "idle"
    public var userCode = ""
    public var verificationURL = ""
    public var message = ""
    public init() {}
}

public final class ChoirSignIn: @unchecked Sendable {
    private let lock = NSLock()
    private var state = ChoirAuthState()
    private var live: (id: String, device: String)?     // the one attempt in progress; the device code never leaves here
    private var wrote: String?                          // the attempt whose key is in the Keychain
    private var beginSeq = 0                            // begins are serialized by this count: only the LATEST one installs its device code
    private let env: ChoirEnvironment
    public var baseURL: String
    public var onChange: (@Sendable () -> Void)?
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

    /// Begin attempt `id` (a later begin replaces it, and only the latest installs its code, whatever order the answers come in). Answers what the card shows and how to poll; the device code stays here.
    public func begin(attempt id: String) async -> [String: Any] {
        lock.lock(); beginSeq += 1; let mine = beginSeq; live = nil; lock.unlock()
        do {
            let o = try await client().postJSON("/auth/device/begin", ["label": deviceLabel], authenticated: false).object()
            guard let device = o["device_code"] as? String, let user = o["user_code"] as? String else { throw ChoirError.decode("begin: no codes") }
            lock.lock(); guard beginSeq == mine else { lock.unlock(); return ["ok": false, "message": "replaced by a newer sign-in"] }; live = (id, device); lock.unlock()
            return ["ok": true, "userCode": user, "link": (o["verification_uri_complete"] as? String) ?? (o["verification_uri"] as? String) ?? "",
                    "interval": max(1, (o["interval"] as? Double) ?? 5), "expires": (o["expires_in"] as? Double) ?? 600]
        } catch { return ["ok": false, "message": "could not start the sign-in (\(error))"] }
    }

    /// One poll for attempt `id`: pending | slow_down | approved | denied | expired | error | stale (not the live attempt) | invalid. The key is written only if `id` is STILL the live
    /// attempt, checked under the lock `forget` takes: a minted key is either stored and belongs to the live attempt, or revoked (forgotten, replaced, sign-out, a Keychain that refuses). No `await` lies between the answer and that guard.
    public func pollOnce(attempt id: String) async -> String {
        lock.lock(); let device = live?.id == id ? live?.device : nil; lock.unlock()
        guard let device else { return "stale" }
        let c = client()
        do {
            let r = try await c.postJSON("/auth/device/token", ["device_code": device], authenticated: false, route: "/auth/device/token", okStatuses: [400])
            let o = try r.object()
            if r.status == 400 {
                switch o["error"] as? String ?? "" {
                case "authorization_pending": return "pending"
                case "slow_down": return "slow_down"
                case "access_denied": forget(attempt: id); return "denied"
                case "expired_token": forget(attempt: id); return "expired"
                default: forget(attempt: id); return "invalid"
                }
            }
            guard let key = o["api_key"] as? String, !key.isEmpty else { throw ChoirError.decode("token: no key") }
            let keyID = o["key_id"] as? String
            lock.lock()
            guard live?.id == id else { lock.unlock(); if let keyID { revoke(keyID: keyID, key: key) }; return "stale" }          // forgotten while the answer was on its way: discard
            let stored: Bool = { do { try env.keys.set(key, account: KeyAccount.apiKey); if let keyID { try? env.keys.set(keyID, account: KeyAccount.keyID) }; return true } catch { return false } }()
            if stored { wrote = id }; live = nil; lock.unlock()           // the attempt ends here, key or not
            guard stored else { if let keyID { revoke(keyID: keyID, key: key) }; return "invalid" }       // minted and not stored: revoked, never left live
            _ = try? await c.computerID()   // best effort now; the first call retries it
            env.log?.emit(.note("choir: signed in to \(baseURL)"))
            update { $0.status = "idle"; $0.message = "signed in" }
            return "approved"
        } catch let e as ChoirError { switch e { case .transport, .timeout: return "error"; default: forget(attempt: id); return "invalid" } }
        catch { return "error" }
    }

    /// Drop attempt `id`: its code is abandoned (it expires on the server as it always did). A key this attempt had already written is deleted and revoked.
    public func forget(attempt id: String) {
        lock.lock(); if live?.id == id { live = nil }
        let had = wrote == id ? (env.keys.get(KeyAccount.keyID), env.keys.get(KeyAccount.apiKey)) : (nil, nil)
        if wrote == id { wrote = nil; env.keys.delete(KeyAccount.apiKey); env.keys.delete(KeyAccount.keyID); env.keys.delete(KeyAccount.computerID) }
        lock.unlock()
        if let k = had.0, let key = had.1 { revoke(keyID: k, key: key) }
        update { $0.status = "idle"; $0.message = "" }
    }

    /// Delete the local key and wipe the account's state at once; then revoke on the server in the background (best effort). The old key is held
    /// only for that one call: it is in no store, and nothing else can use it.
    public func signOut() async {
        lock.lock(); live = nil; wrote = nil; lock.unlock()
        let id = env.keys.get(KeyAccount.keyID), key = env.keys.get(KeyAccount.apiKey)
        env.keys.delete(KeyAccount.apiKey); env.keys.delete(KeyAccount.keyID); env.keys.delete(KeyAccount.computerID)
        await onSignOut?()           // everything of the account on the phone goes (AccountWipe)
        env.log?.emit(.note("choir: signed out" + (id != nil && key != nil ? "; revoking the key in the background" : "")))
        update { $0.status = "idle"; $0.message = "signed out" }
        if let id, let key { revoke(keyID: id, key: key) }
    }
    private func revoke(keyID id: String, key: String) {
        let held = ChoirClient(baseURL: baseURL, keys: InMemoryKeyStore([KeyAccount.apiKey: key]), session: env.session, log: nil, trustedOrigin: URL(string: env.configuredBaseURL))
        let enc = PathSegment.encode(id), log = env.log
        let t = Task.detached {
            do { try await held.send("DELETE", "/auth/api-keys/\(enc)", route: "/auth/api-keys/{id}", timeout: 8); log?.emit(.note("choir: the key was revoked")) }
            catch { log?.emit(.note("choir: the key was not revoked on the server (\(error))")) }
        }
        lock.lock(); revokeTask = t; lock.unlock()
    }
    /// The background revoke (tests wait on it).
    private var revokeTask: Task<Void, Never>?
    public func waitForRevoke() async { let t = { lock.lock(); defer { lock.unlock() }; return revokeTask }(); await t?.value }
    /// Set by the app: wipes what belongs to the account (AccountWipe.run).
    public var onSignOut: (@Sendable () async -> Void)?
}
