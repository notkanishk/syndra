import { Card } from './ui/Card';
import { Pulse } from './ui/Pulse';
import SidebarNav from './SidebarNav';
import SystemModeBadge from './SystemModeBadge';
import ThemeToggle from './ThemeToggle';
import type { SessionUser } from '@/lib/session';

export default function Sidebar({ session }: { session: SessionUser }) {
  const isAdmin = session.role === "admin";

  return (
    <div className="w-64 h-screen bg-surface border-r border-outline-variant flex flex-col">
      <div className="p-6">
        <h2 className="text-xl font-bold text-on-surface tracking-tight">MkAuth</h2>
        <p className="text-xs text-primary font-semibold uppercase tracking-widest mt-1">
          {isAdmin ? "Control Plane" : "Service Portal"}
        </p>
      </div>

      <SidebarNav isAdmin={isAdmin} />

      <div className="p-4">
        <Card className="!p-4 bg-surface-container-high border-none shadow-none">
          <div className="flex items-start justify-between gap-3">
            <div>
              <p className="text-xs text-on-surface-variant">Signed in as</p>
              <div className="mt-2">
                <p className="text-sm font-medium">{session.name}</p>
                <p className="text-xs text-on-surface-variant">{session.role === "admin" ? "Admin session" : "Member session"}</p>
              </div>
            </div>
            <ThemeToggle />
          </div>
          <form action="/auth/logout" method="post" className="mt-4">
            <button type="submit" className="w-full rounded-lg border border-outline-variant px-3 py-2 text-sm text-on-surface-variant transition-colors hover:border-primary hover:text-on-surface">
              Sign out
            </button>
          </form>
          <div className="flex items-center gap-2 mt-4 text-success">
            <Pulse variant="success" static />
            <span className="text-sm font-medium text-on-surface">Proxmox LXC</span>
          </div>
          {/* Quiet by default; renders only in demo mode or unexpected fallback. */}
          <SystemModeBadge token={session.accessToken} />
        </Card>
      </div>
    </div>
  );
}
