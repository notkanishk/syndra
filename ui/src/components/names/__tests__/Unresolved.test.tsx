// @vitest-environment jsdom
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { ProjectName, RoleRef, UserName } from "@/components/names";
import { projectLabel } from "@/components/names/Unresolved";
import { UiViewProvider } from "@/lib/ui-view";

// Zitadel unreachable: the catalog answered, and knows nobody and no project.
const miss = { value: undefined, resolved: true } as const;
vi.mock("@/lib/queries/useNameResolver", () => ({
  useNameResolver: () => ({
    resolveUser: () => miss,
    resolveProject: () => miss,
    resolveRole: () => miss,
    resolveBundle: () => miss,
  }),
}));

const PROJECT_ID = "365951028870251526";
const USER_ID = "366421618001445894";

const inView = (view: "basic" | "advanced", ui: React.ReactNode) =>
  render(
    <UiViewProvider isOperator initialView={view}>
      {ui}
    </UiViewProvider>,
  );

/**
 * One designed fallback for a name nothing could place. A degraded read used to
 * show a raw project id on Roles, a bundle and Insights, "— / admin" on Pending
 * and "Unknown account 3664…" on Home, each page failing its own way.
 */
describe("a name nothing could place", () => {
  it("adds the id in both views, after the words and never instead of them", () => {
    for (const view of ["basic", "advanced"] as const) {
      const { container, unmount } = inView(
        view,
        <p>
          <UserName id={USER_ID} /> · <ProjectName id={PROJECT_ID} />
        </p>,
      );
      expect(container.textContent).toBe(`Unknown account ${USER_ID} · Unknown project ${PROJECT_ID}`);
      expect(container.querySelectorAll(".type-mono.text-faint")).toHaveLength(2);
      unmount();
    }
  });

  it("copies the id, which is the only thing anyone can look up in Zitadel", async () => {
    const writeText = vi.fn().mockResolvedValue(undefined);
    Object.defineProperty(navigator, "clipboard", { value: { writeText }, configurable: true });
    inView("basic", <UserName id={USER_ID} />);
    fireEvent.click(screen.getByRole("button", { name: "Copy account id" }));
    await waitFor(() => expect(writeText).toHaveBeenCalledWith(USER_ID));
  });

  it("never turns a role pair into a dash and a slug", () => {
    const { container } = inView("basic", <RoleRef projectId={PROJECT_ID} roleKey="admin" />);
    expect(container.textContent).toBe(`Unknown project ${PROJECT_ID} / admin`);
    expect(container.textContent).not.toContain("—");
  });

  it("never takes the id a server view sends for a project's name, though it shows it beside the words", () => {
    const { container } = inView("basic", <ProjectName id={PROJECT_ID} name={PROJECT_ID} />);
    expect(container.textContent).toBe(`Unknown project ${PROJECT_ID}`);
  });

  it("uses the name a server view really carries", () => {
    const { container } = inView("basic", <ProjectName id={PROJECT_ID} name="Laser Lab" />);
    expect(container.textContent).toBe("Laser Lab");
  });

  it("gives the same words where a string is needed", () => {
    expect(projectLabel("Laser Lab", PROJECT_ID)).toBe("Laser Lab");
    expect(projectLabel(PROJECT_ID, PROJECT_ID)).toBe("Unknown project");
    expect(projectLabel("", PROJECT_ID)).toBe("Unknown project");
    expect(projectLabel(undefined, null)).toBe("Unknown project");
  });
});
