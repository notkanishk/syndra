import { Card, CardHeader, CardTitle } from "@/components/ui/Card";
import { Badge } from "@/components/ui/Badge";

export default function UsersView() {
  return (
    <div className="space-y-6 animate-fade-in-up">
      <header>
        <h1 className="text-3xl font-bold text-foreground">Users &amp; Access</h1>
        <p className="text-muted mt-2">Trace source vs derived roles across projects for any user.</p>
      </header>

      <Card>
        <CardHeader>
          <CardTitle>Access Lineage</CardTitle>
        </CardHeader>
        <p className="text-sm text-muted mb-4">
          Select a user to answer: &ldquo;Why does this user have access to X?&rdquo;
        </p>

        <div className="border border-border rounded-lg overflow-hidden">
          <div className="grid grid-cols-3 gap-4 px-4 py-3 bg-surfaceHover border-b border-border text-xs font-semibold text-muted uppercase tracking-wider">
            <span>Project</span>
            <span>Source</span>
            <span>Role Key</span>
          </div>
          <div className="divide-y divide-border">
            <div className="grid grid-cols-3 gap-4 px-4 py-3 text-sm">
              <span className="text-foreground font-medium">Machine Shop</span>
              <Badge variant="secondary" className="w-fit">Student Bundle (Derived)</Badge>
              <span className="font-mono text-xs text-muted">basic_tool_use</span>
            </div>
            <div className="grid grid-cols-3 gap-4 px-4 py-3 text-sm">
              <span className="text-foreground font-medium">Door Access</span>
              <Badge variant="outline" className="w-fit border-primary text-primary">Direct Rule</Badge>
              <span className="font-mono text-xs text-muted">3d_lab_pin</span>
            </div>
            <div className="grid grid-cols-3 gap-4 px-4 py-3 text-sm">
              <span className="text-foreground font-medium">3D Printing</span>
              <Badge variant="secondary" className="w-fit">Propagated (Auto)</Badge>
              <span className="font-mono text-xs text-muted">printer_user</span>
            </div>
          </div>
        </div>

        <p className="text-xs text-muted mt-4 italic">
          Live user lookups will be available once Zitadel machine keys are configured.
        </p>
      </Card>
    </div>
  );
}
