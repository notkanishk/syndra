import { useState } from "react";
import { Input, FieldLabel, FieldHint } from "ui";

const col = { display: "flex", flexDirection: "column", gap: 18, width: 340 } as const;

export const Filled = () => {
  const [value, setValue] = useState("Laser cutter operator");
  return (
    <div style={col}>
      <div>
        <FieldLabel htmlFor="role-name">Role name</FieldLabel>
        <Input id="role-name" value={value} onChange={(e) => setValue(e.target.value)} />
        <FieldHint>People see this name on their access page.</FieldHint>
      </div>
    </div>
  );
};

export const Placeholder = () => (
  <div style={col}>
    <Input placeholder="Search name, email or role key…" aria-label="Search people" />
    <div>
      <FieldLabel htmlFor="expiry">Access ends on</FieldLabel>
      <Input id="expiry" type="date" defaultValue="2026-12-15" />
    </div>
  </div>
);

export const Disabled = () => (
  <div style={col}>
    <div>
      <FieldLabel htmlFor="role-key">Role key</FieldLabel>
      <Input id="role-key" value="laser_cutter.operator" disabled readOnly />
      <FieldHint>A role key can't change once people hold the role.</FieldHint>
    </div>
  </div>
);
