import { Card, CardColumns, CardRow } from "ui";

const flex = { flex: 1, minWidth: 0, fontSize: 15, fontWeight: 600 } as const;
const num = (w: number) => ({ width: w, flexShrink: 0, textAlign: "right" as const, fontSize: 15 });
const apps = { width: 200, fontSize: 14, color: "var(--muted)" } as const;

/** Column headings over table-shaped rows (projects list). Hidden below tablet width. */
export const ProjectsTable = () => (
  <Card>
    <CardColumns>
      <span style={{ flex: 1, minWidth: 0 }}>Project</span>
      <span style={{ width: 92, flexShrink: 0, textAlign: "right" }}>People</span>
      <span style={{ width: 60, flexShrink: 0, textAlign: "right" }}>Roles</span>
      <span style={{ width: 200 }}>Apps served</span>
    </CardColumns>
    <CardRow>
      <span style={flex}>Fabrication Lab</span><span style={num(92)}>48</span><span style={num(60)}>6</span>
      <span style={apps}>Door access, Booking</span>
    </CardRow>
    <CardRow>
      <span style={flex}>Electronics Bench</span><span style={num(92)}>21</span><span style={num(60)}>3</span>
      <span style={apps}>TrueNAS</span>
    </CardRow>
    <CardRow>
      <span style={flex}>Wood Shop</span><span style={num(92)}>12</span><span style={num(60)}>4</span>
      <span style={apps}>Door access</span>
    </CardRow>
  </Card>
);
