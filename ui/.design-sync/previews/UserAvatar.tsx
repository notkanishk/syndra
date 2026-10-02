import { SyndraRoot, UserAvatar, UserName } from "ui";

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

const row = { display: "flex", alignItems: "center", gap: 14 } as const;

/** Initials resolve from the same id a row's UserName uses, at all three sizes. */
export const Sizes = () => (
  <SyndraRoot data={seed}>
    <div style={row}>
      <UserAvatar id="284019337710552064" size="row" />
      <UserAvatar id="284019337710552064" size="list" />
      <UserAvatar id="284019337710552064" size="header" />
    </div>
  </SyndraRoot>
);

/** Beside the name, as people lists pair them. */
export const WithName = () => (
  <SyndraRoot data={seed}>
    <div style={{ display: "flex", flexDirection: "column", gap: 10, fontSize: 15 }}>
      {["284019337710552064", "284019337710553311", "284019337710554187"].map((id) => (
        <div key={id} style={{ ...row, gap: 10 }}>
          <UserAvatar id={id} size="list" />
          <UserName id={id} />
        </div>
      ))}
    </div>
  </SyndraRoot>
);

/** An id the catalog does not know, or no id at all, draws the blank disc. */
export const Unknown = () => (
  <SyndraRoot data={seed}>
    <div style={row}>
      <UserAvatar id="284019337710559999" size="list" />
      <UserAvatar id={null} size="list" />
    </div>
  </SyndraRoot>
);
