import { redirect } from "next/navigation";

import GrantsClient from "@/components/grants/GrantsClient";
import { getSession } from "@/lib/session";

/**
 * Admin-only cross-source grants ledger and reconciliation diff. Server-side
 * gates non-admins to / so the page never even hydrates for member sessions.
 * Client island owns the data layer (Zitadel grants + reconciliation diff +
 * mapping rules in parallel).
 */
export default async function GrantsPage() {
  const session = await getSession();
  if (!session) {
    redirect("/login");
  }
  if (session.role !== "admin") {
    redirect("/");
  }
  return <GrantsClient />;
}
