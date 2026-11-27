import type { PolicyHit, Vehicle } from "../types";
import { PolicyHits } from "./PolicyHits";
import { RawOutput } from "./RawOutput";
import { VehicleTable } from "./VehicleTable";

interface Props {
  tool: string;
  output: unknown;
}

// Renders known tool results in a readable form, anything else as JSON.
export function ToolOutput({ tool, output }: Props) {
  const o = output as Record<string, unknown> | null;
  if (tool === "search_policies" && Array.isArray(o?.results)) {
    return <PolicyHits hits={o.results as PolicyHit[]} />;
  }
  if (tool === "search_vehicles" && Array.isArray(o?.vehicles)) {
    return <VehicleTable vehicles={o.vehicles as Vehicle[]} />;
  }
  return <RawOutput output={output} />;
}
