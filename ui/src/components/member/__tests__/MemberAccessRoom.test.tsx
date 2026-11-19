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

beforeEach(() => {
  window.localStorage.clear();
  inZone("Asia/Kolkata");
  vi.useFakeTimers({ toFake: ["Date"] });
  vi.setSystemTime(new Date("2026-11-19T10:00:00Z")); // 15:30 in Kolkata: the afternoon
  view();
});
afterEach(() => {
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

  it("is never lit over a page that could not read Zitadel", () => {
    // Zitadel never read: the page's Notice and no rows.
    view({ read_at: null, current: false, truncated: false });
    mount(true);
    expect(light()).toBeNull();
    // The read itself failed: the page is its error.
    view(null, new Error("unreachable"));
    mount(true);
    expect(light()).toBeNull();
    // Still loading is not a failure: the room is lit while the read is out.
    view(null);
    mount(true);
    expect(light()).not.toBeNull();
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

  it("never replays the door when a read lands unread and then read (the light is unmounted between)", () => {
    // Loading, with the server's door: the room is lit and the door crosses.
    view(null);
    const page = mount(true);
    expect(light()).toHaveAttribute("data-door", "");
    spendDoor(SIGNED_IN); // as the shell does on the sign-in's first page
    // The read lands unread: the light goes.
    view({ read_at: null, current: false, truncated: false });
    page.reread();
    expect(light()).toBeNull();
    // Minutes later a refetch lands read: the pool returns, still. The door was this sign-in's first moment.
    view();
    page.reread();
    expect(light()).not.toBeNull();
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
