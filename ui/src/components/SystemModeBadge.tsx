import { Badge } from "@/components/ui/Badge";
import { fetchSystemMode } from "@/lib/api";

// SystemModeBadge surfaces unexpected directory fallback to admins. Stays
// silent in the steady-state ("zitadel" + not degraded) so the chrome doesn't
// gain noise; renders a small outline tag in pure local-dev demo mode; and
// renders a prominent destructive tag when ZITADEL_DOMAIN is configured but
// the directory has degraded to the demo source (unreachable management API).
export default async function SystemModeBadge({ token }: { token?: string }) {
  const mode = await fetchSystemMode(token);
  if (!mode) return null;

  if (mode.degraded) {
    return (
      <div
        title="ZITADEL_DOMAIN is set but the directory is serving the demo catalog. Check the backend logs for InitClient errors."
        className="mt-3"
      >
        <Badge variant="destructive">DEGRADED · demo fallback</Badge>
      </div>
    );
  }

  if (mode.directory === "demo") {
    return (
      <div
        title="Local development mode. Set ZITADEL_DOMAIN + ZITADEL_MACHINE_KEY_PATH for live Zitadel."
        className="mt-3"
      >
        <Badge variant="outline">Demo mode</Badge>
      </div>
    );
  }

  // Live + healthy — stay silent.
  return null;
}
