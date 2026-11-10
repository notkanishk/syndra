// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import { readdirSync, readFileSync, statSync } from "node:fs";
import { join } from "node:path";
import { describe, expect, it } from "vitest";

import { ListStates } from "@/components/states";
import { UiViewProvider } from "@/lib/ui-view";

function list(isEmpty: boolean) {
  return render(
    <QueryClientProvider client={new QueryClient()}>
      <UiViewProvider isOperator>
        <ListStates
          isLoading={false}
          error={null}
          isEmpty={isEmpty}
          errorTitle="Couldn't load people."
          head={<div>Person</div>}
          empty={<p>Nobody matches that.</p>}
        >
          <p>Roman Reigns</p>
        </ListStates>
      </UiViewProvider>
    </QueryClientProvider>,
  );
}

describe("column headings leave with the rows (A-20 P4)", () => {
  it("draws the headings with rows", () => {
    list(false);
    expect(screen.getByText("Person")).toBeTruthy();
    expect(screen.getByText("Roman Reigns")).toBeTruthy();
  });

  it("draws no headings over the empty state", () => {
    list(true);
    expect(screen.getByText("Nobody matches that.")).toBeTruthy();
    expect(screen.queryByText("Person")).toBeNull();
  });
});

function walk(dir: string): string[] {
  return readdirSync(dir).flatMap((name) => {
    const path = join(dir, name);
    if (statSync(path).isDirectory()) return name === "__tests__" ? [] : walk(path);
    return path.endsWith(".tsx") ? [path] : [];
  });
}

// The guard: a CardColumns that sits directly above a ListStates is a heading row
// that stays over the empty state. It goes in `head`, which ListStates owns.
describe("no page keeps headings outside ListStates", () => {
  it("never puts CardColumns straight above a ListStates", () => {
    const offenders = walk(join(process.cwd(), "src")).filter((file) => {
      const lines = readFileSync(file, "utf8").split("\n");
      return lines.some((line, i) => {
        if (line.trim() !== "</CardColumns>") return false;
        const next = lines.slice(i + 1).find((l) => l.trim() !== "" && l.trim() !== "<div className={view.className}>");
        return next?.trim() === "<ListStates";
      });
    });
    expect(offenders).toEqual([]);
  });
});
