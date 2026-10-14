# Design

`RoleMapping` has one canonical ten-column scan shape. `ListRoleMappings`
already selects that complete shape, but `MappingsForRoles` retained the older
eight-column projection after timestamps were added to the model and scanner.

The query should include both timestamp columns rather than adding a second,
partial scanner. Keeping one row shape prevents list and resolver reads from
interpreting the same record differently. The regression uses the existing
opt-in live-database harness because mocked rows or SQL-text checks can agree
with the same wrong column count and cannot prove pgx accepts the query result.
