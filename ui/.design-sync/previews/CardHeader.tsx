import { Card, CardHeader, CardHeaderLink, CardRow } from "ui";

const body = { fontSize: 14.5, color: "var(--muted)" } as const;

/** Count tone is the axis: accent (work to do), warn (a deadline), danger (decide now). */
export const Accent = () => (
  <Card>
    <CardHeader title="Open requests" count={4} action={<CardHeaderLink href="/requests">See all →</CardHeaderLink>} />
    <CardRow first><span style={body}>Aanya Rao asked for Laser cutter · Operator</span></CardRow>
  </Card>
);

export const Warn = () => (
  <Card>
    <CardHeader title="Expiring soon" count={2} tone="warn" note="Next 14 days" />
    <CardRow first><span style={body}>Rohan Iyer loses CNC router access in 3 days</span></CardRow>
  </Card>
);

export const Danger = () => (
  <Card>
    <CardHeader title="Access Zitadel shows but Syndra never granted" count={3} tone="danger" note="Checked 2 min ago" />
    <CardRow first><span style={body}>Wood shop · Supervisor held by vivek.nair@example.edu</span></CardRow>
  </Card>
);

/** A zero count goes hollow: a solid 0 would be an alarm about nothing. */
export const ZeroCount = () => (
  <Card>
    <CardHeader title="Not going to happen" count={0} note="Declined or withdrawn" />
  </Card>
);

export const TitleOnly = () => (
  <Card>
    <CardHeader title="Roles in this project" />
  </Card>
);
