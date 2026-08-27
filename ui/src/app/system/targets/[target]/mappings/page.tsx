import { MappingScreen } from "@/components/targets/MappingScreen";

/**
 * One add-on's mappings, and their published versions.
 *
 * A sub-route of the target rather than a rail row of its own: a row per add-on
 * already exists, and a second one for its mappings would be structure
 * competing with itself. The breadcrumb keeps the target a live link, because
 * the likeliest move after opening its mappings is going back to it.
 */
export default async function TargetMappingsPage({
  params,
}: {
  params: Promise<{ target: string }>;
}) {
  const { target } = await params;
  return <MappingScreen target={target} />;
}
