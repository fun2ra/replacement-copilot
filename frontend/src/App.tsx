import { useState } from "react";
import { Composer } from "./components/Composer";
import { Conversation } from "./components/Conversation";
import { CaseList } from "./components/CaseList";
import { Header } from "./components/Header";
import { Panel } from "./components/Panel";
import { ReservationList } from "./components/ReservationList";
import { RunButton } from "./components/RunButton";
import { Timeline } from "./components/Timeline";
import { useFetch } from "./hooks/useFetch";
import { useSession } from "./hooks/useSession";
import type { Case, Reservation } from "./types";

// Composition only: state lives in hooks, rendering in components/.
export default function App() {
  const cases = useFetch<Case[]>("/api/cases", []);
  const [caseId, setCaseId] = useState("RC-1001");
  const { session, traces, status, starting, error, start } = useSession();
  const bookings = traces.filter((t) => t.kind === "domain_event").length;
  const reservations = useFetch<Reservation[]>("/api/reservations", [bookings]);

  return (
    <div className="app">
      <Header status={status} session={session} />
      <main>
        <Panel title="Cases" as="aside">
          <CaseList cases={cases ?? []} selectedId={caseId} onSelect={setCaseId} />
          <RunButton caseId={caseId} starting={starting} error={error} onRun={() => start(caseId)} />
          <h2>Confirmed reservations</h2>
          <ReservationList reservations={reservations ?? []} />
        </Panel>

        <Panel title="Conversation" className="chat">
          <Conversation traces={traces} />
          {session && <Composer sessionId={session.id} />}
        </Panel>

        <Panel title="Agent trace">
          <Timeline traces={traces} />
        </Panel>
      </main>
    </div>
  );
}
