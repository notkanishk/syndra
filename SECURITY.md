# Security Policy

Syndra sits in the access-control path for physical and digital infrastructure.
A vulnerability here can mean someone opening a door they should not. Reports
are taken seriously.

## Reporting a vulnerability

**Do not open a public issue.**

Use GitHub's private reporting: **Security → Advisories → Report a
vulnerability** on this repository. That opens a private thread visible only to
maintainers.

Useful to include, in rough order of value:

1. What an attacker gains — the impact, in one sentence.
2. Steps to reproduce, or a proof of concept.
3. Whether it requires an authenticated session, and at what role.
4. Which mode you tested against: local demo, or a live Zitadel instance.

You will get an acknowledgement within a few days. This is a single-maintainer
project, so please allow reasonable time for a fix before disclosing publicly.

## Scope

**In scope** — anything that lets a request obtain access it was not granted:

- Authentication or session handling in the UI (`ui/src/middleware.ts`,
  `ui/src/lib/oidc.ts`, `ui/src/lib/request-url.ts`)
- Authorization on backend endpoints, including the internal API key path
- Zitadel Actions v2 signature verification (`/api/action/inject`,
  `/api/webhooks/zitadel`)
- The policy engine producing grants that the rules do not imply
- Secret handling — the machine key, signing keys, session secret
- SQL injection, SSRF, or injection into the LDAP bridge

**Out of scope:**

- The demo catalog and demo session cookie. These are local-development
  scaffolding, disabled whenever `ZITADEL_DOMAIN` is set, and the middleware
  rejects demo cookies in that mode. A finding that requires demo mode is a
  finding about a development fixture.
- Anything requiring an already-compromised host or an operator with the `admin`
  role acting maliciously. Admins are trusted by design.
- Vulnerabilities in Zitadel itself — report those to
  [Zitadel](https://github.com/zitadel/zitadel/security).
- Missing hardening headers on endpoints that serve no content.

## Known design decisions

These are deliberate, not oversights. Please do not report them as bugs — but do
report a way to *break* one.

- **The internal API key is defence in depth, not authorization.** Privileged
  actions require a validated Zitadel user access token. The shared key exists
  for service-to-service traffic and is explicitly not sufficient on its own.
- **Signature verification can be disabled in local development.** When
  `ZITADEL_DOMAIN` is unset, the Actions endpoints accept unsigned requests and
  log a warning. In any configuration with a live Zitadel, the backend refuses
  to start without signing keys — so this passthrough is unreachable in
  production, by construction.
- **Shadow credentials for the LDAP bridge are a bridge, not an identity
  model.** Some makerspace equipment authenticates only by password. That
  handling is deliberately isolated from the OIDC path and independently
  auditable.
- **Zitadel is authoritative.** Syndra will not defend against a change made
  directly in Zitadel — it detects it as drift and surfaces it for triage. That
  is the intended behaviour.

## What we do on our side

- Secrets are generated per-deployment at mode 600 and never committed. The
  repository history has been scanned; it contains no keys or credentials.
- Signing keys carry a rotation age, surfaced in the operator console.
- Every Syndra-mediated Zitadel mutation leaves a trace before the API call, so
  a change with no trace is detectable rather than silent.
