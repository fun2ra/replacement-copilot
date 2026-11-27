import type { Trace } from "../types";

export function GuardrailVerdict({ trace }: { trace: Trace }) {
  const { passed, violations } = trace.output as { passed: boolean; violations: string[] | null };
  return (
    <div className={`guard ${passed ? "ok" : "bad"}`}>
      Guardrails: {passed ? "passed" : "blocked"}
      {violations?.map((v) => <div key={v}>• {v}</div>)}
    </div>
  );
}
