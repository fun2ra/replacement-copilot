import { useMemo, useState } from "react";
import { post } from "../lib/api";
import type { Trace } from "../types";
import { useTraces } from "./useTraces";

export interface Session {
  id: string;
  consoleUrl: string;
}

// Starts an agent session for a case and exposes its live traces and status.
export function useSession() {
  const [session, setSession] = useState<Session | null>(null);
  const [starting, setStarting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const traces = useTraces(session?.id ?? null);
  const status = useMemo(() => deriveStatus(traces, session !== null), [traces, session]);

  async function start(caseId: string) {
    setStarting(true);
    setError(null);
    try {
      const r = await post("/api/sessions", { case_id: caseId });
      if (!r.ok) throw new Error(await r.text());
      const j = await r.json();
      setSession({ id: j.session_id, consoleUrl: j.console_url });
    } catch (e) {
      setError(String(e));
    } finally {
      setStarting(false);
    }
  }

  return { session, traces, status, starting, error, start };
}

function deriveStatus(traces: Trace[], hasSession: boolean): string {
  for (let i = traces.length - 1; i >= 0; i--) {
    const t = traces[i];
    if (t.kind === "turn_end") return "idle";
    if (t.kind === "approval_required") return "awaiting approval";
    if (t.kind === "status") return t.text ?? "";
  }
  return hasSession ? "starting" : "no session";
}
