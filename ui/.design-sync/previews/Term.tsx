import { Term } from "ui";

const p = { fontSize: 14.5, lineHeight: 1.6, color: "var(--muted)", maxWidth: 520, margin: 0 } as const;

export const InSentence = () => (
  <p style={p}>
    Changes that have not reached <Term name="zitadel">Zitadel</Term> yet wait in the{" "}
    <Term name="outbox">outbox</Term>; nothing is lost while it drains.
  </p>
);

export const DefaultTitle = () => (
  <p style={p}>
    Each edit to a <Term name="bundle" /> starts a <Term name="cascade" /> for everyone who holds it.
  </p>
);
