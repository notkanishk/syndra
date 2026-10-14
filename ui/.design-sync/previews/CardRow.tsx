import { useState } from "react";
import { Card, CardHeader, CardRow, RowField, Badge, Button } from "ui";

const name = { width: 170, fontSize: 15, fontWeight: 600 } as const;
const muted = { flex: 1, fontSize: 14.5, color: "var(--muted)" } as const;

export const PlainRows = () => (
  <Card>
    <CardHeader title="Holders" count={3} />
    <CardRow first>
      <div style={name}>Aanya Rao</div>
      <div style={muted}>Laser cutter · Operator</div>
      <Badge tone="accent">Bundle</Badge>
    </CardRow>
    <CardRow>
      <div style={name}>Kabir Mehta</div>
      <div style={muted}>3D printers · Member</div>
      <Badge>Direct</Badge>
    </CardRow>
    <CardRow>
      <div style={name}>Ishita Sen</div>
      <div style={muted}>Wood shop · Supervisor</div>
      <Badge tone="warn">in 6 days</Badge>
    </CardRow>
  </Card>
);

/** Whole row is the toggle; the panel opens under the row that was tapped. */
export const DisclosureOpen = () => {
  const [open, setOpen] = useState(true);
  return (
    <Card>
      <CardRow
        first
        expanded={open}
        onToggle={() => setOpen((v) => !v)}
        disclosure={
          <>
            <RowField label="Granted by">Priya Nair</RowField>
            <RowField label="Expires">14 Nov 2026</RowField>
          </>
        }
      >
        <div style={name}>Rohan Iyer</div>
        <div style={muted}>CNC router · Operator</div>
      </CardRow>
      <CardRow expanded={false} onToggle={() => {}} disclosure={<RowField label="Granted by">Priya Nair</RowField>}>
        <div style={name}>Meera Pillai</div>
        <div style={muted}>Electronics bench · Member</div>
      </CardRow>
    </Card>
  );
};

/** A row that already holds a button: its own control opens the panel, the row stays inert. */
export const DisclosureWithControl = () => (
  <Card>
    <CardRow
      first
      expanded
      disclosure={<RowField label="Reason given">Needs the CNC router for a capstone enclosure.</RowField>}
    >
      <div style={name}>Arjun Das</div>
      <div style={muted}>CNC router · Member</div>
      <Button size="sm" variant="outline">Hide details</Button>
    </CardRow>
  </Card>
);
