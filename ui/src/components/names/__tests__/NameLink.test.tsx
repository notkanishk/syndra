// @vitest-environment jsdom
import { render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { BundleName, ProjectName, RoleRef, UserName } from "@/components/names";
import { NESTED_NAME_LINK } from "@/components/names/NameLink";
import { ResolvedProjectName } from "@/components/names/ProjectName";
import { Modal } from "@/components/ui/Modal";

const miss = { value: undefined, resolved: true } as const;

vi.mock("@/lib/queries/useNameResolver", () => ({
  useNameResolver: () => ({
    resolveUser: (id: string) => (id === "u1" ? { value: { display_name: "Aanya Rao", email: "a@x.org" }, resolved: true } : miss),
    resolveProject: (id: string) => (id === "p1" ? { value: { name: "Laser Lab" }, resolved: true } : miss),
    resolveRole: () => miss,
    resolveBundle: (id: string) => (id === "b1" ? { value: { name: "Lab Tech" }, resolved: true } : miss),
  }),
}));

afterEach(() => vi.restoreAllMocks());

/** G14 (A-20 P1): a name that refers to something with its own page links there. */
describe("name parts link by default", () => {
  it("a person, a project and a bundle each link to their page, the words as the link's name", () => {
    render(
      <p>
        <UserName id="u1" /> <ProjectName id="p1" /> <BundleName id="b1" />
      </p>,
    );
    expect(screen.getByRole("link", { name: "Aanya Rao" })).toHaveAttribute("href", "/users/u1");
    expect(screen.getByRole("link", { name: "Laser Lab" })).toHaveAttribute("href", "/projects/p1");
    expect(screen.getByRole("link", { name: "Lab Tech" })).toHaveAttribute("href", "/bundles/b1");
  });

  it("a project named by the server view, and a resolved one, link too", () => {
    render(
      <p>
        <ProjectName id="p9" name="Metal Shop" /> <ResolvedProjectName id="p8" name="Wood Shop" resolved />
      </p>,
    );
    expect(screen.getByRole("link", { name: "Metal Shop" })).toHaveAttribute("href", "/projects/p9");
    expect(screen.getByRole("link", { name: "Wood Shop" })).toHaveAttribute("href", "/projects/p8");
  });

  it("a role reference links its project and leaves the role key as text", () => {
    render(<RoleRef projectId="p1" roleKey="admin" />);
    expect(screen.getByRole("link", { name: "Laser Lab" })).toHaveAttribute("href", "/projects/p1");
    expect(screen.getAllByRole("link")).toHaveLength(1);
  });

  it("the email suffix is part of the one link, not a second", () => {
    render(<UserName id="u1" showEmail />);
    expect(screen.getAllByRole("link")).toHaveLength(1);
    expect(screen.getByRole("link")).toHaveTextContent("Aanya Rao · a@x.org");
  });

  it("href sends a service account to its own page", () => {
    render(<UserName id="u1" href="/system/service-accounts/u1" />);
    expect(screen.getByRole("link", { name: "Aanya Rao" })).toHaveAttribute("href", "/system/service-accounts/u1");
  });

  it("a name nothing could place still links, and its copy control stays outside the link", () => {
    render(<UserName id="ghost" />);
    const link = screen.getByRole("link", { name: "Unknown account" });
    expect(link).toHaveAttribute("href", "/users/ghost");
    expect(link.querySelector("button")).toBeNull();
  });

  it("an empty id and the system actor are not names of anything: no link", () => {
    render(
      <p>
        <UserName id={null} /> <UserName id="system" /> <ProjectName id="-" />
      </p>,
    );
    expect(screen.queryAllByRole("link")).toHaveLength(0);
  });
});

describe("the opt-out", () => {
  it("link={false} renders the same words as plain text, for a page's own subject", () => {
    render(
      <p>
        <UserName id="u1" link={false} /> <ProjectName id="p1" link={false} /> <BundleName id="b1" link={false} />{" "}
        <RoleRef projectId="p1" roleKey="admin" link={false} /> <UserName id="ghost" link={false} />
      </p>,
    );
    expect(screen.queryAllByRole("link")).toHaveLength(0);
    for (const words of ["Aanya Rao", "Laser Lab", "Lab Tech", "Unknown account"]) {
      expect(screen.getAllByText(words).length).toBeGreaterThan(0);
    }
  });

  it("a dialog makes names plain by default, and an explicit link still wins", () => {
    render(
      <Modal open onClose={() => {}} title="Remove access">
        <UserName id="u1" /> <ProjectName id="p1" link />
      </Modal>,
    );
    expect(screen.queryByRole("link", { name: "Aanya Rao" })).toBeNull();
    expect(screen.getByRole("link", { name: "Laser Lab" })).toBeInTheDocument();
  });
});

describe("the render-time half of the nesting guard", () => {
  it("says so when a name link lands inside another link or a button", () => {
    const err = vi.spyOn(console, "error").mockImplementation(() => {});
    render(
      <a href="/row">
        <UserName id="u1" />
      </a>,
    );
    render(
      <button>
        <ProjectName id="p1" />
      </button>,
    );
    expect(err.mock.calls.filter(([m]) => m === NESTED_NAME_LINK)).toHaveLength(2);
  });

  it("is silent for a name that carries link={false} inside a link", () => {
    const err = vi.spyOn(console, "error").mockImplementation(() => {});
    render(
      <a href="/row">
        <UserName id="u1" link={false} />
      </a>,
    );
    expect(err.mock.calls.filter(([m]) => m === NESTED_NAME_LINK)).toHaveLength(0);
  });
});
