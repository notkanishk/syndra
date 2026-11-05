// @vitest-environment jsdom
import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { SignInLookup } from "@/components/people/SignInLookup";

const state = vi.hoisted(() => ({
  asked: [] as unknown[],
  data: undefined as unknown,
  error: null as unknown,
}));

vi.mock("@/lib/queries/useSignIns", () => ({
  useLookUpSignIns: () => ({
    mutate: (ask: unknown) => state.asked.push(ask),
    isPending: false,
    data: state.data,
    error: state.error,
  }),
}));
vi.mock("@/lib/queries/useApplications", () => ({
  useApplications: () => ({ data: [{ application: { id: "app-1", name: "Fab queue" } }] }),
}));
vi.mock("@/components/names/UserName", () => ({ UserName: ({ id }: { id: string }) => <a href={`/users/${id}`}>{id}</a> }));

beforeEach(() => {
  state.asked = [];
  state.data = undefined;
  state.error = null;
});

describe("Look up sign-ins (owner, 3 Nov)", () => {
  it("asks only with a reason, for the chosen range", () => {
    render(<SignInLookup userId="u1" name="Aanya" onClose={() => {}} />);
    const go = screen.getByRole("button", { name: "Look up sign-ins" });
    expect(go).toBeDisabled();
    expect(screen.getByText("Say why first.")).toBeInTheDocument();
    expect(screen.getByText(/the lookup is recorded with your reason/)).toBeInTheDocument();

    fireEvent.click(screen.getByRole("radio", { name: "7 days" }));
    fireEvent.change(screen.getByLabelText("Why"), { target: { value: "  They say they never used it  " } });
    fireEvent.click(screen.getByRole("button", { name: "Look up sign-ins" }));
    expect(state.asked).toEqual([{ user_id: "u1", app_id: undefined, days: 7, reason: "They say they never used it" }]);
  });

  it("ends on its own result, with Close, and says when Zitadel's log held more", () => {
    state.data = {
      sign_ins: [
        { user_id: "u1", app_id: "app-1", project_id: "fab", at: "2026-11-02T09:00:00Z" },
        { user_id: "u1", app_id: "", project_id: "", at: "2026-11-01T09:00:00Z" },
      ],
      truncated: true,
      since: "2026-10-03T00:00:00Z",
      until: "2026-11-02T00:00:00Z",
    };
    render(<SignInLookup userId="u1" name="Aanya" onClose={() => {}} />);
    expect(screen.getByText("Looked up")).toBeInTheDocument();
    expect(screen.getByText(/2 sign-ins by Aanya in the last 30 days/)).toBeInTheDocument();
    expect(screen.getByText("Fab queue")).toBeInTheDocument();
    expect(screen.getByText("App unknown")).toBeInTheDocument();
    expect(screen.getByText(/older ones may be missing/)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Close" })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Look up sign-ins" })).toBeNull();
  });

  it("an app's lookup names the people, each linked", () => {
    state.data = { sign_ins: [{ user_id: "u2", app_id: "app-1", project_id: "fab", at: "2026-11-02T09:00:00Z" }], truncated: false, since: "", until: "" };
    render(<SignInLookup appId="app-1" name="Fab queue" onClose={() => {}} />);
    expect(screen.getByRole("heading", { name: "Look up sign-ins to Fab queue" })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "u2" })).toHaveAttribute("href", "/users/u2");
  });
});
