// @vitest-environment jsdom
import { TRUENAS_SCHEMA } from "@/lib/__fixtures__/truenas-schema";
import { render, screen, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { PersonStorage } from "@/components/people/PersonStorage";
import type { PersonTargetView } from "@/lib/queries/useMyStorage";

const state = vi.hoisted(() => ({
  views: { data: undefined as unknown, error: null as unknown, refetch: () => {} },
  advanced: false,
}));

vi.mock("@/lib/queries/useMyStorage", () => ({ usePersonTargets: () => state.views }));
vi.mock("@/lib/queries/useTargets", () => ({ useTargets: () => ({ data: [{ target: "truenas" }] }) }));
vi.mock("@/lib/ui-view", () => ({
  useIsAdvanced: () => state.advanced,
  useUiView: () => ({ view: "basic", setView: () => {} }),
}));
vi.mock("@/lib/queries/useNameResolver", () => ({
  useNameResolver: () => ({ resolveRole: (_p: string, k: string) => ({ value: { display_name: k === "lead" ? "Fabrication lead" : k } }) }),
}));
vi.mock("@/components/names/UserName", () => ({ UserName: ({ id }: { id: string }) => <a href={`/users/${id}`}>{id === "op-9" ? "Kanishk Singh" : id}</a> }));

const GB = 1024 ** 3;
const lead = { project_id: "fab", role_key: "lead" };
const renders = { share: "main", folder: "projects/renders", path: "/mnt/main/projects/renders", access: "change" as const, roles: [lead] };
const homePlace = {
  share: "main",
  folder: "homes/roman.reigns",
  path: "/mnt/main/homes/roman.reigns",
  access: "change" as const,
  home: true,
  usage: { used_bytes: 18.4 * GB, quota_bytes: 50 * GB },
};

function view(over: Partial<PersonTargetView> = {}): PersonTargetView {
  return {
    target: "truenas",
    entitled: true,
    kind: "storage",
    entitlement_schema: TRUENAS_SCHEMA,
    account: { username: "roman.reigns", bound_at: "2026-09-03T10:00:00Z" },
    roles: [lead],
    credential: { set: true, last_changed_at: "2026-10-14T10:00:00Z" },
    reachable: true,
    health: "online",
    checked_at: new Date().toISOString(),
    storage: { username: "roman.reigns", usable: true, needs_password: false, smb_enabled: true, places: [homePlace, renders], places_readable: true },
    home: {
      subject_id: "u1",
      username: "roman.reigns",
      gives_home: true,
      roles_limit: 20 * GB,
      roles_limit_from: lead,
      home: homePlace.path,
      used_bytes: 18.4 * GB,
      quota_bytes: 50 * GB,
      own: {
        allowance_id: "a1",
        bytes: 50 * GB,
        reason: "Thesis renders for the final review.",
        actor_id: "op-9",
        set_at: "2026-10-01T00:00:00Z",
        expires_at: "2026-11-28T00:00:00Z",
      },
    },
    holds: [],
    ...over,
  };
}

function show(v: PersonTargetView) {
  state.views = { data: [v], error: null, refetch: () => {} };
  render(<PersonStorage userId="u1" name="Roman" />);
}

/** The value cell of the row whose label is `label`. */
function rowOf(label: string) {
  const cell = screen.getAllByText(label).find((el) => el.className.includes("tablet:w-[170px]"));
  if (!cell?.parentElement) throw new Error(`no ${label} row`);
  return cell.parentElement;
}

beforeEach(() => {
  state.views = { data: undefined, error: null, refetch: () => {} };
  state.advanced = false;
});

describe("a person's network storage (A-10 Part 1)", () => {
  it("reads every row in the order a person gets to their files, each linked to its one write place", () => {
    show(view());
    expect(screen.getByText("Network storage")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "TrueNAS" })).toHaveAttribute("href", "/system/targets/truenas");
    expect(within(rowOf("Account")).getByText("roman.reigns")).toBeInTheDocument();
    expect(within(rowOf("Account")).getByText("Made by Syndra on 3 Sept")).toBeInTheDocument();
    expect(within(rowOf("Password")).getByText("Set, last changed 14 Oct")).toBeInTheDocument();
    // Nobody but the person sets it, so the row links nowhere.
    expect(within(rowOf("Password")).queryByRole("link")).toBeNull();
    const home = rowOf("Home folder");
    expect(within(home).getByText("18.4 GB of 50 GB")).toBeInTheDocument();
    expect(within(home).getByText(/Up to 50 GB, their own · ends 28 Nov/)).toBeInTheDocument();
    expect(within(home).getByText("In place of the 20 GB their Fabrication lead role gives.")).toBeInTheDocument();
    expect(within(home).getByRole("link", { name: /Set on TrueNAS's page/ })).toHaveAttribute(
      "href",
      "/system/targets/truenas?person=u1#accounts",
    );
    const folders = rowOf("Folders");
    expect(within(folders).getByText("renders")).toBeInTheDocument();
    expect(within(folders).getByText("from Fabrication lead")).toBeInTheDocument();
    expect(within(folders).queryByText("roman.reigns")).toBeNull(); // the home folder isn't repeated
    expect(within(folders).getByRole("link", { name: /Set in TrueNAS's mappings/ })).toHaveAttribute(
      "href",
      "/system/targets/truenas/mappings",
    );
    expect(within(rowOf("Hold")).getByText("None")).toBeInTheDocument();
    expect(screen.queryByRole("button")).toBeNull(); // read-only: no verbs here
  });

  it("an adopted account's password follows TrueNAS, never 'Not set'", () => {
    show(view({ adopted: { actor_id: "op-9", at: "2026-09-12T10:00:00Z" }, credential: { set: false } }));
    expect(within(rowOf("Account")).getByRole("link", { name: "Kanishk Singh" })).toHaveAttribute("href", "/users/op-9");
    expect(within(rowOf("Password")).getByText("Set before Syndra managed this account")).toBeInTheDocument();
    expect(screen.queryByText("Not set yet")).toBeNull();
  });

  it("needs a new password after access was given back", () => {
    show(
      view({
        restored_at: "2026-10-20T10:00:00Z",
        storage: { ...view().storage!, usable: false, smb_enabled: false, needs_new_password: true },
      }),
    );
    expect(within(rowOf("Password")).getByText("Needs a new password")).toBeInTheDocument();
    expect(within(rowOf("Hold")).getByText("The last one ended on 20 Oct")).toBeInTheDocument();
  });

  it("a whole-account hold reads in the Hold row and over the folders", () => {
    show(
      view({
        holds: [{ field: "enabled", value: "true", reason: "Lost their key card. Until it's found.", actor_id: "op-9", since: "2026-10-20T00:00:00Z", review_date: "2026-11-17T00:00:00Z" }],
      }),
    );
    expect(within(rowOf("Hold")).getByText("Their whole account, since 20 Oct")).toBeInTheDocument();
    expect(within(rowOf("Hold")).getByText(/review on 17 Nov/)).toBeInTheDocument();
    expect(within(rowOf("Folders")).getByText("None of these opens while the hold is on.")).toBeInTheDocument();
    expect(screen.getByText("Answering · their account is on hold")).toBeInTheDocument();
  });

  it("no account yet: the step's status, then what they'll get, not open yet", () => {
    show(view({ account: undefined, storage: undefined, home: { subject_id: "u1", username: "", gives_home: true, roles_limit: 20 * GB, roles_limit_from: lead }, preview: [renders] }));
    expect(screen.getByRole("link", { name: /Waiting for you to send/ })).toHaveAttribute("href", "/governance/pending?person=u1");
    expect(screen.getByText("Opens once their account is made")).toBeInTheDocument();
    expect(screen.getByText("Up to 20 GB, from their Fabrication lead role")).toBeInTheDocument();
    expect(screen.getByText("renders")).toBeInTheDocument();
  });

  it("no role reaches it: one row, and the card stays", () => {
    show(view({ entitled: false, account: undefined, storage: undefined, home: undefined, roles: [] }));
    expect(within(rowOf("Account")).getByText("None")).toBeInTheDocument();
    expect(screen.queryByText("Password")).toBeNull();
  });

  it("offline: one Notice, the last read with its time, Syndra's own rows as they are", () => {
    const readAt = new Date(Date.now() - 2 * 60 * 60 * 1000).toISOString();
    show(
      view({
        health: "offline",
        reachable: false,
        reachable_since: new Date(Date.now() - 90 * 60 * 1000).toISOString(),
        storage: undefined,
        last_known: { storage: view().storage!, read_at: readAt },
      }),
    );
    expect(screen.getByText("TrueNAS is offline")).toBeInTheDocument();
    expect(screen.getByText(/Home folder and Folders below are what it said at/)).toBeInTheDocument();
    expect(screen.getByText(/Offline since .* · last read/)).toBeInTheDocument();
    expect(within(rowOf("Folders")).getByText("renders")).toBeInTheDocument();
  });

  it("offline with no last read: the Notice alone, and no numbers", () => {
    show(view({ health: "offline", reachable: false, storage: undefined, last_known: undefined, home: { ...view().home!, used_bytes: undefined, quota_bytes: undefined } }));
    expect(screen.getByText("TrueNAS is offline")).toBeInTheDocument();
    expect(screen.queryByText("Home folder")).toBeNull();
    expect(screen.queryByText("Folders")).toBeNull();
    expect(screen.queryByText(/GB of/)).toBeNull();
  });

  it("one slow answer says so in place, with no Notice", () => {
    show(view({ storage: undefined }));
    expect(screen.queryByText("TrueNAS is offline")).toBeNull();
    expect(within(rowOf("Home folder")).getByText(/didn't answer when this page asked/)).toBeInTheDocument();
  });

  it("being set up: the whole body is not open yet, with the step", () => {
    show(view({ setup: "no_project", account: undefined, entitled: false, storage: undefined }));
    expect(screen.getByText("Opens once TrueNAS is set up")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /Finish setting it up/ })).toHaveAttribute("href", "/system/targets/truenas#project");
  });

  it("nearly full turns the line amber; at the limit it says what still works", () => {
    show(view({ home: { ...view().home!, used_bytes: 46 * GB } }));
    expect(screen.getByText("· nearly full")).toBeInTheDocument();
  });

  it("loading shows the name and caption with no values; an error replaces the card", () => {
    render(<PersonStorage userId="u1" name="Roman" />);
    expect(screen.getByRole("link", { name: "TrueNAS" })).toBeInTheDocument();
    expect(screen.queryByText("roman.reigns")).toBeNull();
    state.views = { data: undefined, error: new Error("down"), refetch: () => {} };
    render(<PersonStorage userId="u1" name="Roman" />);
    expect(screen.getByText("Couldn't load Roman's storage on TrueNAS.")).toBeInTheDocument();
  });

  it("lists folders A to Z, and says the write place once per row, not in a caption too", () => {
    const folder = (name: string) => ({ ...renders, folder: `projects/${name}`, path: `/mnt/main/projects/${name}` });
    const base = view();
    show(view({ storage: { ...base.storage!, places: [folder("zine"), homePlace, folder("assets")] } }));
    const names = within(rowOf("Folders")).getAllByText(/^(zine|assets)$/).map((el) => el.textContent);
    expect(names).toEqual(["assets", "zine"]);
    expect(screen.queryByText(/Limits, holds and folders are set on/)).toBeNull();
  });
});
