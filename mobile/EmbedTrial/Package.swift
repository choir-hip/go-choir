// swift-tools-version: 6.2
import PackageDescription

// The embedding trial (docs/reference/EMBEDDINGGEMMA.md): a Mac command that runs EmbeddingGemma 2 through both routes.
//  A. llama.cpp b12259 (the release xcframework the app already pins) on the official GGUF.
//  B. Core AI, through Everpaper's Swift runner (vendored, unmodified, Apache-2.0). macOS 27 and later.
// Weights are never in the repo: they live in ~/.cache/autoradio-embed-trial/.
let package = Package(
    name: "EmbedTrial",
    platforms: [.macOS("27.0")],
    products: [.executable(name: "embed-trial", targets: ["embed-trial"])],
    dependencies: [.package(path: "../AutoradioKit")],
    targets: [
        .binaryTarget(
            name: "llama-trial",
            url: "https://github.com/ggml-org/llama.cpp/releases/download/b12259/llama-b12259-xcframework.zip",
            checksum: "8b00c7127a5c09a64f4b1a65000d20d66038c4307b11e093fb7c2cea253eca3f"
        ),
        .target(
            name: "EmbeddingGemma2CoreAI",
            path: "Sources/EmbeddingGemma2CoreAI",
            exclude: ["LICENSE", "NOTICE", "VENDORED.md"],
            linkerSettings: [.linkedFramework("CoreAI"), .linkedFramework("ImageIO"), .linkedFramework("CoreGraphics"),
                             .linkedFramework("AVFoundation"), .linkedFramework("Accelerate")]
        ),
        // The first pass (station V3): pure Swift, no model; the embedder is handed in.
        .target(name: "EmbedPass", path: "Sources/EmbedPass"),
        .testTarget(name: "EmbedPassTests", dependencies: ["EmbedPass"], path: "Tests/EmbedPassTests"),
        .executableTarget(
            name: "embed-trial",
            dependencies: [.product(name: "AutoradioKit", package: "AutoradioKit"), "llama-trial", "EmbeddingGemma2CoreAI", "EmbedPass"],
            path: "Sources/embed-trial",
            swiftSettings: [.swiftLanguageMode(.v5)]
        ),
    ]
)
