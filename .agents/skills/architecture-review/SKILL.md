---
name: architecture-review
description: Protect CURRA's architectural boundaries during implementation and review.
---

# Purpose
To maintain the strict separation of concerns within the CURRA repository and prevent boundary leakage between the application and the solver core.

# Use When
Designing a solution, reviewing PRs, or modifying boundary layers (API, services, adapter).

# Required Context
- High-level layering: `web` → `API handlers` → `services` → `CURRA adapter` → `core scheduler`

# Workflow
1. **Check Ownership**: Ensure the logic resides in the correct package (e.g., business logic in `services`, hard constraints in `internal/scheduler`).
2. **Check Dependencies**: Validate that dependencies point inward.
3. **Validate Invariants**: Ensure the adapter boundary is respected.
4. **Question Maintenance Risk**: Ask "Does this change violate an existing boundary or create concrete maintenance risk?" rather than "Can this architecture be redesigned?"

# Verification
- **Only `application/internal/curra` may import the core `internal/scheduler` packages.**
- No solver concerns have leaked into API/frontend layers.
- The adapter has not been accidentally bypassed.
- Application concerns (like database transactions) have not leaked into the solver core.
- No inappropriate shared mutable state exists.
- Business logic is not duplicated.
- No unnecessary coupling between layers exists.

# Guardrails
- **NEVER** recommend refactors merely because another architecture might look cleaner.
- Protect known architectural invariants explicitly defined in CURRA_CONTEXT.md.

# Related Skills
- Use `graphify` to trace boundary dependencies.
- Use `solver-change` if the core is affected.
- Use `api-contract` if API handlers must be updated.
