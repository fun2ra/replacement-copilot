import { useEffect, useState } from "react";

// GET a JSON resource; refetches whenever a value in deps changes.
export function useFetch<T>(url: string, deps: unknown[]): T | null {
  const [data, setData] = useState<T | null>(null);
  useEffect(() => {
    fetch(url).then((r) => r.json()).then(setData).catch(() => setData(null));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, deps);
  return data;
}
