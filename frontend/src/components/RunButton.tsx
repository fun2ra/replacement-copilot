interface Props {
  caseId: string;
  starting: boolean;
  error: string | null;
  onRun: () => void;
}

export function RunButton({ caseId, starting, error, onRun }: Props) {
  return (
    <>
      <button className="primary" disabled={starting} onClick={onRun}>
        {starting ? "Starting session…" : `Run copilot on ${caseId}`}
      </button>
      {error && <p className="error">{error}</p>}
    </>
  );
}
