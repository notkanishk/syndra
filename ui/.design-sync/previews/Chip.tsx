import { Chip } from "ui";

const row = { display: "flex", flexWrap: "wrap", alignItems: "center", gap: 8 } as const;

export const BundleMembership = () => (
  <div style={row}>
    <Chip>Laser cutter</Chip>
    <Chip>Wood shop</Chip>
    <Chip>3D printers</Chip>
  </div>
);

export const AppsServed = () => (
  <div style={row}>
    <Chip>TrueNAS</Chip>
    <Chip>Makerspace wiki</Chip>
  </div>
);
