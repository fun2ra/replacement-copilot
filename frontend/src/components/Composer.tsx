import { useState } from "react";
import { post } from "../lib/api";

export function Composer({ sessionId }: { sessionId: string }) {
  const [text, setText] = useState("");

  const send = async () => {
    if (!text.trim()) return;
    await post(`/api/sessions/${sessionId}/messages`, { text });
    setText("");
  };

  return (
    <form className="composer" onSubmit={(e) => { e.preventDefault(); send(); }}>
      <input
        placeholder="Follow up, e.g. “The customer agreed to pay for the SUV”"
        value={text}
        onChange={(e) => setText(e.target.value)}
      />
      <button className="primary" type="submit">Send</button>
    </form>
  );
}
