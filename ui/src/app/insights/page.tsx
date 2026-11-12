"use client";

import { TrendsTab } from "@/components/insights/TrendsTab";
import { MapTab } from "@/components/insights/MapTab";
import { useInsightsUrl } from "@/components/insights/useInsightsUrl";
import { InsightsUsage } from "@/components/usage/InsightsUsage";
import { INSIGHTS_TAB } from "@/components/usage/copy";
import { PageHeader } from "@/components/ui/PageHeader";
import { Tabs } from "@/components/ui/Tabs";

/**
 * Insights (N12): how access travels through the makerspace, and how that is
 * changing. Map opens first; Trends holds what a map cannot draw. The link
 * holds all of it (tab, period, date, question, view, pins, finding), so a link is an answer.
 */
export default function InsightsPage() {
  const url = useInsightsUrl();

  return (
    <div className="flex flex-col gap-4">
      <PageHeader title="Insights" lede="How access travels through the makerspace, and how that is changing." />
      <Tabs
        label="Views of the makerspace’s access"
        value={url.tab}
        onSelect={(tab) => url.set({ tab: tab === "map" ? null : tab })}
        options={[
          { value: "map", label: "Map" },
          { value: "trends", label: "Trends" },
          { value: "usage", label: INSIGHTS_TAB },
        ]}
      />

      {url.tab === "trends" ? <TrendsTab url={url} /> : url.tab === "usage" ? <InsightsUsage /> : <MapTab url={url} />}
    </div>
  );
}
