import Link from 'next/link';
import { Card } from './ui/Card';

export default function Sidebar() {
  return (
    <div className="w-64 h-screen bg-surface border-r border-border flex flex-col">
      <div className="p-6">
        <h2 className="text-xl font-bold text-foreground tracking-tight">MkAuth</h2>
        <p className="text-xs text-primary font-semibold uppercase tracking-widest mt-1">Control Plane</p>
      </div>
      <nav className="flex-1 px-4 space-y-1">
        <p className="px-3 py-1 text-xs font-semibold text-muted uppercase tracking-wider">Dashboard</p>
        <Link href="/" className="block px-3 py-2 text-sm text-muted hover:text-foreground hover:bg-surfaceHover rounded-md transition-colors">Overview</Link>
        
        <p className="px-3 pt-4 pb-1 text-xs font-semibold text-muted uppercase tracking-wider">Identity</p>
        <Link href="/users" className="block px-3 py-2 text-sm text-muted hover:text-foreground hover:bg-surfaceHover rounded-md transition-colors">Users &amp; Access</Link>
        <Link href="/applications" className="block px-3 py-2 text-sm text-muted hover:text-foreground hover:bg-surfaceHover rounded-md transition-colors">Applications</Link>
        <Link href="/projects" className="block px-3 py-2 text-sm text-muted hover:text-foreground hover:bg-surfaceHover rounded-md transition-colors">Projects</Link>
        
        <p className="px-3 pt-4 pb-1 text-xs font-semibold text-muted uppercase tracking-wider">Policy Engine</p>
        <Link href="/bundles" className="block px-3 py-2 text-sm text-muted hover:text-foreground hover:bg-surfaceHover rounded-md transition-colors">Bundles</Link>
        <Link href="/policies" className="block px-3 py-2 text-sm text-muted hover:text-foreground hover:bg-surfaceHover rounded-md transition-colors">Mapping Rules</Link>
        
        <p className="px-3 pt-4 pb-1 text-xs font-semibold text-muted uppercase tracking-wider">Governance</p>
        <Link href="/audit" className="block px-3 py-2 text-sm text-muted hover:text-foreground hover:bg-surfaceHover rounded-md transition-colors">Audit Log</Link>
        <Link href="/requests" className="block px-3 py-2 text-sm text-muted hover:text-foreground hover:bg-surfaceHover rounded-md transition-colors">Access Requests</Link>
      </nav>
      
      <div className="p-4">
        <Card className="!p-4 bg-surfaceHover border-none shadow-none">
          <p className="text-xs text-muted">Environment</p>
          <div className="flex items-center mt-2">
            <span className="w-2 h-2 rounded-full bg-emerald-500 mr-2"></span>
            <span className="text-sm font-medium">Proxmox LXC</span>
          </div>
        </Card>
      </div>
    </div>
  );
}
