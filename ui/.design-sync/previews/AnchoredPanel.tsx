import { useRef } from "react";
import { AnchoredPanel, Button } from "ui";

/** A popover drawn under body, placed at its trigger so no card clips it. */
export function UnderItsTrigger() {
  const anchor = useRef<HTMLSpanElement>(null);
  return (
    <div style={{ minHeight: 220 }}>
      <span ref={anchor} style={{ display: "inline-block" }}>
        <Button variant="outline" size="sm">Where this came from</Button>
      </span>
      <AnchoredPanel anchor={anchor} open width={300} expectHeight={120}>
        <div
          className="shadow-popover"
          style={{
            border: "1px solid var(--line-strong)",
            background: "var(--surface-1)",
            borderRadius: 10,
            padding: "12px 14px",
            fontSize: 13.5,
            lineHeight: 1.5,
            color: "var(--ink)",
          }}
        >
          Given by the Wood Shop bundle, version 3. Priya Nair added Kabir Mehta on 4 Sept.
        </div>
      </AnchoredPanel>
    </div>
  );
}
