import { useState } from "react";
import { post } from "../lib/api";
import type { Proposal, Trace, Vehicle } from "../types";
import { ApprovalDetails } from "./ApprovalDetails";

interface Props {
  trace: Trace;
  done: boolean;
}

// The open propose_replacement call: a person approves or rejects it here.
export function ApprovalCard({ trace, done }: Props) {
  const { proposal, vehicle } = trace.output as { proposal: Proposal; vehicle: Vehicle };
  const [note, setNote] = useState("");
  const [sent, setSent] = useState(false);

  const decide = async (approved: boolean) => {
    setSent(true);
    await post(`/api/approvals/${trace.approval_id}`, { approved, note });
  };

  return (
    <div className="approval">
      <div className="row">
        <strong>Approval needed · {trace.approval_id}</strong>
        <span className="tag">guardrails passed</span>
      </div>
      <ApprovalDetails proposal={proposal} vehicle={vehicle} />
      {!done && !sent && (
        <>
          <input placeholder="Optional note to the copilot" value={note} onChange={(e) => setNote(e.target.value)} />
          <div className="row gap">
            <button className="primary" onClick={() => decide(true)}>Approve</button>
            <button onClick={() => decide(false)}>Reject</button>
          </div>
        </>
      )}
    </div>
  );
}
