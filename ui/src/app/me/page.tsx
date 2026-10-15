import { MemberAccess } from "@/components/member/MemberAccess";
import { getSession } from "@/lib/session";

/**
 * An operator's own access (A-03 §4, B), reached from the Account sheet. A
 * member has the same page at `/`; an operator's `/` is Home, the makerspace's
 * work, and their own access is asked for like anyone's.
 */
export default async function MyAccessPage() {
  const session = await getSession();
  if (!session) return null;
  return <MemberAccess session={session} />;
}
