## 1. Define the production gate

- [ ] 1.1 Document the production security-boundary requirements for live orchestration
- [ ] 1.2 Record the single-writer onboarding decision and its rationale
- [ ] 1.3 Align the coverage matrix and roadmap language with the production-gate framing

## 2. Specify backend-owned onboarding

- [ ] 2.1 Update `automation-policies` requirements so validated trigger intake and backend-owned mutation responsibilities are testable
- [ ] 2.2 Define idempotency, audit, and retry expectations for welcome-bundle assignment
- [ ] 2.3 Clarify what event sources are allowed to signal onboarding without becoming mutation authorities

## 3. Specify the security boundary

- [ ] 3.1 Add a `production-security-boundary` capability spec covering backend user-token authorization, data-plane perimeter rules, webhook authenticity, and operator-visible degraded behavior
- [ ] 3.2 Update claim-path requirements to define production degraded-mode expectations
- [ ] 3.3 Extend contract-quality and backend-api-testing expectations for high-risk orchestration paths

## 4. Prepare implementation guidance

- [ ] 4.1 Record the recommended rollout order for backend user-token authorization, webhook validation, claim-path hardening, and onboarding automation
- [ ] 4.2 Document the minimum acceptance bar before real Zitadel mutation flows are treated as production-ready
