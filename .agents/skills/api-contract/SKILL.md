---
name: api-contract
description: Protect the frontend/backend boundary by treating the API contract as shared infrastructure.
---

# Purpose
To maintain API consistency and ensure backend changes do not silently break frontend assumptions or diverge from the OpenAPI specification.

# Use When
Modifying handlers, changing DTOs, or updating frontend API clients.

# Required Context
- `api/openapi.yaml`
- Dependency chain: `OpenAPI` → `handler` → `service` → `domain/repository` → `frontend client/types` → `fixtures/tests`

# Workflow
1. **Inspect Spec**: Start by reviewing `openapi.yaml`.
2. **Analyze Change**: Check request shape, response shape, status codes, error behaviors, IDs/relationships, pagination/filter semantics, versioning compatibility, and frontend assumptions.
3. **Implement**: Update the spec, backend handler, frontend client, and fixtures consistently.
4. **Handle Breaking Changes**: If breaking, explicitly identify them, update every known consumer, add/update tests, and make the compatibility decision deliberate.

# Verification
- Backend implementation matches the OpenAPI specification exactly.
- Frontend types align with the updated contract.
- Status codes and error shapes are consistent.

# Guardrails
- **DO NOT** silently change API semantics just because the server implementation can support it.
- **DO NOT** make frontend assumptions and then retrofit the backend without checking the contract.

# Related Skills
- Use `frontend-integration` when applying contract changes to the UI.
- Use `database-change` if the contract change requires schema updates.
