import { MemberSystemPage } from "@/components/member/MemberSystemPage";

/**
 * A member's own page for one connected system that is not storage (the rows
 * `memberNav` draws). A storage system's one address is `/storage`.
 */
export default async function SystemPage({ params }: { params: Promise<{ target: string }> }) {
  const { target } = await params;
  return <MemberSystemPage target={target} />;
}
