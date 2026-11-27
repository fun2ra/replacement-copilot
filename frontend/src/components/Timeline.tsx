import type { Trace } from "../types";
import { TimelineItem } from "./TimelineItem";

// Transparency view: one entry per tool call, joined to its result and guardrail verdict.
export function Timeline({ traces }: { traces: Trace[] }) {
  const results = new Map(traces.filter((t) => t.kind === "tool_result").map((t) => [t.tool_use_id, t]));
  const guards = new Map(traces.filter((t) => t.kind === "guardrail").map((t) => [t.tool_use_id, t]));
  const calls = traces.filter((t) => t.kind === "tool_call");

  if (!calls.length) {
    return <p className="muted">Tool calls, retrieved policy text and guardrail verdicts appear here.</p>;
  }

  return (
    <ol className="timeline">
      {calls.map((c, i) => (
        <TimelineItem
          key={c.seq}
          index={i + 1}
          call={c}
          result={results.get(c.tool_use_id)}
          guardrail={guards.get(c.tool_use_id)}
        />
      ))}
    </ol>
  );
}
