export default function ApplicationsView() {
  return (
    <div className="space-y-6">
      <header>
        <h1 className="text-3xl font-bold">Application-Centric View</h1>
        <p className="text-zinc-400 mt-2">Manage claim-shaping rules and preview specific JWT token outputs.</p>
      </header>
      
      <div className="grid grid-cols-2 gap-6">
        <div className="bg-zinc-900 border border-zinc-800 rounded-lg p-6">
          <h3 className="text-lg font-medium">Claim Shaping Mappings</h3>
          <p className="text-sm text-zinc-500 mt-1">Map internal hierarchical roles to exact JWT claims for legacy systems.</p>
        </div>

        <div className="bg-zinc-900 border border-zinc-800 rounded-lg p-6">
          <h3 className="text-lg font-medium text-amber-500">Token Simulator</h3>
          <p className="text-sm text-zinc-500 mt-1">Preview exact JWT payload structure before deployment.</p>
          <div className="mt-4 bg-black border border-zinc-800 rounded p-4 font-mono text-sm text-emerald-400 overflow-x-auto shadow-inner">
            <pre>{`{
  "iss": "https://auth.makerspace.edu",
  "sub": "user_id_12345",
  "x-custom-group": [
    "student",
    "3d_lab_entry_certified"
  ],
  "door_groups": "room-a,room-b"
}`}</pre>
          </div>
        </div>
      </div>
    </div>
  );
}
