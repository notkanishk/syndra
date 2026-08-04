// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { TokenFormatEditor } from "@/components/apps/TokenFormatEditor";
import type { ClaimShape } from "@/lib/queries/useClaimShape";

const saveProject = vi.hoisted(() => vi.fn().mockResolvedValue({}));
const saveOverride = vi.hoisted(() => vi.fn().mockResolvedValue({}));
const dropOverride = vi.hoisted(() => vi.fn().mockResolvedValue({}));

vi.mock("@/lib/queries/useClaimShape", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@/lib/queries/useClaimShape")>();
  return {
    ...actual,
    useClaimVocabulary: () => ({
      data: {
        attributes: ["user_id", "email", "team"],
        formats: ["array", "csv", "space_delimited"],
      },
    }),
    useSaveProjectClaimProfile: () => ({ mutateAsync: saveProject, isPending: false }),
    useSaveAppClaimOverride: () => ({ mutateAsync: saveOverride, isPending: false }),
    useDeleteAppClaimOverride: () => ({ mutateAsync: dropOverride, isPending: false }),
  };
});

const shape: ClaimShape = {
  project_id: "pLaser",
  project_name: "Laser Lab",
  default: {
    project_id: "pLaser",
    claim_name: "syndra.laser.roles",
    format_type: "array",
    attribute_claims: { "syndra.laser.email": "email" },
    static_claims: {},
  },
  overrides: [
    {
      project_id: "pLaser",
      application_id: "app_badge",
      application_name: "Badge Reader",
      claim_name: "badge.roles",
      format_type: "csv",
    },
  ],
  applications: [],
  emitted_keys: [
    { key: "badge.roles", owner_label: "Badge Reader", application_id: "app_badge", kind: "roles" },
    {
      key: "syndra.laser.email",
      owner_label: "Project default",
      kind: "attribute",
      source: "email",
    },
    { key: "syndra.laser.roles", owner_label: "Project default", kind: "roles" },
  ],
  conflicts: [],
};

function renderEditor(applicationId: string, override: Partial<ClaimShape> = {}) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const query = {
    data: { ...shape, ...override },
    isLoading: false,
    error: null,
    refetch: vi.fn(),
  } as never;

  return render(
    <QueryClientProvider client={client}>
      <TokenFormatEditor
        projectId="pLaser"
        applicationId={applicationId}
        applicationName={applicationId === "app_badge" ? "Badge Reader" : "Bookings"}
        shape={query}
        siblingCount={1}
      />
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  saveProject.mockClear();
  saveOverride.mockClear();
});

describe("token format editor", () => {
  it("states that the project default changes every app reading the project", () => {
    renderEditor("app_bookings");
    expect(screen.getByText(/apps reading Laser Lab receive/i)).toBeInTheDocument();
  });

  it("saves the claim name and format an operator actually typed", async () => {
    renderEditor("app_bookings");

    const claimName = screen.getByPlaceholderText("syndra.laser.roles");
    fireEvent.change(claimName, { target: { value: "laser.roles.v2" } });
    fireEvent.click(screen.getByRole("radio", { name: "csv" }));
    fireEvent.click(screen.getByRole("button", { name: "Save token format" }));

    expect(saveProject).toHaveBeenCalledWith(
      expect.objectContaining({ claim_name: "laser.roles.v2", format_type: "csv" }),
    );
  });

  it("keeps Save inert until something actually changed", () => {
    renderEditor("app_bookings");
    expect(screen.getByRole("button", { name: "Save token format" })).toBeDisabled();
  });

  it("round-trips an attribute claim rather than dropping it on save", () => {
    renderEditor("app_bookings");

    // The existing attribute claim is rendered as an editable row…
    expect(screen.getByDisplayValue("syndra.laser.email")).toBeInTheDocument();

    fireEvent.change(screen.getByPlaceholderText("syndra.laser.roles"), {
      target: { value: "laser.roles" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Save token format" }));

    // …and survives a save that was about something else entirely.
    expect(saveProject).toHaveBeenCalledWith(
      expect.objectContaining({ attribute_claims: { "syndra.laser.email": "email" } }),
    );
  });

  it("shows every key the token carries and who owns each one", () => {
    renderEditor("app_badge");

    expect(screen.getByText("A token for this project carries")).toBeInTheDocument();
    expect(screen.getByText("badge.roles")).toBeInTheDocument();
    // A sibling's key is present and attributed — an operator should learn
    // that here, not by decoding a production token.
    expect(screen.getByText("syndra.laser.roles")).toBeInTheDocument();
    expect(screen.getAllByText("Project default").length).toBeGreaterThan(0);
  });

  it("names a duplicate claim key as the collision it is", () => {
    renderEditor("app_badge", {
      conflicts: [
        { claim_key: "roles", owner: "Badge Reader", other: "the project default" },
      ],
    });

    expect(screen.getByText(/is claimed twice/i)).toBeInTheDocument();
    expect(screen.getByText(/one value per name/i)).toBeInTheDocument();
  });

  it("offers an app without an override the project default, and a way off it", () => {
    renderEditor("app_bookings");
    fireEvent.click(screen.getByRole("radio", { name: "Bookings only" }));

    expect(screen.getByText("Bookings uses the project default.")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: /Give Bookings its own claim/ }));
    expect(saveOverride).toHaveBeenCalledWith(
      expect.objectContaining({ applicationId: "app_bookings", claim_name: "bookings.roles" }),
    );
  });
});
