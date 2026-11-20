// The conversation page's rows, in shared code (it was Swift: Conversation.swift `ConversationInfo.make`). Every question the reader asked and what came back, newest first.
// What stays native, and why: the entries themselves. The voice turn engine (TurnEngine.swift) writes them as a turn is heard, answered and spoken (`Conversation.shared`, which the
// bar's answer card also reads), so the native source `conversationEntries` hands them over whole; nothing is decided there. Everything the page shows is decided here.
// status: asking | answering | answered | failed | cancelled.

export type Entry = { id: string; question: string; first: string; firstSource: string; answer: string; status: string; note: string };
export type Row = Entry & { canRetry: boolean };
export type Info = { rows: Row[]; empty: boolean };

const str = (v: unknown): string => (typeof v === 'string' ? v : '');
const obj = (v: unknown): v is Record<string, unknown> => typeof v === 'object' && v !== null && !Array.isArray(v);

/** The page's rows from the native entries (oldest first): newest first, cancelled ones dropped (a thank you, a cancel, a question asked again are not answers or failures).
 * A row can be asked again when it failed, or was answered but carries a note ("The answer was cut off", "The full answer did not arrive"). */
export function info(raw: unknown): Info {
  const list = obj(raw) && Array.isArray(raw.entries) ? raw.entries : [];
  const rows: Row[] = [];
  for (let i = list.length - 1; i >= 0; i--) {
    const e = list[i];
    if (!obj(e)) continue;
    const status = str(e.status), note = str(e.note);
    if (status === 'cancelled') continue;
    rows.push({ id: str(e.id), question: str(e.question), first: str(e.first), firstSource: str(e.firstSource), answer: str(e.answer), status, note,
      canRetry: status === 'failed' || (status === 'answered' && note !== '') });
  }
  return { rows, empty: rows.length === 0 };
}
