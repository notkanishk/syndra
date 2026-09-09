// @vitest-environment jsdom
import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { ApiError } from "@/lib/api-client";
import UpstreamProjectsPage from "@/app/zitadel/projects/page";
import UpstreamUsersPage from "@/app/zitadel/users/page";

/**
 * (3) A raw Go error string reaching an operator was the actual bug —
 * "zitadel client not initialized" being the literal `err.Error()` from a
 * package-level sentinel, passed straight through to `message`. Rewording the
 * sentinel (deps.go's `errNoClient`) fixes it at the one place every one of
 * these screens' failures routes through. This guards the shape rather than
 * one literal: nothing lowercase-and-colon-prefixed, Go's own style for a
 * wrapped error, may reach rendered text on either screen.
 */
const RAW_GO_ERROR = /\b[a-z][a-z ]*: /;

const zitadelNotConfigured = new ApiError(502, {
  error: "ZITADEL_ERROR",
  message: "Zitadel is not set up — Syndra has no connection configured to it",
});

vi.mock("@/lib/queries/useUpstream", () => ({
  useUpstreamProjects: () => ({ data: undefined, isLoading: false, error: zitadelNotConfigured, refetch: () => {} }),
  useUpstreamProjectRoles: () => ({ data: undefined, isLoading: false, error: null, refetch: () => {} }),
  useUpstreamUsers: () => ({ data: undefined, isLoading: false, error: zitadelNotConfigured, refetch: () => {} }),
  useUpstreamUserGrants: () => ({ data: undefined, isLoading: false, error: null, refetch: () => {} }),
  useUpstreamCreateRole: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useUpstreamUpdateRole: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useUpstreamDeleteRole: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useUpstreamAssignGrant: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useUpstreamUpdateGrant: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useUpstreamRemoveGrant: () => ({ mutateAsync: vi.fn(), isPending: false }),
}));

describe("Zitadel projects and people never render a raw backend error", () => {
  it("projects: no lowercase colon-separated Go string, and no false 'Syndra itself is fine'", async () => {
    render(<UpstreamProjectsPage />);
    const alert = await screen.findByRole("alert");
    const text = alert.textContent!;
    expect(text).toContain("Couldn't read projects from Zitadel");
    expect(text).not.toMatch(RAW_GO_ERROR);
    expect(text).not.toContain("Syndra itself is fine");
    expect(text).toContain("Zitadel is not set up");
  });

  it("people: no lowercase colon-separated Go string, and no false 'Syndra itself is fine'", async () => {
    render(<UpstreamUsersPage />);
    const alert = await screen.findByRole("alert");
    const text = alert.textContent!;
    expect(text).toContain("Couldn't read people from Zitadel");
    expect(text).not.toMatch(RAW_GO_ERROR);
    expect(text).not.toContain("Syndra itself is fine");
    expect(text).toContain("Zitadel is not set up");
  });
});
