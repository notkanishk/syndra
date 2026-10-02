import { CountChip } from "ui";

const row = { display: "flex", alignItems: "center", gap: 16, fontSize: 15, fontWeight: 600 } as const;

export const Filled = () => <div style={row}>Open requests <CountChip n={4} /></div>;

export const Zero = () => <div style={row}>Expiring soon <CountChip n={0} /></div>;

/** A failed read is an em dash, never a 0. */
export const ReadFailed = () => <div style={row}>Drift findings <CountChip n={null} /></div>;
