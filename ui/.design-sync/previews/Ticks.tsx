import { Ticks } from "ui";

const p = { fontSize: 14.5, lineHeight: 1.6, color: "var(--muted)", maxWidth: 520, margin: 0 } as const;

/** Backticks are mono; the words are written once and drawn here. */
export const ClaimSentence = () => (
  <p style={p}>
    <Ticks text="The `roles` claim lists every role Aanya holds, and `email` comes from Google." />
  </p>
);
