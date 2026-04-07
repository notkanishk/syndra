export default function PoliciesView() {
  return (
    <div className="space-y-6">
      <header>
        <h1 className="text-3xl font-bold">Policy Editor</h1>
        <p className="text-zinc-400 mt-2">Notion-style logical row builder for structural role behaviors.</p>
      </header>
      
      <div className="bg-zinc-900 border border-zinc-800 rounded-lg p-6">
        <div className="flex items-center justify-between mb-4">
          <h3 className="text-lg font-medium">Active Structural Rules</h3>
          <button className="bg-emerald-600 hover:bg-emerald-500 text-white px-4 py-2 rounded-md justify-center font-medium text-sm transition-colors shadow-sm">
            + New Logic Rule
          </button>
        </div>
        
        {/* Placeholder Notion-style row */}
        <div className="mt-4 p-4 border border-zinc-800 rounded bg-zinc-950 flex items-center space-x-3 text-sm">
          <span className="font-mono text-zinc-500 mr-2">IF</span>
          <span className="bg-blue-900/40 border border-blue-800 text-blue-300 px-2 py-1 rounded">Project: Printing</span>
          <span className="bg-emerald-900/40 border border-emerald-800 text-emerald-300 px-2 py-1 rounded">Role: User</span>
          
          <span className="font-mono text-zinc-500 mx-2">THEN ADD</span>
          
          <span className="bg-purple-900/40 border border-purple-800 text-purple-300 px-2 py-1 rounded">Project: Door Access</span>
          <span className="bg-amber-900/40 border border-amber-800 text-amber-300 px-2 py-1 rounded">Role: 3D Lab Pin</span>
        </div>
      </div>
    </div>
  );
}
