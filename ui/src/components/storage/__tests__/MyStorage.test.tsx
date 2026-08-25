// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { MyStorage } from "@/components/storage/MyStorage";
import type { MyTargetView } from "@/lib/queries/useMyStorage";
import type { OneShotSecret } from "@/lib/secret";

// 10.5/10.7 — the three states render distinctly, and the credential form
// appears only in the third.
//
// The middle state is the one this test exists for. A two-state design shows
// the form to a member whose account has not been created yet, dispatches at an
// account that does not exist, and tells them their password was set.

const state = { targets: [] as MyTargetView[], sent: [] as OneShotSecret[] };

vi.mock("@/lib/queries/useMyStorage", async () => {
  const actual = await vi.importActual<typeof import("@/lib/queries/useMyStorage")>(
    "@/lib/queries/useMyStorage",
  );
  return {
    ...actual,
    useMyStorage: () => ({
      data: state.targets,
      isLoading: false,
      error: null,
      refetch: vi.fn(),
    }),
    useSetStorageCredential: () => ({
      mutate: (secret: OneShotSecret) => state.sent.push(secret),
      isPending: false,
      data: undefined,
      error: null,
    }),
  };
});

function view(overrides: Partial<MyTargetView> = {}): MyTargetView {
  return {
    target: "truenas",
    entitled: true,
    account: { username: "ada" },
    credential: { set: false },
    reachable: true,
    ...overrides,
  };
}

describe("an account that exists and will not let them in", () => {
  it("says so, and points at the action that fixes it", () => {
    // Syndra creates the account before any password exists, and the target
    // disables password authentication until the member sets one. Without this
    // the page showed an account name, working mount instructions and a green
    // spine to somebody whose every connection attempt was refused.
    state.targets = [
      view({
        storage: { username: "ada", usable: false, needs_password: true, smb_enabled: false },
      }),
    ];
    renderStorage();

    expect(screen.getByText(/not switched on yet/i)).toBeTruthy();
    expect(screen.getByText(/refuse you until you set a password/i)).toBeTruthy();
  });

  it("does not tell a held account to set a password, because that would not help", () => {
    state.targets = [
      view({
        credential: { set: true },
        storage: { username: "ada", usable: false, needs_password: false, smb_enabled: false },
      }),
    ];
    renderStorage();

    expect(screen.getByText(/on hold/i)).toBeTruthy();
    expect(screen.queryByText(/not switched on yet/i)).toBeNull();
  });

  it("says nothing at all when the account works", () => {
    state.targets = [
      view({
        credential: { set: true },
        storage: { username: "ada", usable: true, needs_password: false, smb_enabled: true },
      }),
    ];
    renderStorage();

    expect(screen.queryByText(/not switched on yet/i)).toBeNull();
    expect(screen.queryByText(/on hold/i)).toBeNull();
  });
});

describe("how much room a member is using", () => {
  it("shows usage, and does not draw a full bar when there is no quota", () => {
    // TrueNAS reports no quota field at all when none is set, which is the
    // common case. Drawing 100% for "no limit" would be the most alarming
    // possible way to say "you are fine".
    state.targets = [
      view({
        credential: { set: true },
        storage: {
          username: "ada", usable: true, needs_password: false, smb_enabled: true,
          usage_readable: true,
          shares: [{ share: "main", used_bytes: 18368 }],
        },
      }),
    ];
    renderStorage();

    expect(screen.getByText(/You are using/i)).toBeTruthy();
    expect(screen.getByText(/17\.9 KiB/)).toBeTruthy();
    expect(screen.getByText(/No limit set/i)).toBeTruthy();
  });

  it("shows the limit when there is one", () => {
    state.targets = [
      view({
        credential: { set: true },
        storage: {
          username: "ada", usable: true, needs_password: false, smb_enabled: true,
          usage_readable: true,
          shares: [{ share: "main", used_bytes: 5 * 1024 * 1024 * 1024, quota_bytes: 10 * 1024 * 1024 * 1024 }],
        },
      }),
    ];
    renderStorage();

    expect(screen.getByText(/5\.0 GiB of 10\.0 GiB/)).toBeTruthy();
  });

  it("says nothing when the usage could not be read, rather than showing zero", () => {
    state.targets = [
      view({
        credential: { set: true },
        storage: {
          username: "ada", usable: true, needs_password: false, smb_enabled: true,
          usage_readable: false, shares: [],
        },
      }),
    ];
    renderStorage();

    expect(screen.queryByText(/You are using/i)).toBeNull();
  });
});

function renderStorage() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <MyStorage />
    </QueryClientProvider>,
  );
}

describe("a member's storage view", () => {
  it("offers nothing to set when no role reaches the target", () => {
    state.targets = [view({ entitled: false, account: undefined })];
    renderStorage();

    expect(screen.queryByLabelText(/password/i)).toBeNull();
    expect(screen.getByText(/none of your roles reaches/i)).toBeInTheDocument();
    // And no connection instructions: an account name that does not exist is
    // worse than no instructions at all.
    expect(screen.queryByText("ada")).toBeNull();
  });

  it("withholds the credential form while the account is still pending", () => {
    state.targets = [view({ account: undefined })];
    renderStorage();

    expect(screen.queryByLabelText(/password/i)).toBeNull();
    expect(screen.getByText(/has not been created yet/i)).toBeInTheDocument();
  });

  it("offers everything once the account exists", () => {
    state.targets = [view()];
    renderStorage();

    expect(screen.getByLabelText(/password/i)).toBeInTheDocument();
    expect(screen.getByText("ada")).toBeInTheDocument();
  });

  it("says what the password is for, because members reasonably assume one password", () => {
    state.targets = [view()];
    renderStorage();

    expect(screen.getByText(/not your Syndra sign-in/i)).toBeInTheDocument();
  });

  it("lists only the resources the current entitlements reach", () => {
    state.targets = [view({ resources: { group: ["lab_makers"] } })];
    renderStorage();

    expect(screen.getByText("lab_makers")).toBeInTheDocument();
  });

  it("explains withheld access rather than showing a member an unexplained absence", () => {
    state.targets = [
      view({
        entitled: false,
        account: undefined,
        suspended: [{ field: "group", value: "lab_makers", reason: "safety review", actor_id: "op_1" }],
      }),
    ];
    renderStorage();

    expect(screen.getByText(/safety review/)).toBeInTheDocument();
  });

  it("fails closed when the target is not answering", () => {
    state.targets = [view({ reachable: false })];
    renderStorage();

    // No form: a credential set against an add-on that never answered would be
    // reported to the member as done.
    expect(screen.queryByLabelText(/password/i)).toBeNull();
    expect(screen.getByText(/is not answering/i)).toBeInTheDocument();
    // And it says the access is untouched, so an outage does not read as a
    // withdrawal to the person it is happening to.
    expect(screen.getByText(/Nothing about\s+your access has changed/i)).toBeInTheDocument();
  });
});

// 11.9 — a member with pre-cutover metadata renders as needing enrolment, not
// as enrolled. Their hash was dropped with the vault it lived in, so telling
// them they have a password would send them to a connection that fails.
describe("the re-enrolment cutover", () => {
  it("tells somebody who enrolled before the change that they must set a new one", () => {
    state.targets = [
      view({ credential: { set: false, needs_re_enrolment: true } }),
    ];
    renderStorage();

    expect(screen.getByText(/no longer works/i)).toBeInTheDocument();
    // The form says "set", not "change": they have nothing to change.
    expect(screen.getByText("Set a storage password")).toBeInTheDocument();
  });

  it("says nothing about it to somebody who has never enrolled", () => {
    state.targets = [view({ credential: { set: false } })];
    renderStorage();

    expect(screen.queryByText(/no longer works/i)).toBeNull();
  });
});

/**
 * §30 — the only screen where a member retypes a string into another
 * application. Every rule here is the same rule: only describe what is real.
 */
describe("connection instructions", () => {
  it("are absent until there is an account to connect with", () => {
    state.targets = [
      view({
        account: undefined,
        connection: { protocol: "smb", host: "nas.makerspace.internal" },
      }),
    ];
    renderStorage();

    expect(screen.queryByText(/smb:\/\//)).toBeNull();
  });

  // A deployment that has not named a share host has not named one. A guessed
  // path that fails teaches a member to distrust the whole page — starting with
  // the parts that were right.
  it("are absent when no host is registered, rather than guessed", () => {
    state.targets = [view({ connection: undefined })];
    renderStorage();

    expect(screen.queryByText(/smb:\/\//)).toBeNull();
  });

  it("name every platform for what the member's entitlements actually reach", () => {
    state.targets = [
      view({
        resources: { share: ["members", "lab"] },
        connection: { protocol: "smb", host: "nas.makerspace.internal" },
      }),
    ];
    renderStorage();

    expect(screen.getByText("smb://nas.makerspace.internal/members")).toBeInTheDocument();
    expect(screen.getByText("smb://nas.makerspace.internal/lab")).toBeInTheDocument();
    expect(screen.getByText("\\\\nas.makerspace.internal\\members")).toBeInTheDocument();
  });

  // A member who knows their folder is missing on purpose does not spend twenty
  // minutes hunting a typo.
  it("name a withheld resource as excluded rather than dropping it silently", () => {
    state.targets = [
      view({
        resources: { share: ["members"] },
        suspended: [
          { field: "share", value: "lab", reason: "safety review", actor_id: "op_7" },
        ],
        connection: { protocol: "smb", host: "nas.makerspace.internal" },
      }),
    ];
    renderStorage();

    expect(screen.getByText(/not in this list on purpose/i)).toBeInTheDocument();
  });
});

// The password must not outlive the request. TanStack keeps a mutation's
// variables in the MutationCache, so passing the string directly left a
// member's password in memory under a docblock promising it was kept nowhere.
describe("what the page hands to the mutation", () => {
  it("sends the password in a box that empties on read", () => {
    state.sent = [];
    state.targets = [view()];
    renderStorage();

    fireEvent.change(screen.getByLabelText(/Set a storage password/), {
      target: { value: "correct horse" },
    });
    fireEvent.click(screen.getByRole("button", { name: /Set password/ }));

    expect(state.sent).toHaveLength(1);
    expect(state.sent[0].take()).toBe("correct horse");
    expect(state.sent[0].spent).toBe(true);
  });
});
