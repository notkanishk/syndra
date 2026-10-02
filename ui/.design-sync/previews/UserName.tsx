import { SyndraRoot, UserName } from "ui";

const catalog = {
  users: [
    { id: "284019337710552064", name: "Aanya Rao", email: "aanya.rao@ashoka.edu.in", title: "Student", team: "Fabrication", status: "active", avatar: "" },
    { id: "284019337710553311", name: "Kabir Mehta", email: "kabir.mehta@ashoka.edu.in", title: "Student", team: "Wood Shop", status: "active", avatar: "" },
    { id: "284019337710554187", name: "Priya Nair", email: "priya.nair@ashoka.edu.in", title: "Makerspace staff", team: "Staff", status: "active", avatar: "" },
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

const col = { display: "flex", flexDirection: "column", gap: 10, fontSize: 15 } as const;

/** Ids the catalog knows resolve to the person's name; showEmail adds the address. */
export const Resolved = () => (
  <SyndraRoot data={seed}>
    <div style={col}>
      <UserName id="284019337710552064" />
      <UserName id="284019337710553311" showEmail />
    </div>
  </SyndraRoot>
);

/** An id nothing can place: the label in words, then the raw id in mono. */
export const UnknownAccount = () => (
  <SyndraRoot data={seed}>
    <div style={col}>
      <UserName id="284019337710559999" />
      <UserName id="284019337710558888" fallback="Former member" />
    </div>
  </SyndraRoot>
);

/** Machine principals read as System, and an empty id as the fallback. */
export const SystemAndEmpty = () => (
  <SyndraRoot data={seed}>
    <div style={col}>
      <UserName id="system" />
      <UserName id={null} />
      <UserName id={undefined} fallback="Nobody yet" />
    </div>
  </SyndraRoot>
);

/** In an audit line, the way the change history composes it. */
export const InSentence = () => (
  <SyndraRoot data={seed}>
    <p style={{ fontSize: 14.5, color: "var(--muted)", margin: 0 }}>
      <UserName id="284019337710554187" /> ended laser cutter access for <UserName id="284019337710553311" />.
    </p>
  </SyndraRoot>
);
