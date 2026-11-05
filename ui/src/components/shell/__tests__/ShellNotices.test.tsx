// @vitest-environment jsdom
import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { ShellNotices, ShellNoticesProvider } from "@/components/shell/ShellNotices";
import { PageHeader } from "@/components/ui/PageHeader";

vi.mock("@/components/states/OfflineBanner", () => ({
  OfflineBanner: ({ under }: { under?: boolean }) => <p data-testid="notice" data-under={String(!!under)}>offline</p>,
}));
vi.mock("@/components/states/DegradedBanner", () => ({ DegradedBanner: () => null }));

const follows = (a: Node, b: Node) => !!(a.compareDocumentPosition(b) & Node.DOCUMENT_POSITION_FOLLOWING);

/** A-20 P3 / G7: the shell's Notices sit under the page's header, in one seat. */
describe("the shell's Notices", () => {
  it("sit under the header and above the content, never above the title", () => {
    render(
      <ShellNoticesProvider>
        <PageHeader title="People" lede="Everyone with access." />
        <p>content</p>
      </ShellNoticesProvider>,
    );
    const notice = screen.getByTestId("notice");
    expect(follows(screen.getByRole("heading"), screen.getByText("Everyone with access."))).toBe(true);
    expect(follows(screen.getByText("Everyone with access."), notice)).toBe(true);
    expect(follows(notice, screen.getByText("content"))).toBe(true);
    expect(notice.dataset.under).toBe("true");
  });

  it("are said once, in one seat, when a page mounts two headers", () => {
    render(
      <ShellNoticesProvider>
        <PageHeader title="One" />
        <PageHeader title="Two" />
      </ShellNoticesProvider>,
    );
    expect(screen.getAllByTestId("notice")).toHaveLength(1);
  });

  it("fall back to the top of the content for a page with no header, so an outage is never silent", () => {
    render(
      <ShellNoticesProvider>
        <p>bare page</p>
      </ShellNoticesProvider>,
    );
    const notice = screen.getByTestId("notice");
    expect(notice.dataset.under).toBe("false");
    expect(follows(notice, screen.getByText("bare page"))).toBe(true);
  });

  it("an outlet outside the provider renders nothing rather than throwing", () => {
    render(<ShellNotices />);
    expect(screen.queryByTestId("notice")).toBeNull();
  });
});
