# ui

The operator console. Next.js App Router on the Bun runtime.

```bash
bun install
bun run dev            # :3000
bun run test           # vitest — NOT `bun test`, see below
bun run lint
bun run build
```

> **`bun run test`, never `bun test`.** The latter is Bun's own test runner. It
> picks up the same files but knows nothing about the vitest API they are
> written against, and reports around 75 failures on a completely healthy tree.

## Layout

| Path | Contents |
|---|---|
| `src/app/` | Routes. One directory per surface — `users/`, `roles/`, `bundles/`, `governance/`, `zitadel/`, … |
| `src/app/auth/` | OIDC route handlers: `zitadel` (initiate), `callback`, `login`, `logout` |
| `src/app/api/proxy/` | The only route the UI owns under `/api`. Forwards to the backend with the session's bearer token |
| `src/components/` | Shared components |
| `src/lib/` | Client, session, OIDC, formatting, query hooks |
| `src/middleware.ts` | Session gate on every request |

## Conventions that are not negotiable

**Design tokens live only in `src/app/globals.css`.** Never a hardcoded colour
in a component. Both light and dark are authored in full — a theme that is
"mostly" defined is a theme with a hole in it.

**Navigation structure lives only in `src/lib/nav.ts`.** Structure never moves in
response to data: a nav item does not disappear because a list came back empty.
An operator learning where things are should not have to relearn it when the
system state changes. See the `basic-advanced-ia` design note.

**Absolute URLs come from `src/lib/request-url.ts`.** Every one of them —
redirects, the OIDC `redirect_uri`, and the cookie `secure` decision.

Behind a reverse proxy the app never observes the URL the browser requested:
Next's standalone server builds `request.url` from the address it listens on, so
in production every request arrives looking like `http://localhost:3000/...`.
Only `x-forwarded-host` and `x-forwarded-proto` know otherwise.

Build the origin from scratch. Do not mutate the incoming URL — the WHATWG
`host` setter replaces the port *only* when the value assigned carries one, so a
bare hostname assigned over a `:3000` request URL silently keeps the port. That
sent every redirect to `…:3000` in production and handed Zitadel a
`redirect_uri` it had no reason to accept. This logic previously existed as
three identical copies plus two variants that skipped the headers entirely, and
fixing the copy named in the error message left the other five broken. One
module, one implementation.

## Modes

With `ZITADEL_DOMAIN` unset the console runs against the demo catalog and
accepts a demo session cookie with an Admin/User toggle — the whole UI is
explorable with no external service.

With a live Zitadel configured, demo cookies are rejected by the middleware and
demo entities are never serialized. The two modes are deliberately not a
spectrum.
