import { Relative } from "ui";

const ago = (ms: number) => new Date(Date.now() - ms).toISOString();
const col = { display: "flex", flexDirection: "column", gap: 8, fontSize: 14.5 } as const;
const item = { display: "flex", gap: 12 } as const;
const label = { width: 220, color: "var(--muted)" } as const;

export const Ages = () => (
  <div style={col}>
    <span style={item}><span style={label}>Aanya Rao signed in</span><Relative iso={ago(20_000)} /></span>
    <span style={item}><span style={label}>Laser cutter role granted</span><Relative iso={ago(12 * 60_000)} /></span>
    <span style={item}><span style={label}>Wood shop bundle edited</span><Relative iso={ago(3 * 3_600_000)} /></span>
    <span style={item}><span style={label}>CNC router hold reviewed</span><Relative iso={ago(6 * 86_400_000)} /></span>
    <span style={item}><span style={label}>Kabir Mehta joined</span><Relative iso={ago(45 * 86_400_000)} /></span>
  </div>
);

/** No stamp reads as an em dash. */
export const Missing = () => (
  <div style={item}><span style={label}>Last sync to TrueNAS</span><Relative iso={null} /></div>
);
