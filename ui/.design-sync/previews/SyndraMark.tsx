import { SyndraMark } from "ui";

const row = { display: "flex", alignItems: "center", gap: 24 } as const;

export const Sizes = () => (
  <div style={row}>
    <SyndraMark size={16} />
    <SyndraMark />
    <SyndraMark size={32} />
    <SyndraMark size={64} />
  </div>
);

/** The rail's lockup: mark at its default 22px beside the display-face wordmark. */
export const RailLockup = () => (
  <div style={{ ...row, gap: 10, background: "var(--rail)", padding: "20px 20px", width: 252, borderRight: "1px solid var(--line)" }}>
    <SyndraMark />
    <span style={{ fontFamily: "var(--font-display)", fontSize: 18, fontWeight: 600, letterSpacing: "-0.01em" }}>Syndra</span>
  </div>
);
