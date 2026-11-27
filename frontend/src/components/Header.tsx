import type { Session } from "../hooks/useSession";
import { StatusPill } from "./StatusPill";

interface Props {
  status: string;
  session: Session | null;
}

export function Header({ status, session }: Props) {
  return (
    <header>
      <div>
        <h1>Replacement Copilot</h1>
        <span className="muted">Car Replacement ops · Claude Managed Agents · Go backend</span>
      </div>
      <div className="header-right">
        <StatusPill status={status} />
        {session && (
          <a href={session.consoleUrl} target="_blank" rel="noreferrer" className="muted small">
            open session in Console ↗
          </a>
        )}
      </div>
    </header>
  );
}
