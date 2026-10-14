import { useState } from "react";
import { ConfirmByTyping } from "ui";

const col = { maxWidth: 420 } as const;

export const Empty = () => {
  const [v, setV] = useState("");
  return <div style={col}><ConfirmByTyping expected="Meera Pillai" value={v} onChange={setV} /></div>;
};

export const Matching = () => {
  const [v, setV] = useState("laser cutter");
  return <div style={col}><ConfirmByTyping expected="Laser cutter" value={v} onChange={setV} noun="target name" /></div>;
};

export const Disabled = () => (
  <div style={col}>
    <ConfirmByTyping expected="wood-shop-2026" value="wood-shop-2026" onChange={() => {}} noun="bundle key" disabled />
  </div>
);
