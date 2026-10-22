import { ServiceAccountPage } from "@/components/system/ServiceAccountPage";

/** One service account: its settings, then what it holds (A-07 §27). */
export default async function ServiceAccountRoute({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  return <ServiceAccountPage id={id} />;
}
