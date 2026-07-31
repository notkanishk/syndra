import { MemberAccess } from "@/components/member/MemberAccess";
import { Today } from "@/components/today/Today";
import { getSession } from "@/lib/session";

/**
 * The landing route serves two audiences from one path.
 *
 * Operators get Today — actionable work only. Members get My access, because
 * a work queue for someone with no queue is an empty room, and because it is
 * the one screen that explains to a member why they can badge into the laser
 * lab.
 */
export default async function Home() {
  const session = await getSession();
  if (!session) return null;

  if (session.role === "admin") return <Today session={session} />;
  return <MemberAccess session={session} />;
}
