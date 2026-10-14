import { ResolvedProjectName } from "ui";

const heading = { fontSize: 17, fontWeight: 600 } as const;

/** The server already named it: just the name. */
export const Resolved = () => (
  <div style={heading}><ResolvedProjectName name="Laser Lab" resolved id="382075332397630470" /></div>
);

/** The server could not: labelled, with the id it is searched by - never a made-up name. */
export const Unresolved = () => (
  <div style={heading}>
    <ResolvedProjectName name="382075332397630499" resolved={false} id="382075332397630499" />
  </div>
);
