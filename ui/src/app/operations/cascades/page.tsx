import { redirect } from "next/navigation";

import { RecentCascades } from "@/components/operations/RecentCascades";
import { Eyebrow } from "@/components/ui/Eyebrow";
import { getSession } from "@/lib/session";

/**
 * Recent automated cascades feed (Task 22) — applied bundle/rule/lifecycle
 * outbox projections, so "auto" confirmation mode never means invisible.
 * Server-gated to admins, same posture as the other propagation/drift pages.
 */
export default async function RecentCascadesPage() {
  const session = await getSession();
  if (!session) {
    redirect("/login");
  }
  if (session.role !== "admin") {
    redirect("/");
  }
  return (
    <div className="p-8 space-y-6 animate-fade-in-up relative z-10">
      <header>
        <Eyebrow tone="primary">Operations</Eyebrow>
        <h1 className="text-3xl font-semibold text-on-surface mt-1 font-display">
          Recent cascades
        </h1>
        <p className="text-sm text-on-surface-variant max-w-2xl">
          Bundle, mapping-rule, and lifecycle projections that already reached Zitadel — the
          audit trail for every automated cascade so an &ldquo;auto&rdquo; confirmation mode is
          never invisible.
        </p>
      </header>
      <RecentCascades />
    </div>
  );
}
