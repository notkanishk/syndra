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
  name: "TrueNAS",
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
    name: "Doors",
    kind: "access",
    entitlement_schema: [{ name: "active", role: "account" }],
    entitled: true,
    account: { username: "mick.foley" },
    credential: { set: false },
    reachable: true,
    standing: { usable: false, waiting_on: "member", note: "No card enrolled yet", next: "Enrol a card at the front desk" },
  };

  it("renders the account's standing and its next step, from the add-on's words", () => {
    render(<MemberTargetCard view={view} />);
    expect(screen.getByText("Waiting on you")).toBeInTheDocument();
    expect(screen.getByText("No card enrolled yet")).toBeInTheDocument();
    expect(screen.getByText("Enrol a card at the front desk")).toBeInTheDocument();
  });

  it("says there is nothing to do when the add-on reported no standing", () => {
    render(<CoreMemberCard view={{ ...view, standing: undefined }} />);
    expect(screen.getByText("Nothing for you to do.")).toBeInTheDocument();
  });

  it("chooses the kind's own module: storage gets the storage card, not the core one", () => {
    render(<MemberTargetCard view={{ ...view, kind: "storage" }} />, { wrapper: roster([]) });
    expect(screen.queryByText("Enrol a card at the front desk")).toBeNull();
  });

  // Owner, 11 Nov: a system Syndra has not read is the operators' alone. A
  // member gets no card for it, never a guess at a kind.
  it("draws no member card for a system not read yet", () => {
    const { container } = render(<MemberTargetCard view={{ ...view, kind: undefined, entitlement_schema: undefined }} />);
    expect(container).toBeEmptyDOMElement();
  });

  it("words each state with its next step, by the system's name", () => {
    const { rerender } = render(<CoreMemberCard view={{ ...view, account: undefined, entitled: false, standing: undefined }} />);
    expect(screen.getByText("None of your roles reaches Doors.")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Ask for access in Requests" })).toHaveAttribute("href", "/requests?ask");
    rerender(<CoreMemberCard view={{ ...view, account: undefined, entitled: true, standing: undefined }} />);
    expect(screen.getByText("Your account on Doors is not made yet.")).toBeInTheDocument();
    expect(screen.getByText("Nothing for you to do. Syndra makes it.")).toBeInTheDocument();
    rerender(<CoreMemberCard view={{ ...view, health: "offline" }} />);
    expect(screen.getByText("Doors is offline")).toBeInTheDocument();
    expect(screen.getByText(/This page updates on its own when Doors returns\./)).toBeInTheDocument();
  });
});

describe("a hold on the account is known by role, not by name", () => {
  const items = (field: string, value: string) => [{ field, value, reason: "safety", target: "doors" }];

  it("reads another kind's account field as the whole account", () => {
    const schema = [{ name: "active", role: "account" as const }];
    render(<Withheld audience="operator" items={items("active", "true")} schemaOf={() => schema} />);
    expect(screen.getByText(/Their .* account/i)).toBeInTheDocument();
  });

  it("does not treat a name the schema calls something else as the account", () => {
    const schema = [{ name: "active", role: "account" as const }, { name: "enabled", role: "membership" as const, label: "Zone" }];
    render(<Withheld audience="operator" items={items("enabled", "north")} schemaOf={() => schema} />);
    expect(screen.getByText("north")).toBeInTheDocument();
  });

  it("guesses nothing when the schema has not been read: the value shows as the code it is", () => {
    render(<Withheld audience="operator" items={items("enabled", "true")} schemaOf={() => undefined} />);
    expect(screen.getByText("true")).toBeInTheDocument();
  });
});
