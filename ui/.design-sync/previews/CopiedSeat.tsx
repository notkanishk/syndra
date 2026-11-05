import { CopiedSeat } from "ui";

const row = { display: "flex", flexWrap: "wrap", gap: 24, fontSize: 13, fontWeight: 600 } as const;

/** One seat sized to the wider word, so the button never changes width. The words crossfade on tint; under reduced motion they swap at once. */
export const IdleAndDone = () => (
  <div style={row}>
    <CopiedSeat done={false} idle="Copy" word="Copied" />
    <CopiedSeat done idle="Copy" word="Copied" />
  </div>
);

export const SelectionHasNoDot = () => (
  <div style={row}>
    <CopiedSeat done idle="Copy" word="Selected" dot={false} />
  </div>
);
