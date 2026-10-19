// @vitest-environment jsdom
import { fireEvent, render, screen, within } from "@testing-library/react";
import { useState } from "react";
import { describe, expect, it, vi } from "vitest";

import { RolePicker } from "@/components/bundles/RolePicker";
import type { PickerPlace } from "@/components/bundles/RolePickerOne";

vi.mock("next/link", () => ({
  default: ({ href, children, ...rest }: { href: string; children: React.ReactNode }) => (
    <a href={href} {...rest}>
      {children}
    </a>
  ),
}));

const places: PickerPlace[] = [
  {
    id: "pLaser",
    name: "Laser Lab",
    roles: [
      { key: "operator", name: "Operator", description: "Runs the cutter", people: 4, note: { kind: "none" } },
      { key: "trainee", name: "Trainee", note: { kind: "held" } },
      { key: "keeper", name: "Keeper", note: { kind: "until", until: "2026-12-18T12:00:00Z" } },
    ],
  },
  {
    id: "pWood",
    name: "Woodshop",
    roles: [
      { key: "bandsaw", name: "Bandsaw", note: { kind: "asked", at: new Date(Date.now() - 2 * 3600_000).toISOString() } },
      { key: "lathe", name: "Lathe", note: { kind: "onway" } },
    ],
  },
  { id: "pEmpty", name: "Network storage", roles: [] },
];

function Harness({
  audience = "member",
  cameFrom,
  initial = null,
  data = places,
}: {
  audience?: "member" | "operator";
  cameFrom?: string;
  initial?: string | null;
  data?: PickerPlace[];
}) {
  const [value, setValue] = useState<string | null>(initial);
  return (
    <>
      <RolePicker mode="one" audience={audience} places={data} value={value} onChange={setValue} cameFrom={cameFrom} />
      <output data-testid="value">{value ?? ""}</output>
    </>
  );
}

const fold = (name: RegExp | string) => screen.getByRole("button", { name: new RegExp(typeof name === "string" ? name : name.source) });

describe("RolePicker · single choice", () => {
  it("rests with every place closed, each saying what it holds", () => {
    render(<Harness />);
    expect(fold("Laser Lab")).toHaveAttribute("aria-expanded", "false");
    expect(fold("Laser Lab")).toHaveTextContent("3 roles");
    expect(fold("Woodshop")).toHaveTextContent("2 roles");
    expect(fold("Network storage")).toHaveTextContent("Nothing yet");
    expect(screen.queryByRole("radio")).not.toBeInTheDocument();
  });

  it("opens the place a link named first, at the top, and nothing else", () => {
    render(<Harness cameFrom="pWood" />);
    const folds = screen.getAllByRole("button", { name: /roles?$|Nothing yet|chosen/ }).filter((b) => b.hasAttribute("aria-expanded"));
    expect(folds[0]).toHaveTextContent("Woodshop");
    expect(folds[0]).toHaveAttribute("aria-expanded", "true");
    expect(fold("Laser Lab")).toHaveAttribute("aria-expanded", "false");
  });

  it("picks one role with a radio seat, shows it as one chip, and counts it in its place", () => {
    render(<Harness />);
    fireEvent.click(fold("Laser Lab"));
    fireEvent.click(screen.getByRole("radio", { name: /Operator/ }));
    expect(screen.getByTestId("value")).toHaveTextContent("pLaser:operator");
    expect(fold("Laser Lab")).toHaveTextContent("1 chosen");
    expect(screen.getByRole("button", { name: "Remove Operator" })).toBeInTheDocument();
  });

  it("choosing another role moves the chip; the chip's × clears it", () => {
    render(<Harness initial="pLaser:operator" />);
    expect(fold("Laser Lab")).toHaveAttribute("aria-expanded", "true");
    fireEvent.click(screen.getByRole("button", { name: "Remove Operator" }));
    expect(screen.getByTestId("value")).toHaveTextContent("");
    expect(screen.queryByRole("button", { name: /^Remove/ })).not.toBeInTheDocument();
  });

  it("opens every fold with a match while searching, and says when nothing matches", () => {
    render(<Harness />);
    fireEvent.change(screen.getByRole("searchbox", { name: "Search roles" }), { target: { value: "bandsaw" } });
    expect(fold("Woodshop")).toHaveAttribute("aria-expanded", "true");
    expect(screen.getByRole("radio", { name: /Bandsaw/ })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /Laser Lab/ })).not.toBeInTheDocument();

    fireEvent.change(screen.getByRole("searchbox", { name: "Search roles" }), { target: { value: "zzz" } });
    expect(
      screen.getByText("Nothing you can ask for matches “zzz”. Ask for access, and say what you need in Why."),
    ).toBeInTheDocument();
  });

  it("a member reads what they have in the note seat, and held ones are ticked and can't be picked", () => {
    render(<Harness cameFrom="pLaser" />);
    expect(screen.getByText("You have this")).toBeInTheDocument();
    expect(screen.getByText("Yours until 18 Dec")).toBeInTheDocument();
    expect(screen.getByRole("radio", { name: /Trainee/ })).toBeDisabled();
    expect(screen.getByRole("radio", { name: /Keeper/ })).toBeEnabled();
    fireEvent.click(screen.getByRole("radio", { name: /Trainee/ }));
    expect(screen.getByTestId("value")).toHaveTextContent("");
  });

  it("a member reads Asked and On its way, and never a key, a count or the Waits mark", () => {
    render(<Harness cameFrom="pWood" />);
    expect(screen.getByText("Asked 2h ago")).toBeInTheDocument();
    expect(screen.getByText("On its way")).toBeInTheDocument();
    fireEvent.click(fold("Laser Lab"));
    expect(screen.queryByText("operator")).not.toBeInTheDocument();
    expect(screen.queryByText(/people/)).not.toBeInTheDocument();
    expect(screen.getByText("Runs the cutter")).toBeInTheDocument();
  });

  it("a held or locked role doesn't open its fold", () => {
    render(<Harness data={[{ id: "pA", name: "Safety", roles: [{ key: "basics", name: "Basics", note: { kind: "held" } }] }]} />);
    expect(fold("Safety")).toHaveAttribute("aria-expanded", "false");
    expect(fold("Safety")).toHaveTextContent("1 you have");
  });

  it("an empty place says so, once opened", () => {
    render(<Harness />);
    fireEvent.click(fold("Network storage"));
    expect(
      screen.getByText("Nothing is set up here yet, so there's nothing to ask for. Makerspace staff can set it up."),
    ).toBeInTheDocument();
  });

  it("an operator reads the key after the name, N people with no status, and what's given", () => {
    const operatorPlaces: PickerPlace[] = [
      {
        id: "pLaser",
        name: "Laser Lab",
        roles: [
          { key: "operator", name: "Operator", people: 4, note: { kind: "none" } },
          { key: "solo", name: "Solo", people: 1, note: { kind: "none" } },
          { key: "trainee", name: "Trainee", note: { kind: "held" } },
          { key: "sent", name: "Sent", note: { kind: "onway", at: new Date(Date.now() - 2 * 3600_000).toISOString() } },
        ],
      },
    ];
    render(<Harness audience="operator" cameFrom="pLaser" data={operatorPlaces} />);
    const row = screen.getByRole("radio", { name: /Operator/ }).closest("label")!;
    expect(within(row).getByText("operator")).toBeInTheDocument();
    expect(within(row).getByText("4 people")).toBeInTheDocument();
    expect(screen.getByText("1 person")).toBeInTheDocument();
    expect(screen.getByText("Has this")).toBeInTheDocument();
    expect(screen.getByText("Given 2h ago")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /Waiting for you to send/ })).toHaveAttribute("href", "/governance/pending");
  });

  it("an operator can't pick a role held with an end, and is offered Extend", () => {
    render(
      <Harness
        audience="operator"
        cameFrom="pLaser"
        data={[{ id: "pLaser", name: "Laser Lab", roles: [{ key: "keeper", name: "Keeper", note: { kind: "until", until: "2026-12-18T12:00:00Z" } }] }]}
      />,
    );
    expect(screen.getByText("Has this until 18 Dec")).toBeInTheDocument();
    expect(screen.getByRole("radio", { name: /Keeper/ })).toBeDisabled();
    expect(screen.getByRole("link", { name: /Extend/ })).toHaveAttribute("href", expect.stringContaining("/review/expiring-access"));
  });
});
