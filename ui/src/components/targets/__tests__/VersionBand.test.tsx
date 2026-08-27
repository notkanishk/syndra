// @vitest-environment jsdom
import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { VersionBand } from "@/components/targets/VersionBand";
import type { MappingChange, MappingHistory } from "@/lib/queries/useMappings";

/**
 * The version band (design M1, M2, M5).
 *
 * Two properties it exists for. It keeps one shape across all three readings,
 * because a band that appeared with the first unpublished edit would be
 * structure moving in response to data on the one screen where an operator most
 * needs to know whether what they are reading is what a rollback would restore.
 *
 * And when there are unpublished edits it ENUMERATES them. That is the argument
 * for the screen: "rolling back undoes work listed nowhere" is true only while
 * nothing lists it.
 */

vi.mock("@/lib/queries/useMappings", async () => {
  const actual = await vi.importActual<typeof import("@/lib/queries/useMappings")>(
    "@/lib/queries/useMappings",
  );
  return {
    ...actual,
    usePublishMappingVersion: () => ({ mutate: vi.fn(), isPending: false, error: null }),
  };
});

vi.mock("@/components/names", () => ({
  UserName: ({ id, fallback }: { id: string; fallback?: string }) => <span>{id || fallback}</span>,
  RoleRef: ({ projectId, roleKey }: { projectId: string; roleKey: string }) => (
    <span>
      {projectId} / {roleKey}
    </span>
  ),
}));

function change(over: Partial<MappingChange> = {}): MappingChange {
  return {
    kind: "changed",
    project_id: "p_archive",
    role_key: "admin",
    field: "group",
    value: "archive-write",
    was_value: "archive-admins",
    actor: "a.devi",
    at: "2026-08-25T09:00:00Z",
    holders: 3,
    ...over,
  };
}

function history(over: Partial<MappingHistory> = {}): MappingHistory {
  return {
    target: "truenas",
    current_version: 4,
    unpublished: false,
    unpublished_changes: [],
    versions: [
      {
        version: 4,
        note: "summer build cohort",
        published_by: "a.devi",
        published_at: "2026-08-05T09:00:00Z",
        entries: [],
      },
    ],
    ...over,
  };
}

beforeEach(() => vi.clearAllMocks());

describe("the version band keeps one shape", () => {
  const publishButton = () => screen.getByRole("button", { name: /Publish/ });

  it("offers Publish inert rather than absent when there is nothing to publish", () => {
    render(<VersionBand target="truenas" history={history()} />);

    expect(screen.getByText(/Working copy matches version 4/)).toBeTruthy();
    expect(publishButton().hasAttribute("disabled")).toBe(true);
  });

  it("keeps the version count in its seat when nothing has been published", () => {
    render(<VersionBand target="truenas" history={history({ versions: [], current_version: 0 })} />);

    expect(screen.getByText("Nothing has been published")).toBeTruthy();
    expect(screen.getByText("0")).toBeTruthy();
    expect(publishButton().hasAttribute("disabled")).toBe(true);
  });

  it("names the version it would become once there is something to publish", () => {
    render(
      <VersionBand target="truenas" history={history({ unpublished: true, unpublished_changes: [change()] })} />,
    );

    expect(screen.getByRole("button", { name: "Publish as version 5" })).toBeTruthy();
  });
});

describe("the band enumerates what a rollback would undo", () => {
  it("names each edit, who made it, and how many people it moved", () => {
    render(
      <VersionBand
        target="truenas"
        history={history({
          unpublished: true,
          unpublished_changes: [
            change(),
            change({ kind: "added", project_id: "p_laser", role_key: "operator", value: "laser-users", was_value: undefined, holders: 12 }),
          ],
        })}
      />,
    );

    expect(screen.getByText("Changed")).toBeTruthy();
    expect(screen.getByText("Added")).toBeTruthy();
    expect(screen.getByText(/3 people/)).toBeTruthy();
    expect(screen.getByText(/12 people/)).toBeTruthy();
    expect(screen.getAllByText("a.devi").length).toBeGreaterThan(0);
  });

  // The sentence an operator needs most, because every other tool has taught
  // them the opposite: unpublished usually means not yet in effect, and here it
  // is exactly backwards.
  it("says that publishing does not re-apply what is already live", () => {
    render(
      <VersionBand target="truenas" history={history({ unpublished: true, unpublished_changes: [change()] })} />,
    );

    expect(
      screen.getByText(/Publishing does not re-apply them/),
    ).toBeTruthy();
  });

  // A deleted row takes its `updated_by` with it and nothing records the
  // deletion. Naming the version's publisher would name somebody who did not
  // remove it; an empty space would read as a rendering fault.
  it("says nothing records who removed a mapping, rather than leaving a gap", () => {
    render(
      <VersionBand
        target="truenas"
        history={history({
          unpublished: true,
          unpublished_changes: [
            change({ kind: "removed", value: undefined, actor: undefined, at: undefined, holders: 61 }),
          ],
        })}
      />,
    );

    expect(screen.getByText("Removed")).toBeTruthy();
    expect(screen.getByText(/Nothing records who removed it/)).toBeTruthy();
    expect(screen.getByText(/61 people/)).toBeTruthy();
  });

  it("shows no list at all when the working copy matches", () => {
    render(<VersionBand target="truenas" history={history()} />);

    expect(screen.queryByText(/what a rollback to version 4 would undo/)).toBeNull();
    expect(screen.queryByText(/Publishing does not re-apply them/)).toBeNull();
  });
});

/**
 * The band said a note was required and published without one.
 *
 * The note is the only record of why this set was the right one, and its whole
 * reader is somebody months later deciding whether to roll back to it. A blank
 * one makes the version a date with no argument — which is precisely what the
 * version history exists to stop being necessary.
 */
describe("the note is the audit record, so it is required", () => {
  it("will not publish without one, and says why", () => {
    render(
      <VersionBand
        target="truenas"
        history={history({ unpublished: true, unpublished_changes: [change()] })}
      />,
    );

    const publish = screen.getByRole("button", { name: /Publish as version 5/ });
    expect(publish.hasAttribute("disabled")).toBe(true);
    expect(screen.getByText(/It is what the next operator reads/)).toBeTruthy();
  });

  it("offers it once a reason is given", () => {
    render(
      <VersionBand
        target="truenas"
        history={history({ unpublished: true, unpublished_changes: [change()] })}
      />,
    );

    fireEvent.change(screen.getByPlaceholderText(/Why this set is the one to keep/), {
      target: { value: "summer build cohort" },
    });
    expect(
      screen.getByRole("button", { name: /Publish as version 5/ }).hasAttribute("disabled"),
    ).toBe(false);
  });
});
