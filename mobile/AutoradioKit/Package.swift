// swift-tools-version: 6.0
import PackageDescription

let package = Package(
    name: "AutoradioKit",
    platforms: [.iOS(.v18), .macOS(.v15)],
    products: [
        .library(name: "AutoradioKit", targets: ["AutoradioKit"]),
        .executable(name: "autoradio-replay", targets: ["autoradio-replay"]),
    ],
    targets: [
        .target(name: "AutoradioKit", swiftSettings: [.swiftLanguageMode(.v5)]),
        .testTarget(name: "AutoradioKitTests", dependencies: ["AutoradioKit"], swiftSettings: [.swiftLanguageMode(.v5)]),
        .executableTarget(
            name: "autoradio-replay",
            dependencies: ["AutoradioKit"],
            exclude: ["Info.plist"],
            swiftSettings: [.swiftLanguageMode(.v5)],
            linkerSettings: [
                // Speech recognition needs usage strings; a bare command has no bundle.
                .unsafeFlags(["-Xlinker", "-sectcreate", "-Xlinker", "__TEXT", "-Xlinker", "__info_plist",
                              "-Xlinker", "Sources/autoradio-replay/Info.plist"])
            ]
        ),
    ]
)
