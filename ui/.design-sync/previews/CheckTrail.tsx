import { CheckTrail } from "ui";

export const Checks = () => (
  <CheckTrail
    checks={[
      { tone: "healthy", sentence: "Aanya signed in with Google." },
      { tone: "healthy", sentence: "Syndra knows her as `aanya@example.edu`." },
      { tone: "warn", sentence: "One role is still waiting.", faint: ["It reaches the app once you send it."], link: { label: "Pending changes", href: "/governance/pending" } },
      { tone: "neutral", sentence: "The app has not asked yet.", ring: true },
    ]}
  />
);

export const Compact = () => (
  <CheckTrail
    compact
    checks={[
      { tone: "healthy", sentence: "Aanya signed in with Google." },
      { tone: "danger", sentence: "The app refused the token.", faint: ["It expects a roles claim."] },
    ]}
  />
);
