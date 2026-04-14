## 1. Database

- [x] 1.1 Create `000008_roles.up.sql` migration
- [x] 1.2 Create `000008_roles.down.sql` migration

## 2. Models

- [x] 2.1 Add `Role` struct to `models.go`
- [x] 2.2 Add `CatalogRole` struct to `models.go`

## 3. Zitadel Client

- [x] 3.1 Add `ProjectRoleResult` type to `orchestrator.go`
- [x] 3.2 Add `AddProjectRole` to ZitadelClient interface and managementClient
- [x] 3.3 Add `ListProjectRoles` to ZitadelClient interface and managementClient
- [x] 3.4 Add `UpdateProjectRole` to ZitadelClient interface and managementClient
- [x] 3.5 Update mockClient in orchestrator_test.go

## 4. Repository

- [x] 4.1 Add `RoleUsage` type and `ErrDuplicateRole` sentinel
- [x] 4.2 Add `CreateRole` with ON CONFLICT DO NOTHING
- [x] 4.3 Add `GetRole` by natural key
- [x] 4.4 Add `GetAllLocalRoles`
- [x] 4.5 Add `GetRoleUsageCounts` (bundle_roles + mapping_rules)
- [x] 4.6 Add `GetAssignedUserCounts` (direct_role_grants)
- [x] 4.7 Add `GetAllReferencedRoleKeys` (union across all referencing tables)

## 5. Service layer

- [x] 5.1 Create `services/roles.go` with CreateRoleRequest and CloneRef types
- [x] 5.2 Implement `CreateRole` with clone resolution, Zitadel propagation, local persistence
- [x] 5.3 Implement `GlobalRoleCatalog` with three-source merge and usage computation
- [x] 5.4 Add `resolveRoleMetadata` (local DB -> demo catalog fallback)

## 6. Injectable deps

- [x] 6.1 Add role DB vars to `services/deps.go`
- [x] 6.2 Add role service vars to `handlers/deps.go`

## 7. HTTP handlers

- [x] 7.1 Create `handlers/roles.go` with handleCreateRole and handleGetGlobalRoleCatalog
- [x] 7.2 Add POST and GET /api/v1/roles routes to router.go

## 8. Tests

- [x] 8.1 `TestCreateRole_HappyPath` — 201 with correct JSON
- [x] 8.2 `TestCreateRole_EmptyProjectID` — 400
- [x] 8.3 `TestCreateRole_EmptyRoleKey` — 400
- [x] 8.4 `TestCreateRole_EmptyDisplayName` — 400
- [x] 8.5 `TestCreateRole_UnknownField` — 400 (strict decode)
- [x] 8.6 `TestCreateRole_WithCloneFrom` — 201, clone metadata passed through
- [x] 8.7 `TestCreateRole_CloneFromNotFound` — 404
- [x] 8.8 `TestCreateRole_Duplicate` — 409
- [x] 8.9 `TestGetGlobalRoleCatalog_MergesSources` — returns demo + local + referenced
- [x] 8.10 `TestGetGlobalRoleCatalog_UnusedFlag` — roles with 0 usage + 0 users flagged
- [x] 8.11 `TestGetGlobalRoleCatalog_ProjectFilter` — query param filters
- [x] 8.12 `TestCreateRole_PropagatesCloneMetadata` — clone source populates fields
- [x] 8.13 `TestCreateRole_SkipsZitadelWhenNilClient` — local-policy-only
- [x] 8.14 `TestGlobalRoleCatalog_Deduplicates` — same role in demo + DB merged
- [x] 8.15 `TestGlobalRoleCatalog_DisplayLabel` — format is "ProjectName: DisplayName"
- [x] 8.16 `TestAddProjectRole_HappyPath` — correct URL and body
- [x] 8.17 `TestListProjectRoles_ParsesResponse` — correct parsing
- [x] 8.18 `TestUpdateProjectRole_CorrectEndpoint` — PUT with correct path
- [x] 8.19 `TestCreateRole_ZitadelFailureRollsBackLocalRow` — compensating delete on Zitadel failure
- [x] 8.20 `TestResolveRoleMetadata_DBErrorSurfaces` — real DB errors not masked as 404

## 9. P1/P2 fixes

- [x] 9.1 Reorder CreateRole: persist locally first, then propagate to Zitadel with compensating rollback on failure
- [x] 9.2 Add `DeleteRole` repository function for compensating rollback
- [x] 9.3 Distinguish `pgx.ErrNoRows` from real DB errors in `resolveRoleMetadata`
