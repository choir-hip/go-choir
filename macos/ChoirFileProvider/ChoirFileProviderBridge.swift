// ChoirFileProviderBridge.swift
//
// BridgeClient connects the macOS File Provider extension to the Go sync
// engine's IPC bridge (internal/desktop/fileprovider/bridge.go). The Go
// bridge listens on a Unix domain socket and serves a JSON REST API for
// enumeration, read, write, delete, move, conflicts, and sync status.
//
// The socket path is conventionally:
//   ~/Library/Application Support/Choir/fileprovider.sock
//
// The bridge is started by the Choir desktop app (Wails) when the sync
// engine is running. The File Provider extension connects to the same
// socket. If the socket is unavailable, the extension returns errors to
// Finder and retries on the next enumerator pass.

import Foundation

/// Errors returned by the bridge client.
enum BridgeError: Error, LocalizedError {
    case socketUnavailable
    case decodeFailed(String)
    case httpError(Int, String)
    case invalidResponse

    var errorDescription: String? {
        switch self {
        case .socketUnavailable:
            return "Choir sync bridge is not running. Start the Choir app to sync files."
        case .decodeFailed(let msg):
            return "Failed to decode bridge response: \(msg)"
        case .httpError(let code, let msg):
            return "Bridge error (HTTP \(code)): \(msg)"
        case .invalidResponse:
            return "Invalid response from bridge"
        }
    }
}

/// A single entry in the File Provider domain, mapped from the Go bridge's
/// Entry JSON type.
struct BridgeEntry: Codable {
    let path: String
    let name: String
    let kind: String  // "file", "folder", "conflict"
    let size: Int64
    let modifiedAt: Date
    let syncState: String?
    let conflictPath: String?
    let itemID: String?
}

/// Response from GET /enumerate
struct EnumerateResponse: Codable {
    let path: String
    let entries: [BridgeEntry]
}

/// Response from GET /read
struct ReadResponse: Codable {
    let path: String
    let size: Int64
    let contentB64: String
    let mediaType: String?
    let modifiedAt: Date
}

/// Request for PUT /write
struct WriteRequest: Codable {
    let path: String
    let contentB64: String
    let mediaType: String?
}

/// Response from PUT /write
struct WriteResponse: Codable {
    let path: String
    let syncTriggered: Bool
}

/// Request for POST /mkdir
struct CreateDirRequest: Codable {
    let path: String
}

/// Request for POST /move
struct MoveRequest: Codable {
    let fromPath: String
    let toPath: String
}

/// Request for POST /delete
struct DeleteRequest: Codable {
    let path: String
}

/// A conflict entry from GET /conflicts
struct BridgeConflict: Codable {
    let itemID: String
    let path: String
    let conflictPath: String
    let reason: String
    let resolution: String?
}

/// Response from GET /conflicts
struct ConflictsResponse: Codable {
    let conflicts: [BridgeConflict]
}

/// Response from GET /status
struct StatusResponse: Codable {
    let phase: String
    let lastSyncAt: Date?
    let cursor: Int64
    let remoteHead: Int64
    let conflictsCount: Int
    let lastError: String?
}

/// ErrorResponse is the standard error envelope from the bridge.
struct ErrorResponse: Codable {
    let error: String
}

/// BridgeClient is the HTTP-over-Unix-socket client that talks to the Go
/// sync engine bridge. It is used by the File Provider extension to
/// enumerate, read, and write files in the Choir sync domain.
final class BridgeClient {

    /// The Unix domain socket path for the Go bridge.
    private let socketPath: String

    /// A URL session configured to use the Unix socket via a custom transport.
    private let session: URLSession

    init(socketPath: String) {
        self.socketPath = socketPath
        let config = URLSessionConfiguration.default
        config.timeoutIntervalForRequest = 30
        config.timeoutIntervalForResource = 60
        UnixSocketTransport.configure(socketPath: socketPath)
        config.protocolClasses = [UnixSocketTransport.self]
        self.session = URLSession(configuration: config)
    }

    // MARK: - Public API

    /// Enumerate the children of a directory. Pass empty string for the root.
    func enumerate(path: String) throws -> [BridgeEntry] {
        let qs = path.isEmpty ? "" : "?path=\(urlEncode(path))"
        let data = try get("/enumerate\(qs)")
        let resp = try decode(EnumerateResponse.self, from: data)
        return resp.entries
    }

    /// Read the contents of a file.
    func read(path: String) throws -> Data {
        let data = try get("/read?path=\(urlEncode(path))")
        let resp = try decode(ReadResponse.self, from: data)
        guard let raw = Data(base64Encoded: resp.contentB64) else {
            throw BridgeError.decodeFailed("base64 content")
        }
        return raw
    }

    /// Write file contents (creates or overwrites). Triggers a sync cycle.
    func write(path: String, content: Data, mediaType: String? = nil) throws {
        let req = WriteRequest(
            path: path,
            contentB64: content.base64EncodedString(),
            mediaType: mediaType
        )
        _ = try put("/write", body: req)
    }

    /// Create a directory.
    func createDirectory(path: String) throws {
        let req = CreateDirRequest(path: path)
        _ = try post("/mkdir", body: req)
    }

    /// Move/rename an item.
    func move(from: String, to: String) throws {
        let req = MoveRequest(fromPath: from, toPath: to)
        _ = try post("/move", body: req)
    }

    /// Delete an item.
    func delete(path: String) throws {
        let req = DeleteRequest(path: path)
        _ = try post("/delete", body: req)
    }

    /// List current conflicts.
    func conflicts() throws -> [BridgeConflict] {
        let data = try get("/conflicts")
        let resp = try decode(ConflictsResponse.self, from: data)
        return resp.conflicts
    }

    /// Get sync status.
    func status() throws -> StatusResponse {
        let data = try get("/status")
        return try decode(StatusResponse.self, from: data)
    }

    /// Trigger an immediate sync cycle.
    func syncNow() throws {
        _ = try post("/sync", body: nil as EmptyBody?)
    }

    /// Check if the bridge is reachable.
    func healthCheck() -> Bool {
        do {
            _ = try get("/health")
            return true
        } catch {
            return false
        }
    }

    // MARK: - HTTP helpers

    private struct EmptyBody: Codable {}

    private func get(_ path: String) throws -> Data {
        guard let url = URL(string: "http://unix\(path)") else {
            throw BridgeError.invalidResponse
        }
        return try synchronousRequest(url: url, method: "GET", body: nil)
    }

    private func put<T: Codable>(_ path: String, body: T) throws -> Data {
        guard let url = URL(string: "http://unix\(path)") else {
            throw BridgeError.invalidResponse
        }
        let bodyData = try bridgeEncoder().encode(body)
        return try synchronousRequest(url: url, method: "PUT", body: bodyData)
    }

    private func post<T: Codable>(_ path: String, body: T?) throws -> Data {
        guard let url = URL(string: "http://unix\(path)") else {
            throw BridgeError.invalidResponse
        }
        let bodyData: Data?
        if let body = body {
            bodyData = try bridgeEncoder().encode(body)
        } else {
            bodyData = nil
        }
        return try synchronousRequest(url: url, method: "POST", body: bodyData)
    }

    private func synchronousRequest(url: URL, method: String, body: Data?) throws -> Data {
        var req = URLRequest(url: url)
        req.httpMethod = method
        if let body = body {
            req.httpBody = body
            req.setValue("application/json", forHTTPHeaderField: "Content-Type")
        }

        // Check socket existence first for a better error message.
        if !FileManager.default.fileExists(atPath: socketPath) {
            throw BridgeError.socketUnavailable
        }

        let semaphore = DispatchSemaphore(value: 0)
        var resultData: Data?
        var resultResponse: URLResponse?
        var resultError: Error?

        let task = session.dataTask(with: req) { data, response, error in
            resultData = data
            resultResponse = response
            resultError = error
            semaphore.signal()
        }
        task.resume()
        if semaphore.wait(timeout: .now() + 30) == .timedOut {
            task.cancel()
            throw BridgeError.socketUnavailable
        }

        if let error = resultError {
            throw error
        }

        guard let resp = resultResponse as? HTTPURLResponse else {
            throw BridgeError.invalidResponse
        }

        guard (200..<300).contains(resp.statusCode) else {
            let msg: String
            if let data = resultData {
                if let errResp = try? bridgeDecoder().decode(ErrorResponse.self, from: data) {
                    msg = errResp.error
                } else {
                    msg = "HTTP \(resp.statusCode)"
                }
            } else {
                msg = "HTTP \(resp.statusCode)"
            }
            throw BridgeError.httpError(resp.statusCode, msg)
        }

        guard let data = resultData else {
            throw BridgeError.invalidResponse
        }
        return data
    }

    private func decode<T: Decodable>(_ type: T.Type, from data: Data) throws -> T {
        let decoder = bridgeDecoder()
        do {
            return try decoder.decode(T.self, from: data)
        } catch {
            throw BridgeError.decodeFailed(error.localizedDescription)
        }
    }

    private func urlEncode(_ s: String) -> String {
        s.addingPercentEncoding(withAllowedCharacters: .urlPathAllowed) ?? s
    }

    private func bridgeEncoder() -> JSONEncoder {
        let encoder = JSONEncoder()
        encoder.keyEncodingStrategy = .convertToSnakeCase
        encoder.dateEncodingStrategy = .iso8601
        return encoder
    }

    private func bridgeDecoder() -> JSONDecoder {
        let decoder = JSONDecoder()
        decoder.keyDecodingStrategy = .convertFromSnakeCase
        decoder.dateDecodingStrategy = .iso8601
        return decoder
    }
}
