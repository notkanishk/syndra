import { Card, CardRow, RoleRef, SyndraRoot, UserName } from "ui";

const catalog = {
  users: [
    { id: "284019337710552064", name: "Aanya Rao", email: "aanya.rao@example.edu", title: "Student", team: "Fabrication", status: "active", avatar: "" },
    { id: "284019337710553311", name: "Kabir Mehta", email: "kabir.mehta@example.edu", title: "Student", team: "Wood Shop", status: "active", avatar: "" },
    { id: "284019337710554187", name: "Priya Nair", email: "priya.nair@example.edu", title: "Makerspace staff", team: "Staff", status: "active", avatar: "" },
  ],
  projects: [
    { id: "382075332397630470", name: "Laser Cutter", kind: "machine", description: "", roles: [
      { key: "operator", label: "Operator", description: "" },
      { key: "supervisor", label: "Supervisor", description: "" },
    ] },
    { id: "382075332397630481", name: "Wood Shop", kind: "space", description: "", roles: [
      { key: "operator", label: "Operator", description: "" },
      { key: "supervisor", label: "Supervisor", description: "" },
    ] },
  ],
  applications: [],
};
const seed = [{ key: ["name-catalog"], value: catalog }, { key: ["name-bundles"], value: [] }];

const muted = { flex: 1, fontSize: 14.5, color: "var(--muted)" } as const;
const who = { width: 170, fontSize: 15, fontWeight: 600 } as const;

/** Always the pair: the project resolved to its name, the role key in mono. */
export const Pair = () => (
  <SyndraRoot data={seed}>
    <div style={{ display: "flex", flexDirection: "column", gap: 10, fontSize: 15 }}>
      <RoleRef projectId="382075332397630470" roleKey="operator" />
      <RoleRef projectId="382075332397630481" roleKey="supervisor" />
    </div>
  </SyndraRoot>
);

/** In grant rows, the way the people and audit lists compose it. */
export const InRows = () => (
  <SyndraRoot data={seed}>
    <Card>
      <CardRow first>
        <div style={who}><UserName id="284019337710552064" /></div>
        <span style={muted}><RoleRef projectId="382075332397630470" roleKey="operator" /></span>
      </CardRow>
      <CardRow>
        <div style={who}><UserName id="284019337710553311" /></div>
        <span style={muted}><RoleRef projectId="382075332397630481" roleKey="supervisor" /></span>
      </CardRow>
    </Card>
  </SyndraRoot>
);

/** Half a pair is no pair: either side missing renders the dash. */
export const Missing = () => (
  <SyndraRoot data={seed}>
    <div style={{ fontSize: 15 }}><RoleRef projectId="382075332397630470" roleKey={null} /></div>
  </SyndraRoot>
);
