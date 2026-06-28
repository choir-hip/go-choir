// UnixSocketTransport.swift
//
// A custom URLProtocol that routes HTTP requests through a Unix domain
// socket. This allows URLSession to talk to the Go bridge's Unix-socket
// HTTP server without a TCP port.
//
// The protocol intercepts requests whose URL scheme is "http" and host is
// "unix" (a sentinel), connects to the configured Unix socket, sends the
// HTTP request, and returns the response.

import Foundation

/// UnixSocketTransport routes HTTP requests through a Unix domain socket.
/// It is registered as a custom URLProtocol in the URLSession configuration.
final class UnixSocketTransport: URLProtocol {

    /// The socket path to connect to.
    private let socketPath: String

    /// The active socket connection.
    private var socket: FileHandle?

    /// The accumulated response data.
    private var responseData = Data()

    /// The HTTP response being built.
    private var httpResponse: HTTPURLResponse?

    init(socketPath: String) {
        self.socketPath = socketPath
        super.init(request: URLRequest(url: URL(string: "http://unix/")!,
                                       cachePolicy: .reloadIgnoringLocalCacheData,
                                       timeoutInterval: 30))
    }

    // MARK: - URLProtocol

    override class func canInit(with request: URLRequest) -> Bool {
        guard let url = request.url else { return false }
        return url.scheme == "http" && url.host == "unix"
    }

    override class func canonicalRequest(for request: URLRequest) -> URLRequest {
        return request
    }

    override func startLoading() {
        guard let url = request.url else {
            client?.urlProtocol(self, didFailWithError: BridgeError.invalidResponse)
            return
        }

        // Connect to the Unix socket.
        let fd = socketPath.withCString { path -> Int32 in
            return Darwin.socket(AF_UNIX, SOCK_STREAM, 0)
        }

        guard fd >= 0 else {
            client?.urlProtocol(self, didFailWithError: BridgeError.socketUnavailable)
            return
        }

        var addr = sockaddr_un()
        addr.sun_family = sa_family_t(AF_UNIX)
        socketPath.withCString { path in
            withUnsafeMutableBytes(of: &addr.sun_path) { buf in
                let pathBytes = path.utf8CString
                buf.copyBytes(from: pathBytes)
            }
        }

        let connected = withUnsafePointer(to: &addr) { addrPtr -> Int32 in
            addrPtr.withMemoryRebound(to: sockaddr.self, capacity: 1) { sockaddrPtr in
                return Darwin.connect(fd, sockaddrPtr, socklen_t(MemoryLayout<sockaddr_un>.size))
            }
        }

        guard connected == 0 else {
            Darwin.close(fd)
            client?.urlProtocol(self, didFailWithError: BridgeError.socketUnavailable)
            return
        }

        socket = FileHandle(fileDescriptor: fd, closeOnDealloc: true)

        // Build the HTTP request string.
        var httpRequest = "\(request.httpMethod ?? "GET") \(url.path)\(url.query.map { "?" + $0 } ?? "") HTTP/1.1\r\n"
        httpRequest += "Host: unix\r\n"
        httpRequest += "Connection: close\r\n"
        if let body = request.httpBody {
            httpRequest += "Content-Length: \(body.count)\r\n"
        }
        for (key, value) in request.allHTTPHeaderFields ?? [:] {
            if key.lowercased() != "host" && key.lowercased() != "connection" {
                httpRequest += "\(key): \(value)\r\n"
            }
        }
        httpRequest += "\r\n"

        // Send the request.
        let requestBytes = httpRequest.data(using: .utf8) ?? Data()
        socket?.write(requestBytes)
        if let body = request.httpBody {
            socket?.write(body)
        }

        // Read the response.
        readResponse()
    }

    override func stopLoading() {
        socket?.close()
        socket = nil
    }

    // MARK: - Response reading

    private func readResponse() {
        // Read all available data until EOF.
        let data = socket?.readDataToEndOfFile() ?? Data()
        parseResponse(data)
    }

    private func parseResponse(_ data: Data) {
        // Find the header/body boundary (\r\n\r\n).
        let separator = Data([0x0D, 0x0A, 0x0D, 0x0A])
        guard let range = data.range(of: separator) else {
            client?.urlProtocol(self, didFailWithError: BridgeError.invalidResponse)
            return
        }

        let headerData = data.subdata(in: 0..<range.lowerBound)
        let bodyData = data.subdata(in: range.upperBound..<data.endIndex)

        guard let headerStr = String(data: headerData, encoding: .utf8) else {
            client?.urlProtocol(self, didFailWithError: BridgeError.invalidResponse)
            return
        }

        // Parse status line and headers.
        let lines = headerStr.components(separatedBy: "\r\n")
        guard let firstLine = lines.first else {
            client?.urlProtocol(self, didFailWithError: BridgeError.invalidResponse)
            return
        }

        // "HTTP/1.1 200 OK"
        let parts = firstLine.components(separatedBy: " ")
        guard parts.count >= 2, let statusCode = Int(parts[1]) else {
            client?.urlProtocol(self, didFailWithError: BridgeError.invalidResponse)
            return
        }

        var headers = [String: String]()
        for line in lines.dropFirst() {
            if let colonIdx = line.firstIndex(of: ":") {
                let key = String(line[..<colonIdx]).trimmingCharacters(in: .whitespaces)
                let value = String(line[line.index(after: colonIdx)...]).trimmingCharacters(in: .whitespaces)
                headers[key] = value
            }
        }

        let response = HTTPURLResponse(
            url: request.url!,
            statusCode: statusCode,
            httpVersion: "HTTP/1.1",
            headerFields: headers
        )!

        client?.urlProtocol(self, didReceive: response, cacheStoragePolicy: .notAllowed)
        client?.urlProtocol(self, didLoad: bodyData)
        client?.urlProtocolDidFinishLoading(self)
    }
}
