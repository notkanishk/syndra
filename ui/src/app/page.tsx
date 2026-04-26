import RequestAccessButton from "@/components/RequestAccessButton";
import { Badge } from "@/components/ui/Badge";
import { Card, CardHeader, CardTitle } from "@/components/ui/Card";
import { EmptyState } from "@/components/ui/EmptyState";
import { fetchApplications, fetchAudit, fetchBundles, fetchCatalog, fetchMappingRules, fetchProjects, fetchWithAuth } from "@/lib/api";
import type { AccessRequest, UserAccessView } from "@/lib/types";
import { getSession } from "@/lib/session";

export default async function Home() {
  const session = await getSession();
  if (!session) {
    return null;
  }

  const token = session.accessToken;

  if (session.role === "user") {
    const [apps, access, allRequests] = await Promise.all([
      fetchApplications(token).catch(() => []),
      fetchWithAuth<UserAccessView>(`/users/${session.id}/access`, token).catch(() => null),
      fetchWithAuth<AccessRequest[]>("/requests", token).catch(() => []),
    ]);

    const requests = Array.isArray(allRequests)
      ? allRequests.filter((entry) => entry?.requester_id === session.id)
      : [];

    const activeProjects = new Set(
      access?.projects
        ?.filter((project) => Array.isArray(project.effective_role_keys) && project.effective_role_keys.length > 0)
        .map((project) => project.project_id) ?? []
    );
    const pendingProjects = new Set(
      requests
        .filter((request) => request.status === "pending")
        .map((request) => request.project_id)
    );

    return (
      <div className="space-y-8 animate-fade-in-up">
        <header>
          <p className="text-sm font-semibold uppercase tracking-[0.28em] text-primary">Member Portal</p>
          <h1 className="mt-3 text-3xl font-bold text-foreground tracking-tight">Welcome back, {session.name}</h1>
          <p className="mt-2 text-muted">
            Browse your available services, check what is already active, and request new access without diving into raw Zitadel roles.
          </p>
        </header>

        <div className="grid grid-cols-1 gap-6 md:grid-cols-3">
          <Card>
            <CardHeader>
              <CardTitle>Identity</CardTitle>
            </CardHeader>
            <p className="text-2xl font-semibold">{session.title}</p>
            <p className="mt-2 text-sm text-muted">{session.team} • {session.location}</p>
          </Card>
          <Card>
            <CardHeader>
              <CardTitle>Active Services</CardTitle>
            </CardHeader>
            <p className="text-4xl font-bold text-primary">{activeProjects.size}</p>
            <p className="mt-2 text-sm text-muted">Applications currently available to your session.</p>
          </Card>
          <Card>
            <CardHeader>
              <CardTitle>Pending Reviews</CardTitle>
            </CardHeader>
            <p className="text-4xl font-bold text-primary">{pendingProjects.size}</p>
            <p className="mt-2 text-sm text-muted">Requests still waiting in the governance queue.</p>
          </Card>
        </div>

        <Card>
          <CardHeader>
            <CardTitle>Service Catalog</CardTitle>
          </CardHeader>
          {(!Array.isArray(apps) || apps.length === 0) ? (
            <EmptyState
              title="No services available yet"
              description="Your administrator hasn't published any apps you can request. Check back later or reach out for help."
            />
          ) : (
          <div className="grid grid-cols-1 gap-4 xl:grid-cols-2">
            {apps.map((entry) => {
              const status = activeProjects.has(entry.application.project_id)
                ? "Active"
                : pendingProjects.has(entry.application.project_id)
                  ? "Pending"
                  : "No Access";

              return (
                <div key={entry.application.id} className="rounded-2xl border border-border bg-surfaceHover p-5">
                  <div className="flex items-start justify-between gap-3">
                    <div>
                      <p className="text-lg font-semibold text-foreground">{entry.application.name}</p>
                      <p className="mt-1 text-sm text-muted">{entry.application.description}</p>
                    </div>
                    <Badge variant={status === "Active" ? "secondary" : "outline"}>{status}</Badge>
                  </div>
                  <p className="mt-4 text-xs uppercase tracking-[0.22em] text-muted">{entry.application.consumer}</p>
                  <RequestAccessButton
                    projectId={entry.application.project_id}
                    serviceName={entry.application.name}
                    status={status as "Active" | "Pending" | "No Access"}
                  />
                </div>
              );
            })}
          </div>
          )}
        </Card>
      </div>
    );
  }

  let bundleCount = 0;
  let ruleCount = 0;
  let userCount = 0;
  let appCount = 0;
  let projectCount = 0;
  let recentAudit: Array<{ id: string; action: string; target_id: string; created_at: string }> = [];

  try {
    const bundles = await fetchBundles(token);
    bundleCount = Array.isArray(bundles) ? bundles.length : 0;
  } catch {}

  try {
    const rules = await fetchMappingRules(token);
    ruleCount = Array.isArray(rules) ? rules.length : 0;
  } catch {}

  try {
    const catalog = await fetchCatalog(token);
    userCount = Array.isArray(catalog?.users) ? catalog.users.length : 0;
  } catch {}

  try {
    const apps = await fetchApplications(token);
    appCount = Array.isArray(apps) ? apps.length : 0;
  } catch {}

  try {
    const projects = await fetchProjects(token);
    projectCount = Array.isArray(projects) ? projects.length : 0;
  } catch {}

  try {
    const audit = await fetchAudit(4, token);
    recentAudit = Array.isArray(audit) ? audit : [];
  } catch {}

  return (
    <div className="space-y-8 animate-fade-in-up">
      <header>
        <h1 className="text-3xl font-bold text-foreground tracking-tight">Overview</h1>
        <p className="text-muted mt-2">Identity orchestrator across control-plane workflows, access lineage, and token simulation.</p>
      </header>

      <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-5 gap-6">
        <Card>
          <CardHeader>
            <CardTitle>Bundles</CardTitle>
          </CardHeader>
          <p className="text-4xl font-bold text-primary">{bundleCount}</p>
          <p className="text-sm text-muted mt-1">Role groupings defined</p>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>Mapping Rules</CardTitle>
          </CardHeader>
          <p className="text-4xl font-bold text-primary">{ruleCount}</p>
          <p className="text-sm text-muted mt-1">Active propagation paths</p>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>Users</CardTitle>
          </CardHeader>
          <p className="text-4xl font-bold text-primary">{userCount}</p>
          <p className="text-sm text-muted mt-1">Identities sourced from the directory</p>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>Applications</CardTitle>
          </CardHeader>
          <p className="text-4xl font-bold text-primary">{appCount}</p>
          <p className="text-sm text-muted mt-1">Token-consuming downstream systems</p>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>Projects</CardTitle>
          </CardHeader>
          <p className="text-4xl font-bold text-primary">{projectCount}</p>
          <p className="text-sm text-muted mt-1">Policy domains mapped from Zitadel</p>
        </Card>
      </div>

      <div className="grid grid-cols-1 xl:grid-cols-[1.4fr,1fr] gap-6">
        <Card>
          <CardHeader>
            <CardTitle>Current Stage</CardTitle>
          </CardHeader>
          <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
            <div className="rounded-xl border border-border bg-surfaceHover p-4">
              <p className="text-xs uppercase tracking-[0.24em] text-muted">Control Plane</p>
              <p className="mt-2 text-lg font-semibold">Bundles, mapping rules, and direct grants flow through the same audit and policy engine.</p>
              <p className="mt-2 text-sm text-muted">Every privileged action is audit-logged with the acting admin&apos;s identity.</p>
            </div>
            <div className="rounded-xl border border-border bg-surfaceHover p-4">
              <p className="text-xs uppercase tracking-[0.24em] text-muted">Data Plane</p>
              <p className="mt-2 text-lg font-semibold">Token simulation runs against compiled Redis claims.</p>
              <p className="mt-2 text-sm text-muted">Zitadel Actions v2 reads pre-flattened claims for sub-millisecond JWT enrichment.</p>
            </div>
          </div>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>Recent Activity</CardTitle>
          </CardHeader>
          <div className="space-y-3">
            {recentAudit.length === 0 ? (
              <EmptyState
                title="No activity yet"
                description="Admin actions will appear here as they happen."
                action={{ label: "Open audit log", href: "/audit" }}
              />
            ) : (
              recentAudit.map((entry) => (
                <div key={entry.id} className="rounded-xl border border-border bg-surfaceHover p-4">
                  <div className="flex items-center justify-between gap-3">
                    <Badge variant="secondary">{entry.action}</Badge>
                    <span className="text-xs text-muted">{new Date(entry.created_at).toLocaleString()}</span>
                  </div>
                  <p className="mt-2 text-sm text-foreground">{entry.target_id === "-" ? "System-wide event" : `Target: ${entry.target_id}`}</p>
                </div>
              ))
            )}
          </div>
        </Card>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>System Architecture</CardTitle>
        </CardHeader>
        <div className="grid grid-cols-2 md:grid-cols-4 gap-4 text-sm text-center">
          <div className="p-4 rounded-lg bg-surfaceHover border border-border">
            <p className="font-semibold">Next.js UI</p>
            <p className="text-muted text-xs mt-1">:3000</p>
          </div>
          <div className="p-4 rounded-lg bg-surfaceHover border border-border">
            <p className="font-semibold">Go API</p>
            <p className="text-muted text-xs mt-1">:8080</p>
          </div>
          <div className="p-4 rounded-lg bg-surfaceHover border border-border">
            <p className="font-semibold">PostgreSQL</p>
            <p className="text-muted text-xs mt-1">:5432</p>
          </div>
          <div className="p-4 rounded-lg bg-surfaceHover border border-border">
            <p className="font-semibold">Redis</p>
            <p className="text-muted text-xs mt-1">:6379</p>
          </div>
        </div>
      </Card>
    </div>
  );
}
