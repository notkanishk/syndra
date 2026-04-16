# MkAuth - Architecture Decision Record

## Overview
MkAuth is an authentication and authorization orchestration service that bridges multiple identity providers (Zitadel, LLDAP) with a unified management layer. The project is TypeScript-based with 182 files across 88 folders, featuring 449 functions, 71 classes, and 57 interfaces.

## Components
- **Role Management Service**: Manages roles with Zitadel synchronization and database persistence. Handles role CRUD operations and keeps them in sync with the Zitadel IAM.
- **Provisioning Intent System**: Implements a provisioning intent workflow for LLDAP synchronization, with dedicated database schema and API endpoints. Intents represent desired state changes that get reconciled with LLDAP.
- **Webhook Event Handler**: Persistent webhook event handling with Zitadel event dispatching and role-aware revocation logic. Events are stored and processed reliably.
- **Zitadel Management API Client**: JWT-based machine-to-machine authentication with grant lifecycle management. Handles token acquisition and API calls to Zitadel.
- **API Routes**: 5 indexed routes — compact API surface for managing the orchestration layer.

## Integrations
- **Zitadel**: Primary IAM provider. Connected via Management API using JWT M2M auth. Used for role sync, grant management, and webhook event sourcing.
- **LLDAP**: Lightweight LDAP directory. Synchronized via the provisioning intent system for user/group management.
- **3 HTTP_CALLS**: Cross-service API integrations captured in the graph, reflecting the Zitadel and LLDAP connections.

## Data Flow
1. Webhook events arrive from Zitadel → stored persistently → dispatched for processing
2. Role changes flow through the Role Management Service → synced to Zitadel via Management API
3. Provisioning intents are created via API → reconciled with LLDAP directory
4. 66 file co-change patterns indicate tightly coupled modules that evolve together

## Conventions
- TypeScript throughout
- Database-backed persistence for events, intents, and roles
- Docker Compose for infrastructure with parameterized environment variables
- 176 test edges indicating systematic test coverage across the codebase
- Service-oriented architecture with clear separation between IAM providers