// The tracer's receipt across a kill, kept apart so node --test can prove it
// (scripts/reeng_tracer_receipt_test.mjs). A tracer killed with its parent
// shell left no receipt and its computer running
// (docs/archive/problems/tracer-killed-leaves-computer-running-2026-10-15.md):
//  - the receipt is now written as each phase happens, marked in_progress,
//    so even SIGKILL leaves the minted computer id and the phases on disk;
//  - SIGINT, SIGTERM and SIGHUP record the interruption, stop the minted
//    computer and exit 128 + the signal number;
//  - at start, the tracer reports computers that earlier killed or
//    interrupted runs left running. It only reports: a tracer acts only on
//    the computer it minted.

const SIGNALS = { SIGHUP: 1, SIGINT: 2, SIGTERM: 15 };

export function signalExitCode(signal) {
  return SIGNALS[signal] ? 128 + SIGNALS[signal] : 1;
}

// lastMinted is the newest attempt that minted a computer.
function lastMinted(receipt) {
  const attempts = receipt?.attempts || [];
  for (let i = attempts.length - 1; i >= 0; i--) if (attempts[i]?.computer_id) return attempts[i];
  return null;
}

// inProgressReceipt is what goes on disk while the run is live. exit_code
// stays null, so a partial receipt never reads as a pass.
export function inProgressReceipt(receipt, at) {
  return { ...receipt, in_progress: true, updated_at: at, computer_id: lastMinted(receipt)?.computer_id || null, exit_code: null };
}

// interruptedReceipt closes the receipt on a caught signal. stop is what
// stopping the minted computer answered (null when nothing was minted or
// the computer is kept).
export function interruptedReceipt(receipt, signal, at, stop) {
  const attempts = (receipt?.attempts || []).map((a) => ({ ...a }));
  const minted = lastMinted({ attempts });
  if (minted) minted.computer_stop = stop ?? null;
  return {
    ...receipt,
    attempts,
    in_progress: false,
    finished_at: at,
    computer_id: minted?.computer_id || null,
    interrupted: { signal, at, computer_stop: minted ? (stop ?? null) : null },
    exit_code: signalExitCode(signal),
  };
}

// stopped: a 2xx stop, or, for a two-computer scenario's { a, b }, both.
function stopped(stop) {
  if (stop && (stop.a !== undefined || stop.b !== undefined)) return stopped(stop.a) && stopped(stop.b);
  return Boolean(stop && stop.status >= 200 && stop.status < 300);
}

// strandedTracerComputers names computers earlier runs may have left
// running: a run killed mid-flight (its receipt still in progress) or an
// interrupted run whose stop did not succeed. Kept computers are not
// stranded. entries are { label, receipt } read from the traces directory.
export function strandedTracerComputers(entries) {
  const out = [];
  for (const { label, receipt } of entries || []) {
    if (!receipt || receipt.keep_computer) continue;
    const minted = lastMinted(receipt);
    if (!minted) continue;
    if (receipt.in_progress) out.push({ label, computer_id: minted.computer_id, reason: 'killed', since: receipt.updated_at || receipt.started_at || null });
    else if (receipt.interrupted && !stopped(minted.computer_stop)) out.push({ label, computer_id: minted.computer_id, reason: 'stop_failed', since: receipt.interrupted.at || null });
  }
  return out;
}
