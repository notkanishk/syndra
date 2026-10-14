import { ButtonLink } from "ui";

const row = { display: "flex", flexWrap: "wrap", alignItems: "flex-start", gap: 12 } as const;

/** Same surface as Button, but navigates. */
export const Variants = () => (
  <div style={row}>
    <ButtonLink href="/requests/new" variant="accent">Request access</ButtonLink>
    <ButtonLink href="/bundles/new" variant="accentSoft">New bundle</ButtonLink>
    <ButtonLink href="/users/aanya" variant="outline">Open profile</ButtonLink>
    <ButtonLink href="/audit" variant="ghost">View audit log</ButtonLink>
  </div>
);

export const Sizes = () => (
  <div style={row}>
    <ButtonLink href="/roles" size="sm">Roles</ButtonLink>
    <ButtonLink href="/roles" size="md">All roles</ButtonLink>
    <ButtonLink href="/requests" variant="accent" size="sm">Review</ButtonLink>
    <ButtonLink href="/requests" variant="accent" size="md">Review requests</ButtonLink>
  </div>
);
