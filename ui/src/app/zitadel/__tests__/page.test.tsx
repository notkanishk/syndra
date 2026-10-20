// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import IdentityProviderPage from "@/app/zitadel/page";

const rotation = vi.hoisted(() => ({ value: {} as Record<string, unknown> }));
const health = vi.hoisted(() => ({ value: {} as Record<string, unknown> }));
const projects = vi.hoisted(() => ({ value: [] as unknown[] }));

// The page reads rotation status through the raw request helper and health
// through its own hook, so both are stubbed at their respective seams.
vi.mock("@/lib/api-client", () => ({
  request: async () => rotation.value,
}));

vi.mock("@/lib/queries/useZitadel", () => ({
  useZitadelHealth: () => ({ data: health.value, isLoading: false, error: null, refetch: () => {} }),
}));

vi.mock("@/lib/queries/useProjects", () => ({
  useProjects: () => ({ data: projects.value, isLoading: false, error: null }),
}));

function renderPage() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <IdentityProviderPage />
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  health.value = { status: "ok", mode: "live", latency_ms: 21, domain: "auth.example.org" };
  projects.value = [];
  rotation.value = {
    key_installed: true,
    status: "ok",
    age_days: 12,
    threshold_days: 90,
    last_rotated_at: "2026-07-01T00:00:00Z",
    rotate_command: "make zitadel-actions-rotate-key",
  };
});

/**
 * (1) One page, one fact about reachability — not two cards computing it
 * differently.
 *
 * The "Connection" tile used to key off `mode` alone, which reads "live" for
 * both a healthy connection and a live one whose last call just failed — so a
 * real outage showed "Connected" here directly under "Unreachable" in the
 * banner above it. These assert the tile always agrees with the banner's
 * verdict, for every reachability state the backend can report.
 */
describe("Identity provider · one verdict about reachability", () => {
  it("calls it 'Not connected' everywhere when Zitadel was never set up", async () => {
    health.value = {
      status: "disabled",
      mode: "local-policy-only",
      error: "Management client not initialized — check ZITADEL_DOMAIN, ZITADEL_MACHINE_KEY_PATH, and backend startup logs",
    };
    renderPage();
    expect(await screen.findAllByText(/Not connected/)).not.toHaveLength(0);
    expect(screen.queryByText(/^Unreachable/)).not.toBeInTheDocument();
    expect(screen.queryByText("Connected")).not.toBeInTheDocument();
  });

  it("calls it 'Unreachable' everywhere when Zitadel is configured but did not answer — never 'Connected'", async () => {
    health.value = { status: "error", mode: "live", error: "dial tcp: connection refused" };
    renderPage();
    expect(await screen.findAllByText(/Unreachable/)).not.toHaveLength(0);
    expect(screen.queryByText("Connected")).not.toBeInTheDocument();
    expect(screen.queryByText(/^Not connected/)).not.toBeInTheDocument();
  });
});

/**
 * Health is one line with its detail folded (route-zitadel, 7c). The project
 * count sits in the header and names its source: Zitadel's own number once it
 * answers, Syndra's remembered one — marked last known — when it can't.
 */
describe("Identity provider · health line and its fold", () => {
  it("is one line with the reply time, and the detail is folded until asked for", async () => {
    health.value = { status: "ok", mode: "live", latency_ms: 42, projects_total: 6 };
    renderPage();
    expect(await screen.findByText("Answering")).toBeInTheDocument();
    expect(screen.getByText("replied in 42 ms")).toBeInTheDocument();
    const toggle = screen.getByRole("button", { name: "Show checks" });
    expect(toggle).toHaveAttribute("aria-expanded", "false");
    expect(screen.queryByText("As Zitadel reported them just now.")).not.toBeInTheDocument();

    fireEvent.click(toggle);
    expect(screen.getByRole("button", { name: "Hide checks" })).toHaveAttribute("aria-expanded", "true");
    expect(screen.getByText(/6 in Zitadel, as Zitadel reported them just now/)).toBeInTheDocument();
  });

  it("counts projects as Zitadel reported them once the connection is healthy", async () => {
    health.value = { status: "ok", mode: "live", projects_total: 6 };
    projects.value = [{ id: "cached-only" }];
    renderPage();
    expect(await screen.findByText("6 projects")).toBeInTheDocument();
  });

  it("falls back to Syndra's remembered count, marked last known, and has nothing to fold", async () => {
    health.value = { status: "error", mode: "live", error: "dial tcp: connection refused" };
    projects.value = [{ id: "p1" }, { id: "p2" }, { id: "p3" }];
    renderPage();
    expect(await screen.findByText("3 projects, last known")).toBeInTheDocument();
    expect(screen.getByText("Not answering")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /checks/ })).not.toBeInTheDocument();
  });
});

/**
 * The signing key is one compact row: a word and a sentence, and the guide
 * behind a button. The sentence reads as a fact while there is nothing to do
 * and only turns into an instruction once the threshold is crossed.
 */
describe("Identity provider · signing key row", () => {
  it("says it is within its limit, in words, with no instruction", async () => {
    renderPage();
    expect(await screen.findByText("Within its limit")).toBeInTheDocument();
    expect(screen.getByText(/12 days old, within the 90-day limit. Nothing to do./)).toBeInTheDocument();
    expect(screen.queryByText(/Replace it now|Plan to replace/)).not.toBeInTheDocument();
  });

  it("turns into an instruction once the threshold is crossed, and says so above the page", async () => {
    rotation.value = {
      key_installed: true,
      status: "warn",
      age_days: 104,
      threshold_days: 90,
      last_rotated_at: "2026-01-01T00:00:00Z",
    };
    renderPage();
    expect(await screen.findByText("Past its limit")).toBeInTheDocument();
    expect(screen.getByText(/104 days old, over the 90-day limit. Plan to replace it./)).toBeInTheDocument();
    expect(screen.getByText("The action signing key is 104 days old.")).toBeInTheDocument();
  });

  it("calls a stale key stale rather than reporting a number", async () => {
    rotation.value = {
      key_installed: true,
      status: "stale",
      age_days: 220,
      threshold_days: 90,
      rotate_command: "make zitadel-actions-rotate-key",
    };
    renderPage();
    expect(await screen.findByText(/more than twice the 90-day limit/)).toBeInTheDocument();
  });

  // "disabled" reads like "not set up yet" and means every inbound Action
  // request is being trusted unchecked. The copy has to say the second thing.
  it("treats a missing key as verification being off", async () => {
    rotation.value = { key_installed: false, status: "disabled", threshold_days: 90 };
    renderPage();
    expect(await screen.findByText(/accepts every request that claims to come from Zitadel without checking/)).toBeInTheDocument();
    expect(screen.getByText("Not set up")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "How to set it up" })).toBeInTheDocument();
  });
});

describe("Identity provider · signing key guide", () => {
  it("keeps the command out of the row until 'How to replace it' is pressed", async () => {
    renderPage();
    const button = await screen.findByRole("button", { name: "How to replace it" });
    expect(button).toHaveAttribute("aria-expanded", "false");
    expect(screen.queryByText("make zitadel-actions-rotate-key")).not.toBeInTheDocument();

    fireEvent.click(button);
    expect(await screen.findByText("make zitadel-actions-rotate-key")).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: /Copy command: make zitadel-actions-rotate-key/ }),
    ).toBeInTheDocument();
  });

  // The command alone is half the job: rotation that Zitadel accepts and the
  // backend never picks up leaves every Action call failing verification.
  it("names the env swap and the restart that must follow it", async () => {
    renderPage();
    fireEvent.click(await screen.findByRole("button", { name: "How to replace it" }));
    expect(await screen.findByText(/ZITADEL_ACTION_SIGNING_KEY_ROTATED_AT/)).toBeInTheDocument();
    expect(screen.getByText(/docker compose restart backend/)).toBeInTheDocument();
    expect(screen.getByText(/access\s+details Syndra normally adds at sign-in are missing/)).toBeInTheDocument();
  });

  it("never offers to rotate the key itself: the only button opens the guide", async () => {
    renderPage();
    fireEvent.click(await screen.findByRole("button", { name: "How to replace it" }));
    await screen.findByText("make zitadel-actions-rotate-key");
    expect(screen.queryByRole("button", { name: /^Rotate/ })).not.toBeInTheDocument();
  });

  it("offers register, not rotate, when there is no key", async () => {
    rotation.value = { key_installed: false, status: "disabled", threshold_days: 90 };
    renderPage();
    fireEvent.click(await screen.findByRole("button", { name: "How to set it up" }));
    expect(await screen.findByText("make zitadel-actions-register")).toBeInTheDocument();
    expect(screen.queryByText("make zitadel-actions-rotate-key")).not.toBeInTheDocument();
  });
});

// The read-only doors used to be labelled by their routes ("/zitadel/projects"). A route is where
// the link goes, not what the row is.
describe("Identity provider · Look inside names each door", () => {
  it("gives each row a plain name and a line, and prints no route", async () => {
    renderPage();
    expect(await screen.findByText("Each with how it compares with Syndra's record")).toBeInTheDocument();
    expect(screen.getByText("Everyone Zitadel knows, and the roles it holds for each")).toBeInTheDocument();
    expect(screen.getByText("Every role on every person, in one list")).toBeInTheDocument();
    expect(screen.queryByText(/^\/zitadel\//)).not.toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Open people" })).toHaveAttribute("href", "/zitadel/users");
  });

  it("shows the names but no links while Zitadel is not answering, and says why", async () => {
    health.value = { status: "error", mode: "live", error: "dial tcp: connection refused" };
    renderPage();
    expect(await screen.findByText("Open people")).toBeInTheDocument();
    expect(screen.queryByRole("link", { name: "Open people" })).not.toBeInTheDocument();
    expect(screen.getByText(/Unavailable while Zitadel isn.t answering: these read it live/)).toBeInTheDocument();
  });
});

/**
 * #72: the write half sits behind its disclosure, with the warning beside each
 * control. #75: each warning is its own surface's truth — a person's roles are
 * recorded (they go through Pending changes), a project or role edit is not.
 */
describe("Identity provider · Change Zitadel directly", () => {
  it("is folded: no write control is on the page until it is opened", async () => {
    renderPage();
    const fold = await screen.findByRole("button", { name: /Change Zitadel directly/ });
    expect(fold).toHaveAttribute("aria-expanded", "false");
    expect(screen.getByText("last resort")).toBeInTheDocument();
    expect(screen.queryByRole("link", { name: /Change people.s roles/ })).not.toBeInTheDocument();
    expect(screen.queryByRole("link", { name: /Change projects and roles/ })).not.toBeInTheDocument();
  });

  it("puts a warning beside each control, each saying its own truth", async () => {
    renderPage();
    fireEvent.click(await screen.findByRole("button", { name: /Change Zitadel directly/ }));

    const people = screen.getByRole("link", { name: /Change people.s roles/ });
    expect(people).toHaveAttribute("href", "/zitadel/users");
    const peopleRow = people.closest(".row-divider") as HTMLElement;
    expect(within(peopleRow).getByText(/Recorded in Syndra, but with no preview and no reason/)).toBeInTheDocument();
    expect(within(peopleRow).queryByText(/keeps no record/)).not.toBeInTheDocument();

    const projectsLink = screen.getByRole("link", { name: /Change projects and roles/ });
    expect(projectsLink).toHaveAttribute("href", "/zitadel/projects");
    const projectsRow = projectsLink.closest(".row-divider") as HTMLElement;
    expect(within(projectsRow).getByText(/Goes straight to Zitadel, and Syndra keeps no record of it/)).toBeInTheDocument();
    expect(within(projectsRow).getByRole("link", { name: "Projects →" })).toHaveAttribute("href", "/projects");
  });

  it("disables both controls, with the reason, while Zitadel is not answering", async () => {
    health.value = { status: "error", mode: "live", error: "dial tcp: connection refused" };
    renderPage();
    fireEvent.click(await screen.findByRole("button", { name: /Change Zitadel directly/ }));
    expect(screen.getByRole("button", { name: /Change people.s roles/ })).toBeDisabled();
    expect(screen.getByRole("button", { name: /Change projects and roles/ })).toBeDisabled();
    expect(screen.getByText(/Unavailable while Zitadel isn.t answering: these change it directly/)).toBeInTheDocument();
  });
});
