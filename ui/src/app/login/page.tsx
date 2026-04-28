import { redirect } from "next/navigation";

import { Card, CardHeader, CardTitle } from "@/components/ui/Card";
import { Eyebrow } from "@/components/ui/Eyebrow";
import { getDemoUsers, getSession, type SessionUser } from "@/lib/session";

/**
 * Login surface. Stage 3 reskins the hero treatment to the Obsidian Clarity
 * language: Fraunces display headline, glass-card login form, and the global
 * `bg-blob-hero` background mounted by the root layout. Functional behavior
 * is unchanged — demo cookies are still rejected when ZITADEL_DOMAIN is set,
 * and the post-login redirect goes to `/`.
 */
export default async function LoginPage() {
  const session = await getSession();
  if (session) {
    redirect("/");
  }

  const isOidcMode = Boolean(process.env.ZITADEL_DOMAIN);

  return (
    <div className="min-h-screen px-6 py-10 text-on-surface relative z-10">
      <div className="mx-auto grid max-w-6xl gap-8 lg:grid-cols-[1.15fr,0.95fr] items-start">
        <section className="glass-card p-10">
          <Eyebrow tone="primary">{isOidcMode ? "Live · Zitadel OIDC" : "Local development"}</Eyebrow>
          <h1 className="mt-4 text-5xl md:text-6xl font-semibold tracking-tight font-display">
            MkAuth session gateway
          </h1>
          <p className="mt-4 max-w-2xl text-base leading-7 text-on-surface-variant">
            {isOidcMode
              ? "Live Zitadel OIDC authentication. Your Zitadel-issued token is forwarded to the backend for verification on every request."
              : "Local-development session. Pick a demo identity to exercise admin and member flows without a live Zitadel."}
          </p>

          <div className="mt-8 grid gap-4 md:grid-cols-3">
            <div className="rounded-card border border-outline-variant bg-surface-container-low p-5">
              <Eyebrow>Admin control</Eyebrow>
              <p className="mt-3 text-base font-semibold text-on-surface">
                Policy, governance, and simulation views stay reserved for operators.
              </p>
            </div>
            <div className="rounded-card border border-outline-variant bg-surface-container-low p-5">
              <Eyebrow>Member portal</Eyebrow>
              <p className="mt-3 text-base font-semibold text-on-surface">
                Standard users land in a service-first view with access status
                and request flows.
              </p>
            </div>
            <div className="rounded-card border border-outline-variant bg-surface-container-low p-5">
              <Eyebrow>{isOidcMode ? "Live auth" : "Future OIDC"}</Eyebrow>
              <p className="mt-3 text-base font-semibold text-on-surface">
                {isOidcMode
                  ? "RS256 JWTs from Zitadel are validated by the backend on every API call."
                  : "The cookie session is shaped to be swapped for live Zitadel login later."}
              </p>
            </div>
          </div>
        </section>

        <Card variant="glass" className="self-start">
          {isOidcMode ? <ZitadelLoginCard /> : <DemoIdentityCard />}
        </Card>
      </div>
    </div>
  );
}

function ZitadelLoginCard() {
  return (
    <>
      <CardHeader>
        <CardTitle>Sign in with Zitadel</CardTitle>
      </CardHeader>
      <p className="text-sm text-on-surface-variant mb-6">
        You will be redirected to your Zitadel instance to authenticate. Your
        token will be forwarded to MkAuth on return.
      </p>
      <a
        href="/auth/zitadel"
        className="flex w-full items-center justify-center rounded-full bg-[linear-gradient(135deg,var(--primary),var(--secondary))] text-on-primary px-4 py-3 text-sm font-semibold shadow-[0_8px_24px_-8px_var(--primary),inset_0_1px_0_rgba(255,255,255,0.15)] hover:brightness-110 transition-all focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary-container"
      >
        Continue with Zitadel
      </a>
    </>
  );
}

// DemoIdentityCard is rendered ONLY when ZITADEL_DOMAIN is unset. The runtime
// guard mirrors the JSX branch in LoginPage as defense-in-depth: the demo
// identity catalog must never be serialized into the RSC payload of a
// production deployment.
function DemoIdentityCard() {
  if (process.env.ZITADEL_DOMAIN) return null;

  const users: SessionUser[] = getDemoUsers();

  return (
    <>
      <CardHeader>
        <CardTitle>Choose a demo identity</CardTitle>
      </CardHeader>
      <div className="space-y-3">
        {users.map((user) => (
          <form
            key={user.id}
            action="/auth/login"
            method="post"
            className="rounded-card border border-outline-variant bg-surface-container-low p-4 transition-colors hover:border-primary-container/50"
          >
            <input type="hidden" name="userId" value={user.id} />
            <div className="flex items-start justify-between gap-4">
              <div className="flex items-center gap-3">
                <div className="flex h-11 w-11 items-center justify-center rounded-full bg-primary-container/15 text-sm font-semibold text-primary-container">
                  {user.avatar}
                </div>
                <div>
                  <p className="font-semibold text-on-surface">{user.name}</p>
                  <p className="text-sm text-on-surface-variant">{user.title}</p>
                </div>
              </div>
              <span className="rounded-full border border-outline-variant px-3 py-1 text-xs font-semibold uppercase tracking-[0.18em] text-on-surface-variant">
                {user.role === "admin" ? "Admin" : "Member"}
              </span>
            </div>
            <p className="mt-3 text-sm text-on-surface-variant">
              {user.team} • {user.location} • {user.email}
            </p>
            <button
              type="submit"
              className="mt-4 w-full rounded-full bg-[linear-gradient(135deg,var(--primary),var(--secondary))] text-on-primary px-4 py-3 text-sm font-semibold shadow-[0_8px_24px_-8px_var(--primary),inset_0_1px_0_rgba(255,255,255,0.15)] hover:brightness-110 transition-all focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary-container"
            >
              Continue as {user.name}
            </button>
          </form>
        ))}
      </div>
    </>
  );
}
