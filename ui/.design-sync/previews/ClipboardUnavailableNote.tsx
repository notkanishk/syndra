import { CopyableValue, CommandBlock, ClipboardUnavailableNote } from "ui";

// Each card is its own page, so removing the API here cannot leak into another component's cells.
try { Object.defineProperty(navigator, "clipboard", { value: undefined, configurable: true }); } catch {}

/** Renders only where the browser has no clipboard (http on the LAN); said once under the rows it explains. */
export const UnderCopyRows = () => (
  <div style={{ display: "grid", gap: 10, maxWidth: 520 }}>
    <CopyableValue label="Your account name" value="kabir.mehta" />
    <CopyableValue label="Share path" value="\\truenas.makerspace.local\projects\design-club" />
    <ClipboardUnavailableNote />
  </div>
);

export const UnderCommandBlock = () => (
  <div style={{ display: "grid", gap: 10, maxWidth: 560 }}>
    <CommandBlock
      command="docker compose restart backend"
      caption="Run this in a terminal on the server Syndra runs on. Queued changes are kept."
    />
    <ClipboardUnavailableNote />
  </div>
);
