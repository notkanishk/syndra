import { redirect } from "next/navigation";

import { LoginDoor, type DemoIdentity } from "@/components/login/LoginDoor";
import { loginFailure } from "@/lib/login-error";
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
  const session = await getSession();
  if (session) redirect("/");

  const { error } = await searchParams;
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
    />
  );
}
