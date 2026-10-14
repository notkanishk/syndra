import { useState } from "react";
import { Select, FieldLabel } from "ui";

const box = { width: 320 } as const;

export const Default = () => {
  const [value, setValue] = useState("laser");
  return (
    <div style={box}>
      <FieldLabel htmlFor="project">Project</FieldLabel>
      <Select id="project" value={value} onChange={(e) => setValue(e.target.value)}>
        <option value="">Any project</option>
        <option value="laser">Laser lab</option>
        <option value="cnc">CNC router</option>
        <option value="wood">Wood shop</option>
      </Select>
    </div>
  );
};

/** The accent border marks the field that is the decision on the panel. */
export const Emphasis = () => (
  <div style={box}>
    <FieldLabel htmlFor="role">Role to give</FieldLabel>
    <Select id="role" emphasis defaultValue="operator">
      <option value="member">Member</option>
      <option value="operator">Operator</option>
      <option value="supervisor">Supervisor</option>
    </Select>
  </div>
);

export const Disabled = () => (
  <div style={box}>
    <FieldLabel htmlFor="locked-role">Role</FieldLabel>
    <Select id="locked-role" disabled defaultValue="supervisor">
      <option value="supervisor">Supervisor</option>
    </Select>
  </div>
);
