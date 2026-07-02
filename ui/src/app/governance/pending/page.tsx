import { redirect } from "next/navigation";

import { PendingPropagationsClient } from "@/components/propagation/PendingPropagationsClient";
import { getSession } from "@/lib/session";

/**
 * Operator worklist for the Zitadel propagation outbox: the MkAuth-mediated
 * grant changes buffered and awaiting an explicit drain to Zitadel (B4/D3).
 * Server-gated to admins; the client island owns the data + drain action.
 */
export default async function PendingPropagationsPage() {
  const session = await getSession();
  if (!session) {
    redirect("/login");
  }
  if (session.role !== "admin") {
    redirect("/");
  }
  return <PendingPropagationsClient />;
}
