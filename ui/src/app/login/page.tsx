import { redirect } from "next/navigation";

import { Avatar } from "@/components/ui/Avatar";
import { getDemoUsers, getSession, type SessionUser } from "@/lib/session";

/**
 * The one unauthenticated surface. It sits outside the shell — no rail, no
 * view switch, nothing to navigate — so it is the one place the display face
 * gets to be large.
 *
 * Demo identities are rendered ONLY when ZITADEL_DOMAIN is unset, guarded in
 * two places: a live deployment must never serialise the demo catalog into its
 * RSC payload.
 */
export default async function LoginPage() {
  const session = await getSession();
  if (session) redirect("/");

  const isOidcMode = Boolean(process.env.ZITADEL_DOMAIN);

  return (
    <div className="min-h-screen bg-ground px-6 py-14">
      <div className="mx-auto grid max-w-[1100px] items-start gap-10 lg:grid-cols-[1.05fr_0.95fr]">
        <section>
          <div className="mb-6 flex items-center gap-2.5">
            <span className="flex h-8 w-8 items-center justify-center rounded-[10px] bg-accent font-display text-[17px] font-bold text-accent-ink">
              m
            </span>
            <span className="font-display text-[20px] font-semibold tracking-[-0.01em]">Syndra</span>
          </div>

          <h1 className="font-display text-[62px] font-semibold leading-[0.96] tracking-[-0.03em]">
            Who can get in,
            <br />
            <span className="text-accent-text">and why.</span>
          </h1>
          <p className="mt-6 max-w-[46ch] text-[17px] leading-[1.55] text-muted">
            {isOidcMode
              ? "Sign in with your makerspace account. Your token is forwarded to Syndra and verified on every request."
              : "Local development. Pick an identity to exercise the operator and member surfaces without a live identity provider."}
          </p>
        </section>

        <div className="panel p-6">
          {isOidcMode ? <ZitadelLogin /> : <DemoIdentities />}
        </div>
      </div>
    </div>
  );
}

function ZitadelLogin() {
  return (
    <>
      <h2 className="type-card-title mb-2">Sign in</h2>
      <p className="mb-5 text-[14px] leading-[1.55] text-muted">
        You&rsquo;ll be sent to the identity provider and back.
      </p>
      <a
        href="/auth/zitadel"
        className="flex w-full items-center justify-center rounded-pill bg-accent px-4 py-3 text-[14.5px] font-semibold text-accent-ink transition-all hover:brightness-105"
      >
        Continue
      </a>
    </>
  );
}

function DemoIdentities() {
  if (process.env.ZITADEL_DOMAIN) return null;

  const users: SessionUser[] = getDemoUsers();

  return (
    <>
      <h2 className="type-card-title mb-2">Choose an identity</h2>
      <p className="mb-4 text-[14px] text-muted">Development only.</p>
      <div className="flex flex-col gap-2.5">
        {users.map((user) => (
          <form key={user.id} action="/auth/login" method="post">
            <input type="hidden" name="userId" value={user.id} />
            <button
              type="submit"
              className="flex w-full items-center gap-3 rounded-inner border border-line-strong px-4 py-3 text-left transition-colors hover:bg-[var(--hover)]"
            >
              <Avatar name={user.name} />
              <span className="min-w-0 flex-1">
                <span className="block truncate text-[15px] font-semibold">{user.name}</span>
                <span className="block truncate text-[13px] text-faint">{user.title}</span>
              </span>
              <span className="rounded-pill border border-line-strong px-2.5 py-1 text-[11.5px] font-semibold uppercase tracking-[0.1em] text-muted">
                {user.role === "admin" ? "Operator" : "Member"}
              </span>
            </button>
          </form>
        ))}
      </div>
    </>
  );
}
