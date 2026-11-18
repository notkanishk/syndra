// @vitest-environment jsdom
import type { SessionUser } from "@/lib/session";
import { render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { MemberAccess } from "@/components/member/MemberAccess";
import { inZone } from "@/test-utils/zone";

// A-24 §3.3, Q6: a member's card is lit on the first visit after its access starts, and never again.

const access = vi.hoisted(() => ({ value: {} as Record<string, unknown> }));
const changes = vi.hoisted(() => ({ value: [] as unknown[] | undefined, error: null as Error | null }));

vi.mock("@/components/member/MemberCatalog", () => ({ MemberCatalog: () => null }));
vi.mock("@/components/requests/AskForAccess", () => ({ AskForAccess: () => null }));
vi.mock("@/components/names", () => ({
  ResolvedProjectName: ({ name }: { name: string }) => <span>{name}</span>,
}));
vi.mock("@/lib/queries/useUsers", () => ({
  useUserAccess: () => access.value,
  useUserGrants: () => ({ data: [], error: null }),
}));
vi.mock("@/lib/queries/useMyStorage", () => ({ useMyStorage: () => ({ data: [] }) }));
vi.mock("@/lib/queries/useRequests", () => ({ useRequestsMine: () => ({ data: [] }) }));
vi.mock("@/lib/queries/useMyChanges", () => ({
  useMyChanges: () => ({ data: changes.value, error: changes.error, isLoading: false }),
}));
vi.mock("@/lib/queries/useCatalogUsers", () => ({ useCatalogProjects: () => ({ data: [] }) }));
vi.mock("@/lib/queries/useFieldSchema", async () => (await import("@/lib/__fixtures__/field-schema-mock")).fieldSchemaMock);

const member = (id: string): SessionUser => ({
  id, name: "Cody Rhodes", email: "cody@example.org", title: "", team: "", status: "active", avatar: "", role: "user", sessionType: "oidc",
});

const project = (id: string, name: string, standing: string | null) => ({
  project_id: id,
  project_name: name,
  project_name_resolved: true,
  effective_role_keys: ["operator"],
  source_roles: [{ role_key: "operator", reasons: [{ kind: "direct", queued: false }] }],
  derived_roles: [],
  observed_role_keys: standing === "held" ? ["operator"] : [],
  standings: { operator: standing },
});

const READ = { read_at: "2026-11-19T06:00:00Z", current: true, truncated: false };
function view(laser: string | null = "held", observation: Record<string, unknown> = READ) {
  access.value = {
    isLoading: false,
    error: null,
    data: { projects: [project("p1", "3D printing", "held"), project("p2", "Laser cutting", laser), project("p3", "Wood shop", "held")], bundles: [], allowances: [], observation },
  };
}
const given = (at: string, extra: Record<string, unknown> = {}) => ({ kind: "given", at, project_id: "p2", role_key: "operator", via: null, ...extra });

const card = (name: string) => screen.getByText(name, { selector: "span" }).closest(".card")!;
const litCards = () => Array.from(document.querySelectorAll(".card[data-lit]"));

beforeEach(() => {
  window.localStorage.clear();
  inZone("Asia/Kolkata");
  vi.useFakeTimers({ toFake: ["Date"] });
  vi.setSystemTime(new Date("2026-11-19T10:00:00Z")); // 15:30 in Kolkata
  view();
  changes.value = [given("2026-11-19T04:00:00Z")];
  changes.error = null;
});
afterEach(() => {
  vi.useRealTimers();
  vi.restoreAllMocks();
  inZone(process.env.TZ);
});

describe("the lit card", () => {
  it("lights the card whose access began today, in its alphabetical place, with From today as its note", () => {
    render(<MemberAccess session={member("u1")} />);
    const lit = card("Laser cutting");
    expect(lit).toHaveClass("lit-card");
    expect(lit).toHaveAttribute("data-lit");
    expect(lit.querySelector("[data-card-note]")).toHaveTextContent("From today");
    expect(litCards()).toEqual([lit]);
    // Nothing is pinned: it stays between 3D printing and Wood shop.
    const order = Array.from(document.querySelectorAll('.card [id^="card-"]')).map((title) => title.textContent);
    expect(order).toEqual(["3D printing", "Laser cutting", "Wood shop"]);
  });

  it("is once: the next visit is a card like the others", () => {
    render(<MemberAccess session={member("u1")} />).unmount();
    render(<MemberAccess session={member("u1")} />);
    expect(litCards()).toEqual([]);
    expect(card("Laser cutting").querySelector("[data-card-note]")).toBeNull();
  });

  it("is each member's own on a shared computer, kept by a hash and never the id", () => {
    render(<MemberAccess session={member("u1")} />).unmount();
    render(<MemberAccess session={member("u2")} />);
    expect(litCards()).toHaveLength(1);
    expect(window.localStorage.getItem("syndra:lit")).not.toMatch(/u1|u2/);
  });

  it("counts today on the makerspace's day, not the browser's", () => {
    // A makerspace on UTC, read from a browser in Kolkata (the test run's own zone). At 20:00Z it is
    // still the 19th at the makerspace and already the 20th in the browser: 10:00Z was today.
    inZone("UTC");
    vi.setSystemTime(new Date("2026-11-19T20:00:00Z"));
    changes.value = [given("2026-11-19T10:00:00Z")];
    render(<MemberAccess session={member("u1")} />);
    expect(litCards()).toHaveLength(1);
  });

  it("never lights for a role Zitadel doesn't hold or one on its way, and doesn't spend it on them", () => {
    view("not_held");
    render(<MemberAccess session={member("u1")} />).unmount();
    expect(litCards()).toEqual([]);
    view();
    changes.value = [given("2026-11-19T04:00:00Z", { waiting: true })];
    render(<MemberAccess session={member("u1")} />).unmount();
    expect(litCards()).toEqual([]);
    // Zitadel holds it now and it has arrived: this is the first visit after it starts.
    changes.value = [given("2026-11-19T04:00:00Z")];
    render(<MemberAccess session={member("u1")} />);
    expect(litCards()).toHaveLength(1);
  });

  it("never for a start that wasn't today", () => {
    changes.value = [given("2026-11-17T04:00:00Z")];
    render(<MemberAccess session={member("u1")} />);
    expect(litCards()).toEqual([]);
  });

  it("never on a failed read, and waits for both reads before it decides", () => {
    changes.value = undefined;
    changes.error = new Error("down");
    const page = render(<MemberAccess session={member("u1")} />);
    expect(litCards()).toEqual([]);
    // The feed answers on a retry, on the same visit: it was not decided without it.
    changes.value = [given("2026-11-19T04:00:00Z")];
    changes.error = null;
    page.rerender(<MemberAccess session={member("u1")} />);
    expect(litCards()).toHaveLength(1);
    page.unmount();
    // Another member, Zitadel not read yet: nothing lit, and nothing spent.
    view("held", { current: false, truncated: false });
    render(<MemberAccess session={member("u2")} />).unmount();
    expect(litCards()).toEqual([]);
    view();
    render(<MemberAccess session={member("u2")} />);
    expect(litCards()).toHaveLength(1);
  });

  it("shows nothing extra when this device can't remember, rather than lighting every visit", () => {
    vi.spyOn(window, "localStorage", "get").mockImplementation(() => {
      throw new Error("blocked");
    });
    render(<MemberAccess session={member("u1")} />);
    expect(litCards()).toEqual([]);
  });
});
