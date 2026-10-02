import { MetaDot, MetaRow } from "ui";

const line = { display: "flex", alignItems: "center", gap: 12, fontSize: 14.5, color: "var(--muted)" } as const;

/** The separator MetaRow puts between facts. */
export const InMetaRow = () => <MetaRow>{["Laser cutter", "Operator", "Expires 14 Nov"]}</MetaRow>;

export const Standalone = () => (
  <span style={line}>
    <span>3 bundles</span>
    <MetaDot />
    <span>21 people</span>
  </span>
);
