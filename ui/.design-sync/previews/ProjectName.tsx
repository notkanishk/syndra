import { ProjectName, SyndraRoot } from "ui";

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

const col = { display: "flex", flexDirection: "column", gap: 10, fontSize: 15 } as const;

/** Project ids resolve to their names from the catalog. */
export const Resolved = () => (
  <SyndraRoot data={seed}>
    <div style={col}>
      <ProjectName id="382075332397630470" />
      <ProjectName id="382075332397630481" />
    </div>
  </SyndraRoot>
);

/** An id the catalog does not know shows the caller's fallback. */
export const UnknownWithFallback = () => (
  <SyndraRoot data={seed}>
    <div style={col}>
      <ProjectName id="382075332397639999" fallback="Unknown project" />
      <ProjectName id="382075332397638888" />
    </div>
  </SyndraRoot>
);

/** "-" is the explicit no-project marker; an empty id reads as the default dash. */
export const NoProject = () => (
  <SyndraRoot data={seed}>
    <div style={col}>
      <ProjectName id="-" />
      <ProjectName id={null} />
    </div>
  </SyndraRoot>
);
