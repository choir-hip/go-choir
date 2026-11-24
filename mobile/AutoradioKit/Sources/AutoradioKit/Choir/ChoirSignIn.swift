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

    /// Clear the phone at once: the key, then the account's state (AccountWipe). The server revoke is shared code's (AutoradioLab/account.ts `revokeHeldKey`, run while the key is still here, with a retry file when the
    /// phone is offline), so nothing here waits on a network.
    public func signOut() async {
        env.keys.delete(KeyAccount.apiKey); env.keys.delete(KeyAccount.keyID); env.keys.delete(KeyAccount.computerID)
        await onSignOut?()           // everything of the account on the phone goes (AccountWipe)
        env.log?.emit(.note("choir: signed out"))
        update { $0.status = "idle"; $0.message = "signed out" }
    }
    func revokeNow(keyID id: String, key: String) async -> Bool {
        let held = ChoirClient(baseURL: baseURL, keys: InMemoryKeyStore([KeyAccount.apiKey: key]), session: env.session, log: nil, trustedOrigin: URL(string: env.configuredBaseURL))
        if id.isEmpty { env.log?.emit(.note("choir: a legacy key has no id, so it cannot be revoked here; revoke it in Settings")); return true }
        do { try await held.send("DELETE", "/auth/api-keys/\(PathSegment.encode(id))", route: "/auth/api-keys/{id}", timeout: 8, okStatuses: [401, 404]); env.log?.emit(.note("choir: a key was revoked")); return true }
        catch { env.log?.emit(.note("choir: a key was not revoked on the server; it is kept and retried (\(error))")); return false }
    }
    /// Set by the app: wipes what belongs to the account (AccountWipe.run).
    public var onSignOut: (@Sendable () async -> Void)?
}
