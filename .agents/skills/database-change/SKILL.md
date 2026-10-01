---
name: database-change
description: Prevent mismatches between PostgreSQL schema, migrations, domain structures, repositories, services, and API.
---

# Purpose
To safely evolve the database layer while maintaining consistency across the entire backend stack.

# Use When
Adding new tables, modifying columns, adding indexes, or changing data models.

# Required Context
- `database/migrations/`
- Dependency chain: `schema` → `migration` → `repository` → `domain` → `service` → `handler/API` → `frontend/client`

# Workflow
1. **Inspect Existing**: Inspect existing migrations and determine the correct migration sequence.
2. **Review Conventions**: Inspect current repository conventions and related domain models.
3. **Analyze Dependencies**: Use Graphify to identify dependency relationships.
4. **Implement Migration**: Prefer a *new* migration for schema evolution.
5. **Verify SQL**: Check indexes, foreign keys, nullability, uniqueness, and transaction expectations.
6. **Update Code**: Verify migration/repository/domain naming consistency.
7. **Test**: Test relevant repository/application behavior.
8. **Tooling Check**: Verify migration application where tooling exists, and verify API behavior if the schema change surfaces externally.

# Verification
- The schema change is captured correctly.
- Repositories and domain models accurately reflect the new schema.
- API behavior is updated if necessary.

# Guardrails
- **DO NOT** silently modify old, applied migrations.
- **NEVER** invent schema conventions when existing migrations already establish one.

# Related Skills
- Use `graphify` to trace domain impacts.
- Use `api-contract` if database changes alter the API.
- Use `testing-hardening` for repository tests.
- Use `git-checkpoint` after completing the vertical slice.
