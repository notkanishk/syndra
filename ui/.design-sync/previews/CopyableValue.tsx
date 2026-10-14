import { CopyableValue } from "ui";

const stack = { display: "grid", gap: 10, maxWidth: 520 } as const;

/** The strings a member retypes into another app: account name, share path, host. */
export const StorageDetails = () => (
  <div style={stack}>
    <CopyableValue label="Your account name" value="aanya.rao" />
    <CopyableValue label="Share path" value="\\truenas.makerspace.local\projects\robotics-club" />
    <CopyableValue label="TrueNAS host" value="truenas.makerspace.local" />
  </div>
);

export const LongValueWraps = () => (
  <div style={{ maxWidth: 360 }}>
    <CopyableValue label="Share path" value="smb://truenas.makerspace.local/projects/capstone-2026/enclosure-cad-files" />
  </div>
);
