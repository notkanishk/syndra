// @vitest-environment jsdom
import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { TokenPreview } from "@/components/apps/TokenPreview";

const state = vi.hoisted(() => ({
  simulation: { custom_claims: {}, owned_claims: [], claim_owners: [], raw_roles: [] } as Record<
    string,
    unknown
  >,
}));

vi.mock("@/lib/queries/useCatalogUsers", () => ({
  useCatalogUsers: () => ({
    data: [{ id: "u1", name: "Meera Anand" }],
    isLoading: false,
    error: null,
  }),
}));

vi.mock("@/lib/queries/useApplications", () => ({
  useTokenSimulator: () => ({
    data: state.simulation,
    isLoading: false,
    isFetching: false,
    error: null,
    refetch: vi.fn(),
  }),
}));

beforeEach(() => {
  state.simulation = {
    custom_claims: {},
    owned_claims: [],
    claim_owners: [],
    raw_roles: [],
  };
});

function renderPreview() {
  return render(
    <TokenPreview applicationId="a1" applicationName="Badge Reader" projectId="p1" />,
  );
}

/**
 * A token with nothing in it is the answer this screen is asked for most
 * often, and it is also the one most easily read as a broken preview. It was
 * being delivered as a `//` comment in the quietest type on the page, phrased
 * as a fact about the project rather than about the person just picked.
 */
describe("a preview that would issue no roles", () => {
  it("says so in a sentence, naming the app and the person", async () => {
    renderPreview();
    expect(
      await screen.findByText(/Badge Reader would issue a token with no roles for Meera Anand/),
    ).toBeTruthy();
  });

  it("rules out the misreading rather than leaving it open", async () => {
    renderPreview();
    expect(await screen.findByText(/That is not an error/)).toBeTruthy();
  });

  it("says nothing of the sort when the token has claims in it", () => {
    state.simulation = {
      custom_claims: { "syndra.roles": ["operator"] },
      owned_claims: ["syndra.roles"],
      claim_owners: [],
      raw_roles: ["operator"],
    };
    renderPreview();
    expect(screen.queryByText(/no roles for/)).toBeNull();
  });
});
