# Proposal: Bun-Native Migration — ARCHIVED

> **Status:** Archived. Bun adoption is complete. The UI uses `bun.lock`, `bun run`, and `oven/bun` Docker images. This change is closed.

## Objective
Transition the Syndra UI to a **Bun-native** ecosystem. This replaces the legacy Node.js environment with an all-in-one toolkit that optimizes build times and simplifies the containerized runtime stack.

## Scope
- **Development**: Migrate all local scripts and dev servers to `bun run`.
- **Package Management**: Adopt `bun install` and `bun.lock` as the project standards.
- **Production**: Standardize on the Bun runtime for Dockerized deployments.

## Context
As the project modernizes for 2026, standardizing on Bun aligns with the goal of maximizing performance and developer velocity. All legacy Node.js/npm dependencies and runtime references have been permanently removed.
