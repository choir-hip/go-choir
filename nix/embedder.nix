# The host embedder: llama.cpp's server with the official EmbeddingGemma 2
# Q8_0 file, on Node B's loopback (docs/reference/EMBEDDINGGEMMA.md, "The
# server"). corpusd's embed worker and the import's sample check call the bulk
# unit, and a ranked search's query goes to the query unit below
# (internal/platform/vector_*.go); nothing else calls either.
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
# Two units from one definition (mkEmbedderUnit below), so they cannot drift:
#   - go-choir-embedder (port 8090): the bulk unit. corpusd's embed worker and
#     the import's sample check embed documents here, in batches of 4, at a low
#     CPU weight so guests win.
#   - go-choir-embedder-query (port 8091): a ranked corpus search embeds its
#     query here and nowhere else. A query is a guest's own request with a
#     deadline; on the bulk unit it waited for a slot and was then decoded in
#     the same batch as a worker passage, 2 s to 4 s, and 156 of 292 hit the 4 s
#     deadline (docs/problems/search-waits-four-seconds-behind-the-embed-worker-
#     2026-11-19.md). This unit has two slots of a short context, the default
#     CPU weight and no Nice, and sees no passages.
#
# Rollback: remove the import in node-b.nix and deploy; /var/lib/go-choir-embedder*
# (the 310 MB files) are left in place. corpusd's worker then logs that the
# embedder is down and backs off; stored vectors stay and searches still run
# (without the vector half when the query unit is gone). To drop only the query
# unit: remove its entry and CORPUSD_EMBEDDER_QUERY_URL; queries then go to the
# bulk unit again.
{ config, lib, pkgs, ... }:
let
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

  fetchModel = name: stateDir: pkgs.writeShellScript "${name}-fetch" ''
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

  # One definition of an embedder unit. Everything not a parameter is the same
  # for both units: the package, the model file and its SHA-256 check, the
  # sandboxing, --embeddings with mean pooling.
  mkEmbedderUnit =
    { name          # unit name and state directory name
    , description
    , port
    , threads
    , parallel      # slots
    , ctxSize       # total context; each slot gets ctxSize / parallel (see below)
    , batchSize     # with --embeddings the server runs with batch = ubatch whatever this says
    , ubatchSize    # the largest input a slot takes (an embedding task cannot be split)
    , cpuWeight
    , nice ? null   # null: no Nice line
    , ioWeight
    , memoryMax
    , oomScoreAdjust
    , restartSec
    }:
    let
      # Not shared with the other unit's folder: both run as DynamicUser, so each
      # unit's StateDirectory is a private 0750 directory of its own transient
      # user, and one cannot read the other's. Each unit fetches and checks its
      # own copy of the model (310 MB, once).
      stateDir = "/var/lib/${name}";
    in
    {
      inherit description;
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
        StateDirectory = name;
        StateDirectoryMode = "0750";
        # Reads nothing of the host's.
        InaccessiblePaths = [ "/var/lib/go-choir/secrets" "/var/lib/go-choir/unit-env" ];
        ExecStartPre = fetchModel name stateDir;
        # --embeddings with mean pooling (the file's own); the server returns the
        # pooled vector normalised, corpusd cuts it to 256 and normalises again.
        # Context per slot is ctxSize / parallel (src/llama-context.cpp:429-432,
        # EMBEDDINGGEMMA.md 0.11.2); an input over the slot's context or over
        # --ubatch-size is refused with an error, never cut.
        ExecStart = lib.concatStringsSep " " [
          "${llamaCpp}/bin/llama-server"
          "--model ${stateDir}/${model.file}"
          "--embeddings"
          "--pooling mean"
          "--host 127.0.0.1"
          "--port ${toString port}"
          "--threads ${toString threads}"
          "--parallel ${toString parallel}"
          "--ctx-size ${toString ctxSize}"
          "--batch-size ${toString batchSize}"
          "--ubatch-size ${toString ubatchSize}"
        ];
        Restart = "on-failure";
        RestartSec = restartSec;
        CPUWeight = cpuWeight;
        IOWeight = ioWeight;
        MemoryMax = memoryMax;
        OOMScoreAdjust = oomScoreAdjust;
        TimeoutStartSec = "30min";
      } // lib.optionalAttrs (nice != null) { Nice = nice; };
    };
in
{
  # Bulk: documents (the worker, the import). Guests win: a low weight and a
  # low priority under CPU contention. Four threads: a size, not a limit.
  systemd.services.go-choir-embedder = mkEmbedderUnit {
    name = "go-choir-embedder";
    description = "go-choir EmbeddingGemma 2 embedder (llama.cpp server, loopback only)";
    port = 8090;
    threads = 4;
    # 2 slots of 4096 / 2 = 2048 tokens, the model's own input length.
    parallel = 2;
    ctxSize = 4096;
    batchSize = 2048;
    ubatchSize = 2048;
    cpuWeight = 10;
    nice = 10;
    ioWeight = 10;
    # 310 MB of weights, two 2048-token slots and the process: well under 1 GiB.
    memoryMax = "1536M";
    # Under host memory pressure the kernel kills this before a guest (and
    # before the query unit).
    oomScoreAdjust = 500;
    restartSec = 60;
  };

  # Queries: one short input per search, from guests who are waiting.
  # What this unit buys is that a query never waits behind a passage, and the
  # default CPU weight. It does not buy speed from slots. Measured 2026-11-19
  # on the dev Mac with this build, CPU only, four threads (EMBEDDINGGEMMA.md
  # 0.11, "A second unit for queries"): a short query alone 27 ms; 8 at once
  # all done by 200 ms and 32 at once by 800 ms whether the server had 2, 4
  # or 8 slots, so the cost is the sum of the tokens and a burst is served at
  # one query's rate. Two slots had the better median (121 ms against 202 ms
  # for a burst of 8), so two.
  #   context: 1024 / 2 = 512 tokens a slot, and --ubatch-size 512 is the same
  #     bound. A query is "task: search result | query: " plus a few words,
  #     tens of tokens. One over 512 tokens is refused at once (seen: HTTP 500,
  #     "input (550 tokens) is too large to process. increase the physical
  #     batch size (current batch size: 512)"); corpusd takes that as "vector
  #     half unavailable" and answers from the page text. It is never cut and
  #     never retried on the bulk unit. The bound is also about what the 4 s
  #     deadline could hold: a 360-token query took 740 ms on the Mac, and
  #     Node B's threads are several times slower.
  #   --batch-size 512: with --embeddings the server sets the batch to the
  #     ubatch whatever is asked (its log: "setting n_batch = n_ubatch = 512"),
  #     so the flag says what runs.
  #   weight: the default CPU weight and no Nice, so the kernel treats it as a
  #     guest's own work, not as background.
  systemd.services.go-choir-embedder-query = mkEmbedderUnit {
    name = "go-choir-embedder-query";
    description = "go-choir EmbeddingGemma 2 embedder, queries (llama.cpp server, loopback only)";
    port = 8091;
    threads = 4;
    parallel = 2;
    ctxSize = 1024;
    batchSize = 512;
    ubatchSize = 512;
    cpuWeight = 100;
    ioWeight = 100;
    # Measured on the Mac with these flags after bursts and a 360-token
    # query: 591 MB resident, the 310 MB model file (mapped, file-backed,
    # reclaimable) included. Not measured on x86. 1 GiB is the ceiling.
    memoryMax = "1024M";
    # Dies before a guest, after the bulk unit (500). If it dies, searches lose
    # their vector half until it is back, so the restart is sooner than the
    # bulk unit's; not so soon that a model file whose hash keeps failing is
    # downloaded again every few seconds.
    oomScoreAdjust = 300;
    restartSec = 30;
  };
}
