# Tasks: Bun Migration

## Phase 1: Preparation
- [ ] Create Change Proposal (Done)
- [ ] Verify local Bun installation (`bun -v`)

## Phase 2: Environment Purge
- [ ] Delete `ui/node_modules`
- [ ] Delete `ui/package-lock.json`

## Phase 3: Bun Initialization
- [ ] Run `bun install` in `ui/` to generate `bun.lockb`
- [ ] Remove `engines.node` from `ui/package.json`
- [ ] Add `@types/bun` to `devDependencies`

## Phase 4: Infrastructure
- [ ] Refactor `ui/Dockerfile` for `oven/bun`
- [ ] Verify build with `bun run build`

## Phase 5: Verification
- [ ] Run `bun run dev` and test UI accessibility
- [ ] Verify `docker-compose up` with the new Bun-based image
