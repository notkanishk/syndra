# Keep role-mapping reads aligned with their scanner

## Problem

Applying a target mapping fails before convergence when `MappingsForRoles`
returns a row. Its query selects eight columns while the shared mapping scanner
requires ten, so pgx reports `number of field descriptions must equal number of
destinations, got 8 and 10`.

## Change

Return `created_at` and `updated_at` from the role-set lookup, matching every
other read that uses the shared scanner. Add a live PostgreSQL regression test
that inserts a mapping and exercises the production query and scanner together.
