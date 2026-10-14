import { FieldHint, FieldLabel, Input, Textarea } from "ui";

const box = { width: 340 } as const;

export const UnderAnInput = () => (
  <div style={box}>
    <FieldLabel htmlFor="days">Access lasts for</FieldLabel>
    <Input id="days" defaultValue="90 days" />
    <FieldHint>After that, they'll need to ask again.</FieldHint>
  </div>
);

export const UnderATextarea = () => (
  <div style={box}>
    <FieldLabel htmlFor="why">Why they need this</FieldLabel>
    <Textarea id="why" placeholder="Their course, project or induction…" />
    <FieldHint>A sentence is enough. Supervisors read this before approving.</FieldHint>
  </div>
);
