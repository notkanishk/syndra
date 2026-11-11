// @vitest-environment jsdom
import { fireEvent, render, screen, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { PersonSystems } from "@/components/people/PersonSystems";
import { MemberSystemPage } from "@/components/member/MemberSystemPage";
import { TargetName } from "@/components/targets/TargetName";
import { TRUENAS_SCHEMA } from "@/lib/__fixtures__/truenas-schema";
import type { PersonTargetView } from "@/lib/queries/useMyStorage";
import type { TargetSummary } from "@/lib/queries/useTargets";

/**
 * addon-contract 3.10: a person's systems that are not storage, a member's
 * page for one, and the one write place for a system's name.
 */
const state = vi.hoisted(() => ({
  views: { data: undefined as unknown, error: null as unknown, refetch: () => {} },
  mine: { data: undefined as unknown, isLoading: false, error: null as unknown, refetch: () => {} },
  replace: vi.fn(),
  saved: [] as string[],
}));

vi.mock("next/navigation", () => ({ useRouter: () => ({ replace: state.replace }) }));
vi.mock("@/lib/queries/useMyStorage", () => ({ usePersonTargets: () => state.views, useMyStorage: () => state.mine }));
vi.mock("@/lib/queries/useTargets", () => ({
  useTargets: () => ({ data: [] }),
  useSetTargetName: () => ({
    mutate: (name: string, opts?: { onSuccess?: (a: { name: string; name_set: boolean }) => void }) => {
      state.saved.push(name);
      opts?.onSuccess?.({ name: name || "Ruby", name_set: name !== "" });
    },
    isPending: false,
    error: null,
  }),
}));
vi.mock("@/lib/queries/useIndicators", () => ({ useAutoRetry: () => true }));
vi.mock("@/lib/ui-view", () => ({ useUiView: () => ({ view: "basic", setView: () => {} }), useIsAdvanced: () => false }));
vi.mock("@/components/names/shownNames", () => ({ useShownNames: () => ({ role: (_p: string, k: string) => (k === "laser" ? "Laser operator" : k) }) }));
vi.mock("@/components/names/UserName", () => ({ UserName: ({ id }: { id: string }) => <span>{id}</span> }));

const base = { entitled: true, credential: { set: false }, reachable: true, holds: [] as PersonTargetView["holds"] };
const nas: PersonTargetView = { ...base, target: "truenas", name: "TrueNAS", kind: "storage", entitlement_schema: TRUENAS_SCHEMA, account: { username: "rey" } };
const laser: PersonTargetView = {
  ...base,
  target: "laser",
  name: "Big laser",
  kind: "machine",
  entitlement_schema: [{ name: "on", role: "account" }],
  account: { username: "rey.mysterio" },
  roles: [{ project_id: "p", role_key: "laser" }],
  standing: { usable: false, waiting_on: "member", note: "Induction not done", next: "Book an induction at the front desk" },
};
const door: PersonTargetView = { ...base, target: "door", name: "Door", entitled: false };

beforeEach(() => {
  state.views = { data: [nas, laser, door], error: null, refetch: () => {} };
  state.mine = { data: undefined, isLoading: false, error: null, refetch: () => {} };
  state.replace.mockReset();
  state.saved = [];
});

describe("a person's connected systems (operators)", () => {
  it("draws every system that is not storage, by its name, and none that is", () => {
    render(<PersonSystems userId="u1" name="Rey" />);
    const section = screen.getByRole("region", { name: "Connected systems" });
    expect(within(section).getByRole("link", { name: "Big laser" })).toHaveAttribute("href", "/system/targets/laser");
    expect(within(section).queryByText("TrueNAS")).toBeNull();
    // The add-on's own words for the account, and the roles that reach it.
    expect(within(section).getByText("Induction not done")).toBeInTheDocument();
    expect(within(section).getByText("Book an induction at the front desk")).toBeInTheDocument();
    expect(within(section).getByText("Their Laser operator role reaches Big laser.")).toBeInTheDocument();
  });

  it("shows operators a system not read yet, with the way to its page", () => {
    render(<PersonSystems userId="u1" name="Rey" />);
    expect(screen.getByText(/Syndra has not read Door yet/)).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Open Door's page" })).toHaveAttribute("href", "/system/targets/door");
  });

  it("is not drawn when every system is storage", () => {
    state.views = { data: [nas], error: null, refetch: () => {} };
    const { container } = render(<PersonSystems userId="u1" name="Rey" />);
    expect(container).toBeEmptyDOMElement();
  });
});

describe("a member's page for one system", () => {
  it("draws the kind's card under the system's name", () => {
    state.mine = { ...state.mine, data: [laser] };
    render(<MemberSystemPage target="laser" />);
    expect(screen.getByRole("heading", { name: "Big laser", level: 1 })).toBeInTheDocument();
    expect(screen.getByText("Book an induction at the front desk")).toBeInTheDocument();
  });

  it("sends a storage system to its one address", () => {
    state.mine = { ...state.mine, data: [nas] };
    render(<MemberSystemPage target="truenas" />);
    expect(state.replace).toHaveBeenCalledWith("/storage");
  });

  it("says a system not connected, or not read yet, is not here, with nothing to do", () => {
    state.mine = { ...state.mine, data: [door] };
    render(<MemberSystemPage target="door" />);
    expect(screen.getByText("This system isn't connected here.")).toBeInTheDocument();
    expect(screen.getByText(/Nothing for you to do\./)).toBeInTheDocument();
    expect(screen.queryByText("Door")).toBeNull();
  });
});

describe("the Name setting, the one write place", () => {
  const laserRow = { target: "laser", name: "Ruby", name_set: false } as TargetSummary;

  it("saves the trimmed name and says what members now see", () => {
    render(<TargetName registered={laserRow} />);
    const save = screen.getByRole("button", { name: /Save name/ });
    expect(save).toBeDisabled();
    fireEvent.change(screen.getByLabelText("Name"), { target: { value: "  Big laser  " } });
    fireEvent.click(screen.getByRole("button", { name: /Save name/ }));
    expect(state.saved).toEqual(["Big laser"]);
    expect(screen.getByText("Members see it as Big laser.")).toBeInTheDocument();
  });

  it("goes back to the system's own name", () => {
    render(<TargetName registered={{ ...laserRow, name: "Big laser", name_set: true }} />);
    fireEvent.click(screen.getByRole("button", { name: "Use the system's own name" }));
    expect(state.saved).toEqual([""]);
    expect(screen.getByText("Members see it as Ruby.")).toBeInTheDocument();
  });
});
