import type { Trace } from "../types";

export function ApprovalOutcome({ trace }: { trace: Trace }) {
  const { approved, note } = trace.output as { approved: boolean; note: string };
  return (
    <div className={`system ${approved ? "ok" : "bad"}`}>
      {approved ? "✔ Approved" : "✖ Rejected"} by operations{note ? `: “${note}”` : ""}
    </div>
  );
}
