import { Card, CardHeader, CardTitle } from "@/components/ui/Card";

export default function ApplicationsView() {
  return (
    <div className="space-y-6 animate-fade-in-up">
      <header>
        <h1 className="text-3xl font-bold text-foreground">Applications</h1>
        <p className="text-muted mt-2">Manage claim-shaping rules and preview JWT token outputs per application.</p>
      </header>

      <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
        <Card>
          <CardHeader>
            <CardTitle>Claim Shaping</CardTitle>
          </CardHeader>
          <p className="text-sm text-muted">
            Map internal hierarchical roles to exact JWT claims for legacy systems.
          </p>
          <div className="mt-4 border border-dashed border-border rounded-lg p-6 text-center">
            <p className="text-muted text-sm">No claim profiles configured yet.</p>
          </div>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle className="text-amber-500">Token Simulator</CardTitle>
          </CardHeader>
          <p className="text-sm text-muted mb-4">
            Preview the exact JWT payload a user would receive.
          </p>
          <div className="bg-background border border-border rounded-lg p-4 font-mono text-sm overflow-x-auto">
            <pre className="text-emerald-500">{`{
  "iss": "https://auth.makerspace.local",
  "sub": "user_id_12345",
  "x-custom-group": [
    "student",
    "3d_lab_entry_certified"
  ],
  "door_groups": "room-a,room-b"
}`}</pre>
          </div>
          <p className="text-xs text-muted mt-3 italic">
            Live simulation requires Zitadel connection.
          </p>
        </Card>
      </div>
    </div>
  );
}
