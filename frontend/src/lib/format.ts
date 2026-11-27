// Tool input as one compact line; the justification is shown elsewhere.
export function formatArgs(input?: Record<string, unknown>) {
  if (!input) return "";
  return Object.entries(input)
    .filter(([k]) => k !== "justification")
    .map(([k, v]) => `${k}=${JSON.stringify(v)}`)
    .join("  ");
}
