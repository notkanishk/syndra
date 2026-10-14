import { useState } from "react";
import { AcknowledgeCount } from "ui";

const col = { display: "grid", gap: 12, maxWidth: 520 } as const;

export const Unchecked = () => {
  const [checked, setChecked] = useState(false);
  return (
    <div style={col}>
      <AcknowledgeCount checked={checked} onChange={setChecked} count={142} noun="people" verb="changes access for" />
    </div>
  );
};

export const CheckedWithConsequence = () => {
  const [checked, setChecked] = useState(true);
  return (
    <div style={col}>
      <AcknowledgeCount
        checked={checked}
        onChange={setChecked}
        count={38}
        noun="accounts"
        verb="removes"
        consequence="Their TrueNAS home directories stay on disk; only the sign-in is removed."
      />
    </div>
  );
};

export const SingularDisabled = () => (
  <div style={col}>
    <AcknowledgeCount checked={false} onChange={() => {}} count={1} noun="people" verb="changes access for" disabled />
  </div>
);
