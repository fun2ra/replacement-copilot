import type { Case } from "../types";
import { CaseCard } from "./CaseCard";

interface Props {
  cases: Case[];
  selectedId: string;
  onSelect: (id: string) => void;
}

export function CaseList({ cases, selectedId, onSelect }: Props) {
  const selected = cases.find((c) => c.id === selectedId);
  return (
    <>
      {cases.map((c) => (
        <CaseCard key={c.id} item={c} active={c.id === selectedId} onSelect={onSelect} />
      ))}
      {selected?.customer_note && <p className="note">“{selected.customer_note}”</p>}
    </>
  );
}
