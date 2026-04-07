import { Card, CardHeader, CardTitle } from "@/components/ui/Card";
import { fetchBundles, fetchMappingRules } from "@/lib/api";

export default async function Home() {
  let bundleCount = 0;
  let ruleCount = 0;

  try {
    const bundles = await fetchBundles();
    bundleCount = Array.isArray(bundles) ? bundles.length : 0;
  } catch {}

  try {
    const rules = await fetchMappingRules();
    ruleCount = Array.isArray(rules) ? rules.length : 0;
  } catch {}

  return (
    <div className="space-y-8 animate-fade-in-up">
      <header>
        <h1 className="text-3xl font-bold text-foreground tracking-tight">Overview</h1>
        <p className="text-muted mt-2">MkAuth Identity Orchestrator — Control Plane Dashboard</p>
      </header>

      <div className="grid grid-cols-1 md:grid-cols-3 gap-6">
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
            <CardTitle>Orchestrator</CardTitle>
          </CardHeader>
          <div className="flex items-center gap-2">
            <span className="w-2.5 h-2.5 rounded-full bg-emerald-500 animate-pulse"></span>
            <span className="text-sm font-medium">Healthy</span>
          </div>
          <p className="text-sm text-muted mt-1">Connected to Proxmox LXC</p>
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
