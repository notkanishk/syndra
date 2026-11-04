// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { AppTokenScreen } from "@/components/apps/AppTokenScreen";
import type { AppClaimShape, Explanation, Profile } from "@/lib/queries/useClaimShape";

/**
 * The claims part of an app's page (A-14), driven through its real hooks with
 * the API faked at `request`: the breaking-save gate, the copy draft's
 * not-copied list and blocked row, a template pick, and debug mode's
 * one-account rule and its end.
 */

const api = vi.hoisted(() => ({
  calls: [] as Array<{ path: string; method: string; body?: unknown }>,
  routes: {} as Record<string, (body: unknown) => unknown>,
  view: { advanced: false },
  signIns: undefined as unknown,
}));

vi.mock("@/lib/api-client", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@/lib/api-client")>();
  return {
    ...actual,
    request: vi.fn(async (path: string, init?: { method?: string; body?: unknown }) => {
      const method = init?.method ?? "GET";
      api.calls.push({ path, method, body: init?.body });
      const route = api.routes[`${method} ${path.split("?")[0]}`];
      if (!route) throw new actual.ApiError(404, { error: "NOT_FOUND", message: `no route ${method} ${path}` });
      return route(init?.body);
    }),
  };
});
vi.mock("@/lib/ui-view", () => ({ useIsAdvanced: () => api.view.advanced, useUiView: () => ({ selfId: "me" }) }));
vi.mock("@/lib/page-crumb", () => ({ useCrumb: () => {} }));
vi.mock("@/lib/queries/useSignIns", () => ({
  useAppSignIns: () => ({ data: api.signIns }),
  useLookUpSignIns: () => ({ mutate: () => {}, isPending: false }),
}));
vi.mock("@/lib/queries/useNameResolver", () => ({
  useNameResolver: () => ({ resolveProject: (id: string) => ({ value: { name: id === "pNet" ? "Network" : id === "pAudio" ? "Audio" : "Code" }, resolved: true }) }),
}));
vi.mock("@/components/names", () => ({
  ProjectName: ({ id }: { id: string }) => <span>{id === "pNet" ? "Network" : id === "pAudio" ? "Audio" : "Code"}</span>,
  UserName: ({ id }: { id: string }) => <span>{id}</span>,
  UserAvatar: () => null,
}));

const roles = [
  { key: "staff", label: "Staff", description: "" },
  { key: "admin", label: "Admins", description: "" },
  { key: "guest", label: "Guests", description: "" },
];
const netbird: Profile = {
  claims: [{ path: ["groups"], kind: "roles", format: "array" }],
  roles: [
    { key: "staff", as: "netbird-staff", included: true },
    { key: "admin", as: "netbird-admins", included: true },
    { key: "guest", included: false },
  ],
  include_new: false,
  lands: "both",
};
const pass = (facts: Record<string, unknown> = {}) => ({ state: "pass" as const, facts });
function explanation(token: Record<string, unknown>, draft = false): Explanation {
  return {
    user_id: "ada",
    read_at: "2026-11-03T09:54:00Z",
    token,
    checks: [
      pass({ project_role_assertion: true, access_token_type: "JWT", id_token_userinfo_assertion: true, lands: "both", reads: "access_token", reached: { access_token: true } }),
      pass({ roles: ["staff", "admin"] }),
      pass({ compiled_at: "2026-11-03T09:52:00Z" }),
      pass({ source: "app", version: 3, template_id: "netbird", draft, keys: ["groups"] }),
      pass({ lands: "both" }),
      pass({ last_call_at: "2026-11-03T09:53:00Z", last_outcome: "served", last_served_at: "2026-11-03T09:53:00Z", last_sign_in_at: "2026-11-03T09:53:00Z" }),
    ],
  };
}
const words = (p: Profile) => (p.roles ?? []).filter((r) => r.included).map((r) => r.as || r.key);

function shapeOf(over: Partial<AppClaimShape>): AppClaimShape {
  return {
    application: { id: "a1", name: "NetBird", project_id: "pNet", consumer: "oidc" },
    client_id: "301928374650192837",
    source: "app",
    profile: netbird,
    version: { application_id: "a1", version: 3, project_id: "pNet", profile: netbird, template_id: "netbird", template_version: 1, note: "", saved_by: "me", saved_at: "2026-11-03T09:00:00Z" },
    project_default: { claims: [{ path: ["roles"], kind: "roles", format: "array" }], include_new: true, lands: "both" },
    project_default_source: "builtin",
    project_roles: roles,
    template: { id: "netbird", version: 1, name: "NetBird", changes: [], updated: false },
    ...over,
  };
}

function routes(shape: AppClaimShape) {
  const id = shape.application.id;
  api.routes = {
    [`GET /applications/${id}/claim-shape`]: () => shape,
    "GET /claim-templates": () => ({
      templates: [
        { id: "netbird", version: 1, name: "NetBird", profile: netbird, reads: "access_token", breaking_rule: "rename_or_remove" },
        { id: "gitlab", version: 1, name: "GitLab", profile: { ...netbird, lands: "userinfo" }, reads: "userinfo", breaking_rule: "rename_or_remove" },
        { id: "custom", version: 1, name: "Custom", profile: shape.project_default },
      ],
    }),
    "GET /catalog": () => ({ users: [{ id: "me", name: "Kanishk Singh", email: "k@example.edu", title: "" }, { id: "ada", name: "Ada Lovelace", email: "ada@example.edu", title: "" }], projects: [] }),
    "GET /applications": () => [],
    "GET /zitadel/health": () => ({ domain: "auth.example.edu" }),
    [`GET /applications/${id}/claim-shape/versions`]: () => ({ versions: [] }),
    [`GET /applications/${id}/debug/sign-ins`]: () => api.signIns ?? { on: false, session: null, sign_ins: [] },
    [`POST /applications/${id}/claim-shape/preview`]: (body) => {
      const draft = (body as { draft?: Profile }).draft;
      const now = explanation({ groups: words(shape.profile) });
      if (!draft) return { people: [{ user_id: "ada", now }], changes: [], breaking: [], left_out: [] };
      const renamed = words(draft).includes("network-staff");
      const change = { kind: "rename", role: "staff", from: "netbird-staff", to: "network-staff" };
      return {
        people: [{ user_id: "ada", now, draft: explanation({ groups: words(draft).length ? words(draft) : [] }, true) }],
        changes: renamed ? [change] : [],
        breaking: renamed && shape.source === "app" ? [change] : [],
        reach: renamed ? { by_role: { staff: 12 }, people: 23 } : null,
        left_out: draft.claims.flatMap((c, i) => (c.kind === "if_role" && !c.roles?.length ? [i] : [])),
      };
    },
    [`PUT /applications/${id}/claim-shape`]: (body) => ({
      version: { ...shape.version!, version: 4, profile: (body as { profile: Profile }).profile },
      changes: [],
      breaking: (body as { acknowledge_breaking?: boolean }).acknowledge_breaking ? [{ kind: "rename", role: "staff" }] : [],
    }),
  };
}

function renderScreen(id = "a1") {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <AppTokenScreen applicationId={id} />
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  api.calls = [];
  api.view.advanced = false;
  api.signIns = undefined;
  routes(shapeOf({}));
});

describe("an app's header", () => {
  it("names the app and links its project; the client id only in Advanced", async () => {
    renderScreen();
    expect(await screen.findByRole("heading", { level: 1 })).toHaveTextContent("NetBird");
    for (const link of screen.getAllByRole("link", { name: /Network/ })) expect(link).toHaveAttribute("href", "/projects/pNet");
    expect(screen.queryByText("301928374650192837")).not.toBeInTheDocument();
  });

  it("keeps the client id in Advanced", async () => {
    api.view.advanced = true;
    renderScreen();
    expect(await screen.findByText("301928374650192837")).toBeInTheDocument();
  });

  it("puts people in 7 days and the last sign-in in the meta", async () => {
    api.signIns = { receiving: true, people_7d: 9, people_30d: 23, last_at: new Date(Date.now() - 10 * 60_000).toISOString(), days: [] };
    renderScreen();
    expect(await screen.findByText("9 people in 7 days · last sign-in 10 min ago")).toBeInTheDocument();
  });
});

/** The roles claim's line: the whole line is the button (the chevron is aria-hidden). */
const rolesLine = () => screen.findByRole("button", { name: /^groups/ });
async function openRoles() {
  fireEvent.click(await rolesLine());
  return screen.findByLabelText("NetBird calls Staff");
}

describe("a save NetBird will act on", () => {
  it("is said on the row, in the before-line, and gated in the summary by I understand", async () => {
    renderScreen();
    const staff = await openRoles();
    fireEvent.change(staff, { target: { value: "network-staff" } });

    // Said on its row…
    expect(await screen.findByText("netbird-staff", { selector: ".type-mono" })).toBeInTheDocument();
    // …and in the before-line, with what NetBird will do.
    await screen.findByText(/1 change NetBird will act on\. 12 people move from/);

    fireEvent.click(screen.getByRole("button", { name: "Save" }));
    const dialog = await screen.findByRole("dialog");
    const save = within(dialog).getByRole("button", { name: "Save" });
    expect(save).toBeDisabled();
    fireEvent.click(within(dialog).getByRole("checkbox"));
    expect(save).toBeEnabled();
    fireEvent.click(save);

    await within(dialog).findByText("Saved");
    expect(within(dialog).getByText(/NetBird’s claims are now version 4\./)).toBeInTheDocument();
    const put = api.calls.find((c) => c.method === "PUT");
    expect(put?.body).toMatchObject({ acknowledge_breaking: true, template_id: "netbird" });
    expect((put?.body as { profile: Profile }).profile.roles?.[0]).toEqual({ key: "staff", as: "network-staff", included: true });
  });

  it("previews the draft with − and + lines, not colour alone, and says the one difference", async () => {
    renderScreen();
    fireEvent.change(await openRoles(), { target: { value: "network-staff" } });
    await screen.findByText("All six checks pass on this draft");
    expect(screen.getByText("Removed:", { exact: false })).toBeInTheDocument();
    expect(screen.getByText("Added:", { exact: false })).toBeInTheDocument();
    expect(screen.getByText(/"groups": \["network-staff", "netbird-admins"\]/)).toBeInTheDocument();
    expect(screen.getByText(/One difference for Kanishk Singh/)).toHaveTextContent(
      "One difference for Kanishk Singh: netbird-staff becomes network-staff.",
    );
    // The roles line describes the draft.
    expect(await rolesLine()).toHaveTextContent("2 of 3 roles. 2 renamed, 1 left out.");
  });

  it("shows Save only after the first edit, and folds it away on Discard", async () => {
    renderScreen();
    const staff = await openRoles();
    expect(screen.queryByRole("button", { name: "Save" })).toBeNull();
    fireEvent.change(staff, { target: { value: "network-staff" } });
    expect(await screen.findByRole("button", { name: "Save" })).toBeInTheDocument();
    // The field keeps its border while it differs from what's saved.
    expect(staff.className).not.toContain("border-transparent!");
    fireEvent.click(screen.getByRole("button", { name: "Discard the draft" }));
    await waitFor(() => expect(screen.queryByRole("button", { name: "Save" })).toBeNull());
    expect(screen.getByLabelText("NetBird calls Staff").className).toContain("border-transparent!");
  });
});

/** A seeded v1 shape on Custom: `groups`, then `custom_claims` holding four, one of them its own groups. */
const v1: Profile = {
  claims: [
    { path: ["groups"], kind: "roles", format: "array" },
    { path: ["custom_claims", "groups"], kind: "roles", format: "array" },
    { path: ["custom_claims", "given_name"], kind: "attribute", source: "given_name" },
    { path: ["custom_claims", "family_name"], kind: "attribute", source: "family_name" },
    { path: ["custom_claims", "name"], kind: "attribute", source: "name" },
  ],
  include_new: true,
  lands: "both",
};
/** Each claim line's key, top to bottom: the first mono in a line's button. */
const lineKeys = () =>
  Array.from(document.querySelectorAll("button[aria-expanded]"))
    .map((b) => b.querySelector(".type-mono")?.textContent ?? "")
    .filter(Boolean);

describe("one list of claims (A-16)", () => {
  it("opens on the list: the roles line first and folded, with its state; the template at the foot", async () => {
    renderScreen();
    const roles = await rolesLine();
    expect(roles).toHaveAttribute("aria-expanded", "false");
    expect(roles).toHaveTextContent("The roles each person holds in Network, as a list");
    expect(roles).toHaveTextContent("2 of 3 roles. 2 renamed, 1 left out.");
    // Folded on arrival, even with roles renamed and left out.
    expect(screen.queryByLabelText("NetBird calls Staff")).toBeNull();
    expect(screen.getByText("In the access token and userinfo. From the NetBird template.")).toBeInTheDocument();
    // Gone: the Claim rows fold, the roles type-label.
    expect(screen.queryByRole("button", { name: /Claim rows/ })).toBeNull();
    expect(screen.queryByText(/Roles NetBird sees/)).toBeNull();
  });

  it("puts the roles claim first and the rest A to Z, whatever order the rows were saved in", async () => {
    api.view.advanced = true;
    const p: Profile = {
      ...v1,
      claims: [
        { path: ["zeta"], kind: "static", value: "z" },
        { path: ["alpha"], kind: "attribute", source: "email" },
        ...v1.claims.slice(1),
        v1.claims[0],
      ],
    };
    routes(shapeOf({ profile: p, version: { ...shapeOf({}).version!, profile: p } }));
    renderScreen();
    await rolesLine();
    expect(lineKeys()).toEqual(["groups", "alpha", "custom_claims", "zeta"]);
    expect(screen.getByRole("button", { name: /^custom_claims/ })).toHaveTextContent("An object holding family_name, given_name, groups and name");
    expect(screen.getByRole("button", { name: /^custom_claims/ })).toHaveTextContent("Its groups is the same list of roles.");
  });

  it("in Basic, opens only the roles, to their words and Included: no name, Sent as, Add a claim or Lands in", async () => {
    routes(shapeOf({ profile: v1, version: { ...shapeOf({}).version!, profile: v1 } }));
    renderScreen();
    await openRoles();
    expect(screen.getByLabelText("NetBird sees Staff")).toBeInTheDocument();
    // Basic can't rename the roles claim (owner, 4 Nov): the app's own settings point at it.
    expect(screen.queryByLabelText("Claim name")).toBeNull();
    expect(screen.queryByRole("radiogroup", { name: "Sent as" })).toBeNull();
    expect(screen.queryByText("Roles added later")).toBeNull();
    expect(screen.queryByRole("button", { name: "Add a claim" })).toBeNull();
    expect(screen.queryByText("Lands in")).toBeNull();
    // The other lines say what they carry and don't open.
    expect(screen.getByText("custom_claims", { selector: ".type-mono" })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /^custom_claims/ })).toBeNull();
  });

  it("in Advanced, opens one line at a time, each to its own editor", async () => {
    api.view.advanced = true;
    routes(shapeOf({ profile: v1, version: { ...shapeOf({}).version!, profile: v1 } }));
    renderScreen();
    // Add a claim and Lands in sit after the list, nothing to open.
    expect(await screen.findByRole("button", { name: "Add a claim" })).toBeInTheDocument();
    expect(screen.getByText("Lands in")).toBeInTheDocument();
    expect(screen.queryByLabelText("Claim name")).toBeNull();

    await openRoles();
    expect(screen.getByLabelText("Claim name")).toHaveValue("groups");
    expect(screen.getByText("NetBird reads the name set in its Settings › Groups › JWT claim.")).toBeInTheDocument();
    expect(screen.getByRole("radiogroup", { name: "Sent as" })).toBeInTheDocument();
    expect(screen.getByText("Roles added later")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Remove the groups claim" })).toHaveTextContent("Remove groups");

    fireEvent.click(screen.getByRole("button", { name: /^custom_claims/ }));
    expect(await rolesLine()).toHaveAttribute("aria-expanded", "false");
    await waitFor(() => expect(screen.queryByLabelText("NetBird calls Staff")).toBeNull());
    expect(screen.getByLabelText("Object name")).toHaveValue("custom_claims");
    expect(screen.getAllByLabelText("Claim name").map((i) => (i as HTMLInputElement).value)).toEqual(["family_name", "given_name", "groups", "name"]);
    expect(screen.getByRole("button", { name: "Remove groups from custom_claims" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Remove custom_claims and the 4 claims inside it" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Add a claim inside" })).toBeInTheDocument();
  });

  it("draws each control as its design-system part: outline sm on its own, danger for Remove", async () => {
    api.view.advanced = true;
    renderScreen();
    await openRoles();
    const outline = "border-line-strong";
    await screen.findByRole("button", { name: "Show the checks" });
    for (const name of ["See the template", "Copy another app’s shape", "Show the checks", "Add a claim"]) {
      expect(screen.getByRole("button", { name }).className).toContain(outline);
    }
    expect(screen.getByRole("button", { name: "Remove the groups claim" }).className).toContain("border-danger-line");
    // Fields are the sm control's height.
    expect(screen.getByLabelText("Claim name").className).toContain("py-[5px]");
    expect(screen.getByLabelText("NetBird calls Staff").className).toContain("py-[5px]");
  });
});

describe("adding a claim (A-16 §3, a menu of finished claims)", () => {
  async function openMenu() {
    api.view.advanced = true;
    renderScreen();
    fireEvent.click(await screen.findByRole("button", { name: "Add a claim" }));
    return screen.findByRole("listbox", { name: "Add a claim" });
  }

  it("greys the keys Zitadel sends and the roles claim already sent, saying why, and they do nothing", async () => {
    const menu = await openMenu();
    const email = within(menu).getByRole("option", { name: /^Email/ });
    expect(email).toHaveAttribute("aria-disabled", "true");
    expect(email).toHaveTextContent("Zitadel sends email");
    expect(within(menu).getByRole("option", { name: /^Full name/ })).toHaveTextContent("Zitadel sends name");
    expect(within(menu).getByRole("option", { name: /^Username/ })).toHaveTextContent("Zitadel sends preferred_username");
    expect(within(menu).getByRole("option", { name: /^Roles NetBird sees/ })).toHaveTextContent("Sent as groups");
    fireEvent.click(email);
    expect(screen.getByRole("listbox", { name: "Add a claim" })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Save" })).toBeNull();
  });

  it("adds a finished claim as a working line in its A-to-Z place, opening no editor", async () => {
    const menu = await openMenu();
    fireEvent.click(within(menu).getByRole("option", { name: /^Given name/ }));
    const line = await screen.findByRole("button", { name: /^given_name/ });
    expect(line).toHaveTextContent("Given name");
    expect(line).toHaveTextContent("New in this draft.");
    expect(line).toHaveAttribute("aria-expanded", "false");
    expect(screen.queryByLabelText("Claim name")).toBeNull();
    await waitFor(() => expect(screen.queryByRole("listbox", { name: "Add a claim" })).toBeNull());
    expect(await screen.findByRole("button", { name: "Save" })).toBeEnabled();
    await waitFor(() =>
      expect(
        api.calls.some((c) => c.path.endsWith("/preview") && String(JSON.stringify((c.body as { draft?: Profile }).draft?.claims)).includes('"given_name"')),
      ).toBe(true),
    );
  });

  it("opens the role picker first for Yes if they hold a role, adding nothing until a role is chosen", async () => {
    const menu = await openMenu();
    fireEvent.click(within(menu).getByRole("option", { name: /^Yes if they hold a role/ }));
    const dialog = await screen.findByRole("dialog");
    expect(within(dialog).getByText("Which roles make it true")).toBeInTheDocument();
    expect(within(dialog).getByRole("button", { name: "Use this role" })).toBeDisabled();
    fireEvent.click(within(dialog).getByRole("button", { name: "Cancel" }));
    expect(screen.queryByRole("button", { name: "Save" })).toBeNull();
  });

  it("lands a fixed value opened, its name focused, and keeps Save off until it has a name and a value", async () => {
    const menu = await openMenu();
    fireEvent.click(within(menu).getByRole("option", { name: /^A fixed value/ }));
    const name = await screen.findByLabelText("Claim name");
    expect(name).toHaveValue("");
    await waitFor(() => expect(name).toHaveFocus());
    expect(screen.getByRole("alert")).toHaveTextContent("Name it and give it a value.");
    expect(screen.getByRole("button", { name: "Save" })).toBeDisabled();
    fireEvent.change(name, { target: { value: "org" } });
    expect(await screen.findByRole("alert")).toHaveTextContent("Give it a value.");
    // Still the same line, still open, still focused where you type.
    expect(screen.getByLabelText("Claim name")).toBe(name);
    fireEvent.change(screen.getByLabelText("Value for everyone"), { target: { value: "makerspace" } });
    await waitFor(() => expect(screen.queryByRole("alert")).toBeNull());
    expect(screen.getByRole("button", { name: "Save" })).toBeEnabled();
  });
});

describe("the page, calm by default (A-15)", () => {
  it("keeps the checks to one line when all pass, and shows them on demand, whole", async () => {
    renderScreen();
    expect(await screen.findByText("All six checks pass")).toBeInTheDocument();
    expect(screen.queryByRole("list", { name: "Checks" })).toBeNull();
    fireEvent.click(screen.getByRole("button", { name: "Show the checks" }));
    expect(await screen.findByRole("list", { name: "Checks" })).toBeInTheDocument();
    expect(screen.getByText("Staff and Admins, as Zitadel shows them.")).toBeInTheDocument();
    expect(screen.getByText(/Syndra answers with/)).toHaveTextContent("Syndra answers with groups, below.");
    fireEvent.click(screen.getByRole("button", { name: "Hide the checks" }));
    await waitFor(() => expect(screen.queryByRole("list", { name: "Checks" })).toBeNull());
  });

  it("opens the checks by itself on a failure, compact, and says it under the summary", async () => {
    const failing = (token: Record<string, unknown>): Explanation => {
      const e = explanation(token);
      e.checks[0] = { state: "fail", facts: { project_role_assertion: true, access_token_type: "BEARER", lands: "both", reads: "access_token", reached: { userinfo: true } } };
      e.checks[4] = { state: "fail", facts: { lands: "both" } };
      return e;
    };
    routes(shapeOf({}));
    api.routes["POST /applications/a1/claim-shape/preview"] = () => ({
      people: [{ user_id: "ada", now: failing({ groups: ["netbird-staff"] }) }],
      changes: [],
      breaking: [],
      left_out: [],
    });
    renderScreen();
    expect(await screen.findByText("Checks 1 and 5 fail")).toBeInTheDocument();
    expect(screen.getByRole("list", { name: "Checks" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Hide the checks" })).toHaveAttribute("aria-expanded", "true");
    // Compact: a check that passed keeps its sentence but not its detail.
    expect(screen.getByText(/holds 2 roles in Network/)).toBeInTheDocument();
    expect(screen.queryByText("Staff and Admins, as Zitadel shows them.")).toBeNull();
    expect(screen.getByText("NetBird's token type in Zitadel is Bearer. Only a JWT access token carries claims.")).toBeInTheDocument();
    expect(screen.getByText(/Syndra answers with/)).toHaveTextContent("Syndra answers with groups, but only for userinfo.");
    expect(screen.getByText("NetBird reads its access token, so it sees none of this, and empties every group it made from a token.")).toBeInTheDocument();
    expect(screen.getByText("Doesn’t reach the access token. Check 1 in the preview says why.")).toBeInTheDocument();
  });
});

describe("history", () => {
  const v = (version: number, note = "") => ({
    application_id: "a1",
    version,
    project_id: "pNet",
    profile: netbird,
    template_id: "netbird",
    template_version: 1,
    note,
    saved_by: "me",
    saved_at: "2026-11-03T09:00:00Z",
    changes: [],
  });

  it("shows only the version in use while there is one, with nothing to go back to", async () => {
    api.routes["GET /applications/a1/claim-shape/versions"] = () => ({ versions: [v(1, "As Actions v1 sent it")] });
    renderScreen();
    expect(await screen.findByText("Version 1, as Actions v1 sent it")).toBeInTheDocument();
    expect(screen.getByText(/^Saved 3 Nov by/)).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /Show all/ })).toBeNull();
    expect(screen.queryByRole("button", { name: "Go back to this version" })).toBeNull();
  });

  it("unfolds every version behind Show all N once there are two", async () => {
    api.routes["GET /applications/a1/claim-shape/versions"] = () => ({ versions: [v(2), v(1, "As Actions v1 sent it")] });
    renderScreen();
    expect(await screen.findByText("Version 2")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Go back to this version" })).toBeNull();
    fireEvent.click(screen.getByRole("button", { name: /Show all 2/ }));
    expect(await screen.findByRole("button", { name: "Go back to this version" })).toBeInTheDocument();
  });
});

describe("copying another app's shape", () => {
  beforeEach(() => {
    const audio = shapeOf({
      application: { id: "aud", name: "Audio-Dash", project_id: "pAudio", consumer: "oidc" },
      source: "builtin",
      profile: { claims: [{ path: ["roles"], kind: "roles", format: "array" }], include_new: true, lands: "both" },
      version: null,
      template: null,
      project_roles: [
        { key: "staff", label: "Staff", description: "" },
        { key: "dj", label: "DJ", description: "" },
      ],
    });
    routes(audio);
    api.routes["GET /applications"] = () => [
      { application: { id: "inv", name: "Inventory", project_id: "pInv", consumer: "oidc" }, claims: { source: "app", keys: ["roles", "is_admin"], lands: "both", template_id: "custom", template_changed: true } },
    ];
    api.routes["POST /applications/aud/claim-shape/draft-from"] = () => ({
      profile: {
        claims: [
          { path: ["roles"], kind: "roles", format: "array" },
          { path: ["is_admin"], kind: "if_role", roles: [] },
        ],
        roles: [{ key: "staff", included: true }],
        include_new: true,
        lands: "both",
      },
      template_id: "custom",
      template_version: 1,
      copied_from: { application_id: "inv", version: 5 },
      not_copied: [
        { role_key: "inv-admin", name: "Admin" },
        { role_key: "trained", name: "Trained" },
      ],
      missing_roles: [{ row: 1, role_key: "inv-admin", name: "Admin" }],
    });
  });

  async function copyInventory() {
    renderScreen("aud");
    fireEvent.click(await screen.findByRole("radio", { name: "Copy another app's shape" }));
    fireEvent.click(await screen.findByText("Inventory"));
    expect(await screen.findByText(/Not in this project, so not copied: Admin \(/)).toBeInTheDocument();
    await waitFor(() => expect(api.calls.some((c) => c.path.endsWith("draft-from"))).toBe(true));
    expect(api.calls.find((c) => c.path.endsWith("draft-from"))?.body).toEqual({ application_id: "inv" });

    const copy = await screen.findByRole("button", { name: "Copy Inventory's shape" });
    await waitFor(() => expect(copy).toBeEnabled());
    fireEvent.click(copy);
    expect(await screen.findByText(/as a draft\. Not saved yet\./)).toBeInTheDocument();
  }

  it("lists what didn't copy, and a row whose role isn't here stops Save, opening its line by itself", async () => {
    api.view.advanced = true;
    await copyInventory();
    expect(screen.getByRole("button", { name: /^is_admin/ })).toHaveAttribute("aria-expanded", "true");
    expect(screen.getByRole("alert")).toHaveTextContent("Admin isn't in Audio. Choose a role from Audio, or remove this row.");
    expect(screen.getByRole("button", { name: "Save" })).toBeDisabled();
    expect(screen.getByText(/to save\./)).toHaveTextContent("Fix is_admin to save.");
    // Said under the list too.
    expect(screen.getAllByText(/Not in this project, so not copied/).length).toBeGreaterThan(0);
  });

  it("in Basic, says where the row that stops Save is, without showing it", async () => {
    await copyInventory();
    expect(screen.queryByRole("alert")).toBeNull();
    expect(screen.getByRole("button", { name: "Save" })).toBeDisabled();
    expect(screen.getByText(/to save\./)).toHaveTextContent("Fix is_admin in Advanced to save.");
  });
});

describe("an app with no shape of its own", () => {
  it("starts from the template its name suggests, as a draft that previews at once", async () => {
    const gitlab = shapeOf({
      application: { id: "gl", name: "GitLab", project_id: "pCode", consumer: "oidc" },
      source: "builtin",
      profile: { claims: [{ path: ["roles"], kind: "roles", format: "array" }], include_new: true, lands: "both" },
      version: null,
      template: null,
      suggested_template: "gitlab",
    });
    routes(gitlab);
    api.routes["POST /applications/gl/claim-shape/draft-from"] = () => ({
      profile: { claims: [{ path: ["groups"], kind: "roles", format: "array" }], roles: [], include_new: true, lands: "userinfo" },
      template_id: "gitlab",
      template_version: 1,
      not_copied: [],
      missing_roles: [],
    });
    renderScreen("gl");
    expect(await screen.findByText("The roles each person holds in Code, as a list")).toBeInTheDocument();
    expect(screen.getByText("All 3 roles, in Syndra's words.")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /Code’s default/ })).toHaveAttribute("href", "/projects/pCode#claims-default");
    expect(await screen.findByText("Suggested: this app is called GitLab")).toBeInTheDocument();
    await waitFor(() => expect(api.calls.find((c) => c.path.endsWith("draft-from"))?.body).toEqual({ template_id: "gitlab" }));
    // The preview runs the draft before anything is committed.
    await waitFor(() => expect(api.calls.some((c) => c.path.endsWith("/preview") && (c.body as { draft?: unknown }).draft)).toBe(true));

    const start = screen.getByRole("button", { name: "Start from GitLab" });
    await waitFor(() => expect(start).toBeEnabled());
    fireEvent.click(start);
    expect(await screen.findByText("In the ID token and userinfo. From the GitLab template, as a draft. Not saved yet.")).toBeInTheDocument();
    expect(screen.getByText("groups", { selector: ".type-mono" })).toBeInTheDocument();
    // A first shape on the default is never breaking: Save needs no tick.
    fireEvent.click(screen.getByRole("button", { name: "Save" }));
    const dialog = await screen.findByRole("dialog");
    expect(within(dialog).queryByRole("checkbox")).toBeNull();
    expect(within(dialog).getByRole("button", { name: "Save" })).toBeEnabled();
  });
});

describe("debug mode", () => {
  const session = (over: Record<string, unknown> = {}) => ({
    application_id: "a1",
    user_id: "me",
    started_by: "me",
    started_at: "2026-11-03T10:06:00Z",
    ends_at: new Date(Date.now() + 14 * 60_000).toISOString(),
    ...over,
  });

  it("captures one account, you by default", async () => {
    api.routes["PUT /applications/a1/debug"] = () => ({ ends_at: session().ends_at, switched: false, session: session() });
    renderScreen();
    fireEvent.click(await screen.findByRole("button", { name: "Turn on debug mode" }));
    const dialog = await screen.findByRole("dialog");
    expect(within(dialog).getByText("One account at a time")).toBeInTheDocument();
    expect(within(dialog).getByRole("radio", { name: /You, Kanishk Singh/ })).toBeChecked();
    expect(within(dialog).getByText(/15 minutes from now/)).toBeInTheDocument();
    fireEvent.click(within(dialog).getByRole("button", { name: "Turn on debug mode" }));
    await within(dialog).findByText("On");
    expect(api.calls.find((c) => c.method === "PUT" && c.path === "/applications/a1/debug")?.body).toEqual({});
  });

  it("switches to one other person, keeping the end", async () => {
    api.signIns = { on: true, session: session(), sign_ins: [] };
    api.routes["PUT /applications/a1/debug"] = () => ({ ends_at: session().ends_at, switched: true, session: session({ user_id: "ada" }) });
    renderScreen();
    expect(await screen.findByText("Waiting for your next sign-in to NetBird.")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Change account" }));
    const dialog = await screen.findByRole("dialog");
    // The one captured now is ticked grey; there is no "everyone".
    expect(within(dialog).getByText("Captured now")).toBeInTheDocument();
    expect(within(dialog).queryByText(/everyone/i)).toBeNull();
    fireEvent.change(within(dialog).getByPlaceholderText("Choose a person"), { target: { value: "Ada" } });
    fireEvent.click(within(dialog).getByText("Ada Lovelace"));
    expect(within(dialog).getByText(/It still ends at/)).toBeInTheDocument();
    fireEvent.click(within(dialog).getByRole("button", { name: "Switch to Ada Lovelace" }));
    await within(dialog).findByText("Switched");
    expect(api.calls.find((c) => c.method === "PUT" && c.path === "/applications/a1/debug")?.body).toEqual({ user_id: "ada" });
  });

  it("says it ended by itself, keeps the list as a record, and offers to turn on again", async () => {
    const ends = "2026-11-03T10:21:00Z";
    api.signIns = { on: false, session: session({ ends_at: ends, ended_at: ends, ended_by: "system" }), sign_ins: [] };
    renderScreen();
    expect(await screen.findByText("Ended")).toBeInTheDocument();
    expect(screen.getByText(/after 15 minutes\./)).toBeInTheDocument();
    expect(screen.getByText(/then Syndra forgets it/)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Turn on again" })).toBeInTheDocument();
  });

  it("holds new sign-ins while one is open, and counts them", async () => {
    const row = (seq: number) => ({
      seq,
      at: new Date(Date.now() - seq * 1000).toISOString(),
      function: "function/preaccesstoken",
      client_id: "c",
      user_id: "me",
      grants: [],
      request: { user: { id: "me" }, user_grants: [{ projectId: "pNet", roles: ["staff"] }] },
      response: { append_claims: [{ key: "groups", value: ["netbird-staff"] }] },
      duration_ms: 4,
      checks: explanation({}).checks,
    });
    api.signIns = { on: true, session: session(), sign_ins: [row(1)] };
    renderScreen();
    fireEvent.click(await screen.findByRole("button", { name: /Open the sign-in at/ }));
    expect(await screen.findByText("Rebuilt, not the signed token")).toBeInTheDocument();
    expect(screen.getByText(/The signature isn’t shown\./)).toBeInTheDocument();

    api.signIns = { on: true, session: session(), sign_ins: [row(2), row(1)] };
    await act(async () => {
      await new Promise((r) => setTimeout(r, 3200));
    });
    expect(await screen.findByText(/1 new sign-in waits until you close this one\./)).toBeInTheDocument();
    expect(screen.getAllByRole("button", { name: /sign-in at/ })).toHaveLength(1);
  }, 10_000);
});
