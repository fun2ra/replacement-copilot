import type { PolicyHit } from "../types";

export function PolicyHits({ hits }: { hits: PolicyHit[] }) {
  return (
    <div className="hits">
      {hits.map((h) => (
        <details key={h.id}>
          <summary>
            <span className="cite">{h.id}</span> <span className="muted small">score {h.score}</span>
          </summary>
          <p className="small">{h.text}</p>
        </details>
      ))}
    </div>
  );
}
