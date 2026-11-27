import { useEffect, useState } from "react";
import type { Trace } from "../types";

// Live trace stream for one session over SSE.
export function useTraces(sessionId: string | null): Trace[] {
  const [traces, setTraces] = useState<Trace[]>([]);
  useEffect(() => {
    setTraces([]);
    if (!sessionId) return;
    const es = new EventSource(`/api/sessions/${sessionId}/events`);
    es.onmessage = (e) => {
      const t = JSON.parse(e.data) as Trace;
      // the server replays the backlog on reconnect: dedupe by seq
      setTraces((prev) => (prev.some((p) => p.seq === t.seq) ? prev : [...prev, t]));
    };
    return () => es.close();
  }, [sessionId]);
  return traces;
}
