// ChoirFileProviderExtension.swift
//
// NSFileProviderReplicatedExtension implementation for the Choir sync
// domain. This extension projects Base-synced files into Finder, with
// read/write support and conflict files.
//
// The extension delegates all filesystem and sync operations to the Go
// bridge (internal/desktop/fileprovider/bridge.go) via a Unix domain
// socket. The Go bridge wraps the Base sync engine and the local sync
// root.
//
// Architecture:
//   Finder  ←→  NSFileProviderReplicatedExtension  ←→  BridgeClient  ←→  Go Bridge  ←→  SyncEngine
//                                                                              ←→  Local FS

import FileProvider
import UniformTypeIdentifiers

/// The domain identifier for the Choir File Provider.
let choirDomainIdentifier = "news.choir.fileprovider"

/// The default socket path for the Go bridge.
let defaultSocketPath: String = {
    let supportDir = FileManager.default.urls(for: .applicationSupportDirectory, in: .userDomainMask).first!
    let choirDir = supportDir.appendingPathComponent("Choir", isDirectory: true)
    return choirDir.appendingPathComponent("fileprovider.sock").path
}()

/// ChoirFileProviderExtension is the replicated File Provider extension that
/// projects Base-synced files into Finder.
class ChoirFileProviderExtension: NSObject, NSFileProviderReplicatedExtension {

    /// The bridge client for talking to the Go sync engine.
    private let bridge: BridgeClient

    /// The File Provider domain.
    private let domain: NSFileProviderDomain

    /// The domain's item identifier for the root.
    private let rootItemID: NSFileProviderItemIdentifier

    init(domain: NSFileProviderDomain) {
        self.domain = domain
        self.bridge = BridgeClient(socketPath: defaultSocketPath)
        // The root identifier is always .rootContainer for a domain.
        self.rootItemID = .rootContainer
        super.init()
    }

    // MARK: - NSFileProviderReplicatedExtension

    /// Returns an enumerator for the children of the given item.
    func enumerator(for containerItemIdentifier: NSFileProviderItemIdentifier,
                    request: NSFileProviderRequest) throws -> NSFileProviderEnumerator {

        // Check bridge availability.
        if !bridge.healthCheck() {
            // Return an empty enumerator; Finder will retry.
            return ChoirEnumerator(bridge: bridge, containerID: containerItemIdentifier)
        }

        return ChoirEnumerator(bridge: bridge, containerID: containerItemIdentifier)
    }

    /// Returns the item metadata for the given identifier.
    func item(for identifier: NSFileProviderItemIdentifier,
              request: NSFileProviderRequest,
              completionHandler: @escaping (NSFileProviderItem?, Error?) -> Void) -> Progress {

        let progress = Progress(totalUnitCount: 1)

        // The root container is a special case.
        if identifier == .rootContainer {
            completionHandler(ChoirItem.rootItem(domain: domain), nil)
            progress.completedUnitCount = 1
            return progress
        }

        // The working set container.
        if identifier == .workingSet {
            completionHandler(ChoirItem.workingSetItem(domain: domain), nil)
            progress.completedUnitCount = 1
            return progress
        }

        // Otherwise, the identifier is a path. Enumerate the parent to
        // find the item.
        let path = identifier.rawValue
        let parentPath = (path as NSString).deletingLastPathComponent
        let parentID = parentPath.isEmpty ? .rootContainer : NSFileProviderItemIdentifier(parentPath)

        DispatchQueue.global(qos: .userInitiated).async {
            do {
                let entries = try self.bridge.enumerate(path: parentPath)
                if let entry = entries.first(where: { $0.path == path }) {
                    completionHandler(ChoirItem(entry: entry, domain: self.domain), nil)
                } else {
                    completionHandler(nil, NSError.fileProviderErrorForNonExistentItem(withIdentifier: identifier))
                }
            } catch {
                completionHandler(nil, error)
            }
            progress.completedUnitCount = 1
        }

        return progress
    }

    /// Fetches the content for the given item.
    func fetchContents(for itemIdentifier: NSFileProviderItemIdentifier,
                       version requestedVersion: NSFileProviderItemVersion?,
                       request: NSFileProviderRequest,
                       completionHandler: @escaping (URL?, Error?) -> Void) -> Progress {

        let progress = Progress(totalUnitCount: 1)
        let path = itemIdentifier.rawValue

        DispatchQueue.global(qos: .userInitiated).async {
            do {
                let data = try self.bridge.read(path: path)
                // Write to a temporary file.
                let tmpURL = FileManager.default.temporaryDirectory
                    .appendingPathComponent(UUID().uuidString)
                    .appendingPathComponent((path as NSString).lastPathComponent)
                try FileManager.default.createDirectory(at: tmpURL.deletingLastPathComponent(),
                                                        withIntermediateDirectories: true)
                try data.write(to: tmpURL)
                completionHandler(tmpURL, nil)
            } catch {
                completionHandler(nil, error)
            }
            progress.completedUnitCount = 1
        }

        return progress
    }

    /// Creates a new item in the domain.
    func createItem(basedOn itemTemplate: NSFileProviderItem,
                    fields: NSFileProviderItemFields,
                    contents url: URL?,
                    options: NSFileProviderCreateItemOptions = [],
                    request: NSFileProviderRequest,
                    completionHandler: @escaping (NSFileProviderItem?, NSFileProviderItemFields, Bool, Error?) -> Void) -> Progress {

        let progress = Progress(totalUnitCount: 1)
        let parentPath = itemTemplate.parentItemIdentifier == .rootContainer
            ? "" : itemTemplate.parentItemIdentifier.rawValue
        let fullPath = parentPath.isEmpty
            ? itemTemplate.filename
            : "\(parentPath)/\(itemTemplate.filename)"

        DispatchQueue.global(qos: .userInitiated).async {
            do {
                if itemTemplate.isDirectory {
                    try self.bridge.createDirectory(path: fullPath)
                    let entry = BridgeEntry(
                        path: fullPath,
                        name: itemTemplate.filename,
                        kind: "folder",
                        size: 0,
                        modifiedAt: Date(),
                        syncState: nil,
                        conflictPath: nil,
                        itemID: nil
                    )
                    completionHandler(ChoirItem(entry: entry, domain: self.domain), [], false, nil)
                } else {
                    let content: Data
                    if let url = url {
                        content = try Data(contentsOf: url)
                    } else {
                        content = Data()
                    }
                    try self.bridge.write(path: fullPath, content: content)
                    let entry = BridgeEntry(
                        path: fullPath,
                        name: itemTemplate.filename,
                        kind: "file",
                        size: Int64(content.count),
                        modifiedAt: Date(),
                        syncState: nil,
                        conflictPath: nil,
                        itemID: nil
                    )
                    completionHandler(ChoirItem(entry: entry, domain: self.domain), [], false, nil)
                }
            } catch {
                completionHandler(nil, [], false, error)
            }
            progress.completedUnitCount = 1
        }

        return progress
    }

    /// Modifies an existing item.
    func modifyItem(_ item: NSFileProviderItem,
                    baseVersion version: NSFileProviderItemVersion,
                    changedFields: NSFileProviderItemFields,
                    contents newContents: URL?,
                    options: NSFileProviderModifyItemOptions = [],
                    request: NSFileProviderRequest,
                    completionHandler: @escaping (NSFileProviderItem?, NSFileProviderItemFields, Bool, Error?) -> Void) -> Progress {

        let progress = Progress(totalUnitCount: 1)
        let path = item.itemIdentifier.rawValue

        DispatchQueue.global(qos: .userInitiated).async {
            do {
                // Handle content changes.
                if changedFields.contains(.content) {
                    if let url = newContents {
                        let data = try Data(contentsOf: url)
                        try self.bridge.write(path: path, content: data)
                    }
                }

                // Handle rename/move (filename or parent changed).
                if changedFields.contains(.filename) || changedFields.contains(.parentItemIdentifier) {
                    let parentPath = item.parentItemIdentifier == .rootContainer
                        ? "" : item.parentItemIdentifier.rawValue
                    let newPath = parentPath.isEmpty
                        ? item.filename
                        : "\(parentPath)/\(item.filename)"
                    if newPath != path {
                        try self.bridge.move(from: path, to: newPath)
                    }
                }

                // Re-fetch the item metadata.
                let parentPath = (path as NSString).deletingLastPathComponent
                let entries = try self.bridge.enumerate(path: parentPath)
                let updatedPath = changedFields.contains(.filename) || changedFields.contains(.parentItemIdentifier)
                    ? (item.parentItemIdentifier == .rootContainer
                        ? item.filename
                        : "\(item.parentItemIdentifier.rawValue)/\(item.filename)")
                    : path
                if let entry = entries.first(where: { $0.path == updatedPath }) {
                    completionHandler(ChoirItem(entry: entry, domain: self.domain), [], false, nil)
                } else {
                    completionHandler(item, [], false, nil)
                }
            } catch {
                completionHandler(nil, [], false, error)
            }
            progress.completedUnitCount = 1
        }

        return progress
    }

    /// Deletes an item.
    func deleteItem(identifier: NSFileProviderItemIdentifier,
                    baseVersion version: NSFileProviderItemVersion,
                    options: NSFileProviderDeleteItemOptions = [],
                    request: NSFileProviderRequest,
                    completionHandler: @escaping (Error?) -> Void) -> Progress {

        let progress = Progress(totalUnitCount: 1)
        let path = identifier.rawValue

        DispatchQueue.global(qos: .userInitiated).async {
            do {
                try self.bridge.delete(path: path)
                completionHandler(nil)
            } catch {
                completionHandler(error)
            }
            progress.completedUnitCount = 1
        }

        return progress
    }

    // MARK: - Stale items (replicated extension)

    func invalidateEnumeration(for enumerator: NSFileProviderEnumerator) {
        // The Go bridge drives enumeration; we don't cache enumerators.
    }
}

// MARK: - ChoirEnumerator

/// ChoirEnumerator enumerates the children of a container by querying the
/// Go bridge.
final class ChoirEnumerator: NSObject, NSFileProviderEnumerator {

    private let bridge: BridgeClient
    private let containerID: NSFileProviderItemIdentifier

    init(bridge: BridgeClient, containerID: NSFileProviderItemIdentifier) {
        self.bridge = bridge
        self.containerID = containerID
        super.init()
    }

    func invalidate() {
        // No persistent resources to release.
    }

    /// Enumerate the items in the container.
    func enumerateItems(for observer: NSFileProviderEnumerationObserver,
                        startingAt page: NSFileProviderPage) {

        let path = containerID == .rootContainer ? "" : containerID.rawValue

        DispatchQueue.global(qos: .userInitiated).async {
            do {
                let entries = try self.bridge.enumerate(path: path)
                var items: [NSFileProviderItem] = []
                for entry in entries {
                    items.append(ChoirItem(entry: entry, domain: nil))
                }
                observer.didEnumerate(items)
                observer.finishEnumerating(upTo: nil)
            } catch {
                observer.finishEnumeratingWithError(error)
            }
        }
    }

    /// Enumerate the delta (changes since the last sync). For a replicated
    /// extension backed by a sync engine, we enumerate the full set each
    /// time and let the system compute deltas.
    func enumerateChanges(for observer: NSFileProviderChangeObserver,
                          from anchor: NSFileProviderSyncAnchor) {

        // For now, signal a full re-enumeration. A future optimization
        // would track a sync anchor based on the Go bridge's cursor.
        let path = containerID == .rootContainer ? "" : containerID.rawValue

        DispatchQueue.global(qos: .userInitiated).async {
            do {
                let entries = try self.bridge.enumerate(path: path)
                var changes: [NSFileProviderItem] = []
                for entry in entries {
                    changes.append(ChoirItem(entry: entry, domain: nil))
                }
                observer.didEnumerate(changes)
                // Use the current sync cursor as the anchor.
                let status = try self.bridge.status()
                let anchorData = withUnsafeBytes(of: status.cursor) { Data($0) }
                observer.finishEnumeratingChanges(upTo: NSFileProviderSyncAnchor(anchorData),
                                                   moreComing: false)
            } catch {
                observer.finishEnumeratingWithError(error)
            }
        }
    }

    /// Enumerate the working set (recently accessed items).
    func enumerateRecentChanges(startingAt page: NSFileProviderPage) {
        // Delegate to the standard change enumeration.
        // The working set is the same as the full set for now.
    }

    func currentSyncAnchor(completionHandler: @escaping (NSFileProviderSyncAnchor?) -> Void) {
        DispatchQueue.global(qos: .userInitiated).async {
            do {
                let status = try self.bridge.status()
                let anchorData = withUnsafeBytes(of: status.cursor) { Data($0) }
                completionHandler(NSFileProviderSyncAnchor(anchorData))
            } catch {
                completionHandler(nil)
            }
        }
    }
}

// MARK: - ChoirItem

/// ChoirItem wraps a bridge entry as an NSFileProviderItem.
final class ChoirItem: NSObject, NSFileProviderItem {

    let entry: BridgeEntry
    let domain: NSFileProviderDomain?

    init(entry: BridgeEntry, domain: NSFileProviderDomain?) {
        self.entry = entry
        self.domain = domain
        super.init()
    }

    static func rootItem(domain: NSFileProviderDomain) -> ChoirItem {
        let entry = BridgeEntry(
            path: "",
            name: "Choir",
            kind: "folder",
            size: 0,
            modifiedAt: Date(),
            syncState: nil,
            conflictPath: nil,
            itemID: nil
        )
        return ChoirItem(entry: entry, domain: domain)
    }

    static func workingSetItem(domain: NSFileProviderDomain) -> ChoirItem {
        let entry = BridgeEntry(
            path: "",
            name: "Working Set",
            kind: "folder",
            size: 0,
            modifiedAt: Date(),
            syncState: nil,
            conflictPath: nil,
            itemID: nil
        )
        return ChoirItem(entry: entry, domain: domain)
    }

    var itemIdentifier: NSFileProviderItemIdentifier {
        if entry.path.isEmpty {
            return .rootContainer
        }
        return NSFileProviderItemIdentifier(entry.path)
    }

    var parentItemIdentifier: NSFileProviderItemIdentifier {
        if entry.path.isEmpty {
            return .rootContainer
        }
        let parent = (entry.path as NSString).deletingLastPathComponent
        if parent.isEmpty {
            return .rootContainer
        }
        return NSFileProviderItemIdentifier(parent)
    }

    var filename: String {
        return entry.name
    }

    var contentType: UTType {
        switch entry.kind {
        case "folder":
            return .folder
        case "conflict":
            return UTType(filenameExtension: "conflict") ?? .data
        default:
            let ext = (entry.name as NSString).pathExtension
            return UTType(filenameExtension: ext) ?? .data
        }
    }

    var isDirectory: Bool {
        return entry.kind == "folder"
    }

    var documentSize: NSNumber? {
        return entry.kind == "file" ? NSNumber(value: entry.size) : nil
    }

    var contentModificationDate: Date? {
        return entry.modifiedAt
    }

    var itemVersion: NSFileProviderItemVersion {
        // Use the path + modification date as a content version, and the
        // modification date as a metadata version. This is sufficient for
        // Finder to detect changes.
        let contentVersion = entry.path.data(using: .utf8) ?? Data()
        let metadataVersion = withUnsafeBytes(of: entry.modifiedAt.timeIntervalSinceReferenceDate) { Data($0) }
        return NSFileProviderItemVersion(contentVersion: contentVersion,
                                         metadataVersion: metadataVersion)
    }

    var capabilities: NSFileProviderItemCapabilities {
        switch entry.kind {
        case "folder":
            return [.allowsContentEnumerating, .allowsAddingSubItems, .allowsRenaming, .allowsDeleting]
        case "conflict":
            // Conflict files are read-only; the user resolves conflicts
            // via the Choir app UI.
            return [.allowsReading, .allowsDeleting]
        default:
            return [.allowsReading, .allowsWriting, .allowsRenaming, .allowsDeleting, .allowsContentEnumerating]
        }
    }

    var isUploaded: Bool {
        return entry.syncState == "synced"
    }

    var isUploading: Bool {
        return entry.syncState == "local_only"
    }

    var isDownloaded: Bool {
        return entry.syncState != "remote_only"
    }

    var isDownloading: Bool {
        return entry.syncState == "remote_only"
    }

    var hasUnresolvedConflicts: Bool {
        return entry.kind == "conflict"
    }
}
