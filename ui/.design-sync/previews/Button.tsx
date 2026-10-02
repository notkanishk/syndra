import { Button } from "ui";

const row = { display: "flex", flexWrap: "wrap", alignItems: "flex-start", gap: 12 } as const;

export const Variants = () => (
  <div style={row}>
    <Button variant="accent">Approve request</Button>
    <Button variant="accentSoft">Add to bundle</Button>
    <Button variant="outline">Edit role</Button>
    <Button variant="ghost">Cancel</Button>
    <Button variant="danger">Remove access</Button>
  </div>
);

export const Sizes = () => (
  <div style={row}>
    <Button variant="accent" size="sm">Grant</Button>
    <Button variant="accent" size="md">Grant access</Button>
    <Button variant="outline" size="sm">Details</Button>
    <Button variant="outline" size="md">View details</Button>
  </div>
);

export const Pending = () => (
  <div style={row}>
    <Button variant="accent" isPending>Saving…</Button>
    <Button variant="outline" isPending>Checking Zitadel…</Button>
  </div>
);

export const DisabledWithReason = () => (
  <div style={row}>
    <Button variant="accent" disabled reason="Pick at least one person to grant this role to.">
      Grant role
    </Button>
    <Button variant="danger" disabled reason="Only an operator can remove access.">
      Remove access
    </Button>
  </div>
);

/** The solid destructive fill only ever appears as a dialog's confirming action. */
export const DialogConfirm = () => (
  <div style={row}>
    <Button variant="ghost">Keep access</Button>
    <Button variant="dangerConfirm">Remove 3 people</Button>
  </div>
);
