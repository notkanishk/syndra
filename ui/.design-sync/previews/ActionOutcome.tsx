import { ActionOutcome } from "ui";

const col = { display: "grid", gap: 12, maxWidth: 560 } as const;

export const Block = () => (
  <div style={col}>
    <ActionOutcome outcome={{ kind: "applied", message: "Granted Operator on Laser cutter to Aanya Rao" }} />
    <ActionOutcome
      outcome={{
        kind: "queued",
        message: "Applied to 0 people. 4 people are waiting to be sent to Zitadel",
        detail: "Recorded in Syndra and waiting to be sent to Zitadel. Nothing has changed there yet; send it from Pending changes.",
      }}
    />
    <ActionOutcome outcome={{ kind: "no_change", message: "Kabir Mehta already holds Member on 3D printers" }} />
  </div>
);

export const BlockGrave = () => (
  <div style={col}>
    <ActionOutcome
      outcome={{
        kind: "refused",
        message: "Only a makerspace staff member can grant Supervisor on the Wood shop",
        requestId: "req_7f3a92c1e04b",
      }}
    />
    <ActionOutcome
      outcome={{
        kind: "failed",
        message: "Zitadel did not answer in time",
        detail: "Try again in a minute. If it keeps failing, quote the reference below.",
        requestId: "req_b81d05e6a7c3",
      }}
    />
  </div>
);

export const Inline = () => (
  <div style={col}>
    <ActionOutcome placement="inline" outcome={{ kind: "applied", message: "Extended to 15 Dec" }} />
    <ActionOutcome placement="inline" outcome={{ kind: "queued", message: "Removal recorded" }} />
    <ActionOutcome placement="inline" outcome={{ kind: "no_change", message: "Already a Member" }} />
    <ActionOutcome placement="inline" outcome={{ kind: "refused", message: "Needs a supervisor" }} />
    <ActionOutcome placement="inline" outcome={{ kind: "failed", message: "Zitadel did not answer" }} />
  </div>
);
