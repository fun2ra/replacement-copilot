export function RawOutput({ output }: { output: unknown }) {
  return (
    <details>
      <summary className="muted small">output</summary>
      <pre>{JSON.stringify(output, null, 2)}</pre>
    </details>
  );
}
