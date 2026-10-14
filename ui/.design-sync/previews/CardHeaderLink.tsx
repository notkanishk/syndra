import { Card, CardHeader, CardHeaderLink, CardRow } from "ui";

const body = { fontSize: 14.5, color: "var(--muted)" } as const;

export const InCardHeader = () => (
  <Card>
    <CardHeader title="Recent activity" action={<CardHeaderLink href="/audit">Full audit log →</CardHeaderLink>} />
    <CardRow first><span style={body}>Priya Nair granted Laser cutter · Operator to Aanya Rao</span></CardRow>
  </Card>
);

export const WithCount = () => (
  <Card>
    <CardHeader title="Open requests" count={7} action={<CardHeaderLink href="/requests">See all →</CardHeaderLink>} />
  </Card>
);
