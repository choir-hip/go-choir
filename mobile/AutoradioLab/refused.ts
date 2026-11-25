// Exact's refusals of work whose answer is gone: a fetch or native call made by an answer that was let go ("the answer was let go before this fetch", js/src/fetch.js), and a fetch, a native call
// or storage begun outside an answer, from background work or after the answer's storage wait failed ("... it was never run", js/src/fetch.js, js/src/storage.js:127-137). Such a call was never made:
// it says nothing about the server, the disk or the deck (docs/reference/exact/answers-let-go.md).
export function refusedHere(e: unknown): boolean {
  const m = String((e as { message?: unknown } | null)?.message ?? e ?? '');
  return /the answer was let go before this fetch|it was never run|the answer's storage wait failed/.test(m);
}
