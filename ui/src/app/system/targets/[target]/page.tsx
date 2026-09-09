import { redirect } from "next/navigation";

import { TargetOverview } from "@/components/targets/TargetOverview";

/**
 * One add-on target: its health, the accounts on it Syndra did not create, what
 * it can do, and how to stop it writing.
 *
 * The route exists per registered target, from deployment configuration —
 * navigation never derives it from what this operator can see.
 */
export default async function TargetPage({
  params,
}: {
  params: Promise<{ target: string }>;
}) {
  const { target } = await params;
  // Zitadel is never a registered add-on — it is core to Syndra, not a
  // plugin — so this page's whole model (an addon health check, a maintenance
  // lifecycle, an unmanaged-account inventory) does not apply to it, and every
  // one of those reads answers with "not registered", read here as "not
  // answering". /zitadel already computes the real fact from the real source
  // (the Management client); sending "zitadel" here instead of duplicating
  // that computation is what a second, disagreeing answer would otherwise be.
  if (target === "zitadel") redirect("/zitadel");
  return <TargetOverview target={target} />;
}
