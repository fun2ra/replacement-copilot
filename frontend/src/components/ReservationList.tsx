import type { Reservation } from "../types";

export function ReservationList({ reservations }: { reservations: Reservation[] }) {
  if (!reservations.length) return <p className="muted small">None yet.</p>;
  return (
    <>
      {reservations.map((r) => (
        <div key={r.id} className="reservation">
          <strong>{r.id}</strong> · {r.case_id} → {r.vehicle_id}
          <div className="muted small">
            {r.start_date}, {r.days} days{r.customer_copay ? " · customer co-pay" : ""} · by {r.approved_by}
          </div>
        </div>
      ))}
    </>
  );
}
