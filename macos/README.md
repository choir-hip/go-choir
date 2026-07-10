> **Status (seam-repair 2026-07-10):** The Go File Provider bridge
> (`internal/desktop/fileprovider/`) and SyncEngine were deleted as unwired dead
> code. This Swift extension is orphaned and must not be packaged or registered
> until the PC-5 Base kernel acceptance matrix passes and a new adapter is
> defined. Do not treat the diagrams below as current product authority.

# Choir File Provider Extension (macOS)

This directory contains the macOS File Provider extension that projects
Base-synced files into Finder, with read/write support and conflict files.

## Architecture

```
Finder  ←→  NSFileProviderReplicatedExtension  ←→  BridgeClient  ←→  Go Bridge  ←→  SyncEngine
       (Swift, in .appex)                        (Unix socket)    (Go, in host app)  (Go)
```

The File Provider extension runs as a separate `.appex` process. It cannot
directly call Go code, so it communicates with the Go sync engine via a
Unix domain socket HTTP bridge (`internal/desktop/fileprovider/bridge.go`).

The Go bridge is started by the Choir desktop app (Wails) when the sync
engine is running. The socket path is:

```
~/Library/Application Support/Choir/fileprovider.sock
```

## Files

- `ChoirFileProvider/ChoirFileProviderExtension.swift` — the
  `NSFileProviderReplicatedExtension` implementation (enumerate, read,
  write, create, modify, delete).
- `ChoirFileProvider/ChoirFileProviderBridge.swift` — the HTTP-over-Unix-
  socket client that talks to the Go bridge.
- `ChoirFileProvider/UnixSocketTransport.swift` — a custom `URLProtocol`
  that routes `URLSession` requests through a Unix domain socket.
- `ChoirFileProvider/Info.plist` — the extension's Info.plist with
  `NSExtensionPointIdentifier = com.apple.fileprovider-nonui`.
- `ChoirFileProvider/ChoirFileProvider.entitlements` — development
  entitlements (app sandbox + app group + fileprovider capability).
- `ChoirFileProvider.xcodeproj/` — the Xcode project for building the
  extension target.

## Building (Development)

### Prerequisites

- macOS 13+ (File Provider replicated extensions require macOS 13+)
- Xcode 15+
- A development signing identity (your Apple ID is sufficient for local
  testing; no paid Developer Program membership required for dev builds)

### Steps

1. **Build the Go bridge into the desktop app:**

   The Go bridge is compiled into the Choir desktop app (Wails). When the
   app starts sync, it calls `fileprovider.NewBridge()` and `Start()`.

   ```bash
   nix develop -c go build ./cmd/desktop/
   ```

2. **Build the File Provider extension:**

   ```bash
   open macos/ChoirFileProvider.xcodeproj
   ```

   In Xcode:
   - Select the "ChoirFileProvider" target.
   - Set the signing team to your development team (or "None" for unsigned
     local testing with `CODE_SIGNING_ALLOWED=NO`).
   - Build (Cmd+B).

3. **Install the extension:**

   The built `.appex` must be embedded in the Choir app bundle under
   `Contents/PlugIns/`. For development testing:

   ```bash
   cp -R ~/Library/Developer/Xcode/DerivedData/ChoirFileProvider-*/Build/Products/Debug/ChoirFileProvider.appex \
         ~/Choir.app/Contents/PlugIns/
   ```

4. **Register the domain:**

   The host app registers the File Provider domain on first launch:

   ```swift
   let domain = NSFileProviderDomain(identifier: NSFileProviderDomainIdentifier("news.choir.fileprovider"),
                                      displayName: "Choir")
   NSFileProviderManager.register(domain) { error in
       // ...
   }
   ```

5. **Test in Finder:**

   After registering the domain and starting the Choir app (which starts
   the Go bridge), the Choir domain appears in Finder's sidebar under
   "Locations". Files synced through the Base sync engine appear there.

## Entitlements

### Development (current)

The `ChoirFileProvider.entitlements` file uses:
- `com.apple.security.app-sandbox` = true
- `com.apple.security.application-groups` = `$(TeamIdentifierPrefix)news.choir`
- `com.apple.developer.fileprovider` = true

For unsigned local testing, set `CODE_SIGNING_ALLOWED=NO` in the Xcode
build settings and remove the entitlements file reference.

### Developer ID (later)

For distribution:
1. Obtain a Developer ID Application certificate.
2. Create a provisioning profile that includes the `fileprovider` capability
   and the app group.
3. Set `CODE_SIGN_STYLE = Manual` and `DEVELOPMENT_TEAM` to your team ID.
4. Sign the extension and the host app with the Developer ID certificate.
5. Submit for notarization.

## Conflict Files

Base conflicts are projected as `.conflict` files in Finder. When the sync
engine detects a conflict (both local and remote changed), the bridge's
`/enumerate` endpoint includes a virtual `.conflict` entry alongside the
original file. The `.conflict` file is read-only; the user resolves
conflicts via the Choir app UI (keep local / keep remote / keep both).

## IPC Protocol

The Go bridge serves the following endpoints over a Unix domain socket:

| Method | Path | Description |
|--------|------|-------------|
| GET | `/enumerate?path=...` | List children of a directory |
| GET | `/read?path=...` | Read file contents (base64) |
| PUT | `/write` | Write file contents (base64), triggers sync |
| POST | `/mkdir` | Create a directory |
| POST | `/move` | Rename/move an item |
| POST/DELETE | `/delete` | Delete an item |
| GET | `/conflicts` | List current conflicts |
| GET | `/status` | Get sync status |
| POST | `/sync` | Trigger immediate sync |
| GET | `/health` | Health check |
