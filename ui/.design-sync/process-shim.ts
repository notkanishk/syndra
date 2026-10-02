// Next's client modules (next/link) read process.env.__NEXT_* at render time.
// Inside Next the compiler inlines them; in a plain browser there is no
// `process`, so give them an empty env. Imported first by entry.tsx.
const g = globalThis as unknown as { process?: { env: Record<string, string | undefined> } };
g.process ??= { env: {} };
