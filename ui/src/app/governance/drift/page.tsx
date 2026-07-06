import { redirect } from "next/navigation";

import { DriftTriageClient } from "@/components/drift/DriftTriageClient";
import { getSession } from "@/lib/session";

/**
 * Operator triage worklist for out-of-band Zitadel/MkAuth drift (B2). Every
 * row needs an explicit Attribute / Revoke / Mark-external — drift is RED and
 * undismissible, unlike the amber pending-propagation worklist. Server-gated
 * to admins; the client island owns the data + triage actions.
 */
export default async function DriftPage() {
  const session = await getSession();
  if (!session) {
    redirect("/login");
  }
  if (session.role !== "admin") {
    redirect("/");
  }
  return <DriftTriageClient />;
}
