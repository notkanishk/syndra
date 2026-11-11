// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, fireEvent, render, screen, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { TRUENAS_SCHEMA } from "@/lib/__fixtures__/truenas-schema";
import { StoragePage as MySystems } from "@/components/kinds/storage/StoragePage";
import { deviceFrom } from "@/components/kinds/storage/device";
import type { CredentialAnswer, MyTargetView, StoragePlace } from "@/lib/queries/useMyStorage";
import type { OneShotSecret } from "@/lib/secret";
import type { RetryState } from "@/lib/retry";

// The member's storage page, state by state (route-storage; A-01, A-02).
// What each state may and may not offer is the point: a password form where a
// password can't land, an address that won't answer, or a zero for usage
// nobody read are each a page that lies.

const state = {
  targets: [] as MyTargetView[],
  sent: [] as OneShotSecret[],
  // The mutation's state, as the form reads it (A-13 §4).
  credential: { isPending: false, data: undefined as CredentialAnswer | undefined, error: null as Error | null },
  // What the form hears while the page waits to send it again (the one retry model).
  onRetry: undefined as undefined | ((retry: RetryState) => void),
};

vi.mock("@/lib/queries/useMyStorage", async () => {
  const actual = await vi.importActual<typeof import("@/lib/queries/useMyStorage")>(
    "@/lib/queries/useMyStorage",
  );
  return {
    ...actual,
    useMyStorage: () => ({ data: state.targets, isLoading: false, error: null, refetch: vi.fn() }),
    useSetStorageCredential: (_target: string, onRetry?: (retry: RetryState) => void) => {
      state.onRetry = onRetry;
      return {
        mutate: (secret: OneShotSecret) => state.sent.push(secret),
        ...state.credential,
      };
    },
  };
});

vi.mock("@/lib/queries/useCatalogUsers", () => ({
  useCatalogProjects: () => ({
    data: [{ id: "p1", name: "Fabrication", kind: "", description: "", roles: [{ key: "fab_lead", label: "Fabrication lead", description: "" }] }],
    isLoading: false,
    error: null,
    refetch: vi.fn(),
  }),
}));

const HOST = "nas.example.edu";

function place(over: Partial<StoragePlace> & { path: string }): StoragePlace {
  const [share, ...rest] = over.path.split("/");
  const folder = rest.join("/");
  return {
    share,
    folder: folder || undefined,
    access: "change",
    smb_address: `smb://${HOST}/${over.path}`,
    unc_address: `\\\\${HOST}\\${over.path.split("/").join("\\")}`,
    ...over,
    path: `/mnt/p/${over.path}`,
  };
}

const HOME = place({ path: "main/home_dir/charlotte", home: true, usage: { used_bytes: 0, quota_bytes: 50 * 2 ** 30 } });
const WADDLE = place({ path: "main/waddle_share", usage: { used_bytes: 46 * 2 ** 30, quota_bytes: 50 * 2 ** 30 }, roles: [{ project_id: "p1", role_key: "fab_lead" }] });
// Usage on a read-only place is drawn nowhere, even if one arrived.
const QUACK = place({ path: "main/quack_share", access: "read", usage: { used_bytes: 5 * 2 ** 30 }, roles: [{ project_id: "p1", role_key: "fab_lead" }] });

function view(over: Partial<MyTargetView> = {}): MyTargetView {
  return {
    target: "truenas",
    name: "TrueNAS",
    kind: "storage",
    entitlement_schema: TRUENAS_SCHEMA,
    entitled: true,
    account: { username: "charlotte" },
    credential: { set: true, last_changed_at: "2026-09-20T10:00:00Z" },
    reachable: true,
    health: "online",
    checked_at: new Date().toISOString(),
    lifecycle: "active",
    connection: { protocol: "smb", host: HOST },
    roles: [{ project_id: "p1", role_key: "fab_lead" }],
    storage: {
      usable: true,
      needs_password: false,
      places: [HOME, WADDLE, QUACK],
      places_readable: true,
    },
    ...over,
  };
}

function renderStorage() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <MySystems />
    </QueryClientProvider>,
  );
}

const passwordField = () => document.getElementById("storage-password-truenas");

beforeEach(() => {
  state.sent = [];
  state.credential = { isPending: false, data: undefined, error: null };
});

describe("the three states", () => {
  it("no role: nothing to set and nowhere to connect, and the way to a role", () => {
    state.targets = [view({ entitled: false, account: undefined, storage: undefined, roles: [] })];
    renderStorage();
    expect(screen.getByText("None of your roles reaches TrueNAS")).toBeTruthy();
    expect(screen.getByRole("link", { name: /Ask for access in Requests/ }).getAttribute("href")).toBe("/requests?ask");
    expect(passwordField()).toBeNull();
    expect(screen.queryByText("Sign in as")).toBeNull();
  });

  it("on its way: an age and who it waits on, never a form", () => {
    state.targets = [view({ account: undefined, storage: undefined, recorded_at: "2026-10-12T09:00:00Z" })];
    renderStorage();
    expect(screen.getByText("Your account is on its way")).toBeTruthy();
    expect(screen.getByText(/it waits on makerspace staff/)).toBeTruthy();
    expect(passwordField()).toBeNull();
  });

  it("set a password: the form leads, and the folders are shown but locked", () => {
    state.targets = [
      view({
        credential: { set: false },
        storage: { ...view().storage!, usable: false, needs_password: true },
      }),
    ];
    renderStorage();
    expect(screen.getByText("Set a password to switch your account on")).toBeTruthy();
    expect(screen.getByRole("button", { name: "Set password" })).toBeTruthy();
    expect(screen.getByText("Set your password to open these")).toBeTruthy();
    expect(screen.getByRole("link", { name: "Set it above ↑" }).getAttribute("href")).toBe("#storage-password-truenas");
    const preview = screen.getByText("Your home folder").closest("[aria-disabled]");
    expect(preview?.getAttribute("aria-disabled")).toBe("true");
    expect(preview?.hasAttribute("inert")).toBe(true);
  });

  it("ready: each folder by its name, none pinned first, with what you can do there", () => {
    state.targets = [view()];
    renderStorage();
    expect(screen.getByText("Your account is ready")).toBeTruthy();
    expect(screen.getByText(/Answering and accepting sign-ins/)).toBeTruthy();
    const names = screen.getAllByText(/^(Your home folder|waddle_share|quack_share)$/).map((n) => n.textContent);
    expect(names).toEqual(["quack_share", "waddle_share", "Your home folder"]);
    expect(screen.getByText("In main › home_dir · private to you")).toBeTruthy();
    expect(screen.getAllByText("In main · from your Fabrication lead role")).toHaveLength(2);
    expect(screen.getAllByText("Read only")).toHaveLength(1);
    expect(screen.getByRole("button", { name: "Change password" })).toBeTruthy();
  });
});

describe("usage", () => {
  it("says nothing saved, nearly full from 90%, and nothing at all where you can't save", () => {
    state.targets = [view()];
    renderStorage();
    expect(screen.getByText("Nothing saved here yet")).toBeTruthy();
    expect(screen.getByText(/46\.0 GiB of 50\.0 GiB · nearly full/)).toBeTruthy();
    const quack = screen.getByText("quack_share").closest(".row-divider") as HTMLElement;
    expect(within(quack).queryByText(/GiB|Nothing saved/)).toBeNull();
  });

  it("says no limit set rather than drawing a full bar", () => {
    state.targets = [
      view({ storage: { ...view().storage!, places: [place({ path: "main/waddle_share", usage: { used_bytes: 2 ** 30 } })] } }),
    ];
    renderStorage();
    expect(screen.getByText("1.0 GiB used · no limit set")).toBeTruthy();
  });
});

describe("usage the target could not read", () => {
  it("says so, in a member's words, rather than leaving usage lines out as if nothing were used", () => {
    state.targets = [view({ storage: { ...view().storage!, quota_read: false } })];
    renderStorage();
    expect(screen.getByText("Some usage could not be read from TrueNAS.")).toBeTruthy();
    expect(screen.getByText(/ask makerspace staff/)).toBeTruthy();
    expect(screen.queryByText(/check the add-on's health/)).toBeNull();
  });

  it("says all of it could not be read when no place carries usage", () => {
    const bare = [HOME, WADDLE].map(({ usage: _u, ...p }) => p);
    state.targets = [view({ storage: { ...view().storage!, places: bare, quota_read: false } })];
    renderStorage();
    expect(screen.getByText("Usage could not be read from TrueNAS.")).toBeTruthy();
  });

  it("says all of it could not be read when only a read-only place carries usage, which is drawn nowhere", () => {
    const bare = [HOME, WADDLE].map(({ usage: _u, ...p }) => p);
    state.targets = [view({ storage: { ...view().storage!, places: [...bare, QUACK], quota_read: false } })];
    renderStorage();
    expect(screen.getByText("Usage could not be read from TrueNAS.")).toBeTruthy();
    expect(screen.queryByText(/Some usage could not be read/)).toBeNull();
  });

  it("says all of it could not be read while offline, when no usage line is drawn", () => {
    state.targets = [view({ health: "offline", storage: { ...view().storage!, quota_read: false } })];
    renderStorage();
    expect(screen.getByText("Usage could not be read from TrueNAS.")).toBeTruthy();
  });

  it("says nothing when it was read (quota_read true) or the target does not say", () => {
    for (const quota_read of [true, undefined]) {
      state.targets = [view({ storage: { ...view().storage!, quota_read } })];
      const { unmount } = renderStorage();
      expect(screen.queryByText(/usage could not be read/i)).toBeNull();
      unmount();
    }
  });
});

describe("addresses", () => {
  it("are the backend's, both of them, per folder", () => {
    state.targets = [view()];
    renderStorage();
    expect(screen.getByText(`smb://${HOST}/main/waddle_share`)).toBeTruthy();
    expect(screen.getByText(`\\\\${HOST}\\main\\waddle_share`)).toBeTruthy();
  });

  it("are left out, with the guide, when no host is registered", () => {
    state.targets = [view({ connection: undefined })];
    renderStorage();
    expect(screen.getByText(/doesn't have the address to connect to yet, so ask makerspace staff/)).toBeTruthy();
    expect(screen.queryByText(/^smb:\/\//)).toBeNull();
    expect(screen.queryByRole("button", { name: /How to connect/ })).toBeNull();
  });
});

describe("Q-02 states", () => {
  it("access given back: a new password, the date, and why the old one is gone", () => {
    state.targets = [
      view({
        restored_at: "2026-10-12T10:00:00Z",
        storage: { ...view().storage!, usable: false, needs_new_password: true },
      }),
    ];
    renderStorage();
    expect(screen.getByText("Set a new password to open your folders again")).toBeTruthy();
    expect(screen.getByText(/given back on 12 Oct/)).toBeTruthy();
    expect(screen.getByRole("button", { name: "Set new password" })).toBeTruthy();
    expect(screen.getByText("Set a new password to open these")).toBeTruthy();
  });

  it("a role that opens nothing: says so, names the role, offers no guide", () => {
    state.targets = [view({ storage: { ...view().storage!, places: [] } })];
    renderStorage();
    expect(screen.getByText("Your account is ready, but no folder is open to you yet")).toBeTruthy();
    expect(screen.getByText(/Your Fabrication lead role reaches TrueNAS, but no folder there is open to it/)).toBeTruthy();
    expect(screen.queryByRole("button", { name: /How to connect/ })).toBeNull();
  });

  it("a home no role gives any more stays, and says so", () => {
    state.targets = [view({ home_kept: true })];
    renderStorage();
    expect(screen.getByText(/It stays, with your files in it, and only you can open it/)).toBeTruthy();
  });

  it("an account the target never answered for is unread, not ready", () => {
    state.targets = [view({ storage: undefined, health: "unknown" })];
    renderStorage();
    expect(screen.getByText("Couldn't check your account just now")).toBeTruthy();
    expect(screen.queryByText("Your account is ready")).toBeNull();
    expect(screen.queryByText(/accepting sign-ins/)).toBeNull();
    expect(screen.getByText(/couldn't say which folders are open to you just now/)).toBeTruthy();
  });

  it("folders that couldn't be read are said to be unread, never empty", () => {
    state.targets = [view({ storage: { ...view().storage!, places: [], places_readable: false } })];
    renderStorage();
    expect(screen.getByText(/couldn't say which folders are open to you just now/)).toBeTruthy();
    expect(screen.queryByText(/no folder is open to you yet/)).toBeNull();
  });
});

describe("when changes can't happen", () => {
  it("offline: the Notice comes first, usage is unavailable, and there is no form or guide", () => {
    state.targets = [view({ health: "offline", reachable: false, reachable_since: "2026-10-14T09:40:00Z" })];
    renderStorage();
    expect(screen.getByRole("alert").textContent).toContain("TrueNAS is offline");
    expect(screen.getByText(/^Offline since/)).toBeTruthy();
    expect(screen.getAllByText("Usage unavailable while offline").length).toBeGreaterThan(0);
    expect(screen.queryByText(/Nothing saved here yet|GiB/)).toBeNull();
    expect(passwordField()).toBeNull();
    expect(screen.getByText(/can't change your password while TrueNAS is offline/)).toBeTruthy();
    expect(screen.queryByRole("button", { name: /How to connect/ })).toBeNull();
  });

  it("can't tell yet: no status at all rather than a guess", () => {
    state.targets = [view({ health: "unknown" })];
    renderStorage();
    expect(screen.queryByText(/^Online|^Answering|^Offline/)).toBeNull();
  });

  it("paused: access first, then the pause, and no form, first password included", () => {
    for (const lifecycle of ["draining", "read_only"] as const) {
      state.targets = [
        view({ lifecycle, credential: { set: false }, storage: { ...view().storage!, needs_password: true, usable: false } }),
      ];
      const { unmount, container } = renderStorage();
      expect(passwordField()).toBeNull();
      expect(screen.getAllByText(/can't change your password while account changes are paused/).length).toBeGreaterThan(0);
      expect(container.textContent).not.toMatch(/maintenance|draining|read-only|read_only/i);
      expect(container.textContent).not.toMatch(/apply it|when changes start again/i);
      // A drain earns no person; an open-ended pause gets one instead of an estimate.
      expect(Boolean(screen.queryByText(/Need it today\? Ask makerspace staff/))).toBe(lifecycle === "read_only");
      // Final verification: neither pause ends, nor resumes changes, by itself
      // (an operator sets the add-on back to active; a change it declined
      // waits in Pending changes), and neither has a known length.
      expect(container.textContent).not.toMatch(/by itself|on its own|few minutes/i);
      unmount();
    }
  });

  it("a drain says who ends the pause, and that held-back changes wait until then", () => {
    state.targets = [view({ lifecycle: "draining" })];
    renderStorage();
    expect(screen.getAllByText(/Makerspace staff end the pause; account changes held back wait until then\./).length).toBeGreaterThan(0);
  });

  it("an account on its way while paused waits on staff, said inside the state", () => {
    state.targets = [view({ account: undefined, storage: undefined, lifecycle: "read_only" })];
    renderStorage();
    expect(screen.getByText("Your account is waiting on makerspace staff, not on you")).toBeTruthy();
  });

  it("held: switched off for now, since when, no addresses, and the password row says why", () => {
    state.targets = [
      view({
        entitled: false,
        suspended: [{ field: "enabled", value: "true", reason: "safety review", actor_id: "op", since: "2026-10-09T10:00:00Z" }],
      }),
    ];
    renderStorage();
    expect(screen.getByText("Your account is switched off for now")).toBeTruthy();
    expect(screen.getByText("On hold since 9 Oct")).toBeTruthy();
    expect(screen.getByText(/Answering · your account is on hold/)).toBeTruthy();
    expect(passwordField()).toBeNull();
  });

  it("held: found by the schema's account role, whatever the add-on calls the field", () => {
    const schema = TRUENAS_SCHEMA.map((f) => (f.role === "account" ? { ...f, name: "active" } : f));
    state.targets = [
      view({
        entitled: false,
        entitlement_schema: schema,
        suspended: [{ field: "active", value: "true", reason: "safety review", actor_id: "op", since: "2026-10-09T10:00:00Z" }],
      }),
    ];
    renderStorage();
    expect(screen.getByText("Your account is switched off for now")).toBeTruthy();
    expect(screen.getByText("On hold since 9 Oct")).toBeTruthy();
  });

  it("a field named enabled that is not the account is a held-back change, not a hold", () => {
    const schema = TRUENAS_SCHEMA.map((f) => (f.role === "account" ? { ...f, name: "active" } : f));
    state.targets = [
      view({
        entitled: false,
        entitlement_schema: schema,
        suspended: [{ field: "enabled", value: "true", reason: "safety review", actor_id: "op" }],
      }),
    ];
    renderStorage();
    expect(screen.queryByText("Your account is switched off for now")).toBeNull();
  });

  it("a held-back folder is named with its reason, never dropped", () => {
    state.targets = [view({ suspended: [{ field: "group", value: "lab_archive", reason: "moving pools", actor_id: "op" }] })];
    renderStorage();
    expect(screen.getByText(/lab_archive/)).toBeTruthy();
    expect(screen.getByText(/moving pools/)).toBeTruthy();
  });
});

describe("the connection guide", () => {
  it("opens with the device's own steps and the chosen folder's address", () => {
    state.targets = [view()];
    renderStorage();
    fireEvent.click(screen.getAllByRole("button", { name: "First time? How to connect" })[0]);
    const dialog = screen.getByRole("dialog");
    expect(within(dialog).getByText("Connect to TrueNAS, step by step")).toBeTruthy();
    fireEvent.change(within(dialog).getByLabelText("Folder"), { target: { value: "2" } });
    fireEvent.click(within(dialog).getByRole("radio", { name: "Android" }));
    expect(within(dialog).getByText(/Android has no built-in way/)).toBeTruthy();
    expect(within(dialog).getByText(/share: main\. Then open home_dir › charlotte inside it/)).toBeTruthy();
  });

  it("picks the device from the browser, Mac when it can't tell", () => {
    expect(deviceFrom("Mozilla/5.0 (iPhone; CPU iPhone OS 18_0)")).toBe("ios");
    expect(deviceFrom("Mozilla/5.0 (Windows NT 10.0; Win64)")).toBe("windows");
    expect(deviceFrom("Mozilla/5.0 (Linux; Android 15)")).toBe("android");
    expect(deviceFrom("Mozilla/5.0 (X11; Linux x86_64)")).toBe("linux");
    expect(deviceFrom("something else")).toBe("mac");
  });
});

describe("what the page hands to the mutation", () => {
  it("sends the password in a box that empties on read", () => {
    state.targets = [view({ credential: { set: false }, storage: { ...view().storage!, needs_password: true, usable: false } })];
    renderStorage();
    fireEvent.change(passwordField()!, { target: { value: "correct horse" } });
    fireEvent.click(screen.getByRole("button", { name: "Set password" }));
    expect(state.sent).toHaveLength(1);
    expect(state.sent[0].take()).toBe("correct horse");
    expect(state.sent[0].spent).toBe(true);
  });
});

// A-03: a limit set below what someone holds. TrueNAS refuses new writes and
// keeps every file, and the page says so in the member's terms.
describe("a folder over its limit", () => {
  it("says it's full, and that opening and deleting still work", () => {
    state.targets = [
      view({ storage: { ...view().storage!, places: [place({ path: "main/home_dir/charlotte", home: true, usage: { used_bytes: 34 * 2 ** 30, quota_bytes: 20 * 2 ** 30 } })] } }),
    ];
    renderStorage();
    expect(screen.getByText(/^Full\. You can open and delete files here, but not save new ones until you're under 20\.0 GiB\.$/)).toBeTruthy();
  });
});

// A-13 §4: the change shows as in progress until the add-on confirms it, and
// only then "Changed". Lost twice, the form stays open with what was typed; on
// return one line stays until the next confirmed change.
describe("a password change nobody has confirmed", () => {
  const ready = () => view({ credential: { set: true, last_changed_at: "2026-09-20T10:00:00Z" } });

  it("never says Changed while the sends are in progress", () => {
    vi.useFakeTimers();
    try {
      state.targets = [ready()];
      state.credential = { isPending: true, data: undefined, error: null };
      renderStorage();
      expect(screen.queryByText("Changed")).toBeNull();
      expect(screen.getByRole("button", { name: "Change password" }).getAttribute("aria-busy")).toBe("true");
      // At 4 seconds with no answer, one line says to keep the page open.
      expect(screen.queryByText(/Keep this page open/)).toBeNull();
      act(() => vi.advanceTimersByTime(4000));
      expect(screen.getByText("Still waiting for TrueNAS. Keep this page open until it answers.")).toBeTruthy();
    } finally {
      vi.useRealTimers();
    }
  });

  it("says Changed only on a confirmed set", () => {
    state.targets = [ready()];
    state.credential = { isPending: false, data: { status: "set" }, error: null };
    renderStorage();
    expect(screen.getByText("Changed")).toBeTruthy();
    expect(screen.getByText("TrueNAS has your new password.")).toBeTruthy();
    expect(screen.getByText(/^Last changed just now\./)).toBeTruthy();
  });

  it("unanswered twice: the amber line, the form open, no Changed and no stale last-changed", () => {
    state.targets = [ready()];
    state.credential = { isPending: false, data: { status: "indeterminate" }, error: null };
    renderStorage();
    fireEvent.change(passwordField()!, { target: { value: "correct horse" } });
    expect(screen.getByText("Not confirmed yet")).toBeTruthy();
    expect(screen.getByText("TrueNAS didn't answer, so your password may not have changed.")).toBeTruthy();
    expect(screen.queryByText("Changed")).toBeNull();
    expect(screen.queryByText(/Last changed/)).toBeNull();
    // One press sends what is still typed.
    fireEvent.click(screen.getByRole("button", { name: "Change password" }));
    expect(state.sent).toHaveLength(1);
    expect(state.sent[0].take()).toBe("correct horse");
  });

  // Vocal (owner, 5 Nov 2026): while the page waits to send it again, the
  // person is told when, in IST, and never told it is done.
  it("while Syndra is trying again: says when, never done", () => {
    state.targets = [ready()];
    state.credential = { isPending: true, data: undefined, error: null };
    renderStorage();
    act(() => state.onRetry?.({ state: "retrying", try: 2, of: 2, next_try: "2099-11-05T08:35:00Z" }));
    expect(screen.getByText("Not confirmed yet. Syndra is trying again at 14:05. Keep this page open.")).toBeTruthy();
    expect(screen.queryByText("Changed")).toBeNull();
  });

  it("after the one resend went unanswered too: says so in the shared words", () => {
    state.targets = [ready()];
    state.credential = {
      isPending: false,
      data: { status: "indeterminate", retry: { state: "waiting", try: 2, of: 2 } },
      error: null,
    };
    renderStorage();
    expect(
      screen.getByText(
        "TrueNAS didn't answer after 2 tries, so Syndra won't send it again on its own. Your password may not have changed.",
      ),
    ).toBeTruthy();
  });

  it("a clear refusal is red and says nothing changed", () => {
    state.targets = [ready()];
    state.credential = { isPending: false, data: { status: "rejected" }, error: null };
    renderStorage();
    expect(screen.getByText("Not changed")).toBeTruthy();
    expect(screen.getByText("TrueNAS refused the change. Nothing was changed.")).toBeTruthy();
    expect(screen.queryByText("Changed")).toBeNull();
  });

  it("a thrown error is Not changed, in the one inline result, never Changed", () => {
    state.targets = [ready()];
    state.credential = { isPending: false, data: undefined, error: new Error("offline") };
    renderStorage();
    expect(screen.getByText("Not changed")).toBeTruthy();
    expect(screen.queryByText("Changed")).toBeNull();
    expect(screen.queryByText("Not confirmed yet")).toBeNull();
  });

  it("back later: one line above an open form, until the next confirmed change", () => {
    state.targets = [view({ ...ready(), password_change: { state: "unconfirmed", at: new Date().toISOString() } })];
    renderStorage();
    expect(screen.getByText(/^Your password change at \d\d:\d\d today may not have reached TrueNAS\.$/)).toBeTruthy();
    expect(screen.getByText("If your new password doesn't work, your old one still does. Set it again below to be sure.")).toBeTruthy();
    expect(passwordField()).toBeTruthy();
    expect(screen.queryByText(/Last changed/)).toBeNull();
  });

  it("back while offline: the blocked sentence and the line are one, and the form waits", () => {
    state.targets = [
      view({ ...ready(), health: "offline", reachable: false, password_change: { state: "unconfirmed", at: "2026-11-02T18:05:00Z" } }),
    ];
    renderStorage();
    expect(screen.getByText(/^Your password change on 2 Nov( 2026)? at \d\d:\d\d may not have reached TrueNAS\.$/)).toBeTruthy();
    expect(screen.getByText(/You can set it again here once TrueNAS is back/)).toBeTruthy();
    expect(passwordField()).toBeNull();
    expect(screen.queryByText(/You can't change your password while TrueNAS is offline/)).toBeNull();
  });
});

describe("reads beside the storage card", () => {
  const read = (operation: string) => ({
    operation,
    shape: "records" as const,
    readable: true,
    result: { records: [{ id: "a", label: `From ${operation}` }], taken_at: "2026-11-10T09:30:00Z", current: true },
  });

  it("skips the read it draws itself from view.storage and draws any other", () => {
    state.targets = [view({ reads: [read("storage.status"), read("holdings.list")] })];
    renderStorage();
    expect(screen.queryByText("From storage.status")).toBeNull();
    expect(screen.getByText("From holdings.list")).toBeTruthy();
  });
});
