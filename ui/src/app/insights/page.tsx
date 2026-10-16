"use client";

import { TrendsTab } from "@/components/insights/TrendsTab";
import { MapTab } from "@/components/insights/MapTab";
import { useInsightsUrl } from "@/components/insights/useInsightsUrl";
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
      <div className="flex flex-col gap-3.5 pb-3">
        <PageHeader
          title="Insights"
          lede="How access travels through the makerspace, and how that is changing."
        />
        <Tabs
          label="Views of the makerspace’s access"
          value={url.tab}
          onSelect={(tab) => url.set({ tab: tab === "trends" ? "trends" : null })}
          options={[
            { value: "map", label: "Map" },
            { value: "trends", label: "Trends" },
          ]}
        />
      </div>

      {url.tab === "trends" ? <TrendsTab url={url} /> : <MapTab url={url} />}
    </div>
  );
}
