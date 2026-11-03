import { Card, CardRow, NotOpenYet, StatCard } from "ui";

const name = { width: 170, fontSize: 15, fontWeight: 600 } as const;
const muted = { flex: 1, fontSize: 14.5, color: "var(--muted)" } as const;

/** In a card: a 15px lead, the way on, and the contents drawn as they will be. */
export const InCard = () => (
  <Card>
    <div style={{ padding: 20 }}>
      <NotOpenYet
        open={false}
        title="Storage"
        why="Opens once your TrueNAS password is set."
        step={{ label: "Set it above", href: "#password" }}
      >
        <CardRow first>
          <div style={name}>projects</div>
          <div style={muted}>Read and write</div>
        </CardRow>
        <CardRow>
          <div style={name}>scratch</div>
          <div style={muted}>Read and write</div>
        </CardRow>
      </NotOpenYet>
    </div>
  </Card>
);

/** No way on from here right now: the lead line only. */
export const NoStep = () => (
  <NotOpenYet open={false} title="Laser Cutter jobs" why="Opens after your safety induction is recorded.">
    <div style={{ fontSize: 14.5, color: "var(--muted)" }}>Your last 5 cut jobs will show here.</div>
  </NotOpenYet>
);

/** Around a page's regions: the 19px display lead. */
export const PageSize = () => (
  <NotOpenYet
    open={false}
    size="page"
    title="Access reviews"
    why="Open once Syndra has read Zitadel at least once."
    step={{ label: "Check the connection", href: "/system" }}
  >
    <div style={{ display: "grid", gridTemplateColumns: "repeat(3, minmax(0, 1fr))", gap: 16 }}>
      <StatCard label="Due this week" value="12" detail="Across 4 projects" />
      <StatCard label="Overdue" value="3" detail="Oldest by 9 days" />
      <StatCard label="Done this term" value="148" detail="Since 1 Aug" />
    </div>
  </NotOpenYet>
);

/** Open: only its contents. */
export const Open = () => (
  <NotOpenYet open title="Storage" why="Opens once your TrueNAS password is set.">
    <div style={{ fontSize: 14.5 }}>projects · Read and write</div>
  </NotOpenYet>
);
