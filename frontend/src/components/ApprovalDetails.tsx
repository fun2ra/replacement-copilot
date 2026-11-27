import type { Proposal, Vehicle } from "../types";
import { RichText } from "./RichText";

interface Props {
  proposal: Proposal;
  vehicle: Vehicle;
}

export function ApprovalDetails({ proposal, vehicle }: Props) {
  return (
    <dl>
      <dt>Vehicle</dt>
      <dd>
        {vehicle.model} ({vehicle.category}, {vehicle.automatic ? "automatic" : "manual"}, {vehicle.seats} seats) ·{" "}
        {vehicle.id} @ {vehicle.branch}
      </dd>
      <dt>Dates</dt>
      <dd>{proposal.start_date}, {proposal.days} days</dd>
      <dt>Co-pay</dt>
      <dd>{proposal.customer_copay ? "customer pays the difference" : "no"}</dd>
      <dt>Why</dt>
      <dd><RichText text={proposal.justification} /></dd>
    </dl>
  );
}
