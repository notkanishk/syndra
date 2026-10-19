"use client";

import { TermDatesCard } from "@/components/settings/TermDatesCard";
import { PageHeader } from "@/components/ui/PageHeader";

/**
 * Makerspace settings: choices that belong to the whole makerspace, not to one
 * person or one connected system. Operator-only by omission: the page is not in
 * `MEMBER_NAV`, so middleware refuses a member it, and the backend refuses
 * their write besides. One card today, the term dates; each setting here has
 * this page as its one write place and everywhere else reads it, with a link.
 */
export default function MakerspaceSettingsPage() {
  return (
    <div className="flex flex-col gap-7">
      <PageHeader
        title="Makerspace settings"
        lede="Settings that apply to the whole makerspace, not to one person or one connected system."
      />
      <TermDatesCard />
    </div>
  );
}
