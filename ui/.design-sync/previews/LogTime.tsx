import { LogTime, Mono } from "ui";

const row = { display: "flex", gap: 16, fontSize: 13.5, alignItems: "baseline" } as const;
const t = { width: 64, color: "var(--faint)" } as const;

/** The log ruler: 24-hour, in a mono column. */
export const LogRows = () => (
  <div style={{ display: "flex", flexDirection: "column", gap: 10 }}>
    <div style={row}><Mono style={t}><LogTime iso="2026-10-02T04:12:07Z" /></Mono><span style={{ color: "var(--muted)" }}>Aanya Rao was granted Laser cutter · Operator</span></div>
    <div style={row}><Mono style={t}><LogTime iso="2026-10-02T04:15:42Z" /></Mono><span style={{ color: "var(--muted)" }}>Kabir Mehta signed in for the first time</span></div>
    <div style={row}><Mono style={t}><LogTime iso="2026-10-02T04:21:03Z" /></Mono><span style={{ color: "var(--danger-text)" }}>Sync to TrueNAS was refused</span></div>
  </div>
);

/** Before mount, or with no stamp, the seat holds a placeholder. */
export const Missing = () => (
  <div style={row}><Mono style={t}><LogTime iso={null} /></Mono><span style={{ color: "var(--muted)" }}>Event with no recorded time</span></div>
);
