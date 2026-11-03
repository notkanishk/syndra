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

describe("a save NetBird will act on", () => {
  it("is said on the row, in the before-line, and gated in the summary by I understand", async () => {
    renderScreen();
    const staff = await screen.findByLabelText("NetBird calls Staff");
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

  it("previews the draft with − and + lines, not colour alone", async () => {
    renderScreen();
    fireEvent.change(await screen.findByLabelText("NetBird calls Staff"), { target: { value: "network-staff" } });
    await screen.findByText(/Zitadel would receive this once you save:/);
    expect(screen.getByText("Removed:", { exact: false })).toBeInTheDocument();
    expect(screen.getByText("Added:", { exact: false })).toBeInTheDocument();
    expect(screen.getByText(/"groups": \["network-staff", "netbird-admins"\]/)).toBeInTheDocument();
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

  it("lists what didn't copy, and a row whose role isn't here stops Save", async () => {
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
    expect(screen.getByRole("alert")).toHaveTextContent("Admin isn't in Audio. Choose a role from Audio, or remove this row.");
    expect(screen.getByRole("button", { name: "Save" })).toBeDisabled();
    expect(screen.getByText(/to save\./)).toHaveTextContent("Fix is_admin to save.");
    // The roles table says it again, under the rows.
    expect(screen.getAllByText(/Not in this project, so not copied/).length).toBeGreaterThan(0);
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
    expect(await screen.findByText(/GitLab uses Code's default shape/)).toBeInTheDocument();
    expect(await screen.findByText("Suggested: this app is called GitLab")).toBeInTheDocument();
    await waitFor(() => expect(api.calls.find((c) => c.path.endsWith("draft-from"))?.body).toEqual({ template_id: "gitlab" }));
    // The preview runs the draft before anything is committed.
    await waitFor(() => expect(api.calls.some((c) => c.path.endsWith("/preview") && (c.body as { draft?: unknown }).draft)).toBe(true));

    const start = screen.getByRole("button", { name: "Start from GitLab" });
    await waitFor(() => expect(start).toBeEnabled());
    fireEvent.click(start);
    expect(await screen.findByText("From the GitLab template, as a draft. Not saved yet.")).toBeInTheDocument();
    expect(screen.getByDisplayValue("groups")).toBeInTheDocument();
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
