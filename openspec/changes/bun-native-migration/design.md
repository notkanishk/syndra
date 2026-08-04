# Design: Bun-Native Architecture

## Rationale
Bun provides an all-in-one toolkit (runtime, package manager, and bundler) for the Syndra ecosystem. By adopting Bun, we eliminate the legacy Node.js/npm dependencies, significantly reducing container image sizes and accelerating cold start times.

## Technical Specification

### 1. Docker Environment
The UI will use a multi-stage Docker build based on `oven/bun:alpine`.

```dockerfile
# Build Stage
FROM oven/bun:alpine as builder
WORKDIR /app
COPY package.json bun.lockb ./
RUN bun install --frozen-lockfile
COPY . .
RUN bun run build

# Runner Stage
FROM oven/bun:alpine as runner
WORKDIR /app
COPY --from=builder /app/.next/standalone ./
# ... etc
CMD ["bun", "server.js"]
```

### 2. Package Management
- **Installation**: Use `bun install`.
- **Lockfile**: `bun.lockb` will be the source of truth.
- **Engines**: `package.json` will no longer specify a Node version.

### 3. TypeScript Integration
Bun supports TypeScript out of the box. `@types/node` may be retained for specific Next.js/React peer dependencies if necessary, but will be ideally replaced by `@types/bun` for native Bun APIs.
