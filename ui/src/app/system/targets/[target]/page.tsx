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
  return <TargetOverview target={target} />;
}
