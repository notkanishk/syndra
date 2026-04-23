import Link from 'next/link';
import { Card } from './ui/Card';
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
      <nav className="flex-1 px-4 space-y-1">
        <p className="px-3 py-1 text-xs font-semibold text-muted uppercase tracking-wider">
          {isAdmin ? "Dashboard" : "Portal"}
        </p>
        <Link href="/" className="block px-3 py-2 text-sm text-muted hover:text-foreground hover:bg-surfaceHover rounded-md transition-colors">
          {isAdmin ? "Overview" : "My Services"}
        </Link>

        {isAdmin ? (
          <>
            <p className="px-3 pt-4 pb-1 text-xs font-semibold text-muted uppercase tracking-wider">Identity</p>
            <Link href="/users" className="block px-3 py-2 text-sm text-muted hover:text-foreground hover:bg-surfaceHover rounded-md transition-colors">Users &amp; Access</Link>
            <Link href="/applications" className="block px-3 py-2 text-sm text-muted hover:text-foreground hover:bg-surfaceHover rounded-md transition-colors">Applications</Link>
            <Link href="/projects" className="block px-3 py-2 text-sm text-muted hover:text-foreground hover:bg-surfaceHover rounded-md transition-colors">Projects</Link>

            <p className="px-3 pt-4 pb-1 text-xs font-semibold text-muted uppercase tracking-wider">Policy Engine</p>
            <Link href="/bundles" className="block px-3 py-2 text-sm text-muted hover:text-foreground hover:bg-surfaceHover rounded-md transition-colors">Bundles</Link>
            <Link href="/policies" className="block px-3 py-2 text-sm text-muted hover:text-foreground hover:bg-surfaceHover rounded-md transition-colors">Mapping Rules</Link>
            <Link href="/graph" className="block px-3 py-2 text-sm text-muted hover:text-foreground hover:bg-surfaceHover rounded-md transition-colors">God Mode</Link>

            <p className="px-3 pt-4 pb-1 text-xs font-semibold text-muted uppercase tracking-wider">Governance</p>
            <Link href="/audit" className="block px-3 py-2 text-sm text-muted hover:text-foreground hover:bg-surfaceHover rounded-md transition-colors">Audit Log</Link>
            <Link href="/requests" className="block px-3 py-2 text-sm text-muted hover:text-foreground hover:bg-surfaceHover rounded-md transition-colors">Access Requests</Link>

            <p className="px-3 pt-4 pb-1 text-xs font-semibold text-muted uppercase tracking-wider">Operations</p>
            <Link href="/zitadel" className="block px-3 py-2 text-sm text-muted hover:text-foreground hover:bg-surfaceHover rounded-md transition-colors">Zitadel Diagnostics</Link>
          </>
        ) : (
          <>
            <p className="px-3 pt-4 pb-1 text-xs font-semibold text-muted uppercase tracking-wider">Access</p>
            <Link href="/requests" className="block px-3 py-2 text-sm text-muted hover:text-foreground hover:bg-surfaceHover rounded-md transition-colors">My Requests</Link>
          </>
        )}
      </nav>
      
      <div className="p-4">
        <Card className="!p-4 bg-surfaceHover border-none shadow-none">
          <p className="text-xs text-muted">Signed in as</p>
          <div className="mt-2">
            <p className="text-sm font-medium">{session.name}</p>
            <p className="text-xs text-muted">{session.role === "admin" ? "Admin session" : "Member session"}</p>
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
        </Card>
      </div>
    </div>
  );
}
