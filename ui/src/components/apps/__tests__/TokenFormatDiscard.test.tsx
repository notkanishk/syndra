// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { TokenFormatEditor } from "@/components/apps/TokenFormatEditor";
import type { ClaimShape } from "@/lib/queries/useClaimShape";
import { gapSettled } from "@/test-utils/leaving";
import { motionInTests } from "@/test-utils/motion";

/**
 * A-08 §5: Discard is a change in place, so nothing arrives. Untouched rows hold still, edited
 * values crossfade back in their seats, a claim added since fades and its gap settles, and one
 * removed since opens its gap and tints in.
 */

vi.mock("@/lib/queries/useClaimShape", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@/lib/queries/useClaimShape")>();
  return {
    ...actual,
    useClaimVocabulary: () => ({
      data: { attributes: ["user_id", "email", "team"], formats: ["array", "csv", "space_delimited"] },
    }),
    useSaveProjectClaimProfile: () => ({ mutateAsync: vi.fn(), isPending: false }),
    useSaveAppClaimOverride: () => ({ mutateAsync: vi.fn(), isPending: false }),
    useDeleteAppClaimOverride: () => ({ mutateAsync: vi.fn(), isPending: false }),
  };
});

const shape: ClaimShape = {
  project_id: "pLaser",
  project_name: "Laser Lab",
  default: {
    project_id: "pLaser",
    claim_name: "syndra.laser.roles",
    format_type: "array",
    attribute_claims: { "syndra.laser.email": "email", "syndra.laser.team": "team" },
    static_claims: {},
  },
  overrides: [],
  applications: [],
  emitted_keys: [],
  conflicts: [],
};

// One query object for the whole test, as the page's own read is: a profile that is a new
// object on every render would be a different profile on every render.
function setup() {
  const query = { data: shape, isLoading: false, error: null, refetch: vi.fn() } as never;
  return render(
    <QueryClientProvider client={new QueryClient()}>
      <TokenFormatEditor
        projectId="pLaser"
        applicationId="app_bookings"
        applicationName="Bookings"
        shape={query}
        siblingCount={1}
      />
    </QueryClientProvider>,
  );
}

const keys = () => screen.getAllByLabelText("Claim name") as HTMLInputElement[];
const rowOf = (input: HTMLElement) => input.closest(".leave-row")!;
const discard = () => fireEvent.click(screen.getByRole("button", { name: "Discard changes" }));

beforeEach(() => motionInTests());

describe("Discard in the token format editor", () => {
  it("holds the rows that were never touched: the same elements, nothing arriving", () => {
    setup();
    const [email, team] = keys();
    fireEvent.change(team, { target: { value: "syndra.laser.group" } });
    discard();

    // Same element, same seat. Nothing opened and nothing tinted in on it.
    expect(screen.getByDisplayValue("syndra.laser.email")).toBe(email);
    expect(rowOf(email)).not.toHaveAttribute("data-opens");
    expect(rowOf(email)).not.toHaveAttribute("data-fresh");
    expect(rowOf(email)).not.toHaveAttribute("data-leaving");
    expect(email.closest("[inert]")).toBeNull();
    // The row that was edited is the same row too: its words were put back, it did not leave.
    const edited = rowOf(screen.getByDisplayValue("syndra.laser.team"));
    expect(edited).not.toHaveAttribute("data-leaving");
    expect(edited).not.toHaveAttribute("data-opens");
  });

  it("crossfades an edited value back in its seat, and only that seat", () => {
    setup();
    const [email, team] = keys();
    fireEvent.change(team, { target: { value: "syndra.laser.group" } });
    discard();

    // The saved words are in the seat, and what was typed is still readable, going, over them.
    const back = screen.getByDisplayValue("syndra.laser.team");
    expect(back.closest("[inert]")).toBeNull();
    const going = screen.getByDisplayValue("syndra.laser.group");
    expect(going.closest("[inert]")).not.toBeNull();
    expect(going.closest("[aria-hidden]")).not.toBeNull();
    // A seat nobody edited has nothing to crossfade: one layer, and it is the same element.
    expect(email.closest("[inert]")).toBeNull();
    expect(screen.getByDisplayValue("syndra.laser.email")).toBe(email);
    expect(screen.getAllByDisplayValue("syndra.laser.email")).toHaveLength(1);
  });

  it("crossfades the roles claim when that was what was edited", () => {
    setup();
    const claim = screen.getByPlaceholderText("syndra.laser.roles");
    fireEvent.change(claim, { target: { value: "laser.roles.v2" } });
    discard();
    expect(screen.getByDisplayValue("syndra.laser.roles").closest("[inert]")).toBeNull();
    expect(screen.getByDisplayValue("laser.roles.v2").closest("[inert]")).not.toBeNull();
  });

  it("fades a claim the reader added, then settles its gap", () => {
    setup();
    fireEvent.click(screen.getByRole("button", { name: "Add a claim" }));
    const added = keys()[2];
    discard();

    const row = rowOf(added);
    expect(row).toHaveAttribute("data-leaving");
    expect(row).toHaveAttribute("data-closing");
    expect(row).toHaveAttribute("aria-hidden", "true");
    // The two the form opened with hold still.
    expect(rowOf(keys()[0])).not.toHaveAttribute("data-leaving");

    gapSettled(row);
    expect(keys()).toHaveLength(2);
  });

  it("opens the gap of a claim the reader removed, then tints it in", () => {
    setup();
    const [email] = keys();
    fireEvent.click(screen.getAllByRole("button", { name: /^Remove / })[1]);
    const gone = rowOf(keys().find((input) => input.value === "syndra.laser.team")!);
    gapSettled(gone);
    expect(keys()).toHaveLength(1);

    discard();
    const back = keys().find((input) => input.value === "syndra.laser.team")!;
    expect(rowOf(back)).toHaveAttribute("data-opens");
    // The one that stayed held still through all of it.
    expect(keys()[0]).toBe(email);
    expect(rowOf(email)).not.toHaveAttribute("data-opens");
    expect(screen.queryByRole("button", { name: "Discard changes" })).not.toBeInTheDocument();
  });

  it("turns a claim round if it is brought back while it is still fading out", () => {
    setup();
    fireEvent.click(screen.getAllByRole("button", { name: /^Remove / })[1]);
    const fading = keys().find((input) => input.value === "syndra.laser.team")!;
    expect(rowOf(fading)).toHaveAttribute("data-leaving");

    discard();
    // The same element, no longer leaving, and not replayed as an arrival.
    expect(keys().find((input) => input.value === "syndra.laser.team")).toBe(fading);
    expect(rowOf(fading)).not.toHaveAttribute("data-leaving");
    expect(rowOf(fading)).not.toHaveAttribute("data-opens");
  });

  it("is simply back in reduced motion: no layer, no fade", () => {
    motionInTests(false);
    setup();
    const [, team] = keys();
    fireEvent.change(team, { target: { value: "syndra.laser.group" } });
    discard();
    expect(screen.getByDisplayValue("syndra.laser.team")).toBeInTheDocument();
    expect(screen.queryByDisplayValue("syndra.laser.group")).toBeNull();
  });
});
