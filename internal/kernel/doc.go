// Package kernel is the Go kernel of the modular architecture: a scope tree
// whose close runs awaited LIFO effects, named service provision and
// injection along that tree, a supervisor with restart intensity, a journal
// folded at boot, and a snapshot of all of it.
//
// The kernel is deliberately blind. It imports only the Go standard packages and
// itself, and names no desk and no app (structural gates R1, enforced by
// scripts/check-kernel-imports, check-kernel-names and
// check-kernel-responsibilities; the exported surface is pinned in
// RESPONSIBILITIES).
//
// # Ways it could fail (written before the code, per the testing rule)
//
//  1. A scope closes while an effect is still registering: a late OnClose or
//     Provide must be refused, never silently dropped or run out of order.
//  2. Effects run in registration order, not LIFO, so a dependency is torn
//     down before its dependent. Close must run children first (reverse
//     creation order), then effects in reverse registration order.
//  3. Close returns before an effect finished ("awaited" violated), leaking
//     work past the owner's teardown. Close waits for each effect, bounded
//     only by the caller's context, and reports an abandoned effect by name.
//  4. One effect panics or errors and the rest never run. Every effect runs;
//     errors are joined.
//  5. A second concurrent Close double-runs effects. Exactly one caller
//     runs them; others wait and receive the same result.
//  6. Inject finds a service from a closed scope, or from a sibling. Lookup
//     walks only the parents, and a closed scope serves nothing.
//  7. A crashing unit restarts forever (a crash loop). Two crashes inside
//     the window close the unit with the cause, and only an explicit Reopen
//     (an owner act) starts it again.
//  8. A crash resumes work: a restarted unit receives nothing from the
//     occurrence that crashed. Its Run gets a fresh scope; the supervisor
//     never replays an obligation.
//  9. Boot replays occurrences: the boot fold rebuilds unit state from the
//     journal without calling any Run. A unit journaled as started with no
//     stop was cut off by a restart and is closed with "interrupted by a
//     restart", never re-dispatched.
//  10. The snapshot races with mutation or hides a stuck scope. It is taken
//     under the same locks as mutation and lists open scopes, pending effect
//     names, services and unit states.
//  11. The kernel grows desk or app knowledge. The three gates refuse it.
package kernel
