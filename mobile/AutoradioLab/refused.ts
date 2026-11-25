// Exact's refusals of work whose answer is gone: a fetch or native call made by an answer that was let go ("the answer was let go before this fetch", js/src/fetch.js), one in flight when its answer
// was let go ("the answer was let go before this reply; the request may already have been sent", js/src/prelude.js, and the AbortError "the answer was let go"), and a fetch, a native call or storage
// begun outside an answer, from background work or after the answer's storage wait failed ("... it was never run", js/src/fetch.js, js/src/prelude.js, js/src/storage.js). Such a call says nothing
// about the server, the disk or the deck: it is never counted as their failure (docs/reference/exact/answers-let-go.md). refused.test.ts holds the strings against the pinned source.
export function refusedHere(e: unknown): boolean {
  const m = String((e as { message?: unknown } | null)?.message ?? e ?? '');
  return /the answer was let go|it was never run|the answer's storage wait failed/.test(m);
}
