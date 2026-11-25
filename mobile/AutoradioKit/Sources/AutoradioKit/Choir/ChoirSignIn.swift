import Foundation

// What is left of the Choir sign-in in Swift: the native half of sign-out and `revokeNow`, which only LegacyKey uses. Signing in, creating an account and knowing whether the phone is signed in are
// shared code (AutoradioLab/app-signin.ts, account-state.ts): the key is kept as the Exact secrets `choir.key`, `choir.key-id` and `choir.computer-id` (the Keychain, service = the bundle id).
// This type deletes those same Keychain items at sign-out, natively, because the delete must happen whether or not the shared answer that asked still has its store bound when native answers
// (account.ts `forgetCredentials` then makes Exact's store agree); it reads and writes the key at no time. The voice turn reads it through ChoirEnvironment.keys (it calls the server from native code).
public final class ChoirSignIn: @unchecked Sendable {
    private let env: ChoirEnvironment
    public var baseURL: String

    public init(env: ChoirEnvironment = .shared, baseURL: String = ChoirEnvironment.defaultBaseURL) {
        self.env = env; self.baseURL = baseURL
    }

    /// Clear the phone at once: the key, then the account's state (AccountWipe); the server revoke is shared code's (account.ts `revokeDirect`, made after this with the key it read before).
    public func signOut() async {
        env.keys.delete(KeyAccount.apiKey); env.keys.delete(KeyAccount.keyID); env.keys.delete(KeyAccount.computerID)
        await onSignOut?()           // everything of the account on the phone goes (AccountWipe)
        env.log?.emit(.note("choir: signed out"))
    }
    func revokeNow(keyID id: String, key: String) async -> Bool {
        let held = ChoirClient(baseURL: baseURL, keys: InMemoryKeyStore([KeyAccount.apiKey: key]), session: env.session, log: nil, trustedOrigin: URL(string: env.configuredBaseURL))
        if id.isEmpty { env.log?.emit(.note("choir: a legacy key has no id; the server revokes by id, so it is deleted here unrevoked: revoke it in Settings on the web")); return true }
        do { try await held.send("DELETE", "/auth/api-keys/\(PathSegment.encode(id))", route: "/auth/api-keys/{id}", timeout: 8, okStatuses: [401, 404]); env.log?.emit(.note("choir: a key was revoked")); return true }
        catch { env.log?.emit(.note("choir: a key was not revoked on the server; it is kept and retried (\(error))")); return false }
    }
    /// Set by the app: wipes what belongs to the account (AccountWipe.run).
    public var onSignOut: (@Sendable () async -> Void)?
}
