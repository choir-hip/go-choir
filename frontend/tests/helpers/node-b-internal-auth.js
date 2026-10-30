// Host-internal credential for the internal routes of Node B's host services
// (vmctl 8083, gateway 8084, corpusd 8086, maild 8087; internal/hostauth,
// docs/problems/loopback-is-a-credential-2026-10-14.md).
//
// A shell fragment for a curl command run on Node B over ssh. It expands to
// `-H @<root-only header file>` when that file is readable and to nothing
// otherwise, as the CI deploy does (.github/workflows/ci.yml), so the secret
// never appears on a command line. Without it these calls work only while
// hostauth is in observe mode (docs/archive/problems/surface-broken-calls-2026-10-14.md,
// item 9). It contains no quotes, so it is safe inside single- or
// double-quoted JS strings that build the remote command, but it must sit
// outside any single-quoted shell string.
//
// Never add it to a request for a guest port: Node B DNATs loopback ports to
// guests, and the token would leak to a computer.
export const NODE_B_INTERNAL_TOKEN_HEADER_FILE = '/var/lib/go-choir/secrets/internal-service-token.header';
export const NODE_B_INTERNAL_AUTH = `$(test -r ${NODE_B_INTERNAL_TOKEN_HEADER_FILE} && echo -H @${NODE_B_INTERNAL_TOKEN_HEADER_FILE})`;
