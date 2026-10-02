import { StatusDot } from "ui";

const col = { display: "flex", flexDirection: "column", gap: 10 } as const;
const item = { display: "flex", alignItems: "center", gap: 8, fontSize: 13.5 } as const;

/** A dot and a word — never colour alone. */
export const Tones = () => (
  <div style={col}>
    <span style={item}><StatusDot tone="healthy" /><span style={{ color: "var(--ink)" }}>Reachable</span></span>
    <span style={item}><StatusDot tone="neutral" /><span style={{ color: "var(--muted)" }}>Not answered yet</span></span>
    <span style={item}><StatusDot tone="accent" /><span style={{ color: "var(--accent-text)" }}>Syncing</span></span>
    <span style={item}><StatusDot tone="warn" /><span style={{ color: "var(--warn-text)" }}>Version behind</span></span>
    <span style={item}><StatusDot tone="danger" /><span style={{ color: "var(--danger-text)" }}>Unreachable</span></span>
  </div>
);
