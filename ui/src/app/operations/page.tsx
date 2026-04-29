import { redirect } from "next/navigation";

import OperationsClient from "@/components/operations/OperationsClient";
import { getSession } from "@/lib/session";

/**
 * Admin-only operator queue. Server-side gates non-admins to / so the page
 * never even hydrates for member sessions; the proxy enforces the same rule
 * defense-in-depth on every backend call. The data layer is owned by the
 * client island so polling can keep the cache warm without bloating the RSC.
 */
export default async function OperationsPage() {
  const session = await getSession();
  if (!session) {
    redirect("/login");
  }
  if (session.role !== "admin") {
    redirect("/");
  }
  return <OperationsClient />;
}
