import { useEffect, useRef } from "react";
import type { Trace } from "../types";
import { MessageItem } from "./MessageItem";

// Business view: messages, approval cards and outcomes, in order.
export function Conversation({ traces }: { traces: Trace[] }) {
  const end = useRef<HTMLDivElement>(null);
  const resolved = new Set(traces.filter((t) => t.kind === "approval_resolved").map((t) => t.approval_id));
  // Braces matter: an effect may only return a cleanup function, and
  // scrollIntoView can return a Promise in newer browsers.
  useEffect(() => {
    end.current?.scrollIntoView({ behavior: "smooth" });
  }, [traces.length]);

  return (
    <div className="messages">
      {traces.length === 0 && <p className="muted">Pick a case and run the copilot.</p>}
      {traces.map((t) => (
        <MessageItem key={t.seq} trace={t} approvalDone={resolved.has(t.approval_id)} />
      ))}
      <div ref={end} />
    </div>
  );
}
