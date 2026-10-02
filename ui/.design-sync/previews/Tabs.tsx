import { useState } from "react";
import { Tabs } from "ui";

export const PersonViews = () => {
  const [tab, setTab] = useState("access");
  return (
    <Tabs
      label="Views of this person's access"
      value={tab}
      onSelect={setTab}
      options={[
        { value: "access", label: "Access" },
        { value: "requests", label: "Requests" },
        { value: "activity", label: "Activity" },
      ]}
    />
  );
};

/** A count beside a tab label is the one place a number sits inside a control. */
export const WithCount = () => {
  const [tab, setTab] = useState("triage");
  return (
    <Tabs
      label="Views of unexplained access"
      value={tab}
      onSelect={setTab}
      options={[
        {
          value: "triage",
          label: (
            <>
              Needs a decision <span style={{ fontWeight: 600, color: "var(--danger-text)" }}>7</span>
            </>
          ),
        },
        { value: "reconciliation", label: "Side by side" },
      ]}
    />
  );
};
