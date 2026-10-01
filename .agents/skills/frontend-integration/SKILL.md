---
name: frontend-integration
description: Help frontend agents build the CURRA UI against the existing backend architecture without inventing parallel semantics.
---

# Purpose
To ensure frontend development aligns strictly with existing backend contracts and domain terminology without introducing redundant complexity.

# Use When
Implementing React UI components, integrating API calls, or designing frontend state.

# Required Context
- `api/openapi.yaml`
- `CURRA_CONTEXT.md`

# Workflow
1. **Contextualize**: Read CURRA_CONTEXT.md.
2. **Analyze Structure**: Inspect Graphify for frontend/API structure.
3. **Review Contract**: Inspect the OpenAPI contract and existing web client/types.
4. **Identify Backend Behavior**: Understand the required backend behavior.
5. **Implement UI**: Build the component. Check loading/error/empty states for network-driven views.
6. **Verify**: Test integration with the API.

# Verification
- UI uses existing domain terminology.
- Network access is kept behind the existing client boundary.
- Frontend/backend type consistency is maintained.
- Contract changes (if any) are identified explicitly.

# Guardrails
- **Existing API contracts are authoritative.**
- **DO NOT** duplicate backend business rules in the frontend.
- **DO NOT** invent endpoint behavior.
- **DO NOT** directly couple UI components to backend implementation details.
- **DO NOT** redesign backend architecture to solve a frontend convenience problem.
- Prefer simple UI architecture over premature state-management complexity.
- Avoid touching unrelated backend code or committing another developer's changes (frontend may be developed in parallel).
- Clearly separate UI-only changes from API changes.

# Related Skills
- Use `api-contract` to verify endpoint rules.
- Use `graphify` to understand the frontend/backend boundary.
- Use `testing-hardening` for UI tests.
- Use `git-checkpoint` to isolate UI-only changes from API changes.
