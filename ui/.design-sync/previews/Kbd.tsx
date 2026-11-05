import { Kbd } from "ui";

const row = { display: "flex", alignItems: "center", gap: 6, fontSize: 13, color: "var(--faint)" } as const;

/** A drawn key, written as it is printed on the key. */
export const Keys = () => (
  <div style={{ display: "grid", gap: 14 }}>
    <div style={row}><Kbd>/</Kbd> Go to</div>
    <div style={row}><Kbd>?</Kbd> Keys</div>
    <div style={row}><Kbd>Esc</Kbd> close</div>
    <div style={row}><Kbd>↑</Kbd><Kbd>↓</Kbd> move</div>
    <div style={row}><Kbd>Enter</Kbd> open</div>
  </div>
);

/** A modifier is named by platform at render time: the command mark on a Mac, Ctrl elsewhere. */
export const WithModifier = () => (
  <div style={row}><Kbd>Ctrl</Kbd><Kbd>K</Kbd> Go to</div>
);
