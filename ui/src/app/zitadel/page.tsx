import { redirect } from "next/navigation";

import ZitadelDiagnostics from "@/components/zitadel/ZitadelDiagnostics";
import { getSession } from "@/lib/session";

/**
 * Admin-only Zitadel diagnostics. Server-side gates non-admins to / and
 * anonymous requests to /login so the page never even hydrates for member
 * sessions — closing the pre-split gap where this route carried no guard.
 * The client island owns the section data + CRUD.
 */
export default async function ZitadelPage() {
  const session = await getSession();
  if (!session) redirect("/login");
  if (session.role !== "admin") redirect("/");
  return <ZitadelDiagnostics />;
}
