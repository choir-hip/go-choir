// swift-tools-version: 6.2
import PackageDescription

// AutoradioKit has no third-party dependency: it is also copied, source by source, into the Exact package.
// AutoradioModels holds the on-device models and takes the third-party Swift/Core ML libraries (FluidAudio).
// In the Exact app those libraries arrive as static xcframeworks (mobile/README.md, "Third-party dependencies").
let package = Package(
    name: "AutoradioKit",
    platforms: [.iOS(.v18), .macOS(.v15)],
    products: [
        .library(name: "AutoradioKit", targets: ["AutoradioKit"]),
        .library(name: "AutoradioModels", targets: ["AutoradioModels"]),
        .executable(name: "autoradio-replay", targets: ["autoradio-replay"]),
        .executable(name: "autoradio-models", targets: ["autoradio-models"]),
    ],
    dependencies: [
        // Pinned exactly. The "TTS" trait is Kokoro / PocketTTS / LFM3 speech; Diarizer and NemoTextProcessing are off.
        .package(url: "https://github.com/FluidInference/FluidAudio.git", exact: "0.18.3", traits: ["TTS"]),
    ],
    targets: [
        .target(name: "AutoradioKit", swiftSettings: [.swiftLanguageMode(.v5)]),
        .testTarget(name: "AutoradioKitTests", dependencies: ["AutoradioKit"], swiftSettings: [.swiftLanguageMode(.v5)]),
        // llama.cpp's official release xcframework (dynamic; macOS and iOS-device slices only). The Mac command links
        // it here; the Exact app links a static build of the same tag instead (scripts/build-llama-xcframework.sh).
        .binaryTarget(
            name: "llama",
            url: "https://github.com/ggml-org/llama.cpp/releases/download/b12259/llama-b12259-xcframework.zip",
            checksum: "8b00c7127a5c09a64f4b1a65000d20d66038c4307b11e093fb7c2cea253eca3f"
        ),
        .target(
            name: "AutoradioModels",
            dependencies: ["AutoradioKit", .product(name: "FluidAudio", package: "FluidAudio"),
                           .target(name: "llama", condition: .when(platforms: [.macOS]))],
            swiftSettings: [.swiftLanguageMode(.v5)]
        ),
        .executableTarget(
            name: "autoradio-models",
            dependencies: ["AutoradioKit", "AutoradioModels"],
            swiftSettings: [.swiftLanguageMode(.v5)]
        ),
        .executableTarget(
            name: "autoradio-replay",
            dependencies: ["AutoradioKit", "AutoradioModels"],
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
