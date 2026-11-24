import Foundation

// What is left of the Choir sign-in in Swift: the local sign-out and its revoke. Signing in and creating an account are shared code (AutoradioLab/app-signin.ts): the platform's passkey screen
// through Exact's @exact/passkeys, the finish request, and the key kept as the Exact secrets `choir.key`, `choir.key-id` and `choir.computer-id` (the Keychain, service = the bundle id). This type
// reads and deletes those same Keychain items for the voice code and for sign-out (KeychainKeyStore, KeyAccount); it writes the key at no time. The key goes to the KeyStore and nowhere else.

public struct ChoirAuthState: Codable, Sendable, Equatable {
    public var signedIn = false
    public var baseURL = ChoirEnvironment.defaultBaseURL
    public var status = "idle"
    public var message = ""
    public init() {}
}

public final class ChoirSignIn: @unchecked Sendable {
    private let lock = NSLock()
    private var state = ChoirAuthState()
    private let env: ChoirEnvironment
    public var baseURL: String
    public var onChange: (@Sendable () -> Void)?

    public init(env: ChoirEnvironment = .shared, baseURL: String = ChoirEnvironment.defaultBaseURL) {
        self.env = env; self.baseURL = baseURL
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

    /// Delete the local key and wipe the account's state at once; then revoke on the server in the background (best effort). The old key is held
    /// only for that one call: it is in no store, and nothing else can use it.
    public func signOut() async {
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
