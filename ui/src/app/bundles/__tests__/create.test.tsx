// @vitest-environment jsdom
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import BundlesPage from "@/app/bundles/page";

/**
 * A bundle is created as the thing it was described as.
 *
 * Creating one with a name alone published an empty v1: assigning it granted
 * nothing, and the roles the operator then added came back to them as "2
 * unpublished changes" against a version that had never described anything.
 * There was no point in that sequence where the screen told the truth about a
 * bundle the operator considered finished.
 */

const state = vi.hoisted(() => ({ create: vi.fn() }));

// "Start from" reads another bundle's published roles; these two are what Mill Certified v2 carries.
const source = vi.hoisted(() => ({
  bundles: [{ id: "b-src", name: "Mill Certified", latest_version: 2 }],
  roles: [
    { bundle_id: "b-src", zitadel_project_id: "p1", zitadel_role_key: "laser_trained" },
    { bundle_id: "b-src", zitadel_project_id: "p1", zitadel_role_key: "cnc_trained" },
  ],
}));

const people = vi.hoisted(() => [
  { id: "u1", name: "Priya Nair", email: "priya@example.edu", title: "Student", team: "Operations" },
  { id: "u2", name: "Omar Haddad", email: "omar@example.edu", title: "", team: "" },
]);
vi.mock("@/lib/queries/useCatalogUsers", () => ({ useCatalogUsers: () => ({ data: people }) }));
vi.mock("@/lib/queries/useUsers", () => ({ useUserAccess: () => ({ data: undefined }) }));

vi.mock("@/lib/queries/useBundles", () => ({
  useBundles: () => ({ data: source.bundles, isLoading: false, refetch: vi.fn() }),
  useBundleRoles: (id: string | null) => ({ data: id ? source.roles : undefined, isLoading: false, refetch: vi.fn() }),
  useBundleImpact: () => ({ data: null, isLoading: false }),
  useCreateBundle: () => ({ mutateAsync: state.create, isPending: false }),
  useDeleteBundle: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useRemoveBundleRole: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useSetWelcomeBundle: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useUpdateBundle: () => ({ mutateAsync: vi.fn(), isPending: false }),
}));

vi.mock("@/lib/queries/useBundleVersions", () => ({
  useBundleDraft: () => ({ data: undefined }),
  draftChangeCount: () => 0,
}));

vi.mock("@/lib/queries/useMappingRules", () => ({ useMappingRules: () => ({ data: [] }) }));

vi.mock("@/lib/queries/useRoles", () => ({
  useGlobalRoleCatalog: () => ({
    data: [
      {
        project_id: "p1",
        project_name: "Fabrication",
        role_key: "laser_trained",
        display_name: "Laser trained",
      },
      {
        project_id: "p1",
        project_name: "Fabrication",
        role_key: "cnc_trained",
        display_name: "CNC trained",
      },
    ],
  }),
}));

vi.mock("@/components/names", () => ({
  RoleRef: ({ roleKey }: { roleKey: string }) => <span>{roleKey}</span>,
  UserName: ({ id }: { id: string }) => <span>{id}</span>,
}));

vi.mock("@/components/bundles/BundleVersions", () => ({ BundleVersions: () => null }));

beforeEach(() => {
  state.create = vi.fn().mockResolvedValue({ id: "b1" });
});

// RolePicker folds each project, closed until searched in or something in it is chosen.
function openDialog() {
  render(<BundlesPage />);
  fireEvent.click(screen.getByRole("button", { name: "New bundle" }));
  fireEvent.click(screen.getByRole("button", { name: /Fabrication/ }));
}

describe("creating a bundle", () => {
  it("will not create one with no roles, and says why", () => {
    openDialog();
    fireEvent.change(screen.getByLabelText("Name"), { target: { value: "Lab Tech" } });

    expect(screen.getByRole("button", { name: "Create bundle" })).toBeDisabled();
    expect(screen.getByText(/Pick at least one role/)).toBeInTheDocument();
    expect(state.create).not.toHaveBeenCalled();
  });

  it("will not create one with no name, and says why", () => {
    openDialog();
    fireEvent.click(screen.getByRole("checkbox", { name: /Laser trained/ }));

    expect(screen.getByRole("button", { name: /^Create with 1 role$/ })).toBeDisabled();
    expect(screen.getByText("A bundle needs a name.")).toBeInTheDocument();
  });

  it("sends the ticked roles, so v1 is already what was described", async () => {
    openDialog();
    fireEvent.change(screen.getByLabelText("Name"), { target: { value: "  Lab Tech  " } });
    fireEvent.click(screen.getByRole("checkbox", { name: /Laser trained/ }));
    fireEvent.click(screen.getByRole("checkbox", { name: /CNC trained/ }));

    const create = screen.getByRole("button", { name: /^Create with 2 roles$/ });
    expect(create).toBeEnabled();
    fireEvent.click(create);

    await waitFor(() => expect(state.create).toHaveBeenCalled());
    expect(state.create.mock.calls[0][0]).toMatchObject({
      name: "Lab Tech",
      roles: [
        { project_id: "p1", role_key: "laser_trained" },
        { project_id: "p1", role_key: "cnc_trained" },
      ],
    });
  });

  /**
   * `setOutcome` used to fire in the same breath as `onClose`, which unmounts
   * the dialog before "Lab Tech created" is ever painted — the operator never
   * saw it landed, let alone what it carries. The dialog now stays open on
   * success until Close is pressed.
   */
  it("shows what was created and stays open until Close is pressed", async () => {
    openDialog();
    fireEvent.change(screen.getByLabelText("Name"), { target: { value: "Lab Tech" } });
    fireEvent.click(screen.getByRole("checkbox", { name: /Laser trained/ }));
    fireEvent.click(screen.getByRole("button", { name: /^Create with 1 role$/ }));

    await waitFor(() => expect(screen.getByText(/Lab Tech created/)).toBeInTheDocument());
    expect(screen.getByText(/carries 1 role.*Nobody holds it yet/)).toBeInTheDocument();
    // The create action retires — a second click must not create it twice.
    expect(screen.queryByRole("button", { name: /^Create with 1 role$/ })).toBeNull();

    fireEvent.click(screen.getByRole("button", { name: "Close" }));
    expect(screen.queryByText(/Lab Tech created/)).toBeNull();
  });
});

describe("starting a new bundle from another", () => {
  function copyFromMillCertified() {
    openDialog();
    fireEvent.click(screen.getByRole("radio", { name: "Another bundle" }));
    fireEvent.change(screen.getByLabelText("Copy from which bundle"), { target: { value: "b-src" } });
  }

  it("ticks the published roles and says what was copied", async () => {
    copyFromMillCertified();

    await waitFor(() => expect(screen.getByRole("checkbox", { name: /Laser trained/ })).toBeChecked());
    expect(screen.getByRole("checkbox", { name: /CNC trained/ })).toBeChecked();
    expect(
      screen.getByText(
        (_, el) =>
          el?.tagName === "P" && /Copied Mill Certified v2: its 2 roles are ticked below\./.test(el.textContent ?? ""),
      ),
    ).toBeInTheDocument();
  });

  it("unticks them again with Start empty", async () => {
    copyFromMillCertified();
    await waitFor(() => expect(screen.getByRole("checkbox", { name: /Laser trained/ })).toBeChecked());

    fireEvent.click(screen.getByRole("button", { name: "Start empty" }));

    expect(screen.getByRole("checkbox", { name: /Laser trained/ })).not.toBeChecked();
    expect(screen.getByRole("checkbox", { name: /CNC trained/ })).not.toBeChecked();
    expect(screen.queryByText(/Copied Mill Certified/)).toBeNull();
  });
});

// People everywhere 1: who someone is reads "title · team", then email.
describe("starting a new bundle from a person", () => {
  it("names each person by title · team, then email, and drops what's empty", () => {
    openDialog();
    fireEvent.click(screen.getByRole("radio", { name: "A person" }));
    fireEvent.change(screen.getByLabelText("Choose a person"), { target: { value: "a" } });

    const priya = screen.getByText("Priya Nair").closest("label")!;
    expect(priya.textContent).toContain("Student · Operations");
    expect(priya.textContent).toContain("priya@example.edu");

    // Nothing known but the email: no empty line, no dash.
    const omar = screen.getByText("Omar Haddad").closest("label")!;
    expect(omar.textContent).toBe("Omar Haddadomar@example.edu");
  });
});
