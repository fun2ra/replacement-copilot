import type { Trace } from "../types";
import { ApprovalCard } from "./ApprovalCard";
import { ApprovalOutcome } from "./ApprovalOutcome";
import { RichText } from "./RichText";

interface Props {
  trace: Trace;
  approvalDone: boolean;
}

// Picks the conversation element for one trace; other kinds only show in the timeline.
export function MessageItem({ trace, approvalDone }: Props) {
  switch (trace.kind) {
    case "user":
      return <div className="msg user">{trace.text}</div>;
    case "agent_message":
      return (
        <div className="msg agent">
          <RichText text={trace.text ?? ""} />
        </div>
      );
    case "approval_required":
      return <ApprovalCard trace={trace} done={approvalDone} />;
    case "approval_resolved":
      return <ApprovalOutcome trace={trace} />;
    case "domain_event":
      return <div className="system">⚡ event published: {trace.text}</div>;
    case "error":
      return <div className="error">{trace.text}</div>;
    default:
      return null;
  }
}
