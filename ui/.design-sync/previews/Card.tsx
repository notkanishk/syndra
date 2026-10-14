import { Card, CardHeader, CardHeaderLink, CardRow, Badge, Button } from "ui";

const name = { width: 170, fontSize: 15, fontWeight: 600 } as const;
const muted = { flex: 1, fontSize: 14.5, color: "var(--muted)" } as const;

export const OpenRequests = () => (
  <Card>
    <CardHeader
      title="Open requests"
      count={3}
      action={<CardHeaderLink href="/requests">See all →</CardHeaderLink>}
    />
    <CardRow first>
      <div style={name}>Aanya Rao</div>
      <div style={muted}>Laser cutter · Operator</div>
      <Button size="sm" variant="accent">Approve</Button>
    </CardRow>
    <CardRow>
      <div style={name}>Kabir Mehta</div>
      <div style={muted}>3D printers · Member</div>
      <Button size="sm" variant="accent">Approve</Button>
    </CardRow>
    <CardRow>
      <div style={name}>Ishita Sen</div>
      <div style={muted}>Wood shop · Supervisor</div>
      <Button size="sm" variant="accent">Approve</Button>
    </CardRow>
  </Card>
);

export const ExpiringWithNote = () => (
  <Card>
    <CardHeader title="Expiring soon" count={2} tone="warn" note="Next 14 days" />
    <CardRow first>
      <div style={name}>Rohan Iyer</div>
      <div style={muted}>CNC router · Operator</div>
      <Badge tone="warn">in 3 days</Badge>
    </CardRow>
    <CardRow>
      <div style={name}>Meera Pillai</div>
      <div style={muted}>Electronics bench · Member</div>
      <Badge tone="warn">in 9 days</Badge>
    </CardRow>
  </Card>
);
