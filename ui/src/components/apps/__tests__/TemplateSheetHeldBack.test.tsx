// @vitest-environment jsdom
import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { TemplateSheet } from "@/components/apps/ShapeSheets";
import type { AppClaimShape, ClaimTemplate } from "@/lib/queries/useClaimShape";

const profile = { claims: [], include_new: true, lands: "both" as const };
const shape = {
  application: { id: "a1", name: "NetBird", project_id: "p1", consumer: "oidc" },
  source: "project_default",
  profile,
  project_roles: [],
} as unknown as AppClaimShape;
const template = { id: "netbird", version: 1, name: "NetBird", profile, reads: "access_token" } as unknown as ClaimTemplate;

const show = (heldBack?: string) =>
  render(<TemplateSheet template={template} shape={shape} project="Network" heldBack={heldBack} onClose={() => {}} onStart={() => {}} />);

describe("the template sheet's note", () => {
  it("promises the default only until a save when a save would reach the app", () => {
    show();
    expect(screen.getByText("Makes a draft. NetBird keeps Network's default until you save.")).toBeInTheDocument();
  });

  it("says a save won't change what the app gets while something holds it back", () => {
    show("Zitadel doesn't say which app is signing in, so NetBird keeps getting Network's default until Zitadel is upgraded.");
    expect(screen.queryByText(/until you save/)).toBeNull();
    expect(screen.getByText(/Saving it won't change what NetBird gets yet\. Zitadel doesn't say which app is signing in/)).toBeInTheDocument();
  });
});
