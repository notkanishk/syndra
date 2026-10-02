import { useState } from "react";
import { Segmented } from "ui";

export const TwoOptions = () => {
  const [mode, setMode] = useState("manual");
  return (
    <Segmented
      label="When this rule applies"
      value={mode}
      onChange={setMode}
      options={[
        { value: "manual", label: "Waits under Pending changes" },
        { value: "auto", label: "Applies at once" },
      ]}
    />
  );
};

export const ThreeOptions = () => {
  const [depth, setDepth] = useState("roles");
  return (
    <Segmented
      label="How much to show"
      value={depth}
      onChange={setDepth}
      options={[
        { value: "spaces", label: "Spaces" },
        { value: "roles", label: "Roles" },
        { value: "people", label: "People" },
      ]}
    />
  );
};

export const Small = () => {
  const [format, setFormat] = useState("claims");
  return (
    <Segmented
      size="sm"
      label="Token format"
      value={format}
      onChange={setFormat}
      options={[
        { value: "claims", label: "Claims" },
        { value: "json", label: "Raw JSON" },
      ]}
    />
  );
};
