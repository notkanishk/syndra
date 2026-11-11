// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { CoreMemberCard, MemberTargetCard } from "@/components/kinds/MemberTargetCard";
import { Capabilities } from "@/components/targets/TargetOverview";
import { Withheld } from "@/components/ui/Withheld";
import type { MyTargetView } from "@/lib/queries/useMyStorage";
import type { TargetSummary } from "@/lib/queries/useTargets";

const base: TargetSummary = {
  target: "truenas",
  registered: true,
  auth_mode: "derived",
  callable: true,
  operations: [],
  circuit_open: false,
};

function roster(rows: Partial<TargetSummary>[]) {
  const client = new QueryClient();
  client.setQueryData(["targets"], { targets: rows.map((r) => ({ ...base, ...r })) });
  const Wrapper = ({ children }: { children: React.ReactNode }) => <QueryClientProvider client={client}>{children}</QueryClientProvider>;
  return Wrapper;
}

describe("what the add-on says about itself (3.6)", () => {
  it("quotes also_says as given, escaped, and never as markup", () => {
    const lines = ['x-vendor: "<img src=x onerror=alert(1)>"', "features[3]: next-thing"];
    const { container } = render(<Capabilities registered={{ ...base, also_says: lines }} />);
    expect(screen.getByText(lines[0])).toBeInTheDocument();
    expect(screen.getByText("features[3]: next-thing")).toBeInTheDocument();
    expect(container.querySelector("img")).toBeNull();
    expect(screen.getByText("What the add-on also says")).toBeInTheDocument();
  });

  it("keeps its seat, saying so, when there is nothing", () => {
    render(<Capabilities registered={base} />);
    expect(screen.getByText("What the add-on also says")).toBeInTheDocument();
    expect(screen.getByText("Nothing beyond what Syndra uses.")).toBeInTheDocument();
  });

  it("says a refused description: the reason as given, the last good one in use, and what to do", () => {
    const reason = 'contract_version 3: "<b>not spoken</b>"';
    const { container } = render(<Capabilities registered={{ ...base, last_error: reason }} />);
    expect(screen.getByText(/TrueNAS's latest description of itself was not accepted/)).toBeInTheDocument();
    expect(screen.getByText(/still using the last good one/)).toBeInTheDocument();
    expect(container.querySelector("[data-last-error]")?.textContent).toBe(reason);
    expect(container.querySelector("b")).toBeNull();
    expect(container.querySelector("[data-notice-next]")?.textContent).toMatch(/Ask whoever looks after/);
  });

  it("shows no refusal when none happened", () => {
    render(<Capabilities registered={base} />);
    expect(screen.queryByText(/was not accepted/)).toBeNull();
  });
});

describe("a kind with no module of its own gets the core member card", () => {
  const view: MyTargetView = {
    target: "doors",
    entitled: true,
    account: { username: "mick.foley" },
    credential: { set: false },
    reachable: true,
    standing: { usable: false, waiting_on: "member", note: "No card enrolled yet", next: "Enrol a card at the front desk" },
  };

  it("renders the account's standing and its next step, from the add-on's words", () => {
    render(<MemberTargetCard view={view} kind="access" />, { wrapper: roster([{ target: "doors", kind: "access" }]) });
    expect(screen.getByText("Waiting on you")).toBeInTheDocument();
    expect(screen.getByText("No card enrolled yet")).toBeInTheDocument();
    expect(screen.getByText("Enrol a card at the front desk")).toBeInTheDocument();
  });

  it("says there is nothing to do when the add-on reported no standing", () => {
    render(<CoreMemberCard view={{ ...view, standing: undefined }} />);
    expect(screen.getByText("Nothing for you to do.")).toBeInTheDocument();
  });

  it("chooses by the roster's kind when none is passed", () => {
    render(<MemberTargetCard view={view} />, { wrapper: roster([{ target: "doors", kind: "machine" }]) });
    expect(screen.getByText("Enrol a card at the front desk")).toBeInTheDocument();
  });
});

describe("a hold on the account is known by role, not by name", () => {
  it("reads another kind's account field as the whole account", () => {
    const schema = [{ name: "active", role: "account" as const }, { name: "policy", role: "membership" as const, label: "Access policy" }];
    render(<Withheld audience="operator" items={[{ field: "active", value: "true", reason: "safety", target: "doors" }]} />, {
      wrapper: roster([{ target: "doors", entitlement_schema: schema }]),
    });
    expect(screen.getByText(/Their Doors account|Their doors account/i)).toBeInTheDocument();
  });

  it("does not treat a name the schema calls something else as the account", () => {
    const schema = [{ name: "active", role: "account" as const }, { name: "enabled", role: "membership" as const, label: "Zone" }];
    render(<Withheld audience="operator" items={[{ field: "enabled", value: "north", reason: "safety", target: "doors" }]} />, {
      wrapper: roster([{ target: "doors", entitlement_schema: schema }]),
    });
    expect(screen.getByText("north")).toBeInTheDocument();
  });
});
