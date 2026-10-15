// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { HomeLine, LimitDialog, limitVerb } from "@/components/targets/HomeLimit";
import type { AccountHome } from "@/lib/queries/useTargets";

// A-03 §3: one person's own home folder limit, on the system's account list.

const sent: Array<{ path: string; body: unknown }> = [];

vi.mock("@/lib/api-client", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@/lib/api-client")>()),
  request: vi.fn(async (path: string, init?: { body?: unknown }) => {
    sent.push({ path, body: init?.body });
    return { id: "a-new", status: "lifted" };
  }),
}));

vi.mock("@/lib/queries/useNameResolver", () => ({
  useNameResolver: () => ({
    resolveRole: () => ({ value: { display_name: "Fabrication lead" }, resolved: true }),
  }),
}));

const GB = 1024 ** 3;
const base: AccountHome = {
  subject_id: "u-ada",
  username: "ada",
  gives_home: true,
  roles_limit: 20 * GB,
  roles_limit_from: { project_id: "p", role_key: "fab_lead" },
  used_bytes: 18.4 * GB,
};

function wrap(ui: React.ReactNode) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(<QueryClientProvider client={client}>{ui}</QueryClientProvider>);
}

beforeEach(() => {
  sent.length = 0;
});

describe("the home line", () => {
  it("says the roles' limit and where it comes from", () => {
    wrap(<HomeLine home={base} />);
    expect(screen.getByText("Home folder up to 20 GB, from Fabrication lead")).toBeTruthy();
    expect(limitVerb(base)).toBe("Set a limit");
  });

  it("says a person's own limit, and when it ends", () => {
    const home = { ...base, own: { allowance_id: "a1", bytes: 50 * GB, reason: "thesis data", actor_id: "op", set_at: "", expires_at: "2026-11-30T12:00:00Z" } };
    wrap(<HomeLine home={home} />);
    expect(screen.getByText("Home folder up to 50 GB, their own")).toBeTruthy();
    expect(screen.getByText(/Ends 30 Nov · “thesis data”/)).toBeTruthy();
    expect(limitVerb(home)).toBe("Change limit");
  });

  it("turns amber when its review is due, and nothing ends by itself", () => {
    const home = { ...base, own: { allowance_id: "a1", bytes: 50 * GB, reason: "r", actor_id: "op", set_at: "", review_date: "2026-11-30T00:00:00Z", review_due: true } };
    wrap(<HomeLine home={home} />);
    expect(screen.getByText(/Review was due 30 Nov\. It stays at 50 GB until you keep it or end it\./)).toBeTruthy();
    expect(limitVerb(home)).toBe("Review limit");
  });

  it("says an ended own limit left the folder over, and that nothing was deleted", () => {
    wrap(<HomeLine home={{ ...base, used_bytes: 34 * GB, own_ended: { bytes: 50 * GB, ended_at: "2026-11-30T12:00:00Z" } }} />);
    expect(screen.getByText(/Their own 50 GB ended on 30 Nov with 34\.0 GiB in their folder\./)).toBeTruthy();
    expect(screen.getByText(/can't save new ones until they're under 20 GB\. Nothing was deleted\./)).toBeTruthy();
  });

  it("is nothing for an account no role gives a home", () => {
    const { container } = wrap(<HomeLine home={{ ...base, gives_home: false }} />);
    expect(container.textContent).toBe("");
    expect(limitVerb({ ...base, gives_home: false })).toBeNull();
  });
});

describe("giving one person their own limit", () => {
  it("sends a bounded per-person value, and says what it replaces", async () => {
    const onClose = vi.fn();
    wrap(<LimitDialog target="truenas" name="Ada Rao" home={base} onClose={onClose} />);
    expect(screen.getByText("Give Ada Rao their own home folder limit")).toBeTruthy();
    expect(screen.getByText(/Their roles give them up to 20 GB, from Fabrication lead/)).toBeTruthy();
    expect(screen.getByText(/They have 18\.4 GiB in their home folder now/)).toBeTruthy();

    const give = () => screen.getByRole("button", { name: /^Give them/ });
    expect(give().hasAttribute("disabled")).toBe(true);
    fireEvent.change(screen.getByLabelText("Limit in GB"), { target: { value: "50" } });
    fireEvent.change(screen.getByLabelText("Why"), { target: { value: "thesis data" } });
    fireEvent.change(screen.getByLabelText("Ends on"), { target: { value: "2026-11-30" } });
    fireEvent.click(screen.getByRole("button", { name: "Give them 50 GB" }));

    await waitFor(() => expect(onClose).toHaveBeenCalled());
    expect(sent[0].path).toBe("/allowances");
    expect(sent[0].body).toMatchObject({
      subject_id: "u-ada",
      target: "truenas",
      field: "home_quota",
      value: String(50 * GB),
      direction: "set",
      reason: "thesis data",
    });
    expect((sent[0].body as { expires_at?: string }).expires_at).toBeTruthy();
  });

  it("keeps a reviewed limit as one write (the server replaces the old), or just ends it", async () => {
    const own = { allowance_id: "a1", bytes: 50 * GB, reason: "thesis data", actor_id: "op", set_at: "", review_date: "2026-11-30T00:00:00Z", review_due: true };
    const onClose = vi.fn();
    const { unmount } = wrap(<LimitDialog target="truenas" name="Ada Rao" home={{ ...base, own }} onClose={onClose} />);
    expect(screen.getByText("Review Ada Rao's own home folder limit")).toBeTruthy();
    fireEvent.change(screen.getByLabelText("Review on"), { target: { value: "2027-02-28" } });
    fireEvent.click(screen.getByRole("button", { name: "Keep it" }));
    await waitFor(() => expect(onClose).toHaveBeenCalled());
    expect(sent.map((s) => s.path)).toEqual(["/allowances"]);
    unmount();

    sent.length = 0;
    wrap(<LimitDialog target="truenas" name="Ada Rao" home={{ ...base, own }} onClose={vi.fn()} />);
    fireEvent.click(screen.getByRole("button", { name: "End it" }));
    await waitFor(() => expect(sent.map((s) => s.path)).toEqual(["/allowances/a1/lift"]));
  });
});
