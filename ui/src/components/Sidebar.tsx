import Link from 'next/link';

export default function Sidebar() {
  return (
    <div className="w-64 h-screen bg-zinc-900 border-r border-zinc-800 flex flex-col">
      <div className="p-6">
        <h2 className="text-xl font-bold text-white tracking-tight">MkAuth</h2>
        <p className="text-xs text-zinc-500 uppercase tracking-widest mt-1">Control Plane</p>
      </div>
      <nav className="flex-1 px-4 space-y-2">
        <Link href="/" className="block px-3 py-2 text-sm text-zinc-400 hover:text-white hover:bg-zinc-800 rounded-md transition-colors">Overview</Link>
        <Link href="/users" className="block px-3 py-2 text-sm text-zinc-400 hover:text-white hover:bg-zinc-800 rounded-md transition-colors">Users & Access</Link>
        <Link href="/applications" className="block px-3 py-2 text-sm text-zinc-400 hover:text-white hover:bg-zinc-800 rounded-md transition-colors">Applications</Link>
        <Link href="/policies" className="block px-3 py-2 text-sm text-zinc-400 hover:text-white hover:bg-zinc-800 rounded-md transition-colors">Policy Editor</Link>
      </nav>
    </div>
  );
}
