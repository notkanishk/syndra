import { redirect } from "next/navigation";

import { Card, CardHeader, CardTitle } from "@/components/ui/Card";
import { getDemoUsers, getSession } from "@/lib/session";

export default async function LoginPage() {
  const session = await getSession();
  if (session) {
    redirect("/");
  }

  const users = getDemoUsers();

  return (
    <div className="min-h-screen bg-background px-6 py-10 text-foreground">
      <div className="mx-auto grid max-w-6xl gap-8 lg:grid-cols-[1.15fr,0.95fr]">
        <section className="rounded-[2rem] border border-border bg-surface p-8 shadow-[0_30px_80px_rgba(12,22,44,0.10)]">
          <p className="text-sm font-semibold uppercase tracking-[0.32em] text-primary">Phase 2</p>
          <h1 className="mt-4 text-5xl font-semibold tracking-tight">MkAuth Session Gateway</h1>
          <p className="mt-4 max-w-2xl text-base leading-7 text-muted">
            This first orchestration-core pass introduces a session boundary in front of the existing demo stack, so the app behaves like an admin control plane or a member portal instead of a single shared surface.
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
              <p className="text-xs uppercase tracking-[0.24em] text-muted">Future OIDC</p>
              <p className="mt-3 text-lg font-semibold">The cookie session is shaped to be swapped for live Zitadel login later.</p>
            </div>
          </div>
        </section>

        <Card className="self-start">
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
        </Card>
      </div>
    </div>
  );
}

