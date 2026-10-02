import { Badge } from "ui";

const row = { display: "flex", flexWrap: "wrap", alignItems: "center", gap: 10 } as const;

export const Tones = () => (
  <div style={row}>
    <Badge tone="neutral">Member</Badge>
    <Badge tone="accent">3 open</Badge>
    <Badge tone="warn">in 3 days</Badge>
    <Badge tone="danger">2 failed</Badge>
  </div>
);

/** Soft tones name something dangerous or refused beside prose already saying so. */
export const SoftTones = () => (
  <div style={row}>
    <Badge tone="dangerSoft">Safety role</Badge>
    <Badge tone="warnSoft">Refused</Badge>
  </div>
);

/** The zero form keeps its seat rather than disappearing. */
export const Hollow = () => (
  <div style={row}>
    <Badge hollow>0</Badge>
    <Badge hollow>None expiring</Badge>
  </div>
);
