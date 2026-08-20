import { redirect } from "next/navigation";

import { LoginDoor, type DemoIdentity } from "@/components/login/LoginDoor";
import { loginFailure } from "@/lib/login-error";
import { safeReturnPath } from "@/lib/return-path";
import { getDemoUsers, getSession } from "@/lib/session";

export const metadata = {
  title: "Sign in · Syndra",
};

/**
 * The one unauthenticated route, and the only ceremonial screen in the app.
 * It sits outside the shell — no rail, no view switch, nothing to navigate.
 *
 * Demo identities render ONLY when ZITADEL_DOMAIN is unset. `getDemoUsers()`
 * gates that at the source as well; a live deployment must never serialise the
 * demo catalog into its RSC payload.
 */
export default async function LoginPage({
  searchParams,
}: {
  searchParams: Promise<Record<string, string | string[] | undefined>>;
}) {
  const params = await searchParams;
  const nextParam = Array.isArray(params.next) ? params.next[0] : params.next;
  // Validated here as well as at the two routes that consume it, because this
  // is where it re-enters the page as a value the browser will act on.
  const next = safeReturnPath(nextParam);

  const session = await getSession();
  // Already signed in and arriving with a destination — usually a link opened
  // in a second tab. Send them to it rather than to the landing.
  if (session) redirect(next);

  const { error } = params;
  const code = Array.isArray(error) ? error[0] : error;

  const identities: DemoIdentity[] = getDemoUsers().map((user) => ({
    id: user.id,
    name: user.name,
    role: user.role,
  }));

  return (
    <LoginDoor
      mode={process.env.ZITADEL_DOMAIN ? "oidc" : "demo"}
      identities={identities}
      failure={loginFailure(code)}
      next={next}
    />
  );
}
