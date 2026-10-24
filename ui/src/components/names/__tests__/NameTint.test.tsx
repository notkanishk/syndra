// @vitest-environment jsdom
import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { BundleName } from "@/components/names/BundleName";
import { ProjectName } from "@/components/names/ProjectName";
import { UserName } from "@/components/names/UserName";

const names = vi.hoisted(() => ({ resolved: false }));

vi.mock("@/lib/queries/useNameResolver", () => ({
  useNameResolver: () => {
    const read = (name: string) =>
      names.resolved ? { value: { display_name: name, name }, resolved: true } : { value: undefined, resolved: false };
    return { resolveUser: () => read("Aanya Rao"), resolveProject: () => read("Laser Lab"), resolveBundle: () => read("Lab Tech") };
  },
}));

const CASES = [
  ["a person", "Aanya Rao", () => <UserName id="u1" />],
  ["a project", "Laser Lab", () => <ProjectName id="p1" />],
  ["a bundle", "Lab Tech", () => <BundleName id="b1" />],
] as const;

/** S7: a name that lands after its skeleton tints in; one that was there is simply there. */
describe("names after their skeleton", () => {
  beforeEach(() => {
    names.resolved = false;
  });

  it.each(CASES)("%s tints in once resolved", (_, name, at) => {
    const { container, rerender } = render(at());
    expect(container.querySelector("[aria-busy]")).not.toBeNull();
    names.resolved = true;
    rerender(at());
    expect(screen.getByText(name)).toHaveClass("tint-in");
  });

  it.each(CASES)("%s that never was a skeleton does not", (_, name, at) => {
    names.resolved = true;
    render(at());
    expect(screen.getByText(name)).not.toHaveClass("tint-in");
  });
});
