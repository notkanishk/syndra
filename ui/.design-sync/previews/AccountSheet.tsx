import { useEffect, useRef } from "react";
import { AccountSheet } from "ui";

const operator = {
  id: "284019337710552064",
  name: "Priya Nair",
  email: "priya.nair@example.edu",
  title: "Makerspace Manager",
  team: "Makerspace",
  status: "active",
  avatar: "PN",
  role: "admin" as const,
  sessionType: "oidc" as const,
};

/** Every naming source came up empty: the email names the human, never the id. */
const unnamed = { ...operator, id: "284019337710552077", name: "", email: "arjun.das@example.edu", avatar: "AD", role: "user" as const };

/** The chip that opens the sheet. The name shows from the tablet breakpoint up. */
export const Trigger = () => <AccountSheet session={operator} />;

export const TriggerEmailOnly = () => <AccountSheet session={unnamed} />;

/** Opens the sheet on mount by pressing its own trigger. */
export const Open = () => {
  const ref = useRef<HTMLDivElement>(null);
  useEffect(() => ref.current?.querySelector("button")?.click(), []);
  return <div ref={ref} style={{ minHeight: 420 }}><AccountSheet session={operator} /></div>;
};
