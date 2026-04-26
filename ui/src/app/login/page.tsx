import { redirect } from "next/navigation";

import { Card, CardHeader, CardTitle } from "@/components/ui/Card";
import { getDemoUsers, getSession, type SessionUser } from "@/lib/session";

export default async function LoginPage() {
  const session = await getSession();
  if (session) {
    redirect("/");
  }

  const isOidcMode = Boolean(process.env.ZITADEL_DOMAIN);

  return (
    <div className="min-h-screen bg-background px-6 py-10 text-foreground">
      <div className="mx-auto grid max-w-6xl gap-8 lg:grid-cols-[1.15fr,0.95fr]">
        <section className="rounded-[2rem] border border-border bg-surface p-8 shadow-[0_30px_80px_rgba(12,22,44,0.10)]">
          <p className="text-sm font-semibold uppercase tracking-[0.32em] text-primary">
            {isOidcMode ? "Live" : "Local Dev"}
          </p>
          <h1 className="mt-4 text-5xl font-semibold tracking-tight">MkAuth Session Gateway</h1>
          <p className="mt-4 max-w-2xl text-base leading-7 text-muted">
            {isOidcMode
              ? "Live Zitadel OIDC authentication. Your Zitadel-issued token is forwarded to the backend for verification on every request."
              : "Local-development session. Pick a demo identity to exercise admin and member flows without a live Zitadel."}
          </p>

          <div className="mt-8 grid gap-4 md:grid-cols-3">
            <div className="rounded-2xl border border-border bg-background p-5">
              <p className="text-xs uppercase tracking-[0.24em] text-muted">Admin Control</p>
              <p className="mt-3 text-lg font-semibold">Policy, governance, and simulation views stay reserved for operators.</p>
            </div>
            <div className="rounded-2xl border border-border bg-background p-5">
              <p className="text-xs uppercase tracking-[0.24em] text-muted">Member Portal</p>
              <p className="mt-3 text-lg font-semibold">Standard users land in a service-first view with access status and request flows.</p>
            </div>
            <div className="rounded-2xl border border-border bg-background p-5">
              <p className="text-xs uppercase tracking-[0.24em] text-muted">
                {isOidcMode ? "Live Auth" : "Future OIDC"}
              </p>
              <p className="mt-3 text-lg font-semibold">
                {isOidcMode
                  ? "RS256 JWTs from Zitadel are validated by the backend on every API call."
                  : "The cookie session is shaped to be swapped for live Zitadel login later."}
              </p>
            </div>
          </div>
        </section>

        <Card className="self-start">
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
      <p className="text-sm text-muted mb-6">
        You will be redirected to your Zitadel instance to authenticate. Your token will be forwarded to MkAuth on return.
      </p>
      <a
        href="/auth/zitadel"
        className="flex w-full items-center justify-center rounded-xl bg-primary px-4 py-3 text-sm font-semibold text-white"
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
          <form key={user.id} action="/auth/login" method="post" className="rounded-2xl border border-border bg-surfaceHover p-4">
            <input type="hidden" name="userId" value={user.id} />
            <div className="flex items-start justify-between gap-4">
              <div className="flex items-center gap-3">
                <div className="flex h-11 w-11 items-center justify-center rounded-full bg-primary/10 text-sm font-semibold text-primary">
                  {user.avatar}
                </div>
                <div>
                  <p className="font-semibold text-foreground">{user.name}</p>
                  <p className="text-sm text-muted">{user.title}</p>
                </div>
              </div>
              <span className="rounded-full border border-border px-3 py-1 text-xs font-semibold uppercase tracking-[0.18em] text-muted">
                {user.role === "admin" ? "Admin" : "Member"}
              </span>
            </div>
            <p className="mt-3 text-sm text-muted">
              {user.team} • {user.location} • {user.email}
            </p>
            <button
              type="submit"
              className="mt-4 w-full rounded-xl bg-primary px-4 py-3 text-sm font-semibold text-white"
            >
              Continue as {user.name}
            </button>
          </form>
        ))}
      </div>
    </>
  );
}
