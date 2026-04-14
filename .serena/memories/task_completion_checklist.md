# Task Completion Checklist

After completing any code change:

1. `cd backend && go build ./...` — Verify compilation
2. `cd backend && go vet ./...` — Static analysis
3. `cd backend && go test ./...` — All backend tests pass
4. `cd ui && bun run build` — Frontend builds (if UI changed)
5. `cd ui && bun test` — Frontend tests pass (if UI changed)
6. Update OpenSpec docs if the change is tracked:
   - Check off tasks in `tasks.md`
   - Update `ROADMAP.md` if a phase item is completed
   - Update `feature-coverage.md` if status changed
   - Update `AUDIT.md` architecture map if new files added
7. Update `.env.example` if new environment variables introduced
