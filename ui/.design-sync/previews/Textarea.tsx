import { useState } from "react";
import { Textarea, FieldLabel, FieldHint } from "ui";

const box = { width: 360 } as const;

export const Filled = () => {
  const [value, setValue] = useState("Finished the laser cutter safety induction with Rohan on 28 Sept.");
  return (
    <div style={box}>
      <FieldLabel htmlFor="reason">Why they need this</FieldLabel>
      <Textarea id="reason" value={value} onChange={(e) => setValue(e.target.value)} />
      <FieldHint>Kept with the request, so the next reviewer can see it.</FieldHint>
    </div>
  );
};

export const Empty = () => (
  <div style={box}>
    <FieldLabel htmlFor="note">Note for the person</FieldLabel>
    <Textarea id="note" placeholder="Say what they should do before their next session…" />
  </div>
);

export const Taller = () => (
  <div style={box}>
    <FieldLabel htmlFor="why-revoke">Why you're removing access</FieldLabel>
    <Textarea
      id="why-revoke"
      rows={4}
      defaultValue={"Left the CNC router running unattended twice this week.\nSpoke to them on Monday; access can return after a refresher session."}
    />
  </div>
);

export const Disabled = () => (
  <div style={box}>
    <FieldLabel htmlFor="locked">Reason on record</FieldLabel>
    <Textarea id="locked" disabled defaultValue="Granted by the wood shop supervisor for the furniture studio course." />
  </div>
);
