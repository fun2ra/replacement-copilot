import { Fragment, type ReactNode } from "react";

const TOKEN = /(\*\*[^*]+\*\*|\[[a-z0-9-]+#[a-z0-9-]+\])/g;
const CITATION = /^\[[a-z0-9-]+#[a-z0-9-]+\]$/;
const BULLET = /^[-*•]\s/;

// Minimal, safe markdown: **bold**, "- " bullets, [doc#section] citation chips.
export function RichText({ text }: { text: string }) {
  return (
    <>
      {text.split("\n").map((line, i) => {
        const t = line.trim();
        if (!t) return null;
        if (BULLET.test(t)) return <div key={i} className="bullet">• {inline(t.replace(BULLET, ""))}</div>;
        return <p key={i}>{inline(t.replace(/^#+\s/, ""))}</p>;
      })}
    </>
  );
}

function inline(s: string): ReactNode[] {
  return s.split(TOKEN).map((part, i) => {
    if (part.startsWith("**") && part.endsWith("**")) return <strong key={i}>{part.slice(2, -2)}</strong>;
    if (CITATION.test(part)) return <span key={i} className="cite">{part.slice(1, -1)}</span>;
    return <Fragment key={i}>{part}</Fragment>;
  });
}
