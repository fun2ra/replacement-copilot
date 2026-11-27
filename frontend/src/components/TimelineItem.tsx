import { formatArgs } from "../lib/format";
import type { Trace } from "../types";
import { GuardrailVerdict } from "./GuardrailVerdict";
import { ToolOutput } from "./ToolOutput";

interface Props {
  index: number;
  call: Trace;
  result?: Trace;
  guardrail?: Trace;
}

export function TimelineItem({ index, call, result, guardrail }: Props) {
  const state = result?.is_error ? "err" : result ? "ok" : "pending";
  return (
    <li className={state}>
      <div className="row">
        <strong>{index}. {call.tool}</strong>
        <span className="muted small">{result ? `${result.duration_ms} ms` : "running…"}</span>
      </div>
      <code className="args">{formatArgs(call.input)}</code>
      {guardrail && <GuardrailVerdict trace={guardrail} />}
      {result && <ToolOutput tool={call.tool ?? ""} output={result.output} />}
    </li>
  );
}
