import { cookies } from "next/headers";

import { MemberAccess } from "@/components/member/MemberAccess";
import { Home } from "@/components/home/Home";
import { DOOR_COOKIE, doorFor } from "@/lib/motion/door";
import { getSession } from "@/lib/session";

/**
 * The landing route serves two audiences from one path.
 *
 * Operators get Home — actionable work first. Members get My access, because
 * a work queue for someone with no queue is an empty room, and because it is
 * the one screen that explains to a member why they can badge into the laser
 * lab.
 */
// `LandingPage`, not `Home`: the route serves Home to operators and My access
// to members, so naming it after one of them would describe half of what it
// does — and it would collide with the Home component it imports.
export default async function LandingPage() {
  const session = await getSession();
  if (!session) return null;

  // The door (A-24 §3.2) is decided here, before the first paint: the sign-in's own cookie, read by the server.
  if (session.role === "admin") {
    const door = doorFor((await cookies()).get(DOOR_COOKIE)?.value, session.signedInAt);
    return <Home session={session} door={door} />;
  }
  return <MemberAccess session={session} />;
}
