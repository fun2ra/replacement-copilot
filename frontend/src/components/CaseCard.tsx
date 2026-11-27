import type { Case } from "../types";

interface Props {
  item: Case;
  active: boolean;
  onSelect: (id: string) => void;
}

export function CaseCard({ item, active, onSelect }: Props) {
  return (
    <button className={`case ${active ? "active" : ""}`} onClick={() => onSelect(item.id)}>
      <div className="row">
        <strong>{item.id}</strong>
        <span className="tag">{item.partner}</span>
      </div>
      <div>{item.customer} · {item.city}</div>
      <div className="muted small">
        {item.own_car} ({item.own_category}){item.needs_automatic ? " · automatic" : ""} ·{" "}
        {item.estimated_repair_days} days from {item.start_date}
      </div>
    </button>
  );
}
