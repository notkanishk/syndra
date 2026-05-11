import RequestAccessButton from "@/components/RequestAccessButton";
import { AdminDashboard } from "@/components/dashboard/AdminDashboard";
import { Badge } from "@/components/ui/Badge";
import { Card, CardHeader, CardTitle } from "@/components/ui/Card";
import { EmptyState } from "@/components/ui/EmptyState";
import { fetchApplications, fetchWithAuth } from "@/lib/api";
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
          <h1 className="mt-3 text-3xl font-bold text-on-surface tracking-tight">Welcome back, {session.name}</h1>
          <p className="mt-2 text-on-surface-variant">
            Browse your available services, check what is already active, and request new access without diving into raw Zitadel roles.
          </p>
        </header>

        <div className="grid grid-cols-1 gap-6 md:grid-cols-3">
          <Card>
            <CardHeader>
              <CardTitle>Identity</CardTitle>
            </CardHeader>
            <p className="text-2xl font-semibold">{session.title}</p>
            <p className="mt-2 text-sm text-on-surface-variant">{session.team} • {session.location}</p>
          </Card>
          <Card>
            <CardHeader>
              <CardTitle>Active Services</CardTitle>
            </CardHeader>
            <p className="text-4xl font-bold text-primary">{activeProjects.size}</p>
            <p className="mt-2 text-sm text-on-surface-variant">Applications currently available to your session.</p>
          </Card>
          <Card>
            <CardHeader>
              <CardTitle>Pending Reviews</CardTitle>
            </CardHeader>
            <p className="text-4xl font-bold text-primary">{pendingProjects.size}</p>
            <p className="mt-2 text-sm text-on-surface-variant">Requests still waiting in the governance queue.</p>
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
                <div key={entry.application.id} className="rounded-2xl border border-outline-variant bg-surface-container-high p-5">
                  <div className="flex items-start justify-between gap-3">
                    <div>
                      <p className="text-lg font-semibold text-on-surface">{entry.application.name}</p>
                      <p className="mt-1 text-sm text-on-surface-variant">{entry.application.description}</p>
                    </div>
                    <Badge variant={status === "Active" ? "secondary" : "outline"}>{status}</Badge>
                  </div>
                  <p className="mt-4 text-xs uppercase tracking-[0.22em] text-on-surface-variant">{entry.application.consumer}</p>
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

  return <AdminDashboard adminName={session.name} />;
}
