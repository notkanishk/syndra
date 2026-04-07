import Link from 'next/link';
import { Card } from './ui/Card';

export default function Sidebar() {
  return (
    <div className="w-64 h-screen bg-surface border-r border-border flex flex-col">
      <div className="p-6">
        <h2 className="text-xl font-bold text-foreground tracking-tight">MkAuth</h2>
        <p className="text-xs text-primary font-semibold uppercase tracking-widest mt-1">Control Plane</p>
      </div>
      <nav className="flex-1 px-4 space-y-2">
        <Link href="/" className="block px-3 py-2 text-sm text-muted hover:text-foreground hover:bg-surfaceHover rounded-md transition-colors">Overview</Link>
        <Link href="/users" className="block px-3 py-2 text-sm text-muted hover:text-foreground hover:bg-surfaceHover rounded-md transition-colors">Users & Access</Link>
        <Link href="/applications" className="block px-3 py-2 text-sm text-muted hover:text-foreground hover:bg-surfaceHover rounded-md transition-colors">Applications</Link>
        <Link href="/policies" className="block px-3 py-2 text-sm text-muted hover:text-foreground hover:bg-surfaceHover rounded-md transition-colors font-medium">Policy Editor</Link>
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
