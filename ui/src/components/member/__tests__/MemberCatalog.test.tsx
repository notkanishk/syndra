// @vitest-environment jsdom
import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { MemberCatalog } from "@/components/member/MemberCatalog";
import type { ProjectCatalog } from "@/lib/types";

const R = (key: string, label: string, description: string) => ({ key, label, description });
const CATALOG: ProjectCatalog[] = [
  {
    id: "laser",
    name: "Laser Lab",
    kind: "space",
    description: "Two laser cutters in the laser bay",
    roles: [
      R("member", "Laser cutter · Member", "Book time on the laser cutter"),
      R("operator", "Laser cutter · Operator", "Run the laser cutter without a supervisor"),
      R("door", "Laser bay · Door", "Badge entry to the laser bay"),
      R("sup", "Laser cutter · Supervisor", "Sign others off on the laser cutter"),
    ],
  },
  {
    id: "metal",
    name: "Metal shop",
    kind: "space",
    description: "Welding and the metal lathe",
    roles: [
      R("weld", "Welding · Operator", "Use the MIG welder"),
      R("lathe", "Lathe · Operator", "Use the metal lathe"),
      R("door", "Metal shop · Door", "Badge entry to the metal shop"),
    ],
  },
  {
    id: "main",
    name: "Main lab",
    kind: "space",
    description: "The makerspace itself",
    roles: [R("front", "Front door", "Badge entry to the makerspace")],
  },
];

vi.mock("@/lib/queries/useCatalogUsers", () => ({
  useCatalogProjects: () => ({ isLoading: false, error: null, data: CATALOG }),
}));

const HELD = new Map([
  ["laser", new Set(["member", "operator", "door"])],
  ["main", new Set(["front"])],
]);

function fold(name: string) {
  return screen.getByText(name).closest("button")!;
}

describe("MemberCatalog header", () => {
  it("stacks the heading and caption instead of squeezing them into a row", () => {
    render(<MemberCatalog heldByProject={new Map()} />);
    const heading = screen.getByText("What else is here");
    const caption = screen.getByText(/Everything the makerspace offers/);
    expect(heading.parentElement).toBe(caption.parentElement);
    expect(heading.parentElement?.className ?? "").not.toMatch(/\bflex\b/);
  });
});

/** Rooms fold (52b): one row each, saying how much of it you have. */
describe("MemberCatalog folds", () => {
  it("counts each room from what you hold, folded until opened", () => {
    render(<MemberCatalog heldByProject={HELD} />);
    expect(fold("Laser Lab").textContent).toContain("You have 3 of 4");
    expect(fold("Metal shop").textContent).toContain("3 you can ask for");
    expect(fold("Main lab").textContent).toContain("You have all of it");
    expect(fold("Laser Lab").getAttribute("aria-expanded")).toBe("false");
    expect(screen.queryByText("Laser cutter · Supervisor")).toBeNull();

    fireEvent.click(fold("Laser Lab"));
    expect(fold("Laser Lab").getAttribute("aria-expanded")).toBe("true");
    expect(screen.getAllByText("You have this")).toHaveLength(3);
    expect(screen.getByText("Ask for this").closest("a")!.getAttribute("href")).toBe(
      "/requests?project=laser&role=sup",
    );
  });
});

/** One search: it opens rooms with a match, tints the match, and folds the rest. */
describe("MemberCatalog search", () => {
  it("opens the room at the welder and folds the rest with No match", () => {
    const { container } = render(<MemberCatalog heldByProject={HELD} />);
    fireEvent.click(fold("Laser Lab"));
    fireEvent.change(screen.getByLabelText("Search everything on offer"), { target: { value: "weld" } });

    expect(fold("Metal shop").getAttribute("aria-expanded")).toBe("true");
    expect(fold("Metal shop").textContent).toContain("1 match");
    expect(fold("Laser Lab").getAttribute("aria-expanded")).toBe("false");
    expect(fold("Laser Lab").textContent).toContain("No match");
    expect(fold("Main lab").textContent).toContain("No match");

    const hits = container.querySelectorAll("[data-match]");
    expect(hits).toHaveLength(1);
    expect(hits[0].textContent).toContain("Welding · Operator");
    expect(hits[0].className).toContain("bg-accent-soft");
    // Only the match is listed in an open room during a search.
    expect(screen.queryByText("Lathe · Operator")).toBeNull();
  });

  it("matches a room by its name", () => {
    render(<MemberCatalog heldByProject={HELD} />);
    fireEvent.change(screen.getByLabelText("Search everything on offer"), { target: { value: "main lab" } });
    expect(fold("Main lab").getAttribute("aria-expanded")).toBe("true");
    expect(screen.getByText("Front door")).toBeTruthy();
  });

  it("says when nothing matches and points at Ask for access; clearing folds everything back", () => {
    render(<MemberCatalog heldByProject={HELD} />);
    const search = screen.getByLabelText("Search everything on offer");
    fireEvent.change(search, { target: { value: "kiln" } });
    expect(
      screen.getByText(/Nothing you can ask for matches “kiln”\. Ask for access, and say what you need in Why\./),
    ).toBeTruthy();
    // A button, not an arrow link: it opens the asking, it doesn't go anywhere to read.
    const ask = screen.getByText("Ask for access").closest("a")!;
    expect(ask.getAttribute("href")).toBe("/requests?ask");
    expect(ask.textContent).toBe("Ask for access");
    expect(ask.className).toContain("rounded-pill");

    fireEvent.change(search, { target: { value: "" } });
    expect(screen.queryByText(/Nothing you can ask for matches/)).toBeNull();
    expect(fold("Laser Lab").textContent).toContain("You have 3 of 4");
    for (const name of ["Laser Lab", "Metal shop", "Main lab"]) {
      expect(fold(name).getAttribute("aria-expanded")).toBe("false");
    }
  });
});
