import { useEffect } from "react";
import { AppShell, Badge, Button, Card, CardHeader, CardRow, PageHeader, SyndraRoot, useUiView } from "ui";

const operator = {
  id: "284019337710552064",
  name: "Priya Nair",
  email: "priya.nair@example.edu",
  title: "Makerspace Manager",
  team: "Makerspace",
  status: "active",
  avatar: "PN",
  role: "admin" as const,
  sessionType: "oidc" as const,
};

const member = {
  id: "284019337710552099",
  name: "Kabir Mehta",
  email: "kabir.mehta@example.edu",
  title: "Student",
  team: "Design Club",
  status: "active",
  avatar: "KM",
  role: "user" as const,
  sessionType: "oidc" as const,
};

/** Full-bleed: AppShell is h-dvh, so cancel the 24px card padding and let it fill the viewport. */
const frame = { margin: -24, height: "100dvh", overflow: "hidden" } as const;
const name = { width: 170, fontSize: 15, fontWeight: 600 } as const;
const muted = { flex: 1, fontSize: 14.5, color: "var(--muted)" } as const;

/** Pins the operator's view; AppShell's own UiViewProvider persists it, so every cell sets its own. */
function View({ view }: { view: "basic" | "advanced" }) {
  const { setView } = useUiView();
  useEffect(() => setView(view), [setView, view]);
  return null;
}

export const OperatorBasic = () => (
  <SyndraRoot pathname="/users">
    <div style={frame}>
      <AppShell session={operator}>
        <View view="basic" />
        <PageHeader
          title="People"
          lede="Everyone who has signed in with a university Google account, and the access they hold."
          actions={<Button variant="accent">Invite someone</Button>}
        />
        <Card>
          <CardHeader title="Recently active" count={3} />
          <CardRow first>
            <div style={name}>Aanya Rao</div>
            <div style={muted}>Laser cutter · Operator</div>
            <Badge tone="neutral">Active</Badge>
          </CardRow>
          <CardRow>
            <div style={name}>Rohan Iyer</div>
            <div style={muted}>CNC router · Operator</div>
            <Badge tone="warn">Expires in 3 days</Badge>
          </CardRow>
          <CardRow>
            <div style={name}>Ishita Sen</div>
            <div style={muted}>Wood shop · Supervisor</div>
            <Badge tone="neutral">Active</Badge>
          </CardRow>
        </Card>
      </AppShell>
    </div>
  </SyndraRoot>
);

export const OperatorAdvanced = () => (
  <SyndraRoot pathname="/governance/drift">
    <div style={frame}>
      <AppShell session={operator}>
        <View view="advanced" />
        <PageHeader
          title="Drift"
          lede="Changes in Zitadel that Syndra did not make. Each one needs a decision: keep it, or put it back."
        />
        <Card>
          <CardHeader title="Unexplained changes" count={2} tone="danger" />
          <CardRow first>
            <div style={name}>Meera Pillai</div>
            <div style={muted}>Electronics bench · Operator added in Zitadel</div>
            <Button size="sm" variant="outline">Review</Button>
          </CardRow>
          <CardRow>
            <div style={name}>Arjun Das</div>
            <div style={muted}>3D printers · Member removed in Zitadel</div>
            <Button size="sm" variant="outline">Review</Button>
          </CardRow>
        </Card>
      </AppShell>
    </div>
  </SyndraRoot>
);

/** A member gets the shell without the view switch, and their own three destinations. */
export const Member = () => (
  <SyndraRoot pathname="/">
    <div style={frame}>
      <AppShell session={member}>
        <PageHeader title="My access" lede="The machines and spaces you can use, and when each one ends." />
        <Card>
          <CardHeader title="Current access" count={2} />
          <CardRow first>
            <div style={name}>3D printers</div>
            <div style={muted}>Member · no end date</div>
          </CardRow>
          <CardRow>
            <div style={name}>Laser cutter</div>
            <div style={muted}>Operator · ends 14 Dec</div>
            <Badge tone="warn">in 9 days</Badge>
          </CardRow>
        </Card>
      </AppShell>
    </div>
  </SyndraRoot>
);
