import { DegradedBanner, SyndraRoot } from "ui";

const mode = (value: Record<string, unknown>) => [{ key: ["system", "mode"], value }];

/** The directory fell back to sample data: every number on screen is fiction. */
export const DirectoryDown = () => (
  <SyndraRoot data={mode({ directory: "demo", zitadel_configured: true, degraded: true })}>
    <DegradedBanner />
  </SyndraRoot>
);

/** Same, with the backend's reason for the server operator. */
export const DirectoryDownWithReason = () => (
  <SyndraRoot data={mode({ directory: "demo", zitadel_configured: true, degraded: true, reason: "Zitadel at auth.makerspace.tools did not answer within 5 seconds." })}>
    <DegradedBanner />
  </SyndraRoot>
);

/** Live directory, sample rows still seeding: switch seeding off first. */
export const SampleDataSeeding = () => (
  <SyndraRoot data={mode({ directory: "zitadel", zitadel_configured: true, seed_active: true, seed_residue: 42, reset_command: "make reset-demo-data" })}>
    <DegradedBanner />
  </SyndraRoot>
);

/** Live directory, seeding off, rows left over from an earlier run. */
export const SampleDataLeftOver = () => (
  <SyndraRoot data={mode({ directory: "zitadel", zitadel_configured: true, seed_active: false, seed_residue: 17, reset_command: "make reset-demo-data" })}>
    <DegradedBanner />
  </SyndraRoot>
);
