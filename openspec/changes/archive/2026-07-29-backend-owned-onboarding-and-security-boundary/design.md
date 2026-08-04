# Backend-Owned Onboarding and Production Security Boundary Design

## 1. Goal

Syndra needs a clean production gate for live orchestration. The system should not move from demo/local-policy mode into real Zitadel-backed mutation flows until it has one mutation authority, one trustworthy backend user-token authorization boundary for privileged actions, and one clearly documented degraded-behavior story for its production data plane.

This change defines that gate.

## 2. Core Decisions

### 2.1 Welcome-bundle mutation ownership

Welcome-bundle assignment is a business mutation, not just an event reaction. Zitadel-compatible triggers may indicate that a new user exists, but Syndra Backend must remain the single writer that decides whether, when, and how onboarding mutations occur.

```text
Zitadel event / hook / action signal
              |
              v
      Syndra intake validation
              |
              v
      policy + idempotency check
              |
              v
      backend-owned assignment
              |
              v
     audit log + retry visibility
```

This keeps auditability, retries, idempotency, and policy evaluation in one place.

### 2.2 Production gate before broader live orchestration

Syndra should treat the following as a precondition for production-grade live Zitadel mutation flows:

* backend user-token authorization for privileged actions
* authenticated and bounded action-injection/data-plane access
* validated webhook authenticity before mutation or invalidation work
* explicit degraded behavior for claim injection
* operator visibility into onboarding and data-plane failures

These are not polish items. They are trust-boundary requirements.

## 3. Responsibility Split

### 3.1 Zitadel-compatible triggers

Zitadel Actions v2 or a validated backend webhook may be used to detect that a new user exists or that claim-related work should be re-evaluated. These are compatibility and detection mechanisms.

### 3.2 Backend-owned mutations

All business mutations that change managed state remain backend-owned:

* assigning Welcome Bundles
* reconciling grants or memberships in Zitadel
* invalidating and rebuilding compiled claim state
* emitting provisioning intents

The backend may use the Zitadel service account where required, but the frontend and Zitadel-hosted trigger code must not become independent mutation authorities.

## 4. Production Security Boundary

### 4.1 Backend User-Token Authorization

The backend must stop treating a shared API key as sufficient proof for privileged admin mutations. The production path should require a Zitadel-issued user access token on privileged frontend-to-backend requests, validate that token in the backend, and authorize the acting admin for the requested scope.

Minimum expectations:

* authenticated admin identity reaches the backend through a Zitadel-issued user access token
* backend validates issuer, signature, audience, expiry, and subject claims
* backend performs its own authorization check
* privileged mutations are audit-attributed to the acting admin
* member or proxy-local assumptions cannot escalate into backend trust
* a shared internal API key may remain only as optional defense-in-depth and not as primary authorization proof

### 4.2 Action-injection perimeter

The data plane must stay Actions v2-compatible, but its production posture must be explicit:

* authenticate the caller or invocation path appropriately
* bound latency and timeout handling
* define cache-miss behavior
* define malformed-cache behavior
* surface degraded-mode outcomes to operators

### 4.3 Webhook authenticity boundary

Webhook receipt is not enough. Before any cache invalidation, onboarding trigger, or downstream orchestration occurs, Syndra must validate that the event is authentic, fresh enough, and structurally valid.

### 4.4 High-risk credential-bridge handling

Samba/LLDAP password flows are required now, but they sit on the same production boundary. Any onboarding or provisioning path that transports infrastructure secrets must follow stricter internal contract rules than ordinary control-plane payloads.

## 5. Failure Model

### 5.1 Onboarding failures

If onboarding cannot complete, the system should avoid silent drift.

Expected posture:

* the trigger intake is recorded
* idempotency prevents duplicate grants on retry
* failures are visible for operators
* replay or retry can occur without corrupting state

### 5.2 Claim-injection failures

Applications need an explicit failure posture. Syndra should support one of two documented models per application:

* fail closed: deny or emit no effective claims
* minimal safe fallback: emit a constrained documented claim set

Implicit fallback behavior should not exist.

## 6. Rollout Shape

This change should be implemented in this order:

1. backend user-token authorization
2. webhook authenticity validation
3. action-injection perimeter and degraded behavior
4. backend-owned welcome-bundle assignment flow
5. operator visibility and regression coverage across the whole boundary

That order keeps the highest-risk trust assumptions from lingering underneath live orchestration work.
