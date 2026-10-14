import { useState } from "react";
import { FilterPills } from "ui";

export const Status = () => {
  const [value, setValue] = useState("open");
  return (
    <FilterPills
      label="Filter requests by status"
      value={value}
      onChange={setValue}
      options={[
        { value: "open", label: "Open" },
        { value: "approved", label: "Approved" },
        { value: "declined", label: "Declined" },
        { value: "all", label: "All" },
      ]}
    />
  );
};

export const Source = () => {
  const [value, setValue] = useState("all");
  return (
    <FilterPills
      label="Filter changes by source"
      value={value}
      onChange={setValue}
      options={[
        { value: "all", label: "Everything" },
        { value: "syndra", label: "Made here" },
        { value: "zitadel", label: "Made in Zitadel" },
      ]}
    />
  );
};
