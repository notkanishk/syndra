import { useEffect, useRef } from "react";
import { FilterPanel, FilterField, Select, FilterPills } from "ui";

/** FilterField only lives inside an open FilterPanel; this cell opens it on mount. */
export function InPanel() {
  const ref = useRef<HTMLDivElement>(null);
  useEffect(() => {
    ref.current?.querySelector<HTMLButtonElement>("button[aria-expanded]")?.click();
  }, []);
  return (
    <div ref={ref} style={{ display: "flex", justifyContent: "flex-end", alignItems: "flex-start", minHeight: 300 }}>
      <FilterPanel activeCount={1} onClear={() => {}}>
        <FilterField label="Space">
          <Select defaultValue="cnc" aria-label="Filter by space">
            <option value="">Any space</option>
            <option value="cnc">CNC router</option>
            <option value="electronics">Electronics bench</option>
          </Select>
        </FilterField>
        <FilterField label="Status">
          <FilterPills label="Filter by status" value="open" onChange={() => {}} options={[
            { value: "open", label: "Open" },
            { value: "approved", label: "Approved" },
            { value: "all", label: "All" },
          ]} />
        </FilterField>
      </FilterPanel>
    </div>
  );
}
