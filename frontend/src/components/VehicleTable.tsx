import type { Vehicle } from "../types";

export function VehicleTable({ vehicles }: { vehicles: Vehicle[] }) {
  return (
    <table>
      <tbody>
        {vehicles.map((v) => (
          <tr key={v.id}>
            <td>{v.id}</td>
            <td>{v.model}</td>
            <td>{v.category}</td>
            <td>{v.automatic ? "auto" : "manual"}</td>
          </tr>
        ))}
      </tbody>
    </table>
  );
}
