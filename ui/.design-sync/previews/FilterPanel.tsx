import { useEffect, useRef, useState } from "react";
import { FilterPanel, FilterField, Select, FilterPills } from "ui";

const right = { display: "flex", justifyContent: "flex-end" } as const;

export const NoneActive = () => (
  <div style={right}>
    <FilterPanel activeCount={0} onClear={() => {}}>
      <FilterField label="Project">
        <Select defaultValue="" aria-label="Filter by project"><option value="">Any project</option></Select>
      </FilterField>
    </FilterPanel>
  </div>
);

export const TwoActive = () => (
  <div style={right}>
    <FilterPanel activeCount={2} onClear={() => {}}>
      <FilterField label="Project">
        <Select defaultValue="laser" aria-label="Filter by project"><option value="laser">Laser lab</option></Select>
      </FilterField>
    </FilterPanel>
  </div>
);

/** Opens itself once mounted so the panel's contents can be seen. */
export function OpenPanel() {
  const ref = useRef<HTMLDivElement>(null);
  const [project, setProject] = useState("laser");
  const [attention, setAttention] = useState("expiring");
  useEffect(() => {
    ref.current?.querySelector<HTMLButtonElement>("button[aria-expanded]")?.click();
  }, []);
  return (
    <div ref={ref} style={{ ...right, alignItems: "flex-start", minHeight: 400 }}>
      <FilterPanel activeCount={2} onClear={() => {}}>
        <FilterField label="Project">
          <Select value={project} onChange={(e) => setProject(e.target.value)} aria-label="Filter by project">
            <option value="">Any project</option>
            <option value="laser">Laser lab</option>
            <option value="wood">Wood shop</option>
          </Select>
        </FilterField>
        <FilterField label="Needs attention">
          <Select value={attention} onChange={(e) => setAttention(e.target.value)} aria-label="Filter by what needs attention">
            <option value="">Anything</option>
            <option value="expiring">Access expiring soon</option>
            <option value="unexplained">Access with no record</option>
          </Select>
        </FilterField>
        <FilterField label="Account">
          <FilterPills label="Filter by account" value="active" onChange={() => {}} options={[
            { value: "active", label: "Active" },
            { value: "departed", label: "Left" },
            { value: "all", label: "All" },
          ]} />
        </FilterField>
      </FilterPanel>
    </div>
  );
}
