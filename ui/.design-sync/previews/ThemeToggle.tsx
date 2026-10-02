import { AccountSheet, ThemeToggle } from "ui";

const operator = {
  id: "284019337710552064",
  name: "Priya Nair",
  email: "priya.nair@ashoka.edu.in",
  title: "Makerspace Manager",
  team: "Makerspace",
  status: "active",
  avatar: "PN",
  role: "admin" as const,
  sessionType: "oidc" as const,
};

/** Shows the sun in dark (switches to light) and the moon in light. */
export const Default = () => <ThemeToggle />;

/** Its home: the quiet neighbour of the account chip in the top bar. */
export const BesideAccount = () => (
  <div style={{ display: "flex", alignItems: "center", gap: 16 }}>
    <ThemeToggle />
    <AccountSheet session={operator} />
  </div>
);
