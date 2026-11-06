Package trees from another publisher, for the sandboxed mode of the contract
runner (MA6.1): `node scripts/package-contracts.mjs --packages-dir
tests/fixtures/sandboxed-packages --modes sandboxed`. Each tree is listed as an
installed release on a stranger's track, so the shell runs it in the
opaque-origin frame and every SDK call crosses the bridge.
