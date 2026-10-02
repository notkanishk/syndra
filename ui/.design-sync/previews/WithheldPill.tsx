import { WithheldPill } from "ui";

const line = { display: "flex", alignItems: "center", gap: 10, fontSize: 14.5 } as const;

export const Alone = () => <WithheldPill />;

/** Inline in a row of a longer list — a door, a share. */
export const InRow = () => (
  <div style={{ display: "grid", gap: 10 }}>
    <div style={line}><span>Main makerspace door</span></div>
    <div style={line}><span>Wood shop door</span><WithheldPill /></div>
    <div style={line}><span>Electronics store room</span></div>
  </div>
);
