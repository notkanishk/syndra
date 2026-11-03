import { Radio } from "ui";

const label = { display: "flex", alignItems: "center", gap: 10, fontSize: 14.5 } as const;

/** The one radio: a dot in the same seat. */
export const Group = () => (
  <div role="radiogroup" aria-label="How long" style={{ display: "grid", gap: 12 }}>
    <label style={label}><Radio name="length" defaultChecked /> Until the end of term</label>
    <label style={label}><Radio name="length" /> For 7 days</label>
    <label style={label}><Radio name="length" /> No end date</label>
  </div>
);
