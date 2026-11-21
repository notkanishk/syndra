// @vitest-environment jsdom
import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { OpensNote } from "@/components/kinds/storage/MappingReach";

// A last read with no time behind it (Go's zero time, now never sent) says when nothing: never "at —".
describe("what each group opens, as last read", () => {
  it("names no time it was not given", () => {
    render(<OpensNote name="TrueNAS" reach={{ read: true, current: false }} />);
    expect(screen.getByText(/as last read from TrueNAS's folder permissions/).textContent).not.toMatch(/ at /);
  });

  it("names the time it was given", () => {
    render(<OpensNote name="TrueNAS" reach={{ read: true, current: false, read_at: "2026-11-21T09:00:00Z" }} />);
    expect(screen.getByText(/as last read from TrueNAS's folder permissions/).textContent).toMatch(/ at \d\d:\d\d/);
  });
});
