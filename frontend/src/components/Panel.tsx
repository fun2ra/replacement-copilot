import type { ReactNode } from "react";

interface Props {
  title: string;
  as?: "aside" | "section";
  className?: string;
  children: ReactNode;
}

export function Panel({ title, as: Tag = "section", className = "", children }: Props) {
  return (
    <Tag className={`panel ${className}`.trim()}>
      <h2>{title}</h2>
      {children}
    </Tag>
  );
}
