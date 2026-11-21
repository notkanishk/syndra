// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render } from "@testing-library/react";
import type { ReactElement } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { TRUENAS_SCHEMA } from "@/lib/__fixtures__/truenas-schema";
import { CoreMemberCard } from "@/components/kinds/MemberTargetCard";
import { StorageMemberCard } from "@/components/kinds/storage/MyStorage";
import { PersonStorage } from "@/components/kinds/storage/PersonStorage";
import { stateOf } from "@/components/kinds/storage/state";
import { PersonSystemCard } from "@/components/people/PersonSystems";
import { ReadResult } from "@/components/results/ReadResult";
import { MEMBER } from "@/components/results/copy";
import type { MyTargetView, PersonTargetView } from "@/lib/queries/useMyStorage";

/**
 * Design §13.6's guard, on every member and person consumer: a view whose
 * add-on read is `none` draws the why and its next step, and never a 0, an
 * empty state, a date or a password step. Which box is down is not a member's
 * to fix (one sentence for both outages); an operator is told which side.
 */
const state = vi.hoisted(() => ({ views: [] as unknown[] }));
vi.mock("@/lib/queries/useMyStorage", async (orig) => ({
  ...(await orig<typeof import("@/lib/queries/useMyStorage")>()),
  usePersonTargets: () => ({ data: state.views, error: null, refetch: () => {} }),
  usePersonTargetsAgain: () => ({ press: async () => {}, pending: false, error: null }),
}));
vi.mock("@/lib/queries/useCatalogUsers", () => ({ useCatalogProjects: () => ({ data: [], isLoading: false, error: null, refetch: vi.fn() }) }));
vi.mock("@/lib/queries/useTargets", () => ({ useTargets: () => ({ data: [{ target: "truenas", name: "TrueNAS", kind: "storage" }] }) }));
vi.mock("@/lib/queries/useIndicators", () => ({ useAutoRetry: () => true }));
vi.mock("@/lib/ui-view", () => ({ useUiView: () => ({ view: "basic", setView: () => {} }), useIsAdvanced: () => false }));
vi.mock("@/components/names/UserName", () => ({ UserName: ({ id }: { id: string }) => <span>{id}</span> }));
vi.mock("@/lib/queries/useNameResolver", () => ({ useNameResolver: () => ({ resolveRole: (_p: string, k: string) => ({ value: { display_name: k } }) }) }));

const WHYS = [
  ["the add-on down", { reason: "addon_unreachable", cause: "no_answer" }, "addon_unreachable"],
  ["the app down", { reason: "app_unreachable" }, "app_unreachable"],
] as const;

/** A view with an account, no password recorded, and nothing read: the case that once asked for a password. */
const view = (why: { reason: string; cause?: string }, health: MyTargetView["health"], kind = "storage"): PersonTargetView => ({
  target: "truenas",
  name: "TrueNAS",
  kind,
  entitlement_schema: TRUENAS_SCHEMA,
  entitled: true,
  account: { username: "becky.lynch", bound_at: "2026-09-03T10:00:00Z" },
  credential: { set: false },
  health,
  since: "2026-11-21T09:40:00Z",
  usage: { reported: false },
  read: { state: "none", why },
  holds: [],
});

const withClient = (ui: ReactElement) => render(<QueryClientProvider client={new QueryClient()}>{ui}</QueryClientProvider>);

/** No number, no empty state, no date, no password step. */
function expectNothingFromANonRead(text: string) {
  expect(text).not.toMatch(/\b0\b/);
  expect(text).not.toMatch(/Nothing saved here yet|No folder is open|None yet|lists nothing|reports no usage|No recorded activity/);
  expect(text).not.toMatch(/\b\d{1,2} (Jan|Feb|Mar|Apr|May|Jun|Jul|Aug|Sept?|Oct|Nov|Dec)\b|was read|last read|said at/);
  expect(text).not.toMatch(/Set a password|Set password|Set new password/);
}

beforeEach(() => {
  state.views = [];
});

describe("a member's view of a system that was not read", () => {
  it.each(WHYS)("the storage card, %s: one sentence and its step", (_name, why, health) => {
    const { container } = withClient(<StorageMemberCard view={view(why, health)} />);
    expect(container.textContent).toContain("TrueNAS can't be read right now. Nothing for you to do.");
    expect(container.textContent).not.toContain("add-on");
    expectNothingFromANonRead(container.textContent ?? "");
  });

  it.each(WHYS)("the core card, %s: one sentence and its step", (_name, why, health) => {
    const { container } = withClient(<CoreMemberCard view={view(why, health, "inventory")} />);
    expect(container.textContent).toContain("TrueNAS can't be read right now. Nothing for you to do.");
    expectNothingFromANonRead(container.textContent ?? "");
  });

  it("one read with nothing to give", () => {
    const { container } = render(
      <ReadResult read={{ operation: "holdings.list", shape: "records", read: { state: "none", why: { reason: "read_refused" } } }} name="TrueNAS" reader={MEMBER} />,
    );
    expect(container.textContent).toContain("TrueNAS can't be read right now. Ask makerspace staff.");
    expectNothingFromANonRead(container.textContent ?? "");
  });
});

describe("an operator's view of a person on a system that was not read", () => {
  it("the storage card names the add-on as the side that is down", () => {
    state.views = [view(WHYS[0][1], "addon_unreachable")];
    const { container } = withClient(<PersonStorage userId="u1" name="Becky" />);
    expect(container.textContent).toContain("TrueNAS's add-on isn't answering");
    expect(container.textContent).toContain("Syndra can't reach TrueNAS's add-on. Check that its container is running.");
    expectNothingFromANonRead((container.textContent ?? "").replace(/Made by Syndra on 3 Sept/, ""));
  });

  it("a system card names the app as the side that is down", () => {
    const { container } = withClient(<PersonSystemCard view={view(WHYS[1][1], "app_unreachable", "machine")} userId="u1" name="Becky" />);
    expect(container.textContent).toContain("TrueNAS isn't answering");
    expect(container.textContent).toContain("TrueNAS's add-on is running but can't reach TrueNAS. Check that TrueNAS is on.");
  });
});

describe("the member's state", () => {
  it("asks whether the system was read before it asks for a password (design §13.6)", () => {
    const none = view(WHYS[0][1], "addon_unreachable");
    expect(stateOf(none)).toBe("unread");
    // The same account, read now with no storage told and no password recorded: then, and only then, the password step.
    expect(stateOf({ ...none, read: { state: "now", takenAt: "2026-11-21T10:00:00Z", data: {} } })).toBe("needs-password");
  });
});
