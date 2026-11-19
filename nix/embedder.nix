# The host embedder: llama.cpp's server with the official EmbeddingGemma 2
# Q8_0 file, on Node B's loopback (docs/reference/EMBEDDINGGEMMA.md, "The
# server"). corpusd's embed worker and its vector search call it
# (internal/platform/vector_*.go); nothing else does.
#
# Three pins, each stated once here:
#   - llama.cpp b12259 (351025ec5, 2026-11-10). The nixpkgs pin carries 0.6.0
#     (2026-10-05), which has no `gemma-embedding2` architecture (the file does
#     not load), so the package is overridden to the commit the Mac trial ran.
#   - the model file, by SHA-256, at one Hub commit. It is fetched when the unit
#     starts, never kept in the repo or the Nix store; a file whose hash differs
#     is deleted and the unit fails and retries.
#   - four threads: a size, not a limit. The server has no request limit and
#     counts nothing.
#
# Rollback: remove the import in node-b.nix and deploy; /var/lib/go-choir-embedder
# (the 310 MB file) is left in place. corpusd's worker then logs that the
# embedder is down and backs off; stored vectors stay and searches still run.
{ config, lib, pkgs, ... }:
let
  port = 8090;

  llamaCpp = pkgs.llama-cpp.overrideAttrs (old: {
    version = "0.6.0-b12259";
    src = pkgs.fetchFromGitHub {
      owner = "ggml-org";
      repo = "llama.cpp";
      rev = "351025ec532f825c7eb6c7ae1017d8e6d46d0cc9"; # tag b12259
      hash = "sha256-tB+aeXiqzy9D5tBHLNy+AJRjfilggVaX/ppA4YlGM70=";
    };
    # The nixpkgs patch is for a test fixture of 0.6.0; the tests are not run
    # here (a CPU-only server build, checked by loading the model instead).
    patches = [ ];
    doCheck = false;
    # The web UI's npm dependencies are those of tools/ui at b12259.
    npmDepsHash = "sha256-34AKbVBkQZI1+z83Kg4hA43YbxkeDsESKnIt3T5WdL8=";
    # nixpkgs bakes 0.6.0's build number into the binary and /props; later
    # flags win.
    cmakeFlags = old.cmakeFlags ++ [
      "-DLLAMA_BUILD_NUMBER=12259"
      "-DLLAMA_BUILD_COMMIT=351025e"
    ];
  });

  model = {
    file = "embeddinggemma-2-Q8_0.gguf";
    # Full SHA-256 and size of ggml-org/embeddinggemma-2-GGUF at the commit in
    # the URL (the Hub's LFS listing, 2026-11-19). Apache-2.0 with the Gemma
    # Prohibited Use Policy; the owner has ruled the licence fine.
    sha256 = "2188ac1deca4b77dffefd603c2776a9d76d9d74ec01841392982ebb840b09135";
    url = "https://huggingface.co/ggml-org/embeddinggemma-2-GGUF/resolve/6ea3e3a309d12a4779cfda98b8c19800d2d5da71/embeddinggemma-2-Q8_0.gguf";
  };

  stateDir = "/var/lib/go-choir-embedder";

  fetchModel = pkgs.writeShellScript "go-choir-embedder-fetch" ''
    set -euo pipefail
    export PATH=${lib.makeBinPath [ pkgs.coreutils pkgs.curl ]}
    f=${stateDir}/${model.file}
    want=${model.sha256}
    if [ -f "$f" ] && [ "$(sha256sum "$f" | cut -d' ' -f1)" = "$want" ]; then
      exit 0
    fi
    rm -f "$f" "$f.part"
    echo "embedder model: fetching ${model.file}"
    curl --fail --location --silent --show-error --retry 5 --retry-delay 10 \
      --output "$f.part" ${lib.escapeShellArg model.url}
    got="$(sha256sum "$f.part" | cut -d' ' -f1)"
    if [ "$got" != "$want" ]; then
      echo "embedder model: sha256 mismatch (got $got), file removed" >&2
      rm -f "$f.part"
      exit 1
    fi
    mv "$f.part" "$f"
    echo "embedder model: ${model.file} verified"
  '';
in
{
  systemd.services.go-choir-embedder = {
    description = "go-choir EmbeddingGemma 2 embedder (llama.cpp server, loopback only)";
    wantedBy = [ "multi-user.target" ];
    after = [ "network-online.target" ];
    wants = [ "network-online.target" ];
    # Not part of a deploy's critical path: a new binary or flags take effect
    # on the unit's own restart, and nothing waits for it.
    # It holds nothing and calls only the Hub, so a failing fetch retries
    # forever instead of tripping the start limit.
    startLimitIntervalSec = 0;
    serviceConfig = {
      ProtectSystem = "strict";
      PrivateTmp = true;
      NoNewPrivileges = true;
      ProtectKernelModules = true;
      ProtectKernelTunables = true;
      ProtectControlGroups = true;
      SystemCallArchitectures = "native";
      PrivateDevices = true;
      SystemCallFilter = [ "@system-service" "~@privileged" "~@mount" ];
      SystemCallErrorNumber = "EPERM";
      RestrictAddressFamilies = [ "AF_INET" "AF_INET6" "AF_UNIX" ];
      DynamicUser = true;
      StateDirectory = "go-choir-embedder";
      StateDirectoryMode = "0750";
      # Reads nothing of the host's.
      InaccessiblePaths = [ "/var/lib/go-choir/secrets" "/var/lib/go-choir/unit-env" ];
      ExecStartPre = fetchModel;
      # --embeddings with mean pooling (the file's own); the server returns the
      # pooled vector normalised, corpusd cuts it to 256 and normalises again.
      # --parallel 2 with 4096 context gives each slot the model's 2048 tokens.
      ExecStart = lib.concatStringsSep " " [
        "${llamaCpp}/bin/llama-server"
        "--model ${stateDir}/${model.file}"
        "--embeddings"
        "--pooling mean"
        "--host 127.0.0.1"
        "--port ${toString port}"
        "--threads 4"
        "--parallel 2"
        "--ctx-size 4096"
        "--batch-size 2048"
        "--ubatch-size 2048"
      ];
      Restart = "on-failure";
      RestartSec = 60;
      # Guests win: a low weight and a low priority under CPU contention.
      CPUWeight = 10;
      Nice = 10;
      IOWeight = 10;
      # 310 MB of weights, two 2048-token slots and the process: well under 1 GiB.
      MemoryMax = "1536M";
      # Under host memory pressure the kernel kills this before a guest.
      OOMScoreAdjust = 500;
      TimeoutStartSec = "30min";
    };
  };
}
