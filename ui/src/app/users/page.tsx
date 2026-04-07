export default function UsersView() {
  return (
    <div className="space-y-6">
      <header>
        <h1 className="text-3xl font-bold">User-Centric View</h1>
        <p className="text-zinc-400 mt-2">Visually trace source vs derived underlying roles across projects.</p>
      </header>
      
      <div className="bg-zinc-900 border border-zinc-800 rounded-lg p-6">
        <h3 className="text-lg font-medium">Access Lineage</h3>
        <p className="text-sm text-zinc-500 mt-1">Select a user to explicitly answer: "Why does this user have access to X?"</p>
        
        {/* Placeholder table for user roles grouped by project */}
        <div className="mt-4 border border-zinc-800 rounded bg-zinc-950 p-4">
          <div className="flex justify-between border-b border-zinc-800 pb-2 mb-2 text-sm font-semibold">
            <span>Project</span>
            <span>Bundle / Source Component</span>
            <span>Raw Role Name</span>
          </div>
          <div className="flex justify-between text-sm text-zinc-300">
            <span>Machine Shop</span>
            <span className="text-emerald-400 font-medium">Student Bundle (Derived)</span>
            <span className="font-mono text-xs">basic_tool_use</span>
          </div>
          <div className="flex justify-between text-sm text-zinc-300 mt-2">
            <span>Door Access</span>
            <span className="text-blue-400 font-medium">Direct Rule (Source)</span>
            <span className="font-mono text-xs">3d_lab_pin</span>
          </div>
        </div>
      </div>
    </div>
  );
}
