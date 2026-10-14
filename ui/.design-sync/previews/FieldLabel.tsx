import { FieldLabel, FieldHint, Input, Select, Segmented } from "ui";

const box = { width: 340 } as const;

export const ForAnInput = () => (
  <div style={box}>
    <FieldLabel htmlFor="email">University email</FieldLabel>
    <Input id="email" defaultValue="aanya.rao@example.edu" />
  </div>
);

export const ForASelect = () => (
  <div style={box}>
    <FieldLabel htmlFor="space">Space</FieldLabel>
    <Select id="space" defaultValue="wood">
      <option value="laser">Laser lab</option>
      <option value="wood">Wood shop</option>
      <option value="electronics">Electronics bench</option>
    </Select>
  </div>
);

/** A group label: the control points back at it with aria-labelledby. */
export const ForAGroup = () => (
  <div style={box}>
    <FieldLabel id="mode-label">When this rule applies</FieldLabel>
    <div aria-labelledby="mode-label">
      <Segmented label="When this rule applies" value="manual" onChange={() => {}} options={[
        { value: "manual", label: "Waits under Pending changes" },
        { value: "auto", label: "Applies at once" },
      ]} />
    </div>
    <FieldHint>You can change this later.</FieldHint>
  </div>
);
