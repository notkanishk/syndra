// Next's client modules (next/link) read process.env.__NEXT_* at render time.
// Inside Next the compiler inlines them; in a plain browser there is no
// `process`, so give them an empty env. Imported first by entry.tsx.
const g = globalThis as unknown as { process?: { env: Record<string, string | undefined> } };
g.process ??= { env: {} };

// lib/query-client calls React.cache (React 19) at module load; design hosts
// supply React 18. Outside a server render cache is a pass-through anyway.
const R = (globalThis as unknown as { React?: { cache?: <F>(fn: F) => F } }).React;
if (R && !R.cache) R.cache = (fn) => fn;
