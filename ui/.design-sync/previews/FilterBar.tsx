import { useState } from "react";
import { FilterBar, Input, FilterPanel, FilterField, Select, SelectModeToggle, Tabs, FilterPills } from "ui";

function Search({ placeholder }: { placeholder: string }) {
  const [q, setQ] = useState("");
  return <Input value={q} onChange={(e) => setQ(e.target.value)} placeholder={placeholder} aria-label={placeholder} />;
}

/** People: search, the panel, and selection mode trailing. */
export const People = () => {
  const [bulk, setBulk] = useState(false);
  return (
    <FilterBar
      search={<Search placeholder="Search name, email or role key…" />}
      filters={
        <FilterPanel activeCount={2} onClear={() => {}}>
          <FilterField label="Project">
            <Select defaultValue="laser" aria-label="Filter by project"><option value="laser">Laser lab</option></Select>
          </FilterField>
        </FilterPanel>
      }
      trailing={<SelectModeToggle active={bulk} onToggle={() => setBulk(!bulk)} />}
    />
  );
};

/** Tabs lead because they decide what the page is. */
export const WithTabs = () => {
  const [tab, setTab] = useState("triage");
  return (
    <FilterBar
      leading={
        <Tabs label="Views of unexplained access" value={tab} onSelect={setTab} options={[
          { value: "triage", label: "Needs a decision" },
          { value: "reconciliation", label: "Side by side" },
        ]} />
      }
      search={<Search placeholder="Search by person or role…" />}
      filters={
        <FilterPanel activeCount={0} onClear={() => {}}>
          <FilterField label="Project">
            <Select defaultValue="" aria-label="Filter by project"><option value="">Any project</option></Select>
          </FilterField>
        </FilterPanel>
      }
    />
  );
};

/** One narrowing control stays on the row instead of behind a panel. */
export const OneControl = () => {
  const [status, setStatus] = useState("open");
  return (
    <FilterBar
      search={<Search placeholder="Search requests…" />}
      filters={
        <FilterPills label="Filter requests by status" value={status} onChange={setStatus} options={[
          { value: "open", label: "Open" },
          { value: "approved", label: "Approved" },
          { value: "declined", label: "Declined" },
          { value: "all", label: "All" },
        ]} />
      }
    />
  );
};
