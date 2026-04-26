import { Card } from './ui/Card';
import SidebarNav from './SidebarNav';
import SystemModeBadge from './SystemModeBadge';
import ThemeToggle from './ThemeToggle';
import type { SessionUser } from '@/lib/session';

export default function Sidebar({ session }: { session: SessionUser }) {
  const isAdmin = session.role === "admin";

  return (
    <div className="w-64 h-screen bg-surface border-r border-border flex flex-col">
      <div className="p-6">
        <h2 className="text-xl font-bold text-foreground tracking-tight">MkAuth</h2>
        <p className="text-xs text-primary font-semibold uppercase tracking-widest mt-1">
          {isAdmin ? "Control Plane" : "Service Portal"}
        </p>
      </div>

      <SidebarNav isAdmin={isAdmin} />

      <div className="p-4">
        <Card className="!p-4 bg-surfaceHover border-none shadow-none">
          <div className="flex items-start justify-between gap-3">
            <div>
              <p className="text-xs text-muted">Signed in as</p>
              <div className="mt-2">
                <p className="text-sm font-medium">{session.name}</p>
                <p className="text-xs text-muted">{session.role === "admin" ? "Admin session" : "Member session"}</p>
              </div>
            </div>
            <ThemeToggle />
          </div>
          <form action="/auth/logout" method="post" className="mt-4">
            <button type="submit" className="w-full rounded-lg border border-border px-3 py-2 text-sm text-muted transition-colors hover:border-primary/40 hover:text-foreground">
              Sign out
            </button>
          </form>
          <div className="flex items-center mt-4">
            <span className="w-2 h-2 rounded-full bg-emerald-500 mr-2"></span>
            <span className="text-sm font-medium">Proxmox LXC</span>
          </div>
          {/* Quiet by default; renders only in demo mode or unexpected fallback. */}
          <SystemModeBadge token={session.accessToken} />
        </Card>
      </div>
    </div>
  );
}
