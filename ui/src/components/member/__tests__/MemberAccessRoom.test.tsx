// @vitest-environment jsdom
import type { SessionUser } from "@/lib/session";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render } from "@testing-library/react";
import { readFileSync, readdirSync, statSync } from "node:fs";
import { join, resolve } from "node:path";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { MemberAccess } from "@/components/member/MemberAccess";
import { AppShell } from "@/components/shell/AppShell";
import { RoomContext } from "@/components/shell/RoomLight";
import { DOOR_COOKIE, spendDoor } from "@/lib/motion/door";
import { DOOR_OPEN_MS } from "@/lib/motion";
import { inZone } from "@/test-utils/zone";

// A-25 moment 1: the member's room. My access gets Home's light, by Home's own path: the hour's pool
// in the shell's column and, once after a sign-in, the door's crossing; never over a page that could
// not read Zitadel.

const access = vi.hoisted(() => ({ value: {} as Record<string, unknown> }));

vi.mock("@/components/member/MemberCatalog", () => ({ MemberCatalog: () => null }));
vi.mock("@/components/requests/AskForAccess", () => ({ AskForAccess: () => null }));
vi.mock("@/components/names", () => ({ ResolvedProjectName: ({ name }: { name: string }) => <span>{name}</span> }));
vi.mock("@/lib/queries/useUsers", () => ({ useUserAccess: () => access.value, useUserGrants: () => ({ data: [], error: null }) }));
vi.mock("@/lib/queries/useMyStorage", () => ({ useMyStorage: () => ({ data: [] }) }));
vi.mock("@/lib/queries/useRequests", () => ({ useRequestsMine: () => ({ data: [] }) }));
vi.mock("@/lib/queries/useMyChanges", () => ({ useMyChanges: () => ({ data: [], error: null, isLoading: false, isFetchedAfterMount: true }) }));
vi.mock("@/lib/queries/useCatalogUsers", () => ({ useCatalogProjects: () => ({ data: [] }) }));
vi.mock("@/lib/queries/useFieldSchema", async () => (await import("@/lib/__fixtures__/field-schema-mock")).fieldSchemaMock);
// The real AppShell, its own parts stood in for (as AppShell.test.tsx does): it spends the door.
vi.mock("@/components/shell/Sidebar", () => ({ default: () => null }));
vi.mock("@/components/shell/TopBar", () => ({ TopBar: () => <header>top</header> }));
vi.mock("@/components/shell/PlaceHost", () => ({ PlaceHost: () => null }));
vi.mock("@/components/shell/IntentPrefetchHost", () => ({ IntentPrefetchHost: () => null }));
vi.mock("@/components/shell/TouchNav", () => ({ TouchNav: () => null }));
vi.mock("@/components/shell/GoToHost", () => ({ GoToHost: () => null }));

const SIGNED_IN = "2026-11-19T08:00:00.000Z";
const member: SessionUser = {
  id: "u1", name: "Cody Rhodes", email: "cody@example.org", title: "", team: "", status: "active", avatar: "", role: "user",
  sessionType: "oidc", signedInAt: SIGNED_IN,
};
const READ = { read_at: "2026-11-19T06:00:00Z", current: true, truncated: false };
function view(observation: Record<string, unknown> | null = READ, error: Error | null = null) {
  access.value = {
    isLoading: observation === null && !error,
    isFetchedAfterMount: true,
    error,
    data: observation === null ? undefined : { projects: [], bundles: [], allowances: [], observation },
  };
}

/** The shell as AppShell has it: the page renders in the scroller, the light in the column. */
function mount(door = false) {
  document.querySelector("[data-test-column]")?.remove();
  const column = document.createElement("div");
  column.setAttribute("data-test-column", "");
  const scroll = document.createElement("div");
  scroll.id = "app-scroll";
  column.append(scroll);
  document.body.append(column);
  const page = () => (
    <RoomContext.Provider value={column}>
      <MemberAccess session={member} door={door} />
    </RoomContext.Provider>
  );
  const view = render(page(), { container: scroll });
  return { column, unmount: view.unmount, reread: () => view.rerender(page()) };
}
const light = () => document.querySelector(".hour-light");
/** Whether a light, or a door, was ever in the document while the test ran: nothing drawn and taken away unseen. */
function everDrawn() {
  const seen = { light: false, door: false };
  const look = (node: Node) => {
    if (!(node instanceof Element)) return;
    const lights = [node, ...node.querySelectorAll(".hour-light")].filter((el) => el.matches(".hour-light"));
    seen.light ||= lights.length > 0;
    seen.door ||= lights.some((el) => el.hasAttribute("data-door"));
  };
  const watcher = new MutationObserver(() => {});
  watcher.observe(document.body, { subtree: true, childList: true, attributes: true, attributeFilter: ["data-door"] });
  return () => {
    // Every node added, and every node whose door changed, since the watch began: even one gone again.
    for (const record of watcher.takeRecords()) {
      record.addedNodes.forEach(look);
      look(record.target);
    }
    look(document.body);
    watcher.disconnect();
    return seen;
  };
}

beforeEach(() => {
  window.localStorage.clear();
  inZone("Asia/Kolkata");
  vi.useFakeTimers({ toFake: ["Date"] });
  vi.setSystemTime(new Date("2026-11-19T10:00:00Z")); // 15:30 in Kolkata: the afternoon
  view();
});
afterEach(() => {
  vi.restoreAllMocks();
  vi.useRealTimers();
  inZone(process.env.TZ);
});

describe("the member's room (A-25 moment 1)", () => {
  it("is lit like Home: the hour's pool in the shell's column, and the door's crossing after a sign-in", () => {
    const { column } = mount(true);
    expect(light()).toHaveAttribute("data-hour", "afternoon");
    expect(light()).toHaveAttribute("data-door", "");
    expect(light()).toHaveAttribute("aria-hidden");
    expect(light()!.parentElement).toBe(column);
    expect(light()!.closest("#app-scroll")).toBeNull();
  });

  it("is the pool at rest without the server's door", () => {
    mount();
    expect(light()).toHaveAttribute("data-hour", "afternoon");
    expect(light()).not.toHaveAttribute("data-door");
  });

  it("is never lit over a page that could not read Zitadel, and never while the read is out", () => {
    // Zitadel never read: the page's Notice and no rows.
    view({ read_at: null, current: false, truncated: false });
    mount(true);
    expect(light()).toBeNull();
    // The read itself failed: the page is its error.
    view(null, new Error("unreachable"));
    mount(true);
    expect(light()).toBeNull();
    // Still loading: nothing yet, so nothing can be cut off when it lands.
    view(null);
    mount(true);
    expect(light()).toBeNull();
  });

  it("crosses once a sign-in: spent by the first page, so My access again stays still", () => {
    const first = mount(true);
    expect(light()).toHaveAttribute("data-door", "");
    spendDoor(SIGNED_IN);
    first.unmount();
    // A cached render that still says "door" (Back) finds it already opened on this device.
    mount(true);
    expect(light()).not.toBeNull();
    expect(light()).not.toHaveAttribute("data-door");
  });

  it("loading, then unread: never lit, and no door ever drawn", () => {
    const drawn = everDrawn();
    view(null);
    const page = mount(true);
    view({ read_at: null, current: false, truncated: false });
    page.reread();
    expect(light()).toBeNull();
    expect(drawn()).toEqual({ light: false, door: false });
  });

  it("loading, then read: the door crosses once when the read lands in time, and the pool settles in after that", () => {
    view(null);
    const page = mount(true);
    expect(light()).toBeNull();
    view();
    page.reread();
    expect(light()).toHaveAttribute("data-door", "");
    spendDoor(SIGNED_IN); // as the shell does on the sign-in's first page
    // A refetch lands unread, then read: the pool goes, then settles back in; the door never crosses again.
    view({ read_at: null, current: false, truncated: false });
    page.reread();
    expect(light()).toBeNull();
    view();
    page.reread();
    expect(light()).toHaveAttribute("data-settle", "");
    expect(light()).not.toHaveAttribute("data-door");
  });

  it("settles in without crossing when the read takes longer than the crossing itself (--door-open)", () => {
    let clock = 1000;
    vi.spyOn(performance, "now").mockImplementation(() => clock);
    view(null);
    const page = mount(true);
    clock += DOOR_OPEN_MS + 1;
    view();
    page.reread();
    expect(light()).toHaveAttribute("data-settle", "");
    expect(light()).not.toHaveAttribute("data-door");
  });

  it("on a revisit, the still pool only: settled in after the read, or there at once from a cached one", () => {
    spendDoor(SIGNED_IN);
    view(null);
    const page = mount(true);
    view();
    page.reread();
    expect(light()).toHaveAttribute("data-settle", "");
    expect(light()).not.toHaveAttribute("data-door");
    page.unmount();
    // A read already in hand at the first render: the pool is simply there, as on Home.
    mount(true);
    expect(light()).not.toBeNull();
    expect(light()).not.toHaveAttribute("data-settle");
    expect(light()).not.toHaveAttribute("data-door");
  });

  it("crosses on the first visit inside the real shell, which spends the door after the page decides it", () => {
    document.cookie = `${DOOR_COOKIE}=${SIGNED_IN}; Path=/`;
    const shell = () => (
      <QueryClientProvider client={new QueryClient()}>
        <AppShell session={member}>
          <MemberAccess session={member} door />
        </AppShell>
      </QueryClientProvider>
    );
    const first = render(shell());
    expect(light()).toHaveAttribute("data-door", "");
    expect(document.cookie).not.toContain(DOOR_COOKIE);
    first.unmount();
    // The second visit (Back to a page whose render still says door) is the pool at rest.
    render(shell());
    expect(light()).not.toBeNull();
    expect(light()).not.toHaveAttribute("data-door");
  });

  it("takes Home's path, not a second one: the light is drawn in one place", () => {
    const SRC = resolve(__dirname, "../../..");
    const files: string[] = [];
    const walk = (dir: string) => {
      for (const name of readdirSync(dir)) {
        const path = join(dir, name);
        if (statSync(path).isDirectory()) {
          if (name !== "__tests__" && name !== "node_modules") walk(path);
        } else if (/\.tsx?$/.test(name)) files.push(path);
      }
    };
    walk(SRC);
    const drawing = files.filter((path) => /["'`]hour-light\b/.test(readFileSync(path, "utf8")));
    expect(drawing.map((path) => path.slice(SRC.length + 1))).toEqual(["components/shell/RoomLight.tsx"]);
  });
});
