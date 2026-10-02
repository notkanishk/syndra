import { useState } from "react";
import { SelectModeToggle } from "ui";

export const Idle = () => {
  const [on, setOn] = useState(false);
  return <SelectModeToggle active={on} onToggle={() => setOn(!on)} />;
};

export const Active = () => {
  const [on, setOn] = useState(true);
  return <SelectModeToggle active={on} onToggle={() => setOn(!on)} />;
};

export const CustomLabels = () => (
  <div style={{ display: "flex", gap: 12 }}>
    <SelectModeToggle active={false} onToggle={() => {}} idle="Choose grants" busy="Done choosing" />
    <SelectModeToggle active onToggle={() => {}} idle="Choose grants" busy="Done choosing" />
  </div>
);
