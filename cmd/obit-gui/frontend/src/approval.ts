import type { BatchReply, ProposalResult } from "./types";

// A transport or settlement error can accompany valid per-candidate results.
export function approvalReceipt(reply: BatchReply<ProposalResult>, noReply: string) {
  const result = reply.results?.[0]; // the GUI submits one reviewed proposal at a time
  const remaining = new Set(result?.remaining);
  const rows = (result?.results ?? []).map(({ ref, error }) => ({ ref, error, pending: remaining.has(ref) }));
  const error = result?.error || reply.error || (!result ? noReply : undefined);
  return { rows, error, code: result?.errorCode ?? reply.errorCode, partial: !!error || rows.some(r => r.error || r.pending) };
}
